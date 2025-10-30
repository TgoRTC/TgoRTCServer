package config

import (
	"os"
	"strconv"
)

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
	LiveKitURL       string
	LiveKitAPIKey    string
	LiveKitAPISecret string
	LiveKitTimeout   int // 单位：秒

	// 参与者超时配置
	ParticipantTimeoutCheckInterval int // 检查间隔，单位：秒，默认 10 秒

	// 业务 Webhook 配置（用于通知其他服务）
	BusinessWebhookEnabled bool   // 是否启用业务 webhook
	BusinessWebhookURL     string // 业务 webhook URL
	BusinessWebhookSecret  string // 业务 webhook 签名密钥
	BusinessWebhookTimeout int    // webhook 请求超时时间（秒），默认 10 秒

	// 业务 Webhook 日志清理配置
	BusinessWebhookLogRetentionDays   int  // 日志保留天数，默认 30 天
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

	// 业务 webhook 配置
	businessWebhookEnabled := false
	if os.Getenv("BUSINESS_WEBHOOK_ENABLED") == "true" {
		businessWebhookEnabled = true
	}

	businessWebhookURL := getEnv("BUSINESS_WEBHOOK_URL", "")

	businessWebhookTimeout := 10 // 默认 10 秒
	if timeout := os.Getenv("BUSINESS_WEBHOOK_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			businessWebhookTimeout = t
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
		DBName:     getEnv("DB_NAME", "tgo_call"),

		// Redis 配置
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,

		// LiveKit 配置
		LiveKitURL:       getEnv("LIVEKIT_URL", "http://localhost:7880"),
		LiveKitAPIKey:    getEnv("LIVEKIT_API_KEY", ""),
		LiveKitAPISecret: getEnv("LIVEKIT_API_SECRET", ""),
		LiveKitTimeout:   liveKitTimeout,

		// 参与者超时配置
		ParticipantTimeoutCheckInterval: participantTimeoutCheckInterval,

		// 业务 Webhook 配置
		BusinessWebhookEnabled: businessWebhookEnabled,
		BusinessWebhookURL:     businessWebhookURL,
		BusinessWebhookSecret:  getEnv("BUSINESS_WEBHOOK_SECRET", ""),
		BusinessWebhookTimeout: businessWebhookTimeout,

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
