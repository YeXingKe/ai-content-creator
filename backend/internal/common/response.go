package common

// BaseResponse 统一响应结构
type BaseResponse struct {
	Code    int         `json:"code" example:"0"`       // 业务状态码，0 表示成功，非 0 表示失败
	Data    interface{} `json:"data,omitempty"`         // 响应数据，成功时返回；失败时通常为 null
	Message string      `json:"message" example:"ok"`     // 响应消息，成功时为 ok，失败时为错误描述
}

// Success 成功响应
func Success(data interface{}) BaseResponse {
	return BaseResponse{
		Code:    0,
		Data:    data,
		Message: "ok",
	}
}

// SuccessWithMsg 成功响应（自定义消息）
func SuccessWithMsg(data interface{}, message string) BaseResponse {
	return BaseResponse{
		Code:    0,
		Data:    data,
		Message: message,
	}
}

// Error 错误响应
func Error(err *AppError) BaseResponse {
	return BaseResponse{
		Code:    err.Code,
		Data:    nil,
		Message: err.Message,
	}
}

// ErrorWithMsg 错误响应（自定义消息）
func ErrorWithMsg(err *AppError, message string) BaseResponse {
	return BaseResponse{
		Code:    err.Code,
		Data:    nil,
		Message: message,
	}
}
