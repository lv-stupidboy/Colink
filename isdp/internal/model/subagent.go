package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Subagent Models ==========

// Subagent 子代理模型
type Subagent struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Content     string    `json:"content"`            // Markdown内容
	SkillID     uuid.UUID `json:"skill_id,omitempty"` // 所属技能包ID（可选）
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *Subagent) TableName() string {
	return "subagents"
}

// AgentSubagentBinding Agent角色与子代理绑定
type AgentSubagentBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agent_role_id"`
	SubagentID  uuid.UUID `json:"subagent_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *AgentSubagentBinding) TableName() string {
	return "agent_subagent_bindings"
}

// CreateSubagentRequest 创建Subagent请求
type CreateSubagentRequest struct {
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
	Content     string    `json:"content" binding:"required"`
	SkillID     uuid.UUID `json:"skill_id"`
}

// UpdateSubagentRequest 更新Subagent请求
type UpdateSubagentRequest struct {
	Description string `json:"description"`
	Content     string `json:"content"`
}

// SubagentListQuery Subagent列表查询参数
type SubagentListQuery struct {
	Search   string `form:"search"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// BindSubagentRequest 绑定Subagent请求
type BindSubagentRequest struct {
	SubagentIDs []uuid.UUID `json:"subagent_ids" binding:"required"`
}

// SubagentSkillBinding 子代理与技能绑定
type SubagentSkillBinding struct {
	ID         uuid.UUID `json:"id"`
	SubagentID uuid.UUID `json:"subagent_id"`
	SkillID    uuid.UUID `json:"skill_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *SubagentSkillBinding) TableName() string {
	return "subagent_skill_bindings"
}

// BindSkillsToSubagentRequest 绑定技能到Subagent请求
type BindSkillsToSubagentRequest struct {
	SkillIDs []uuid.UUID `json:"skill_ids" binding:"required"`
}