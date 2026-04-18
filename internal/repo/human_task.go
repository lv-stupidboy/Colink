package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// HumanTaskRepository 人工任务数据访问
type HumanTaskRepository struct {
	BaseRepository
}

// NewHumanTaskRepository 创建HumanTask Repository
func NewHumanTaskRepository(db *sql.DB, dbType DBType) *HumanTaskRepository {
	return &HumanTaskRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建人工任务
func (r *HumanTaskRepository) Create(ctx context.Context, task *model.HumanTask) error {
	query := `
		INSERT INTO human_tasks (
			id, thread_id, invocation_id, agent_config_id, agent_name,
			wait_reason, status, created_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var completedAt interface{}
	if task.CompletedAt != nil {
		completedAt = task.CompletedAt.Format("2006-01-02 15:04:05")
	}

	_, err := r.DB().ExecContext(ctx, query,
		task.ID.String(),
		task.ThreadID.String(),
		task.InvocationID.String(),
		task.AgentConfigID.String(),
		task.AgentName,
		task.WaitReason,
		task.Status,
		task.CreatedAt.Format("2006-01-02 15:04:05"),
		completedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create human task: %w", err)
	}
	return nil
}

// FindByID 根据ID查找人工任务
func (r *HumanTaskRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.HumanTask, error) {
	query := `
		SELECT id, thread_id, invocation_id, agent_config_id, agent_name,
			wait_reason, status, created_at, completed_at
		FROM human_tasks WHERE id = ?
	`

	task := &model.HumanTask{}
	var idStr, threadIDStr, invocationIDStr, agentConfigIDStr string
	var agentName, waitReason sql.NullString
	var createdAt SQLiteTimeScanner
	var completedAt sql.NullString

	err := r.DB().QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &threadIDStr, &invocationIDStr, &agentConfigIDStr,
		&agentName, &waitReason, &task.Status,
		&createdAt, &completedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("human task not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find human task: %w", err)
	}

	task.ID, _ = uuid.Parse(idStr)
	task.ThreadID, _ = uuid.Parse(threadIDStr)
	task.InvocationID, _ = uuid.Parse(invocationIDStr)
	task.AgentConfigID, _ = uuid.Parse(agentConfigIDStr)

	if agentName.Valid {
		task.AgentName = agentName.String
	}
	if waitReason.Valid {
		task.WaitReason = waitReason.String
	}

	task.CreatedAt = createdAt.Time

	if completedAt.Valid {
		t := parseSQLiteTime(completedAt.String)
		if !t.IsZero() {
			task.CompletedAt = &t
		}
	}

	return task, nil
}

// FindByInvocation 根据 invocation_id 查找 pending 状态的任务
func (r *HumanTaskRepository) FindByInvocation(ctx context.Context, invocationID uuid.UUID) (*model.HumanTask, error) {
	query := `
		SELECT id, thread_id, invocation_id, agent_config_id, agent_name,
			wait_reason, status, created_at, completed_at
		FROM human_tasks
		WHERE invocation_id = ? AND status = 'pending'
		LIMIT 1
	`

	task := &model.HumanTask{}
	var idStr, threadIDStr, invocationIDStr, agentConfigIDStr string
	var agentName, waitReason sql.NullString
	var createdAt SQLiteTimeScanner
	var completedAt sql.NullString

	err := r.DB().QueryRowContext(ctx, query, invocationID.String()).Scan(
		&idStr, &threadIDStr, &invocationIDStr, &agentConfigIDStr,
		&agentName, &waitReason, &task.Status,
		&createdAt, &completedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 无记录返回 nil
		}
		return nil, fmt.Errorf("failed to find human task: %w", err)
	}

	task.ID, _ = uuid.Parse(idStr)
	task.ThreadID, _ = uuid.Parse(threadIDStr)
	task.InvocationID, _ = uuid.Parse(invocationIDStr)
	task.AgentConfigID, _ = uuid.Parse(agentConfigIDStr)

	if agentName.Valid {
		task.AgentName = agentName.String
	}
	if waitReason.Valid {
		task.WaitReason = waitReason.String
	}

	task.CreatedAt = createdAt.Time

	if completedAt.Valid {
		t := parseSQLiteTime(completedAt.String)
		if !t.IsZero() {
			task.CompletedAt = &t
		}
	}

	return task, nil
}

// ListByThread 根据ThreadID列出人工任务
func (r *HumanTaskRepository) ListByThread(ctx context.Context, threadID uuid.UUID) ([]*model.HumanTask, error) {
	query := `
		SELECT id, thread_id, invocation_id, agent_config_id, agent_name,
			wait_reason, status, created_at, completed_at
		FROM human_tasks WHERE thread_id = ? ORDER BY created_at DESC
	`

	rows, err := r.DB().QueryContext(ctx, query, threadID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to list human tasks by thread: %w", err)
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// ListByStatus 根据状态列出人工任务
func (r *HumanTaskRepository) ListByStatus(ctx context.Context, status model.HumanTaskStatus) ([]*model.HumanTask, error) {
	query := `
		SELECT id, thread_id, invocation_id, agent_config_id, agent_name,
			wait_reason, status, created_at, completed_at
		FROM human_tasks WHERE status = ? ORDER BY created_at DESC
	`

	rows, err := r.DB().QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list human tasks by status: %w", err)
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// Update 更新人工任务
func (r *HumanTaskRepository) Update(ctx context.Context, task *model.HumanTask) error {
	query := `
		UPDATE human_tasks
		SET agent_name = ?, wait_reason = ?, status = ?, completed_at = ?
		WHERE id = ?
	`

	var completedAt interface{}
	if task.CompletedAt != nil {
		completedAt = task.CompletedAt.Format("2006-01-02 15:04:05")
	}

	_, err := r.DB().ExecContext(ctx, query,
		task.AgentName,
		task.WaitReason,
		task.Status,
		completedAt,
		task.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update human task: %w", err)
	}
	return nil
}

// CompleteByInvocation 根据 invocation_id 完成 pending 任务
func (r *HumanTaskRepository) CompleteByInvocation(ctx context.Context, invocationID uuid.UUID) error {
	now := time.Now()
	query := `
		UPDATE human_tasks
		SET status = 'completed', completed_at = ?
		WHERE invocation_id = ? AND status = 'pending'
	`
	_, err := r.DB().ExecContext(ctx, query,
		now.Format("2006-01-02 15:04:05"),
		invocationID.String())
	if err != nil {
		return fmt.Errorf("failed to complete human task: %w", err)
	}
	return nil
}

// CountByStatus 统计各状态任务数量
func (r *HumanTaskRepository) CountByStatus(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM human_tasks
		GROUP BY status
	`

	rows, err := r.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count human tasks: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}

	return counts, nil
}

// Delete 删除人工任务
func (r *HumanTaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM human_tasks WHERE id = ?`
	_, err := r.DB().ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete human task: %w", err)
	}
	return nil
}

// scanTasks 扫描多行数据
func (r *HumanTaskRepository) scanTasks(rows *sql.Rows) ([]*model.HumanTask, error) {
	tasks := make([]*model.HumanTask, 0)

	for rows.Next() {
		task := &model.HumanTask{}
		var idStr, threadIDStr, invocationIDStr, agentConfigIDStr string
		var agentName, waitReason sql.NullString
		var createdAt SQLiteTimeScanner
		var completedAt sql.NullString

		err := rows.Scan(
			&idStr, &threadIDStr, &invocationIDStr, &agentConfigIDStr,
			&agentName, &waitReason, &task.Status,
			&createdAt, &completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan human task: %w", err)
		}

		task.ID, _ = uuid.Parse(idStr)
		task.ThreadID, _ = uuid.Parse(threadIDStr)
		task.InvocationID, _ = uuid.Parse(invocationIDStr)
		task.AgentConfigID, _ = uuid.Parse(agentConfigIDStr)

		if agentName.Valid {
			task.AgentName = agentName.String
		}
		if waitReason.Valid {
			task.WaitReason = waitReason.String
		}

		task.CreatedAt = createdAt.Time

		if completedAt.Valid {
			t := parseSQLiteTime(completedAt.String)
			if !t.IsZero() {
				task.CompletedAt = &t
			}
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}