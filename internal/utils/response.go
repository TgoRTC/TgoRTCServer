package utils

import (
	"net/http"
	"tgo-rtc-server/internal/errors"
	"tgo-rtc-server/internal/i18n"
	"tgo-rtc-server/internal/middleware"
	"tgo-rtc-server/internal/models"

	"github.com/gin-gonic/gin"
)

// RespondWithError 返回错误响应
func RespondWithError(c *gin.Context, statusCode int, code, msg string) {
	c.JSON(statusCode, models.NewErrorResponse(code, msg))
}

// RespondWithBusinessError 返回业务错误响应
func RespondWithBusinessError(c *gin.Context, err error) {
	lang := middleware.GetLanguageFromContext(c)

	if businessErr, ok := err.(*errors.BusinessError); ok {
		errorCode := businessErr.GetErrorCode()
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(errorCode, businessErr.GetLocalizedMessage(lang)))
		return
	}

	// 如果不是 BusinessError，返回通用错误
	c.JSON(http.StatusInternalServerError, models.NewErrorResponse("internal_error", err.Error()))
}

// RespondWithBindError 返回参数绑定错误响应
func RespondWithBindError(c *gin.Context) {
	lang := middleware.GetLanguageFromContext(c)
	c.JSON(http.StatusBadRequest, models.NewErrorResponse("400", i18n.Translate(lang, i18n.InvalidParameters)))
}

// RespondWithSuccess 返回成功响应（只有消息）
func RespondWithSuccess(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, models.NewSuccessResponse(msg, nil))
}

// RespondWithData 返回成功响应（带数据）
func RespondWithData(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

// RespondWithSuccessData 返回成功响应（带消息和数据）
func RespondWithSuccessData(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, models.NewSuccessResponse(msg, data))
}

// RespondCreated 返回创建成功响应（HTTP 201）
func RespondCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, data)
}

// RespondNoContent 返回无内容响应（HTTP 204）
func RespondNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// RespondUnauthorized 返回未授权错误（HTTP 401）
func RespondUnauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, models.NewErrorResponse("unauthorized", msg))
}

// RespondForbidden 返回禁止访问错误（HTTP 403）
func RespondForbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, models.NewErrorResponse("forbidden", msg))
}

// RespondNotFound 返回未找到错误（HTTP 404）
func RespondNotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, models.NewErrorResponse("not_found", msg))
}

// RespondInternalError 返回内部服务器错误（HTTP 500）
func RespondInternalError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, models.NewErrorResponse("internal_error", msg))
}

// RespondBadRequest 返回错误请求（HTTP 400）
func RespondBadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, models.NewErrorResponse("bad_request", msg))
}
