package main

import (
	"log"

	"tgo-rtc-server/internal/config"
	"tgo-rtc-server/internal/database"
	"tgo-rtc-server/internal/router"
	"tgo-rtc-server/internal/service"
	"tgo-rtc-server/internal/utils"

	"github.com/joho/godotenv"

	_ "tgo-rtc-server/docs"
)

// @title TgoRTC Server API
// @version 1.0.0
// @description 基于 LiveKit 的音视频服务 API
// @contact.name API Support
// @contact.url https://github.com/TgoRTC/TgoRTCServer
// @host livekit.example.com
// @schemes https http
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Println("未找到 .env 文件，使用系统环境变量")
	}

	// 初始化日志记录器
	if err := utils.InitLogger(); err != nil {
		log.Fatalf("日志初始化失败: %v", err)
	}
	defer utils.CloseLogger()

	// 初始化配置
	cfg := config.LoadConfig()

	// 初始化数据库
	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 初始化 Redis
	redisClient, err := database.InitRedis(cfg)
	if err != nil {
		log.Fatalf("Redis 初始化失败: %v", err)
	}

	// 创建路由
	r := router.SetupRouter(db, redisClient, cfg)

	// 启动参与者超时检查定时器
	scheduler := service.NewSchedulerService(db, cfg)
	scheduler.Start()
	defer scheduler.Stop()

	// 启动 webhook 日志清理定时器
	logCleanup := service.NewWebhookLogCleanupService(db, cfg)
	logCleanup.Start()
	defer logCleanup.Stop()

	// 启动服务器
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 音视频服务启动在端口 %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
