package main

import (
	"flag"     // 命令行参数解析
	"fmt"      // 格式化字符串（如拼接地址）
	"log"      // 日志输出
	"net/http" // HTTP 状态码、响应等

	"github.com/ai-content-creator/backend/docs"         // Swagger 文档（swag 生成）
	"github.com/ai-content-creator/backend/internal/app" // 应用初始化（DB、Handler 等）
	"github.com/ai-content-creator/backend/internal/common"
	"github.com/ai-content-creator/backend/internal/config"     // 配置加载
	"github.com/ai-content-creator/backend/internal/middleware" // 中间件（CORS、Session）
	"github.com/gin-gonic/gin"                                  // Web 框架
	swaggerFiles "github.com/swaggo/files"                      // Swagger 静态资源
	ginSwagger "github.com/swaggo/gin-swagger"                  // Gin 的 Swagger UI 集成
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
		// 健康检查
		api.GET("/health", application.HealthHandler.Check)
		api.GET("/v3/api-docs", func(c *gin.Context) {
			c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.SwaggerInfo.ReadDoc()))
		})
		api.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		// 用户路由
		user := api.Group("/user")
		{
			// 无需登录
			user.POST("/register", application.UserHandler.Register)
			user.POST("/login", application.UserHandler.Login)

			// 需要登录
			user.GET("/get/login", application.UserHandler.GetLoginUser)
			user.POST("/logout", application.UserHandler.Logout)
			user.GET("/get/vo", application.UserHandler.GetVO)

			// 需要管理员权限
			adminAuth := middleware.AuthCheck(application.UserService, common.AdminRole)
			user.POST("/add", adminAuth, application.UserHandler.Add)
			user.GET("/get", adminAuth, application.UserHandler.Get)
			user.POST("/delete", adminAuth, application.UserHandler.Delete)
			user.POST("/update", adminAuth, application.UserHandler.Update)
			user.POST("/list/page/vo", adminAuth, application.UserHandler.ListPageVO)
		}

		// 文章路由
		userAuth := middleware.AuthCheck(application.UserService, common.UserRole)
		article := api.Group("/article")
		{
			article.POST("/create", userAuth, application.ArticleHandler.Create)
			article.POST("/confirm-title", userAuth, application.ArticleHandler.ConfirmTitle)
			article.POST("/confirm-outline", userAuth, application.ArticleHandler.ConfirmOutline)
			article.POST("/ai-modify-outline", userAuth, application.ArticleHandler.AiModifyOutline)
			article.GET("/progress/:taskId", application.ArticleHandler.GetProgress)
			article.GET("/execution-logs/:taskId", application.ArticleHandler.GetExecutionLogs)
			article.GET("/:taskId", userAuth, application.ArticleHandler.Get)
			article.POST("/list", userAuth, application.ArticleHandler.List)
			article.POST("/delete", userAuth, application.ArticleHandler.Delete)
		}
	}


	// 启动服务器
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("server starting at http://localhost%s%s", addr, cfg.Server.ContextPath)
	if err := r.Run(addr); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
