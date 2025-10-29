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
	if room.Status == models.RoomStatusFinished || room.Status == models.RoomStatusCancelled {

	}
	// 检查参与者是否已存在
	var existingParticipant models.Participant
	if err := ps.db.Where("room_id = ? AND uid = ?", req.RoomID, req.UID).First(&existingParticipant).Error; err == nil {
		// 参与者已存在，更新状态为已加入
		if err := ps.db.Model(&existingParticipant).Update("status", models.ParticipantStatusJoined).Update("join_time", time.Now().Unix()).Error; err != nil {
			return nil, fmt.Errorf("更新参与者状态失败: %w", err)
		}
	} else if err == gorm.ErrRecordNotFound {
		// 创建新的参与者记录
		participant := models.Participant{
			RoomID:   req.RoomID,
			UID:      req.UID,
			Status:   models.ParticipantStatusJoined,
			JoinTime: time.Now().Unix(),
		}
		if err := ps.db.Create(&participant).Error; err != nil {
			return nil, fmt.Errorf("创建参与者记录失败: %w", err)
		}
	} else {
		return nil, fmt.Errorf("查询参与者失败: %w", err)
	}

	// 生成 Token 和获取配置信息
	tokenResult, err := ps.tokenGenerator.GenerateTokenWithConfig(req.RoomID, req.UID)
	if err != nil {
		return nil, fmt.Errorf("生成 Token 失败: %w", err)
	}

	return &models.JoinRoomResponse{
		RoomID:          req.RoomID,
		Creator:         room.Creator,
		Token:           tokenResult.Token,
		URL:             tokenResult.URL,
		Status:          room.Status,
		CreatedAt:       ps.timeFormatter.FormatDateTime(room.CreatedAt),
		MaxParticipants: room.MaxParticipants,
		Timeout:         tokenResult.Timeout,
	}, nil
}

// LeaveRoom 参与者离开房间
func (ps *ParticipantService) LeaveRoom(req *models.LeaveRoomRequest) error {
	// 检查房间是否存在
	var room models.Room
	if err := ps.db.Where("room_id = ?", req.RoomID).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.NewBusinessErrorf("房间不存在: %s", req.RoomID)
		}
		return fmt.Errorf("查询房间失败: %w", err)
	}

	if err := ps.db.Model(&models.Participant{}).
		Where("room_id = ? AND uid = ?", req.RoomID, req.UID).
		Updates(map[string]interface{}{
			"status":     models.ParticipantStatusHangup,
			"leave_time": time.Now().Unix(),
		}).Error; err != nil {
		return fmt.Errorf("更新参与者状态失败: %w", err)
	}
	return nil
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

// CheckUserCallStatus 检查用户是否正在通话
// 只查询 status=0(邀请中) 或 status=1(已加入) 的数据
func (ps *ParticipantService) CheckUserCallStatus(uids []string) ([]string, error) {
	if len(uids) == 0 {
		return []string{}, nil
	}

	var participants []models.Participant
	// 查询 status=0(邀请中) 或 status=1(已加入) 的参与者
	if err := ps.db.Where("uid IN ? AND (status = ? OR status = ?)", uids, models.ParticipantStatusInviting, models.ParticipantStatusJoined).
		Find(&participants).Error; err != nil {
		return nil, fmt.Errorf("查询用户通话状态失败: %w", err)
	}

	// 提取 UID 列表，去重
	uidMap := make(map[string]bool)
	for _, p := range participants {
		uidMap[p.UID] = true
	}

	result := make([]string, 0, len(uidMap))
	for uid := range uidMap {
		result = append(result, uid)
	}

	return result, nil
}
