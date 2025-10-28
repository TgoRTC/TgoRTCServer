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
