package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	businessWebhookService *BusinessWebhookService
}

// NewWebhookService 创建 webhook 服务
func NewWebhookService(db *gorm.DB, redisClient *redis.Client) *WebhookService {
	return &WebhookService{
		db:          db,
		redisClient: redisClient,
	}
}

// SetBusinessWebhookService 设置业务 webhook 服务
func (ws *WebhookService) SetBusinessWebhookService(bws *BusinessWebhookService) {
	ws.businessWebhookService = bws
}

// HandleWebhookEvent 处理 webhook 事件
// 支持分布式环境中的事件去重（使用 Redis）
func (ws *WebhookService) HandleWebhookEvent(event *models.WebhookEvent) error {
	logger := utils.GetLogger()
	logger.Info("收到 webhook 事件",
		zap.String("event_type", event.Event),
		zap.String("event_id", event.ID),
	)

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
			logger.Info("事件已处理过，跳过",
				zap.String("event_type", event.Event),
				zap.String("event_id", event.ID),
			)
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
	logger := utils.GetLogger()

	if event.Room == nil {
		return nil
	}

	logger.Info("房间已开始",
		zap.String("room_name", event.Room.Name),
		zap.String("room_sid", event.Room.SID),
	)

	// 1、查询房间是否存在，如果存在则更新状态为进行中
	var room models.Room
	if err := ws.db.Where("room_id = ?", event.Room.Name).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warn("房间不存在",
				zap.String("room_id", event.Room.Name),
			)
			return nil
		}
		logger.Error("查询房间失败",
			zap.String("room_id", event.Room.Name),
			zap.Error(err),
		)
		return err
	}

	// 更新房间状态为进行中
	if err := ws.db.Model(&room).Update("status", models.RoomStatusInProgress).Error; err != nil {
		logger.Error("更新房间状态失败",
			zap.String("room_id", event.Room.Name),
			zap.Error(err),
		)
		return err
	}

	logger.Info("房间状态已更新为进行中",
		zap.String("room_id", event.Room.Name),
	)

	// 2、通知业务的webhook
	if ws.businessWebhookService != nil {
		eventData := &models.RoomEventData{
			RoomID:          room.RoomID,
			Creator:         room.Creator,
			RTCType:         int(room.RTCType),
			InviteOn:        int(room.InviteOn),
			Status:          int(models.RoomStatusInProgress),
			MaxParticipants: room.MaxParticipants,
			CreatedAt:       room.CreatedAt.Unix(),
			UpdatedAt:       room.UpdatedAt.Unix(),
		}
		if err := ws.businessWebhookService.SendEvent(models.BusinessEventRoomStarted, eventData); err != nil {
			logger.Error("发送业务 webhook 事件失败",
				zap.String("room_id", room.RoomID),
				zap.String("event_type", models.BusinessEventRoomStarted),
				zap.Error(err),
			)
			// 不返回错误，因为房间状态已经更新成功
		}
	}

	return nil
}

// handleRoomFinished 处理房间结束事件
func (ws *WebhookService) handleRoomFinished(event *models.WebhookEvent) error {
	logger := utils.GetLogger()

	if event.Room == nil {
		return nil
	}

	logger.Info("房间已结束",
		zap.String("room_name", event.Room.Name),
		zap.String("room_sid", event.Room.SID),
	)

	// 更新房间状态为已结束
	var room models.Room
	if err := ws.db.Model(&models.Room{}).
		Where("room_id = ?", event.Room.Name).
		Update("status", models.RoomStatusFinished).Error; err != nil {
		logger.Error("更新房间状态失败",
			zap.String("room_id", event.Room.Name),
			zap.Error(err),
		)
		return err
	}

	// 查询房间信息（用于后续 webhook 通知）
	if err := ws.db.Where("room_id = ?", event.Room.Name).First(&room).Error; err != nil {
		logger.Error("查询房间信息失败",
			zap.String("room_id", event.Room.Name),
			zap.Error(err),
		)
		// 继续执行，不返回错误
	}

	// 1、查询该房间中的成员，如果 status=0/1 的成员全部改成 ParticipantStatusHangup
	if err := ws.db.Model(&models.Participant{}).
		Where("room_id = ? AND (status = ? OR status = ?)", event.Room.Name, models.ParticipantStatusInviting, models.ParticipantStatusJoined).
		Update("status", models.ParticipantStatusHangup).Error; err != nil {
		logger.Error("更新房间参与者状态失败",
			zap.String("room_id", event.Room.Name),
			zap.Error(err),
		)
		// 继续执行，不返回错误
	} else {
		logger.Info("房间参与者状态已更新为已挂断",
			zap.String("room_id", event.Room.Name),
		)
	}

	// 2、通知业务的 webhook
	if ws.businessWebhookService != nil {
		eventData := &models.RoomEventData{
			RoomID:          room.RoomID,
			Creator:         room.Creator,
			RTCType:         int(room.RTCType),
			InviteOn:        int(room.InviteOn),
			Status:          int(models.RoomStatusFinished),
			MaxParticipants: room.MaxParticipants,
			CreatedAt:       room.CreatedAt.Unix(),
			UpdatedAt:       room.UpdatedAt.Unix(),
		}
		if err := ws.businessWebhookService.SendEvent(models.BusinessEventRoomFinished, eventData); err != nil {
			logger.Error("发送业务 webhook 事件失败",
				zap.String("room_id", room.RoomID),
				zap.String("event_type", models.BusinessEventRoomFinished),
				zap.Error(err),
			)
			// 不返回错误，因为房间状态已经更新成功
		}
	}

	return nil
}

// handleParticipantJoined 处理参与者加入事件
func (ws *WebhookService) handleParticipantJoined(event *models.WebhookEvent) error {
	logger := utils.GetLogger()

	if event.Room == nil || event.Participant == nil {
		return nil
	}

	logger.Info("参与者已加入",
		zap.String("participant_name", event.Participant.Name),
		zap.String("participant_identity", event.Participant.Identity),
		zap.String("room_name", event.Room.Name),
	)

	// 1、判断参与者是否在 rtc_participant 表存在
	var participant models.Participant
	if err := ws.db.Where("room_id = ? AND uid = ?", event.Room.Name, event.Participant.Identity).First(&participant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 参与者不存在，插入一条新记录
			participant = models.Participant{
				RoomID:   event.Room.Name,
				UID:      event.Participant.Identity,
				Status:   models.ParticipantStatusJoined,
				JoinTime: time.Now().Unix(),
			}
			if err := ws.db.Create(&participant).Error; err != nil {
				logger.Error("创建参与者记录失败",
					zap.String("room_id", event.Room.Name),
					zap.String("uid", event.Participant.Identity),
					zap.Error(err),
				)
				return err
			}
			logger.Info("参与者记录已创建",
				zap.String("room_id", event.Room.Name),
				zap.String("uid", event.Participant.Identity),
			)
		} else {
			logger.Error("查询参与者记录失败",
				zap.String("room_id", event.Room.Name),
				zap.String("uid", event.Participant.Identity),
				zap.Error(err),
			)
			return err
		}
	} else {
		// 参与者已存在，更新状态为已加入
		if err := ws.db.Model(&participant).Updates(map[string]interface{}{
			"status":    models.ParticipantStatusJoined,
			"join_time": time.Now().Unix(),
		}).Error; err != nil {
			logger.Error("更新参与者状态失败",
				zap.String("room_id", event.Room.Name),
				zap.String("uid", event.Participant.Identity),
				zap.Error(err),
			)
			return err
		}
		logger.Info("参与者状态已更新为已加入",
			zap.String("room_id", event.Room.Name),
			zap.String("uid", event.Participant.Identity),
		)
	}

	// 2、通知业务的 webhook
	if ws.businessWebhookService != nil {
		eventData := &models.ParticipantEventData{
			RoomID:    participant.RoomID,
			UID:       participant.UID,
			Status:    int(participant.Status),
			JoinTime:  participant.JoinTime,
			LeaveTime: participant.LeaveTime,
			CreatedAt: participant.CreatedAt.Unix(),
			UpdatedAt: participant.UpdatedAt.Unix(),
		}
		if err := ws.businessWebhookService.SendEvent(models.BusinessEventParticipantJoined, eventData); err != nil {
			logger.Error("发送业务 webhook 事件失败",
				zap.String("room_id", participant.RoomID),
				zap.String("uid", participant.UID),
				zap.String("event_type", models.BusinessEventParticipantJoined),
				zap.Error(err),
			)
			// 不返回错误，因为参与者状态已经更新成功
		}
	}

	return nil
}

// handleParticipantLeft 处理参与者离开事件
func (ws *WebhookService) handleParticipantLeft(event *models.WebhookEvent) error {
	logger := utils.GetLogger()

	if event.Room == nil || event.Participant == nil {
		return nil
	}

	logger.Info("参与者已离开",
		zap.String("participant_name", event.Participant.Name),
		zap.String("participant_identity", event.Participant.Identity),
		zap.String("room_name", event.Room.Name),
	)

	// 更新参与者状态为已挂断
	var leftParticipant models.Participant
	if err := ws.db.Model(&models.Participant{}).
		Where("uid = ? AND room_id = ?", event.Participant.Identity, event.Room.Name).
		Update("status", models.ParticipantStatusHangup).Error; err != nil {
		logger.Error("更新参与者状态失败",
			zap.String("participant_uid", event.Participant.Identity),
			zap.String("room_id", event.Room.Name),
			zap.Error(err),
		)
		return err
	}

	// 查询离开的参与者信息
	if err := ws.db.Where("uid = ? AND room_id = ?", event.Participant.Identity, event.Room.Name).First(&leftParticipant).Error; err != nil {
		logger.Error("查询参与者信息失败",
			zap.String("participant_uid", event.Participant.Identity),
			zap.String("room_id", event.Room.Name),
			zap.Error(err),
		)
	}

	// 1、查询房间信息
	var room models.Room
	if err := ws.db.Where("room_id = ?", event.Room.Name).First(&room).Error; err != nil {
		logger.Error("查询房间信息失败",
			zap.String("room_id", event.Room.Name),
			zap.Error(err),
		)
		return err
	}

	// 2、检查是否需要标记房间为已结束
	shouldFinishRoom := false

	// 情况1：房间的 max_participants=2，则标记房间已经结束
	if room.MaxParticipants == 2 {
		shouldFinishRoom = true
		logger.Info("房间为双人通话，标记房间已结束",
			zap.String("room_id", event.Room.Name),
			zap.Int("max_participants", room.MaxParticipants),
		)

		// 如果另外一个参与者 status=0（邀请中），将其改为 ParticipantStatusTimeout
		if err := ws.db.Model(&models.Participant{}).
			Where("room_id = ? AND uid != ? AND status = ?", event.Room.Name, event.Participant.Identity, models.ParticipantStatusInviting).
			Update("status", models.ParticipantStatusTimeout).Error; err != nil {
			logger.Error("更新其他参与者状态失败",
				zap.String("room_id", event.Room.Name),
				zap.Error(err),
			)
		} else {
			logger.Info("其他邀请中的参与者状态已更新为超时",
				zap.String("room_id", event.Room.Name),
			)
		}
	} else {
		// 情况2：检查房间中是否所有参与者都已离开（status=3）
		var activeParticipantCount int64
		if err := ws.db.Where("room_id = ? AND status IN (?, ?)", event.Room.Name, models.ParticipantStatusInviting, models.ParticipantStatusJoined).
			Count(&activeParticipantCount).Error; err != nil {
			logger.Error("查询活跃参与者数失败",
				zap.String("room_id", event.Room.Name),
				zap.Error(err),
			)
		} else if activeParticipantCount == 0 {
			shouldFinishRoom = true
			logger.Info("房间中所有参与者都已离开，标记房间已结束",
				zap.String("room_id", event.Room.Name),
			)
		}
	}

	// 如果需要标记房间为已结束，则更新房间状态
	if shouldFinishRoom && room.Status != models.RoomStatusFinished {
		if err := ws.db.Model(&room).Update("status", models.RoomStatusFinished).Error; err != nil {
			logger.Error("更新房间状态为已结束失败",
				zap.String("room_id", event.Room.Name),
				zap.Error(err),
			)
		} else {
			logger.Info("房间状态已更新为已结束",
				zap.String("room_id", event.Room.Name),
			)
		}
	}

	// 3、通知业务的 webhook
	if ws.businessWebhookService != nil {
		eventData := &models.ParticipantEventData{
			RoomID:    leftParticipant.RoomID,
			UID:       leftParticipant.UID,
			Status:    int(leftParticipant.Status),
			JoinTime:  leftParticipant.JoinTime,
			LeaveTime: leftParticipant.LeaveTime,
			CreatedAt: leftParticipant.CreatedAt.Unix(),
			UpdatedAt: leftParticipant.UpdatedAt.Unix(),
		}
		if err := ws.businessWebhookService.SendEvent(models.BusinessEventParticipantLeft, eventData); err != nil {
			logger.Error("发送业务 webhook 事件失败",
				zap.String("room_id", leftParticipant.RoomID),
				zap.String("uid", leftParticipant.UID),
				zap.String("event_type", models.BusinessEventParticipantLeft),
				zap.Error(err),
			)
			// 不返回错误，因为参与者状态已经更新成功
		}
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
