package service

import (
	"encoding/json"

	"tgo-call-server/internal/models"
	"tgo-call-server/internal/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// WebhookService webhook 服务
type WebhookService struct {
	db                     *gorm.DB
	businessWebhookService *BusinessWebhookService
}

// NewWebhookService 创建 webhook 服务
func NewWebhookService(db *gorm.DB) *WebhookService {
	return &WebhookService{
		db: db,
	}
}

// SetBusinessWebhookService 设置业务 webhook 服务
func (ws *WebhookService) SetBusinessWebhookService(bws *BusinessWebhookService) {
	ws.businessWebhookService = bws
}

// HandleWebhookEvent 处理 webhook 事件
func (ws *WebhookService) HandleWebhookEvent(event *models.WebhookEvent) error {
	logger := utils.GetLogger()
	logger.Info("收到 webhook 事件",
		zap.String("event_type", event.Event),
		zap.String("event_id", event.ID),
	)

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
			CallType:        int(room.CallType),
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
	if err := ws.db.Model(&models.Room{}).
		Where("room_id = ?", event.Room.Name).
		Update("status", models.RoomStatusFinished).Error; err != nil {
		logger.Error("更新房间状态失败",
			zap.String("room_id", event.Room.Name),
			zap.Error(err),
		)
		return err
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

	// 可以在这里添加业务逻辑
	// - 更新参与者状态
	// - 发送通知
	// - 记录日志

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
