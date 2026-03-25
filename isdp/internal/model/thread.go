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
	ID                 uuid.UUID    `json:"id"`
	ProjectID          uuid.UUID    `json:"projectId"`
	Name               string       `json:"name"` // 任务名称
	Status             ThreadStatus `json:"status"`
	CurrentPhase       Phase        `json:"currentPhase"`
	CurrentAgent       string       `json:"currentAgent"`
	Depth              int          `json:"depth"`
	AbortToken         *string      `json:"abortToken,omitempty"`
	WorkflowTemplateID *uuid.UUID   `json:"workflowTemplateId,omitempty"` // 新增：使用的工作流模板ID
	CreatedAt          time.Time    `json:"createdAt"`
	UpdatedAt          time.Time    `json:"updatedAt"`
}

func (t *Thread) TableName() string {
	return "threads"
}