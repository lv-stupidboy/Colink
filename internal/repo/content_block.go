package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// ContentBlockRepository 内容块持久化仓库
type ContentBlockRepository struct {
	BaseRepository
}

// NewContentBlockRepository 创建仓库
func NewContentBlockRepository(db *sql.DB, dbType DBType) *ContentBlockRepository {
	return &ContentBlockRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// getUpsertQuery 返回适合当前数据库的 Upsert SQL
func (r *ContentBlockRepository) getUpsertQuery() string {
	if r.DBType() == DBTypeMySQL {
		// MySQL 使用 INSERT ... ON DUPLICATE KEY UPDATE
		return `
			INSERT INTO invocation_content_blocks (id, invocation_id, type, content, tool_name, tool_id, input, output, is_error, status, timestamp, started_at, completed_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				type = VALUES(type),
				content = VALUES(content),
				tool_name = VALUES(tool_name),
				tool_id = VALUES(tool_id),
				input = VALUES(input),
				output = VALUES(output),
				is_error = VALUES(is_error),
				status = VALUES(status),
				timestamp = VALUES(timestamp),
				started_at = VALUES(started_at),
				completed_at = VALUES(completed_at),
				created_at = VALUES(created_at)
		`
	}
	// SQLite 使用 INSERT OR REPLACE
	return `
		INSERT OR REPLACE INTO invocation_content_blocks (id, invocation_id, type, content, tool_name, tool_id, input, output, is_error, status, timestamp, started_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
}

// Upsert 插入或更新单个内容块（幂等）
func (r *ContentBlockRepository) Upsert(ctx context.Context, block *model.InvocationContentBlock) error {
	now := time.Now()
	query := r.getUpsertQuery()

	var inputJSON []byte
	if block.Input != nil {
		inputJSON, _ = json.Marshal(block.Input)
	}

	_, err := r.DB().ExecContext(ctx, query,
		block.ID, block.InvocationID, block.Type, block.Content, block.ToolName, block.ToolID, inputJSON, block.Output, block.IsError, block.Status, block.Timestamp, block.StartedAt, block.CompletedAt, now,
	)
	return err
}

// BatchUpsert 批量插入或更新内容块
func (r *ContentBlockRepository) BatchUpsert(ctx context.Context, blocks []model.InvocationContentBlock) error {
	if len(blocks) == 0 {
		return nil
	}

	tx, err := r.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	query := r.getUpsertQuery()

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
			block.ID, block.InvocationID, block.Type, block.Content, block.ToolName, block.ToolID, inputJSON, block.Output, block.IsError, block.Status, block.Timestamp, block.StartedAt, block.CompletedAt, now,
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

	rows, err := r.DB().QueryContext(ctx, query, invocationID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []model.InvocationContentBlock
	for rows.Next() {
		var block model.InvocationContentBlock
		var inputJSON []byte
		var startedAt, completedAt sql.NullInt64
		err := rows.Scan(
			&block.ID, &block.InvocationID, &block.Type, &block.Content, &block.ToolName, &block.ToolID, &inputJSON, &block.Output, &block.IsError, &block.Status, &block.Timestamp, &startedAt, &completedAt,
		)
		if err != nil {
			return nil, err
		}
		if len(inputJSON) > 0 {
			json.Unmarshal(inputJSON, &block.Input)
		}
		if startedAt.Valid {
			block.StartedAt = startedAt.Int64
		}
		if completedAt.Valid {
			block.CompletedAt = completedAt.Int64
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
	err := r.DB().QueryRowContext(ctx, query, invocationID.String()).Scan(&result)
	return result, err
}

// DeleteByInvocation 删除某个 invocation 的所有内容块
func (r *ContentBlockRepository) DeleteByInvocation(ctx context.Context, invocationID uuid.UUID) error {
	query := `DELETE FROM invocation_content_blocks WHERE invocation_id = ?`
	_, err := r.DB().ExecContext(ctx, query, invocationID.String())
	return err
}

// DeleteOlderThan 删除指定天数之前的内容块（归档清理）
func (r *ContentBlockRepository) DeleteOlderThan(ctx context.Context, days int) error {
	// SQLite 使用 datetime 函数计算过期时间
	cutoffTime := time.Now().AddDate(0, 0, -days)
	query := `DELETE FROM invocation_content_blocks WHERE created_at < ?`
	_, err := r.DB().ExecContext(ctx, query, cutoffTime)
	return err
}