package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Agent Config Models ==========

// AgentRole Agent角色
type AgentRole string

const (
	AgentRoleRequirement  AgentRole = "requirement"
	AgentRoleArchitect    AgentRole = "architect"
	AgentRoleDeveloper    AgentRole = "developer"
	AgentRoleReviewer     AgentRole = "reviewer"
	AgentRoleTestEngineer AgentRole = "testengineer"
	AgentRoleDevOps       AgentRole = "devops"
)

// RoutingConfig 路由配置
type RoutingConfig struct {
	CanRouteTo    []AgentRole `json:"can_route_to"`
	RouteOnSignal []string    `json:"route_on_signal"`
}

// AgentConfig Agent配置模型
type AgentConfig struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	Role         AgentRole      `json:"role"`
	Description  string         `json:"description"`
	SystemPrompt string         `json:"system_prompt"`
	ModelName    string         `json:"model_name"`
	MaxTokens    int            `json:"max_tokens"`
	Temperature  float64        `json:"temperature"`
	RoutingConfig RoutingConfig `json:"routing_config"`
	IsDefault    bool           `json:"is_default"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

func (a *AgentConfig) TableName() string {
	return "agent_configs"
}

// CreateAgentRequest 创建Agent请求
type CreateAgentRequest struct {
	Name         string         `json:"name" binding:"required"`
	Role         AgentRole      `json:"role" binding:"required"`
	Description  string         `json:"description"`
	SystemPrompt string         `json:"system_prompt"`
	ModelName    string         `json:"model_name"`
	MaxTokens    int            `json:"max_tokens"`
	Temperature  float64        `json:"temperature"`
	RoutingConfig *RoutingConfig `json:"routing_config"`
	IsDefault    bool           `json:"is_default"`
}