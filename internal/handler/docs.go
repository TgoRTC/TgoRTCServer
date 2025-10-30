package handler

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// GetSwaggerJSON 提供 Swagger JSON 文件
func GetSwaggerJSON(c *gin.Context) {
	swaggerPath := filepath.Join("docs", "swagger.json")
	c.File(swaggerPath)
}

