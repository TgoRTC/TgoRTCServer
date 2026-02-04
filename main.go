package main

import (
	"log"

	"tgo-rtc-server/internal/config"
	"tgo-rtc-server/internal/database"
	"tgo-rtc-server/internal/router"
	"tgo-rtc-server/internal/service"
	"tgo-rtc-server/internal/utils"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

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

	logger := utils.GetLogger()

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

	// 初始化业务 webhook 服务
	businessWebhookService := service.NewBusinessWebhookService(db, redisClient, cfg)

	// 创建路由（同时获取 participantService 和 roomService）
	r, participantService, roomService := router.SetupRouter(db, redisClient, cfg, businessWebhookService)

	// 启动参与者超时检查定时器
	scheduler := service.NewSchedulerService(db, cfg)
	scheduler.SetBusinessWebhookService(businessWebhookService)
	scheduler.SetParticipantService(participantService)

	// 设置 scheduler 到各个服务（用于精确定时器）
	participantService.SetSchedulerService(scheduler)
	roomService.SetSchedulerService(scheduler)

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

	logger.Info("🚀 音视频服务启动",
		zap.String("port", port),
	)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
