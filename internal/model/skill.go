package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Skill Models ==========

// SkillSourceType 来源类型
type SkillSourceType string

const (
	SkillSourcePlatform  SkillSourceType = "platform"  // 平台内置
	SkillSourcePersonal  SkillSourceType = "personal"  // 个人上传
	SkillSourceFederated SkillSourceType = "federated" // 联邦同步
)

// SkillStatus Skill状态
type SkillStatus string

const (
	SkillStatusActive     SkillStatus = "active"
	SkillStatusDeprecated SkillStatus = "deprecated"
)

// Skill 技能模型
type Skill struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Tags        []string  `json:"tags,omitempty"` // 技能标签，多选

	// 来源信息
	SourceType       SkillSourceType `json:"sourceType"`
	SourceRegistryID uuid.UUID       `json:"sourceRegistryId,omitempty"`
	AuthorID         uuid.UUID       `json:"authorId,omitempty"`
	ProjectID        uuid.UUID       `json:"projectId,omitempty"`

	// 兼容性
	SupportedAgents []string `json:"supportedAgents,omitempty"`

	// 统计数据
	UseCount int `json:"useCount"`

	// 状态
	Status   SkillStatus `json:"status"`
	IsPublic bool        `json:"isPublic"` // 仅对 uploaded 类型有效

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (s *Skill) TableName() string {
	return "skills"
}

// AgentSkillBinding Agent与Skill关联
type AgentSkillBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agentRoleId"`
	SkillID     uuid.UUID `json:"skillId"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (a *AgentSkillBinding) TableName() string {
	return "agent_skill_bindings"
}

// CreateSkillRequest 创建Skill请求
type CreateSkillRequest struct {
	Name            string          `json:"name" binding:"required"`
	Description     string          `json:"description"`
	Tags            []string        `json:"tags"`
	SourceType      SkillSourceType `json:"sourceType" binding:"required"`
	SupportedAgents []string        `json:"supportedAgents" binding:"required,min=1"`
	IsPublic        bool            `json:"isPublic"` // 仅对 uploaded 类型有效
}

// UpdateSkillRequest 更新Skill请求
type UpdateSkillRequest struct {
	Description     string   `json:"description"`
	Tags            []string `json:"tags"`
	SupportedAgents []string `json:"supportedAgents"`
	Status          string   `json:"status"`
	IsPublic        bool     `json:"isPublic"` // 仅对 uploaded 类型有效
}

// BindSkillRequest 绑定Skill请求
type BindSkillRequest struct {
	SkillIDs []uuid.UUID `json:"skillIds" binding:"required"`
}

// SkillListQuery Skill列表查询参数
type SkillListQuery struct {
	Tag        string `form:"tag"`
	SourceType string `form:"source_type"`
	AgentType  string `form:"agent_type"`
	Search     string `form:"search"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

// SkillWithBindings Skill及其绑定的Agent列表
type SkillWithBindings struct {
	*Skill
	BoundAgents []*AgentRoleConfig `json:"boundAgents,omitempty"`
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
	DisplayName  string             `json:"displayName,omitempty"`
	Type         RegistryType       `json:"type"`
	URL          string             `json:"url"`
	AuthConfig   map[string]string  `json:"authConfig,omitempty"` // 加密存储
	SyncInterval int                `json:"syncInterval"`
	LastSyncAt   *time.Time         `json:"lastSyncAt,omitempty"`
	SyncStatus   RegistrySyncStatus `json:"syncStatus"`
	SkillCount   int                `json:"skillCount"`
	Status       RegistryStatus     `json:"status"`
	CreatedAt    time.Time          `json:"createdAt"`
}

func (r *SkillRegistry) TableName() string {
	return "skill_registries"
}

// CreateRegistryRequest 创建注册表请求
type CreateRegistryRequest struct {
	Name         string            `json:"name" binding:"required"`
	DisplayName  string            `json:"displayName"`
	Type         RegistryType      `json:"type" binding:"required"`
	URL          string            `json:"url" binding:"required"`
	AuthConfig   map[string]string `json:"authConfig"`
	SyncInterval int               `json:"syncInterval"`
}

// UpdateRegistryRequest 更新注册表请求
type UpdateRegistryRequest struct {
	DisplayName  string            `json:"displayName"`
	URL          string            `json:"url"`
	AuthConfig   map[string]string `json:"authConfig"`
	SyncInterval int               `json:"syncInterval"`
	Status       RegistryStatus    `json:"status"`
}

// SyncResult 同步结果
type SyncResult struct {
	RegistryID    uuid.UUID `json:"registryId"`
	RegistryName  string    `json:"registryName"`
	SkillsAdded   int       `json:"skillsAdded"`
	SkillsUpdated int       `json:"skillsUpdated"`
	SkillsRemoved int       `json:"skillsRemoved"`
	Error         string    `json:"error,omitempty"`
}

// RemoteSkill 远程 Skill 信息（扫描结果）
type RemoteSkill struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Path          string `json:"path"`          // Skill 在仓库中的相对路径
	ExistsLocally bool   `json:"existsLocally"` // 是否已存在本地同名 Skill
}

// SkillImportItem 单个 Skill 导入项
type SkillImportItem struct {
	Name            string   `json:"name" binding:"required"`
	Path            string   `json:"path" binding:"required"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags"`
	SupportedAgents []string `json:"supportedAgents" binding:"required,min=1"`
}

// BatchImportRequest 批量导入请求
type BatchImportRequest struct {
	RegistryID string            `json:"registryId" binding:"required"`
	Skills     []SkillImportItem `json:"skills" binding:"required,min=1"`
}

// BatchImportResult 批量导入结果
type BatchImportResult struct {
	Imported []*Skill           `json:"imported"`
	Skipped  []SkippedSkillInfo `json:"skipped"`
}

// SkippedSkillInfo 跳过的 Skill 信息
type SkippedSkillInfo struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ScanResult 扫描结果
type ScanResult struct {
	RegistryID   string         `json:"registryId"`
	RegistryName string         `json:"registryName"`
	RegistryURL  string         `json:"registryUrl"`
	Skills       []*RemoteSkill `json:"skills"`
}