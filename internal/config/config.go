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

	// Webhook 配置
	WebhookEnabled bool
	WebhookSecret  string
}

// LoadConfig 从环境变量加载配置
func LoadConfig() *Config {
	webhookEnabled := false
	if os.Getenv("WEBHOOK_ENABLED") == "true" {
		webhookEnabled = true
	}

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

		// Webhook 配置
		WebhookEnabled: webhookEnabled,
		WebhookSecret:  getEnv("WEBHOOK_SECRET", ""),
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
