package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WorkflowTemplate 工作流模板
type WorkflowTemplate struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	AgentIDs      json.RawMessage `json:"agent_ids"`      // Agent实例ID列表 (JSON数组)
	Checkpoints   json.RawMessage `json:"checkpoints"`    // 人工检查点列表 (JSON数组)
	EstimatedTime string          `json:"estimated_time"` // 预计耗时
	IsSystem      bool            `json:"is_system"`      // 是否系统预设
	IsDefault     bool            `json:"is_default"`     // 新增：是否为默认工作流
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

func (w *WorkflowTemplate) TableName() string {
	return "workflow_templates"
}

// CreateWorkflowTemplateRequest 创建工作流模板请求
type CreateWorkflowTemplateRequest struct {
	Name          string   `json:"name" binding:"required"`
	Description   string   `json:"description"`
	AgentIDs      []string `json:"agent_ids"`
	Checkpoints   []string `json:"checkpoints"`
	EstimatedTime string   `json:"estimated_time"`
	BasedOn       string   `json:"based_on,omitempty"` // 基于的模板ID
}

// UpdateWorkflowTemplateRequest 更新工作流模板请求
type UpdateWorkflowTemplateRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	AgentIDs      []string `json:"agent_ids"`
	Checkpoints   []string `json:"checkpoints"`
	EstimatedTime string   `json:"estimated_time"`
}