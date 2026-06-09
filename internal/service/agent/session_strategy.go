// Session 策略配置和类型识别
package agent

import (
	"time"

	"github.com/anthropic/isdp/internal/model"
)

// SessionStrategyConfig Session 策略配置
// 不同 CLI 工具的 session 能力不同，需要差异化处理
type SessionStrategyConfig struct {
	// UseLongRunning 是否使用长连接模式
	// true: 保持进程存活，避免上下文丢失（OpenCode/CodeAgent）
	// false: 每次执行启动新进程（Claude CLI）
	UseLongRunning bool `json:"useLongRunning"`

	// UseNativeResume 是否使用 CLI 原生 resume
	// true: 使用 CLI 内置的 session 恢复机制（Claude CLI --resume）
	// false: 不支持原生 resume，需要通过 prompt 注入恢复（OpenCode/CodeAgent）
	UseNativeResume bool `json:"useNativeResume"`

	// IdleTimeout 空闲超时（秒），长连接模式下使用
	// 进程空闲超过此时间后，进入 Sealing 状态，释放资源
	IdleTimeout int `json:"idleTimeout"`

	// ResumeExpiry Resume 有效期（小时），原生 resume 模式下使用
	// CLI 的 session ID 有效期，超过此时间后需要创建新 session
	ResumeExpiry int `json:"resumeExpiry"`

	// PersistInterval 每几轮对话后持久化（长连接模式）
	// 定期保存对话内容到数据库，防止意外断连丢失
	PersistInterval int `json:"persistInterval"`

	// MaxHistoryTokens 恢复历史最大 Token 数
	// 通过 prompt 注入恢复时，控制历史长度，防止上下文溢出
	MaxHistoryTokens int `json:"maxHistoryTokens"`

	// ParentType 父类型（继承策略）
	// 如 CodeAgent 继承 OpenCode 的处理逻辑
	ParentType string `json:"parentType"`
}

// SealReason 封存原因
type SealReason string

const (
	SealReasonTimeout           SealReason = "timeout"           // 空闲超时
	SealReasonManual            SealReason = "manual"            // 用户主动取消
	SealReasonProcessCrash      SealReason = "process_crash"     // 进程崩溃
	SealReasonServerError       SealReason = "server_error"      // 服务错误
	SealReasonAskUserTimeout    SealReason = "ask_user_timeout"  // AskUserQuestion 超时
	SealReasonGracefulShutdown  SealReason = "graceful_shutdown" // 优雅关闭
	SealReasonContextOverflow   SealReason = "context_overflow"  // 上下文过长
)

// 预定义的 OpenCode 兼容类型列表
// 这些类型使用和 OpenCode 相同的 session 策略（长连接 + Prompt 注入恢复）
// CodeAgent 等基于 OpenCode 的衍生类型加入此列表即可自动继承策略
var openCodeCompatibleTypes = []model.BaseAgentType{
	model.BaseAgentType("open_code"),
	model.BaseAgentType("code_agent"),
	// 未来可继续添加基于 OpenCode 的衍生类型
}

// defaultStrategies 默认策略配置
// 根据 CLI 类型定义不同的 session 处理方式
var defaultStrategies = map[model.BaseAgentType]SessionStrategyConfig{
	// Claude CLI: 使用原生 resume（不使用长连接）
	model.BaseAgentType("claude_code"): {
		UseLongRunning:   false,
		UseNativeResume:  true,
		ResumeExpiry:     168, // 7 天 = 168 小时
		PersistInterval:  0,   // 不需要持久化对话内容（CLI 内部管理）
		MaxHistoryTokens: 0,   // 不需要 prompt 注入恢复
	},

	// OpenCode ACP: 使用长连接（不支持原生 resume）
	model.BaseAgentType("open_code"): {
		UseLongRunning:   true,
		UseNativeResume:  false,
		IdleTimeout:      600, // 10 分钟
		PersistInterval:  3,   // 每 3 轮对话持久化
		MaxHistoryTokens: 4000, // 恢复历史最大 4000 tokens
	},

	// CodeAgent: 继承 OpenCode 策略（基于 OpenCode 定制开发）
	model.BaseAgentType("code_agent"): {
		UseLongRunning:   true,
		UseNativeResume:  false,
		IdleTimeout:      600,
		PersistInterval:  3,
		MaxHistoryTokens: 4000,
		ParentType:       "open_code", // 标记继承 OpenCode
	},
}

// IsOpenCodeCompatible 判断类型是否使用 OpenCode 兼容策略
// 返回 true 表示该类型使用长连接 + Prompt 注入恢复
func IsOpenCodeCompatible(agentType model.BaseAgentType) bool {
	for _, t := range openCodeCompatibleTypes {
		if t == agentType {
			return true
		}
	}
	return false
}

// GetSessionStrategy 获取指定类型的 session 策略
// 支持动态扩展：
// 1. 首先查找预定义策略
// 2. 如果未定义，检查是否为 OpenCode 兼容类型
// 3. 最后返回默认策略（不使用长连接，不使用 resume）
func GetSessionStrategy(agentType model.BaseAgentType) SessionStrategyConfig {
	// 1. 查找预定义策略
	if config, exists := defaultStrategies[agentType]; exists {
		return config
	}

	// 2. 检查是否为 OpenCode 兼容类型
	if IsOpenCodeCompatible(agentType) {
		return defaultStrategies[model.BaseAgentType("open_code")]
	}

	// 3. 默认策略：不使用长连接，不使用 resume（每次新进程）
	return SessionStrategyConfig{
		UseLongRunning:   false,
		UseNativeResume:  false,
		IdleTimeout:      300, // 5 分钟
		PersistInterval:  0,
		MaxHistoryTokens: 0,
	}
}

// IsLongRunningMode 判断是否使用长连接模式
func (c SessionStrategyConfig) IsLongRunningMode() bool {
	return c.UseLongRunning && !c.UseNativeResume
}

// GetIdleTimeout 获取空闲超时时间
func (c SessionStrategyConfig) GetIdleTimeout() time.Duration {
	if c.IdleTimeout <= 0 {
		return 10 * time.Minute // 默认 10 分钟
	}
	return time.Duration(c.IdleTimeout) * time.Second
}

// GetResumeExpiry 获取 resume 有效期
func (c SessionStrategyConfig) GetResumeExpiry() time.Duration {
	if c.ResumeExpiry <= 0 {
		return 7 * 24 * time.Hour // 默认 7 天
	}
	return time.Duration(c.ResumeExpiry) * time.Hour
}

// RegisterOpenCodeCompatibleType 注册新的 OpenCode 兼容类型
// 用于扩展：当添加新的基于 OpenCode 的衍生类型时，调用此函数
func RegisterOpenCodeCompatibleType(agentType model.BaseAgentType, config SessionStrategyConfig) {
	// 添加到兼容列表
	openCodeCompatibleTypes = append(openCodeCompatibleTypes, agentType)

	// 添加到策略配置
	if config.ParentType == "" {
		config.ParentType = "open_code"
	}
	defaultStrategies[agentType] = config
}

// GetOpenCodeCompatibleTypes 获取所有 OpenCode 兼容类型（用于调试/监控）
func GetOpenCodeCompatibleTypes() []model.BaseAgentType {
	return openCodeCompatibleTypes
}