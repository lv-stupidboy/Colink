package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Agent Invocation Models ==========

type InvocationStatus string

const (
	InvocationStatusPending   InvocationStatus = "pending"
	InvocationStatusRunning   InvocationStatus = "running"
	InvocationStatusCompleted InvocationStatus = "completed"
	InvocationStatusFailed    InvocationStatus = "failed"
	InvocationStatusCancelled InvocationStatus = "cancelled"
)

// AgentInvocation Agent调用记录模型
type AgentInvocation struct {
	ID            uuid.UUID        `json:"id"`
	ThreadID      uuid.UUID        `json:"threadId"`
	AgentConfigID uuid.UUID        `json:"agentConfigId"`
	Role          AgentRole        `json:"role"`
	Status        InvocationStatus `json:"status"`
	Input         string           `json:"input"`
	Output        string           `json:"output,omitempty"`
	StartedAt     *time.Time       `json:"startedAt,omitempty"`
	CompletedAt   *time.Time       `json:"completedAt,omitempty"`
	CreatedAt     time.Time        `json:"createdAt"`

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