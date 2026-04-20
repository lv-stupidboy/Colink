// Package config 提供配置管理功能
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Data        DataConfig        `mapstructure:"data"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Redis       RedisConfig       `mapstructure:"redis"`
	Claude      ClaudeConfig      `mapstructure:"claude"`
	Sandbox     SandboxConfig     `mapstructure:"sandbox"`
	Agent       AgentConfig       `mapstructure:"agent"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	MCP         MCPConfig         `mapstructure:"mcp"`
	Auth        AuthConfig        `mapstructure:"auth"`
	AgentAssets AgentAssetsConfig `mapstructure:"agent_assets"`
	Skill       SkillConfig       `mapstructure:"skill"`
	Subagent    SubagentConfig    `mapstructure:"subagent"`
	AgentConfig AgentConfigConfig `mapstructure:"agent_config"`
	Command     CommandConfig     `mapstructure:"command"`
	Rule        RuleConfig        `mapstructure:"rule"`
	Feishu      FeishuConfig      `mapstructure:"feishu"`
	IM          IMConfig          `mapstructure:"im"`
	Reporter    ReporterConfig    `mapstructure:"reporter"`
	HumanTask   HumanTaskConfig   `mapstructure:"human_task"`
	TeamPackageSync TeamPackageSyncConfig `mapstructure:"team_package_sync"`
}

// DataConfig 数据目录配置
type DataConfig struct {
	// BasePath 数据根目录（必须配置）
	BasePath string `mapstructure:"base_path"`
}

// GetDataPath 获取数据根目录，未配置则返回空字符串
func (c *DataConfig) GetDataPath() string {
	return c.BasePath
}

// GetLogsPath 获取日志目录
func (c *DataConfig) GetLogsPath() string {
	if c.BasePath == "" {
		return ""
	}
	return c.BasePath + "/logs"
}

// GetConfigsPath 获取配置文件目录
func (c *DataConfig) GetConfigsPath() string {
	if c.BasePath == "" {
		return ""
	}
	return c.BasePath + "/configs"
}

// GetAgentAssetsPath 获取Agent资产目录
func (c *DataConfig) GetAgentAssetsPath() string {
	if c.BasePath == "" {
		return ""
	}
	return c.BasePath + "/agent-assets"
}

// GetAgentConfigsPath 获取Agent配置池目录
func (c *DataConfig) GetAgentConfigsPath() string {
	if c.BasePath == "" {
		return ""
	}
	return c.BasePath + "/agent-configs"
}

// GetReposPath 获取代码仓库目录
func (c *DataConfig) GetReposPath() string {
	if c.BasePath == "" {
		return ""
	}
	return c.BasePath + "/repos"
}

// GetDBPath 获取SQLite数据库路径（已废弃，请使用 DatabaseConfig.Path）
func (c *DataConfig) GetDBPath() string {
	return "" // 返回空，强制使用 DatabaseConfig.Path
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
	// Path 不设默认值，必须在配置文件中明确指定
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

// AgentAssetsConfig Agent资产配置
type AgentAssetsConfig struct {
	// BasePath 资产存储基础路径（必须配置）
	BasePath string `mapstructure:"base_path"`
}

// GetBasePath 获取资产基础路径，未配置则返回空字符串
func (c *AgentAssetsConfig) GetBasePath() string {
	return c.BasePath
}

// SkillConfig 技能配置
type SkillConfig struct {
	// UseCountUpdateInterval 技能使用次数更新间隔，默认 1 小时
	// 格式示例: "1h", "30m", "2h30m"
	UseCountUpdateInterval string `mapstructure:"use_count_update_interval"`

	// UploadMaxSize 技能包上传最大大小，单位 MB，默认 5
	UploadMaxSize int `mapstructure:"upload_max_size"`
}

// SubagentConfig 子代理配置
type SubagentConfig struct {
	// UploadMaxSize 子代理文件上传最大大小，单位 MB，默认 2
	UploadMaxSize int `mapstructure:"upload_max_size"`
}

// AgentConfigConfig Agent配置相关配置
type AgentConfigConfig struct {
	// DataDir ISDP数据目录，用于存储Agent配置池
	DataDir string `mapstructure:"data_dir"`
}

// CommandConfig 命令配置
type CommandConfig struct {
	// UploadMaxSize 命令文件上传最大大小，单位 MB，默认 2
	UploadMaxSize int `mapstructure:"upload_max_size"`
}

// RuleConfig 规约配置
type RuleConfig struct {
	// UploadMaxSize 规约文件上传最大大小，单位 MB，默认 2
	UploadMaxSize int `mapstructure:"upload_max_size"`
}

// ReporterConfig 数据上报配置
type ReporterConfig struct {
	// Enabled 是否启用上报，默认 false
	Enabled bool `mapstructure:"enabled"`
	// Endpoint 上报服务地址
	Endpoint string `mapstructure:"endpoint"`
	// Interval 上报间隔，格式示例: "30m", "1h"
	Interval string `mapstructure:"interval"`
	// RetryTimes 失败重试次数，默认 3
	RetryTimes int `mapstructure:"retry_times"`
	// RetryInterval 重试间隔，格式示例: "1m", "30s"
	RetryInterval string `mapstructure:"retry_interval"`
}

// ApplyDefaults 设置 Reporter 配置默认值
func (c *ReporterConfig) ApplyDefaults() {
	if c.Interval == "" {
		c.Interval = "30m"
	}
	if c.RetryTimes == 0 {
		c.RetryTimes = 3
	}
	if c.RetryInterval == "" {
		c.RetryInterval = "1m"
	}
}

// IsRunnable 返回是否应该启动 Reporter
func (c *ReporterConfig) IsRunnable() bool {
	return c.Enabled && c.Endpoint != ""
}

// GetInterval 获取上报间隔（解析为 time.Duration）
func (c *ReporterConfig) GetInterval() time.Duration {
	d, err := time.ParseDuration(c.Interval)
	if err != nil {
		return 30 * time.Minute
	}
	return d
}

// GetRetryInterval 获取重试间隔（解析为 time.Duration）
func (c *ReporterConfig) GetRetryInterval() time.Duration {
	d, err := time.ParseDuration(c.RetryInterval)
	if err != nil {
		return 1 * time.Minute
	}
	return d
}

// HumanTaskConfig 待办任务配置
type HumanTaskConfig struct {
	// Enabled 是否启用待办任务自动创建，默认 false
	// 当 Agent 等待用户输入（AskUserQuestion）时自动创建待办任务
	// 用户回复后自动关闭对应的待办任务
	Enabled bool `mapstructure:"enabled"`
}

// TeamPackageSyncConfig 团队包同步配置
type TeamPackageSyncConfig struct {
	RemoteRepoURL     string `mapstructure:"remote_repo_url"`
	AutoUpdateEnabled bool   `mapstructure:"auto_update_enabled"`
	CheckInterval     string `mapstructure:"check_interval"`
	Branch            string `mapstructure:"branch"`
	TempDir           string `mapstructure:"temp_dir"` // 临时目录路径，相对于 base_path
}

// ApplyDefaults 设置团队包同步配置默认值
func (c *TeamPackageSyncConfig) ApplyDefaults() {
	if c.RemoteRepoURL == "" {
		c.RemoteRepoURL = "https://gitee.com/colink_1/isdp.git"
	}
	if c.CheckInterval == "" {
		c.CheckInterval = "24h"
	}
	if c.Branch == "" {
		c.Branch = "main"
	}
	if c.TempDir == "" {
		c.TempDir = "temp" // 默认在 base_path 下创建 temp 目录
	}
}

// GetCheckInterval 获取检查间隔（解析为 time.Duration）
func (c *TeamPackageSyncConfig) GetCheckInterval() time.Duration {
	d, err := time.ParseDuration(c.CheckInterval)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}

// IsEnabled 返回是否启用团队包同步
func (c *TeamPackageSyncConfig) IsEnabled() bool {
	return c.AutoUpdateEnabled && c.RemoteRepoURL != ""
}

const (
	EventModeWebhook  = "webhook"
	EventModeListener = "listener"
)

// FeishuConfig 飞书集成配置 (deprecated: use IM.Platforms instead)
type FeishuConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	AppID             string `mapstructure:"app_id"`
	AppSecret         string `mapstructure:"app_secret"`
	VerificationToken string `mapstructure:"verification_token"`
	EncryptKey        string `mapstructure:"encrypt_key"`
	LarkCLIPath       string `mapstructure:"lark_cli_path"`
	DefaultProjectID  string `mapstructure:"default_project_id"`
	EventMode         string `mapstructure:"event_mode"`
}

// ApplyDefaults 设置飞书配置默认值
func (c *FeishuConfig) ApplyDefaults() {
	if c.LarkCLIPath == "" {
		c.LarkCLIPath = "lark-cli"
	}
	if c.EventMode == "" {
		c.EventMode = EventModeListener
	}
}

// IMConfig 多平台IM集成配置
type IMConfig struct {
	Platforms []IMPlatformConfig `mapstructure:"platforms"`
}

// IMPlatformConfig IM平台配置
type IMPlatformConfig struct {
	// 平台类型: "feishu", "slack", "discord"
	Type string `mapstructure:"type"`
	// 是否启用
	Enabled bool `mapstructure:"enabled"`

	// 通用配置
	RateLimitMax    int           `mapstructure:"rate_limit_max"`    // 速率限制最大请求数，默认: 20
	RateLimitWindow time.Duration `mapstructure:"rate_limit_window"` // 速率限制时间窗口，默认: 60s
	MaxRetries      int           `mapstructure:"max_retries"`       // 最大重试次数，默认: 3

	// Feishu-specific 配置
	AppID             string `mapstructure:"app_id"`
	AppSecret         string `mapstructure:"app_secret"`
	VerificationToken string `mapstructure:"verification_token"`
	EncryptKey        string `mapstructure:"encrypt_key"`
	LarkCLIPath       string `mapstructure:"lark_cli_path"`
	DefaultProjectID  string `mapstructure:"default_project_id"`
	EventMode         string `mapstructure:"event_mode"`

	// TODO: Slack-specific 配置
	// BotToken string `mapstructure:"bot_token"`
	// SigningSecret string `mapstructure:"signing_secret"`

	// TODO: Discord-specific 配置
	// BotToken string `mapstructure:"bot_token"`
	// ApplicationID string `mapstructure:"application_id"`
}

// ApplyDefaults 设置IM平台配置默认值
func (c *IMPlatformConfig) ApplyDefaults() {
	// 通用默认值
	if c.RateLimitMax == 0 {
		c.RateLimitMax = 20
	}
	if c.RateLimitWindow == 0 {
		c.RateLimitWindow = 60 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}

	// Feishu 平台默认值
	if c.Type == "feishu" && c.LarkCLIPath == "" {
		c.LarkCLIPath = "lark-cli"
	}
	if c.Type == "feishu" && c.EventMode == "" {
		c.EventMode = EventModeListener
	}
}

// GetUseCountUpdateInterval 获取技能使用次数更新间隔
func (c *SkillConfig) GetUseCountUpdateInterval() time.Duration {
	if c.UseCountUpdateInterval == "" {
		return time.Hour // 默认 1 小时
	}
	d, err := time.ParseDuration(c.UseCountUpdateInterval)
	if err != nil {
		return time.Hour
	}
	return d
}

// GetUploadMaxSize 获取上传文件最大大小（字节）
func (c *SkillConfig) GetUploadMaxSize() int64 {
	if c.UploadMaxSize <= 0 {
		return 5 * 1024 * 1024 // 默认 5MB
	}
	return int64(c.UploadMaxSize) * 1024 * 1024
}

// GetUploadMaxSize 获取子代理上传文件最大大小（字节）
func (c *SubagentConfig) GetUploadMaxSize() int64 {
	if c.UploadMaxSize <= 0 {
		return 2 * 1024 * 1024 // 默认 2MB
	}
	return int64(c.UploadMaxSize) * 1024 * 1024
}

// GetUploadMaxSize 获取命令文件上传最大大小（字节）
func (c *CommandConfig) GetUploadMaxSize() int64 {
	if c.UploadMaxSize <= 0 {
		return 2 * 1024 * 1024 // 默认 2MB
	}
	return int64(c.UploadMaxSize) * 1024 * 1024
}

// GetUploadMaxSize 获取规约文件上传最大大小（字节）
func (c *RuleConfig) GetUploadMaxSize() int64 {
	if c.UploadMaxSize <= 0 {
		return 2 * 1024 * 1024 // 默认 2MB
	}
	return int64(c.UploadMaxSize) * 1024 * 1024
}

// GetSkillStoragePath 获取技能包存储路径
func (c *Config) GetSkillStoragePath() string {
	return c.AgentAssets.GetBasePath() + "/skills"
}

// GetSubagentStoragePath 获取子代理存储路径
func (c *Config) GetSubagentStoragePath() string {
	return c.AgentAssets.GetBasePath() + "/subagents"
}

// GetCommandStoragePath 获取命令存储路径
func (c *Config) GetCommandStoragePath() string {
	return c.AgentAssets.GetBasePath() + "/commands"
}

// GetRuleStoragePath 获取规约存储路径
func (c *Config) GetRuleStoragePath() string {
	return c.AgentAssets.GetBasePath() + "/rules"
}

// GetSettingsStoragePath 获取Settings存储路径
func (c *Config) GetSettingsStoragePath() string {
	return c.AgentAssets.GetBasePath() + "/settings"
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
	cfg.Feishu.ApplyDefaults()
	cfg.Reporter.ApplyDefaults()

	// 应用IM平台默认值
	for i := range cfg.IM.Platforms {
		cfg.IM.Platforms[i].ApplyDefaults()
	}

	// 应用团队包同步默认值
	cfg.TeamPackageSync.ApplyDefaults()

	// 验证必须的路径配置
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// validateConfig 验证必须的配置项
func validateConfig(cfg *Config) error {
	// 数据目录必须配置
	if cfg.Data.BasePath == "" {
		return fmt.Errorf("配置错误: data.base_path 未设置")
	}

	// SQLite 数据库路径必须配置
	if cfg.Database.Type == DBTypeSQLite && cfg.Database.Path == "" {
		return fmt.Errorf("配置错误: database.path 未设置（SQLite数据库路径）")
	}

	// MySQL 数据库配置必须完整
	if cfg.Database.Type == DBTypeMySQL {
		if cfg.Database.MySQL.Host == "" {
			return fmt.Errorf("配置错误: database.mysql.host 未设置")
		}
		if cfg.Database.MySQL.Database == "" {
			return fmt.Errorf("配置错误: database.mysql.database 未设置")
		}
		if cfg.Database.MySQL.Username == "" {
			return fmt.Errorf("配置错误: database.mysql.username 未设置")
		}
	}

	// Agent资产目录必须配置
	if cfg.AgentAssets.BasePath == "" {
		return fmt.Errorf("配置错误: agent_assets.base_path 未设置")
	}

	// Agent配置池目录必须配置
	if cfg.AgentConfig.DataDir == "" {
		return fmt.Errorf("配置错误: agent_config.data_dir 未设置")
	}

	// 验证启用的IM平台配置
	for i, platform := range cfg.IM.Platforms {
		if !platform.Enabled {
			continue
		}

		if platform.Type == "" {
			return fmt.Errorf("配置错误: im.platforms[%d].type 未设置", i)
		}

		// Feishu 平台必需配置验证
		if platform.Type == "feishu" {
			if platform.AppID == "" {
				return fmt.Errorf("配置错误: im.platforms[%d] (feishu) app_id 未设置", i)
			}
			if platform.AppSecret == "" {
				return fmt.Errorf("配置错误: im.platforms[%d] (feishu) app_secret 未设置", i)
			}
			// verification_token 仅 webhook 模式需要；
			// listener 模式通过 WebSocket 接收事件，无需 HTTP 回调验证
			if platform.EventMode == EventModeWebhook && platform.VerificationToken == "" {
				return fmt.Errorf("配置错误: im.platforms[%d] (feishu) verification_token 未设置（webhook 模式必需）", i)
			}
		}

		// TODO: 添加 Slack/Discord 验证
	}

	return nil
}

// setDefaults 设置默认值（仅设置非路径类配置的默认值）
func setDefaults() {
	viper.SetDefault("server.port", 26305)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("agent.max_depth", 15)
	viper.SetDefault("mcp.base_url", "http://localhost:26305/api/v1/mcp")
	viper.SetDefault("mcp.token_ttl", "30m")
	viper.SetDefault("skill.use_count_update_interval", "1h")
	viper.SetDefault("skill.upload_max_size", 5)
	viper.SetDefault("subagent.upload_max_size", 2)
	viper.SetDefault("command.upload_max_size", 2)
	viper.SetDefault("rule.upload_max_size", 2)
	viper.SetDefault("reporter.enabled", true)
	viper.SetDefault("reporter.interval", "30m")
	viper.SetDefault("reporter.retry_times", 3)
	viper.SetDefault("reporter.retry_interval", "1m")

	// IM 平台默认值（可选配置，默认为空数组）
	// 具体平台配置通过 ApplyDefaults() 动态设置
}
