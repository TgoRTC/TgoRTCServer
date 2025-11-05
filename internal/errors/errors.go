package errors

import (
	"fmt"
	"tgo-rtc-server/internal/i18n"
)

// BusinessError 业务错误（返回 400）
type BusinessError struct {
	Message string
	Key     i18n.MessageKey
	Args    []interface{}
}

func (e *BusinessError) Error() string {
	return e.Message
}

// NewBusinessError 创建业务错误
func NewBusinessError(message string) *BusinessError {
	return &BusinessError{Message: message}
}

// NewBusinessErrorf 创建格式化的业务错误
func NewBusinessErrorf(format string, args ...interface{}) *BusinessError {
	return &BusinessError{Message: fmt.Sprintf(format, args...)}
}

// NewBusinessErrorWithKey 创建带有翻译键的业务错误
func NewBusinessErrorWithKey(key i18n.MessageKey, args ...interface{}) *BusinessError {
	return &BusinessError{
		Key:  key,
		Args: args,
	}
}

// GetLocalizedMessage 获取本地化的错误消息
func (e *BusinessError) GetLocalizedMessage(lang string) string {
	if e.Key != "" {
		return i18n.Translate(lang, e.Key, e.Args...)
	}
	return e.Message
}

// IsBusinessError 判断是否为业务错误
func IsBusinessError(err error) bool {
	_, ok := err.(*BusinessError)
	return ok
}
