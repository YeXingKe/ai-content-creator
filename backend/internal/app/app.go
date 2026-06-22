package app

import (
	"fmt"
	"log"

	"github.com/ai-content-creator/backend/internal/config"
	"github.com/ai-content-creator/backend/internal/handler"
	"github.com/ai-content-creator/backend/internal/service"
	"github.com/ai-content-creator/backend/internal/store"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// App 应用程序
type App struct {
	Config *config.Config
	DB     *gorm.DB

	// Handlers
	UserHandler   *handler.UserHandler
	HealthHandler *handler.HealthHandler

	// Services (用于中间件)
	UserService *service.UserService
}

func New(cfg *config.Config) (*App, error) {
    db, err := initDB(cfg)
    if err != nil {
        return nil, fmt.Errorf("init database: %w", err)
    }

    userStore := store.NewUserStore(db)
    articleStore := store.NewArticleStore(db)
    sseManager := common.NewSSEManager()

    userService := service.NewUserService(userStore)
    quotaService := service.NewQuotaService(userStore)
    pexelsService := service.NewPexelsService(cfg)
    cosService := service.NewCosService()

    agentService, err := service.NewArticleAgentService(cfg, pexelsService, cosService, sseManager)
    if err != nil {
        return nil, fmt.Errorf("init agent service: %w", err)
    }

    articleService := service.NewArticleService(articleStore, agentService, quotaService, sseManager)

    return &App{
        Config:         cfg,
        DB:             db,
        UserHandler:    handler.NewUserHandler(userService),
        ArticleHandler: handler.NewArticleHandler(articleService, userService, sseManager),
        HealthHandler:  handler.NewHealthHandler(),
        UserService:    userService,
    }, nil
}


// initDB 初始化数据库
func initDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.Database.GetDSN() // 数据库连接字符串

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get database instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns) // 空闲连接数上限
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns) // 最大打开连接数

	log.Println("database connected")
	return db, nil
}

// Close 关闭资源
func (a *App) Close() error {
	sqlDB, err := a.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
