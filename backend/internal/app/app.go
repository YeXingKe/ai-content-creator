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

// New 创建应用实例
func New(cfg *config.Config) (*App, error) {
	// 初始化数据库
	db, err := initDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	// 初始化各层
	userStore := store.NewUserStore(db) // SQL、表、字段
	userService := service.NewUserService(userStore) // 业务逻辑、校验
	userHandler := handler.NewUserHandler(userService) // 处理请求、响应
	healthHandler := handler.NewHealthHandler()
    
	// Go 没有 try/catch，习惯用 「结果 + error」 表示成败
	// & 表示取地址，返回的是 *App 指针，不是拷贝整个结构体。
	return &App{
		Config:        cfg,           // 配置（端口、数据库地址等）
		DB:            db,            // 数据库连接（Close 时要关）
		UserHandler:   userHandler,   // 用户接口（注册、登录等）
		HealthHandler: healthHandler, // 健康检查 /api/health
		UserService:   userService,   // 用户业务（中间件鉴权也会用）
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
