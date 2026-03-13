package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Thread Models ==========

type ThreadStatus string

const (
	ThreadStatusIdle     ThreadStatus = "idle"
	ThreadStatusRunning  ThreadStatus = "running"
	ThreadStatusPaused   ThreadStatus = "paused"
	ThreadStatusComplete ThreadStatus = "complete"
	ThreadStatusFailed   ThreadStatus = "failed"
)

type Phase string

const (
	PhaseRequirement Phase = "requirement"
	PhaseDesign      Phase = "design"
	PhaseDevelopment Phase = "development"
	PhaseReview      Phase = "review"
	PhaseTest        Phase = "test"
	PhaseMerge       Phase = "merge"
	PhaseComplete    Phase = "complete"
)

// Thread 开发会话模型
type Thread struct {
	ID           uuid.UUID    `json:"id"`
	ProjectID    uuid.UUID    `json:"project_id"`
	Status       ThreadStatus `json:"status"`
	CurrentPhase Phase        `json:"current_phase"`
	CurrentAgent string       `json:"current_agent"`
	Depth        int          `json:"depth"`
	AbortToken   *string      `json:"abort_token,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

func (t *Thread) TableName() string {
	return "threads"
}