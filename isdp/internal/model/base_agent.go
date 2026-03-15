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
	ID            uuid.UUID     `json:"id"`
	Name          string        `json:"name"`
	Type          BaseAgentType `json:"type"`
	ApiURL        string        `json:"api_url,omitempty"`
	ApiToken      string        `json:"api_token,omitempty"` // 加密存储，返回时隐藏
	DefaultModel  string        `json:"default_model"`
	CliPath       string        `json:"cli_path"`
	GitBashPath   string        `json:"git_bash_path,omitempty"` // Windows下git-bash路径，用于Claude CLI
	MaxTokens     int           `json:"max_tokens"`
	TimeoutMinutes int          `json:"timeout_minutes"`
	IsActive      bool          `json:"is_active"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

func (b *BaseAgent) TableName() string {
	return "base_agents"
}

// CreateBaseAgentRequest 创建基础Agent请求
type CreateBaseAgentRequest struct {
	Name          string        `json:"name" binding:"required"`
	Type          BaseAgentType `json:"type" binding:"required"`
	ApiURL        string        `json:"api_url"`
	ApiToken      string        `json:"api_token"`
	DefaultModel  string        `json:"default_model"`
	CliPath       string        `json:"cli_path"`
	GitBashPath   string        `json:"git_bash_path"`
	MaxTokens     int           `json:"max_tokens"`
	TimeoutMinutes int          `json:"timeout_minutes"`
	IsActive      bool          `json:"is_active"`
}

// UpdateBaseAgentRequest 更新基础Agent请求
type UpdateBaseAgentRequest struct {
	Name          string        `json:"name"`
	Type          BaseAgentType `json:"type"`
	ApiURL        string        `json:"api_url"`
	ApiToken      string        `json:"api_token"`
	DefaultModel  string        `json:"default_model"`
	CliPath       string        `json:"cli_path"`
	GitBashPath   string        `json:"git_bash_path"`
	MaxTokens     int           `json:"max_tokens"`
	TimeoutMinutes int          `json:"timeout_minutes"`
	IsActive      bool          `json:"is_active"`
}

// BaseAgentTypeInfo 基础Agent类型信息
type BaseAgentTypeInfo struct {
	Type        BaseAgentType `json:"type"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
}