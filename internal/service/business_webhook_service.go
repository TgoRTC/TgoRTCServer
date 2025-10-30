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
	"time"

	"tgo-call-server/internal/config"
	"tgo-call-server/internal/models"
	"tgo-call-server/internal/utils"
	"gorm.io/gorm"
	"go.uber.org/zap"
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

	// 检查是否启用业务 webhook
	if !bws.config.BusinessWebhookEnabled || len(bws.config.BusinessWebhookURLs) == 0 {
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
	payload, err := json.Marshal(event)
	if err != nil {
		logger.Error("序列化 webhook 事件失败",
			zap.String("event_type", eventType),
			zap.Error(err),
		)
		return err
	}

	// 发送到所有配置的 URL
	for _, url := range bws.config.BusinessWebhookURLs {
		go bws.sendToURL(url, event, payload)
	}

	return nil
}

// sendToURL 发送事件到指定 URL
func (bws *BusinessWebhookService) sendToURL(url string, event *models.BusinessWebhookEvent, payload []byte) {
	logger := utils.GetLogger()

	// 创建请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		logger.Error("创建 webhook 请求失败",
			zap.String("url", url),
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
		bws.logWebhookAttempt(event, url, 0, "", err.Error())
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
			zap.String("url", url),
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
		bws.logWebhookAttempt(event, url, 0, "", err.Error())
		return
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取 webhook 响应失败",
			zap.String("url", url),
			zap.String("event_id", event.EventID),
			zap.Error(err),
		)
		bws.logWebhookAttempt(event, url, resp.StatusCode, "", err.Error())
		return
	}

	// 检查响应状态
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Info("webhook 事件发送成功",
			zap.String("url", url),
			zap.String("event_type", event.EventType),
			zap.String("event_id", event.EventID),
			zap.Int("status_code", resp.StatusCode),
		)
		bws.logWebhookAttempt(event, url, resp.StatusCode, string(respBody), "")
	} else {
		logger.Warn("webhook 事件发送失败",
			zap.String("url", url),
			zap.String("event_type", event.EventType),
			zap.String("event_id", event.EventID),
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(respBody)),
		)
		bws.logWebhookAttempt(event, url, resp.StatusCode, string(respBody), "HTTP "+fmt.Sprintf("%d", resp.StatusCode))
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

