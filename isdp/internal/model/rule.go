package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Rule Models ==========

// RuleScope 规约范围类型
type RuleScope string

const (
	RuleScopePublic   RuleScope = "public"   // 公共规约，自动绑定到所有Agent
	RuleScopeInstance RuleScope = "instance" // 实例规约，需手动绑定
)

// Rule 规约模型
type Rule struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Scope       RuleScope `json:"scope"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (r *Rule) TableName() string {
	return "rules"
}

// AgentRuleBinding Agent角色与规约绑定
type AgentRuleBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agent_role_id"`
	RuleID      uuid.UUID `json:"rule_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *AgentRuleBinding) TableName() string {
	return "agent_rule_bindings"
}

// CreateRuleRequest 创建Rule请求
type CreateRuleRequest struct {
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
	Scope       RuleScope `json:"scope"`
}

// UpdateRuleRequest 更新Rule请求
type UpdateRuleRequest struct {
	Description string    `json:"description"`
	Scope       RuleScope `json:"scope"`
}

// RuleListQuery Rule列表查询参数
type RuleListQuery struct {
	Search   string    `form:"search"`
	Scope    string    `form:"scope"`
	Page     int       `form:"page"`
	PageSize int       `form:"page_size"`
}

// BindRuleRequest 绑定Rule请求
type BindRuleRequest struct {
	RuleIDs []uuid.UUID `json:"rule_ids" binding:"required"`
}