package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// WebhookEndpoint 单个 webhook 端点配置
type WebhookEndpoint struct {
	URL     string `json:"url"`               // Webhook URL
	Secret  string `json:"secret"`            // 该 URL 对应的签名密钥
	Timeout int    `json:"timeout,omitempty"` // 该端点的超时时间（秒），0 表示使用全局默认值
}

// Config 应用配置
type Config struct {
	// 服务配置
	Port     string
	Env      string
	LogLevel string

	// 数据库配置
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis 配置
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// LiveKit 配置
	LiveKitURL       string // 后端调用 LiveKit API 的地址（内部网络）
	LiveKitClientURL string // 前端连接 LiveKit 的地址（公网地址）
	LiveKitAPIKey    string
	LiveKitAPISecret string
	LiveKitTimeout   int // 单位：秒

	// 参与者超时配置
	ParticipantTimeoutCheckInterval int // 检查间隔，单位：秒，默认 10 秒

	// 业务 Webhook 配置（用于通知其他服务）
	BusinessWebhookEndpoints []WebhookEndpoint // 业务 webhook 端点列表（支持每个 URL 配置独立的密钥）
	BusinessWebhookTimeout   int               // webhook 请求超时时间（秒），默认 10 秒

	// 业务 Webhook 日志清理配置
	BusinessWebhookLogRetentionDays   int  // 日志保留天数，默认 7 天
	BusinessWebhookLogCleanupEnabled  bool // 是否启用日志自动清理
	BusinessWebhookLogCleanupInterval int  // 日志清理间隔（秒），默认 86400（1 天）
}

// LoadConfig 从环境变量加载配置
func LoadConfig() *Config {
	redisDB := 0
	if db := os.Getenv("REDIS_DB"); db != "" {
		if d, err := strconv.Atoi(db); err == nil {
			redisDB = d
		}
	}

	liveKitTimeout := 3600 // 默认 1 小时
	if timeout := os.Getenv("LIVEKIT_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			liveKitTimeout = t
		}
	}

	participantTimeoutCheckInterval := 10 // 默认 10 秒
	if interval := os.Getenv("PARTICIPANT_TIMEOUT_CHECK_INTERVAL"); interval != "" {
		if i, err := strconv.Atoi(interval); err == nil {
			participantTimeoutCheckInterval = i
		}
	}

	// 解析业务 webhook 配置
	// 支持两种配置方式：
	// 1. 新方式：BUSINESS_WEBHOOK_ENDPOINTS (JSON 格式，支持每个 URL 独立配置密钥和超时)
	// 2. 旧方式：BUSINESS_WEBHOOK_URLS + BUSINESS_WEBHOOK_SECRET (向后兼容)
	var businessWebhookEndpoints []WebhookEndpoint

	// 全局默认超时时间
	businessWebhookTimeout := 10 // 默认 10 秒
	if timeout := os.Getenv("BUSINESS_WEBHOOK_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			businessWebhookTimeout = t
		}
	}

	// 优先使用新方式（JSON 配置）
	if endpointsJSON := os.Getenv("BUSINESS_WEBHOOK_ENDPOINTS"); endpointsJSON != "" {
		if err := json.Unmarshal([]byte(endpointsJSON), &businessWebhookEndpoints); err != nil {
			// JSON 解析失败，记录错误但不中断程序
			// 可以考虑使用日志记录，这里暂时忽略
		} else {
			// 为没有配置超时的端点设置默认超时
			for i := range businessWebhookEndpoints {
				if businessWebhookEndpoints[i].Timeout <= 0 {
					businessWebhookEndpoints[i].Timeout = businessWebhookTimeout
				}
			}
		}
	}

	// 如果新方式没有配置，尝试使用旧方式（向后兼容）
	if len(businessWebhookEndpoints) == 0 {
		businessWebhookURLsStr := getEnv("BUSINESS_WEBHOOK_URLS", "")
		businessWebhookSecret := getEnv("BUSINESS_WEBHOOK_SECRET", "")

		if businessWebhookURLsStr != "" {
			// 按逗号分隔，并去除空格
			for _, url := range strings.Split(businessWebhookURLsStr, ",") {
				if trimmedURL := strings.TrimSpace(url); trimmedURL != "" {
					businessWebhookEndpoints = append(businessWebhookEndpoints, WebhookEndpoint{
						URL:     trimmedURL,
						Secret:  businessWebhookSecret,  // 所有 URL 使用相同的密钥
						Timeout: businessWebhookTimeout, // 使用全局超时配置
					})
				}
			}
		}
	}

	// 业务 webhook 日志清理配置
	businessWebhookLogRetentionDays := 7 // 默认保留 7 天
	if days := os.Getenv("BUSINESS_WEBHOOK_LOG_RETENTION_DAYS"); days != "" {
		if d, err := strconv.Atoi(days); err == nil && d > 0 {
			businessWebhookLogRetentionDays = d
		}
	}

	businessWebhookLogCleanupEnabled := false
	if os.Getenv("BUSINESS_WEBHOOK_LOG_CLEANUP_ENABLED") == "true" {
		businessWebhookLogCleanupEnabled = true
	}

	businessWebhookLogCleanupInterval := 86400 // 默认 1 天
	if interval := os.Getenv("BUSINESS_WEBHOOK_LOG_CLEANUP_INTERVAL"); interval != "" {
		if i, err := strconv.Atoi(interval); err == nil && i > 0 {
			businessWebhookLogCleanupInterval = i
		}
	}

	return &Config{
		// 服务配置
		Port:     getEnv("PORT", "8080"),
		Env:      getEnv("ENV", "development"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		// 数据库配置
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "tgo_rtc"),

		// Redis 配置
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,

		// LiveKit 配置
		LiveKitURL:       getEnv("LIVEKIT_URL", "http://localhost:7880"),
		LiveKitClientURL: getEnv("LIVEKIT_CLIENT_URL", getEnv("LIVEKIT_URL", "http://localhost:7880")), // 默认使用 LIVEKIT_URL
		LiveKitAPIKey:    getEnv("LIVEKIT_API_KEY", ""),
		LiveKitAPISecret: getEnv("LIVEKIT_API_SECRET", ""),
		LiveKitTimeout:   liveKitTimeout,

		// 参与者超时配置
		ParticipantTimeoutCheckInterval: participantTimeoutCheckInterval,

		// 业务 Webhook 配置
		BusinessWebhookEndpoints: businessWebhookEndpoints,
		BusinessWebhookTimeout:   businessWebhookTimeout,

		// 业务 Webhook 日志清理配置
		BusinessWebhookLogRetentionDays:   businessWebhookLogRetentionDays,
		BusinessWebhookLogCleanupEnabled:  businessWebhookLogCleanupEnabled,
		BusinessWebhookLogCleanupInterval: businessWebhookLogCleanupInterval,
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
