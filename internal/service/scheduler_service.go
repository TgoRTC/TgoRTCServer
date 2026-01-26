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
	db                      *gorm.DB
	config                  *config.Config
	ticker                  *time.Ticker
	done                    chan bool
	businessWebhookService  *BusinessWebhookService
	participantService      *ParticipantService
	participantDeduplicator *utils.ParticipantDeduplicator
}

// NewSchedulerService 创建定时器服务
func NewSchedulerService(db *gorm.DB, cfg *config.Config) *SchedulerService {
	return &SchedulerService{
		db:                      db,
		config:                  cfg,
		done:                    make(chan bool),
		participantDeduplicator: utils.NewParticipantDeduplicator(),
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

	var missedParticipants []models.Participant
	for _, p := range participants {
		nowTime := time.Now().Unix()
		createdTime := p.CreatedAt.Unix()
		timeDiff := nowTime - createdTime
		if timeDiff >= int64(ss.config.LiveKitTimeout) {
			missedParticipants = append(missedParticipants, p)
			logger.Info("检查超时的参与者---->发现超时的参与者",
				zap.String("room_id", p.RoomID),
				zap.String("uid", p.UID),
			)
		}
	}
	if len(missedParticipants) == 0 {
		logger.Info("检查超时的参与者---->未发现超时的参与者")
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
		logger.Info("检查超时的参与者--->未找到相关房间")
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
		// 更新房间状态为超时未接听
		if err := ss.db.Model(&models.Room{}).
			Where("room_id = ?", roomId).
			Update("status", models.RoomStatusMissed).Error; err != nil {
			logger.Error("检查超时的参与者--->更新房间状态为超时未接听失败",
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
		// 发送参与者超时事件
		ss.businessWebhookService.sendParticipantMissed(&room, uids)
		// 发送房间完成事件
		ss.businessWebhookService.checkAndFinishRoom(&room)
	}
}
