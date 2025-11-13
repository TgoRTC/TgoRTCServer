package livekit

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// WebhookValidator LiveKit webhook 验证器
type WebhookValidator struct {
	apiKey    string
	apiSecret string
}

// NewWebhookValidator 创建 webhook 验证器
func NewWebhookValidator(apiKey, apiSecret string) *WebhookValidator {
	return &WebhookValidator{
		apiKey:    apiKey,
		apiSecret: apiSecret,
	}
}

// ValidateRequest 验证 webhook 请求
// 返回原始请求体和错误信息
func (wv *WebhookValidator) ValidateRequest(r *http.Request) ([]byte, error) {
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("读取请求体失败: %w", err)
	}

	// 获取 Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("缺少 Authorization header")
	}

	// 验证 JWT token
	if err := wv.verifyToken(authHeader, body); err != nil {
		return nil, fmt.Errorf("验证 token 失败: %w", err)
	}

	return body, nil
}

// verifyToken 验证 JWT token
func (wv *WebhookValidator) verifyToken(tokenString string, body []byte) error {
	// 移除 "Bearer " 前缀
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	// 计算 payload 的 SHA256 hash
	hash := sha256.Sum256(body)
	hashHex := hex.EncodeToString(hash[:])

	// 解析 token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(wv.apiSecret), nil
	})

	if err != nil {
		return fmt.Errorf("解析 token 失败: %w", err)
	}

	if !token.Valid {
		return fmt.Errorf("token 无效")
	}

	// 验证 claims 中的 sha256 hash
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("无法获取 token claims")
	}

	claimHash, ok := claims["sha256"].(string)
	if !ok {
		return fmt.Errorf("token 中缺少 sha256 claim")
	}

	// LiveKit 可能发送 base64 编码或 hex 编码的 hash
	// 尝试将 base64 编码转换为 hex 编码
	var claimHashHex string
	if decoded, err := base64.StdEncoding.DecodeString(claimHash); err == nil {
		// 是 base64 编码，转换为 hex
		claimHashHex = hex.EncodeToString(decoded)
	} else {
		// 假设是 hex 编码
		claimHashHex = claimHash
	}

	if claimHashHex != hashHex {
		return fmt.Errorf("sha256 hash 不匹配: expected %s, got %s (原始: %s)", hashHex, claimHashHex, claimHash)
	}

	return nil
}
