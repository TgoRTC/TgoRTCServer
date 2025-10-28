package handler

import (
	"net/http"
	"strconv"

	"tgo-call-server/internal/errors"
	"tgo-call-server/internal/middleware"
	"tgo-call-server/internal/models"
	"tgo-call-server/internal/service"
	"tgo-call-server/internal/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RoomHandler 房间处理器
type RoomHandler struct {
	roomService *service.RoomService
}

// NewRoomHandler 创建房间处理器
func NewRoomHandler(roomService *service.RoomService) *RoomHandler {
	return &RoomHandler{
		roomService: roomService,
	}
}

// CreateRoom 创建房间
// POST /api/rooms
func (rh *RoomHandler) CreateRoom(c *gin.Context) {
	lang := middleware.GetLanguageFromContext(c)
	logger := utils.GetLogger()

	var req models.CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("创建房间参数绑定失败",
			zap.Error(err),
			zap.String("language", lang),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	resp, err := rh.roomService.CreateRoom(&req)
	if err != nil {
		if businessErr, ok := err.(*errors.BusinessError); ok {
			logger.Warn("创建房间业务错误",
				zap.String("error_key", string(businessErr.Key)),
				zap.String("error_message", businessErr.GetLocalizedMessage(lang)),
				zap.String("creator", req.Creator),
				zap.String("source_channel_id", req.SourceChannelID),
				zap.String("language", lang),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  businessErr.GetLocalizedMessage(lang),
			})
		} else {
			logger.Error("创建房间系统错误",
				zap.Error(err),
				zap.String("creator", req.Creator),
				zap.String("source_channel_id", req.SourceChannelID),
				zap.String("language", lang),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// GetRoom 获取房间信息
// GET /api/rooms/:room_id
func (rh *RoomHandler) GetRoom(c *gin.Context) {
	lang := middleware.GetLanguageFromContext(c)
	logger := utils.GetLogger()
	roomID := c.Param("room_id")

	resp, err := rh.roomService.GetRoom(roomID)
	if err != nil {
		if businessErr, ok := err.(*errors.BusinessError); ok {
			logger.Warn("获取房间业务错误",
				zap.String("error_key", string(businessErr.Key)),
				zap.String("error_message", businessErr.GetLocalizedMessage(lang)),
				zap.String("room_id", roomID),
				zap.String("language", lang),
			)
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  businessErr.GetLocalizedMessage(lang),
			})
		} else {
			logger.Error("获取房间系统错误",
				zap.Error(err),
				zap.String("room_id", roomID),
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

// UpdateRoomStatus 更新房间状态
// PUT /api/rooms/:room_id/status
func (rh *RoomHandler) UpdateRoomStatus(c *gin.Context) {
	logger := utils.GetLogger()
	roomID := c.Param("room_id")

	var req models.UpdateRoomStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("更新房间状态参数绑定失败",
			zap.Error(err),
			zap.String("room_id", roomID),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	if err := rh.roomService.UpdateRoomStatus(roomID, int16(req.Status)); err != nil {
		logger.Error("更新房间状态系统错误",
			zap.Error(err),
			zap.String("room_id", roomID),
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

// EndRoom 结束房间
// POST /api/rooms/:room_id/end
func (rh *RoomHandler) EndRoom(c *gin.Context) {
	roomID := c.Param("room_id")

	if err := rh.roomService.EndRoom(roomID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ListRooms 列出房间列表
// GET /api/rooms?limit=10&offset=0
func (rh *RoomHandler) ListRooms(c *gin.Context) {
	limit := 10
	offset := 0

	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	rooms, total, err := rh.roomService.ListRooms(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rooms": rooms,
		"total": total,
	})
}
