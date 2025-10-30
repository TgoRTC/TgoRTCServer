package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"tgo-call-server/internal/config"
	"tgo-call-server/internal/models"
	"tgo-call-server/internal/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BusinessWebhookService 业务 webhook 服务
type BusinessWebhookService struct {
	db     *gorm.DB
	config *config.Config
	client *http.Client
}

// NewBusinessWebhookService 创建业务 webhook 服务
func NewBusinessWebhookService(db *gorm.DB, cfg *config.Config) *BusinessWebhookService {
	return &BusinessWebhookService{
		db:     db,
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.BusinessWebhookTimeout) * time.Second,
		},
	}
}

// SendEvent 发送业务 webhook 事件
func (bws *BusinessWebhookService) SendEvent(eventType string, data interface{}) error {
	logger := utils.GetLogger()

	// 检查是否配置了业务 webhook URL（如果没有配置则不发送）
	if bws.config.BusinessWebhookURL == "" {
		return nil
	}

	// 创建事件
	event := &models.BusinessWebhookEvent{
		EventType: eventType,
		EventID:   generateEventID(),
		Timestamp: time.Now().Unix(),
		Data:      data,
		Retry:     0,
	}

	// 序列化事件
	payload, err := json.Marshal(data)
	if err != nil {
		logger.Error("序列化 webhook 事件失败",
			zap.String("event_type", eventType),
			zap.Error(err),
		)
		return err
	}

	// 异步发送到配置的 URL
	go bws.sendToURL(bws.config.BusinessWebhookURL, event, payload)

	return nil
}

// sendToURL 发送事件到指定 URL
func (bws *BusinessWebhookService) sendToURL(baseURL string, event *models.BusinessWebhookEvent, payload []byte) {
	logger := utils.GetLogger()

	// 构建带有 event 参数的 URL
	// 格式: baseURL?event=room_started
	u, err := url.Parse(baseURL)
	if err != nil {
		logger.Error("解析 webhook URL 失败",
			zap.String("url", baseURL),
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
		bws.logWebhookAttempt(event, baseURL, 0, "", err.Error())
		return
	}

	// 添加 event 查询参数
	q := u.Query()
	q.Set("event", event.EventType)
	u.RawQuery = q.Encode()
	finalURL := u.String()

	// 创建请求
	req, err := http.NewRequest("POST", finalURL, bytes.NewBuffer(payload))
	if err != nil {
		logger.Error("创建 webhook 请求失败",
			zap.String("url", finalURL),
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
		// 记录请求创建失败
		bws.logWebhookAttempt(event, finalURL, 0, "", err.Error())
		return
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Event-Type", event.EventType)
	req.Header.Set("X-Event-ID", event.EventID)
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", event.Timestamp))

	// 计算签名
	signature := bws.calculateSignature(payload)
	req.Header.Set("X-Signature", signature)

	// 发送请求
	resp, err := bws.client.Do(req)
	if err != nil {
		logger.Error("发送 webhook 请求失败",
			zap.String("url", finalURL),
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
		// 记录网络错误
		bws.logWebhookAttempt(event, finalURL, 0, "", err.Error())
		return
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取 webhook 响应失败",
			zap.String("url", finalURL),
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
		// 记录响应读取失败
		bws.logWebhookAttempt(event, finalURL, resp.StatusCode, "", err.Error())
		return
	}

	// 检查响应状态
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Info("webhook 事件发送成功",
			zap.String("url", finalURL),
			zap.String("event_type", event.EventType),
			zap.String("event_id", event.EventID),
			zap.Int("status_code", resp.StatusCode),
		)
		// 成功的请求不记录日志
	} else {
		logger.Warn("webhook 事件发送失败",
			zap.String("url", finalURL),
			zap.String("event_type", event.EventType),
			zap.String("event_id", event.EventID),
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(respBody)),
		)
		// 只记录失败的请求
		bws.logWebhookAttempt(event, finalURL, resp.StatusCode, string(respBody), "HTTP "+fmt.Sprintf("%d", resp.StatusCode))
	}
}

// calculateSignature 计算请求签名
func (bws *BusinessWebhookService) calculateSignature(payload []byte) string {
	h := hmac.New(sha256.New, []byte(bws.config.BusinessWebhookSecret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// logWebhookAttempt 记录 webhook 发送尝试
func (bws *BusinessWebhookService) logWebhookAttempt(event *models.BusinessWebhookEvent, url string, statusCode int, response, errMsg string) {
	logger := utils.GetLogger()

	payload, _ := json.Marshal(event)

	log := &models.BusinessWebhookLog{
		EventType: event.EventType,
		EventID:   event.EventID,
		URL:       url,
		Status:    statusCode,
		Request:   string(payload),
		Response:  response,
		Error:     errMsg,
		Retry:     event.Retry,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := bws.db.Create(log).Error; err != nil {
		logger.Error("记录 webhook 日志失败",
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
	}
}

// generateEventID 生成事件 ID
func generateEventID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%1000)
}

// CleanupOldLogs 清理旧的 webhook 日志
// retentionDays: 保留天数，超过此天数的日志将被删除
func (bws *BusinessWebhookService) CleanupOldLogs(retentionDays int) error {
	logger := utils.GetLogger()

	if retentionDays <= 0 {
		retentionDays = 30 // 默认保留 30 天
	}

	// 计算截断时间
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	// 删除旧日志
	result := bws.db.Where("created_at < ?", cutoffTime).Delete(&models.BusinessWebhookLog{})
	if result.Error != nil {
		logger.Error("清理 webhook 日志失败",
			zap.Error(result.Error),
		)
		return result.Error
	}

	logger.Info("webhook 日志清理完成",
		zap.Int64("deleted_count", result.RowsAffected),
		zap.Time("cutoff_time", cutoffTime),
	)

	return nil
}

// GetLogStats 获取日志统计信息
// 注意：日志表只记录失败的请求，所以这里统计的都是失败的请求
func (bws *BusinessWebhookService) GetLogStats() (map[string]interface{}, error) {
	logger := utils.GetLogger()

	var totalFailureCount int64

	// 获取失败日志总数
	if err := bws.db.Model(&models.BusinessWebhookLog{}).Count(&totalFailureCount).Error; err != nil {
		logger.Error("获取失败日志总数失败", zap.Error(err))
		return nil, err
	}

	// 获取最大日志 ID（用于估算表大小）
	var maxID int64
	bws.db.Model(&models.BusinessWebhookLog{}).Select("MAX(id)").Scan(&maxID)

	// 按事件类型统计失败数
	type EventStats struct {
		EventType string
		Count     int64
	}
	var eventStats []EventStats
	bws.db.Model(&models.BusinessWebhookLog{}).
		Select("event_type, COUNT(*) as count").
		Group("event_type").
		Scan(&eventStats)

	eventStatsMap := make(map[string]int64)
	for _, stat := range eventStats {
		eventStatsMap[stat.EventType] = stat.Count
	}

	stats := map[string]interface{}{
		"total_failure_count": totalFailureCount,
		"failure_by_event":    eventStatsMap,
		"max_id":              maxID,
	}

	return stats, nil
}

// ArchiveOldLogs 归档旧日志到另一个表（可选）
// 这个方法可以用于将旧日志移到归档表中
func (bws *BusinessWebhookService) ArchiveOldLogs(retentionDays int) error {
	logger := utils.GetLogger()

	if retentionDays <= 0 {
		retentionDays = 90 // 默认保留 90 天后归档
	}

	// 计算截断时间
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	// 这里可以实现将旧日志复制到归档表的逻辑
	// 例如：INSERT INTO business_webhook_log_archive SELECT * FROM business_webhook_log WHERE created_at < ?

	logger.Info("webhook 日志归档完成",
		zap.Time("cutoff_time", cutoffTime),
	)

	return nil
}
