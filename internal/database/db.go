package database

import (
	"fmt"
	"strings"

	"tgo-rtc-server/internal/config"
	"tgo-rtc-server/internal/models"
	"tgo-rtc-server/internal/utils"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// InitDB 初始化数据库连接
func InitDB(cfg *config.Config) (*gorm.DB, error) {
	// 构建 DSN（带数据库名）
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	// 优雅处理“Unknown database”场景：尝试自动创建数据库（仅在开发环境有效）
	openWithDSN := func(d string) (*gorm.DB, error) { return gorm.Open(mysql.Open(d), &gorm.Config{}) }

	db, err := openWithDSN(dsn)
	if err != nil {
		if strings.Contains(err.Error(), "Unknown database") || strings.Contains(err.Error(), "1049") {
			// 连接到不带数据库名的 DSN
			dsnNoDB := fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
				cfg.DBUser,
				cfg.DBPassword,
				cfg.DBHost,
				cfg.DBPort,
			)
			noDB, err2 := openWithDSN(dsnNoDB)
			if err2 != nil {
				return nil, fmt.Errorf("数据库连接失败: %w", err)
			}
			// 创建数据库
			createSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;", cfg.DBName)
			if err2 = noDB.Exec(createSQL).Error; err2 != nil {
				return nil, fmt.Errorf("创建数据库失败: %w", err2)
			}
			// 再次尝试连接目标数据库
			db, err = openWithDSN(dsn)
			if err != nil {
				return nil, fmt.Errorf("数据库连接失败: %w", err)
			}
		} else {
			return nil, fmt.Errorf("数据库连接失败: %w", err)
		}
	}

	logger := utils.GetLogger()
	logger.Info("✅ 数据库连接成功")

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

	logger.Info("✅ 数据库迁移完成")

	// 自动迁移（用于其他模型）
	if err := db.AutoMigrate(&models.Room{}, &models.Participant{}); err != nil {
		return nil, fmt.Errorf("自动迁移失败: %w", err)
	}

	return db, nil
}
