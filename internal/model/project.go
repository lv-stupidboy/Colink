// Package model 定义ISDP平台的核心数据模型
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ========== Project Models ==========

type ProjectType string

const (
	ProjectTypeService ProjectType = "service"
	ProjectTypeApp     ProjectType = "app"
	ProjectTypeTask    ProjectType = "task"
)

type ProjectMode string

const (
	ProjectModeNew     ProjectMode = "new"
	ProjectModeEnhance ProjectMode = "enhance"
)

type ProjectStatus string

const (
	ProjectStatusDraft      ProjectStatus = "draft"
	ProjectStatusDeveloping ProjectStatus = "developing"
	ProjectStatusTesting    ProjectStatus = "testing"
	ProjectStatusDeployed   ProjectStatus = "deployed"
	ProjectStatusArchived   ProjectStatus = "archived"
)

// Project 项目模型
type Project struct {
	ID                 uuid.UUID       `json:"id"`
	Name               string          `json:"name"`
	Type               ProjectType     `json:"type"`
	Mode               ProjectMode     `json:"mode"`
	Status             ProjectStatus   `json:"status"`
	LocalPath          string          `json:"localPath"`                       // 本地路径（必填）
	GitRepo            *string         `json:"gitRepo,omitempty"`
	Config             json.RawMessage `json:"config,omitempty"`
	WorkflowTemplateID *uuid.UUID      `json:"workflowTemplateId,omitempty"` // 新增：绑定的工作流模板ID
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
}

func (p *Project) TableName() string {
	return "projects"
}

// CreateProjectRequest 创建项目请求
type CreateProjectRequest struct {
	Name               string      `json:"name" binding:"required"`
	Type               ProjectType `json:"type"` // 可选，默认 service
	Mode               ProjectMode `json:"mode"` // 可选，默认 new
	LocalPath          string      `json:"localPath" binding:"required"` // 本地路径（必填）
	ExistingRepoURL    string      `json:"existingRepoUrl,omitempty"`
	Branch             string      `json:"branch,omitempty"`
	WorkflowTemplateID *uuid.UUID  `json:"workflowTemplateId,omitempty"` // 新增：绑定的工作流模板ID
}

// UpdateProjectRequest 更新项目请求
type UpdateProjectRequest struct {
	Name               *string        `json:"name"`
	Type               *ProjectType   `json:"type"`
	Mode               *ProjectMode   `json:"mode"`
	Status             *ProjectStatus `json:"status"`
	LocalPath          *string        `json:"localPath"`
	GitRepo            *string        `json:"gitRepo"`
	WorkflowTemplateID *uuid.UUID     `json:"workflowTemplateId"` // 可为null表示解绑
}

// Validate 验证请求
func (r *CreateProjectRequest) Validate() error {
	if r.Name == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	if r.LocalPath == "" {
		return &ValidationError{Field: "localPath", Message: "localPath is required"}
	}
	if r.Mode == ProjectModeEnhance && r.ExistingRepoURL == "" {
		return &ValidationError{Field: "existingRepoUrl", Message: "enhance mode requires existingRepoUrl"}
	}
	return nil
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// FileInfo 文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// ListFilesResponse 文件列表响应
type ListFilesResponse struct {
	Path    string     `json:"path"`
	Files   []FileInfo `json:"files"`
	HasMore bool       `json:"hasMore"`
}

// BrowsePathRequest 浏览路径请求
type BrowsePathRequest struct {
	Path string `json:"path"`
}

// BrowsePathResponse 浏览路径响应
type BrowsePathResponse struct {
	CurrentPath string     `json:"currentPath"`
	ParentPath  string     `json:"parentPath"`
	Entries     []FileInfo `json:"entries"`
	Drives      []string   `json:"drives,omitempty"` // Windows 驱动器列表
	IsValid     bool       `json:"isValid"`
	Error       string     `json:"error,omitempty"`
}

// CreateFolderRequest 创建文件夹请求
type CreateFolderRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// ValidatePathResponse 验证路径响应
type ValidatePathResponse struct {
	IsValid   bool   `json:"isValid"`
	Exists    bool   `json:"exists"`
	IsDir     bool   `json:"isDir"`
	Writable  bool   `json:"writable"`
	Error     string `json:"error,omitempty"`
	CanCreate bool   `json:"canCreate"` // 是否可以在此路径创建项目
}

// FileContentResponse 文件内容响应
type FileContentResponse struct {
	Content   string `json:"content"`   // 文件内容
	Size      int64  `json:"size"`      // 文件大小（字节）
	Truncated bool   `json:"truncated"` // 是否截断（超过1MB）
	Path      string `json:"path"`      // 文件路径
	IsBinary  bool   `json:"isBinary"`  // 是否二进制文件
}