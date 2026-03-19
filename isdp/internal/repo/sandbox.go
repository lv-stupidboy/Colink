package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// SandboxRepository 沙箱数据访问
type SandboxRepository struct {
	db *sql.DB
}

// NewSandboxRepository 创建Sandbox Repository
func NewSandboxRepository(db *sql.DB) *SandboxRepository {
	return &SandboxRepository{db: db}
}

// Create 创建沙箱
func (r *SandboxRepository) Create(ctx context.Context, sandbox *model.Sandbox) error {
	query := `
		INSERT INTO sandboxes (id, thread_id, name, image, status, container_id, port, created_at, ended_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		sandbox.ID.String(), sandbox.ThreadID.String(), sandbox.Name, sandbox.Image, sandbox.Status, sandbox.ContainerID, sandbox.Port, sandbox.CreatedAt, sandbox.EndedAt,
	)
	return err
}

// FindByID 根据ID查找
func (r *SandboxRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Sandbox, error) {
	query := `
		SELECT id, thread_id, name, image, status, container_id, port, created_at, ended_at
		FROM sandboxes WHERE id = ?
	`
	sandbox := &model.Sandbox{}
	var idStr, threadIDStr string
	var port sql.NullInt32
	var endedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &threadIDStr, &sandbox.Name, &sandbox.Image, &sandbox.Status, &sandbox.ContainerID, &port, &sandbox.CreatedAt, &endedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find sandbox: %w", err)
	}
	sandbox.ID, _ = uuid.Parse(idStr)
	sandbox.ThreadID, _ = uuid.Parse(threadIDStr)
	if port.Valid {
		sandbox.Port = int(port.Int32)
	}
	if endedAt.Valid {
		sandbox.EndedAt = &endedAt.Time
	}
	return sandbox, nil
}

// FindByThreadID 根据ThreadID查找
func (r *SandboxRepository) FindByThreadID(ctx context.Context, threadID uuid.UUID) ([]*model.Sandbox, error) {
	query := `
		SELECT id, thread_id, name, image, status, container_id, port, created_at, ended_at
		FROM sandboxes WHERE thread_id = ? ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, threadID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find sandboxes: %w", err)
	}
	defer rows.Close()

	var sandboxes = make([]*model.Sandbox, 0) // 初始化为空数组，避免 JSON null
	for rows.Next() {
		sandbox := &model.Sandbox{}
		var idStr, threadIDStr string
		var port sql.NullInt32
		var endedAt sql.NullTime
		err := rows.Scan(
			&idStr, &threadIDStr, &sandbox.Name, &sandbox.Image, &sandbox.Status, &sandbox.ContainerID, &port, &sandbox.CreatedAt, &endedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sandbox: %w", err)
		}
		sandbox.ID, _ = uuid.Parse(idStr)
		sandbox.ThreadID, _ = uuid.Parse(threadIDStr)
		if port.Valid {
			sandbox.Port = int(port.Int32)
		}
		if endedAt.Valid {
			sandbox.EndedAt = &endedAt.Time
		}
		sandboxes = append(sandboxes, sandbox)
	}
	return sandboxes, nil
}

// Update 更新沙箱
func (r *SandboxRepository) Update(ctx context.Context, sandbox *model.Sandbox) error {
	query := `
		UPDATE sandboxes
		SET status = ?, container_id = ?, port = ?, ended_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		sandbox.Status, sandbox.ContainerID, sandbox.Port, sandbox.EndedAt, sandbox.ID.String(),
	)
	return err
}

// Delete 删除沙箱
func (r *SandboxRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sandboxes WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}