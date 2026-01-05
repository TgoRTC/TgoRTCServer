package livekit

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"tgo-rtc-server/internal/config"
	"tgo-rtc-server/internal/utils"

	"github.com/livekit/protocol/auth"
	"go.uber.org/zap"
)

// ParticipantMetadata 参与者元数据
type ParticipantMetadata struct {
	DeviceType string `json:"device_type"`
}

// TokenResult Token 生成结果，包含 Token 和配置信息
type TokenResult struct {
	Token   string
	URL     string
	Timeout int // 单位：秒
}

// TokenGenerator LiveKit Token 生成器
type TokenGenerator struct {
	apiKey    string
	apiSecret string
	url       string // 后端调用 LiveKit API 的地址
	clientURL string // 前端连接 LiveKit 的地址
	timeout   int
}

// NewTokenGenerator 创建 Token 生成器
func NewTokenGenerator(cfg *config.Config) *TokenGenerator {
	return &TokenGenerator{
		apiKey:    cfg.LiveKitAPIKey,
		apiSecret: cfg.LiveKitAPISecret,
		url:       cfg.LiveKitURL,
		clientURL: cfg.LiveKitClientURL,
		timeout:   cfg.LiveKitTimeout,
	}
}

// GenerateToken 生成 LiveKit Token
func (tg *TokenGenerator) GenerateToken(roomName, uid, deviceType string) (string, error) {
	return tg.GenerateTokenWithExpiry(roomName, uid, deviceType)
}

// GenerateTokenWithConfig 生成 Token 并返回配置信息
func (tg *TokenGenerator) GenerateTokenWithConfig(roomName, uid, deviceType string) (*TokenResult, error) {
	logger := utils.GetLogger()

	token, err := tg.GenerateTokenWithExpiry(roomName, uid, deviceType)
	if err != nil {
		return nil, err
	}

	result := &TokenResult{
		Token:   token,
		URL:     tg.clientURL, // 返回前端可访问的 URL
		Timeout: tg.timeout,
	}

	// 记录 Token 生成和 LiveKit URL 分配信息
	logger.Info("LiveKit Token 生成成功",
		zap.String("room_id", roomName),
		zap.String("uid", uid),
		zap.String("device_type", deviceType),
		zap.String("livekit_url", tg.clientURL),
		zap.String("backend_url", tg.url),
		zap.Int("timeout", tg.timeout),
	)

	return result, nil
}

// GenerateTokenWithExpiry 生成指定过期时间的 Token
func (tg *TokenGenerator) GenerateTokenWithExpiry(roomName, uid, deviceType string) (string, error) {
	if tg.apiKey == "" || tg.apiSecret == "" {
		return "", fmt.Errorf("LiveKit API 密钥未配置")
	}

	// 构建 metadata JSON
	metadata := ParticipantMetadata{
		DeviceType: deviceType,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("序列化 metadata 失败: %w", err)
	}

	at := auth.NewAccessToken(tg.apiKey, tg.apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin:   true,
		RoomCreate: true,
		Room:       roomName,
	}
	at.AddGrant(grant).
		SetIdentity(uid).
		SetMetadata(string(metadataJSON)).
		SetValidFor(time.Hour)

	token, err := at.ToJWT()
	if err != nil {
		return "", errors.New("生成token错误")
	}
	return token, nil
}
