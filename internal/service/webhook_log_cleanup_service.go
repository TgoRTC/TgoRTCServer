package service

import (
	"time"

	"tgo-rtc-server/internal/config"
	"tgo-rtc-server/internal/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// WebhookLogCleanupService webhook 日志清理服务
type WebhookLogCleanupService struct {
	db     *gorm.DB
	config *config.Config
	ticker *time.Ticker
	done   chan bool
}

// NewWebhookLogCleanupService 创建 webhook 日志清理服务
func NewWebhookLogCleanupService(db *gorm.DB, cfg *config.Config) *WebhookLogCleanupService {
	return &WebhookLogCleanupService{
		db:     db,
		config: cfg,
		done:   make(chan bool),
	}
}

// Start 启动日志清理定时器
func (wlcs *WebhookLogCleanupService) Start() {
	logger := utils.GetLogger()

	// 检查是否启用日志清理
	if !wlcs.config.BusinessWebhookLogCleanupEnabled {
		logger.Info("webhook 日志清理已禁用")
		return
	}

	// 立即执行一次清理
	wlcs.cleanup()

	// 启动定时器
	interval := time.Duration(wlcs.config.BusinessWebhookLogCleanupInterval) * time.Second
	wlcs.ticker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-wlcs.ticker.C:
				wlcs.cleanup()
			case <-wlcs.done:
				return
			}
		}
	}()

	logger.Info("webhook 日志清理定时器已启动",
		zap.Int("cleanup_interval_seconds", wlcs.config.BusinessWebhookLogCleanupInterval),
		zap.Int("retention_days", wlcs.config.BusinessWebhookLogRetentionDays),
	)
}

// Stop 停止日志清理定时器
func (wlcs *WebhookLogCleanupService) Stop() {
	logger := utils.GetLogger()

	if wlcs.ticker != nil {
		wlcs.ticker.Stop()
		wlcs.done <- true
		logger.Info("webhook 日志清理定时器已停止")
	}
}

// cleanup 执行清理操作
func (wlcs *WebhookLogCleanupService) cleanup() {
	logger := utils.GetLogger()

	// 计算截断时间
	cutoffTime := time.Now().AddDate(0, 0, -wlcs.config.BusinessWebhookLogRetentionDays)

	// 删除旧日志（使用原始 SQL）
	result := wlcs.db.Exec("DELETE FROM business_webhook_log WHERE created_at < ?", cutoffTime)
	if result.Error != nil {
		logger.Error("清理 webhook 日志失败",
			zap.Error(result.Error),
		)
		return
	}

	if result.RowsAffected > 0 {
		logger.Info("webhook 日志清理完成",
			zap.Int64("deleted_count", result.RowsAffected),
			zap.Time("cutoff_time", cutoffTime),
		)
	}
}
