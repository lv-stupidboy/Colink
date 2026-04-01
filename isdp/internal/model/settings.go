package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Settings Models ==========

// Settings 配置目录资产模型
type Settings struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	DirectoryPath string    `json:"directoryPath,omitempty"` // 存储路径
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (s *Settings) TableName() string {
	return "settings"
}

// AgentSettingsBinding Agent角色与Settings绑定
type AgentSettingsBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agentRoleId"`
	SettingsID  uuid.UUID `json:"settingsId"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (a *AgentSettingsBinding) TableName() string {
	return "agent_settings_bindings"
}

// CreateSettingsRequest 创建Settings请求
type CreateSettingsRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateSettingsRequest 更新Settings请求
type UpdateSettingsRequest struct {
	Description string `json:"description"`
}

// SettingsListQuery Settings列表查询参数
type SettingsListQuery struct {
	Search   string `form:"search"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// BindSettingsRequest 绑定Settings请求
type BindSettingsRequest struct {
	SettingsIDs []uuid.UUID `json:"settingsIds" binding:"required"`
}