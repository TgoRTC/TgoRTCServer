package handler

import (
	"net/http"

	"tgo-call-server/internal/errors"
	"tgo-call-server/internal/middleware"
	"tgo-call-server/internal/models"
	"tgo-call-server/internal/service"
	"tgo-call-server/internal/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ParticipantHandler 参与者处理器
type ParticipantHandler struct {
	participantService *service.ParticipantService
}

// NewParticipantHandler 创建参与者处理器
func NewParticipantHandler(participantService *service.ParticipantService) *ParticipantHandler {
	return &ParticipantHandler{
		participantService: participantService,
	}
}

// JoinRoom 参与者加入房间
// POST /api/participants/join
func (ph *ParticipantHandler) JoinRoom(c *gin.Context) {
	lang := middleware.GetLanguageFromContext(c)
	logger := utils.GetLogger()

	var req models.JoinRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("加入房间参数绑定失败",
			zap.Error(err),
			zap.String("language", lang),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

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
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  businessErr.GetLocalizedMessage(lang),
			})
		} else {
			logger.Error("加入房间系统错误",
				zap.Error(err),
				zap.String("room_id", req.RoomID),
				zap.String("uid", req.UID),
				zap.String("language", lang),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// LeaveRoom 参与者离开房间
// POST /api/participants/leave
func (ph *ParticipantHandler) LeaveRoom(c *gin.Context) {
	logger := utils.GetLogger()

	var req models.LeaveRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("离开房间参数绑定失败",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	if err := ph.participantService.LeaveRoom(&req); err != nil {
		logger.Error("离开房间系统错误",
			zap.Error(err),
			zap.String("room_id", req.RoomID),
			zap.String("uid", req.UID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// GetParticipants 获取房间内的参与者列表
// GET /api/rooms/:room_id/participants
func (ph *ParticipantHandler) GetParticipants(c *gin.Context) {
	roomID := c.Param("room_id")

	participants, err := ph.participantService.GetParticipants(roomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, participants)
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
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
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
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  businessErr.GetLocalizedMessage(lang),
			})
		} else {
			logger.Error("邀请参与者系统错误",
				zap.Error(err),
				zap.String("room_id", roomID),
				zap.Strings("uids", req.UIDs),
				zap.String("language", lang),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// UpdateParticipantStatus 更新参与者状态
// PUT /api/rooms/:room_id/participants/:uid/status
func (ph *ParticipantHandler) UpdateParticipantStatus(c *gin.Context) {
	logger := utils.GetLogger()
	roomID := c.Param("room_id")
	uid := c.Param("uid")

	var req models.UpdateParticipantStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("更新参与者状态参数绑定失败",
			zap.Error(err),
			zap.String("room_id", roomID),
			zap.String("uid", uid),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	if err := ph.participantService.UpdateParticipantStatus(roomID, uid, int16(req.Status)); err != nil {
		logger.Error("更新参与者状态系统错误",
			zap.Error(err),
			zap.String("room_id", roomID),
			zap.String("uid", uid),
			zap.Int("status", req.Status),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// CheckUserCallStatus 检查用户是否正在通话
// POST /api/participants/check-call-status
func (ph *ParticipantHandler) CheckUserCallStatus(c *gin.Context) {
	logger := utils.GetLogger()

	var req models.CheckUserCallStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("检查用户通话状态参数绑定失败",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	uids, err := ph.participantService.CheckUserCallStatus(req.UIDs)
	if err != nil {
		logger.Error("检查用户通话状态系统错误",
			zap.Error(err),
			zap.Strings("uids", req.UIDs),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	// 如果没有正在通话的用户，返回空数组而不是 nil
	if uids == nil {
		uids = []string{}
	}

	c.JSON(http.StatusOK, models.CheckUserCallStatusResponse{
		UIDs: uids,
	})
}
