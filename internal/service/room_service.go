package service

import (
	"fmt"

	"tgo-call-server/internal/errors"
	"tgo-call-server/internal/i18n"
	"tgo-call-server/internal/livekit"
	"tgo-call-server/internal/models"
	"tgo-call-server/internal/utils"

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
	// 1. 如果 room_id 没有传递，则生成 UUID
	roomID := req.RoomID
	if roomID == "" {
		roomID = uuid.New().String()
	} else {
		// 2. 如果 room_id 已传递，检查是否已存在
		var existingRoom models.Room
		if err := rs.db.Where("room_id = ?", roomID).First(&existingRoom).Error; err == nil {
			return nil, errors.NewBusinessErrorWithKey(i18n.RoomAlreadyExists, roomID)
		} else if err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("查询房间失败: %w", err)
		}
	}

	// 3. 检查 source_channel_id 和 source_channel_type 是否存在正在通话的房间
	var activeRoom models.Room
	if err := rs.db.Where("source_channel_id = ? AND source_channel_type = ? AND (status = ? OR status = ?)",
		req.SourceChannelID, req.SourceChannelType, models.RoomStatusNotStarted, models.RoomStatusInProgress).
		First(&activeRoom).Error; err == nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.ChannelHasActiveRoom)
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("查询房间失败: %w", err)
	}

	// 4. 检查 creator 是否在 call_participant 表存在 status=0/1 的情况
	var participant models.Participant
	if err := rs.db.Where("uid = ? AND (status = ? OR status = ?)",
		req.Creator, models.ParticipantStatusInviting, models.ParticipantStatusJoined).
		First(&participant).Error; err == nil {
		return nil, errors.NewBusinessErrorWithKey(i18n.CreatorInAnotherCall)
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("查询参与者失败: %w", err)
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
			return nil, fmt.Errorf("查询参与者失败: %w", err)
		}
	}

	// 设置 MaxParticipants，如果未传递则默认为 2
	maxParticipants := req.MaxParticipants
	if maxParticipants <= 0 {
		maxParticipants = 2
	}

	// 使用事务确保数据一致性
	tx := rs.db.Begin()

	// 创建房间
	room := models.Room{
		SourceChannelID:   req.SourceChannelID,
		SourceChannelType: int16(req.SourceChannelType),
		Creator:           req.Creator,
		RoomID:            roomID,
		CallType:          int16(req.CallType),
		InviteOn:          int16(req.InviteOn),
		Status:            models.RoomStatusNotStarted,
		MaxParticipants:   maxParticipants,
	}

	if err := tx.Create(&room).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建房间失败: %w", err)
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
		tx.Rollback()
		return nil, fmt.Errorf("添加参与者记录失败: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	// 生成 Token
	token, err := rs.tokenGenerator.GenerateToken(roomID, req.Creator)
	if err != nil {
		return nil, fmt.Errorf("生成 Token 失败: %w", err)
	}

	return &models.CreateRoomResponse{
		RoomID:          roomID,
		Token:           token,
		URL:             "http://localhost:7880", // 应该从配置读取
		Status:          room.Status,
		CreatedAt:       rs.timeFormatter.FormatDateTime(room.CreatedAt),
		MaxParticipants: maxParticipants,
		Timeout:         3600, // 默认超时时间（秒）
	}, nil
}

// GetRoom 获取房间信息
func (rs *RoomService) GetRoom(roomID string) (*models.GetRoomResponse, error) {
	var room models.Room
	if err := rs.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.NewBusinessErrorWithKey(i18n.RoomNotFound, roomID)
		}
		return nil, fmt.Errorf("查询房间失败: %w", err)
	}

	return &models.GetRoomResponse{
		ID:                room.ID,
		SourceChannelID:   room.SourceChannelID,
		SourceChannelType: room.SourceChannelType,
		Creator:           room.Creator,
		RoomID:            room.RoomID,
		CallType:          room.CallType,
		InviteOn:          room.InviteOn,
		Status:            room.Status,
		CreatedAt:         rs.timeFormatter.FormatDateTime(room.CreatedAt),
		UpdatedAt:         rs.timeFormatter.FormatDateTime(room.UpdatedAt),
	}, nil
}

// UpdateRoomStatus 更新房间状态
func (rs *RoomService) UpdateRoomStatus(roomID string, status int16) error {
	if err := rs.db.Model(&models.Room{}).Where("room_id = ?", roomID).Update("status", status).Error; err != nil {
		return fmt.Errorf("更新房间状态失败: %w", err)
	}
	return nil
}

// EndRoom 结束房间
func (rs *RoomService) EndRoom(roomID string) error {
	return rs.UpdateRoomStatus(roomID, models.RoomStatusFinished)
}

// ListRooms 列出房间列表
func (rs *RoomService) ListRooms(limit, offset int) ([]models.GetRoomResponse, int64, error) {
	var rooms []models.Room
	var total int64

	if err := rs.db.Model(&models.Room{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询房间总数失败: %w", err)
	}

	if err := rs.db.Limit(limit).Offset(offset).Find(&rooms).Error; err != nil {
		return nil, 0, fmt.Errorf("查询房间列表失败: %w", err)
	}

	var responses []models.GetRoomResponse
	for _, room := range rooms {
		responses = append(responses, models.GetRoomResponse{
			ID:                room.ID,
			SourceChannelID:   room.SourceChannelID,
			SourceChannelType: room.SourceChannelType,
			Creator:           room.Creator,
			RoomID:            room.RoomID,
			CallType:          room.CallType,
			InviteOn:          room.InviteOn,
			Status:            room.Status,
			CreatedAt:         rs.timeFormatter.FormatDateTime(room.CreatedAt),
			UpdatedAt:         rs.timeFormatter.FormatDateTime(room.UpdatedAt),
		})
	}

	return responses, total, nil
}
