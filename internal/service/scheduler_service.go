package service

import (
	"sync"
	"time"

	"tgo-rtc-server/internal/config"
	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SchedulerService 定时器服务
type SchedulerService struct {
	db                      *gorm.DB
	config                  *config.Config
	ticker                  *time.Ticker
	done                    chan bool
	businessWebhookService  *BusinessWebhookService
	participantService      *ParticipantService
	participantDeduplicator *utils.ParticipantDeduplicator

	// 精确定时器相关
	timersMu sync.RWMutex
	timers   map[string]*time.Timer // key: "roomID:uid"
}

// NewSchedulerService 创建定时器服务
func NewSchedulerService(db *gorm.DB, cfg *config.Config) *SchedulerService {
	return &SchedulerService{
		db:                      db,
		config:                  cfg,
		done:                    make(chan bool),
		participantDeduplicator: utils.NewParticipantDeduplicator(),
		timers:                  make(map[string]*time.Timer),
	}
}

// SetBusinessWebhookService 设置业务 webhook 服务
func (ss *SchedulerService) SetBusinessWebhookService(bws *BusinessWebhookService) {
	ss.businessWebhookService = bws
}

// SetParticipantService 设置参与者服务
func (ss *SchedulerService) SetParticipantService(ps *ParticipantService) {
	ss.participantService = ps
}

// Start 启动定时器
func (ss *SchedulerService) Start() {
	interval := time.Duration(ss.config.ParticipantTimeoutCheckInterval) * time.Second
	ss.ticker = time.NewTicker(interval)

	go func() {
		// 立即执行一次
		ss.checkParticipantTimeout()

		// 然后定期执行
		for {
			select {
			case <-ss.ticker.C:
				ss.checkParticipantTimeout()
			case <-ss.done:
				return
			}
		}
	}()

	logger := utils.GetLogger()
	logger.Info("参与者超时检查定时器已启动",
		zap.Int("interval_seconds", ss.config.ParticipantTimeoutCheckInterval),
	)
}

// Stop 停止定时器
func (ss *SchedulerService) Stop() {
	if ss.ticker != nil {
		ss.ticker.Stop()
	}
	ss.done <- true

	// 清理所有精确定时器
	ss.timersMu.Lock()
	for key, timer := range ss.timers {
		timer.Stop()
		delete(ss.timers, key)
	}
	ss.timersMu.Unlock()

	logger := utils.GetLogger()
	logger.Info("参与者超时检查定时器已停止")
}

// ScheduleParticipantTimeout 为参与者设置精确超时定时器
func (ss *SchedulerService) ScheduleParticipantTimeout(roomID, uid string) {
	key := roomID + ":" + uid
	timeout := time.Duration(ss.config.LiveKitTimeout) * time.Second

	ss.timersMu.Lock()
	defer ss.timersMu.Unlock()

	// 如果已存在定时器，先取消
	if existingTimer, exists := ss.timers[key]; exists {
		existingTimer.Stop()
		delete(ss.timers, key)
	}

	// 创建新的定时器
	timer := time.AfterFunc(timeout, func() {
		ss.checkSingleParticipantTimeout(roomID, uid)
	})
	ss.timers[key] = timer
}

// CancelParticipantTimeout 取消参与者的超时定时器
func (ss *SchedulerService) CancelParticipantTimeout(roomID, uid string) {
	key := roomID + ":" + uid

	ss.timersMu.Lock()
	defer ss.timersMu.Unlock()

	if timer, exists := ss.timers[key]; exists {
		timer.Stop()
		delete(ss.timers, key)
	}
}

// checkSingleParticipantTimeout 检查单个参与者是否超时（精确定时器触发）
func (ss *SchedulerService) checkSingleParticipantTimeout(roomID, uid string) {
	logger := utils.GetLogger()
	key := roomID + ":" + uid

	// 从定时器 map 中移除
	ss.timersMu.Lock()
	delete(ss.timers, key)
	ss.timersMu.Unlock()

	// 查询参与者当前状态
	var participant models.Participant
	if err := ss.db.Where("room_id = ? AND uid = ? AND status = ?",
		roomID, uid, models.ParticipantStatusInviting).First(&participant).Error; err != nil {
		// 参与者不存在或状态已改变，无需处理
		return
	}

	// 更新参与者状态为超时
	if err := ss.db.Model(&models.Participant{}).
		Where("room_id = ? AND uid = ?", roomID, uid).
		Update("status", models.ParticipantStatusMissed).Error; err != nil {
		logger.Error("更新参与者状态为超时失败",
			zap.String("room_id", roomID),
			zap.String("uid", uid),
			zap.Error(err),
		)
		return
	}

	// 查询房间信息
	var room models.Room
	if err := ss.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		logger.Error("查询房间失败",
			zap.String("room_id", roomID),
			zap.Error(err),
		)
		return
	}

	// 检查房间中是否还有已加入的参与者
	var joinedCount int64
	if err := ss.db.Model(&models.Participant{}).
		Where("room_id = ? AND status = ?", roomID, models.ParticipantStatusJoined).
		Count(&joinedCount).Error; err != nil {
		logger.Error("查询已加入的参与者数量失败",
			zap.String("room_id", roomID),
			zap.Error(err),
		)
		return
	}

	// 只有当房间中没有已加入的参与者时，才更新房间状态为超时未接听
	if joinedCount == 0 {
		if err := ss.db.Model(&models.Room{}).
			Where("room_id = ?", roomID).
			Update("status", models.RoomStatusMissed).Error; err != nil {
			logger.Error("更新房间状态为超时未接听失败",
				zap.String("room_id", roomID),
				zap.Error(err),
			)
			return
		}
	}

	// 发送参与者超时事件
	if ss.businessWebhookService != nil {
		ss.businessWebhookService.sendParticipantMissed(&room, []string{uid})
		// 只有房间状态变成 missed 时才检查是否需要发送房间完成事件
		if joinedCount == 0 {
			ss.businessWebhookService.checkAndFinishRoom(&room)
		}
	}
}

// checkParticipantTimeout 检查超时的参与者
func (ss *SchedulerService) checkParticipantTimeout() {
	// 获取所有状态为 0（邀请中）的参与者
	logger := utils.GetLogger()
	var participants []models.Participant
	if err := ss.db.Where("status = ?", models.ParticipantStatusInviting).Find(&participants).Error; err != nil {
		logger.Error("查询邀请中的参与者失败",
			zap.Error(err),
		)
		return
	}

	if len(participants) == 0 {
		return
	}

	var missedParticipants []models.Participant
	for _, p := range participants {
		nowTime := time.Now().Unix()
		createdTime := p.CreatedAt.Unix()
		timeDiff := nowTime - createdTime
		if timeDiff >= int64(ss.config.LiveKitTimeout) {
			missedParticipants = append(missedParticipants, p)
		}
	}
	if len(missedParticipants) == 0 {
		return
	}
	var roomIDs []string
	for _, p := range missedParticipants {
		roomIDs = append(roomIDs, p.RoomID)
	}
	reallyRoomIds := ss.participantDeduplicator.DeduplicateUIDs(roomIDs)

	var rooms []models.Room
	if err := ss.db.Where("room_id IN ?", reallyRoomIds).Find(&rooms).Error; err != nil {
		logger.Error("检查超时的参与者--->查询房间失败",
			zap.Error(err),
		)
		return
	}
	if len(rooms) == 0 {
		return
	}
	// 分组
	roomParticipantMap := make(map[string][]models.Participant)
	for _, p := range missedParticipants {
		roomParticipantMap[p.RoomID] = append(roomParticipantMap[p.RoomID], p)
	}
	for roomId, participants := range roomParticipantMap {
		uids := make([]string, 0, len(participants))
		for _, p := range participants {
			uids = append(uids, p.UID)
		}
		// 更新参与者状态为超时
		if err := ss.db.Model(&models.Participant{}).
			Where("room_id = ? AND uid IN ?", roomId, uids).
			Update("status", models.ParticipantStatusMissed).Error; err != nil {
			logger.Error("检查超时的参与者--->更新参与者状态为超时失败",
				zap.String("room_id", roomId),
				zap.Error(err),
			)
			continue
		}

		// 检查房间中是否还有已加入的参与者
		var joinedCount int64
		if err := ss.db.Model(&models.Participant{}).
			Where("room_id = ? AND status = ?", roomId, models.ParticipantStatusJoined).
			Count(&joinedCount).Error; err != nil {
			logger.Error("检查超时的参与者--->查询已加入的参与者数量失败",
				zap.String("room_id", roomId),
				zap.Error(err),
			)
			continue
		}

		var room models.Room
		for _, r := range rooms {
			if r.RoomID == roomId {
				room = r
				break
			}
		}

		// 只有当房间中没有已加入的参与者时，才更新房间状态为超时未接听
		if joinedCount == 0 {
			if err := ss.db.Model(&models.Room{}).
				Where("room_id = ?", roomId).
				Update("status", models.RoomStatusMissed).Error; err != nil {
				logger.Error("更新房间状态为超时未接听失败",
					zap.String("room_id", roomId),
					zap.Error(err),
				)
				continue
			}
		}

		// 发送参与者超时事件
		ss.businessWebhookService.sendParticipantMissed(&room, uids)
		// 只有房间状态变成 missed 时才检查是否需要发送房间完成事件
		if joinedCount == 0 {
			ss.businessWebhookService.checkAndFinishRoom(&room)
		}
	}
}
