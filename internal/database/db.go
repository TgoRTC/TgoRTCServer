package database

import (
	"fmt"
	"log"

	"tgo-call-server/internal/config"
	"tgo-call-server/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// InitDB 初始化数据库连接
func InitDB(cfg *config.Config) (*gorm.DB, error) {
	// 构建 DSN
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	// 连接数据库
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	log.Println("✅ 数据库连接成功")

	// 初始化迁移管理器
	mm := NewMigrationManager(db)

	// 初始化迁移表
	if err := mm.InitMigrationTable(); err != nil {
		return nil, fmt.Errorf("迁移表初始化失败: %w", err)
	}

	// 运行迁移脚本
	if err := RunMigrations(mm); err != nil {
		return nil, fmt.Errorf("运行迁移脚本失败: %w", err)
	}

	log.Println("✅ 数据库迁移完成")

	// 自动迁移（用于其他模型）
	if err := db.AutoMigrate(&models.Room{}, &models.Participant{}); err != nil {
		return nil, fmt.Errorf("自动迁移失败: %w", err)
	}

	return db, nil
}
