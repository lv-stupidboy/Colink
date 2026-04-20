package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Human Task Models ==========

// HumanTaskStatus 人工任务状态
type HumanTaskStatus string

const (
	HumanTaskStatusPending   HumanTaskStatus = "pending"
	HumanTaskStatusCompleted HumanTaskStatus = "completed"
	HumanTaskStatusCancelled HumanTaskStatus = "cancelled"
)

// HumanTask 人工任务（简化版）
type HumanTask struct {
	ID            uuid.UUID       `json:"id"`
	ThreadID      uuid.UUID       `json:"threadId"`
	InvocationID  uuid.UUID       `json:"invocationId"`   // 关联 invocation
	AgentConfigID uuid.UUID       `json:"agentConfigId"`  // 等待用户的 Agent
	AgentName     string          `json:"agentName"`      // Agent 名称
	WaitReason    string          `json:"waitReason"`     // 等待原因（输出摘要）
	ProjectID     uuid.UUID       `json:"projectId"`      // 项目 ID
	ProjectName   string          `json:"projectName"`    // 项目名称
	ThreadName    string          `json:"threadName"`     // 任务名称（Thread 名称）
	Status        HumanTaskStatus `json:"status"`
	CreatedAt     time.Time       `json:"createdAt"`
	CompletedAt   *time.Time      `json:"completedAt"`
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
	Success   bool           `json:"success"`
	NextAgent *NextAgentInfo `json:"nextAgent,omitempty"`
	Triggered bool           `json:"triggered"`
}

// NextAgentInfo 下游 Agent 信息
type NextAgentInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}