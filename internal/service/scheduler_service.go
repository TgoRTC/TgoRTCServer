package service

import (
	"time"

	"tgo-rtc-server/internal/config"
	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SchedulerService 定时器服务
type SchedulerService struct {
	db                     *gorm.DB
	config                 *config.Config
	ticker                 *time.Ticker
	done                   chan bool
	businessWebhookService *BusinessWebhookService
	participantService     *ParticipantService
}

// NewSchedulerService 创建定时器服务
func NewSchedulerService(db *gorm.DB, cfg *config.Config) *SchedulerService {
	return &SchedulerService{
		db:     db,
		config: cfg,
		done:   make(chan bool),
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
	logger.Info("✅ 参与者超时检查定时器已启动",
		zap.Int("interval_seconds", ss.config.ParticipantTimeoutCheckInterval),
	)
}

// Stop 停止定时器
func (ss *SchedulerService) Stop() {
	if ss.ticker != nil {
		ss.ticker.Stop()
	}
	ss.done <- true
	logger := utils.GetLogger()
	logger.Info("✅ 参与者超时检查定时器已停止")
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

	// 获取所有房间的超时配置
	var rooms []models.Room
	if err := ss.db.Find(&rooms).Error; err != nil {
		logger.Error("查询房间失败",
			zap.Error(err),
		)
		return
	}

	// 构建房间 ID 到超时时间的映射
	roomTimeoutMap := make(map[string]int)
	for _, room := range rooms {
		// 从 LiveKit Token 生成器获取超时时间
		// 这里使用配置中的 LiveKitTimeout 作为默认超时时间
		roomTimeoutMap[room.RoomID] = ss.config.LiveKitTimeout
	}

	// 检查每个参与者是否超时
	now := time.Now()
	var timeoutParticipants []models.Participant

	for _, participant := range participants {
		timeout, exists := roomTimeoutMap[participant.RoomID]
		if !exists {
			// 如果房间不存在，使用默认超时时间
			timeout = ss.config.LiveKitTimeout
		}

		// 计算邀请时间到现在的时间差
		timeDiff := now.Sub(participant.CreatedAt).Seconds()

		// 如果超过超时时间，标记为超时
		if int(timeDiff) > timeout {
			timeoutParticipants = append(timeoutParticipants, participant)
		}
	}

	// 批量更新超时的参与者状态
	if len(timeoutParticipants) > 0 {
		var roomIDs []string
		for _, p := range timeoutParticipants {
			roomIDs = append(roomIDs, p.RoomID)
		}

		if err := ss.db.Model(&models.Participant{}).
			Where("room_id IN ?", roomIDs).
			Update("status", models.ParticipantStatusTimeout).Error; err != nil {
			logger.Error("更新超时参与者状态失败",
				zap.Error(err),
			)
			return
		}
		logger.Info("✅ 检查完成: 发现超时的参与者，已更新状态为超时",
			zap.Int("timeout_count", len(timeoutParticipants)),
		)

		// 批量查询涉及的所有房间
		var rooms []models.Room
		if err := ss.db.Where("room_id IN ?", roomIDs).Find(&rooms).Error; err != nil {
			logger.Error("批量查询房间失败",
				zap.Error(err),
			)
			return
		}

		// 创建 roomID 到 room 对象的映射
		roomMap := make(map[string]*models.Room)
		for i := range rooms {
			roomMap[rooms[i].RoomID] = &rooms[i]
		}

		// 为每个超时的参与者发送 participant.missed 事件
		if ss.businessWebhookService != nil {
			for _, p := range timeoutParticipants {
				room, exists := roomMap[p.RoomID]
				if !exists {
					logger.Error("房间不存在",
						zap.String("room_id", p.RoomID),
						zap.String("uid", p.UID),
					)
					continue
				}
				ss.businessWebhookService.sendParticipantMissed(room, p.UID)
			}
			for _, room := range rooms {
				ss.businessWebhookService.checkAndFinishRoom(&room)
			}
		}

	}
}

// checkAndFinishRoom 检查房间的所有参与者是否都已结束，如果是则将房间状态改为完成
// func (ss *SchedulerService) checkAndFinishRoom(roomID string) {
// 	logger := utils.GetLogger()

// 	// 查询房间信息
// 	var room models.Room
// 	if err := ss.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
// 		logger.Error("查询房间失败",
// 			zap.String("room_id", roomID),
// 			zap.Error(err),
// 		)
// 		return
// 	}

// 	// 调用 ParticipantService 的 checkAndFinishRoom 方法
// 	if ss.businessWebhookService != nil {
// 		ss.businessWebhookService.checkAndFinishRoom(&room)
// 	}
// }
