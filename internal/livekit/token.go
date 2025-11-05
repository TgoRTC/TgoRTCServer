package livekit

import (
	"errors"
	"fmt"
	"time"

	"tgo-rtc-server/internal/config"

	"github.com/livekit/protocol/auth"
)

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
	url       string
	timeout   int
}

// NewTokenGenerator 创建 Token 生成器
func NewTokenGenerator(cfg *config.Config) *TokenGenerator {
	return &TokenGenerator{
		apiKey:    cfg.LiveKitAPIKey,
		apiSecret: cfg.LiveKitAPISecret,
		url:       cfg.LiveKitURL,
		timeout:   cfg.LiveKitTimeout,
	}
}

// GenerateToken 生成 LiveKit Token
func (tg *TokenGenerator) GenerateToken(roomName, uid string) (string, error) {
	return tg.GenerateTokenWithExpiry(roomName, uid)
}

// GenerateTokenWithConfig 生成 Token 并返回配置信息
func (tg *TokenGenerator) GenerateTokenWithConfig(roomName, uid string) (*TokenResult, error) {
	token, err := tg.GenerateTokenWithExpiry(roomName, uid)
	if err != nil {
		return nil, err
	}
	return &TokenResult{
		Token:   token,
		URL:     tg.url,
		Timeout: tg.timeout,
	}, nil
}

// GenerateTokenWithExpiry 生成指定过期时间的 Token
func (tg *TokenGenerator) GenerateTokenWithExpiry(roomName, uid string) (string, error) {
	if tg.apiKey == "" || tg.apiSecret == "" {
		return "", fmt.Errorf("LiveKit API 密钥未配置")
	}
	at := auth.NewAccessToken(tg.apiKey, tg.apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin:   true,
		RoomCreate: true,
		Room:       roomName,
	}
	at.AddGrant(grant).
		SetIdentity(uid).
		SetValidFor(time.Hour)

	token, err := at.ToJWT()
	if err != nil {
		return "", errors.New("生成token错误")
	}
	return token, nil
}
