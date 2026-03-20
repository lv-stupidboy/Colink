// Package config 提供配置管理功能
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Claude   ClaudeConfig   `mapstructure:"claude"`
	Sandbox  SandboxConfig  `mapstructure:"sandbox"`
	Agent    AgentConfig    `mapstructure:"agent"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	MCP      MCPConfig      `mapstructure:"mcp"`
	Auth     AuthConfig     `mapstructure:"auth"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// DBType 数据库类型
type DBType string

const (
	DBTypeSQLite DBType = "sqlite"
	DBTypeMySQL  DBType = "mysql"
)

// MySQLConfig MySQL数据库配置
type MySQLConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Database        string `mapstructure:"database"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	Schema          string `mapstructure:"schema"`
	Charset         string `mapstructure:"charset"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

// ApplyDefaults 设置MySQL配置默认值
func (c *MySQLConfig) ApplyDefaults() {
	if c.Port == 0 {
		c.Port = 3306
	}
	if c.Charset == "" {
		c.Charset = "utf8mb4"
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 10
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 5
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = 300
	}
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type  DBType      `mapstructure:"type"`  // 数据库类型: sqlite | mysql
	Path  string      `mapstructure:"path"`  // SQLite 数据库文件路径
	MySQL MySQLConfig `mapstructure:"mysql"` // MySQL 配置
}

// ApplyDefaults 设置数据库配置默认值
func (c *DatabaseConfig) ApplyDefaults() {
	if c.Type == "" {
		c.Type = DBTypeSQLite
	}
	if c.Path == "" {
		c.Path = "./data/isdp.db"
	}
	c.MySQL.ApplyDefaults()
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// ClaudeConfig Claude CLI配置
type ClaudeConfig struct {
	Path         string        `mapstructure:"path"`
	DefaultModel string        `mapstructure:"default_model"`
	Timeout      time.Duration `mapstructure:"timeout"`
}

// SandboxConfig 沙箱配置
type SandboxConfig struct {
	PortRangeStart  int    `mapstructure:"port_range_start"`
	PortRangeEnd    int    `mapstructure:"port_range_end"`
	DefaultCPULimit int    `mapstructure:"default_cpu_limit"`
	DefaultMemLimit int    `mapstructure:"default_memory_limit"`
	Network         string `mapstructure:"network"`
	ReposDir        string `mapstructure:"repos_dir"`
}

// AgentConfig Agent配置
type AgentConfig struct {
	MaxDepth        int `mapstructure:"max_depth"`
	MaxRetries      int `mapstructure:"max_retries"`
	ContextMaxLines int `mapstructure:"context_max_lines"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// MCPConfig MCP配置
type MCPConfig struct {
	BaseURL  string        `mapstructure:"base_url"`
	TokenTTL time.Duration `mapstructure:"token_ttl"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	InviteCode string `mapstructure:"invite_code"` // 访问邀请码，为空则不启用验证
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置默认值
	setDefaults()

	// 读取配置文件
	v.SetConfigFile(configPath)
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 应用默认值（确保零值字段有合理的默认值）
	cfg.Database.ApplyDefaults()

	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults() {
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("database.path", "./data/isdp.db")
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("claude.path", "claude")
	viper.SetDefault("claude.default_model", "claude-sonnet-4-6")
	viper.SetDefault("claude.timeout", "30m")
	viper.SetDefault("sandbox.port_range_start", 30000)
	viper.SetDefault("sandbox.port_range_end", 40000)
	viper.SetDefault("sandbox.default_cpu_limit", 2)
	viper.SetDefault("sandbox.default_memory_limit", 4096)
	viper.SetDefault("sandbox.network", "isdp-network")
	viper.SetDefault("sandbox.repos_dir", "./repos")
	viper.SetDefault("agent.max_depth", 15)
	viper.SetDefault("agent.max_retries", 3)
	viper.SetDefault("agent.context_max_lines", 400)
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("mcp.base_url", "http://localhost:8080/api/v1/mcp")
	viper.SetDefault("mcp.token_ttl", "30m")
}