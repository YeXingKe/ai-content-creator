package model

// RegisterRequest 用户注册请求
type RegisterRequest struct {
	UserAccount   string `json:"userAccount" binding:"required,min=4" example:"zhangsan"`   // 登录账号，至少 4 位
	UserPassword  string `json:"userPassword" binding:"required,min=8" example:"12345678"`  // 登录密码，至少 8 位
	CheckPassword string `json:"checkPassword" binding:"required,min=8" example:"12345678"` // 确认密码，需与 userPassword 一致
}

// LoginRequest 用户登录请求
type LoginRequest struct {
	UserAccount  string `json:"userAccount" binding:"required,min=4" example:"zhangsan"`  // 登录账号
	UserPassword string `json:"userPassword" binding:"required,min=8" example:"12345678"` // 登录密码
}

// AddUserRequest 创建用户请求（管理员）
type AddUserRequest struct {
	UserAccount string  `json:"userAccount" binding:"required" example:"newuser"` // 登录账号
	UserName    *string `json:"userName" example:"新用户"`                              // 用户昵称（可选）
	UserAvatar  *string `json:"userAvatar" example:"https://example.com/avatar.png"` // 头像 URL（可选）
	UserProfile *string `json:"userProfile" example:"个人简介"`                          // 个人简介（可选）
	UserRole    string  `json:"userRole" example:"user" enums:"user,admin,vip"`      // 用户角色：user / admin / vip
}

// UpdateUserRequest 更新用户请求（管理员）
type UpdateUserRequest struct {
	ID          int64   `json:"id" binding:"required" example:"1"`                     // 用户主键 ID
	UserName    *string `json:"userName" example:"新昵称"`                                // 用户昵称（可选，传 null 表示不更新）
	UserAvatar  *string `json:"userAvatar" example:"https://example.com/avatar.png"`   // 头像 URL（可选）
	UserProfile *string `json:"userProfile" example:"更新后的简介"`                          // 个人简介（可选）
	UserRole    *string `json:"userRole" example:"vip" enums:"user,admin,vip"`         // 用户角色（可选）
}

// QueryUserRequest 查询用户请求
type QueryUserRequest struct {
	ID          *int64  `json:"id" example:"1"`                    // 按用户 ID 筛选（可选）
	UserAccount *string `json:"userAccount" example:"zhangsan"`    // 按账号模糊筛选（可选）
	UserName    *string `json:"userName" example:"张三"`             // 按昵称模糊筛选（可选）
	UserProfile *string `json:"userProfile"`                       // 按简介模糊筛选（可选）
	UserRole    *string `json:"userRole" example:"user"`           // 按角色筛选（可选）
	PageNum     int64   `json:"pageNum" example:"1"`                 // 页码，从 1 开始
	PageSize    int64   `json:"pageSize" example:"10"`               // 每页条数
	SortField   *string `json:"sortField" example:"createTime"`    // 排序字段（可选）
	SortOrder   *string `json:"sortOrder" example:"desc"`          // 排序方向：asc / desc（可选）
}

// DeleteRequest 删除请求
type DeleteRequest struct {
	ID int64 `json:"id" binding:"required,gt=0" example:"1"` // 要删除的记录主键 ID（用户或文章）
}

// PageResult 分页结果
type PageResult struct {
	Total    int64       `json:"total" example:"100"`    // 符合条件的总条数
	Records  interface{} `json:"records"`                // 当前页数据列表（用户列表或文章列表）
	PageNum  int64       `json:"pageNum" example:"1"`    // 当前页码
	PageSize int64       `json:"pageSize" example:"10"`  // 每页条数
}
