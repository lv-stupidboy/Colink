package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// ThreadRepository Thread数据访问
type ThreadRepository struct {
	db *sql.DB
}

// NewThreadRepository 创建Thread Repository
func NewThreadRepository(db *sql.DB) *ThreadRepository {
	return &ThreadRepository{db: db}
}

// Create 创建Thread
func (r *ThreadRepository) Create(ctx context.Context, thread *model.Thread) error {
	query := `
		INSERT INTO threads (id, project_id, name, status, current_phase, current_agent, depth, workflow_template_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	var workflowTemplateID interface{}
	if thread.WorkflowTemplateID != nil {
		workflowTemplateID = thread.WorkflowTemplateID.String()
	}
	_, err := r.db.ExecContext(ctx, query,
		thread.ID.String(), thread.ProjectID.String(), thread.Name, thread.Status, thread.CurrentPhase, thread.CurrentAgent, thread.Depth, workflowTemplateID, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create thread: %w", err)
	}
	thread.CreatedAt = now
	thread.UpdatedAt = now
	return nil
}

// FindByID 根据ID查找Thread
func (r *ThreadRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Thread, error) {
	query := `
		SELECT id, project_id, name, status, current_phase, current_agent, depth, workflow_template_id, abort_token, created_at, updated_at
		FROM threads WHERE id = ?
	`
	thread := &model.Thread{}
	var idStr, projectIDStr string
	var projectID sql.NullString
	var workflowTemplateID sql.NullString
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &projectID, &thread.Name, &thread.Status, &thread.CurrentPhase, &thread.CurrentAgent,
		&thread.Depth, &workflowTemplateID, &thread.AbortToken, &thread.CreatedAt, &thread.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find thread: %w", err)
	}
	thread.ID, _ = uuid.Parse(idStr)
	if projectID.Valid {
		projectIDStr = projectID.String
		thread.ProjectID, _ = uuid.Parse(projectIDStr)
	}
	if workflowTemplateID.Valid {
		wid, _ := uuid.Parse(workflowTemplateID.String)
		thread.WorkflowTemplateID = &wid
	}
	return thread, nil
}

// FindByProjectID 根据项目ID查找Thread列表
func (r *ThreadRepository) FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]*model.Thread, error) {
	query := `
		SELECT id, project_id, name, status, current_phase, current_agent, depth, workflow_template_id, abort_token, created_at, updated_at
		FROM threads WHERE project_id = ? ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, projectID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find threads: %w", err)
	}
	defer rows.Close()

	var threads = make([]*model.Thread, 0) // 初始化为空数组，避免 JSON null
	for rows.Next() {
		thread := &model.Thread{}
		var idStr, projIDStr string
		var projID sql.NullString
		var workflowTemplateID sql.NullString
		err := rows.Scan(
			&idStr, &projID, &thread.Name, &thread.Status, &thread.CurrentPhase, &thread.CurrentAgent,
			&thread.Depth, &workflowTemplateID, &thread.AbortToken, &thread.CreatedAt, &thread.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thread: %w", err)
		}
		thread.ID, _ = uuid.Parse(idStr)
		if projID.Valid {
			projIDStr = projID.String
			thread.ProjectID, _ = uuid.Parse(projIDStr)
		}
		if workflowTemplateID.Valid {
			wid, _ := uuid.Parse(workflowTemplateID.String)
			thread.WorkflowTemplateID = &wid
		}
		threads = append(threads, thread)
	}
	return threads, nil
}

// Update 更新Thread
func (r *ThreadRepository) Update(ctx context.Context, thread *model.Thread) error {
	query := `
		UPDATE threads
		SET status = ?, current_phase = ?, current_agent = ?, depth = ?, abort_token = ?, updated_at = ?
		WHERE id = ?
	`
	thread.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, query,
		thread.Status, thread.CurrentPhase, thread.CurrentAgent, thread.Depth, thread.AbortToken, thread.UpdatedAt, thread.ID.String(),
	)
	return err
}