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
	SourcePath       string          `json:"sourcePath,omitempty"` // 联邦源仓库相对路径
	AuthorID         uuid.UUID       `json:"authorId,omitempty"`
	ProjectID        uuid.UUID       `json:"projectId,omitempty"`

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
	IsPublic        bool            `json:"isPublic"` // 仅对 uploaded 类型有效
}

// UpdateSkillRequest 更新Skill请求
type UpdateSkillRequest struct {
	Description     string   `json:"description"`
	Tags            []string `json:"tags"`
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
	RegistryTypeGitHub  RegistryType = "github"
	RegistryTypeGitLab  RegistryType = "gitlab"
	RegistryTypeAPI     RegistryType = "api"
	RegistryTypeCustom  RegistryType = "custom"
	RegistryTypeCodeHub RegistryType = "codehub" // 华为内网 CodeHub
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

// LocalSkillInfo 本地同名 Skill 信息（用于冲突展示）
type LocalSkillInfo struct {
	ID               uuid.UUID `json:"id"`
	SourceType       string    `json:"sourceType"`
	SourceRegistryID uuid.UUID `json:"sourceRegistryId,omitempty"`
	SourceRegistryName string  `json:"sourceRegistryName,omitempty"` // 联邦源名称（如果是 federated）
	SourcePath       string    `json:"sourcePath,omitempty"` // 本地路径
	Description      string    `json:"description"`
}

// RemoteSkill 远程 Skill 信息（扫描结果）
type RemoteSkill struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Path          string          `json:"path"`          // Skill 在仓库中的相对路径
	ExistsLocally bool            `json:"existsLocally"` // 是否已存在本地同名 Skill
	LocalSkill    *LocalSkillInfo `json:"localSkill,omitempty"` // 本地同名 Skill 信息
}

// SkillImportItem 单个 Skill 导入项
type SkillImportItem struct {
	Name            string    `json:"name" binding:"required"`
	Path            string    `json:"path" binding:"required"`
	Description     string    `json:"description"`
	Tags            []string  `json:"tags"`
	ImportMode      string    `json:"importMode"`    // create 或 update（默认 create）
	TargetSkillID   uuid.UUID `json:"targetSkillId"` // update 时指定目标 Skill ID
}

// BatchImportRequest 批量导入请求
type BatchImportRequest struct {
	RegistryID uuid.UUID        `json:"registryId" binding:"required"`
	Skills     []SkillImportItem `json:"skills" binding:"required,min=1"`
}

// BatchImportResult 批量导入结果
type BatchImportResult struct {
	Imported        []*Skill           `json:"imported"`
	Updated         []*Skill           `json:"updated"` // 更新的 Skill 列表
	Skipped         []SkippedSkillInfo `json:"skipped"`
	ConflictSummary *ConflictSummary   `json:"conflictSummary,omitempty"` // 冲突处理汇总
	ConfigRefreshErrors []RefreshError `json:"configRefreshErrors,omitempty"` // 配置刷新错误列表
}

// ConflictSummary 冲突处理汇总
type ConflictSummary struct {
	AutoUpdated int `json:"autoUpdated"` // 自动更新的数量（同源）
	UserCreated int `json:"userCreated"` // 用户选择新建的数量
	UserUpdated int `json:"userUpdated"` // 用户选择更新的数量
}

// SkippedSkillInfo 跳过的 Skill 信息
type SkippedSkillInfo struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ScanResult 扫描结果
type ScanResult struct {
	RegistryID   uuid.UUID       `json:"registryId"`
	RegistryName string          `json:"registryName"`
	RegistryURL  string          `json:"registryUrl"`
	Skills       []*RemoteSkill  `json:"skills"`
}

// SyncPreviewSkill 同步预览 skill（同源）
type SyncPreviewSkill struct {
	Name         string    `json:"name"`
	LocalSkillID uuid.UUID `json:"localSkillId"`
	Description  string    `json:"description"`
	Path         string    `json:"path,omitempty"` // 远程路径
}

// SyncConflictSkill 同步冲突 skill（异源）
type SyncConflictSkill struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Path        string          `json:"path,omitempty"` // 远程路径
	LocalSkill  *LocalSkillInfo `json:"localSkill"` // 复用已有类型
}

// SyncPreviewResult 同步预览结果
type SyncPreviewResult struct {
	RegistryID       uuid.UUID            `json:"registryId"`
	RegistryName     string               `json:"registryName"`
	AutoUpdateSkills []*SyncPreviewSkill  `json:"autoUpdateSkills"` // 同源同名
	ConflictSkills   []*SyncConflictSkill `json:"conflictSkills"`   // 异源同名
	NewSkills        []*RemoteSkill       `json:"newSkills"`        // 远程有本地无
	SkippedSkills    []*RemoteSkill       `json:"skippedSkills"`    // 本地有远程无
}

// SyncOperation 同步操作
type SyncOperation struct {
	Action        string    `json:"action"`        // "update" 或 "skip"
	SkillName     string    `json:"skillName"`
	TargetSkillID uuid.UUID `json:"targetSkillId"` // 仅 update 时需要
	Description   string    `json:"description"`   // 远程 skill 描述
}

// SyncConfirmRequest 同步确认请求
type SyncConfirmRequest struct {
	RegistryID uuid.UUID        `json:"registryId"`
	Operations []*SyncOperation `json:"operations"`
}

// SkippedSkill 跳过的 skill
type SkippedSkill struct {
	Name string `json:"name"`
}

// SyncConfirmResult 同步确认结果
type SyncConfirmResult struct {
	Updated     []*Skill        `json:"updated"`
	Skipped     []*SkippedSkill `json:"skipped"`
	AutoUpdated int             `json:"autoUpdated"` // 自动更新数量
	UserUpdated int             `json:"userUpdated"` // 用户选择更新数量
	UserSkipped int             `json:"userSkipped"` // 用户选择跳过数量
	ConfigRefreshErrors []RefreshError `json:"configRefreshErrors,omitempty"` // 配置刷新错误列表
}

// RefreshError 刷新配置目录错误
type RefreshError struct {
	AgentRoleID   uuid.UUID `json:"agentRoleId"`
	AgentRoleName string    `json:"agentRoleName"`
	Error         string    `json:"error"`
}