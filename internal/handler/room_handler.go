package handler

import (
	"net/http"
	"time"

	"tgo-rtc-server/internal/errors"
	"tgo-rtc-server/internal/i18n"
	"tgo-rtc-server/internal/middleware"
	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/service"
	"tgo-rtc-server/internal/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RoomHandler 房间处理器
type RoomHandler struct {
	roomService            *service.RoomService
	businessWebhookService *service.BusinessWebhookService
}

// NewRoomHandler 创建房间处理器
func NewRoomHandler(roomService *service.RoomService) *RoomHandler {
	return &RoomHandler{
		roomService: roomService,
	}
}

// SetBusinessWebhookService 设置业务 webhook 服务
func (rh *RoomHandler) SetBusinessWebhookService(bws *service.BusinessWebhookService) {
	rh.businessWebhookService = bws
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
			"msg":  i18n.Translate(lang, i18n.InvalidParameters),
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

	// 发送业务 webhook 事件
	if rh.businessWebhookService != nil && resp != nil {
		eventData := &models.RoomEventData{
			RoomID:          resp.RoomID,
			Creator:         resp.Creator,
			Status:          int(resp.Status),
			MaxParticipants: resp.MaxParticipants,
			CreatedAt:       time.Now().Unix(),
			UpdatedAt:       time.Now().Unix(),
		}
		_ = rh.businessWebhookService.SendEvent(models.BusinessEventRoomCreated, eventData)
	}

	c.JSON(http.StatusOK, resp)
}
