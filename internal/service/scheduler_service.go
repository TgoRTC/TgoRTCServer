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
	db     *gorm.DB
	config *config.Config
	ticker *time.Ticker
	done   chan bool

	lifecycleService *RoomLifecycleService

	// 精确定时器相关
	timersMu sync.RWMutex
	timers   map[string]*time.Timer // key: "roomID:uid"
}

// NewSchedulerService 创建定时器服务
func NewSchedulerService(db *gorm.DB, cfg *config.Config) *SchedulerService {
	return &SchedulerService{
		db:     db,
		config: cfg,
		done:   make(chan bool),
		timers: make(map[string]*time.Timer),
	}
}

// SetLifecycleService 设置房间生命周期服务
func (ss *SchedulerService) SetLifecycleService(ls *RoomLifecycleService) {
	ss.lifecycleService = ls
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
	key := roomID + ":" + uid

	// 从定时器 map 中移除
	ss.timersMu.Lock()
	delete(ss.timers, key)
	ss.timersMu.Unlock()

	if ss.lifecycleService != nil {
		if err := ss.lifecycleService.HandleTimeout(roomID, uid); err != nil {
			logger := utils.GetLogger()
			logger.Error("精确定时器超时处理失败",
				zap.String("room_id", roomID),
				zap.String("uid", uid),
				zap.Error(err),
			)
		}
	}
}

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
				if err := ss.lifecycleService.HandleTimeout(p.RoomID, p.UID); err != nil {
					logger.Error("兜底超时处理失败",
						zap.String("room_id", p.RoomID),
						zap.String("uid", p.UID),
						zap.Error(err),
					)
				}
			}
		}
	}
}
