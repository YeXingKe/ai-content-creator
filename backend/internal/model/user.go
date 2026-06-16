package model

import "time"

// User 用户模型
type User struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Username  string    `json:"username" gorm:"type:varchar(50);uniqueIndex;not null;comment:用户名"`
	Email     string    `json:"email" gorm:"type:varchar(100);uniqueIndex;not null;comment:邮箱"`
	Password  string    `json:"-" gorm:"type:varchar(100);not null;comment:密码"`
	Nickname  string    `json:"nickname" gorm:"type:varchar(50);comment:昵称"`
	Avatar    string    `json:"avatar" gorm:"type:varchar(255);comment:头像"`
	Bio       string    `json:"bio" gorm:"type:varchar(200);comment:个人简介"`
	Status    int       `json:"status" gorm:"type:tinyint;default:1;comment:状态:1正常,2禁用"`
	LastLogin time.Time `json:"last_login" gorm:"comment:最后登录时间"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;comment:创建时间"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime;comment:更新时间"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// UserStatus 用户状态常量
const (
	UserStatusActive  = 1 // 正常
	UserStatusDisabled = 2 // 禁用
)

// IsActive 检查用户是否激活
func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}

// SensitiveFields 敏感字段列表（用于日志过滤等）
var SensitiveFields = []string{
	"password",
	"old_password",
	"new_password",
}
