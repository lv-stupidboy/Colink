package agent

import (
	"context"
	"os/exec"

	"github.com/google/uuid"
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

// LongRunningSessionCapable 长连接 Session 能力接口
// 用于 OpenCode/CodeAgent 等不支持原生 resume 的 CLI
// 使用接口断言方式，避免对 AgentAdapter 接口进行侵入式修改
type LongRunningSessionCapable interface {
	// StartLongRunningSession 启动长连接 session（进程保持存活）
	// 返回 ACP session ID 用于后续 SendPromptToSession
	StartLongRunningSession(ctx context.Context, req *ExecutionRequest) (string, error)

	// SendPromptToSession 向已有 session 发送新 prompt（复用进程）
	SendPromptToSession(ctx context.Context, sessionID string, prompt string, onChunk func(Chunk)) error

	// StopLongRunningSession 停止长连接 session
	StopLongRunningSession(sessionID string) error

	// IsSessionAlive 检查 session 进程是否存活
	IsSessionAlive(sessionID string) bool
}

// ToolResultSender 发送工具结果的接口（用于 AskUserQuestion 等需要用户输入的工具）
// ACP adapter 等需要支持此接口
type ToolResultSender interface {
	SendToolResult(invocationID uuid.UUID, toolCallID string, result string) error
}

// SessionStatus 会话状态
type SessionStatus string

const (
	// 基础状态（原有）
	SessionStatusIdle      SessionStatus = "idle"
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusPaused    SessionStatus = "paused"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusStopped   SessionStatus = "stopped"

	// 长连接 Session 状态（新增）
	SessionStatusActive     SessionStatus = "active"     // 正在执行
	SessionStatusSealing    SessionStatus = "sealing"    // 正在封存
	SessionStatusSealed     SessionStatus = "sealed"     // 已封存（可恢复）
	SessionStatusRecovering SessionStatus = "recovering" // 正在恢复
	SessionStatusError      SessionStatus = "error"      // 异常状态
)

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