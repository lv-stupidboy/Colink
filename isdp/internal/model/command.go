package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Command Models ==========

// Command 命令模型
type Command struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (c *Command) TableName() string {
	return "commands"
}

// AgentCommandBinding Agent角色与命令绑定
type AgentCommandBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agent_role_id"`
	CommandID   uuid.UUID `json:"command_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *AgentCommandBinding) TableName() string {
	return "agent_command_bindings"
}

// CommandSkillBinding 命令与技能绑定
type CommandSkillBinding struct {
	ID        uuid.UUID `json:"id"`
	CommandID uuid.UUID `json:"command_id"`
	SkillID   uuid.UUID `json:"skill_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *CommandSkillBinding) TableName() string {
	return "command_skill_bindings"
}

// CreateCommandRequest 创建Command请求
type CreateCommandRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateCommandRequest 更新Command请求
type UpdateCommandRequest struct {
	Description string `json:"description"`
}

// CommandListQuery Command列表查询参数
type CommandListQuery struct {
	Search   string `form:"search"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// BindCommandRequest 绑定Command请求
type BindCommandRequest struct {
	CommandIDs []uuid.UUID `json:"command_ids" binding:"required"`
}

// BindSkillsToCommandRequest 绑定技能到Command请求
type BindSkillsToCommandRequest struct {
	SkillIDs []uuid.UUID `json:"skill_ids" binding:"required"`
}