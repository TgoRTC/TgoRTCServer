package handler

import (
	"net/http"

	"tgo-call-server/internal/errors"
	"tgo-call-server/internal/i18n"
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
			zap.String("language", lang),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  i18n.Translate(lang, i18n.InvalidParameters),
		})
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
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  i18n.Translate(lang, i18n.InvalidParameters),
		})
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
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  businessErr.GetLocalizedMessage(lang),
			})
		} else {
			logger.Error("离开房间系统错误",
				zap.Error(err),
				zap.String("room_id", req.RoomID),
				zap.String("uid", req.UID),
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
			"msg":  i18n.Translate(lang, i18n.InvalidParameters),
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

// CheckUserCallStatus 查询正在通话的成员
// POST /api/v1/participants/calling
func (ph *ParticipantHandler) CheckUserCallStatus(c *gin.Context) {
	lang := middleware.GetLanguageFromContext(c)
	logger := utils.GetLogger()

	var req models.CheckUserCallStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("查询正在通话成员参数绑定失败",
			zap.Error(err),
			zap.String("language", lang),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  i18n.Translate(lang, i18n.InvalidParameters),
		})
		return
	}

	uids, err := ph.participantService.CheckUserCallStatus(req.UIDs)
	if err != nil {
		logger.Error("查询正在通话成员系统错误",
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
	if len(uids) == 0 {
		uids = []string{}
	}

	c.JSON(http.StatusOK, models.CheckUserCallStatusResponse{
		UIDs: uids,
	})
}
