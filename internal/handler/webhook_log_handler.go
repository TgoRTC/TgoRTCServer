package handler

import (
	"net/http"

	"tgo-rtc-server/internal/service"
	"tgo-rtc-server/internal/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WebhookLogHandler webhook 日志处理器
type WebhookLogHandler struct {
	businessWebhookService *service.BusinessWebhookService
}

// NewWebhookLogHandler 创建 webhook 日志处理器
func NewWebhookLogHandler(businessWebhookService *service.BusinessWebhookService) *WebhookLogHandler {
	return &WebhookLogHandler{
		businessWebhookService: businessWebhookService,
	}
}

// GetLogStats 获取 webhook 日志统计信息
// GET /api/v1/webhooks/logs/stats
func (wlh *WebhookLogHandler) GetLogStats(c *gin.Context) {
	logger := utils.GetLogger()

	stats, err := wlh.businessWebhookService.GetLogStats()
	if err != nil {
		logger.Error("获取 webhook 日志统计失败",
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "获取统计信息失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "ok",
		"data": stats,
	})
}

// CleanupLogs 手动清理 webhook 日志
// POST /api/v1/webhooks/logs/cleanup
func (wlh *WebhookLogHandler) CleanupLogs(c *gin.Context) {
	logger := utils.GetLogger()

	var req struct {
		RetentionDays int `json:"retention_days" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("清理日志参数绑定失败",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误",
		})
		return
	}

	if err := wlh.businessWebhookService.CleanupOldLogs(req.RetentionDays); err != nil {
		logger.Error("清理 webhook 日志失败",
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "清理日志失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "日志清理成功",
	})
}

