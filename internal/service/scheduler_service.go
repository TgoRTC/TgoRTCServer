package service

import (
	"log"
	"time"

	"tgo-call-server/internal/config"
	"tgo-call-server/internal/models"

	"gorm.io/gorm"
)

// SchedulerService 定时器服务
type SchedulerService struct {
	db     *gorm.DB
	config *config.Config
	ticker *time.Ticker
	done   chan bool
}

// NewSchedulerService 创建定时器服务
func NewSchedulerService(db *gorm.DB, cfg *config.Config) *SchedulerService {
	return &SchedulerService{
		db:     db,
		config: cfg,
		done:   make(chan bool),
	}
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

	log.Printf("✅ 参与者超时检查定时器已启动，检查间隔: %d 秒", ss.config.ParticipantTimeoutCheckInterval)
}

// Stop 停止定时器
func (ss *SchedulerService) Stop() {
	if ss.ticker != nil {
		ss.ticker.Stop()
	}
	ss.done <- true
	log.Println("✅ 参与者超时检查定时器已停止")
}

// checkParticipantTimeout 检查超时的参与者
func (ss *SchedulerService) checkParticipantTimeout() {
	// 获取所有状态为 0（邀请中）的参与者
	var participants []models.Participant
	if err := ss.db.Where("status = ?", models.ParticipantStatusInviting).Find(&participants).Error; err != nil {
		log.Printf("❌ 查询邀请中的参与者失败: %v", err)
		return
	}

	if len(participants) == 0 {
		return
	}

	// 获取所有房间的超时配置
	var rooms []models.Room
	if err := ss.db.Find(&rooms).Error; err != nil {
		log.Printf("❌ 查询房间失败: %v", err)
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
		var ids []int
		for _, p := range timeoutParticipants {
			ids = append(ids, p.ID)
		}

		if err := ss.db.Model(&models.Participant{}).
			Where("id IN ?", ids).
			Update("status", models.ParticipantStatusTimeout).Error; err != nil {
			log.Printf("❌ 更新超时参与者状态失败: %v", err)
			return
		}

		log.Printf("✅ 检查完成: 发现 %d 个超时的参与者，已更新状态为超时", len(timeoutParticipants))
	}
}

