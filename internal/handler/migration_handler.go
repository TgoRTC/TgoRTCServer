package handler

import (
	"net/http"

	"tgo-call-server/internal/database"

	"github.com/gin-gonic/gin"
)

// MigrationHandler 迁移处理器
type MigrationHandler struct {
	migrationManager *database.MigrationManager
}

// NewMigrationHandler 创建迁移处理器
func NewMigrationHandler(migrationManager *database.MigrationManager) *MigrationHandler {
	return &MigrationHandler{
		migrationManager: migrationManager,
	}
}

// GetMigrationHistory 获取迁移历史
// GET /api/migrations/history
func (mh *MigrationHandler) GetMigrationHistory(c *gin.Context) {
	migrations, err := mh.migrationManager.GetMigrationHistory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, migrations)
}

// GetMigrationStatus 获取迁移状态
// GET /api/migrations/status
func (mh *MigrationHandler) GetMigrationStatus(c *gin.Context) {
	status, err := mh.migrationManager.GetMigrationStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, status)
}
