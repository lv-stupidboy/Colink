package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Human Task Models ==========

// HumanTaskStatus 人工任务状态
type HumanTaskStatus string

const (
	HumanTaskStatusPending    HumanTaskStatus = "pending"
	HumanTaskStatusInProgress HumanTaskStatus = "in_progress"
	HumanTaskStatusCompleted  HumanTaskStatus = "completed"
	HumanTaskStatusRejected   HumanTaskStatus = "rejected"
	HumanTaskStatusFailed     HumanTaskStatus = "failed"
)

// HumanTaskType 任务类型
type HumanTaskType string

const (
	HumanTaskTypeDispatch HumanTaskType = "task_dispatch" // 任务分发
	HumanTaskTypeReview   HumanTaskType = "review"        // 审核决策
	HumanTaskTypeConfirm  HumanTaskType = "confirm"       // 人工确认
)

// HumanTask 人工任务
type HumanTask struct {
	ID               uuid.UUID       `json:"id"`
	ThreadID         uuid.UUID       `json:"threadId"`
	RoleConfigID     uuid.UUID       `json:"roleConfigId"`
	RoleName         string          `json:"roleName"`        // 角色名称
	TaskType         HumanTaskType   `json:"taskType"`        // 任务类型
	TaskContent      string          `json:"taskContent"`     // 任务描述
	ExpectedOutput   string          `json:"expectedOutput"`  // 期望交付物
	SourceAgentID    uuid.UUID       `json:"sourceAgentId"`   // 来源 Agent invocation ID
	SourceAgentName  string          `json:"sourceAgentName"` // 来源 Agent 名称
	Status           HumanTaskStatus `json:"status"`          // 任务状态
	SubmittedAt      *time.Time      `json:"submittedAt"`     // 提交时间
	SubmittedBy      string          `json:"submittedBy"`     // 提交人
	OutputContent    string          `json:"outputContent"`   // 交付物内容
	OutputFiles      []string        `json:"outputFiles"`     // 交付物文件路径
	TargetAgentID    uuid.UUID       `json:"targetAgentId"`   // 下游目标 Agent ID
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

func (t *HumanTask) TableName() string {
	return "human_tasks"
}

// SubmitHumanTaskRequest 提交交付物请求
type SubmitHumanTaskRequest struct {
	OutputContent string   `json:"outputContent"`
	OutputFiles   []string `json:"outputFiles"`
}

// SubmitHumanTaskResponse 提交响应
type SubmitHumanTaskResponse struct {
	Success    bool           `json:"success"`
	NextAgent  *NextAgentInfo `json:"nextAgent,omitempty"`
	Triggered  bool           `json:"triggered"`
}

// NextAgentInfo 下游 Agent 信息
type NextAgentInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}