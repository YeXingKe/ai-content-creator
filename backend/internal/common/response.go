package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`              // 业务错误码
	Message string      `json:"message"`           // 提示信息
	Data    interface{} `json:"data,omitempty"`    // 响应数据
}

// SuccessResponse 成功响应
func SuccessResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    Success,
		Message: SuccessMsg,
		Data:    data,
	})
}

// ErrorResponse 错误响应
func ErrorResponse(c *gin.Context, code int, message string) {
	httpStatus := http.StatusOK
	switch {
	case code >= 2000 && code < 3000:
		// 用户错误，根据具体错误码确定 HTTP 状态
		switch code {
		case ErrUserNotFound, ErrTokenExpired, ErrTokenInvalid:
			httpStatus = http.StatusUnauthorized
		default:
			httpStatus = http.StatusBadRequest
		}
	case code >= 3000:
		// 数据库错误
		httpStatus = http.StatusInternalServerError
	case code >= 1000:
		// 通用错误
		switch code {
		case ErrNotFound:
			httpStatus = http.StatusNotFound
		case ErrMethodNotAllowed:
			httpStatus = http.StatusMethodNotAllowed
		default:
			httpStatus = http.StatusBadRequest
		}
	}

	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// BizErrorResponse 业务错误响应
func BizErrorResponse(c *gin.Context, err *BizError) {
	ErrorResponse(c, err.Code, err.Message)
}

// PageResponse 分页响应数据
type PageResponse struct {
	Total    int64       `json:"total"`     // 总记录数
	Page     int         `json:"page"`      // 当前页码
	PageSize int         `json:"page_size"` // 每页大小
	Data     interface{} `json:"data"`      // 数据列表
}

// SuccessPageResponse 分页成功响应
func SuccessPageResponse(c *gin.Context, total int64, page, pageSize int, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    Success,
		Message: SuccessMsg,
		Data: PageResponse{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			Data:     data,
		},
	})
}
