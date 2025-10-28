package router

import (
	"tgo-call-server/internal/config"
	"tgo-call-server/internal/database"
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

	// 初始化迁移管理器
	migrationManager := database.NewMigrationManager(db)

	// 初始化处理器
	roomHandler := handler.NewRoomHandler(roomService)
	participantHandler := handler.NewParticipantHandler(participantService)
	migrationHandler := handler.NewMigrationHandler(migrationManager)

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
			rooms.POST("", roomHandler.CreateRoom)                      // 创建房间
			rooms.GET("", roomHandler.ListRooms)                        // 列出房间列表
			rooms.GET("/:room_id", roomHandler.GetRoom)                 // 获取房间信息
			rooms.PUT("/:room_id/status", roomHandler.UpdateRoomStatus) // 更新房间状态
			rooms.POST("/:room_id/end", roomHandler.EndRoom)            // 结束房间

			// 参与者相关接口
			rooms.GET("/:room_id/participants", participantHandler.GetParticipants)                     // 获取参与者列表
			rooms.POST("/:room_id/invite", participantHandler.InviteParticipants)                       // 邀请参与者
			rooms.PUT("/:room_id/participants/:uid/status", participantHandler.UpdateParticipantStatus) // 更新参与者状态
		}

		// 参与者相关接口
		participants := api.Group("/participants")
		{
			participants.POST("/join", participantHandler.JoinRoom)               // 加入房间
			participants.POST("/leave", participantHandler.LeaveRoom)             // 离开房间
			participants.POST("/calling", participantHandler.CheckUserCallStatus) // 检查用户通话状态
		}

		// 迁移管理接口
		migrations := api.Group("/migrations")
		{
			migrations.GET("/history", migrationHandler.GetMigrationHistory) // 获取迁移历史
			migrations.GET("/status", migrationHandler.GetMigrationStatus)   // 获取迁移状态
		}
	}

	return router
}
