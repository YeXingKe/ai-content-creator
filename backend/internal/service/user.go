package service

import (
	"errors"
	"time"

	"github.com/ai-content-creator/backend/internal/common"
	"github.com/ai-content-creator/backend/internal/model"
	"github.com/ai-content-creator/backend/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte

// UserService 用户服务
type UserService struct {
	store *store.UserStore
}

// NewUserService 创建用户服务
func NewUserService(userStore *store.UserStore) *UserService {
	return &UserService{
		store: userStore,
	}
}

// InitJWT 初始化 JWT 密钥
func InitJWT(secret string) {
	jwtSecret = []byte(secret)
}

// Register 用户注册
func (s *UserService) Register(req *model.RegisterRequest) (*model.User, *common.BizError) {
	// 检查用户名是否已存在
	exists, err := s.store.ExistsByUsername(req.Username)
	if err != nil {
		return nil, common.NewError(common.ErrDBInsert, "检查用户名失败", err)
	}
	if exists {
		return nil, common.NewError(common.ErrUserExists, common.ErrUserExistsMsg, nil)
	}

	// 检查邮箱是否已存在
	exists, err = s.store.ExistsByEmail(req.Email)
	if err != nil {
		return nil, common.NewError(common.ErrDBInsert, "检查邮箱失败", err)
	}
	if exists {
		return nil, common.NewError(common.ErrUserExists, "邮箱已被使用", nil)
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, common.NewError(common.ErrInternalServer, "密码加密失败", err)
	}

	// 创建用户
	user := &model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashedPassword),
		Nickname: req.Username, // 默认昵称为用户名
		Status:   model.UserStatusActive,
	}

	if err := s.store.Create(user); err != nil {
		return nil, common.NewError(common.ErrDBInsert, common.ErrDBInsertMsg, err)
	}

	return user, nil
}

// Login 用户登录
func (s *UserService) Login(req *model.LoginRequest) (string, *model.User, *common.BizError) {
	// 查找用户（支持用户名或邮箱登录）
	user, err := s.store.FindByUsernameOrEmail(req.Username)
	if err != nil {
		return "", nil, common.NewError(common.ErrDBQuery, common.ErrDBQueryMsg, err)
	}
	if user == nil {
		return "", nil, common.NewError(common.ErrUserNotFound, common.ErrUserNotFoundMsg, nil)
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return "", nil, common.NewError(common.ErrPasswordWrong, common.ErrPasswordWrongMsg, nil)
	}

	// 检查用户状态
	if !user.IsActive() {
		return "", nil, common.NewError(common.ErrUserDisabled, common.ErrUserDisabledMsg, nil)
	}

	// 更新最后登录时间
	user.LastLogin = time.Now()
	if err := s.store.Update(user); err != nil {
		return "", nil, common.NewError(common.ErrDBUpdate, common.ErrDBUpdateMsg, err)
	}

	// 生成 JWT token
	token, err := s.generateToken(user)
	if err != nil {
		return "", nil, common.NewError(common.ErrInternalServer, "生成令牌失败", err)
	}

	return token, user, nil
}

// GetByID 根据ID获取用户
func (s *UserService) GetByID(id int64) (*model.User, *common.BizError) {
	user, err := s.store.FindByID(id)
	if err != nil {
		return nil, common.NewError(common.ErrDBQuery, common.ErrDBQueryMsg, err)
	}
	if user == nil {
		return nil, common.NewError(common.ErrUserNotFound, common.ErrUserNotFoundMsg, nil)
	}
	return user, nil
}

// UpdateProfile 更新用户资料
func (s *UserService) UpdateProfile(userID int64, req *model.UpdateUserRequest) (*model.User, *common.BizError) {
	// 获取用户
	user, err := s.GetByID(userID)
	if err != nil {
		return nil, err
	}

	// 更新字段
	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}
	if req.Bio != "" {
		user.Bio = req.Bio
	}

	// 保存更新
	if err := s.store.Update(user); err != nil {
		return nil, common.NewError(common.ErrDBUpdate, common.ErrDBUpdateMsg, err)
	}

	return user, nil
}

// ChangePassword 修改密码
func (s *UserService) ChangePassword(userID int64, oldPassword, newPassword string) *common.BizError {
	// 获取用户
	user, err := s.GetByID(userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return common.NewError(common.ErrPasswordWrong, "原密码错误", nil)
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return common.NewError(common.ErrInternalServer, "密码加密失败", err)
	}

	// 更新密码
	user.Password = string(hashedPassword)
	if err := s.store.Update(user); err != nil {
		return common.NewError(common.ErrDBUpdate, common.ErrDBUpdateMsg, err)
	}

	return nil
}

// generateToken 生成 JWT token
func (s *UserService) generateToken(user *model.User) (string, error) {
	if len(jwtSecret) == 0 {
		return "", errors.New("JWT secret not initialized")
	}

	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"exp":      time.Now().Add(168 * time.Hour).Unix(), // 7天过期
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}
