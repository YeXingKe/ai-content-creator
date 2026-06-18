package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/ai-content-creator/backend/internal/common"
	"github.com/ai-content-creator/backend/internal/model"
	"github.com/ai-content-creator/backend/internal/service"
)

// AuthCheck 权限校验中间件
func AuthCheck(userSvc *service.UserService, mustRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		// 获取当前登录用户
		loginUser, err := userSvc.GetLoginUser(session)
		if err != nil {
			c.JSON(http.StatusUnauthorized, common.Error(common.ErrNotLogin))
			c.Abort() // 终止后续 handler，不再执行路由后面的逻辑
			return // 退出当前函数，不再执行后续代码
		}

		// 不需要权限，直接放行
		if mustRole == "" {
			c.Set("loginUser", loginUser)
			c.Next()
			return
		}

		// 检查角色权限
		mustRoleEnum := model.UserRole(mustRole)
		if !mustRoleEnum.IsValid() {
			c.Set("loginUser", loginUser)
			c.Next()
			return
		}

		userRoleEnum := model.UserRole(loginUser.UserRole)
		if !userRoleEnum.IsValid() {
			c.JSON(http.StatusForbidden, common.Error(common.ErrNoAuth))
			c.Abort()
			return
		}

		// 要求管理员权限，但用户不是管理员
		if mustRoleEnum == model.RoleAdmin && userRoleEnum != model.RoleAdmin {
			c.JSON(http.StatusForbidden, common.Error(common.ErrNoAuth))
			c.Abort()
			return
		}

		// 通过权限校验
		// Gin 框架里 *gin.Context 的方法，用来在同一次 HTTP 请求的生命周期内存键值对，供后面的中间件或 handler 读取。
		c.Set("loginUser", loginUser)
		c.Next()
	}
}