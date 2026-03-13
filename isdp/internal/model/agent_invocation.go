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
	ThreadID      uuid.UUID        `json:"thread_id"`
	AgentConfigID uuid.UUID        `json:"agent_config_id"`
	Role          AgentRole        `json:"role"`
	Status        InvocationStatus `json:"status"`
	Input         string           `json:"input"`
	Output        string           `json:"output,omitempty"`
	StartedAt     *time.Time       `json:"started_at,omitempty"`
	CompletedAt   *time.Time       `json:"completed_at,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
}

func (a *AgentInvocation) TableName() string {
	return "agent_invocations"
}