package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Agent Invocation Models ==========

type InvocationStatus string

const (
	InvocationStatusPending     InvocationStatus = "pending"
	InvocationStatusRunning     InvocationStatus = "running"
	InvocationStatusCompleted   InvocationStatus = "completed"
	InvocationStatusFailed      InvocationStatus = "failed"
	InvocationStatusCancelled   InvocationStatus = "cancelled"
	InvocationStatusInterrupted InvocationStatus = "interrupted" // 后台执行支持：服务重启时中断
)

// AgentInvocation Agent调用记录模型
type AgentInvocation struct {
	ID            uuid.UUID        `json:"id"`
	ThreadID      uuid.UUID        `json:"threadId"`
	AgentConfigID uuid.UUID        `json:"agentConfigId"`
	Role          AgentRole        `json:"role"`
	RequiresHuman bool             `json:"requiresHuman"` // 是否需要人工参与
	AgentName     string           `json:"agentName"` // Agent名称（从 agent_configs.name 复制，用于历史显示）
	Status        InvocationStatus `json:"status"`
	Input         string           `json:"input"`
	FullPrompt    string           `json:"fullPrompt,omitempty"` // 完整提示词（系统提示 + 历史 + 输入）
	Output        string           `json:"output,omitempty"`
	StartedAt     *time.Time       `json:"startedAt,omitempty"`
	CompletedAt   *time.Time       `json:"completedAt,omitempty"`
	CreatedAt     time.Time        `json:"createdAt"`

	// 后台执行支持：进程追踪
	ProcessID *string `json:"processId,omitempty"` // Agent 进程 ID（用于启动恢复）

	// CLI 会话追踪
	SessionID string `json:"sessionId,omitempty"` // CLI 会话 ID（用于问题定位和 resume）

	// Token 使用统计
	InputTokens         int64   `json:"inputTokens,omitempty"`
	OutputTokens        int64   `json:"outputTokens,omitempty"`
	CacheReadTokens     int64   `json:"cacheReadTokens,omitempty"`
	CacheCreationTokens int64   `json:"cacheCreationTokens,omitempty"`
	CostUsd             float64 `json:"costUsd,omitempty"`
	DurationMs          int64   `json:"durationMs,omitempty"`
	DurationApiMs       int64   `json:"durationApiMs,omitempty"`
}

func (a *AgentInvocation) TableName() string {
	return "agent_invocations"
}