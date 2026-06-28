package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler 健康检查处理器
type HealthHandler struct{}

// NewHealthHandler 创建健康检查处理器
func NewHealthHandler() *HealthHandler {
	 return &HealthHandler{}
}

// Check 健康检查
// @Summary      健康检查
// @Description  服务存活探测，返回纯文本 ok
// @Tags         系统
// @Produce      plain
// @Success      200  {string}  string  "ok"
// @Router       /health [get]
func (h *HealthHandler) Check(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}