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
			id, thread_id, role_config_id, role_name, task_type, task_content,
			expected_output, source_agent_id, source_agent_name, status,
			submitted_at, submitted_by, output_content, output_files, target_agent_id,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	outputFiles, _ := json.Marshal(task.OutputFiles)

	var sourceAgentID, targetAgentID interface{}
	if task.SourceAgentID != uuid.Nil {
		sourceAgentID = task.SourceAgentID.String()
	}
	if task.TargetAgentID != uuid.Nil {
		targetAgentID = task.TargetAgentID.String()
	}

	_, err := r.DB().ExecContext(ctx, query,
		task.ID.String(),
		task.ThreadID.String(),
		task.RoleConfigID.String(),
		task.RoleName,
		task.TaskType,
		task.TaskContent,
		task.ExpectedOutput,
		sourceAgentID,
		task.SourceAgentName,
		task.Status,
		task.SubmittedAt,
		task.SubmittedBy,
		task.OutputContent,
		outputFiles,
		targetAgentID,
		task.CreatedAt,
		task.UpdatedAt,
	)
	return err
}

// FindByID 根据ID查找人工任务
func (r *HumanTaskRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.HumanTask, error) {
	query := `
		SELECT id, thread_id, role_config_id, role_name, task_type, task_content,
			expected_output, source_agent_id, source_agent_name, status,
			submitted_at, submitted_by, output_content, output_files, target_agent_id,
			created_at, updated_at
		FROM human_tasks WHERE id = ?
	`

	task := &model.HumanTask{}
	var idStr, threadIDStr, roleConfigIDStr string
	var sourceAgentID, targetAgentID, expectedOutput, sourceAgentName, submittedBy, outputContent sql.NullString
	var submittedAt sql.NullString
	var outputFiles []byte
	var createdAt, updatedAt SQLiteTimeScanner

	err := r.DB().QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &threadIDStr, &roleConfigIDStr, &task.RoleName, &task.TaskType, &task.TaskContent,
		&expectedOutput, &sourceAgentID, &sourceAgentName, &task.Status,
		&submittedAt, &submittedBy, &outputContent, &outputFiles, &targetAgentID,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find human task: %w", err)
	}

	task.ID, _ = uuid.Parse(idStr)
	task.ThreadID, _ = uuid.Parse(threadIDStr)
	task.RoleConfigID, _ = uuid.Parse(roleConfigIDStr)

	if expectedOutput.Valid {
		task.ExpectedOutput = expectedOutput.String
	}
	if sourceAgentID.Valid {
		task.SourceAgentID, _ = uuid.Parse(sourceAgentID.String)
	}
	if sourceAgentName.Valid {
		task.SourceAgentName = sourceAgentName.String
	}
	if submittedAt.Valid {
		t := parseSQLiteTime(submittedAt.String)
		if !t.IsZero() {
			task.SubmittedAt = &t
		}
	}
	if submittedBy.Valid {
		task.SubmittedBy = submittedBy.String
	}
	if outputContent.Valid {
		task.OutputContent = outputContent.String
	}
	if len(outputFiles) > 0 {
		json.Unmarshal(outputFiles, &task.OutputFiles)
	}
	if targetAgentID.Valid {
		task.TargetAgentID, _ = uuid.Parse(targetAgentID.String)
	}

	task.CreatedAt = createdAt.Time
	task.UpdatedAt = updatedAt.Time

	return task, nil
}

// ListByThread 根据ThreadID列出人工任务
func (r *HumanTaskRepository) ListByThread(ctx context.Context, threadID uuid.UUID) ([]*model.HumanTask, error) {
	query := `
		SELECT id, thread_id, role_config_id, role_name, task_type, task_content,
			expected_output, source_agent_id, source_agent_name, status,
			submitted_at, submitted_by, output_content, output_files, target_agent_id,
			created_at, updated_at
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
		SELECT id, thread_id, role_config_id, role_name, task_type, task_content,
			expected_output, source_agent_id, source_agent_name, status,
			submitted_at, submitted_by, output_content, output_files, target_agent_id,
			created_at, updated_at
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
		SET role_name = ?, task_type = ?, task_content = ?, expected_output = ?,
			source_agent_id = ?, source_agent_name = ?, status = ?,
			submitted_at = ?, submitted_by = ?, output_content = ?, output_files = ?,
			target_agent_id = ?, updated_at = ?
		WHERE id = ?
	`

	outputFiles, _ := json.Marshal(task.OutputFiles)

	var sourceAgentID, targetAgentID interface{}
	if task.SourceAgentID != uuid.Nil {
		sourceAgentID = task.SourceAgentID.String()
	}
	if task.TargetAgentID != uuid.Nil {
		targetAgentID = task.TargetAgentID.String()
	}

	task.UpdatedAt = time.Now()

	_, err := r.DB().ExecContext(ctx, query,
		task.RoleName,
		task.TaskType,
		task.TaskContent,
		task.ExpectedOutput,
		sourceAgentID,
		task.SourceAgentName,
		task.Status,
		task.SubmittedAt,
		task.SubmittedBy,
		task.OutputContent,
		outputFiles,
		targetAgentID,
		task.UpdatedAt,
		task.ID.String(),
	)
	return err
}

// scanTasks 扫描多行数据
func (r *HumanTaskRepository) scanTasks(rows *sql.Rows) ([]*model.HumanTask, error) {
	tasks := make([]*model.HumanTask, 0)

	for rows.Next() {
		task := &model.HumanTask{}
		var idStr, threadIDStr, roleConfigIDStr string
		var sourceAgentID, targetAgentID, expectedOutput, sourceAgentName, submittedBy, outputContent sql.NullString
		var submittedAt sql.NullString
		var outputFiles []byte
		var createdAt, updatedAt SQLiteTimeScanner

		err := rows.Scan(
			&idStr, &threadIDStr, &roleConfigIDStr, &task.RoleName, &task.TaskType, &task.TaskContent,
			&expectedOutput, &sourceAgentID, &sourceAgentName, &task.Status,
			&submittedAt, &submittedBy, &outputContent, &outputFiles, &targetAgentID,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan human task: %w", err)
		}

		task.ID, _ = uuid.Parse(idStr)
		task.ThreadID, _ = uuid.Parse(threadIDStr)
		task.RoleConfigID, _ = uuid.Parse(roleConfigIDStr)

		if expectedOutput.Valid {
			task.ExpectedOutput = expectedOutput.String
		}
		if sourceAgentID.Valid {
			task.SourceAgentID, _ = uuid.Parse(sourceAgentID.String)
		}
		if sourceAgentName.Valid {
			task.SourceAgentName = sourceAgentName.String
		}
		if submittedAt.Valid {
			t := parseSQLiteTime(submittedAt.String)
			if !t.IsZero() {
				task.SubmittedAt = &t
			}
		}
		if submittedBy.Valid {
			task.SubmittedBy = submittedBy.String
		}
		if outputContent.Valid {
			task.OutputContent = outputContent.String
		}
		if len(outputFiles) > 0 {
			json.Unmarshal(outputFiles, &task.OutputFiles)
		}
		if targetAgentID.Valid {
			task.TargetAgentID, _ = uuid.Parse(targetAgentID.String)
		}

		task.CreatedAt = createdAt.Time
		task.UpdatedAt = updatedAt.Time

		tasks = append(tasks, task)
	}

	return tasks, nil
}