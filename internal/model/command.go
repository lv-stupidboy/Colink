package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Command Models ==========

// Command 命令模型
type Command struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	Content         string    `json:"content,omitempty"` // 文件内容（上传或查看时返回）
	SupportedAgents []string  `json:"supportedAgents,omitempty"` // 支持的Agent类型
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (c *Command) TableName() string {
	return "commands"
}

// AgentCommandBinding Agent角色与命令绑定
type AgentCommandBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agentRoleId"`
	CommandID   uuid.UUID `json:"commandId"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (a *AgentCommandBinding) TableName() string {
	return "agent_command_bindings"
}

// CommandSkillBinding 命令与技能绑定
type CommandSkillBinding struct {
	ID        uuid.UUID `json:"id"`
	CommandID uuid.UUID `json:"commandId"`
	SkillID   uuid.UUID `json:"skillId"`
	CreatedAt time.Time `json:"createdAt"`
}

func (c *CommandSkillBinding) TableName() string {
	return "command_skill_bindings"
}

// CreateCommandRequest 创建Command请求
type CreateCommandRequest struct {
	Name            string   `json:"name" binding:"required"`
	Description     string   `json:"description"`
	Content         string   `json:"content"` // 命令内容（可选，传入则保存文件）
	SupportedAgents []string `json:"supportedAgents"` // 支持的Agent类型（可选）
}

// UpdateCommandRequest 更新Command请求
type UpdateCommandRequest struct {
	Description     string   `json:"description"`
	Content         string   `json:"content"` // 命令内容（可选，传入则保存文件）
	SupportedAgents []string `json:"supportedAgents"` // 支持的Agent类型（可选）
}

// CommandListQuery Command列表查询参数
type CommandListQuery struct {
	Search    string `form:"search"`
	AgentType string `form:"agent_type"` // 按Agent类型过滤
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
}

// BindCommandRequest 绑定Command请求
type BindCommandRequest struct {
	CommandIDs []uuid.UUID `json:"commandIds" binding:"required"`
}

// BindSkillsToCommandRequest 绑定技能到Command请求
type BindSkillsToCommandRequest struct {
	SkillIDs []uuid.UUID `json:"skillIds" binding:"required"`
}