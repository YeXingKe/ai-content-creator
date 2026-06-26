// Package app 负责应用级「组装」：连接基础设施、创建各层依赖并完成注入。
//
// 为什么需要 app.go？
//   - main.go 只负责读配置、注册路由、启动 HTTP 服务，不应塞满 Store/Service/Handler 的创建逻辑
//   - 依赖关系复杂（DB → Store → Service → Handler，文章模块还涉及 Agent、SSE、COS 等），集中在一处便于维护
//   - 便于测试：测试代码可调用 app.New(cfg) 获得完整应用实例，无需重复写一遍 wiring
//   - 生命周期管理：Close() 统一释放 DB、Redis 等资源
package app

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/ai-content-creator/backend/internal/agent"
	"github.com/ai-content-creator/backend/internal/common"
	"github.com/ai-content-creator/backend/internal/config"
	"github.com/ai-content-creator/backend/internal/handler"
	"github.com/ai-content-creator/backend/internal/service"
	"github.com/ai-content-creator/backend/internal/store"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// App 应用程序容器，持有基础设施与各 Handler，供 main 注册路由时使用
type App struct {
	Config      *config.Config // 全局配置（端口、DB、Redis、AI、COS 等）
	DB          *gorm.DB       // MySQL 连接，供 Store 层使用
	RedisClient *redis.Client  // Redis 连接，Session 等中间件使用

	// Handlers — HTTP 入口，main.go 将其实例绑定到 Gin 路由
	UserHandler    *handler.UserHandler
	ArticleHandler *handler.ArticleHandler
	HealthHandler  *handler.HealthHandler
	// PaymentHandler    *handler.PaymentHandler
	// WebhookHandler    *handler.WebhookHandler
	// StatisticsHandler *handler.StatisticsHandler

	// UserService 暴露给中间件（AuthCheck 需要查当前用户角色）
	UserService *service.UserService
}

// New 创建并组装整个应用：基础设施 → Store → Service → Handler
func New(cfg *config.Config) (*App, error) {
	db, err := initDB(cfg) // 连接 MySQL，配置连接池
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	redisClient, err := initRedis(cfg) // 连接 Redis，Session 依赖它
	if err != nil {
		return nil, fmt.Errorf("init redis: %w", err)
	}

	// --- 数据访问层：每个 Store 封装一张表/一组表的 CRUD ---
	userStore := store.NewUserStore(db)
	articleStore := store.NewArticleStore(db)
	// paymentStore := store.NewPaymentStore(db)
	agentLogStore := store.NewAgentLogStore(db)

	sseManager := common.NewSSEManager() // SSE 连接管理器，文章生成进度推送给前端

	// --- 业务服务层：组合 Store，实现业务规则 ---
	userService := service.NewUserService(userStore)
	quotaService := service.NewQuotaService(userStore)       // 文章配额校验与扣减
	agentLogService := service.NewAgentLogService(agentLogStore) // 智能体执行日志
	// statisticsService := service.NewStatisticsService(db, userStore, articleStore, redisClient)

	// COS 对象存储：配置了密钥才启用，否则配图使用原始 URL
	cosEnabled := cfg.COS.Bucket != "" && cfg.COS.SecretID != "" && cfg.COS.SecretKey != ""
	var cosService *service.CosService
	if cosEnabled {
		cosService = service.NewCosService(cfg.COS)
		log.Printf("COS 服务已启用, bucket=%s, region=%s", cfg.COS.Bucket, cfg.COS.Region)
	} else {
		log.Println("COS 服务未配置，图片将使用原始 URL")
	}

	// 图片服务：Pexels 等，通过策略模式统一注册与调用
	pexelsService := service.NewPexelsService(cfg)
	// iconifyService := service.NewIconifyService(cfg.Iconify)
	// mermaidService := service.NewMermaidService(cfg.Mermaid)
	// nanoBananaService := service.NewNanoBananaService(cfg.NanoBanana)
	// svgDiagramService := service.NewSVGDiagramService(cfg.SVGDiagram, cfg.AI)
	// emojiPackService := service.NewEmojiPackService(cfg.EmojiPack)
	// picsumService := service.NewPicsumService() // 降级服务

	imageStrategy := service.NewImageServiceStrategy(cosService, cosEnabled) // 策略上下文，按类型路由到具体图片服务
	imageStrategy.RegisterService(pexelsService)
	// imageStrategy.RegisterService(iconifyService)
	// imageStrategy.RegisterService(mermaidService)
	// imageStrategy.RegisterService(nanoBananaService)
	// imageStrategy.RegisterService(svgDiagramService)
	// imageStrategy.RegisterService(emojiPackService)
	// imageStrategy.RegisterService(picsumService) // 注册降级服务

	log.Println("图片服务策略初始化完成，已注册 7 个图片服务（含降级服务）")

	// 文章智能体：调用 LLM 生成标题/大纲/正文，并写 AgentLog、推 SSE
	agentService, err := service.NewArticleAgentService(cfg, imageStrategy, agentLogService, sseManager)
	if err != nil {
		return nil, fmt.Errorf("init agent service: %w", err)
	}

	// 多智能体编排器：按阶段调度 title/outline/content 等 Agent，复用 agentService 的 LLM 实例
	orchestrator := agent.NewArticleAgentOrchestrator(
		cfg,
		agentService.GetLLM(),
		agentLogService,
		sseManager,
		imageStrategy,
	)

	log.Printf("智能体编排器初始化完成，启用状态: %v", cfg.Agent.Orchestrator.Enabled)

	// 文章业务：串联 Store、Agent、编排器、配额、SSE
	articleService := service.NewArticleService(
		articleStore,
		agentService,
		orchestrator,
		cfg,
		quotaService,
		sseManager,
	)

	// paymentService := service.NewPaymentService(&cfg.Stripe, userStore, paymentStore, db)

	// --- 处理器层：解析 HTTP 请求，调用 Service，返回 JSON/SSE ---
	userHandler := handler.NewUserHandler(userService)
	articleHandler := handler.NewArticleHandler(articleService, userService, agentLogService, sseManager)
	healthHandler := handler.NewHealthHandler()
	// paymentHandler := handler.NewPaymentHandler(paymentService)
	// webhookHandler := handler.NewWebhookHandler(paymentService)
	// statisticsHandler := handler.NewStatisticsHandler(statisticsService)

	return &App{
		Config:         cfg,
		DB:             db,
		RedisClient:    redisClient,
		UserHandler:    userHandler,
		ArticleHandler: articleHandler,
		HealthHandler:  healthHandler,
		// PaymentHandler:    paymentHandler,
		// WebhookHandler:    webhookHandler,
		// StatisticsHandler: statisticsHandler,
		UserService: userService,
	}, nil
}

// initDB 初始化 MySQL 连接并设置连接池参数
func initDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.Database.GetDSN() // 从配置拼 DSN：user:pass@tcp(host:port)/dbname?...

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // 打印 SQL 日志，便于开发调试
	})
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	sqlDB, err := db.DB() // 获取底层 *sql.DB，用于设置连接池
	if err != nil {
		return nil, fmt.Errorf("get database instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns) // 空闲连接数上限
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns) // 最大打开连接数

	log.Println("database connected")
	return db, nil
}

// initRedis 初始化 Redis 客户端并 Ping 验证连通性
func initRedis(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.GetRedisAddr(), // host:port
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB, // 默认 0，Session 常用独立 DB 号
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil { // 启动时探测，避免运行中才发现连不上
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	log.Println("redis connected")
	return client, nil
}

// Close 进程退出时释放 DB、Redis 连接
func (a *App) Close() error {
	sqlDB, err := a.DB.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}

	if a.RedisClient != nil {
		if err := a.RedisClient.Close(); err != nil {
			log.Printf("close redis: %v", err)
		}
	}

	return nil
}
