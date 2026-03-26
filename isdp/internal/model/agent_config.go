package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Agent Config Models ==========

// AgentRole Agent角色类型
type AgentRole string

const (
	AgentRoleRequirement       AgentRole = "requirement"
	AgentRoleArchitect         AgentRole = "architect"
	AgentRoleDeveloper         AgentRole = "developer"
	AgentRoleReviewer          AgentRole = "reviewer"
	AgentRoleTestEngineer      AgentRole = "testengineer"
	AgentRoleDevOps            AgentRole = "devops"
	AgentRoleFullstackEngineer AgentRole = "fullstack_engineer" // 全栈工程师
	AgentRoleCustom            AgentRole = "custom"             // 自定义角色
)

// RoutingConfig 路由配置（已废弃，路由逻辑已迁移到 WorkflowTemplate.Transitions）
type RoutingConfig struct {
	CanRouteTo    []AgentRole `json:"canRouteTo"`
	RouteOnSignal []string    `json:"routeOnSignal"`
}

// AgentConfig Agent配置模型（保留兼容性，现已改名为AgentRole）
// 建议使用AgentRole别名
type AgentConfig = AgentRoleConfig

// AgentRoleConfig Agent角色配置模型
type AgentRoleConfig struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Role         AgentRole `json:"role"`
	BaseAgentID  uuid.UUID `json:"baseAgentId,omitempty"`
	Description  string    `json:"description"`
	SystemPrompt string    `json:"systemPrompt"`
	MaxTokens    int       `json:"maxTokens"`
	Temperature  float64   `json:"temperature"`
	IsDefault    bool      `json:"isDefault"`
	IsSystem     bool      `json:"isSystem"` // 是否为系统预置角色

	// Mention 触发模式（支持 A2A 动态路由）
	MentionPatterns []string `json:"mentionPatterns"` // @mention 触发模式列表，如 ["@architect", "@架构师"]

	// 路由配置（已废弃，路由逻辑已迁移到 WorkflowTemplate.Transitions）
	RoutingConfig RoutingConfig `json:"routingConfig"`

	// 能力声明（已废弃，计划由 system_prompt 描述）
	Capabilities []string `json:"capabilities"` // 能力列表
	Dependencies []string `json:"dependencies"` // 依赖列表
	Outputs      []string `json:"outputs"`      // 产出列表

	// 配置生成相关字段
	ConfigGeneratedAt *time.Time `json:"configGeneratedAt,omitempty"` // 配置最后生成时间
	ConfigPath        string     `json:"configPath,omitempty"`        // 配置目录路径

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (a *AgentRoleConfig) TableName() string {
	return "agent_configs"
}

// CreateAgentRequest 创建Agent请求
type CreateAgentRequest struct {
	Name           string    `json:"name" binding:"required"`
	Role           AgentRole `json:"role"` // 可选，默认为 custom
	BaseAgentID    uuid.UUID `json:"baseAgentId"`
	Description    string    `json:"description"`
	SystemPrompt   string    `json:"systemPrompt" binding:"required"`
	MaxTokens      int       `json:"maxTokens"`
	Temperature    float64   `json:"temperature"`
	IsDefault      bool      `json:"isDefault"`
	MentionPatterns []string `json:"mentionPatterns"` // @mention 触发模式列表

	// 能力声明
	Capabilities []string `json:"capabilities"`
	Dependencies []string `json:"dependencies"`
	Outputs      []string `json:"outputs"`
}

// UpdateAgentRequest 更新Agent请求
type UpdateAgentRequest struct {
	Name           *string    `json:"name"`
	Role           *AgentRole `json:"role"`
	BaseAgentID    *uuid.UUID `json:"baseAgentId"`
	Description    *string    `json:"description"`
	SystemPrompt   *string    `json:"systemPrompt"`
	MaxTokens      *int       `json:"maxTokens"`
	Temperature    *float64   `json:"temperature"`
	IsDefault      *bool      `json:"isDefault"`
	MentionPatterns *[]string `json:"mentionPatterns"`
}

// AgentWithConfig Agent及其配置
type AgentWithConfig struct {
	AgentRoleConfig
	ConfigPath string `json:"configPath,omitempty"`
}