package service

import (
	"strings"
	"tgo-rtc-server/internal/errors"
	"tgo-rtc-server/internal/i18n"
	"tgo-rtc-server/internal/livekit"
	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/utils"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RoomService 房间服务
type RoomService struct {
	db                      *gorm.DB
	tokenGenerator          *livekit.TokenGenerator
	agentDispatcher         *livekit.AgentDispatcher
	timeFormatter           *utils.TimeFormatter
	participantDeduplicator *utils.ParticipantDeduplicator
	schedulerService        *SchedulerService
}

// NewRoomService 创建房间服务
func NewRoomService(db *gorm.DB, tokenGenerator *livekit.TokenGenerator, agentDispatcher *livekit.AgentDispatcher) *RoomService {
	return &RoomService{
		db:                      db,
		tokenGenerator:          tokenGenerator,
		agentDispatcher:         agentDispatcher,
		timeFormatter:           utils.NewTimeFormatter(),
		participantDeduplicator: utils.NewParticipantDeduplicator(),
	}
}

// SetSchedulerService 设置调度器服务
func (rs *RoomService) SetSchedulerService(ss *SchedulerService) {
	rs.schedulerService = ss
}

// CreateRoom 创建房间
func (rs *RoomService) CreateRoom(req *models.CreateRoomRequest) (*models.CreateRoomResponse, error) {
	// AI 通话走独立流程
	logger := utils.GetLogger()
	if req.CallMode == models.CallModeAI {
		logger.Info("create AI room", zap.String("room_id", req.RoomID))
		return rs.createAIRoom(req)
	}
	logger.Info("create Normal room", zap.String("room_id", req.RoomID))
	// 0. 兼容 UIDs 未传递的情况，初始化为空切片
	if req.UIDs == nil {
		req.UIDs = []string{}
	}

	// 1. 如果 room_id 没有传递，则生成 UUID（去掉 '-'）
	roomID := req.RoomID
	if roomID == "" {
		roomID = strings.ReplaceAll(uuid.New().String(), "-", "")
	} else {
		// 2. 如果 room_id 已传递，检查是否已存在
		var existingRoom models.Room
		if err := rs.db.Where("room_id = ?", roomID).First(&existingRoom).Error; err == nil {
			return nil, errors.NewBusinessErrorWithKey(i18n.RoomAlreadyExists, roomID)
		} else if err != gorm.ErrRecordNotFound {
			return nil, errors.NewBusinessErrorWithKey(i18n.RoomQueryFailed, err.Error())
		}
	}

	// 3. 检查 creator 是否在 rtc_participant 表存在 status=0/1 的情况
	var participant models.Participant
	if err := rs.db.Where("uid = ? AND (status = ? OR status = ?)",
		req.Creator, models.ParticipantStatusInviting, models.ParticipantStatusJoined).
		First(&participant).Error; err == nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.CreatorInAnotherCall)
	} else if err != gorm.ErrRecordNotFound {
		return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
	}

	// 5. 对 UIDs 进行去重，并移除创建者（避免重复添加）
	deduplicatedUIDs := rs.participantDeduplicator.DeduplicateUIDs(req.UIDs)
	deduplicatedUIDs = rs.participantDeduplicator.RemoveDuplicateUIDs(deduplicatedUIDs, req.Creator)
	isBusy := false
	var busyParticipantUID string
	// 6. 检查 UIDs 中的用户是否在通话中
	if len(deduplicatedUIDs) > 0 {
		var busyParticipant models.Participant
		if err := rs.db.Where("uid IN ? AND (status = ? OR status = ?)",
			deduplicatedUIDs, models.ParticipantStatusInviting, models.ParticipantStatusJoined).
			First(&busyParticipant).Error; err == nil {
			isBusy = true
			busyParticipantUID = busyParticipant.UID
			//return nil, errors.NewConflictError(i18n.ParticipantInCall, busyParticipant.UID)
		} else if err != gorm.ErrRecordNotFound {
			return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
		}
	}

	// 设置 MaxParticipants，如果未传递则默认为 2
	maxParticipants := req.MaxParticipants
	if maxParticipants <= 0 {
		maxParticipants = 2
	}
	participantStatus := models.ParticipantStatusInviting
	roomStatus := models.RoomStatusNotStarted
	if isBusy {
		roomStatus = models.RoomStatusBusy
		participantStatus = models.ParticipantStatusBusy
	}
	// 使用事务确保数据一致性
	err := rs.db.Transaction(func(tx *gorm.DB) error {
		// 创建房间
		room := models.Room{
			Creator:         req.Creator,
			RoomID:          roomID,
			RTCType:         req.RTCType,
			InviteOn:        req.InviteOn,
			Status:          uint8(roomStatus),
			MaxParticipants: maxParticipants,
		}

		if err := tx.Create(&room).Error; err != nil {
			return errors.NewBusinessErrorWithKey(i18n.RoomCreationFailed, err.Error())
		}

		// 合并创建者和去重后的邀请用户到一个数组中
		participants := make([]models.Participant, 0)

		// 添加创建者
		participants = append(participants, models.Participant{
			RoomID:     roomID,
			UID:        req.Creator,
			DeviceType: req.DeviceType,
			Status:     uint8(participantStatus),
		})

		// 添加去重后的邀请用户
		if len(deduplicatedUIDs) > 0 {
			for _, uid := range deduplicatedUIDs {
				participants = append(participants, models.Participant{
					RoomID: roomID,
					UID:    uid,
					Status: uint8(participantStatus),
				})
			}
		}

		// 批量创建参与者记录
		if err := tx.Create(&participants).Error; err != nil {
			return errors.NewBusinessErrorWithKey(i18n.ParticipantAddFailed, err.Error())
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	// 如果正在通话中直接返回错误，不能返回房间信息
	if isBusy {
		return nil, errors.NewConflictError(i18n.ParticipantInCall, busyParticipantUID)
	}

	// 为所有参与者设置超时定时器
	if rs.schedulerService != nil {
		// 为创建者设置定时器
		rs.schedulerService.ScheduleParticipantTimeout(roomID, req.Creator)
		// 为被邀请者设置定时器
		for _, uid := range deduplicatedUIDs {
			rs.schedulerService.ScheduleParticipantTimeout(roomID, uid)
		}
	}

	// 生成 Token 和获取配置信息
	tokenResult, err := rs.tokenGenerator.GenerateTokenWithConfig(roomID, req.Creator, req.DeviceType)
	if err != nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.TokenGenerationFailed, err.Error())
	}

	uids := append(req.UIDs, req.Creator)
	uids = rs.participantDeduplicator.DeduplicateUIDs(uids)
	return &models.CreateRoomResponse{
		RoomID:          roomID,
		Creator:         req.Creator,
		Token:           tokenResult.Token,
		URL:             tokenResult.URL,
		Status:          models.RoomStatusNotStarted,
		CreatedAt:       rs.timeFormatter.FormatDateTime(time.Now()),
		MaxParticipants: maxParticipants,
		Timeout:         tokenResult.Timeout,
		RTCType:         req.RTCType,
		UIDs:            uids,
	}, nil
}

// createAIRoom 创建 AI Agent 通话房间
func (rs *RoomService) createAIRoom(req *models.CreateRoomRequest) (*models.CreateRoomResponse, error) {
	logger := utils.GetLogger()
	logger.Info("createAIRoom: 开始",
		zap.String("creator", req.Creator),
		zap.Uint8("call_mode", req.CallMode),
	)
	// 1. 生成 roomID
	roomID := req.RoomID
	if roomID == "" {
		roomID = strings.ReplaceAll(uuid.New().String(), "-", "")
	} else {
		var existingRoom models.Room
		if err := rs.db.Where("room_id = ?", roomID).First(&existingRoom).Error; err == nil {
			return nil, errors.NewBusinessErrorWithKey(i18n.RoomAlreadyExists, roomID)
		} else if err != gorm.ErrRecordNotFound {
			return nil, errors.NewBusinessErrorWithKey(i18n.RoomQueryFailed, err.Error())
		}
	}

	// 2. 检查创建者是否在通话中
	var participant models.Participant
	if err := rs.db.Where("uid = ? AND (status = ? OR status = ?)",
		req.Creator, models.ParticipantStatusInviting, models.ParticipantStatusJoined).
		First(&participant).Error; err == nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.CreatorInAnotherCall)
	} else if err != gorm.ErrRecordNotFound {
		return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
	}

	// 3. 事务：创建房间 + 仅创建者参与者
	err := rs.db.Transaction(func(tx *gorm.DB) error {
		room := models.Room{
			Creator:         req.Creator,
			RoomID:          roomID,
			RTCType:         req.RTCType,
			Status:          uint8(models.RoomStatusNotStarted),
			MaxParticipants: 2,
			CallMode:        models.CallModeAI,
		}
		if err := tx.Create(&room).Error; err != nil {
			return errors.NewBusinessErrorWithKey(i18n.RoomCreationFailed, err.Error())
		}

		creator := models.Participant{
			RoomID:     roomID,
			UID:        req.Creator,
			DeviceType: req.DeviceType,
			Status:     uint8(models.ParticipantStatusInviting),
		}
		if err := tx.Create(&creator).Error; err != nil {
			return errors.NewBusinessErrorWithKey(i18n.ParticipantAddFailed, err.Error())
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 4. 设置创建者超时定时器
	if rs.schedulerService != nil {
		rs.schedulerService.ScheduleParticipantTimeout(roomID, req.Creator)
	}

	// 5. 生成 Token
	tokenResult, err := rs.tokenGenerator.GenerateTokenWithConfig(roomID, req.Creator, req.DeviceType)
	if err != nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.TokenGenerationFailed, err.Error())
	}

	// 6. 调度 AI Agent 加入房间
	if rs.agentDispatcher != nil {
		if err := rs.agentDispatcher.Dispatch(roomID); err != nil {
			logger.Error("dispatch agent failed", zap.String("room_id", roomID), zap.Error(err))
			return nil, errors.NewBusinessErrorWithKey(i18n.RoomCreationFailed, err.Error())
		}
	}

	return &models.CreateRoomResponse{
		RoomID:          roomID,
		Creator:         req.Creator,
		Token:           tokenResult.Token,
		URL:             tokenResult.URL,
		Status:          models.RoomStatusNotStarted,
		CreatedAt:       rs.timeFormatter.FormatDateTime(time.Now()),
		MaxParticipants: 2,
		Timeout:         tokenResult.Timeout,
		RTCType:         req.RTCType,
		UIDs:            []string{req.Creator},
	}, nil
}
