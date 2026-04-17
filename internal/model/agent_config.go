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

	// 配置生成相关字段
	ConfigGeneratedAt *time.Time `json:"configGeneratedAt,omitempty"` // 配置最后生成时间
	ConfigPath        string     `json:"configPath,omitempty"`        // 配置目录路径

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (a *AgentRoleConfig) TableName() string {
	return "agent_configs"
}

// IsSameRole 判断是否为同一角色类型
func (a *AgentRoleConfig) IsSameRole(other *AgentRoleConfig) bool {
	if a == nil || other == nil {
		return false
	}
	return a.Role == other.Role
}

// IsSameAgent 判断是否为同一个 Agent 实例
func (a *AgentRoleConfig) IsSameAgent(other *AgentRoleConfig) bool {
	if a == nil || other == nil {
		return false
	}
	return a.ID == other.ID
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

// ========== Batch Operation Models ==========

// BatchGenerateConfigRequest 批量生成配置请求
type BatchGenerateConfigRequest struct {
	AgentIds []string `json:"agentIds" binding:"required"`
	CliType  string   `json:"cliType"`
}

// BatchGenerateResult 批量生成配置结果
type BatchGenerateResult struct {
	Total   int                  `json:"total"`
	Success int                  `json:"success"`
	Failed  int                  `json:"failed"`
	Results []GenerateResultItem `json:"results"`
}

// GenerateResultItem 单个Agent生成结果
type GenerateResultItem struct {
	AgentId        uuid.UUID `json:"agentId"`
	AgentName      string    `json:"agentName"`
	Status         string    `json:"status"`
	SkillsCount    int       `json:"skillsCount"`
	CommandsCount  int       `json:"commandsCount"`
	SubagentsCount int       `json:"subagentsCount"`
	RulesCount     int       `json:"rulesCount"`
	SettingsCount  int       `json:"settingsCount"`
	Error          string    `json:"error,omitempty"`
}

// BatchUpdateBaseAgentRequest 批量修改基础Agent请求
type BatchUpdateBaseAgentRequest struct {
	AgentIds    []string `json:"agentIds" binding:"required"`
	BaseAgentId string   `json:"baseAgentId" binding:"required"`
}

// BatchUpdateResult 批量修改基础Agent结果
type BatchUpdateResult struct {
	Total   int               `json:"total"`
	Success int               `json:"success"`
	Failed  int               `json:"failed"`
	Results []UpdateResultItem `json:"results"`
}

// UpdateResultItem 单个Agent修改结果
type UpdateResultItem struct {
	AgentId       uuid.UUID `json:"agentId"`
	AgentName     string    `json:"agentName"`
	BaseAgentName string    `json:"baseAgentName"`
	Status        string    `json:"status"`
	Error         string    `json:"error,omitempty"`
}