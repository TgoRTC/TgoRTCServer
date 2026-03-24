package service

import (
	"time"

	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/utils"

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

// isRoomTerminal 判断房间是否已进入终态
func isRoomTerminal(status uint8) bool {
	return status > models.RoomStatusInProgress
}

// findParticipant 在参与者列表中查找指定 UID 的参与者
func findParticipant(participants []models.Participant, uid string) *models.Participant {
	for i := range participants {
		if participants[i].UID == uid {
			return &participants[i]
		}
	}
	return nil
}

// hasMissedParticipant 检查是否有超时参与者（排除指定 UID）
func hasMissedParticipant(participants []models.Participant, excludeUID string) bool {
	for _, p := range participants {
		if p.UID != excludeUID && p.Status == models.ParticipantStatusMissed {
			return true
		}
	}
	return false
}

// hasOtherJoinedParticipant 检查是否有其他参与者曾经加入过
func hasOtherJoinedParticipant(participants []models.Participant, excludeUID string) bool {
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
//
// 注意：快照读取和 CAS 更新之间存在时间窗口，并发入口可能基于过期快照做出判定。
// CAS 写入保证单个状态转换的原子性，但跨参与者/房间的决策可能基于过期数据。
// 这是 snapshot-then-CAS 模式的固有限制，通过 CAS 的幂等性和 webhook 去重来缓解。
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
	if isRoomTerminal(room.Status) {
		logger.Info("processParticipantLeave: 房间已是终态",
			zap.String("room_id", roomID),
			zap.String("uid", uid),
			zap.Uint8("status", room.Status),
		)
		return &LeaveResult{Action: LeaveActionSkipped, Room: room, UIDs: snapshot.UIDs}, nil
	}

	current := findParticipant(snapshot.Participants, uid)
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
	hasMissedOther := hasMissedParticipant(snapshot.Participants, uid)
	hasOtherJoined := hasOtherJoinedParticipant(snapshot.Participants, uid)

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
				s.executeCancel(room)
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
func (s *RoomLifecycleService) executeCancel(room *models.Room) {
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

// ========== 四个入口方法 ==========

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
	if isRoomTerminal(room.Status) {
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

	if isRoomTerminal(room.Status) {
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
		if err := s.db.Model(&models.Participant{}).
			Where("room_id = ? AND status = ?", roomID, models.ParticipantStatusJoined).
			Count(&joinedCount).Error; err != nil {
			logger.Error("HandleTimeout: 查询已加入参与者数量失败",
				zap.String("room_id", roomID),
				zap.Error(err),
			)
			return err
		}

		if joinedCount == 0 {
			s.CASUpdateRoom(roomID,
				[]uint8{models.RoomStatusNotStarted, models.RoomStatusInProgress},
				models.RoomStatusMissed)
			room.Status = models.RoomStatusMissed
		}
	}

	// 重新查询以获取最新状态
	if err := s.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		logger.Error("HandleTimeout: 重新查询房间失败",
			zap.String("room_id", roomID),
			zap.Error(err),
		)
		return err
	}

	var allUIDs []string
	if err := s.db.Model(&models.Participant{}).
		Where("room_id = ?", roomID).
		Pluck("uid", &allUIDs).Error; err != nil {
		logger.Error("HandleTimeout: 查询参与者 UID 失败",
			zap.String("room_id", roomID),
			zap.Error(err),
		)
		return err
	}

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
