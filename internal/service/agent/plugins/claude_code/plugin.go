// internal/service/agent/plugins/claude_code/plugin.go
package claude_code

import (
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
)

func init() {
	agent.RegisterPlugin(agent.PluginMeta{
		Type:                Type,
		Name:                "ClaudeCode",
		Description:         "Anthropic Claude CLI - 支持 ACP 协议和原生 CLI 模式",
		Factory:             NewClaudeAdapterFactory, // 使用新的 Factory 函数
		ConfigDir:           ".claude",
		DefaultPath:         "claude",
		ConfigGeneratorFactory: NewClaudeConfigGenerator,
	})
}

// NewClaudeAdapterFactory 根据配置返回不同的 adapter
// 如果 claude_code.use_acp = true，返回 ACP adapter（使用 claude-agent-acp）
// 否则返回原生 CLI adapter（使用 claude）
func NewClaudeAdapterFactory(baseAgent *model.BaseAgent) agent.AgentAdapter {
	if agent.GetClaudeCodeUseACP() {
		// ACP 协议模式 - 使用 claude-agent-acp
		return NewClaudeACPAdapter(baseAgent)
	}
	// 原生 CLI 模式 - 使用 claude
	return NewClaudeCLIAdapter(baseAgent)
}

// NewClaudeAdapter 是 NewClaudeCLIAdapter 的别名，保持向后兼容
func NewClaudeAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
	return NewClaudeCLIAdapter(baseAgent)
}