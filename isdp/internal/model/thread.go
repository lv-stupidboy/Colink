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

// ThreadType 会话类型
type ThreadType string

const (
	ThreadTypeWorkflow       ThreadType = "workflow"        // 工作流模式（默认）
	ThreadTypeFreeDiscussion ThreadType = "free_discussion" // 自由协作模式
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
	WorkflowTemplateID *uuid.UUID   `json:"workflowTemplateId,omitempty"` // 使用的工作流模板ID
	Type               ThreadType   `json:"type"`                         // 会话类型：workflow/free_discussion
	AvailableAgents    []string     `json:"availableAgents,omitempty"`    // 可用 Agent 范围（自由模式）
	CreatedAt          time.Time    `json:"createdAt"`
	UpdatedAt          time.Time    `json:"updatedAt"`
}

func (t *Thread) TableName() string {
	return "threads"
}