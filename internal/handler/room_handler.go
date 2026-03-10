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
		utils.RespondWithBindError(c)
		return
	}
	if req.DeviceType == "" {
		logger.Error("创建房间参数 device_type 缺失",
			zap.String("language", lang),
		)
		utils.RespondWithBindError(c)
		return
	}
	resp, err := rh.roomService.CreateRoom(&req)
	if err != nil {
		if businessErr, ok := err.(*errors.BusinessError); ok {
			logger.Warn("创建房间业务错误",
				zap.String("error_key", string(businessErr.Key)),
				zap.String("error_message", businessErr.GetLocalizedMessage(lang)),
				zap.Int("error_code", businessErr.GetErrorCode()),
				zap.String("creator", req.Creator),
				zap.String("language", lang),
			)
		} else {
			logger.Error("创建房间系统错误",
				zap.Error(err),
				zap.String("creator", req.Creator),
				zap.String("language", lang),
			)
		}
		utils.RespondWithBusinessError(c, err)
		return
	}

	utils.RespondWithData(c, resp)
}
