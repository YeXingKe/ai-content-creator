package middleware

import (
	"strings"

	"github.com/ai-content-creator/backend/internal/common"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 声明结构
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

var jwtSecret []byte

// InitAuth 初始化认证中间件
func InitAuth(secret string) {
	jwtSecret = []byte(secret)
}

// Auth JWT 认证中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Authorization header 获取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			common.ErrorResponse(c, common.ErrUserNotLogin, common.ErrUserNotLoginMsg)
			c.Abort()
			return
		}

		// Bearer token 格式
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			common.ErrorResponse(c, common.ErrTokenInvalid, common.ErrTokenInvalidMsg)
			c.Abort()
			return
		}

		tokenString := parts[1]

		// 解析 token
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			common.ErrorResponse(c, common.ErrTokenInvalid, common.ErrTokenInvalidMsg)
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set(common.ContextKeyUserID, claims.UserID)
		c.Set(common.ContextKeyUsername, claims.Username)

		c.Next()
	}
}

// GetUserID 从上下文获取用户ID
func GetUserID(c *gin.Context) (int64, bool) {
	userID, exists := c.Get(common.ContextKeyUserID)
	if !exists {
		return 0, false
	}
	if uid, ok := userID.(int64); ok {
		return uid, true
	}
	return 0, false
}

// GetUsername 从上下文获取用户名
func GetUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get(common.ContextKeyUsername)
	if !exists {
		return "", false
	}
	if uname, ok := username.(string); ok {
		return uname, true
	}
	return "", false
}
