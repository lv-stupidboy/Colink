package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Agent Config Models ==========

// AgentRole Agent角色类型
type AgentRole string

const (
	AgentRoleRequirement      AgentRole = "requirement"
	AgentRoleArchitect        AgentRole = "architect"
	AgentRoleDeveloper        AgentRole = "developer"
	AgentRoleReviewer         AgentRole = "reviewer"
	AgentRoleTestEngineer     AgentRole = "testengineer"
	AgentRoleDevOps           AgentRole = "devops"
	AgentRoleFullstackEngineer AgentRole = "fullstack_engineer" // 全栈工程师
	AgentRoleCustom           AgentRole = "custom"              // 自定义角色
)

// RoutingConfig 路由配置
type RoutingConfig struct {
	CanRouteTo    []AgentRole `json:"can_route_to"`
	RouteOnSignal []string    `json:"route_on_signal"`
}

// AgentConfig Agent配置模型（保留兼容性，现已改名为AgentRole）
// 建议使用AgentRole别名
type AgentConfig = AgentRoleConfig

// AgentRoleConfig Agent角色配置模型
type AgentRoleConfig struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	Role         AgentRole      `json:"role"`
	BaseAgentID  uuid.UUID      `json:"base_agent_id,omitempty"`
	Description  string         `json:"description"`
	SystemPrompt string         `json:"system_prompt"`
	MaxTokens    int            `json:"max_tokens"`
	Temperature  float64        `json:"temperature"`
	RoutingConfig RoutingConfig `json:"routing_config"`
	IsDefault    bool           `json:"is_default"`
	IsSystem     bool           `json:"is_system"`     // 是否为系统预置角色

	// 能力声明（用于自动编排工作流）
	Capabilities []string `json:"capabilities"`  // 能力列表：这个 Agent 能做什么
	Dependencies []string `json:"dependencies"`  // 依赖列表：需要什么上游产物
	Outputs      []string `json:"outputs"`       // 产出列表：产出什么产物

	// 配置生成相关字段
	ConfigGeneratedAt *time.Time `json:"config_generated_at,omitempty"` // 配置最后生成时间
	ConfigPath        string     `json:"config_path,omitempty"`         // 配置目录路径

	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

func (a *AgentRoleConfig) TableName() string {
	return "agent_configs"
}

// CreateAgentRequest 创建Agent请求
type CreateAgentRequest struct {
	Name         string         `json:"name" binding:"required"`
	Role         AgentRole      `json:"role"`  // 可选，默认为 custom
	BaseAgentID  uuid.UUID      `json:"base_agent_id"`
	Description  string         `json:"description"`
	SystemPrompt string         `json:"system_prompt" binding:"required"`
	MaxTokens    int            `json:"max_tokens"`
	Temperature  float64        `json:"temperature"`
	RoutingConfig *RoutingConfig `json:"routing_config"`
	IsDefault    bool           `json:"is_default"`

	// 能力声明
	Capabilities []string `json:"capabilities"`
	Dependencies []string `json:"dependencies"`
	Outputs      []string `json:"outputs"`
}