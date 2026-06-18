package model

// Go 结构体字段默认是 UserAccount（大写开头），而前端 JSON 通常是 userAccount（小写驼峰）
// 所以需要用 json:"userAccount" 标签告诉编译器，JSON 字段名是 userAccount
// 只有 json：表示 JSON 里有 userName 这个键
// 没有 binding：表示 可选，不传也可以
// 用 *string：nil 表示「没传这个字段」

// string 字符串本身
// *string 指向字符串的指针（可以指向某个值，也可以是 nil）

// RegisterRequest 用户注册请求
type RegisterRequest struct {
	UserAccount   string `json:"userAccount" binding:"required,min=4"`
	UserPassword  string `json:"userPassword" binding:"required,min=8"`
	CheckPassword string `json:"checkPassword" binding:"required,min=8"`
}

// LoginRequest 用户登录请求
type LoginRequest struct {
	UserAccount  string `json:"userAccount" binding:"required,min=4"` // 账号，必填字符串
	UserPassword string `json:"userPassword" binding:"required,min=8"` // JSON 字段名，和前端一致（驼峰）
}

// AddUserRequest 创建用户请求（管理员）
type AddUserRequest struct {
	UserAccount string  `json:"userAccount" binding:"required"`
	UserName    *string `json:"userName"`
	UserAvatar  *string `json:"userAvatar"`
	UserProfile *string `json:"userProfile"`
	UserRole    string  `json:"userRole"`
}

// UpdateUserRequest 更新用户请求（管理员）
type UpdateUserRequest struct {
	ID          int64   `json:"id" binding:"required"`
	UserName    *string `json:"userName"`
	UserAvatar  *string `json:"userAvatar"`
	UserProfile *string `json:"userProfile"`
	UserRole    *string `json:"userRole"`
}

// QueryUserRequest 查询用户请求
type QueryUserRequest struct {
	ID          *int64  `json:"id"`
	UserAccount *string `json:"userAccount"`
	UserName    *string `json:"userName"`
	UserProfile *string `json:"userProfile"`
	UserRole    *string `json:"userRole"`
	PageNum     int64   `json:"pageNum"`
	PageSize    int64   `json:"pageSize"`
	SortField   *string `json:"sortField"`
	SortOrder   *string `json:"sortOrder"`
}

// DeleteRequest 删除请求
type DeleteRequest struct {
	ID int64 `json:"id" binding:"required,gt=0"`
}

// PageResult 分页结果
type PageResult struct {
	Total    int64       `json:"total"`
	Records  interface{} `json:"records"`
	PageNum  int64       `json:"pageNum"`
	PageSize int64       `json:"pageSize"`
}
