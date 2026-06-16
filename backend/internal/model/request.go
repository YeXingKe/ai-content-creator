package model

import "time"

// RegisterRequest 用户注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20"` // 用户名
	Email    string `json:"email" binding:"required,email"`             // 邮箱
	Password string `json:"password" binding:"required,min=6,max=20"`   // 密码
}

// LoginRequest 用户登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"` // 用户名或邮箱
	Password string `json:"password" binding:"required"` // 密码
}

// UpdateUserRequest 更新用户信息请求
type UpdateUserRequest struct {
	Nickname string `json:"nickname" binding:"omitempty,max=30"` // 昵称
	Avatar   string `json:"avatar" binding:"omitempty,url"`      // 头像URL
	Bio      string `json:"bio" binding:"omitempty,max=200"`     // 个人简介
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"` // 旧密码
	NewPassword string `json:"new_password" binding:"required,min=6,max=20"` // 新密码
}

// ListRequest 通用列表查询请求
type ListRequest struct {
	Page     int    `form:"page,default=1" binding:"min=1"`           // 页码
	PageSize int    `form:"page_size,default=10" binding:"min=1,max=100"` // 每页大小
	Keyword  string `form:"keyword"`                                  // 搜索关键词
	SortBy   string `form:"sort_by,default=created_at"`              // 排序字段
	SortDesc bool   `form:"sort_desc,default=true"`                  // 是否降序
}

// UserListRequest 用户列表请求
type UserListRequest struct {
	ListRequest
	Status *int `form:"status"` // 用户状态筛选
}

// IDRequest 按 ID 查询请求
type IDRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"` // 资源ID
}

// DateRangeRequest 日期范围查询请求
type DateRangeRequest struct {
	StartTime *time.Time `form:"start_time"` // 开始时间
	EndTime   *time.Time `form:"end_time"`   // 结束时间
}
