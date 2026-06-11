package agent

import (
	"context"
	"os/exec"
	"time"

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

// SessionResumeCapable ACP 原生 Session 恢复能力接口
// 用于支持 ACP 协议的 session/resume、session/list 等方法的 CLI
// OpenCode 1.17.0+ 支持 sessionCapabilities.resume
type SessionResumeCapable interface {
	// SessionList 获取历史会话列表
	SessionList(ctx context.Context, cwd string) ([]SessionInfo, error)

	// SessionResume 恢复已有会话（不回放历史）
	// 返回 ACP session ID 用于后续 prompt
	SessionResume(ctx context.Context, acpSessionID string, cwd string, mcpServers []interface{}) (string, error)

	// SessionLoad 加载已有会话（回放完整历史）
	// 注意：会通过 session/update 通知回放所有历史消息
	SessionLoad(ctx context.Context, acpSessionID string, cwd string, mcpServers []interface{}) (string, error)

	// SessionClose 关闭会话
	SessionClose(ctx context.Context, acpSessionID string) error

	// ExecuteWithResume 使用 session/resume 执行
	// 如果有历史 session ID，先 resume 再发送 prompt
	// 否则创建新 session
	ExecuteWithResume(ctx context.Context, req *ExecutionRequest, acpSessionID string, onChunk func(Chunk)) (result *ExecutionResult, newSessionID string, err error)
}

// SessionInfo 会话信息（用于 session/list 返回）
type SessionInfo struct {
	SessionID string    `json:"sessionId"` // ACP session ID
	CWD       string    `json:"cwd"`       // 工作目录
	Title     string    `json:"title"`     // 会话标题
	UpdatedAt time.Time `json:"updatedAt"` // 最后更新时间
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