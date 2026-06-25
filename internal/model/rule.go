package model

import (
	"time"

	"github.com/google/uuid"
)

// Rule 规约模型
type Rule struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	Content         string    `json:"content,omitempty"` // 文件内容（上传或查看时返回）
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (r *Rule) TableName() string {
	return "rules"
}

// AgentRuleBinding Agent角色与规约绑定
type AgentRuleBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agentRoleId"`
	RuleID      uuid.UUID `json:"ruleId"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (a *AgentRuleBinding) TableName() string {
	return "agent_rule_bindings"
}

// CreateRuleRequest 创建Rule请求
type CreateRuleRequest struct {
	Name            string   `json:"name" binding:"required"`
	Description     string   `json:"description"`
	Content         string   `json:"content"` // 规约内容（可选，传入则保存文件）
}

// UpdateRuleRequest 更新Rule请求
type UpdateRuleRequest struct {
	Description     string   `json:"description"`
	Content         string   `json:"content"` // 规约内容（可选，传入则保存文件）
}

// RuleListQuery Rule列表查询参数
type RuleListQuery struct {
	Search    string `form:"search"`
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
}

// BindRuleRequest 绑定Rule请求
type BindRuleRequest struct {
	RuleIDs []uuid.UUID `json:"ruleIds" binding:"required"`
}