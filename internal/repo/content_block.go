package repo

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// ContentBlockRepository 内容块持久化仓库
type ContentBlockRepository struct {
	db *sql.DB
}

// NewContentBlockRepository 创建仓库
func NewContentBlockRepository(db *sql.DB) *ContentBlockRepository {
	return &ContentBlockRepository{db: db}
}

// Upsert 插入或更新单个内容块（幂等）
func (r *ContentBlockRepository) Upsert(ctx context.Context, block *model.InvocationContentBlock) error {
	query := `
		INSERT INTO invocation_content_blocks (id, invocation_id, type, content, tool_name, tool_id, input, output, is_error, status, timestamp, started_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE
			content = VALUES(content),
			status = VALUES(status),
			output = VALUES(output),
			is_error = VALUES(is_error),
			completed_at = VALUES(completed_at)
	`

	var inputJSON []byte
	if block.Input != nil {
		inputJSON, _ = json.Marshal(block.Input)
	}

	_, err := r.db.ExecContext(ctx, query,
		block.ID, block.InvocationID, block.Type, block.Content, block.ToolName, block.ToolID, inputJSON, block.Output, block.IsError, block.Status, block.Timestamp, block.StartedAt, block.CompletedAt,
	)
	return err
}

// BatchUpsert 批量插入或更新内容块
func (r *ContentBlockRepository) BatchUpsert(ctx context.Context, blocks []model.InvocationContentBlock) error {
	if len(blocks) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO invocation_content_blocks (id, invocation_id, type, content, tool_name, tool_id, input, output, is_error, status, timestamp, started_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE
			content = VALUES(content),
			status = VALUES(status),
			output = VALUES(output),
			is_error = VALUES(is_error),
			completed_at = VALUES(completed_at)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, block := range blocks {
		var inputJSON []byte
		if block.Input != nil {
			inputJSON, _ = json.Marshal(block.Input)
		}

		_, err := stmt.ExecContext(ctx,
			block.ID, block.InvocationID, block.Type, block.Content, block.ToolName, block.ToolID, inputJSON, block.Output, block.IsError, block.Status, block.Timestamp, block.StartedAt, block.CompletedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// FindByInvocation 查找某个 invocation 的所有内容块
func (r *ContentBlockRepository) FindByInvocation(ctx context.Context, invocationID uuid.UUID) ([]model.InvocationContentBlock, error) {
	query := `
		SELECT id, invocation_id, type, content, tool_name, tool_id, input, output, is_error, status, timestamp, started_at, completed_at
		FROM invocation_content_blocks
		WHERE invocation_id = ?
		ORDER BY timestamp ASC
	`

	rows, err := r.db.QueryContext(ctx, query, invocationID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []model.InvocationContentBlock
	for rows.Next() {
		var block model.InvocationContentBlock
		var inputJSON []byte
		err := rows.Scan(
			&block.ID, &block.InvocationID, &block.Type, &block.Content, &block.ToolName, &block.ToolID, &inputJSON, &block.Output, &block.IsError, &block.Status, &block.Timestamp, &block.StartedAt, &block.CompletedAt,
		)
		if err != nil {
			return nil, err
		}
		if len(inputJSON) > 0 {
			json.Unmarshal(inputJSON, &block.Input)
		}
		blocks = append(blocks, block)
	}

	return blocks, rows.Err()
}

// FindByInvocationRaw 查找并返回 JSON 原始数据（用于 WebSocket 推送）
func (r *ContentBlockRepository) FindByInvocationRaw(ctx context.Context, invocationID uuid.UUID) (json.RawMessage, error) {
	query := `
		SELECT COALESCE(
			(SELECT JSON_ARRAYAGG(
				JSON_OBJECT(
					'id', id,
					'type', type,
					'content', content,
					'toolName', tool_name,
					'toolId', tool_id,
					'input', input,
					'output', output,
					'isError', is_error,
					'status', status,
					'timestamp', timestamp,
					'startedAt', started_at,
					'completedAt', completed_at
				)
			)
			FROM invocation_content_blocks
			WHERE invocation_id = ?
			ORDER BY timestamp ASC),
			'[]'
		)
	`

	var result json.RawMessage
	err := r.db.QueryRowContext(ctx, query, invocationID.String()).Scan(&result)
	return result, err
}

// DeleteByInvocation 删除某个 invocation 的所有内容块
func (r *ContentBlockRepository) DeleteByInvocation(ctx context.Context, invocationID uuid.UUID) error {
	query := `DELETE FROM invocation_content_blocks WHERE invocation_id = ?`
	_, err := r.db.ExecContext(ctx, query, invocationID.String())
	return err
}

// DeleteOlderThan 删除指定天数之前的内容块（归档清理）
func (r *ContentBlockRepository) DeleteOlderThan(ctx context.Context, days int) error {
	query := `DELETE FROM invocation_content_blocks WHERE created_at < DATE_SUB(NOW(), INTERVAL ? DAY)`
	_, err := r.db.ExecContext(ctx, query, days)
	return err
}