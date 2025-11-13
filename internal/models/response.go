package models

// ErrorResponse 统一错误响应结构
type ErrorResponse struct {
	Code    string `json:"code"`    // 错误代码
	Message string `json:"message"` // 错误消息
}

// SuccessResponse 统一成功响应结构（带数据）
type SuccessResponse struct {
	Code    string      `json:"code"`           // 成功代码，通常为 "success"
	Message string      `json:"message"`        // 成功消息
	Data    interface{} `json:"data,omitempty"` // 响应数据，可选
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(code, msg string) *ErrorResponse {
	return &ErrorResponse{
		Code:    code,
		Message: msg,
	}
}

// NewSuccessResponse 创建成功响应
func NewSuccessResponse(msg string, data interface{}) *SuccessResponse {
	return &SuccessResponse{
		Code:    "success",
		Message: msg,
		Data:    data,
	}
}

// NewSuccessResponseWithCode 创建带自定义代码的成功响应
func NewSuccessResponseWithCode(code, msg string, data interface{}) *SuccessResponse {
	return &SuccessResponse{
		Code:    code,
		Message: msg,
		Data:    data,
	}
}
