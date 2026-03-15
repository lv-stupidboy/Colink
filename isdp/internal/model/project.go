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
	GitRepo            string          `json:"git_repo,omitempty"`
	Config             json.RawMessage `json:"config,omitempty"`
	WorkflowTemplateID *uuid.UUID      `json:"workflow_template_id,omitempty"` // 新增：绑定的工作流模板ID
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

func (p *Project) TableName() string {
	return "projects"
}

// CreateProjectRequest 创建项目请求
type CreateProjectRequest struct {
	Name            string      `json:"name" binding:"required"`
	Type            ProjectType `json:"type" binding:"required,oneof=service app task"`
	Mode            ProjectMode `json:"mode" binding:"required,oneof=new enhance"`
	ExistingRepoURL string      `json:"existing_repo_url,omitempty"`
	Branch          string      `json:"branch,omitempty"`
}

// UpdateProjectRequest 更新项目请求
type UpdateProjectRequest struct {
	Name               *string        `json:"name"`
	Type               *ProjectType   `json:"type"`
	Mode               *ProjectMode   `json:"mode"`
	Status             *ProjectStatus `json:"status"`
	GitRepo            *string        `json:"git_repo"`
	WorkflowTemplateID *uuid.UUID     `json:"workflow_template_id"` // 可为null表示解绑
}

// Validate 验证请求
func (r *CreateProjectRequest) Validate() error {
	if r.Name == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	if r.Mode == ProjectModeEnhance && r.ExistingRepoURL == "" {
		return &ValidationError{Field: "existing_repo_url", Message: "enhance mode requires existing_repo_url"}
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