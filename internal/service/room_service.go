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
	"gorm.io/gorm"
)

// RoomService 房间服务
type RoomService struct {
	db                      *gorm.DB
	tokenGenerator          *livekit.TokenGenerator
	timeFormatter           *utils.TimeFormatter
	participantDeduplicator *utils.ParticipantDeduplicator
}

// NewRoomService 创建房间服务
func NewRoomService(db *gorm.DB, tokenGenerator *livekit.TokenGenerator) *RoomService {
	return &RoomService{
		db:                      db,
		tokenGenerator:          tokenGenerator,
		timeFormatter:           utils.NewTimeFormatter(),
		participantDeduplicator: utils.NewParticipantDeduplicator(),
	}
}

// CreateRoom 创建房间
func (rs *RoomService) CreateRoom(req *models.CreateRoomRequest) (*models.CreateRoomResponse, error) {
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

	// 3. 检查 source_channel_id 和 source_channel_type 是否存在正在通话的房间
	var activeRoom models.Room
	if err := rs.db.Where("source_channel_id = ? AND source_channel_type = ? AND (status = ? OR status = ?)",
		req.SourceChannelID, req.SourceChannelType, models.RoomStatusNotStarted, models.RoomStatusInProgress).
		First(&activeRoom).Error; err == nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.ChannelHasActiveRoom)
	} else if err != gorm.ErrRecordNotFound {
		return nil, errors.NewBusinessErrorWithKey(i18n.RoomQueryFailed, err.Error())
	}

	// 4. 检查 creator 是否在 rtc_participant 表存在 status=0/1 的情况
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

	// 6. 检查 UIDs 中的用户是否在通话中
	if len(deduplicatedUIDs) > 0 {
		var busyParticipant models.Participant
		if err := rs.db.Where("uid IN ? AND (status = ? OR status = ?)",
			deduplicatedUIDs, models.ParticipantStatusInviting, models.ParticipantStatusJoined).
			First(&busyParticipant).Error; err == nil {
			return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantInCall, busyParticipant.UID)
		} else if err != gorm.ErrRecordNotFound {
			return nil, errors.NewBusinessErrorWithKey(i18n.ParticipantQueryFailed, err.Error())
		}
	}

	// 设置 MaxParticipants，如果未传递则默认为 2
	maxParticipants := req.MaxParticipants
	if maxParticipants <= 0 {
		maxParticipants = 2
	}

	// 使用事务确保数据一致性
	err := rs.db.Transaction(func(tx *gorm.DB) error {
		// 创建房间
		room := models.Room{
			SourceChannelID:   req.SourceChannelID,
			SourceChannelType: int16(req.SourceChannelType),
			Creator:           req.Creator,
			RoomID:            roomID,
			RTCType:           int16(req.RTCType),
			InviteOn:          int16(req.InviteOn),
			Status:            models.RoomStatusNotStarted,
			MaxParticipants:   maxParticipants,
		}

		if err := tx.Create(&room).Error; err != nil {
			return errors.NewBusinessErrorWithKey(i18n.RoomCreationFailed, err.Error())
		}

		// 合并创建者和去重后的邀请用户到一个数组中
		participants := make([]models.Participant, 0)

		// 添加创建者
		participants = append(participants, models.Participant{
			RoomID: roomID,
			UID:    req.Creator,
			Status: models.ParticipantStatusInviting,
		})

		// 添加去重后的邀请用户
		if len(deduplicatedUIDs) > 0 {
			for _, uid := range deduplicatedUIDs {
				participants = append(participants, models.Participant{
					RoomID: roomID,
					UID:    uid,
					Status: models.ParticipantStatusInviting,
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

	// 生成 Token 和获取配置信息
	tokenResult, err := rs.tokenGenerator.GenerateTokenWithConfig(roomID, req.Creator)
	if err != nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.TokenGenerationFailed, err.Error())
	}

	return &models.CreateRoomResponse{
		RoomID:          roomID,
		Creator:         req.Creator,
		Token:           tokenResult.Token,
		URL:             tokenResult.URL,
		Status:          models.RoomStatusNotStarted,
		CreatedAt:       rs.timeFormatter.FormatDateTime(time.Now()),
		MaxParticipants: maxParticipants,
		Timeout:         tokenResult.Timeout,
	}, nil
}
