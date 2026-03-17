package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// ProjectRepository 项目数据访问
type ProjectRepository struct {
	db *sql.DB
}

// NewProjectRepository 创建项目Repository
func NewProjectRepository(db *sql.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

// Create 创建项目
func (r *ProjectRepository) Create(ctx context.Context, project *model.Project) error {
	query := `
		INSERT INTO projects (id, name, type, mode, status, local_path, git_repo, config, workflow_template_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()

	// 处理 nullable workflow_template_id
	var workflowTemplateID interface{}
	if project.WorkflowTemplateID != nil {
		workflowTemplateID = project.WorkflowTemplateID.String()
	}

	_, err := r.db.ExecContext(ctx, query,
		project.ID.String(),
		project.Name,
		project.Type,
		project.Mode,
		project.Status,
		project.LocalPath,
		project.GitRepo,
		project.Config,
		workflowTemplateID,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	project.CreatedAt = now
	project.UpdatedAt = now
	return nil
}

// FindByID 根据ID查找项目
func (r *ProjectRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	query := `
		SELECT id, name, type, mode, status, local_path, git_repo, config, workflow_template_id, created_at, updated_at
		FROM projects WHERE id = ?
	`
	project := &model.Project{}
	var idStr string
	var config []byte
	var workflowTemplateID sql.NullString
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr,
		&project.Name,
		&project.Type,
		&project.Mode,
		&project.Status,
		&project.LocalPath,
		&project.GitRepo,
		&config,
		&workflowTemplateID,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find project: %w", err)
	}
	project.ID, _ = uuid.Parse(idStr)
	if config != nil {
		project.Config = config
	}
	if workflowTemplateID.Valid {
		wid, _ := uuid.Parse(workflowTemplateID.String)
		project.WorkflowTemplateID = &wid
	}
	return project, nil
}

// FindAll 查找所有项目
func (r *ProjectRepository) FindAll(ctx context.Context, limit, offset int) ([]*model.Project, error) {
	query := `
		SELECT id, name, type, mode, status, local_path, git_repo, config, workflow_template_id, created_at, updated_at
		FROM projects ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to find projects: %w", err)
	}
	defer rows.Close()

	var projects []*model.Project
	for rows.Next() {
		project := &model.Project{}
		var idStr string
		var config []byte
		var workflowTemplateID sql.NullString
		err := rows.Scan(
			&idStr,
			&project.Name,
			&project.Type,
			&project.Mode,
			&project.Status,
			&project.LocalPath,
			&project.GitRepo,
			&config,
			&workflowTemplateID,
			&project.CreatedAt,
			&project.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		project.ID, _ = uuid.Parse(idStr)
		if config != nil {
			project.Config = config
		}
		if workflowTemplateID.Valid {
			wid, _ := uuid.Parse(workflowTemplateID.String)
			project.WorkflowTemplateID = &wid
		}
		projects = append(projects, project)
	}
	return projects, nil
}

// Update 更新项目
func (r *ProjectRepository) Update(ctx context.Context, project *model.Project) error {
	query := `
		UPDATE projects
		SET name = ?, type = ?, mode = ?, status = ?, local_path = ?, git_repo = ?, config = ?, workflow_template_id = ?, updated_at = ?
		WHERE id = ?
	`
	project.UpdatedAt = time.Now()

	// 处理 nullable workflow_template_id
	var workflowTemplateID interface{}
	if project.WorkflowTemplateID != nil {
		workflowTemplateID = project.WorkflowTemplateID.String()
	}

	_, err := r.db.ExecContext(ctx, query,
		project.Name,
		project.Type,
		project.Mode,
		project.Status,
		project.LocalPath,
		project.GitRepo,
		project.Config,
		workflowTemplateID,
		project.UpdatedAt,
		project.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}
	return nil
}

// Delete 删除项目
func (r *ProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM projects WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	return nil
}

// GetByThreadID 根据ThreadID获取项目
func (r *ProjectRepository) GetByThreadID(ctx context.Context, threadID uuid.UUID) (*model.Project, error) {
	query := `
		SELECT p.id, p.name, p.type, p.mode, p.status, p.local_path, p.git_repo, p.config, p.workflow_template_id, p.created_at, p.updated_at
		FROM projects p
		JOIN threads t ON t.project_id = p.id
		WHERE t.id = ?
	`
	project := &model.Project{}
	var idStr string
	var config []byte
	var workflowTemplateID sql.NullString
	err := r.db.QueryRowContext(ctx, query, threadID.String()).Scan(
		&idStr,
		&project.Name,
		&project.Type,
		&project.Mode,
		&project.Status,
		&project.LocalPath,
		&project.GitRepo,
		&config,
		&workflowTemplateID,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find project by thread: %w", err)
	}
	project.ID, _ = uuid.Parse(idStr)
	project.Config = json.RawMessage(config)
	if workflowTemplateID.Valid {
		wid, _ := uuid.Parse(workflowTemplateID.String)
		project.WorkflowTemplateID = &wid
	}
	return project, nil
}