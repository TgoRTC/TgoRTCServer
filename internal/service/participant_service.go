package service

import (
	"time"

	"tgo-rtc-server/internal/errors"
	"tgo-rtc-server/internal/i18n"
	"tgo-rtc-server/internal/livekit"
	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ParticipantService 参与者服务
type ParticipantService struct {
	db                     *gorm.DB
	tokenGenerator         *livekit.TokenGenerator
	timeFormatter          *utils.TimeFormatter
	businessWebhookService *BusinessWebhookService
	schedulerService       *SchedulerService
	lifecycleService       *RoomLifecycleService
}

// NewParticipantService 创建参与者服务
func NewParticipantService(db *gorm.DB, tokenGenerator *livekit.TokenGenerator, businessWebhookService *BusinessWebhookService) *ParticipantService {
	return &ParticipantService{
		db:                     db,
		tokenGenerator:         tokenGenerator,
		timeFormatter:          utils.NewTimeFormatter(),
		businessWebhookService: businessWebhookService,
	}
}

// SetSchedulerService 设置调度器服务
func (ps *ParticipantService) SetSchedulerService(ss *SchedulerService) {
	ps.schedulerService = ss
}

// SetLifecycleService 设置房间生命周期服务
func (ps *ParticipantService) SetLifecycleService(ls *RoomLifecycleService) {
	ps.lifecycleService = ls
}

// JoinRoom 参与者加入房间
func (ps *ParticipantService) JoinRoom(req *models.JoinRoomRequest) (*models.JoinRoomResponse, error) {
	// 检查房间是否存在
	var room models.Room
	if err := ps.db.Where("room_id = ?", req.RoomID).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.NewBusinessErrorWithKey(i18n.RoomNotFound, req.RoomID)
		}
		return nil, errors.NewBusinessErrorWithKey(i18n.RoomQueryFailed, err.Error())
	}
	if room.Status == models.RoomStatusFinished || room.Status == models.RoomStatusCancelled {
		return nil, errors.NewBusinessErrorWithKey(i18n.RoomNotActive)
	}

	// 检查房间参与者人数是否已达到最大值（包括邀请中和已加入的）
	var participantCount int64
	if err := ps.db.Model(&models.Participant{}).
		Where("room_id = ? AND status IN ?", req.RoomID, []int{models.ParticipantStatusInviting, models.ParticipantStatusJoined}).
		Count(&participantCount).Error; err != nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
	}

	if int(participantCount) >= room.MaxParticipants {
		return nil, errors.NewBusinessErrorWithKey(i18n.RoomFull)
	}

	// 如果房间开启了邀请，检查该用户是否被邀请
	if room.InviteOn == models.InviteEnabled {
		var invitedParticipant models.Participant
		if err := ps.db.Where("room_id = ? AND uid = ? AND status = ?", req.RoomID, req.UID, models.ParticipantStatusInviting).First(&invitedParticipant).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantNotInvited)
			}
			return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
		}
	}

	// 检查参与者是否已存在
	var existingParticipant models.Participant
	if err := ps.db.Where("room_id = ? AND uid = ?", req.RoomID, req.UID).First(&existingParticipant).Error; err == nil {
		// 参与者已存在，更新状态为已加入
		if err := ps.db.Model(&existingParticipant).Updates(map[string]interface{}{
			"status":      models.ParticipantStatusJoined,
			"join_time":   time.Now().Unix(),
			"device_type": req.DeviceType,
		}).Error; err != nil {
			return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantStatusUpdateFailed, err.Error())
		}
		// 取消超时定时器
		if ps.schedulerService != nil {
			ps.schedulerService.CancelParticipantTimeout(req.RoomID, req.UID)
		}
	} else if err == gorm.ErrRecordNotFound {
		// 创建新的参与者记录
		participant := models.Participant{
			RoomID:     req.RoomID,
			UID:        req.UID,
			DeviceType: req.DeviceType,
			Status:     models.ParticipantStatusJoined,
			JoinTime:   time.Now().Unix(),
		}
		if err := ps.db.Create(&participant).Error; err != nil {
			return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantAddFailed, err.Error())
		}
	} else {
		return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
	}

	// 生成 Token 和获取配置信息
	tokenResult, err := ps.tokenGenerator.GenerateTokenWithConfig(req.RoomID, req.UID, req.DeviceType)
	if err != nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.TokenGenerationFailed, err.Error())
	}

	// 获取所有参与者的 UIDs
	var participants []models.Participant
	if err := ps.db.Where("room_id = ?", req.RoomID).Find(&participants).Error; err != nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
	}

	uids := make([]string, 0, len(participants))
	for _, p := range participants {
		uids = append(uids, p.UID)
	}

	return &models.JoinRoomResponse{
		RoomID:          req.RoomID,
		Creator:         room.Creator,
		Token:           tokenResult.Token,
		URL:             tokenResult.URL,
		RTCType:         room.RTCType,
		Status:          room.Status,
		CreatedAt:       ps.timeFormatter.FormatDateTime(room.CreatedAt),
		MaxParticipants: room.MaxParticipants,
		Timeout:         tokenResult.Timeout,
		UIDs:            uids,
	}, nil
}

// LeaveRoom 参与者离开房间
func (ps *ParticipantService) LeaveRoom(req *models.LeaveRoomRequest) error {
	if ps.lifecycleService == nil {
		return errors.NewBusinessErrorWithKey(i18n.RoomQueryFailed, "lifecycle service not initialized")
	}
	return ps.lifecycleService.HandleUserLeave(req.RoomID, req.UID)
}

// InviteParticipants 邀请参与者
func (ps *ParticipantService) InviteParticipants(req *models.InviteParticipantRequest) error {
	logger := utils.GetLogger()

	// 检查房间是否存在
	var room models.Room
	if err := ps.db.Where("room_id = ?", req.RoomID).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.NewBusinessErrorWithKey(i18n.RoomNotFound, req.RoomID)
		}
		return errors.NewBusinessErrorWithKey(i18n.RoomQueryFailed, err.Error())
	}

	// 检查房间状态是否可以邀请（只有未开始或进行中的房间可以邀请）
	if room.Status != models.RoomStatusNotStarted && room.Status != models.RoomStatusInProgress {
		return errors.NewBusinessErrorWithKey(i18n.RoomNotActive)
	}

	// 查询房间参与者
	var roomParticipants []models.Participant
	if err := ps.db.Where("room_id = ?", req.RoomID).Find(&roomParticipants).Error; err != nil {
		return errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
	}

	// 判断邀请者是否在房间里
	var inviteParticipant models.Participant

	// 检查当前房间参与者人数（包括邀请中和已加入的）
	var currentParticipantCount int64
	// 这里直接用已查到的 roomParticipants 计算当前人数（只统计邀请中和已加入的）
	currentParticipantCount = 0
	for _, p := range roomParticipants {
		if p.Status == models.ParticipantStatusInviting || p.Status == models.ParticipantStatusJoined {
			currentParticipantCount++
		}
		if p.UID == req.InviteUID {
			inviteParticipant = p
		}
	}
	// 邀请者不在房间里
	if inviteParticipant.ID == 0 {
		return errors.NewBusinessErrorWithKey(i18n.ParticipantNotFound, req.InviteUID)
	}

	// 检查邀请后是否会超过最大人数
	if int(currentParticipantCount)+len(req.UIDs) > room.MaxParticipants {
		return errors.NewBusinessErrorWithKey(i18n.RoomFull)
	}

	// 批量查询该房间中已存在的参与者（限定在 req.UIDs 范围内）
	var existingParticipants []models.Participant
	if err := ps.db.Where("room_id = ? AND uid IN ?", req.RoomID, req.UIDs).
		Find(&existingParticipants).Error; err != nil {
		return errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
	}

	// 构建已存在 uid 的 map，便于快速查找
	existingUIDMap := make(map[string]models.Participant)
	for _, p := range existingParticipants {
		existingUIDMap[p.UID] = p
	}

	// 在事务中处理：已存在的更新状态，不存在的创建新记录
	err := ps.db.Transaction(func(tx *gorm.DB) error {
		for _, uid := range req.UIDs {
			if existingParticipant, exists := existingUIDMap[uid]; exists {
				// 参与者已存在，更新状态为邀请中，并重置 created_at 以便超时检查重新计时
				if err := tx.Model(&models.Participant{}).
					Where("id = ?", existingParticipant.ID).
					Updates(map[string]interface{}{
						"status":     models.ParticipantStatusInviting,
						"created_at": time.Now(),
					}).Error; err != nil {
					return errors.NewBusinessErrorWithKey(i18n.ParticipantStatusUpdateFailed, err.Error())
				}
			} else {
				// 参与者不存在，创建新记录
				participant := models.Participant{
					RoomID: req.RoomID,
					UID:    uid,
					Status: models.ParticipantStatusInviting,
				}
				if err := tx.Create(&participant).Error; err != nil {
					return errors.NewBusinessErrorWithKey(i18n.InvitedParticipantAddFailed, err.Error())
				}
			}
		}
		return nil
	})

	if err != nil {
		logger.Error("邀请参与者事务失败",
			zap.String("room_id", req.RoomID),
			zap.Error(err),
		)
		return err
	}

	// 为被邀请的参与者设置超时定时器
	if ps.schedulerService != nil {
		for _, uid := range req.UIDs {
			ps.schedulerService.ScheduleParticipantTimeout(req.RoomID, uid)
		}
	}

	// 发送邀请业务 webhook 事件
	if ps.businessWebhookService != nil {
		joinedUids := make([]string, 0, len(roomParticipants))
		for _, p := range roomParticipants {
			if p.Status == models.ParticipantStatusJoined {
				joinedUids = append(joinedUids, p.UID)
			}
		}
		ps.businessWebhookService.sendParticipantInvited(&room, joinedUids, req.UIDs, req.InviteUID)
	}

	return nil
}

// GetUserAvailableRooms 获取用户可加入的房间列表
// 查询该用户被邀请（status=0）或已加入（status=1）的所有房间
// 返回 RoomResp 数组
func (ps *ParticipantService) GetUserAvailableRooms(uid string, deviceType string) ([]models.RoomResp, error) {
	// 查询用户的参与者记录（邀请中或已加入）
	var participants []models.Participant
	if err := ps.db.Where("uid = ? AND status IN ?", uid, []int{models.ParticipantStatusInviting, models.ParticipantStatusJoined}).
		Find(&participants).Error; err != nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
	}

	// 如果没有参与的房间，返回空数组
	if len(participants) == 0 {
		return []models.RoomResp{}, nil
	}

	// 提取所有房间 ID
	roomIDs := make([]string, 0, len(participants))
	for _, p := range participants {

		// 如果参与者正在通话，且设备类型不匹配，则跳过
		if p.Status == models.ParticipantStatusJoined && p.DeviceType != deviceType {
			continue
		}

		roomIDs = append(roomIDs, p.RoomID)
	}

	// 查询所有房间信息（只查询未结束和未取消的房间）
	var rooms []models.Room
	if err := ps.db.Where("room_id IN ? AND status IN ?", roomIDs, []int{models.RoomStatusNotStarted, models.RoomStatusInProgress}).
		Find(&rooms).Error; err != nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.RoomQueryFailed, err.Error())
	}

	if len(rooms) == 0 {
		return []models.RoomResp{}, nil
	}

	// 提取查询到的房间 ID 列表
	queryRoomIDs := make([]string, 0, len(rooms))
	for _, r := range rooms {
		queryRoomIDs = append(queryRoomIDs, r.RoomID)
	}

	// 一次性查询这些房间的所有活跃参与者（邀请中或已加入）
	var allRoomParticipants []models.Participant
	if err := ps.db.Where("room_id IN ? AND status IN ?", queryRoomIDs, []int{models.ParticipantStatusInviting, models.ParticipantStatusJoined}).
		Find(&allRoomParticipants).Error; err != nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
	}

	// 按房间 ID 分组参与者
	roomParticipantMap := make(map[string][]string)
	for _, p := range allRoomParticipants {
		roomParticipantMap[p.RoomID] = append(roomParticipantMap[p.RoomID], p.UID)
	}

	// 构建返回结果
	result := make([]models.RoomResp, 0, len(rooms))
	for _, room := range rooms {
		tempDeviceType := ""
		for _, p := range participants {
			if p.RoomID == room.RoomID && p.UID == uid {
				tempDeviceType = p.DeviceType
				break
			}
		}
		if tempDeviceType == "" {
			tempDeviceType = deviceType
		}
		// 为每个房间生成 Token
		tokenResult, err := ps.tokenGenerator.GenerateTokenWithConfig(room.RoomID, uid, tempDeviceType)
		if err != nil {
			return nil, errors.NewBusinessErrorWithKey(i18n.TokenGenerationFailed, err.Error())
		}

		// 从预先查询的 map 中获取参与者 UIDs
		uids := roomParticipantMap[room.RoomID]
		if uids == nil {
			uids = []string{}
		}

		result = append(result, models.RoomResp{
			RoomID:          room.RoomID,
			Creator:         room.Creator,
			Token:           tokenResult.Token,
			URL:             tokenResult.URL,
			RTCType:         room.RTCType,
			Status:          room.Status,
			CreatedAt:       ps.timeFormatter.FormatDateTime(room.CreatedAt),
			MaxParticipants: room.MaxParticipants,
			Timeout:         tokenResult.Timeout,
			UIDs:            uids,
		})
	}

	return result, nil
}
