package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"tgo-rtc-server/internal/config"
	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/utils"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BusinessWebhookService 业务 webhook 服务
type BusinessWebhookService struct {
	db          *gorm.DB
	redisClient *redis.Client
	config      *config.Config
	client      *http.Client
}

// NewBusinessWebhookService 创建业务 webhook 服务
func NewBusinessWebhookService(db *gorm.DB, redisClient *redis.Client, cfg *config.Config) *BusinessWebhookService {
	return &BusinessWebhookService{
		db:          db,
		redisClient: redisClient,
		config:      cfg,
		client:      &http.Client{
			// 不设置全局超时，每个请求使用端点配置的超时
		},
	}
}

// SendEvent 发送业务 webhook 事件
func (bws *BusinessWebhookService) SendEvent(eventType string, data interface{}) error {
	logger := utils.GetLogger()

	// 检查是否配置了业务 webhook 端点（如果没有配置则不发送）
	if len(bws.config.BusinessWebhookEndpoints) == 0 {
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

	// 异步发送到所有配置的端点
	for _, endpoint := range bws.config.BusinessWebhookEndpoints {
		go bws.sendToEndpoint(endpoint, event, payload)
	}

	return nil
}

// SendRoomFinishedEventOnce 发送房间完成事件（确保同一个房间只发送一次）
// 使用 Redis 记录已发送的房间ID，避免重复发送
func (bws *BusinessWebhookService) SendRoomFinishedEventOnce(roomID string, data *models.RoomEventData) error {
	logger := utils.GetLogger()

	// 检查是否配置了业务 webhook 端点（如果没有配置则不发送）
	if len(bws.config.BusinessWebhookEndpoints) == 0 {
		return nil
	}

	// 构建 Redis key
	redisKey := fmt.Sprintf("room:finished:sent:%s", roomID)

	// 检查是否已经发送过
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exists, err := bws.redisClient.Exists(ctx, redisKey).Result()
	if err != nil {
		logger.Error("检查 Redis key 失败",
			zap.String("room_id", roomID),
			zap.String("redis_key", redisKey),
			zap.Error(err),
		)
		// Redis 失败不影响业务，继续发送
	} else if exists > 0 {
		// 已经发送过，跳过
		logger.Info("房间完成事件已发送过，跳过",
			zap.String("room_id", roomID),
		)
		return nil
	}

	// 发送事件
	if err := bws.SendEvent(models.BusinessEventRoomFinished, data); err != nil {
		logger.Error("发送房间完成事件失败",
			zap.String("room_id", roomID),
			zap.Error(err),
		)
		return err
	}

	// 标记为已发送（设置 24 小时过期）
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	if err := bws.redisClient.Set(ctx2, redisKey, "1", 24*time.Hour).Err(); err != nil {
		logger.Warn("标记房间完成事件已发送失败",
			zap.String("room_id", roomID),
			zap.String("redis_key", redisKey),
			zap.Error(err),
		)
		// 不返回错误，因为事件已经发送成功
	}

	return nil
}

// sendToEndpoint 发送事件到指定端点
func (bws *BusinessWebhookService) sendToEndpoint(endpoint config.WebhookEndpoint, event *models.BusinessWebhookEvent, payload []byte) {
	logger := utils.GetLogger()

	// 构建带有 event 参数的 URL
	// 格式: baseURL?event=room_started
	u, err := url.Parse(endpoint.URL)
	if err != nil {
		logger.Error("解析 webhook URL 失败",
			zap.String("url", endpoint.URL),
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
		bws.logWebhookAttempt(event, endpoint.URL, 0, "", err.Error())
		return
	}

	// 添加 event 查询参数
	q := u.Query()
	q.Set("event_type", event.EventType)
	q.Set("event_id", event.EventID)
	u.RawQuery = q.Encode()
	finalURL := u.String()

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(endpoint.Timeout)*time.Second)
	defer cancel()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", finalURL, bytes.NewBuffer(payload))
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

	// 计算签名（使用该端点对应的密钥）
	signature := bws.calculateSignatureWithSecret(payload, endpoint.Secret)
	req.Header.Set("X-Signature", signature)

	// 发送请求（使用端点配置的超时时间）
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

// calculateSignature 计算请求签名（已废弃，保留用于向后兼容）
func (bws *BusinessWebhookService) calculateSignature(payload []byte) string {
	// 如果有配置端点，使用第一个端点的密钥
	if len(bws.config.BusinessWebhookEndpoints) > 0 {
		return bws.calculateSignatureWithSecret(payload, bws.config.BusinessWebhookEndpoints[0].Secret)
	}
	return ""
}

// calculateSignatureWithSecret 使用指定密钥计算请求签名
func (bws *BusinessWebhookService) calculateSignatureWithSecret(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
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
	if retentionDays <= 0 {
		retentionDays = 90 // 默认保留 90 天后归档
	}

	// 计算截断时间
	// cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	// 这里可以实现将旧日志复制到归档表的逻辑
	// 例如：INSERT INTO business_webhook_log_archive SELECT * FROM business_webhook_log WHERE created_at < ?
	_ = retentionDays // 占位符，待实现归档逻辑时使用

	return nil
}
