package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/ai-content-creator/backend/docs"
	"github.com/ai-content-creator/backend/internal/app"
	"github.com/ai-content-creator/backend/internal/config"
	"github.com/ai-content-creator/backend/internal/middleware"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var (
	configFile = flag.String("config", "config.yaml", "配置文件路径")
)

// @title AI Content Creator API
// @version 1.0
// @description Go backend API 文档
// @BasePath /api
func main() {
	// 加载配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// 初始化应用
	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("init app: %v", err)
	}
	defer application.Close()

	// 创建路由器
	r := gin.Default()

	// 全局中间件
	r.Use(middleware.CORS())

	// 配置 Session
	if err := middleware.SetupSession(r, cfg); err != nil {
		log.Fatalf("setup session: %v", err)
	}

	// 注册路由
	api := r.Group(cfg.Server.ContextPath)
	{
		api.GET("/v3/api-docs", func(c *gin.Context) {
			c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.SwaggerInfo.ReadDoc()))
		})
		api.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		// 健康检查
		api.GET("/health", application.HealthHandler.Check)
	}

	// 启动服务器
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("server starting at http://localhost%s%s", addr, cfg.Server.ContextPath)
	if err := r.Run(addr); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
