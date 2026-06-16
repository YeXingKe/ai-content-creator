package store

import (
	"time"

	"github.com/ai-content-creator/backend/internal/config"
	"github.com/ai-content-creator/backend/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Store 数据存储结构
type Store struct {
	DB        *gorm.DB
	UserStore *UserStore
}

// UserStore 用户数据访问层
type UserStore struct {
	db *gorm.DB
}

// New 创建数据存储实例
func New(cfg *config.Config) (*Store, error) {
	// 连接数据库
	db, err := gorm.Open(mysql.Open(cfg.Database.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}

	// 获取通用数据库对象 sql.DB ，然后使用其提供的功能
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)

	// 自动迁移
	if err := db.AutoMigrate(&model.User{}); err != nil {
		return nil, err
	}

	// 创建用户存储
	userStore := &UserStore{db: db}

	return &Store{
		DB:        db,
		UserStore: userStore,
	}, nil
}

// Close 关闭数据库连接
func (s *Store) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Create 创建用户
func (us *UserStore) Create(user *model.User) error {
	return us.db.Create(user).Error
}

// FindByID 根据ID查找用户
func (us *UserStore) FindByID(id int64) (*model.User, error) {
	var user model.User
	err := us.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByUsername 根据用户名查找用户
func (us *UserStore) FindByUsername(username string) (*model.User, error) {
	var user model.User
	err := us.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByEmail 根据邮箱查找用户
func (us *UserStore) FindByEmail(email string) (*model.User, error) {
	var user model.User
	err := us.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindByUsernameOrEmail 根据用户名或邮箱查找用户
func (us *UserStore) FindByUsernameOrEmail(usernameOrEmail string) (*model.User, error) {
	var user model.User
	err := us.db.Where("username = ? OR email = ?", usernameOrEmail, usernameOrEmail).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// ExistsByUsername 检查用户名是否存在
func (us *UserStore) ExistsByUsername(username string) (bool, error) {
	var count int64
	err := us.db.Model(&model.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

// ExistsByEmail 检查邮箱是否存在
func (us *UserStore) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := us.db.Model(&model.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

// Update 更新用户
func (us *UserStore) Update(user *model.User) error {
	return us.db.Save(user).Error
}

// Delete 删除用户
func (us *UserStore) Delete(id int64) error {
	return us.db.Delete(&model.User{}, id).Error
}

// List 获取用户列表
func (us *UserStore) List(offset, limit int) ([]*model.User, int64, error) {
	var users []*model.User
	var total int64

	// 获取总数
	if err := us.db.Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取列表
	err := us.db.Offset(offset).Limit(limit).Order("created_at DESC").Find(&users).Error
	return users, total, err
}

// UpdateLastLogin 更新最后登录时间
func (us *UserStore) UpdateLastLogin(id int64, lastLogin time.Time) error {
	return us.db.Model(&model.User{}).Where("id = ?", id).Update("last_login", lastLogin).Error
}
