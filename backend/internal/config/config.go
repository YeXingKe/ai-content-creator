package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// 作用：把 config.yaml 里的配置读进 Go 结构体，供全项目使用。

// Config 应用配置
// Viper 用 mapstructure 把 yaml 映射到 Go 结构体
// mapstructure 标签 yaml 蛇形 ↔ Go 驼峰
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Session  SessionConfig  `mapstructure:"session"`
	Log      LogConfig      `mapstructure:"log"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port        int    `mapstructure:"port"`
	ContextPath string `mapstructure:"context_path"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Name         string `mapstructure:"name"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	DB       int    `mapstructure:"db"`
	Password string `mapstructure:"password"`
}

// SessionConfig Session配置
type SessionConfig struct {
	Secret string `mapstructure:"secret"`
	MaxAge int    `mapstructure:"max_age"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level    string `mapstructure:"level"`
	FilePath string `mapstructure:"file_path"`
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New() // 新建一个 Viper 实例
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// 从磁盘读文件；文件不存在、权限问题等会在这里失败
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	// 把 Viper 内存里的配置反序列化进 Config 结构体
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &config, nil
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Name,
	)
}

// GetRedisAddr 获取 Redis 地址
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
