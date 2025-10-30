package service

import (
	"encoding/json"
	"log"

	"tgo-call-server/internal/models"

	"gorm.io/gorm"
)

// WebhookService webhook 服务
type WebhookService struct {
	db *gorm.DB
}

// NewWebhookService 创建 webhook 服务
func NewWebhookService(db *gorm.DB) *WebhookService {
	return &WebhookService{
		db: db,
	}
}

// HandleWebhookEvent 处理 webhook 事件
func (ws *WebhookService) HandleWebhookEvent(event *models.WebhookEvent) error {
	log.Printf("📨 收到 webhook 事件: %s (ID: %s)", event.Event, event.ID)

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
		log.Printf("⚠️  未知的 webhook 事件类型: %s", event.Event)
		return nil
	}
}

// handleRoomStarted 处理房间开始事件
func (ws *WebhookService) handleRoomStarted(event *models.WebhookEvent) error {
	if event.Room == nil {
		return nil
	}

	log.Printf("✅ 房间已开始: %s (SID: %s)", event.Room.Name, event.Room.SID)

	// 可以在这里添加业务逻辑，例如：
	// - 更新房间状态
	// - 发送通知
	// - 记录日志

	return nil
}

// handleRoomFinished 处理房间结束事件
func (ws *WebhookService) handleRoomFinished(event *models.WebhookEvent) error {
	if event.Room == nil {
		return nil
	}

	log.Printf("✅ 房间已结束: %s (SID: %s)", event.Room.Name, event.Room.SID)

	// 更新房间状态为已结束
	if err := ws.db.Model(&models.Room{}).
		Where("room_id = ?", event.Room.Name).
		Update("status", models.RoomStatusFinished).Error; err != nil {
		log.Printf("❌ 更新房间状态失败: %v", err)
		return err
	}

	return nil
}

// handleParticipantJoined 处理参与者加入事件
func (ws *WebhookService) handleParticipantJoined(event *models.WebhookEvent) error {
	if event.Room == nil || event.Participant == nil {
		return nil
	}

	log.Printf("✅ 参与者已加入: %s (Identity: %s) 房间: %s",
		event.Participant.Name, event.Participant.Identity, event.Room.Name)

	// 可以在这里添加业务逻辑
	// - 更新参与者状态
	// - 发送通知
	// - 记录日志

	return nil
}

// handleParticipantLeft 处理参与者离开事件
func (ws *WebhookService) handleParticipantLeft(event *models.WebhookEvent) error {
	if event.Room == nil || event.Participant == nil {
		return nil
	}

	log.Printf("✅ 参与者已离开: %s (Identity: %s) 房间: %s",
		event.Participant.Name, event.Participant.Identity, event.Room.Name)

	// 更新参与者状态为已挂断
	if err := ws.db.Model(&models.Participant{}).
		Where("uid = ? AND room_id = ?", event.Participant.Identity, event.Room.Name).
		Update("status", models.ParticipantStatusHangup).Error; err != nil {
		log.Printf("❌ 更新参与者状态失败: %v", err)
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
