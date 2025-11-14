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
		logger.Error("checkAndFinishRoom: 查询房间参与者失败",
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

		// 如果所有参与者都已结束，更新房间状态为完成|超时未接听
		roomStatus := models.RoomStatusFinished
		if room.MaxParticipants == 2 {
			for _, p := range participants {
				if p.Status == models.ParticipantStatusMissed {
					roomStatus = models.RoomStatusMissed
					break
				}
				if p.Status == models.ParticipantStatusBusy {
					roomStatus = models.RoomStatusBusy
					break
				}
				if p.Status == models.ParticipantStatusCancelled {
					roomStatus = models.RoomStatusCancelled
					break
				}
				if p.Status == models.ParticipantStatusRejected {
					roomStatus = models.RoomStatusRejected
					break
				}
			}
		} else {
			// 多人通话场景
		}

		if allFinished {
			room.Status = uint8(roomStatus)
			if err := ps.db.Model(&models.Room{}).
				Where("room_id = ?", room.RoomID).
				Update("status", room.Status).Error; err != nil {
				logger.Error("checkAndFinishRoom: 更新房间状态为完成失败",
					zap.String("room_id", room.RoomID),
					zap.Uint8("room.Status", room.Status),
					zap.Error(err),
				)
				return
			}
			room.UpdatedAt = time.Now()
			isSendWebhook = true
		}
	}
	logger.Info("checkAndFinishRoom: 房间状态",
		zap.String("room_id", room.RoomID),
		zap.Int("room_status", int(room.Status)),
		zap.Bool("isSendWebhook", isSendWebhook),
	)
	if isSendWebhook {
		// 计算通话时长
		var startTime int64 = 0
		var endTime int64 = 0

		for _, p := range participants {
			if p.Status != models.ParticipantStatusHangup {
				continue
			}
			if p.JoinTime > startTime {
				startTime = p.JoinTime
			}
			if p.LeaveTime > endTime {
				endTime = p.LeaveTime
			}
		}

		duration = endTime - startTime
		if startTime == 0 || endTime == 0 {
			duration = 0
		}

		uids := make([]string, 0, len(participants))
		for _, p := range participants {
			uids = append(uids, p.UID)
		}
		// 发送房间完成事件
		ps.sendRoomFinished(room, duration, uids)
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
	uids, err := bws.getRoomParticipantsUids(room.RoomID)
	if err != nil {
		return
	}
	eventData.Uids = uids
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
func (bws *BusinessWebhookService) sendRoomFinished(room *models.Room, duration int64, uids []string) {
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
		Uids:              uids,
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
func (bws *BusinessWebhookService) sendParticipantJoined(room *models.Room, uid string, deviceType string) {
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
		UID:        uid,        // 加入者 UID
		DeviceType: deviceType, // 设备类型
	}
	uids, err := bws.getRoomParticipantsUids(room.RoomID)
	if err != nil {
		return
	}
	eventData.Uids = uids
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
func (bws *BusinessWebhookService) sendParticipantLeft(room *models.Room, uid string, uids []string) {
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
			Uids:              uids,
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
func (bws *BusinessWebhookService) sendParticipantRejected(room *models.Room, uid string, uids []string) {
	logger := utils.GetLogger()
	eventData := &models.ParticipantEventData{
		RoomEventData: models.RoomEventData{
			SourceChannelID:   room.SourceChannelID,
			SourceChannelType: room.SourceChannelType,
			RoomID:            room.RoomID,
			Creator:           room.Creator,
			RTCType:           room.RTCType,
			Uids:              uids,
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
func (bws *BusinessWebhookService) sendParticipantMissed(room *models.Room, uids []string) {
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
		MissedUids: uids,
	}
	uids, err := bws.getRoomParticipantsUids(room.RoomID)
	if err != nil {
		return
	}
	eventData.Uids = uids
	// 发送 participant.missed 事件
	if err := bws.SendEvent(models.BusinessEventParticipantMissed, eventData); err != nil {
		logger.Error("发送参与者超时事件失败",
			zap.String("room_id", room.RoomID),
			zap.Strings("uids", uids),
			zap.String("event_type", models.BusinessEventParticipantMissed),
			zap.Error(err),
		)
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

// 获取房间所有参与者的 UID 列表
func (bws *BusinessWebhookService) getRoomParticipantsUids(roomID string) ([]string, error) {
	logger := utils.GetLogger()
	// 查询所有参与者
	var participants []models.Participant
	if err := bws.db.Where("room_id = ?", roomID).Find(&participants).Error; err != nil {
		logger.Error("查询参与者失败",
			zap.String("room_id", roomID),
			zap.Error(err),
		)
		return nil, err
	}
	uids := make([]string, 0, len(participants))
	for _, p := range participants {
		uids = append(uids, p.UID)
	}
	return uids, nil

}
