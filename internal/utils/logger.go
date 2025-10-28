package utils

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

// InitLogger 初始化日志记录器
func InitLogger() error {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.NameKey = "logger"
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.MessageKey = "msg"
	config.EncoderConfig.StacktraceKey = "stacktrace"
	config.EncoderConfig.LineEnding = zapcore.DefaultLineEnding
	config.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	config.EncoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	logger, err := config.Build()
	if err != nil {
		return err
	}

	Logger = logger
	return nil
}

// GetLogger 获取全局日志记录器
func GetLogger() *zap.Logger {
	if Logger == nil {
		// 如果未初始化，创建一个默认的日志记录器
		logger, _ := zap.NewProduction()
		Logger = logger
	}
	return Logger
}

// CloseLogger 关闭日志记录器
func CloseLogger() error {
	if Logger != nil {
		return Logger.Sync()
	}
	return nil
}

