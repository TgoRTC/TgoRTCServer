package handler

import (
	"net/http"

	"tgo-call-server/internal/errors"
	"tgo-call-server/internal/middleware"
	"tgo-call-server/internal/models"
	"tgo-call-server/internal/service"

	"github.com/gin-gonic/gin"
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

	var req models.JoinRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	resp, err := ph.participantService.JoinRoom(&req)
	if err != nil {
		if businessErr, ok := err.(*errors.BusinessError); ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  businessErr.GetLocalizedMessage(lang),
			})
		} else {
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
	var req models.LeaveRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	if err := ph.participantService.LeaveRoom(&req); err != nil {
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
	roomID := c.Param("room_id")

	var req models.InviteParticipantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	req.RoomID = roomID

	if err := ph.participantService.InviteParticipants(&req); err != nil {
		if businessErr, ok := err.(*errors.BusinessError); ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  businessErr.GetLocalizedMessage(lang),
			})
		} else {
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
	roomID := c.Param("room_id")
	uid := c.Param("uid")

	var req models.UpdateParticipantStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	if err := ph.participantService.UpdateParticipantStatus(roomID, uid, int16(req.Status)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
