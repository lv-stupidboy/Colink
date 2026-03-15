package agent

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
)

// AgentAdapter Agent适配器接口
type AgentAdapter interface {
	// Execute 执行Agent
	Execute(ctx context.Context, config *model.AgentRoleConfig, layers *ContextLayers, input string, workDir string) (string, error)
	// ExecuteWithStream 流式执行Agent
	ExecuteWithStream(ctx context.Context, config *model.AgentRoleConfig, layers *ContextLayers, input string, workDir string, onChunk func(string)) error
	// CheckHealth 检查健康状态
	CheckHealth(ctx context.Context) error
}

// NewAdapter 根据基础Agent类型创建适配器
func NewAdapter(baseAgent *model.BaseAgent) AgentAdapter {
	if baseAgent == nil {
		return nil
	}

	switch baseAgent.Type {
	case model.BaseAgentTypeClaudeCode:
		return NewClaudeAdapterFromBaseAgent(baseAgent)
	case model.BaseAgentTypeOpenCode:
		return NewOpenCodeAdapter(baseAgent)
	default:
		return nil
	}
}