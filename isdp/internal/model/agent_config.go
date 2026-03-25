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
	CanRouteTo    []AgentRole `json:"canRouteTo"`
	RouteOnSignal []string    `json:"routeOnSignal"`
}

// AgentConfig Agent配置模型（保留兼容性，现已改名为AgentRole）
// 建议使用AgentRole别名
type AgentConfig = AgentRoleConfig

// AgentRoleConfig Agent角色配置模型
type AgentRoleConfig struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	Role         AgentRole      `json:"role"`
	BaseAgentID  uuid.UUID      `json:"baseAgentId,omitempty"`
	Description  string         `json:"description"`
	SystemPrompt string         `json:"systemPrompt"`
	MaxTokens    int            `json:"maxTokens"`
	Temperature  float64        `json:"temperature"`
	RoutingConfig RoutingConfig `json:"routingConfig"`
	IsDefault    bool           `json:"isDefault"`
	IsSystem     bool           `json:"isSystem"`     // 是否为系统预置角色

	// 能力声明（用于自动编排工作流）
	Capabilities []string `json:"capabilities"`  // 能力列表：这个 Agent 能做什么
	Dependencies []string `json:"dependencies"`  // 依赖列表：需要什么上游产物
	Outputs      []string `json:"outputs"`       // 产出列表：产出什么产物

	// 配置生成相关字段
	ConfigGeneratedAt *time.Time `json:"configGeneratedAt,omitempty"` // 配置最后生成时间
	ConfigPath        string     `json:"configPath,omitempty"`         // 配置目录路径

	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

func (a *AgentRoleConfig) TableName() string {
	return "agent_configs"
}

// CreateAgentRequest 创建Agent请求
type CreateAgentRequest struct {
	Name         string         `json:"name" binding:"required"`
	Role         AgentRole      `json:"role"`  // 可选，默认为 custom
	BaseAgentID  uuid.UUID      `json:"baseAgentId"`
	Description  string         `json:"description"`
	SystemPrompt string         `json:"systemPrompt" binding:"required"`
	MaxTokens    int            `json:"maxTokens"`
	Temperature  float64        `json:"temperature"`
	RoutingConfig *RoutingConfig `json:"routingConfig"`
	IsDefault    bool           `json:"isDefault"`

	// 能力声明
	Capabilities []string `json:"capabilities"`
	Dependencies []string `json:"dependencies"`
	Outputs      []string `json:"outputs"`
}