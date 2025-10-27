package livekit

import (
	"fmt"
	"time"

	"tgo-call-server/internal/config"

	"github.com/livekit/protocol/auth"
)

// TokenGenerator LiveKit Token 生成器
type TokenGenerator struct {
	apiKey    string
	apiSecret string
}

// NewTokenGenerator 创建 Token 生成器
func NewTokenGenerator(cfg *config.Config) *TokenGenerator {
	return &TokenGenerator{
		apiKey:    cfg.LiveKitAPIKey,
		apiSecret: cfg.LiveKitAPISecret,
	}
}

// GenerateToken 生成 LiveKit Token
func (tg *TokenGenerator) GenerateToken(roomName, participantName string) (string, error) {
	return tg.GenerateTokenWithExpiry(roomName, participantName, time.Hour)
}

// GenerateTokenWithExpiry 生成指定过期时间的 Token
func (tg *TokenGenerator) GenerateTokenWithExpiry(roomName, participantName string, expiry time.Duration) (string, error) {
	if tg.apiKey == "" || tg.apiSecret == "" {
		return "", fmt.Errorf("LiveKit API 密钥未配置")
	}

	// 创建 Token 生成器
	at := auth.NewAccessToken(tg.apiKey, tg.apiSecret)

	// 添加视频权限
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     roomName,
	}
	at.AddGrant(grant)

	// 生成 Token
	token, err := at.ToJWT()
	if err != nil {
		return "", fmt.Errorf("生成 Token 失败: %w", err)
	}

	return token, nil
}
