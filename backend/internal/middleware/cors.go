package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
// 返回类型 gin.HandlerFunc，即 Gin 中间件/处理函数的标准签名：func(c *gin.Context)
// CORS() 本身不处理请求，而是返回一个函数，这个函数会被 Gin 框架自动调用。
// 作用：处理跨域请求，允许前端从 http://localhost:5173 访问本后端。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) { // 每次请求都会执行这个函数
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173") // 允许前端从 http://localhost:5173 访问本后端
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true") // 允许携带凭证（如 Cookies）
		c.Writer.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With") // 允许的请求头
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE") // 允许的请求方法

		if c.Request.Method == "OPTIONS" { // 如果是 OPTIONS 请求，则返回 204 状态码
			c.AbortWithStatus(204)
			return
		}

		c.Next() // 继续处理下一个中间件或路由处理函数
	}
}
