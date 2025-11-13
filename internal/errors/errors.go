package errors

import (
	"fmt"
	"tgo-rtc-server/internal/i18n"
)

// BusinessError 业务错误（HTTP 状态码统一返回 400）
type BusinessError struct {
	Message string
	Key     i18n.MessageKey
	Args    []interface{}
	Code    int // 业务错误码，用于响应体中的 code 字段
}

func (e *BusinessError) Error() string {
	return e.Message
}

// NewBusinessError 创建业务错误（默认 code 400）
func NewBusinessError(message string) *BusinessError {
	return &BusinessError{
		Message: message,
		Code:    400,
	}
}

// NewBusinessErrorf 创建格式化的业务错误（默认 code 400）
func NewBusinessErrorf(format string, args ...interface{}) *BusinessError {
	return &BusinessError{
		Message: fmt.Sprintf(format, args...),
		Code:    400,
	}
}

// NewBusinessErrorWithKey 创建带有翻译键的业务错误（默认 code 400）
func NewBusinessErrorWithKey(key i18n.MessageKey, args ...interface{}) *BusinessError {
	return &BusinessError{
		Key:  key,
		Args: args,
		Code: 400,
	}
}

// NewConflictError 创建冲突错误（code 409，HTTP 状态码仍为 400）
func NewConflictError(key i18n.MessageKey, args ...interface{}) *BusinessError {
	return &BusinessError{
		Key:  key,
		Args: args,
		Code: 409,
	}
}

// GetLocalizedMessage 获取本地化的错误消息
func (e *BusinessError) GetLocalizedMessage(lang string) string {
	if e.Key != "" {
		return i18n.Translate(lang, e.Key, e.Args...)
	}
	return e.Message
}

// GetErrorCode 获取错误代码（字符串格式）
func (e *BusinessError) GetErrorCode() string {
	if e.Code == 0 {
		return "400"
	}
	return fmt.Sprintf("%d", e.Code)
}

// IsBusinessError 判断是否为业务错误
func IsBusinessError(err error) bool {
	_, ok := err.(*BusinessError)
	return ok
}
