package handler

import (
	"github.com/ai-content-creator/backend/internal/common"
	"github.com/ai-content-creator/backend/internal/model"
	"github.com/ai-content-creator/backend/internal/service"
	"github.com/gin-gonic/gin"
)

var userService *service.UserService

// InitUserService 初始化用户服务（应该在应用启动时调用）
func InitUserService(svc *service.UserService) {
	userService = svc
}

// Register 用户注册
func Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResponse(c, common.ErrParamInvalid, common.ErrParamInvalidMsg)
		return
	}

	user, err := userService.Register(&req)
	if err != nil {
		common.BizErrorResponse(c, err)
		return
	}

	common.SuccessResponse(c, gin.H{
		"user_id":   user.ID,
		"username":  user.Username,
		"email":     user.Email,
		"created_at": user.CreatedAt,
	})
}

// Login 用户登录
func Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResponse(c, common.ErrParamInvalid, common.ErrParamInvalidMsg)
		return
	}

	token, user, err := userService.Login(&req)
	if err != nil {
		common.BizErrorResponse(c, err)
		return
	}

	common.SuccessResponse(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// GetUserProfile 获取用户信息
func GetUserProfile(c *gin.Context) {
	userID := c.GetInt64(common.ContextKeyUserID)

	user, err := userService.GetByID(userID)
	if err != nil {
		common.BizErrorResponse(c, err)
		return
	}

	common.SuccessResponse(c, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"nickname":   user.Nickname,
		"avatar":     user.Avatar,
		"bio":        user.Bio,
		"status":     user.Status,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	})
}

// UpdateUserProfile 更新用户信息
func UpdateUserProfile(c *gin.Context) {
	userID := c.GetInt64(common.ContextKeyUserID)

	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResponse(c, common.ErrParamInvalid, common.ErrParamInvalidMsg)
		return
	}

	user, err := userService.UpdateProfile(userID, &req)
	if err != nil {
		common.BizErrorResponse(c, err)
		return
	}

	common.SuccessResponse(c, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"nickname":   user.Nickname,
		"avatar":     user.Avatar,
		"bio":        user.Bio,
		"updated_at": user.UpdatedAt,
	})
}
