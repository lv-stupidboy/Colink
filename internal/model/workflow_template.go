package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TransitionType 转换类型
type TransitionType string

const (
	TransitionTypeSequence TransitionType = "sequence" // 顺序执行
	TransitionTypeParallel TransitionType = "parallel" // 并行执行（分支）
	TransitionTypeMerge    TransitionType = "merge"    // 汇聚执行（等待上游）
)

// Transition Agent间协作转换规则
type Transition struct {
	FromAgentID string         `json:"fromAgentId"` // 源 Agent ID
	ToAgentID   string         `json:"toAgentId"`   // 目标 Agent ID
	Type        TransitionType `json:"type"`        // 转换类型: sequence/parallel/merge

	// 触发提示（用户可编辑，注入到源 Agent 的 system prompt）
	TriggerHint string `json:"triggerHint"` // "@前端开发工程师 当需要前端实现时"

	// 汇聚配置
	WaitFor []string `json:"waitFor"` // 等待的 Agent ID 列表 (用于汇聚)
}

// WorkflowTemplate 工作流模板
type WorkflowTemplate struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	AgentIDs      json.RawMessage `json:"agentIds"`      // Agent实例ID列表 (JSON数组)
	Transitions   json.RawMessage `json:"transitions"`    // Agent间转换规则 (JSON数组)
	Checkpoints   json.RawMessage `json:"checkpoints"`    // 人工检查点列表 (JSON数组)
	EstimatedTime string          `json:"estimatedTime"` // 预计耗时
	IsSystem      bool            `json:"isSystem"`      // 是否系统预设
	IsDefault     bool            `json:"isDefault"`     // 新增：是否为默认工作流
	RoutableTeams json.RawMessage `json:"routableTeams"` // A2A Enhancement: 可路由到的目标 Team ID 列表 (JSON数组)
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

func (w *WorkflowTemplate) TableName() string {
	return "workflow_templates"
}

// CreateWorkflowTemplateRequest 创建工作流模板请求
type CreateWorkflowTemplateRequest struct {
	Name          string       `json:"name" binding:"required"`
	Description   string       `json:"description"`
	AgentIDs      []string     `json:"agentIds"`
	Transitions   []Transition `json:"transitions"`
	Checkpoints   []string     `json:"checkpoints"`
	EstimatedTime string       `json:"estimatedTime"`
	BasedOn       string       `json:"basedOn,omitempty"` // 基于的模板ID
}

// UpdateWorkflowTemplateRequest 更新工作流模板请求
type UpdateWorkflowTemplateRequest struct {
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	AgentIDs      []string     `json:"agentIds"`
	Transitions   []Transition `json:"transitions"`
	Checkpoints   []string     `json:"checkpoints"`
	EstimatedTime string       `json:"estimatedTime"`
	RoutableTeams []string     `json:"routableTeams"` // A2A Enhancement: 可路由到的目标 Team ID 列表
}