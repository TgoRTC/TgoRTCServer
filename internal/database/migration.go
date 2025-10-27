package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// Migration 迁移记录模型
type Migration struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	Version   string    `gorm:"column:version;size:50;not null;uniqueIndex" json:"version"`
	Name      string    `gorm:"column:name;size:255;not null" json:"name"`
	SQL       string    `gorm:"column:sql;type:longtext;not null" json:"sql"`
	Status    string    `gorm:"column:status;size:20;not null;default:'pending'" json:"status"` // pending, success, failed
	Error     string    `gorm:"column:error;type:text" json:"error"`
	ExecutedAt *time.Time `gorm:"column:executed_at" json:"executed_at"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (Migration) TableName() string {
	return "migrations"
}

// MigrationManager 迁移管理器
type MigrationManager struct {
	db *gorm.DB
}

// NewMigrationManager 创建迁移管理器
func NewMigrationManager(db *gorm.DB) *MigrationManager {
	return &MigrationManager{db: db}
}

// InitMigrationTable 初始化迁移表
func (mm *MigrationManager) InitMigrationTable() error {
	if err := mm.db.AutoMigrate(&Migration{}); err != nil {
		return fmt.Errorf("迁移表初始化失败: %w", err)
	}
	log.Println("✅ 迁移表初始化完成")
	return nil
}

// ExecuteMigration 执行迁移
func (mm *MigrationManager) ExecuteMigration(version, name, sql string) error {
	// 检查迁移是否已执行
	var existingMigration Migration
	if err := mm.db.Where("version = ?", version).First(&existingMigration).Error; err == nil {
		if existingMigration.Status == "success" {
			log.Printf("⏭️  迁移已执行，跳过: %s (%s)", version, name)
			return nil
		}
	}

	// 记录迁移开始
	migration := Migration{
		Version: version,
		Name:    name,
		SQL:     sql,
		Status:  "pending",
	}

	if err := mm.db.Create(&migration).Error; err != nil {
		return fmt.Errorf("记录迁移失败: %w", err)
	}

	// 执行 SQL
	log.Printf("🔄 执行迁移: %s (%s)", version, name)
	log.Printf("📝 SQL: %s", sql)

	if err := mm.db.Exec(sql).Error; err != nil {
		// 更新迁移状态为失败
		mm.db.Model(&migration).Updates(map[string]interface{}{
			"status": "failed",
			"error":  err.Error(),
		})
		return fmt.Errorf("执行迁移失败: %w", err)
	}

	// 更新迁移状态为成功
	now := time.Now()
	if err := mm.db.Model(&migration).Updates(map[string]interface{}{
		"status":      "success",
		"executed_at": now,
	}).Error; err != nil {
		return fmt.Errorf("更新迁移状态失败: %w", err)
	}

	log.Printf("✅ 迁移执行成功: %s (%s)", version, name)
	return nil
}

// GetMigrationHistory 获取迁移历史
func (mm *MigrationManager) GetMigrationHistory() ([]Migration, error) {
	var migrations []Migration
	if err := mm.db.Order("version ASC").Find(&migrations).Error; err != nil {
		return nil, fmt.Errorf("查询迁移历史失败: %w", err)
	}
	return migrations, nil
}

// GetMigrationStatus 获取迁移状态
func (mm *MigrationManager) GetMigrationStatus() (map[string]interface{}, error) {
	var total int64
	var successCount int64
	var failedCount int64

	mm.db.Model(&Migration{}).Count(&total)
	mm.db.Model(&Migration{}).Where("status = ?", "success").Count(&successCount)
	mm.db.Model(&Migration{}).Where("status = ?", "failed").Count(&failedCount)

	return map[string]interface{}{
		"total":   total,
		"success": successCount,
		"failed":  failedCount,
		"pending": total - successCount - failedCount,
	}, nil
}

