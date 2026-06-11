// Session 策略配置和类型识别
// 使用 ACP 原生 session/resume 能力
package agent

import (
	"time"

	"github.com/anthropic/isdp/internal/model"
)

// SessionStrategyConfig Session 策略配置
// 所有 CLI 类型统一使用 ACP 原生 resume
type SessionStrategyConfig struct {
	// UseNativeResume 是否使用 CLI 原生 resume
	// true: 使用 ACP session/resume 或 Claude CLI --resume
	// false: 每次执行启动新进程（无历史上下文）
	UseNativeResume bool `json:"useNativeResume"`

	// ResumeExpiry Resume 有效期（小时）
	// CLI 的 session ID 有效期，超过此时间后需要创建新 session
	ResumeExpiry int `json:"resumeExpiry"`

	// ParentType 父类型（继承策略）
	// 如 CodeAgent 继承 OpenCode 的处理逻辑
	ParentType string `json:"parentType"`
}

// SealReason 封存原因（保留用于日志和兼容）
type SealReason string

const (
	SealReasonManual            SealReason = "manual"            // 用户主动取消
	SealReasonServerError       SealReason = "server_error"      // 服务错误
	SealReasonGracefulShutdown  SealReason = "graceful_shutdown" // 优雅关闭
)

// defaultStrategies 默认策略配置
// 所有支持 ACP 协议的 CLI 使用原生 resume
var defaultStrategies = map[model.BaseAgentType]SessionStrategyConfig{
	// Claude CLI: 使用原生 resume
	model.BaseAgentType("claude_code"): {
		UseNativeResume: true,
		ResumeExpiry:    168, // 7 天 = 168 小时
	},

	// OpenCode ACP: 使用 ACP 原生 session/resume
	model.BaseAgentType("open_code"): {
		UseNativeResume: true,
		ResumeExpiry:    168, // 7 天
	},

	// CodeAgent: 继承 OpenCode 策略（基于 OpenCode 定制开发）
	model.BaseAgentType("code_agent"): {
		UseNativeResume: true,
		ResumeExpiry:    168,
		ParentType:      "open_code",
	},
}

// GetSessionStrategy 获取指定类型的 session 策略
func GetSessionStrategy(agentType model.BaseAgentType) SessionStrategyConfig {
	// 1. 查找预定义策略
	if config, exists := defaultStrategies[agentType]; exists {
		return config
	}

	// 2. 默认策略：不使用 resume（每次新进程）
	return SessionStrategyConfig{
		UseNativeResume: false,
		ResumeExpiry:    168,
	}
}

// GetResumeExpiry 获取 resume 有效期
func (c SessionStrategyConfig) GetResumeExpiry() time.Duration {
	if c.ResumeExpiry <= 0 {
		return 7 * 24 * time.Hour // 默认 7 天
	}
	return time.Duration(c.ResumeExpiry) * time.Hour
}

// RegisterAgentType 注册新的 Agent 类型策略
// 用于扩展：当添加新的 Agent 类型时，调用此函数
func RegisterAgentType(agentType model.BaseAgentType, config SessionStrategyConfig) {
	defaultStrategies[agentType] = config
}

// GetRegisteredTypes 获取所有注册的类型（用于调试/监控）
func GetRegisteredTypes() []model.BaseAgentType {
	types := make([]model.BaseAgentType, 0, len(defaultStrategies))
	for t := range defaultStrategies {
		types = append(types, t)
	}
	return types
}