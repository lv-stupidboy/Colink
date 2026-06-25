package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Subagent Models ==========

// Subagent 子代理模型
type Subagent struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	Content         string    `json:"content"` // Markdown内容（从文件读取，不存数据库）
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (s *Subagent) TableName() string {
	return "subagents"
}

// AgentSubagentBinding Agent角色与子代理绑定
type AgentSubagentBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agentRoleId"`
	SubagentID  uuid.UUID `json:"subagentId"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (a *AgentSubagentBinding) TableName() string {
	return "agent_subagent_bindings"
}

// CreateSubagentRequest 创建Subagent请求
type CreateSubagentRequest struct {
	Name            string   `json:"name" binding:"required"`
	Description     string   `json:"description"`
	Content         string   `json:"content" binding:"required"`
}

// UpdateSubagentRequest 更新Subagent请求
type UpdateSubagentRequest struct {
	Description     string   `json:"description"`
	Content         string   `json:"content"`
}

// SubagentListQuery Subagent列表查询参数
type SubagentListQuery struct {
	Search    string `form:"search"`
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
}

// BindSubagentRequest 绑定Subagent请求
type BindSubagentRequest struct {
	SubagentIDs []uuid.UUID `json:"subagentIds" binding:"required"`
}

// SubagentSkillBinding 子代理与技能绑定
type SubagentSkillBinding struct {
	ID         uuid.UUID `json:"id"`
	SubagentID uuid.UUID `json:"subagentId"`
	SkillID    uuid.UUID `json:"skillId"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (s *SubagentSkillBinding) TableName() string {
	return "subagent_skill_bindings"
}

// BindSkillsToSubagentRequest 绑定技能到Subagent请求
type BindSkillsToSubagentRequest struct {
	SkillIDs []uuid.UUID `json:"skillIds" binding:"required"`
}