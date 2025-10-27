package service

import (
	"fmt"
	"time"

	"tgo-call-server/internal/errors"
	"tgo-call-server/internal/livekit"
	"tgo-call-server/internal/models"
	"tgo-call-server/internal/utils"

	"gorm.io/gorm"
)

// ParticipantService 参与者服务
type ParticipantService struct {
	db             *gorm.DB
	tokenGenerator *livekit.TokenGenerator
	timeFormatter  *utils.TimeFormatter
}

// NewParticipantService 创建参与者服务
func NewParticipantService(db *gorm.DB, tokenGenerator *livekit.TokenGenerator) *ParticipantService {
	return &ParticipantService{
		db:             db,
		tokenGenerator: tokenGenerator,
		timeFormatter:  utils.NewTimeFormatter(),
	}
}

// JoinRoom 参与者加入房间
func (ps *ParticipantService) JoinRoom(req *models.JoinRoomRequest) (*models.JoinRoomResponse, error) {
	// 检查房间是否存在
	var room models.Room
	if err := ps.db.Where("room_id = ?", req.RoomID).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.NewBusinessErrorf("房间不存在: %s", req.RoomID)
		}
		return nil, fmt.Errorf("查询房间失败: %w", err)
	}

	// 检查参与者是否已存在
	var existingParticipant models.Participant
	if err := ps.db.Where("room_id = ? AND uid = ?", req.RoomID, req.UID).First(&existingParticipant).Error; err == nil {
		// 参与者已存在，更新状态为已加入
		if err := ps.db.Model(&existingParticipant).Update("status", models.ParticipantStatusJoined).Update("join_time", time.Now().UnixMilli()).Error; err != nil {
			return nil, fmt.Errorf("更新参与者状态失败: %w", err)
		}
	} else if err == gorm.ErrRecordNotFound {
		// 创建新的参与者记录
		participant := models.Participant{
			RoomID:   req.RoomID,
			UID:      req.UID,
			Status:   models.ParticipantStatusJoined,
			JoinTime: time.Now().UnixMilli(),
		}
		if err := ps.db.Create(&participant).Error; err != nil {
			return nil, fmt.Errorf("创建参与者记录失败: %w", err)
		}
	} else {
		return nil, fmt.Errorf("查询参与者失败: %w", err)
	}

	// 生成 Token
	token, err := ps.tokenGenerator.GenerateToken(req.RoomID, req.UID)
	if err != nil {
		return nil, fmt.Errorf("生成 Token 失败: %w", err)
	}

	return &models.JoinRoomResponse{
		RoomID: req.RoomID,
		UID:    req.UID,
		Token:  token,
		Status: models.ParticipantStatusJoined,
	}, nil
}

// LeaveRoom 参与者离开房间
func (ps *ParticipantService) LeaveRoom(req *models.LeaveRoomRequest) error {
	if err := ps.db.Model(&models.Participant{}).
		Where("room_id = ? AND uid = ?", req.RoomID, req.UID).
		Updates(map[string]interface{}{
			"status":     models.ParticipantStatusHangup,
			"leave_time": time.Now().UnixMilli(),
		}).Error; err != nil {
		return fmt.Errorf("更新参与者状态失败: %w", err)
	}
	return nil
}

// GetParticipants 获取房间内的参与者列表
func (ps *ParticipantService) GetParticipants(roomID string) ([]models.GetParticipantsResponse, error) {
	var participants []models.Participant
	if err := ps.db.Where("room_id = ?", roomID).Find(&participants).Error; err != nil {
		return nil, fmt.Errorf("查询参与者列表失败: %w", err)
	}

	var responses []models.GetParticipantsResponse
	for _, p := range participants {
		responses = append(responses, models.GetParticipantsResponse{
			ID:        p.ID,
			RoomID:    p.RoomID,
			UID:       p.UID,
			Status:    p.Status,
			JoinTime:  p.JoinTime,
			LeaveTime: p.LeaveTime,
			CreatedAt: ps.timeFormatter.FormatDateTime(p.CreatedAt),
			UpdatedAt: ps.timeFormatter.FormatDateTime(p.UpdatedAt),
		})
	}

	return responses, nil
}

// InviteParticipants 邀请参与者
func (ps *ParticipantService) InviteParticipants(req *models.InviteParticipantRequest) error {
	// 检查房间是否存在
	var room models.Room
	if err := ps.db.Where("room_id = ?", req.RoomID).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.NewBusinessErrorf("房间不存在: %s", req.RoomID)
		}
		return fmt.Errorf("查询房间失败: %w", err)
	}

	// 为每个 UID 创建参与者记录
	for _, uid := range req.UIDs {
		participant := models.Participant{
			RoomID: req.RoomID,
			UID:    uid,
			Status: models.ParticipantStatusInviting,
		}
		if err := ps.db.Create(&participant).Error; err != nil {
			return fmt.Errorf("创建参与者记录失败: %w", err)
		}
	}

	return nil
}

// UpdateParticipantStatus 更新参与者状态
func (ps *ParticipantService) UpdateParticipantStatus(roomID, uid string, status int16) error {
	if err := ps.db.Model(&models.Participant{}).
		Where("room_id = ? AND uid = ?", roomID, uid).
		Update("status", status).Error; err != nil {
		return fmt.Errorf("更新参与者状态失败: %w", err)
	}
	return nil
}
