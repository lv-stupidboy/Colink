package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Base Agent Models ==========

// BaseAgentType 基础Agent类型
type BaseAgentType string

const (
	BaseAgentTypeClaudeCode BaseAgentType = "claude_code"
	BaseAgentTypeOpenCode   BaseAgentType = "open_code"
)

// BaseAgent 基础Agent配置模型
type BaseAgent struct {
	ID             uuid.UUID     `json:"id"`
	Name           string        `json:"name"`
	Type           BaseAgentType `json:"type"`
	ApiURL         string        `json:"apiUrl,omitempty"`
	ApiToken       string        `json:"apiToken,omitempty"` // 加密存储，返回时隐藏
	DefaultModel   string        `json:"defaultModel"`
	CliPath        string        `json:"cliPath"`
	GitBashPath    string        `json:"gitBashPath,omitempty"` // Windows下git-bash路径，用于Claude CLI
	MaxTokens      int           `json:"maxTokens"`
	TimeoutMinutes int           `json:"timeoutMinutes"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
}

func (b *BaseAgent) TableName() string {
	return "base_agents"
}

// CreateBaseAgentRequest 创建基础Agent请求
type CreateBaseAgentRequest struct {
	Name           string        `json:"name" binding:"required"`
	Type           BaseAgentType `json:"type" binding:"required"`
	ApiURL         string        `json:"apiUrl"`
	ApiToken       string        `json:"apiToken"`
	DefaultModel   string        `json:"defaultModel"`
	CliPath        string        `json:"cliPath"`
	GitBashPath    string        `json:"gitBashPath"`
	MaxTokens      int           `json:"maxTokens"`
	TimeoutMinutes int           `json:"timeoutMinutes"`
}

// UpdateBaseAgentRequest 更新基础Agent请求
type UpdateBaseAgentRequest struct {
	Name           string        `json:"name"`
	Type           BaseAgentType `json:"type"`
	ApiURL         string        `json:"apiUrl"`
	ApiToken       string        `json:"apiToken"`
	DefaultModel   string        `json:"defaultModel"`
	CliPath        string        `json:"cliPath"`
	GitBashPath    string        `json:"gitBashPath"`
	MaxTokens      int           `json:"maxTokens"`
	TimeoutMinutes int           `json:"timeoutMinutes"`
}

// BaseAgentTypeInfo 基础Agent类型信息
type BaseAgentTypeInfo struct {
	Type        BaseAgentType `json:"type"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
}