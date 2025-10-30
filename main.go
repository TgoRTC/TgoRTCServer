package main

import (
	"log"

	"tgo-call-server/internal/config"
	"tgo-call-server/internal/database"
	"tgo-call-server/internal/router"
	"tgo-call-server/internal/service"
	"tgo-call-server/internal/utils"

	"github.com/joho/godotenv"
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
