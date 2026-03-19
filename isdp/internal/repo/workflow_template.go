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

// WorkflowTemplateRepository 工作流模板数据访问
type WorkflowTemplateRepository struct {
	db *sql.DB
}

// NewWorkflowTemplateRepository 创建工作流模板Repository
func NewWorkflowTemplateRepository(db *sql.DB) *WorkflowTemplateRepository {
	return &WorkflowTemplateRepository{db: db}
}

// Create 创建工作流模板
func (r *WorkflowTemplateRepository) Create(ctx context.Context, template *model.WorkflowTemplate) error {
	query := `
		INSERT INTO workflow_templates (id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query,
		template.ID.String(),
		template.Name,
		template.Description,
		[]byte(template.AgentIDs),      // 转换为 []byte
		[]byte(template.Transitions),   // 转换为 []byte
		[]byte(template.Checkpoints),   // 转换为 []byte
		template.EstimatedTime,
		template.IsSystem,
		template.IsDefault,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create workflow template: %w", err)
	}
	template.CreatedAt = now
	template.UpdatedAt = now
	return nil
}

// FindByID 根据ID查找工作流模板
func (r *WorkflowTemplateRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.WorkflowTemplate, error) {
	query := `
		SELECT id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, created_at, updated_at
		FROM workflow_templates WHERE id = ?
	`
	template := &model.WorkflowTemplate{}
	var idStr string
	var agentIDs, transitions, checkpoints []byte
	var isSystem, isDefault int
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr,
		&template.Name,
		&template.Description,
		&agentIDs,
		&transitions,
		&checkpoints,
		&template.EstimatedTime,
		&isSystem,
		&isDefault,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow template: %w", err)
	}
	template.ID, _ = uuid.Parse(idStr)
	template.AgentIDs = json.RawMessage(agentIDs)
	template.Transitions = json.RawMessage(transitions)
	template.Checkpoints = json.RawMessage(checkpoints)
	template.IsSystem = isSystem == 1
	template.IsDefault = isDefault == 1
	return template, nil
}

// FindAll 查找所有工作流模板
func (r *WorkflowTemplateRepository) FindAll(ctx context.Context) ([]*model.WorkflowTemplate, error) {
	query := `
		SELECT id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, created_at, updated_at
		FROM workflow_templates ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow templates: %w", err)
	}
	defer rows.Close()

	var templates = make([]*model.WorkflowTemplate, 0) // 初始化为空数组，避免 JSON null
	for rows.Next() {
		template := &model.WorkflowTemplate{}
		var idStr string
		var agentIDs, transitions, checkpoints []byte
		var isSystem, isDefault int
		err := rows.Scan(
			&idStr,
			&template.Name,
			&template.Description,
			&agentIDs,
			&transitions,
			&checkpoints,
			&template.EstimatedTime,
			&isSystem,
			&isDefault,
			&template.CreatedAt,
			&template.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workflow template: %w", err)
		}
		template.ID, _ = uuid.Parse(idStr)
		template.AgentIDs = json.RawMessage(agentIDs)
		template.Transitions = json.RawMessage(transitions)
		template.Checkpoints = json.RawMessage(checkpoints)
		template.IsSystem = isSystem == 1
		template.IsDefault = isDefault == 1
		templates = append(templates, template)
	}
	return templates, nil
}

// Update 更新工作流模板
func (r *WorkflowTemplateRepository) Update(ctx context.Context, template *model.WorkflowTemplate) error {
	query := `
		UPDATE workflow_templates
		SET name = ?, description = ?, agent_ids = ?, transitions = ?, checkpoints = ?, estimated_time = ?, updated_at = ?
		WHERE id = ?
	`
	template.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, query,
		template.Name,
		template.Description,
		[]byte(template.AgentIDs),      // 转换为 []byte
		[]byte(template.Transitions),   // 转换为 []byte
		[]byte(template.Checkpoints),   // 转换为 []byte
		template.EstimatedTime,
		template.UpdatedAt,
		template.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update workflow template: %w", err)
	}
	return nil
}

// Delete 删除工作流模板
func (r *WorkflowTemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM workflow_templates WHERE id = ? AND is_system = 0`
	result, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete workflow template: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("cannot delete system template or template not found")
	}
	return nil
}

// GetDefault 获取默认工作流模板
func (r *WorkflowTemplateRepository) GetDefault(ctx context.Context) (*model.WorkflowTemplate, error) {
	query := `
		SELECT id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, created_at, updated_at
		FROM workflow_templates WHERE is_default = 1 LIMIT 1
	`
	template := &model.WorkflowTemplate{}
	var idStr string
	var agentIDs, transitions, checkpoints []byte
	var isSystem, isDefault int
	err := r.db.QueryRowContext(ctx, query).Scan(
		&idStr,
		&template.Name,
		&template.Description,
		&agentIDs,
		&transitions,
		&checkpoints,
		&template.EstimatedTime,
		&isSystem,
		&isDefault,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("no default workflow template found: %w", err)
	}
	template.ID, _ = uuid.Parse(idStr)
	template.AgentIDs = json.RawMessage(agentIDs)
	template.Transitions = json.RawMessage(transitions)
	template.Checkpoints = json.RawMessage(checkpoints)
	template.IsSystem = isSystem == 1
	template.IsDefault = isDefault == 1
	return template, nil
}

// SetDefault 设置默认工作流模板
func (r *WorkflowTemplateRepository) SetDefault(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. 清除所有工作流的默认标记
	_, err = tx.ExecContext(ctx, "UPDATE workflow_templates SET is_default = 0")
	if err != nil {
		return fmt.Errorf("failed to clear default flags: %w", err)
	}

	// 2. 设置指定工作流为默认
	result, err := tx.ExecContext(ctx, "UPDATE workflow_templates SET is_default = 1 WHERE id = ?", id.String())
	if err != nil {
		return fmt.Errorf("failed to set default: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("workflow template not found: %s", id.String())
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CountProjectReferences 统计引用该工作流的项目数量
func (r *WorkflowTemplateRepository) CountProjectReferences(ctx context.Context, id uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM projects WHERE workflow_template_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count project references: %w", err)
	}
	return count, nil
}