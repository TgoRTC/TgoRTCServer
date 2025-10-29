package router

import (
	"tgo-call-server/internal/config"
	"tgo-call-server/internal/handler"
	"tgo-call-server/internal/livekit"
	"tgo-call-server/internal/middleware"
	"tgo-call-server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// SetupRouter 设置路由
func SetupRouter(db *gorm.DB, redisClient *redis.Client, cfg *config.Config) *gin.Engine {
	router := gin.Default()

	// 添加多语言中间件
	router.Use(middleware.LanguageMiddleware())

	// 初始化 Token 生成器
	tokenGenerator := livekit.NewTokenGenerator(cfg)

	// 初始化服务层
	roomService := service.NewRoomService(db, tokenGenerator)
	participantService := service.NewParticipantService(db, tokenGenerator)

	// 初始化处理器
	roomHandler := handler.NewRoomHandler(roomService)
	participantHandler := handler.NewParticipantHandler(participantService)

	// 初始化 webhook 服务和处理器
	webhookService := service.NewWebhookService(db)
	webhookValidator := livekit.NewWebhookValidator(cfg.LiveKitAPIKey, cfg.LiveKitAPISecret)
	webhookHandler := handler.NewWebhookHandler(webhookService, webhookValidator)

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// API 路由组
	api := router.Group("/api/v1")
	{
		// 房间相关接口
		rooms := api.Group("/rooms")
		{
			rooms.POST("", roomHandler.CreateRoom)                                // 创建房间
			rooms.POST("/:room_id/invite", participantHandler.InviteParticipants) // 邀请参与者
			rooms.POST("/:room_id/join", participantHandler.JoinRoom)             // 加入房间
			rooms.POST("/:room_id/leave", participantHandler.LeaveRoom)           // 离开房间
		}

		// 参与者相关接口
		participants := api.Group("/participants")
		{
			participants.POST("/calling", participantHandler.CheckUserCallStatus) // 查询正在通话的成员
		}

		// Webhook 相关接口
		webhooks := api.Group("/webhooks")
		{
			webhooks.POST("/livekit", webhookHandler.HandleWebhook) // LiveKit webhook
		}
	}

	return router
}
