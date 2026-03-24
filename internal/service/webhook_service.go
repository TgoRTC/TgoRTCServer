package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"tgo-rtc-server/internal/config"
	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/utils"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// WebhookService webhook 服务
type WebhookService struct {
	db                     *gorm.DB
	redisClient            *redis.Client
	config                 *config.Config
	businessWebhookService *BusinessWebhookService
	lifecycleService       *RoomLifecycleService
}

// NewWebhookService 创建 webhook 服务
func NewWebhookService(db *gorm.DB, redisClient *redis.Client, cfg *config.Config) *WebhookService {
	return &WebhookService{
		db:          db,
		redisClient: redisClient,
		config:      cfg,
	}
}

// SetBusinessWebhookService 设置业务 webhook 服务
func (ws *WebhookService) SetBusinessWebhookService(bws *BusinessWebhookService) {
	ws.businessWebhookService = bws
}

// SetLifecycleService 设置房间生命周期服务
func (ws *WebhookService) SetLifecycleService(ls *RoomLifecycleService) {
	ws.lifecycleService = ls
}

// HandleWebhookEvent 处理 webhook 事件
// 支持分布式环境中的事件去重（使用 Redis）
func (ws *WebhookService) HandleWebhookEvent(event *models.WebhookEvent) error {
	logger := utils.GetLogger()

	// 使用 Redis 进行事件去重（防止分布式环境中的重复处理）
	if ws.redisClient != nil {
		// 生成事件 ID（用于去重）
		// 格式: webhook:{event_type}:{event_id}
		deduplicationKey := fmt.Sprintf("webhook:%s:%s", event.Event, event.ID)

		// 使用 Redis 检查是否已处理过
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		exists, err := ws.redisClient.Exists(ctx, deduplicationKey).Result()
		if err != nil {
			logger.Warn("Redis 查询失败，继续处理事件",
				zap.String("event_id", event.ID),
				zap.Error(err),
			)
		} else if exists > 0 {
			// 事件已处理过，直接返回
			return nil
		}

		// 处理事件后，标记为已处理（设置 1 小时过期）
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := ws.redisClient.Set(ctx, deduplicationKey, "1", time.Hour).Err(); err != nil {
				logger.Warn("Redis 设置失败，事件已处理但未标记",
					zap.String("event_id", event.ID),
					zap.Error(err),
				)
			}
		}()
	}

	switch event.Event {
	case models.WebhookEventRoomStarted:
		return ws.handleRoomStarted(event) // 房间开始
	case models.WebhookEventRoomFinished:
		return ws.handleRoomFinished(event) // 房间结束
	case models.WebhookEventParticipantJoined:
		return ws.handleParticipantJoined(event) // 参与者加入房间
	case models.WebhookEventParticipantLeft:
		return ws.handleParticipantLeft(event) // 参与者离开房间
	// case models.WebhookEventParticipantConnectionAborted:
	// 	return ws.handleParticipantConnectionAborted(event)
	// case models.WebhookEventTrackPublished:
	// 	return ws.handleTrackPublished(event)
	// case models.WebhookEventTrackUnpublished:
	// 	return ws.handleTrackUnpublished(event)
	default:
		logger.Warn("未知的 webhook 事件类型",
			zap.String("event_type", event.Event),
		)
		return nil
	}
}

// handleRoomStarted 处理房间开始事件
func (ws *WebhookService) handleRoomStarted(event *models.WebhookEvent) error {
	if event.Room == nil {
		return nil
	}
	logger := utils.GetLogger()

	// 1、查询房间是否存在，如果存在则更新状态为进行中
	var room models.Room
	if err := ws.db.Where("room_id = ?", event.Room.Name).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warn("livekit事件: 房间开始--->房间不存在",
				zap.String("room_id", event.Room.Name),
			)
			return nil
		}
		logger.Error("livekit事件: 房间开始--->查询房间失败",
			zap.String("room_id", event.Room.Name),
			zap.Error(err),
		)
		return err
	}

	// 更新房间状态为进行中
	if err := ws.db.Model(&room).Update("status", models.RoomStatusInProgress).Error; err != nil {
		logger.Error("livekit事件: 房间开始--->更新房间状态失败",
			zap.String("room_id", event.Room.Name),
			zap.Uint8("room_status", models.RoomStatusInProgress),
			zap.Error(err),
		)
		return err
	}
	room.Status = models.RoomStatusInProgress
	// 2、通知业务的webhook
	if ws.businessWebhookService != nil {
		ws.businessWebhookService.sendRoomStarted(&room)
	}

	return nil
}

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

// handleParticipantJoined 处理参与者加入事件
func (ws *WebhookService) handleParticipantJoined(event *models.WebhookEvent) error {
	logger := utils.GetLogger()

	if event.Room == nil || event.Participant == nil {
		return nil
	}

	// 解析参与者的 metadata 获取 device_type
	var deviceType string
	if event.Participant.Metadata != "" {
		var metadata struct {
			DeviceType string `json:"device_type"`
		}
		if err := json.Unmarshal([]byte(event.Participant.Metadata), &metadata); err != nil {
			logger.Warn("解析参与者 metadata 失败",
				zap.String("metadata", event.Participant.Metadata),
				zap.Error(err),
			)
		} else {
			deviceType = metadata.DeviceType
		}
	}

	// 1、判断参与者是否在 rtc_participant 表存在
	var participant models.Participant
	if err := ws.db.Where("room_id = ? AND uid = ?", event.Room.Name, event.Participant.Identity).First(&participant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 参与者不存在，插入一条新记录
			participant = models.Participant{
				RoomID:     event.Room.Name,
				UID:        event.Participant.Identity,
				DeviceType: deviceType,
				Status:     models.ParticipantStatusJoined,
				JoinTime:   time.Now().Unix(),
			}
			if err := ws.db.Create(&participant).Error; err != nil {
				logger.Error("创建参与者记录失败",
					zap.String("room_id", event.Room.Name),
					zap.String("uid", event.Participant.Identity),
					zap.Error(err),
				)
				return err
			}
		} else {
			logger.Error("查询参与者记录失败",
				zap.String("room_id", event.Room.Name),
				zap.String("uid", event.Participant.Identity),
				zap.Error(err),
			)
			return err
		}
	} else {
		// 参与者已存在，CAS 更新：仅从 Inviting 状态更新为 Joined（防止覆盖终态）
		result := ws.db.Model(&models.Participant{}).
			Where("room_id = ? AND uid = ? AND status = ?", event.Room.Name, event.Participant.Identity, models.ParticipantStatusInviting).
			Updates(map[string]interface{}{
				"status":      models.ParticipantStatusJoined,
				"join_time":   time.Now().Unix(),
				"device_type": deviceType,
			})
		if result.Error != nil {
			logger.Error("更新参与者状态失败",
				zap.String("room_id", event.Room.Name),
				zap.String("uid", event.Participant.Identity),
				zap.Error(result.Error),
			)
			return result.Error
		}
		if result.RowsAffected == 0 {
			logger.Info("参与者状态已非 Inviting，跳过 Joined 更新",
				zap.String("room_id", event.Room.Name),
				zap.String("uid", event.Participant.Identity),
				zap.Uint8("current_status", participant.Status),
			)
		}
	}

	// 2、通知业务的 webhook
	if ws.businessWebhookService != nil {
		// 查询房间信息
		var room models.Room
		if err := ws.db.Where("room_id = ?", participant.RoomID).First(&room).Error; err != nil {
			logger.Error("查询房间信息失败",
				zap.String("room_id", participant.RoomID),
				zap.Error(err),
			)
		} else {
			ws.businessWebhookService.sendParticipantJoined(&room, participant.UID, deviceType)
			// 构建事件数据
		}
	}

	return nil
}

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

// // handleParticipantConnectionAborted 处理参与者连接中止事件
// func (ws *WebhookService) handleParticipantConnectionAborted(event *models.WebhookEvent) error {
// 	if event.Room == nil || event.Participant == nil {
// 		return nil
// 	}

// 	log.Printf("⚠️  参与者连接已中止: %s (Identity: %s) 房间: %s",
// 		event.Participant.Name, event.Participant.Identity, event.Room.Name)

// 	return nil
// }

// // handleTrackPublished 处理轨道发布事件
// func (ws *WebhookService) handleTrackPublished(event *models.WebhookEvent) error {
// 	if event.Room == nil || event.Participant == nil || event.Track == nil {
// 		return nil
// 	}

// 	log.Printf("✅ 轨道已发布: %s (Type: %s) 参与者: %s 房间: %s",
// 		event.Track.Name, event.Track.Type, event.Participant.Identity, event.Room.Name)

// 	return nil
// }

// // handleTrackUnpublished 处理轨道取消发布事件
// func (ws *WebhookService) handleTrackUnpublished(event *models.WebhookEvent) error {
// 	if event.Room == nil || event.Participant == nil || event.Track == nil {
// 		return nil
// 	}

// 	log.Printf("✅ 轨道已取消发布: %s (Type: %s) 参与者: %s 房间: %s",
// 		event.Track.Name, event.Track.Type, event.Participant.Identity, event.Room.Name)

// 	return nil
// }

// ParseWebhookEvent 解析 webhook 事件
func ParseWebhookEvent(body []byte) (*models.WebhookEvent, error) {
	var event models.WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}
	return &event, nil
}
