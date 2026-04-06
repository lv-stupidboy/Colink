package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Base Agent Models ==========

// BaseAgentType 基础Agent类型
type BaseAgentType string

const (
	BaseAgentTypeClaudeCode  BaseAgentType = "claude_code"
	BaseAgentTypeOpenCode    BaseAgentType = "open_code"
	BaseAgentTypeOpenCodeACP BaseAgentType = "open_code_acp"
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
	IsDefault      bool          `json:"is_default"` // 是否为默认基础Agent
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
// 使用指针类型区分"不更新"（nil）和"清空"（空字符串）
type UpdateBaseAgentRequest struct {
	Name           string        `json:"name"`
	Type           BaseAgentType `json:"type"`
	ApiURL         *string       `json:"apiUrl"`         // 指针类型，支持清空
	ApiToken       *string       `json:"apiToken"`       // 指针类型，支持清空
	DefaultModel   *string       `json:"defaultModel"`   // 指针类型，支持清空
	CliPath        string        `json:"cliPath"`
	GitBashPath    *string       `json:"gitBashPath"`    // 指针类型，支持清空
	MaxTokens      int           `json:"maxTokens"`
	TimeoutMinutes int           `json:"timeoutMinutes"`
}

// BaseAgentTypeInfo 基础Agent类型信息
type BaseAgentTypeInfo struct {
	Type        BaseAgentType `json:"type"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
}