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
	BaseRepository
}

// NewThreadRepository 创建Thread Repository
func NewThreadRepository(db *sql.DB, dbType DBType) *ThreadRepository {
	return &ThreadRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
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
	_, err := r.DB().ExecContext(ctx, query,
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
	var idStr string
	var projectID sql.NullString
	var workflowTemplateID sql.NullString
	var createdAt, updatedAt SQLiteTimeScanner
	err := r.DB().QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &projectID, &thread.Name, &thread.Status, &thread.CurrentPhase, &thread.CurrentAgent,
		&thread.Depth, &workflowTemplateID, &thread.AbortToken, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find thread: %w", err)
	}
	thread.ID, _ = uuid.Parse(idStr)
	if projectID.Valid {
		thread.ProjectID, _ = uuid.Parse(projectID.String)
	}
	if workflowTemplateID.Valid {
		wid, _ := uuid.Parse(workflowTemplateID.String)
		thread.WorkflowTemplateID = &wid
	}
	thread.CreatedAt = createdAt.Time
	thread.UpdatedAt = updatedAt.Time
	return thread, nil
}

// FindByProjectID 根据项目ID查找Thread列表
func (r *ThreadRepository) FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]*model.Thread, error) {
	query := `
		SELECT id, project_id, name, status, current_phase, current_agent, depth, workflow_template_id, abort_token, created_at, updated_at
		FROM threads WHERE project_id = ? ORDER BY created_at DESC
	`
	rows, err := r.DB().QueryContext(ctx, query, projectID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find threads: %w", err)
	}
	defer rows.Close()

	var threads = make([]*model.Thread, 0) // 初始化为空数组，避免 JSON null
	for rows.Next() {
		thread := &model.Thread{}
		var idStr string
		var projID sql.NullString
		var workflowTemplateID sql.NullString
		var createdAt, updatedAt SQLiteTimeScanner
		err := rows.Scan(
			&idStr, &projID, &thread.Name, &thread.Status, &thread.CurrentPhase, &thread.CurrentAgent,
			&thread.Depth, &workflowTemplateID, &thread.AbortToken, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thread: %w", err)
		}
		thread.ID, _ = uuid.Parse(idStr)
		if projID.Valid {
			thread.ProjectID, _ = uuid.Parse(projID.String)
		}
		if workflowTemplateID.Valid {
			wid, _ := uuid.Parse(workflowTemplateID.String)
			thread.WorkflowTemplateID = &wid
		}
		thread.CreatedAt = createdAt.Time
		thread.UpdatedAt = updatedAt.Time
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
	_, err := r.DB().ExecContext(ctx, query,
		thread.Status, thread.CurrentPhase, thread.CurrentAgent, thread.Depth, thread.AbortToken, thread.UpdatedAt, thread.ID.String(),
	)
	return err
}

// Delete 删除Thread（连同相关消息）
func (r *ThreadRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// 先删除关联的消息
	deleteMessagesQuery := `DELETE FROM messages WHERE thread_id = ?`
	_, err := r.DB().ExecContext(ctx, deleteMessagesQuery, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete thread messages: %w", err)
	}

	// 删除 Thread
	query := `DELETE FROM threads WHERE id = ?`
	_, err = r.DB().ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete thread: %w", err)
	}
	return nil
}