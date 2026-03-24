# Room Lifecycle Service 重构实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将分散在 `participant_service.go`、`webhook_service.go`、`scheduler_service.go` 三处的房间/参与者状态管理逻辑统一收敛到 `RoomLifecycleService`，通过 CAS 更新解决竞态问题，确保不同入口的业务 webhook 准确下发。

**Architecture:** 新建 `room_lifecycle_service.go` 作为房间生命周期的唯一状态管理者。核心设计：提取共享方法 `processParticipantLeave` 完成状态判定和 CAS 更新，`HandleUserLeave` 和 `HandleMediaDisconnect` 都调用它，各自发送对应的业务 webhook。`HandleTimeout` 独立处理超时逻辑。`RoomLifecycleService` 在 `main.go` 中只创建一个实例，通过 setter 注入到各 service。

**Tech Stack:** Go 1.x, GORM, Redis, Gin

> **部署注意:** Task 3、4、5 必须作为一个整体部署，不可只部署其中一部分。部分部署会导致新旧逻辑混用，CAS 和非 CAS 路径可能相互覆盖状态。

> **性能说明:** `checkParticipantTimeout`（定期兜底）从批量处理改为逐个调用 `HandleTimeout`，会增加 DB 查询次数。这是为代码清晰度做的折中——精确定时器是主要超时检测机制，定期轮询只是兜底，执行频率低。

---

## File Structure

| 文件 | 操作 | 职责 |
|-----|------|------|
| `internal/service/room_lifecycle_service.go` | **新建** | 共享状态判定 `processParticipantLeave` + CAS 更新方法 + 各入口 Handle 方法 |
| `internal/service/participant_service.go` | **修改** | `LeaveRoom` 及其 handle 方法删除，委托 lifecycle |
| `internal/service/webhook_service.go` | **修改** | `handleParticipantLeft` 和 `handleRoomFinished` 简化，委托 lifecycle |
| `internal/service/scheduler_service.go` | **修改** | `checkSingleParticipantTimeout` 和 `checkParticipantTimeout` 简化，委托 lifecycle |
| `internal/service/event.go` | **不变** | webhook 事件构建和发送函数保持原样 |
| `internal/service/business_webhook_service.go` | **不变** | webhook 发送基础设施保持原样 |
| `internal/router/router.go` | **修改** | 接收 `RoomLifecycleService` 并注入到 participantService 和 webhookService |
| `main.go` | **修改** | 创建唯一 `RoomLifecycleService` 实例，传入 router 和 scheduler |

---

### Task 1: 创建 RoomLifecycleService 基础结构

**Files:**
- Create: `internal/service/room_lifecycle_service.go`

- [ ] **Step 1: 创建 `room_lifecycle_service.go` 文件**

```go
package service

import (
	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/utils"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RoomLifecycleService 房间生命周期服务
// 统一管理房间和参与者的状态转换，使用 CAS 防止竞态
type RoomLifecycleService struct {
	db                     *gorm.DB
	businessWebhookService *BusinessWebhookService
}

// NewRoomLifecycleService 创建房间生命周期服务
func NewRoomLifecycleService(db *gorm.DB, businessWebhookService *BusinessWebhookService) *RoomLifecycleService {
	return &RoomLifecycleService{
		db:                     db,
		businessWebhookService: businessWebhookService,
	}
}

// ========== 数据类型 ==========

// LeaveAction 离开动作类型
type LeaveAction int

const (
	LeaveActionSkipped LeaveAction = iota // 已终态或无需处理
	LeaveActionCancel                     // 创建者取消通话
	LeaveActionReject                     // 参与者拒绝通话
	LeaveActionHangup                     // 正常挂断
)

// LeaveResult processParticipantLeave 的返回结果
type LeaveResult struct {
	Action LeaveAction
	Room   *models.Room
	UIDs   []string
}

// RoomSnapshot 房间快照
type RoomSnapshot struct {
	Room         models.Room
	Participants []models.Participant
	UIDs         []string
}

// ========== 工具函数 ==========

// IsRoomTerminal 判断房间是否已进入终态
func IsRoomTerminal(status uint8) bool {
	return status > models.RoomStatusInProgress
}

// FindParticipant 在参与者列表中查找指定 UID 的参与者
func FindParticipant(participants []models.Participant, uid string) *models.Participant {
	for i := range participants {
		if participants[i].UID == uid {
			return &participants[i]
		}
	}
	return nil
}

// CountJoinedParticipants 统计房间中已加入（Joined 或 Hangup）的参与者数量
func CountJoinedParticipants(participants []models.Participant) int {
	count := 0
	for _, p := range participants {
		if p.Status == models.ParticipantStatusJoined || p.Status == models.ParticipantStatusHangup {
			count++
		}
	}
	return count
}

// HasMissedParticipant 检查是否有超时参与者（排除指定 UID）
func HasMissedParticipant(participants []models.Participant, excludeUID string) bool {
	for _, p := range participants {
		if p.UID != excludeUID && p.Status == models.ParticipantStatusMissed {
			return true
		}
	}
	return false
}

// HasOtherJoinedParticipant 检查是否有其他参与者曾经加入过
func HasOtherJoinedParticipant(participants []models.Participant, excludeUID string) bool {
	for _, p := range participants {
		if p.UID != excludeUID && (p.Status == models.ParticipantStatusJoined || p.Status == models.ParticipantStatusHangup || p.LeaveTime > 0) {
			return true
		}
	}
	return false
}

// ========== 快照读取 ==========

// LoadRoomSnapshot 加载房间及所有参与者的当前快照
func (s *RoomLifecycleService) LoadRoomSnapshot(roomID string) (*RoomSnapshot, error) {
	logger := utils.GetLogger()

	var room models.Room
	if err := s.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		logger.Error("加载房间快照失败", zap.String("room_id", roomID), zap.Error(err))
		return nil, err
	}

	var participants []models.Participant
	if err := s.db.Where("room_id = ?", roomID).Find(&participants).Error; err != nil {
		logger.Error("加载参与者快照失败", zap.String("room_id", roomID), zap.Error(err))
		return nil, err
	}

	uids := make([]string, 0, len(participants))
	for _, p := range participants {
		uids = append(uids, p.UID)
	}

	return &RoomSnapshot{
		Room:         room,
		Participants: participants,
		UIDs:         uids,
	}, nil
}

// ========== CAS 更新方法 ==========

// CASUpdateParticipant CAS 更新参与者状态
// 仅当参与者当前状态在 fromStatus 列表中时才执行更新
// 返回 true 表示更新成功（RowsAffected > 0）
func (s *RoomLifecycleService) CASUpdateParticipant(
	roomID, uid string,
	fromStatus []uint8,
	toStatus uint8,
	extras map[string]interface{},
) bool {
	logger := utils.GetLogger()

	updates := map[string]interface{}{"status": toStatus}
	for k, v := range extras {
		updates[k] = v
	}

	result := s.db.Model(&models.Participant{}).
		Where("room_id = ? AND uid = ? AND status IN ?", roomID, uid, fromStatus).
		Updates(updates)

	if result.Error != nil {
		logger.Error("CAS 更新参与者状态失败",
			zap.String("room_id", roomID),
			zap.String("uid", uid),
			zap.Error(result.Error),
		)
		return false
	}

	return result.RowsAffected > 0
}

// CASUpdateRoom CAS 更新房间状态
// 仅当房间当前状态在 fromStatus 列表中时才执行更新
// 返回 true 表示更新成功（RowsAffected > 0）
func (s *RoomLifecycleService) CASUpdateRoom(
	roomID string,
	fromStatus []uint8,
	toStatus uint8,
) bool {
	logger := utils.GetLogger()

	result := s.db.Model(&models.Room{}).
		Where("room_id = ? AND status IN ?", roomID, fromStatus).
		Update("status", toStatus)

	if result.Error != nil {
		logger.Error("CAS 更新房间状态失败",
			zap.String("room_id", roomID),
			zap.Error(result.Error),
		)
		return false
	}

	return result.RowsAffected > 0
}

// UpdateOtherParticipants 更新房间中除指定 UID 外的其他参与者
func (s *RoomLifecycleService) UpdateOtherParticipants(
	roomID, excludeUID string,
	fromStatus []uint8,
	toStatus uint8,
	extras map[string]interface{},
) {
	logger := utils.GetLogger()

	updates := map[string]interface{}{"status": toStatus}
	for k, v := range extras {
		updates[k] = v
	}

	if err := s.db.Model(&models.Participant{}).
		Where("room_id = ? AND uid != ? AND status IN ?", roomID, excludeUID, fromStatus).
		Updates(updates).Error; err != nil {
		logger.Error("更新其他参与者状态失败",
			zap.String("room_id", roomID),
			zap.String("exclude_uid", excludeUID),
			zap.Error(err),
		)
	}
}

// UpdateAllParticipants 更新房间中所有参与者状态
func (s *RoomLifecycleService) UpdateAllParticipants(
	roomID string,
	fromStatus []uint8,
	toStatus uint8,
	extras map[string]interface{},
) {
	logger := utils.GetLogger()

	updates := map[string]interface{}{"status": toStatus}
	for k, v := range extras {
		updates[k] = v
	}

	if err := s.db.Model(&models.Participant{}).
		Where("room_id = ? AND status IN ?", roomID, fromStatus).
		Updates(updates).Error; err != nil {
		logger.Error("更新所有参与者状态失败",
			zap.String("room_id", roomID),
			zap.Error(err),
		)
	}
}

// ResolveAndFinishRoom 检查房间是否应该结束，并发送 room.finished webhook
func (s *RoomLifecycleService) ResolveAndFinishRoom(room *models.Room) {
	if s.businessWebhookService != nil {
		s.businessWebhookService.checkAndFinishRoom(room)
	}
}

// ========== 核心共享方法 ==========

// processParticipantLeave 共享的参与者离开处理逻辑
// 负责：读取状态快照 → 判定业务动作 → CAS 更新 DB
// 不负责：发送 webhook（由调用方根据 LeaveResult.Action 决定）
func (s *RoomLifecycleService) processParticipantLeave(roomID, uid string) (*LeaveResult, error) {
	logger := utils.GetLogger()

	snapshot, err := s.LoadRoomSnapshot(roomID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		logger.Warn("processParticipantLeave: 房间不存在", zap.String("room_id", roomID))
		return &LeaveResult{Action: LeaveActionSkipped}, nil
	}

	room := &snapshot.Room
	if IsRoomTerminal(room.Status) {
		logger.Info("processParticipantLeave: 房间已是终态",
			zap.String("room_id", roomID),
			zap.String("uid", uid),
			zap.Uint8("status", room.Status),
		)
		return &LeaveResult{Action: LeaveActionSkipped, Room: room, UIDs: snapshot.UIDs}, nil
	}

	current := FindParticipant(snapshot.Participants, uid)
	if current == nil {
		logger.Warn("processParticipantLeave: 参与者不存在",
			zap.String("room_id", roomID),
			zap.String("uid", uid),
		)
		return &LeaveResult{Action: LeaveActionSkipped, Room: room, UIDs: snapshot.UIDs}, nil
	}

	isOneToOne := room.MaxParticipants == 2
	isCreator := room.Creator == uid
	hasJoined := current.Status == models.ParticipantStatusJoined || current.LeaveTime > 0
	hasMissedOther := HasMissedParticipant(snapshot.Participants, uid)
	hasOtherJoined := HasOtherJoinedParticipant(snapshot.Participants, uid)

	now := time.Now().Unix()
	result := &LeaveResult{Room: room, UIDs: snapshot.UIDs}

	// ===== 1v1 场景 =====
	if isOneToOne {
		if isCreator {
			if hasMissedOther {
				// 对方已超时，创建者挂断 → 正常挂断，保留超时状态
				result.Action = LeaveActionHangup
				s.executeHangup(room, uid, now)
				return result, nil
			}
			if !hasOtherJoined {
				// 对方从未加入过 → 取消通话
				result.Action = LeaveActionCancel
				s.executeCancel(room, uid)
				return result, nil
			}
		} else if !hasJoined {
			// 非创建者未加入就离开 → 拒绝通话
			result.Action = LeaveActionReject
			s.executeReject(room, uid)
			return result, nil
		}
		// 默认：正常挂断（双方都曾加入）
		result.Action = LeaveActionHangup
		s.executeHangup(room, uid, now)
		return result, nil
	}

	// ===== 多人场景 =====
	if !hasJoined {
		// 未加入就离开 → 拒绝
		result.Action = LeaveActionReject
		s.executeReject(room, uid)
		return result, nil
	}
	if isCreator && !hasOtherJoined {
		// 创建者离开且无其他人曾加入 → 取消通话
		result.Action = LeaveActionCancel
		s.executeCancelAll(room)
		return result, nil
	}
	// 已加入后离开 → 正常挂断
	result.Action = LeaveActionHangup
	s.executeHangup(room, uid, now)
	return result, nil
}

// ========== CAS 执行方法（由 processParticipantLeave 调用） ==========

// executeCancel 执行取消操作（1v1：创建者取消）
func (s *RoomLifecycleService) executeCancel(room *models.Room, uid string) {
	// 更新所有参与者为已取消
	s.UpdateAllParticipants(room.RoomID,
		[]uint8{models.ParticipantStatusInviting, models.ParticipantStatusJoined},
		models.ParticipantStatusCancelled, nil)

	// CAS 更新房间状态为已取消
	s.CASUpdateRoom(room.RoomID,
		[]uint8{models.RoomStatusNotStarted, models.RoomStatusInProgress},
		models.RoomStatusCancelled)

	room.Status = models.RoomStatusCancelled
}

// executeCancelAll 执行多人通话取消（创建者离开，无人加入）
func (s *RoomLifecycleService) executeCancelAll(room *models.Room) {
	// 更新所有参与者为已取消
	s.UpdateAllParticipants(room.RoomID,
		[]uint8{models.ParticipantStatusInviting, models.ParticipantStatusJoined, models.ParticipantStatusHangup},
		models.ParticipantStatusCancelled, nil)

	// CAS 更新房间状态为已取消
	s.CASUpdateRoom(room.RoomID,
		[]uint8{models.RoomStatusNotStarted, models.RoomStatusInProgress},
		models.RoomStatusCancelled)

	room.Status = models.RoomStatusCancelled
}

// executeReject 执行拒绝操作
func (s *RoomLifecycleService) executeReject(room *models.Room, uid string) {
	// CAS 更新当前参与者为已拒绝
	s.CASUpdateParticipant(room.RoomID, uid,
		[]uint8{models.ParticipantStatusInviting},
		models.ParticipantStatusRejected, nil)

	// 1v1 场景：更新房间和对方状态
	if room.MaxParticipants == 2 {
		s.UpdateOtherParticipants(room.RoomID, uid,
			[]uint8{models.ParticipantStatusInviting, models.ParticipantStatusJoined},
			models.ParticipantStatusRejected, nil)

		s.CASUpdateRoom(room.RoomID,
			[]uint8{models.RoomStatusNotStarted, models.RoomStatusInProgress},
			models.RoomStatusRejected)

		room.Status = models.RoomStatusRejected
	}
}

// executeHangup 执行正常挂断操作
func (s *RoomLifecycleService) executeHangup(room *models.Room, uid string, now int64) {
	// CAS 更新当前参与者为已挂断
	s.CASUpdateParticipant(room.RoomID, uid,
		[]uint8{models.ParticipantStatusJoined},
		models.ParticipantStatusHangup,
		map[string]interface{}{"leave_time": now})

	// 1v1 场景：更新房间和对方状态
	if room.MaxParticipants == 2 {
		s.UpdateOtherParticipants(room.RoomID, uid,
			[]uint8{models.ParticipantStatusJoined},
			models.ParticipantStatusHangup,
			map[string]interface{}{"leave_time": now})

		s.CASUpdateRoom(room.RoomID,
			[]uint8{models.RoomStatusNotStarted, models.RoomStatusInProgress},
			models.RoomStatusFinished)

		room.Status = models.RoomStatusFinished
	}
}

// ========== 三个入口方法 ==========

// HandleUserLeave 处理用户主动离开（LeaveRoom API 调用）
// 调用共享方法 processParticipantLeave 判定动作并执行 CAS 更新，
// 然后根据动作发送对应的业务 webhook（cancelled/rejected）
func (s *RoomLifecycleService) HandleUserLeave(roomID, uid string) error {
	result, err := s.processParticipantLeave(roomID, uid)
	if err != nil {
		return err
	}
	if result.Action == LeaveActionSkipped {
		return nil
	}

	if s.businessWebhookService != nil {
		switch result.Action {
		case LeaveActionCancel:
			s.businessWebhookService.sendParticipantCancelled(result.Room, result.UIDs)
		case LeaveActionReject:
			s.businessWebhookService.sendParticipantRejected(result.Room, uid, result.UIDs)
		}
		s.ResolveAndFinishRoom(result.Room)
	}
	return nil
}

// HandleMediaDisconnect 处理媒体层断开（LiveKit webhook participant_left 调用）
// 调用共享方法 processParticipantLeave 判定动作并执行 CAS 更新，
// 始终发送 participant.left webhook（媒体断开是事实），
// 如果动作是 Cancel 则额外发送 participant.cancelled
func (s *RoomLifecycleService) HandleMediaDisconnect(roomID, uid string) error {
	result, err := s.processParticipantLeave(roomID, uid)
	if err != nil {
		return err
	}

	if s.businessWebhookService != nil {
		// 房间已终态时不发 participant.left（保持与原行为一致）
		if result.Action != LeaveActionSkipped {
			s.businessWebhookService.sendParticipantLeft(result.Room, uid, result.UIDs)
		}

		// 如果是取消动作（多人通话创建者离开无人加入），额外发送 cancelled
		if result.Action == LeaveActionCancel {
			s.businessWebhookService.sendParticipantCancelled(result.Room, result.UIDs)
		}

		if result.Action != LeaveActionSkipped {
			s.ResolveAndFinishRoom(result.Room)
		}
	}
	return nil
}

// HandleRoomClosed 处理 LiveKit 房间关闭事件（room_finished webhook）
func (s *RoomLifecycleService) HandleRoomClosed(roomID string) error {
	logger := utils.GetLogger()

	var room models.Room
	if err := s.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warn("HandleRoomClosed: 房间不存在", zap.String("room_id", roomID))
			return nil
		}
		return err
	}

	// 房间已终态，不覆盖
	if IsRoomTerminal(room.Status) {
		return nil
	}

	// CAS 更新房间状态为已结束
	updated := s.CASUpdateRoom(roomID,
		[]uint8{models.RoomStatusNotStarted, models.RoomStatusInProgress},
		models.RoomStatusFinished)

	if updated {
		room.Status = models.RoomStatusFinished
	}

	// 将仍在 Inviting/Joined 状态的参与者标记为 Hangup
	s.UpdateAllParticipants(roomID,
		[]uint8{models.ParticipantStatusInviting, models.ParticipantStatusJoined},
		models.ParticipantStatusHangup, nil)

	s.ResolveAndFinishRoom(&room)
	return nil
}

// HandleTimeout 处理参与者超时（SchedulerService 调用）
// CAS 更新参与者为 Missed，根据房间类型处理级联状态变更，
// 发送 participant.missed webhook
func (s *RoomLifecycleService) HandleTimeout(roomID, uid string) error {
	logger := utils.GetLogger()

	// CAS 更新：仅当参与者仍在 Inviting 状态时标记为 Missed
	updated := s.CASUpdateParticipant(roomID, uid,
		[]uint8{models.ParticipantStatusInviting},
		models.ParticipantStatusMissed, nil)

	if !updated {
		return nil
	}

	var room models.Room
	if err := s.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		logger.Error("HandleTimeout: 查询房间失败",
			zap.String("room_id", roomID),
			zap.Error(err),
		)
		return err
	}

	if IsRoomTerminal(room.Status) {
		return nil
	}

	if room.MaxParticipants == 2 {
		// 1v1：一方超时，整个通话结束
		s.CASUpdateRoom(roomID,
			[]uint8{models.RoomStatusNotStarted, models.RoomStatusInProgress},
			models.RoomStatusMissed)

		s.UpdateOtherParticipants(roomID, uid,
			[]uint8{models.ParticipantStatusInviting},
			models.ParticipantStatusMissed, nil)

		// 将已加入的参与者（创建者）标记为挂断（不设置 leave_time，与旧代码保持一致）
		s.UpdateAllParticipants(roomID,
			[]uint8{models.ParticipantStatusJoined},
			models.ParticipantStatusHangup, nil)

		room.Status = models.RoomStatusMissed
	} else {
		// 多人：仅当无人在通话时标记房间为 Missed
		var joinedCount int64
		s.db.Model(&models.Participant{}).
			Where("room_id = ? AND status = ?", roomID, models.ParticipantStatusJoined).
			Count(&joinedCount)

		if joinedCount == 0 {
			s.CASUpdateRoom(roomID,
				[]uint8{models.RoomStatusNotStarted, models.RoomStatusInProgress},
				models.RoomStatusMissed)
			room.Status = models.RoomStatusMissed
		}
	}

	// 重新查询以获取最新状态
	s.db.Where("room_id = ?", roomID).First(&room)

	var allUIDs []string
	s.db.Model(&models.Participant{}).
		Where("room_id = ?", roomID).
		Pluck("uid", &allUIDs)

	if s.businessWebhookService != nil {
		if room.MaxParticipants == 2 {
			s.businessWebhookService.sendParticipantMissed(&room, allUIDs)
		} else {
			s.businessWebhookService.sendParticipantMissed(&room, []string{uid})
		}
		s.ResolveAndFinishRoom(&room)
	}

	return nil
}
```

- [ ] **Step 2: 验证文件编译通过**

Run: `cd /Users/songlun/Desktop/workspace/Go/TgoRTCServer && go build ./internal/service/`
Expected: 编译成功，无错误

- [ ] **Step 3: Commit**

```bash
git add internal/service/room_lifecycle_service.go
git commit -m "feat: 新建 RoomLifecycleService 统一房间生命周期状态管理"
```

---

### Task 2: 注入 RoomLifecycleService 到初始化链（单实例）

**Files:**
- Modify: `internal/service/participant_service.go:17-38`
- Modify: `internal/service/webhook_service.go:18-38`
- Modify: `internal/service/scheduler_service.go:16-49`
- Modify: `internal/router/router.go:19-45`（修改函数签名，接收 lifecycleService 参数）
- Modify: `main.go:46-58`（创建唯一实例）

- [ ] **Step 1: 给 ParticipantService 添加 lifecycleService 字段和 setter**

在 `internal/service/participant_service.go` 中：

结构体添加字段：
```go
type ParticipantService struct {
	db                     *gorm.DB
	tokenGenerator         *livekit.TokenGenerator
	timeFormatter          *utils.TimeFormatter
	businessWebhookService *BusinessWebhookService
	schedulerService       *SchedulerService
	lifecycleService       *RoomLifecycleService // 新增
}
```

添加 setter 方法（在 `SetSchedulerService` 之后）：
```go
// SetLifecycleService 设置房间生命周期服务
func (ps *ParticipantService) SetLifecycleService(ls *RoomLifecycleService) {
	ps.lifecycleService = ls
}
```

- [ ] **Step 2: 给 WebhookService 添加 lifecycleService 字段和 setter**

在 `internal/service/webhook_service.go` 中：

结构体添加字段：
```go
type WebhookService struct {
	db                     *gorm.DB
	redisClient            *redis.Client
	config                 *config.Config
	businessWebhookService *BusinessWebhookService
	lifecycleService       *RoomLifecycleService // 新增
}
```

添加 setter 方法（在 `SetBusinessWebhookService` 之后）：
```go
// SetLifecycleService 设置房间生命周期服务
func (ws *WebhookService) SetLifecycleService(ls *RoomLifecycleService) {
	ws.lifecycleService = ls
}
```

- [ ] **Step 3: 给 SchedulerService 添加 lifecycleService 字段和 setter**

在 `internal/service/scheduler_service.go` 中，结构体添加字段（在 `participantDeduplicator` 之后）：
```go
lifecycleService *RoomLifecycleService // 新增
```

添加 setter 方法（在 `SetParticipantService` 之后）：
```go
// SetLifecycleService 设置房间生命周期服务
func (ss *SchedulerService) SetLifecycleService(ls *RoomLifecycleService) {
	ss.lifecycleService = ls
}
```

- [ ] **Step 4: 修改 router.go 函数签名，接收 lifecycleService 参数**

将 `internal/router/router.go` 的 `SetupRouter` 函数签名改为：

```go
func SetupRouter(db *gorm.DB, redisClient *redis.Client, cfg *config.Config, businessWebhookService *service.BusinessWebhookService, lifecycleService *service.RoomLifecycleService) (*gin.Engine, *service.ParticipantService, *service.RoomService) {
```

在 `webhookService.SetBusinessWebhookService(businessWebhookService)` 之后（约第 45 行后）添加：

```go
// 注入房间生命周期服务
participantService.SetLifecycleService(lifecycleService)
webhookService.SetLifecycleService(lifecycleService)
```

- [ ] **Step 5: 在 main.go 中创建唯一 lifecycleService 实例，传入 router 和 scheduler**

在 `main.go` 中，在 `businessWebhookService := ...` 之后（第 46 行后）添加：

```go
// 创建房间生命周期服务（唯一实例）
lifecycleService := service.NewRoomLifecycleService(db, businessWebhookService)
```

修改 `SetupRouter` 调用（第 49 行）为：

```go
r, participantService, roomService := router.SetupRouter(db, redisClient, cfg, businessWebhookService, lifecycleService)
```

在 `scheduler.SetParticipantService(participantService)` 之后添加：

```go
scheduler.SetLifecycleService(lifecycleService)
```

- [ ] **Step 6: 验证编译通过**

Run: `cd /Users/songlun/Desktop/workspace/Go/TgoRTCServer && go build ./...`
Expected: 编译成功

- [ ] **Step 7: Commit**

```bash
git add internal/service/participant_service.go internal/service/webhook_service.go internal/service/scheduler_service.go internal/router/router.go main.go
git commit -m "feat: 将 RoomLifecycleService 单实例注入到各 service 初始化链"
```

---

### Task 3: 重写 ParticipantService.LeaveRoom 委托给 lifecycle

**Files:**
- Modify: `internal/service/participant_service.go:139-398`

- [ ] **Step 1: 删除旧的 LeaveRoom 及其三个 handle 方法**

删除 `participant_service.go` 中的以下函数（第 139-398 行）：
- `LeaveRoom` (139-246)
- `handleCreatorCancelCall` (248-282)
- `handleParticipantReject` (284-340)
- `handleNormalHangup` (342-398)

- [ ] **Step 2: 用委托方式重写 LeaveRoom**

替换为（保留已有的 `errors` 和 `i18n` import）：

```go
// LeaveRoom 参与者离开房间
func (ps *ParticipantService) LeaveRoom(req *models.LeaveRoomRequest) error {
	if ps.lifecycleService == nil {
		return errors.NewBusinessErrorWithKey(i18n.RoomQueryFailed, "lifecycle service not initialized")
	}
	return ps.lifecycleService.HandleUserLeave(req.RoomID, req.UID)
}
```

- [ ] **Step 3: 验证编译通过**

Run: `cd /Users/songlun/Desktop/workspace/Go/TgoRTCServer && go build ./...`
Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add internal/service/participant_service.go
git commit -m "refactor: LeaveRoom 委托给 RoomLifecycleService.HandleUserLeave"
```

---

### Task 4: 重写 WebhookService 委托给 lifecycle

**Files:**
- Modify: `internal/service/webhook_service.go:144-453`

- [ ] **Step 1: 重写 handleRoomFinished**

将 `webhook_service.go` 中的 `handleRoomFinished` 函数（第 144-202 行）替换为：

```go
// handleRoomFinished 处理房间结束事件
func (ws *WebhookService) handleRoomFinished(event *models.WebhookEvent) error {
	if event.Room == nil {
		return nil
	}
	if ws.lifecycleService != nil {
		return ws.lifecycleService.HandleRoomClosed(event.Room.Name)
	}
	return nil
}
```

- [ ] **Step 2: 重写 handleParticipantLeft**

将 `webhook_service.go` 中的 `handleParticipantLeft` 函数（第 290-453 行）替换为：

```go
// handleParticipantLeft 处理参与者离开事件
func (ws *WebhookService) handleParticipantLeft(event *models.WebhookEvent) error {
	if event.Room == nil || event.Participant == nil {
		return nil
	}
	if ws.lifecycleService != nil {
		return ws.lifecycleService.HandleMediaDisconnect(event.Room.Name, event.Participant.Identity)
	}
	return nil
}
```

- [ ] **Step 3: 验证编译通过，清理未使用的 import**

Run: `cd /Users/songlun/Desktop/workspace/Go/TgoRTCServer && go build ./...`

如果出现 unused import 错误（`"fmt"`, `"time"` 等），检查：
- `"fmt"` 仍被 `HandleWebhookEvent` 第 49 行使用 → 保留
- `"time"` 仍被 `handleParticipantJoined` 和 `HandleWebhookEvent` 使用 → 保留
- `"tgo-rtc-server/internal/config"` 仍被 `WebhookService` 结构体使用 → 保留

- [ ] **Step 4: Commit**

```bash
git add internal/service/webhook_service.go
git commit -m "refactor: webhook handleParticipantLeft/handleRoomFinished 委托给 RoomLifecycleService"
```

---

### Task 5: 重写 SchedulerService 超时处理委托给 lifecycle

**Files:**
- Modify: `internal/service/scheduler_service.go:130-397`
- Modify: `main.go:53-54`

- [ ] **Step 1: 重写 checkSingleParticipantTimeout**

将 `scheduler_service.go` 中的 `checkSingleParticipantTimeout`（第 130-254 行）替换为：

```go
// checkSingleParticipantTimeout 检查单个参与者是否超时（精确定时器触发）
func (ss *SchedulerService) checkSingleParticipantTimeout(roomID, uid string) {
	key := roomID + ":" + uid

	// 从定时器 map 中移除
	ss.timersMu.Lock()
	delete(ss.timers, key)
	ss.timersMu.Unlock()

	if ss.lifecycleService != nil {
		ss.lifecycleService.HandleTimeout(roomID, uid)
	}
}
```

- [ ] **Step 2: 重写 checkParticipantTimeout**

将 `scheduler_service.go` 中的 `checkParticipantTimeout`（第 256-397 行）替换为：

```go
// checkParticipantTimeout 检查超时的参与者（定期轮询兜底）
// 注意：这是兜底机制，主要超时检测由精确定时器完成。
// 此处逐个调用 HandleTimeout 会比旧的批量处理多一些 DB 查询，
// 但执行频率低，可接受。
func (ss *SchedulerService) checkParticipantTimeout() {
	logger := utils.GetLogger()

	var participants []models.Participant
	if err := ss.db.Where("status = ?", models.ParticipantStatusInviting).Find(&participants).Error; err != nil {
		logger.Error("查询邀请中的参与者失败", zap.Error(err))
		return
	}

	if len(participants) == 0 {
		return
	}

	for _, p := range participants {
		timeDiff := time.Now().Unix() - p.CreatedAt.Unix()
		if timeDiff >= int64(ss.config.LiveKitTimeout) {
			if ss.lifecycleService != nil {
				ss.lifecycleService.HandleTimeout(p.RoomID, p.UID)
			}
		}
	}
}
```

- [ ] **Step 3: 清理不再需要的字段和方法**

在 `SchedulerService` 结构体中移除以下字段：
- `businessWebhookService *BusinessWebhookService`
- `participantService *ParticipantService`
- `participantDeduplicator *utils.ParticipantDeduplicator`

结构体变为：
```go
type SchedulerService struct {
	db     *gorm.DB
	config *config.Config
	ticker *time.Ticker
	done   chan bool

	lifecycleService *RoomLifecycleService

	// 精确定时器相关
	timersMu sync.RWMutex
	timers   map[string]*time.Timer // key: "roomID:uid"
}
```

简化 `NewSchedulerService`：
```go
func NewSchedulerService(db *gorm.DB, cfg *config.Config) *SchedulerService {
	return &SchedulerService{
		db:     db,
		config: cfg,
		done:   make(chan bool),
		timers: make(map[string]*time.Timer),
	}
}
```

删除 `SetBusinessWebhookService` 和 `SetParticipantService` 方法。

- [ ] **Step 4: 更新 main.go 移除旧的 setter 调用**

在 `main.go` 中删除这两行（约第 53-54 行）：
```go
scheduler.SetBusinessWebhookService(businessWebhookService)
scheduler.SetParticipantService(participantService)
```

- [ ] **Step 5: 验证编译通过**

Run: `cd /Users/songlun/Desktop/workspace/Go/TgoRTCServer && go build ./...`
Expected: 编译成功

- [ ] **Step 6: Commit**

```bash
git add internal/service/scheduler_service.go main.go
git commit -m "refactor: SchedulerService 超时处理委托给 RoomLifecycleService"
```

---

### Task 6: 清理残留依赖和编译验证

**Files:**
- Modify: 任何有编译问题的文件

- [ ] **Step 1: 完整编译验证**

Run: `cd /Users/songlun/Desktop/workspace/Go/TgoRTCServer && go build ./...`
Expected: 编译成功

- [ ] **Step 2: go vet 检查**

Run: `cd /Users/songlun/Desktop/workspace/Go/TgoRTCServer && go vet ./...`
Expected: 无警告

- [ ] **Step 3: 修复所有编译和 vet 问题**

常见问题清单：
- `scheduler_service.go` 移除了 `participantDeduplicator` → 检查 import 中 `"tgo-rtc-server/internal/utils"` 是否仍需要（`utils.GetLogger()` 需要 → 保留）
- `participant_service.go` 删除了大量代码 → 检查 `"go.uber.org/zap"` 是否仍需要（其他方法可能不用 → 如不需要则移除）
- `main.go` 中 `businessWebhookService` 仍被 `SetupRouter` 使用 → 保留

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "chore: 清理重构后的残留依赖和未使用代码"
```

---

### Task 7: 功能验证

- [ ] **Step 1: 启动服务验证无崩溃**

Run: `cd /Users/songlun/Desktop/workspace/Go/TgoRTCServer && go run main.go`
Expected: 服务正常启动，日志中显示"参与者超时检查定时器已启动"

- [ ] **Step 2: 验证健康检查接口正常**

Run: `curl http://localhost:8080/health`
Expected: `{"status":"ok"}`

- [ ] **Step 3: 代码走读确认 webhook 准确性**

| 场景 | 入口 | 期望 webhook |
|------|------|-------------|
| 创建者取消（1v1） | LeaveRoom → HandleUserLeave → processParticipantLeave(Cancel) | `participant.cancelled` + `room.finished` |
| 参与者拒绝（1v1） | LeaveRoom → HandleUserLeave → processParticipantLeave(Reject) | `participant.rejected` + `room.finished` |
| 正常挂断（1v1） | LeaveRoom → HandleUserLeave → processParticipantLeave(Hangup) | `room.finished` |
| 创建者取消（多人） | LeaveRoom → HandleUserLeave → processParticipantLeave(Cancel) | `participant.cancelled` + `room.finished` |
| 超时未接听 | Timer → HandleTimeout | `participant.missed` + `room.finished` |
| LiveKit 断开（正常） | Webhook → HandleMediaDisconnect → processParticipantLeave(Hangup) | `participant.left` + `room.finished` |
| LiveKit 断开（创建者取消多人） | Webhook → HandleMediaDisconnect → processParticipantLeave(Cancel) | `participant.left` + `participant.cancelled` + `room.finished` |
| LiveKit 房间关闭 | Webhook → HandleRoomClosed | `room.finished` |

- [ ] **Step 4: 最终 Commit**

```bash
git add -A
git commit -m "refactor: 完成 RoomLifecycleService 重构，统一房间状态管理"
```
