package agent

import (
	"context"
	"os/exec"

	"github.com/anthropic/isdp/internal/model"
)

// AgentAdapter Agent适配器接口 - 统一的执行和会话管理接口
type AgentAdapter interface {
	// Execute 执行单次任务（无会话上下文）
	Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error)

	// ExecuteWithStream 流式执行，实时回调输出
	ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) (*ExecutionResult, error)

	// StartSession 启动交互式会话
	StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error

	// ResumeSession 恢复会话，发送新消息
	ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error

	// StopSession 停止会话
	StopSession(sessionID string) error

	// GetSessionStatus 获取会话状态
	GetSessionStatus(sessionID string) SessionStatus

	// CheckHealth 检查CLI健康状态
	CheckHealth(ctx context.Context) error

	// GetCurrentProcess 获取当前执行的进程（用于取消）
	// 返回 nil 表示当前没有正在执行的进程
	GetCurrentProcess() *exec.Cmd
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
		return NewClaudeAdapter(baseAgent)
	case model.BaseAgentTypeOpenCode:
		return NewOpenCodeAdapter(baseAgent)
	default:
		return nil
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// maskToken 对敏感token进行掩码处理，只显示前4位和后4位
// 例如: "sk-ant-api03-xxxxx" -> "sk-a****xxxx"
func maskToken(token string) string {
	if token == "" {
		return "<empty>"
	}
	if len(token) <= 8 {
		return "****"
	}
	// 显示前4位和后4位，中间用****替代
	return token[:4] + "****" + token[len(token)-4:]
}