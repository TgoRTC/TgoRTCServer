package middleware

import (
	"github.com/gin-gonic/gin"
	"tgo-rtc-server/internal/i18n"
)

// LanguageMiddleware 语言中间件
// 从请求头中提取语言信息，如果未传递则使用默认语言
func LanguageMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头中获取语言，支持以下几种方式：
		// 1. Accept-Language 标准 HTTP 头
		// 2. X-Language 自定义头
		// 3. lang 查询参数
		lang := c.GetHeader("X-Language")
		if lang == "" {
			lang = c.Query("lang")
		}
		if lang == "" {
			lang = c.GetHeader("Accept-Language")
		}

		// 获取支持的语言，如果不支持则使用默认语言
		lang = i18n.GetLanguage(lang)

		// 将语言信息存储在上下文中
		c.Set(i18n.LanguageContextKey, lang)

		c.Next()
	}
}

// GetLanguageFromContext 从上下文中获取语言
func GetLanguageFromContext(c *gin.Context) string {
	if lang, exists := c.Get(i18n.LanguageContextKey); exists {
		if langStr, ok := lang.(string); ok {
			return langStr
		}
	}
	return i18n.DefaultLanguage
}

