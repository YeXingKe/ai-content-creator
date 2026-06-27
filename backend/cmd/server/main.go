package main // 程序入口包，编译为可执行文件

import (
	"flag"     // 命令行参数解析
	"fmt"      // 格式化字符串（如拼接地址）
	"log"      // 日志输出
	"net/http" // HTTP 状态码、响应等

	"github.com/ai-content-creator/backend/docs"                // Swagger 文档（swag 生成）
	"github.com/ai-content-creator/backend/internal/app"        // 应用初始化（DB、Handler、Service 组装）
	"github.com/ai-content-creator/backend/internal/common"       // 公共常量（角色等）
	"github.com/ai-content-creator/backend/internal/config"     // 配置加载
	"github.com/ai-content-creator/backend/internal/middleware" // 中间件（CORS、Session、鉴权）
	"github.com/gin-gonic/gin"                                  // Web 框架
	swaggerFiles "github.com/swaggo/files"                      // Swagger 静态资源
	ginSwagger "github.com/swaggo/gin-swagger"                  // Gin 的 Swagger UI 集成
)

var (
	configFile = flag.String("config", "config.yaml", "配置文件路径") // 可通过 -config 指定配置文件（当前未使用）
)

// @title AI Content Creator API
// @version 1.0
// @description Go backend API 文档
// @BasePath /api
func main() {
	cfg, err := config.LoadConfig("config.yaml") // 读取 config.yaml（端口、数据库、Redis、AI 等配置）
	if err != nil {
		log.Fatalf("load config: %v", err) // 配置加载失败则终止启动
	}

	application, err := app.New(cfg) // 初始化 DB、Store、Service、Handler 并完成依赖注入
	if err != nil {
		log.Fatalf("init app: %v", err) // 应用初始化失败则终止启动
	}
	defer application.Close() // 进程退出时关闭数据库连接等资源

	r := gin.Default() // 创建 Gin 引擎（内置 Logger + Recovery 中间件）

	r.Use(middleware.CORS()) // 全局跨域中间件，允许前端 localhost:5173 访问

	if err := middleware.SetupSession(r, cfg); err != nil { // 注册 Redis Session 中间件（Cookie 名 session）
		log.Fatalf("setup session: %v", err) // Session 初始化失败则终止启动
	}

	api := r.Group(cfg.Server.ContextPath) // 路由组，默认前缀 /api（来自 config.yaml 的 context_path）
	{
		api.GET("/health", application.HealthHandler.Check) // 健康检查，返回 ok
		api.GET("/v3/api-docs", func(c *gin.Context) {    // OpenAPI JSON，供前端 openapi2ts 生成 TS 类型
			c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.SwaggerInfo.ReadDoc()))
		})
		api.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler)) // Swagger UI 页面

		user := api.Group("/user") // 用户模块路由组 /api/user
		{
			user.POST("/register", application.UserHandler.Register) // 用户注册（公开）
			user.POST("/login", application.UserHandler.Login)       // 用户登录，写入 Session（公开）

			user.GET("/get/login", application.UserHandler.GetLoginUser) // 获取当前登录用户（需 Session）
			user.POST("/logout", application.UserHandler.Logout)          // 注销，清除 Session
			user.GET("/get/vo", application.UserHandler.GetVO)            // 按 ID 获取用户脱敏信息

			adminAuth := middleware.AuthCheck(application.UserService, common.AdminRole) // 管理员鉴权中间件
			user.POST("/add", adminAuth, application.UserHandler.Add)                  // 创建用户
			user.GET("/get", adminAuth, application.UserHandler.Get)                   // 按 ID 获取用户详情
			user.POST("/delete", adminAuth, application.UserHandler.Delete)             // 删除用户（软删）
			user.POST("/update", adminAuth, application.UserHandler.Update)             // 更新用户
			user.POST("/list/page/vo", adminAuth, application.UserHandler.ListPageVO)  // 分页查询用户列表
		}

		userAuth := middleware.AuthCheck(application.UserService, common.UserRole) // 登录用户鉴权（仅校验已登录）
		article := api.Group("/article")                                           // 文章模块路由组 /api/article
		{
			article.POST("/create", userAuth, application.ArticleHandler.Create)                   // 创建文章任务，返回 taskId
			article.POST("/confirm-title", userAuth, application.ArticleHandler.ConfirmTitle)     // 确认标题，进入下一阶段
			article.POST("/confirm-outline", userAuth, application.ArticleHandler.ConfirmOutline) // 确认大纲
			article.POST("/ai-modify-outline", userAuth, application.ArticleHandler.AiModifyOutline) // AI 修改大纲
			article.GET("/progress/:taskId", application.ArticleHandler.GetProgress)              // SSE 推送生成进度（无需 userAuth，handler 内校验）
			article.GET("/execution-logs/:taskId", application.ArticleHandler.GetExecutionLogs)   // 查询智能体执行日志
			article.GET("/:taskId", userAuth, application.ArticleHandler.Get)                    // 按 taskId 获取文章详情
			article.POST("/list", userAuth, application.ArticleHandler.List)                    // 分页查询文章列表
			article.POST("/delete", userAuth, application.ArticleHandler.Delete)                // 删除文章
		}
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port) // 监听地址，如 :8567
	baseURL := fmt.Sprintf("http://localhost%s%s", addr, cfg.Server.ContextPath)
	log.Printf("server starting at %s", baseURL)
	log.Printf("swagger UI:    %s/swagger/index.html", baseURL)
	log.Printf("openapi JSON:  %s/v3/api-docs", baseURL)
	if err := r.Run(addr); err != nil { // 阻塞启动 HTTP 服务
		log.Fatalf("start server: %v", err) // 启动失败则退出
	}
}
