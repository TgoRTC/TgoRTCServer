package handler

import (
	"tgo-rtc-server/internal/livekit"
	"tgo-rtc-server/internal/service"
	"tgo-rtc-server/internal/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WebhookHandler webhook 处理器
type WebhookHandler struct {
	webhookService   *service.WebhookService
	webhookValidator *livekit.WebhookValidator
}

// NewWebhookHandler 创建 webhook 处理器
func NewWebhookHandler(webhookService *service.WebhookService, webhookValidator *livekit.WebhookValidator) *WebhookHandler {
	return &WebhookHandler{
		webhookService:   webhookService,
		webhookValidator: webhookValidator,
	}
}

// HandleWebhook 处理 webhook 请求
// POST /api/v1/webhooks/livekit
func (wh *WebhookHandler) HandleWebhook(c *gin.Context) {
	logger := utils.GetLogger()

	// 验证请求
	body, err := wh.webhookValidator.ValidateRequest(c.Request)
	if err != nil {
		logger.Warn("webhook 验证失败",
			zap.Error(err),
			zap.String("remote_addr", c.RemoteIP()),
		)
		utils.RespondUnauthorized(c, "验证失败")
		return
	}

	// 解析事件
	event, err := service.ParseWebhookEvent(body)
	if err != nil {
		logger.Error("解析 webhook 事件失败",
			zap.Error(err),
			zap.String("remote_addr", c.RemoteIP()),
		)
		utils.RespondBadRequest(c, "解析事件失败")
		return
	}

	// 处理事件
	if err := wh.webhookService.HandleWebhookEvent(event); err != nil {
		logger.Error("处理 webhook 事件失败",
			zap.Error(err),
			zap.String("event_type", event.Event),
			zap.String("event_id", event.ID),
		)
		utils.RespondInternalError(c, "处理事件失败")
		return
	}

	logger.Info("webhook 事件处理成功",
		zap.String("event_type", event.Event),
		zap.String("event_id", event.ID),
	)

	utils.RespondWithSuccess(c, "ok")
}
