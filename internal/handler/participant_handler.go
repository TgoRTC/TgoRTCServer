package handler

import (
	"tgo-rtc-server/internal/errors"
	"tgo-rtc-server/internal/middleware"
	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/service"
	"tgo-rtc-server/internal/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ParticipantHandler 参与者处理器
type ParticipantHandler struct {
	participantService     *service.ParticipantService
	businessWebhookService *service.BusinessWebhookService
}

// NewParticipantHandler 创建参与者处理器
func NewParticipantHandler(participantService *service.ParticipantService) *ParticipantHandler {
	return &ParticipantHandler{
		participantService: participantService,
	}
}

// SetBusinessWebhookService 设置业务 webhook 服务
func (ph *ParticipantHandler) SetBusinessWebhookService(bws *service.BusinessWebhookService) {
	ph.businessWebhookService = bws
}

// JoinRoom 参与者加入房间
// POST /api/v1/rooms/:room_id/join
func (ph *ParticipantHandler) JoinRoom(c *gin.Context) {
	lang := middleware.GetLanguageFromContext(c)
	logger := utils.GetLogger()
	roomID := c.Param("room_id")

	var req models.JoinRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("加入房间参数绑定失败",
			zap.Error(err),
			zap.String("room_id", roomID),
			zap.String("uid", req.UID),
			zap.String("language", lang),
		)
		utils.RespondWithBindError(c)
		return
	}

	// 从 URL 参数中获取 room_id
	req.RoomID = roomID

	resp, err := ph.participantService.JoinRoom(&req)
	if err != nil {
		if businessErr, ok := err.(*errors.BusinessError); ok {
			logger.Warn("加入房间业务错误",
				zap.String("error_key", string(businessErr.Key)),
				zap.String("error_message", businessErr.GetLocalizedMessage(lang)),
				zap.String("room_id", req.RoomID),
				zap.String("uid", req.UID),
				zap.String("language", lang),
			)
		} else {
			logger.Error("加入房间系统错误",
				zap.Error(err),
				zap.String("room_id", req.RoomID),
				zap.String("uid", req.UID),
				zap.String("language", lang),
			)
		}
		utils.RespondWithBusinessError(c, err)
		return
	}

	// 记录参与者加入成功和 LiveKit 连接信息
	logger.Info("参与者加入房间成功",
		zap.String("room_id", resp.RoomID),
		zap.String("uid", req.UID),
		zap.String("creator", resp.Creator),
		zap.String("livekit_url", resp.URL),
		zap.Uint8("room_status", resp.Status),
		zap.String("language", lang),
	)

	// 发送业务 webhook 事件
	// if ph.businessWebhookService != nil && resp != nil {
	// 	eventData := &models.ParticipantEventData{
	// 		RoomEventData: models.RoomEventData{
	// 			SourceChannelID:   resp.SourceChannelID,
	// 			SourceChannelType: resp.SourceChannelType,
	// 			RoomID:            resp.RoomID,
	// 			Creator:           resp.Creator,
	// 			RTCType:           resp.RTCType,
	// 			InviteOn:          0, // 从 resp 中无法获取，设为默认值
	// 			Status:            resp.Status,
	// 			MaxParticipants:   resp.MaxParticipants,
	// 			CreatedAt:         0, // 从 resp 中无法获取，设为默认值
	// 			UpdatedAt:         0, // 从 resp 中无法获取，设为默认值
	// 		},
	// 		UID: req.UID, // 加入者 UID
	// 	}
	// 	_ = ph.businessWebhookService.SendEvent(models.BusinessEventParticipantJoined, eventData)
	// }

	utils.RespondWithData(c, resp)
}

// LeaveRoom 参与者离开房间
// POST /api/v1/rooms/:room_id/leave
func (ph *ParticipantHandler) LeaveRoom(c *gin.Context) {
	lang := middleware.GetLanguageFromContext(c)
	logger := utils.GetLogger()
	roomID := c.Param("room_id")

	var req models.LeaveRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("离开房间参数绑定失败",
			zap.Error(err),
			zap.String("room_id", roomID),
			zap.String("language", lang),
		)
		utils.RespondWithBindError(c)
		return
	}

	// 从 URL 参数中获取 room_id
	req.RoomID = roomID

	if err := ph.participantService.LeaveRoom(&req); err != nil {
		if businessErr, ok := err.(*errors.BusinessError); ok {
			logger.Warn("离开房间业务错误",
				zap.String("error_key", string(businessErr.Key)),
				zap.String("error_message", businessErr.GetLocalizedMessage(lang)),
				zap.String("room_id", req.RoomID),
				zap.String("uid", req.UID),
				zap.String("language", lang),
			)
		} else {
			logger.Error("离开房间系统错误",
				zap.Error(err),
				zap.String("room_id", req.RoomID),
				zap.String("uid", req.UID),
			)
		}
		utils.RespondWithBusinessError(c, err)
		return
	}

	utils.RespondWithData(c, nil)
}

// InviteParticipants 邀请参与者
// POST /api/rooms/:room_id/invite
func (ph *ParticipantHandler) InviteParticipants(c *gin.Context) {
	lang := middleware.GetLanguageFromContext(c)
	logger := utils.GetLogger()
	roomID := c.Param("room_id")

	var req models.InviteParticipantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("邀请参与者参数绑定失败",
			zap.Error(err),
			zap.String("room_id", roomID),
			zap.String("language", lang),
		)
		utils.RespondWithBindError(c)
		return
	}

	req.RoomID = roomID

	if err := ph.participantService.InviteParticipants(&req); err != nil {
		if businessErr, ok := err.(*errors.BusinessError); ok {
			logger.Warn("邀请参与者业务错误",
				zap.String("error_key", string(businessErr.Key)),
				zap.String("error_message", businessErr.GetLocalizedMessage(lang)),
				zap.String("room_id", roomID),
				zap.Strings("uids", req.UIDs),
				zap.String("language", lang),
			)
		} else {
			logger.Error("邀请参与者系统错误",
				zap.Error(err),
				zap.String("room_id", roomID),
				zap.Strings("uids", req.UIDs),
				zap.String("language", lang),
			)
		}
		utils.RespondWithBusinessError(c, err)
		return
	}

	utils.RespondWithData(c, nil)
}

// GetUserAvailableRooms 同步用户可加入的房间列表
// GET /api/v1/rooms/sync?uid=xxx
func (ph *ParticipantHandler) GetUserAvailableRooms(c *gin.Context) {
	lang := middleware.GetLanguageFromContext(c)
	logger := utils.GetLogger()

	// 从 query 参数获取 uid
	uid := c.Query("uid")
	if uid == "" {
		logger.Error("获取用户可加入房间列表参数缺失",
			zap.String("language", lang),
		)
		utils.RespondWithBindError(c)
		return
	}

	data, err := ph.participantService.GetUserAvailableRooms(uid)
	if err != nil {
		logger.Error("获取用户可加入房间列表失败",
			zap.Error(err),
			zap.String("uid", uid),
		)
		utils.RespondWithBusinessError(c, err)
		return
	}

	// 如果没有可加入的房间，返回空数组而不是 nil
	if len(data) == 0 {
		data = []models.RoomResp{}
	}

	// 直接返回数组，不包装在 data 节点中
	utils.RespondWithData(c, data)
}
