package agent

import (
	"context"
	"time"

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

// SessionExecutor 会话执行器接口，扩展了AgentAdapter的会话管理能力
type SessionExecutor interface {
	AgentAdapter
	StartSession(ctx context.Context, sessionID string, config *model.AgentRoleConfig, baseAgent *model.BaseAgent, workDir string) error
	ResumeSession(ctx context.Context, sessionID string, input string) error
	StopSession(ctx context.Context, sessionID string) error
	GetSessionStatus(sessionID string) SessionStatus
}

// SessionStatus 会话状态
type SessionStatus string

const (
	SessionStatusIdle      SessionStatus = "idle"
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusPaused    SessionStatus = "paused"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusStopped   SessionStatus = "stopped"
)

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

func timePtr(t time.Time) *time.Time {
	return &t
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}