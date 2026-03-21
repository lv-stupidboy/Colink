package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Skill Models ==========

// SkillType Skill类型
type SkillType string

const (
	SkillTypeSkill SkillType = "skill"
	SkillTypeRule  SkillType = "rule"
)

// SkillSourceType 来源类型
type SkillSourceType string

const (
	SkillSourceBuiltIn   SkillSourceType = "built_in"
	SkillSourceUploaded  SkillSourceType = "uploaded"
	SkillSourceFederated SkillSourceType = "federated"
)

// SkillStatus Skill状态
type SkillStatus string

const (
	SkillStatusActive     SkillStatus = "active"
	SkillStatusDeprecated SkillStatus = "deprecated"
)

// InstallSource 安装源配置
type InstallSource map[string]string // agent_type -> url

// Skill 技能模型
type Skill struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	DisplayName   string          `json:"display_name,omitempty"`
	Description   string          `json:"description,omitempty"`
	Type          SkillType       `json:"type"`
	Category      string          `json:"category,omitempty"`

	// 来源信息
	SourceType       SkillSourceType `json:"source_type"`
	SourceRegistryID uuid.UUID       `json:"source_registry_id,omitempty"`
	AuthorID         uuid.UUID       `json:"author_id,omitempty"`
	ProjectID        uuid.UUID       `json:"project_id,omitempty"`

	// 安装信息
	InstallSource InstallSource `json:"install_source,omitempty"`

	// 兼容性
	SupportedAgents []string `json:"supported_agents,omitempty"`

	// 版本
	Version string `json:"version"`

	// 统计数据
	UseCount      int `json:"use_count"`
	StarCount     int `json:"star_count"`
	FavoriteCount int `json:"favorite_count"`

	// 状态
	Status   SkillStatus `json:"status"`
	IsPublic bool        `json:"is_public"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Skill) TableName() string {
	return "skills"
}

// AgentSkillBinding Agent与Skill关联
type AgentSkillBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agent_role_id"`
	SkillID     uuid.UUID `json:"skill_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *AgentSkillBinding) TableName() string {
	return "agent_skill_bindings"
}

// SkillFavorite Skill收藏记录
type SkillFavorite struct {
	ID        uuid.UUID `json:"id"`
	SkillID   uuid.UUID `json:"skill_id"`
	UserID    uuid.UUID `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *SkillFavorite) TableName() string {
	return "skill_favorites"
}

// CreateSkillRequest 创建Skill请求
type CreateSkillRequest struct {
	Name            string          `json:"name" binding:"required"`
	DisplayName     string          `json:"display_name"`
	Description     string          `json:"description"`
	Type            SkillType       `json:"type"`
	Category        string          `json:"category"`
	SourceType      SkillSourceType `json:"source_type" binding:"required"`
	InstallSource   InstallSource   `json:"install_source"`
	SupportedAgents []string        `json:"supported_agents"`
	Version         string          `json:"version"`
	IsPublic        bool            `json:"is_public"`
}

// UpdateSkillRequest 更新Skill请求
type UpdateSkillRequest struct {
	DisplayName     string        `json:"display_name"`
	Description     string        `json:"description"`
	Type            SkillType     `json:"type"`
	Category        string        `json:"category"`
	InstallSource   InstallSource `json:"install_source"`
	SupportedAgents []string      `json:"supported_agents"`
	Version         string        `json:"version"`
	Status          string        `json:"status"`
	IsPublic        bool          `json:"is_public"`
}

// BindSkillRequest 绑定Skill请求
type BindSkillRequest struct {
	SkillIDs []uuid.UUID `json:"skill_ids" binding:"required"`
}

// SkillListQuery Skill列表查询参数
type SkillListQuery struct {
	Type       string `form:"type"`
	Category   string `form:"category"`
	SourceType string `form:"source_type"`
	AgentType  string `form:"agent_type"`
	Search     string `form:"search"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

// SkillWithBindings Skill及其绑定的Agent列表
type SkillWithBindings struct {
	*Skill
	BoundAgents []*AgentRoleConfig `json:"bound_agents,omitempty"`
}

// ========== Skill Registry Models ==========

// RegistryType 注册表类型
type RegistryType string

const (
	RegistryTypeGitHub RegistryType = "github"
	RegistryTypeGitLab RegistryType = "gitlab"
	RegistryTypeAPI    RegistryType = "api"
	RegistryTypeCustom RegistryType = "custom"
)

// RegistrySyncStatus 同步状态
type RegistrySyncStatus string

const (
	RegistrySyncPending RegistrySyncStatus = "pending"
	RegistrySyncSuccess RegistrySyncStatus = "success"
	RegistrySyncFailed  RegistrySyncStatus = "failed"
)

// RegistryStatus 注册表状态
type RegistryStatus string

const (
	RegistryStatusActive   RegistryStatus = "active"
	RegistryStatusInactive RegistryStatus = "inactive"
)

// SkillRegistry 联邦技能源配置
type SkillRegistry struct {
	ID           uuid.UUID          `json:"id"`
	Name         string             `json:"name"`
	DisplayName  string             `json:"display_name,omitempty"`
	Type         RegistryType       `json:"type"`
	URL          string             `json:"url"`
	AuthConfig   map[string]string  `json:"auth_config,omitempty"` // 加密存储
	SyncInterval int                `json:"sync_interval"`
	LastSyncAt   *time.Time         `json:"last_sync_at,omitempty"`
	SyncStatus   RegistrySyncStatus `json:"sync_status"`
	SkillCount   int                `json:"skill_count"`
	Status       RegistryStatus     `json:"status"`
	CreatedAt    time.Time          `json:"created_at"`
}

func (r *SkillRegistry) TableName() string {
	return "skill_registries"
}

// CreateRegistryRequest 创建注册表请求
type CreateRegistryRequest struct {
	Name         string            `json:"name" binding:"required"`
	DisplayName  string            `json:"display_name"`
	Type         RegistryType      `json:"type" binding:"required"`
	URL          string            `json:"url" binding:"required"`
	AuthConfig   map[string]string `json:"auth_config"`
	SyncInterval int               `json:"sync_interval"`
}

// UpdateRegistryRequest 更新注册表请求
type UpdateRegistryRequest struct {
	DisplayName  string            `json:"display_name"`
	URL          string            `json:"url"`
	AuthConfig   map[string]string `json:"auth_config"`
	SyncInterval int               `json:"sync_interval"`
	Status       RegistryStatus    `json:"status"`
}

// SyncResult 同步结果
type SyncResult struct {
	RegistryID   uuid.UUID `json:"registry_id"`
	RegistryName string    `json:"registry_name"`
	SkillsAdded  int       `json:"skills_added"`
	SkillsUpdated int      `json:"skills_updated"`
	SkillsRemoved int      `json:"skills_removed"`
	Error        string    `json:"error,omitempty"`
}