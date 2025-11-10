package service

import (
	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/utils"
	"time"

	"go.uber.org/zap"
)

// checkAndFinishRoom 检查房间的所有参与者是否都已结束，如果是则将房间状态改为完成
// room 参数会被更新，调用者可以使用更新后的 room.Status
func (ps *BusinessWebhookService) checkAndFinishRoom(room *models.Room) {
	logger := utils.GetLogger()
	isSendWebhook := false
	// 如果房间已经是完成状态或拒绝状态，跳过
	if room.Status > models.RoomStatusInProgress {
		isSendWebhook = true
	}
	var duration int64 = 0
	var participants []models.Participant
	if err := ps.db.Where("room_id = ?", room.RoomID).Find(&participants).Error; err != nil {
		logger.Error("查询房间参与者失败",
			zap.String("room_id", room.RoomID),
			zap.Error(err),
		)
		return
	}

	if !isSendWebhook {
		// 查询房间的所有参与者
		// 检查是否所有参与者都已结束
		// 结束状态包括: 超时(4)、挂断(3)、取消(6)、拒绝(2)
		allFinished := true
		for _, p := range participants {
			if p.Status < models.ParticipantStatusRejected {
				allFinished = false
				break
			}
		}

		// 如果所有参与者都已结束，更新房间状态为完成
		if allFinished {
			if err := ps.db.Model(&models.Room{}).
				Where("room_id = ?", room.RoomID).
				Update("status", models.RoomStatusFinished).Error; err != nil {
				logger.Error("更新房间状态为完成失败",
					zap.String("room_id", room.RoomID),
					zap.Error(err),
				)
				return
			}

			// 更新传入的 room 对象
			room.Status = models.RoomStatusFinished
			room.UpdatedAt = time.Now()
			isSendWebhook = true
			logger.Info("✅ 房间所有参与者已结束，更新房间状态为完成",
				zap.String("room_id", room.RoomID),
				zap.Int("participant_count", len(participants)),
			)

		}
	}

	if isSendWebhook {
		var maxJoinTime int64 = 0
		var maxLeaveTime int64 = 0
		for _, p := range participants {
			if p.Status != models.ParticipantStatusHangup {
				continue
			}
			if p.JoinTime > maxJoinTime {
				maxJoinTime = p.JoinTime
			}
			if p.LeaveTime > maxLeaveTime {
				maxLeaveTime = p.LeaveTime
			}
		}
		duration = maxLeaveTime - maxJoinTime
		// 发送房间完成事件
		ps.sendRoomFinished(room, duration)
	}
}

// 发送房间开始事件
func (bws *BusinessWebhookService) sendRoomStarted(room *models.Room) {
	logger := utils.GetLogger()
	eventData := &models.RoomEventData{
		SourceChannelID:   room.SourceChannelID,
		SourceChannelType: room.SourceChannelType,
		RoomID:            room.RoomID,
		Creator:           room.Creator,
		RTCType:           room.RTCType,
		InviteOn:          room.InviteOn,
		Status:            models.RoomStatusInProgress,
		MaxParticipants:   room.MaxParticipants,
		CreatedAt:         room.CreatedAt.Unix(),
		UpdatedAt:         room.UpdatedAt.Unix(),
	}
	if err := bws.SendEvent(models.BusinessEventRoomStarted, eventData); err != nil {
		logger.Error("发送业务 webhook 事件失败",
			zap.String("room_id", room.RoomID),
			zap.String("event_type", models.BusinessEventRoomStarted),
			zap.Error(err),
		)
	}
}

// sendRoomFinished 发送房间完成事件
// 使用 Redis 确保同一个房间只发送一次
func (bws *BusinessWebhookService) sendRoomFinished(room *models.Room, duration int64) {
	logger := utils.GetLogger()

	// 构建事件数据
	eventData := &models.RoomEventData{
		SourceChannelID:   room.SourceChannelID,
		SourceChannelType: room.SourceChannelType,
		RoomID:            room.RoomID,
		Creator:           room.Creator,
		RTCType:           room.RTCType,
		InviteOn:          room.InviteOn,
		Status:            room.Status,
		MaxParticipants:   room.MaxParticipants,
		CreatedAt:         room.CreatedAt.Unix(),
		UpdatedAt:         room.UpdatedAt.Unix(),
		Duration:          duration,
	}

	// 发送业务 webhook 通知（确保同一个房间只发送一次）
	if err := bws.SendRoomFinishedEventOnce(room.RoomID, eventData); err != nil {
		logger.Error("发送房间完成事件失败",
			zap.String("room_id", room.RoomID),
			zap.Error(err),
		)
		// 不返回错误，因为房间状态已经更新成功
	}
}

// 发送参与者加入事件
func (bws *BusinessWebhookService) sendParticipantJoined(room *models.Room, uid string) {
	logger := utils.GetLogger()
	eventData := &models.ParticipantEventData{
		RoomEventData: models.RoomEventData{
			SourceChannelID:   room.SourceChannelID,
			SourceChannelType: room.SourceChannelType,
			RoomID:            room.RoomID,
			Creator:           room.Creator,
			RTCType:           room.RTCType,
			InviteOn:          room.InviteOn,
			Status:            room.Status,
			MaxParticipants:   room.MaxParticipants,
			CreatedAt:         room.CreatedAt.Unix(),
			UpdatedAt:         room.UpdatedAt.Unix(),
		},
		UID: uid, // 加入者 UID
	}

	// 发送 webhook 事件
	if err := bws.SendEvent(models.BusinessEventParticipantJoined, eventData); err != nil {
		logger.Error("发送业务 webhook 事件失败",
			zap.String("room_id", room.RoomID),
			zap.String("uid", uid),
			zap.String("event_type", models.BusinessEventParticipantJoined),
			zap.Error(err),
		)
		// 不返回错误，因为参与者状态已经更新成功
	}
}

// 发送参与者离开事件
func (bws *BusinessWebhookService) sendParticipantLeft(room *models.Room, uid string) {
	logger := utils.GetLogger()
	eventData := &models.ParticipantEventData{
		RoomEventData: models.RoomEventData{
			SourceChannelID:   room.SourceChannelID,
			SourceChannelType: room.SourceChannelType,
			RoomID:            room.RoomID,
			Creator:           room.Creator,
			RTCType:           room.RTCType,
			InviteOn:          room.InviteOn,
			Status:            room.Status,
			MaxParticipants:   room.MaxParticipants,
			CreatedAt:         room.CreatedAt.Unix(),
			UpdatedAt:         time.Now().Unix(),
		},
		UID: uid, // 离开者是当前离开的参与者
	}

	// 发送一次 webhook 事件
	if err := bws.SendEvent(models.BusinessEventParticipantLeft, eventData); err != nil {
		logger.Error("发送业务 webhook 事件失败",
			zap.String("room_id", room.RoomID),
			zap.String("event_type", models.BusinessEventParticipantLeft),
			zap.Error(err),
		)
	}
}

// 发送参与者拒绝事件
func (bws *BusinessWebhookService) sendParticipantRejected(room *models.Room, uid string) {
	logger := utils.GetLogger()
	eventData := &models.ParticipantEventData{
		RoomEventData: models.RoomEventData{
			SourceChannelID:   room.SourceChannelID,
			SourceChannelType: room.SourceChannelType,
			RoomID:            room.RoomID,
			Creator:           room.Creator,
			RTCType:           room.RTCType,
			InviteOn:          room.InviteOn,
			Status:            room.Status,
			MaxParticipants:   room.MaxParticipants,
			CreatedAt:         room.CreatedAt.Unix(),
			UpdatedAt:         time.Now().Unix(),
		},
		UID: uid, // 拒绝者是当前离开的参与者
	}

	// 发送一次 webhook 事件
	if err := bws.SendEvent(models.BusinessEventParticipantRejected, eventData); err != nil {
		logger.Error("发送业务 webhook 事件失败",
			zap.String("room_id", room.RoomID),
			zap.String("event_type", models.BusinessEventParticipantRejected),
			zap.Error(err),
		)
	}
}

// 发送参与者超时事件
func (bws *BusinessWebhookService) sendParticipantMissed(room *models.Room, uid string) {
	logger := utils.GetLogger()
	eventData := &models.ParticipantEventData{
		RoomEventData: models.RoomEventData{
			SourceChannelID:   room.SourceChannelID,
			SourceChannelType: room.SourceChannelType,
			RoomID:            room.RoomID,
			Creator:           room.Creator,
			RTCType:           room.RTCType,
			InviteOn:          room.InviteOn,
			Status:            room.Status,
			MaxParticipants:   room.MaxParticipants,
			CreatedAt:         room.CreatedAt.Unix(),
			UpdatedAt:         room.UpdatedAt.Unix(),
		},
		UID: uid,
	}

	// 发送 participant.missed 事件
	if err := bws.SendEvent(models.BusinessEventParticipantMissed, eventData); err != nil {
		logger.Error("发送业务 webhook 事件失败",
			zap.String("room_id", room.RoomID),
			zap.String("uid", uid),
			zap.String("event_type", models.BusinessEventParticipantMissed),
			zap.Error(err),
		)
		// 不返回错误，继续处理其他参与者
	}
}

// 发送参与者取消事件
func (bws *BusinessWebhookService) sendParticipantCancelled(room *models.Room, uids []string) {
	logger := utils.GetLogger()
	// 构建事件数据
	eventData := &models.ParticipantEventData{
		RoomEventData: models.RoomEventData{
			SourceChannelID:   room.SourceChannelID,
			SourceChannelType: room.SourceChannelType,
			RoomID:            room.RoomID,
			Creator:           room.Creator,
			RTCType:           room.RTCType,
			InviteOn:          room.InviteOn,
			Status:            models.RoomStatusCancelled,
			MaxParticipants:   room.MaxParticipants,
			Uids:              uids,
			CreatedAt:         room.CreatedAt.Unix(),
			UpdatedAt:         time.Now().Unix(),
		},
		UID: room.Creator, // 取消者是房间创建者
	}

	// 发送一次 webhook 事件
	if err := bws.SendEvent(models.BusinessEventParticipantCancelled, eventData); err != nil {
		logger.Error("发送业务 webhook 事件失败",
			zap.String("room_id", room.RoomID),
			zap.String("event_type", models.BusinessEventParticipantCancelled),
			zap.Error(err),
		)
	}
}
