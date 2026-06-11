// internal/repo/session_record_repo.go
// Session 记录 Repository
// 用于 ACP 原生 session/resume
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// SessionRecordRepository Session 记录 Repository
type SessionRecordRepository interface {
	// 基本操作
	Create(ctx context.Context, record *model.SessionRecord) error
	Update(ctx context.Context, record *model.SessionRecord) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.SessionRecord, error)

	// 查询操作
	FindByThreadAndAgent(ctx context.Context, threadID, agentID string) (*model.SessionRecord, error)
	FindExpiredRecords(ctx context.Context, expiryDuration time.Duration) ([]*model.SessionRecord, error)

	// 批量操作
	DeleteExpiredRecords(ctx context.Context, expiryDuration time.Duration) error

	// 统计
	CountByThread(ctx context.Context, threadID string) (int, error)
	CountByAgentType(ctx context.Context, agentType model.BaseAgentType) (int, error)
}

// SessionRecordRepoImpl Session 记录 Repository 实现
type SessionRecordRepoImpl struct {
	db *sql.DB
}

// NewSessionRecordRepository 创建 Session 记录 Repository
func NewSessionRecordRepository(db *sql.DB) SessionRecordRepository {
	return &SessionRecordRepoImpl{db: db}
}

// Create 创建 Session 记录
func (r *SessionRecordRepoImpl) Create(ctx context.Context, record *model.SessionRecord) error {
	record.BeforeCreate()

	query := `
		INSERT INTO session_records (
			id, thread_id, agent_id, agent_type,
			acp_session_id, cli_session_id, resume_expiry,
			status, last_active_at,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		record.ID.String(),
		record.ThreadID.String(),
		record.AgentID.String(),
		record.AgentType,
		record.AcpSessionID,
		record.CliSessionID,
		record.ResumeExpiry,
		record.Status,
		record.LastActiveAt,
		record.CreatedAt,
		record.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create session record: %w", err)
	}

	return nil
}

// Update 更新 Session 记录
func (r *SessionRecordRepoImpl) Update(ctx context.Context, record *model.SessionRecord) error {
	record.BeforeUpdate()

	query := `
		UPDATE session_records SET
			acp_session_id = ?,
			cli_session_id = ?,
			resume_expiry = ?,
			status = ?,
			last_active_at = ?,
			updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		record.AcpSessionID,
		record.CliSessionID,
		record.ResumeExpiry,
		record.Status,
		record.LastActiveAt,
		record.UpdatedAt,
		record.ID.String(),
	)

	if err != nil {
		return fmt.Errorf("failed to update session record: %w", err)
	}

	return nil
}

// Delete 删除 Session 记录
func (r *SessionRecordRepoImpl) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM session_records WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete session record: %w", err)
	}
	return nil
}

// FindByID 根据 ID 查找
func (r *SessionRecordRepoImpl) FindByID(ctx context.Context, id uuid.UUID) (*model.SessionRecord, error) {
	query := `
		SELECT id, thread_id, agent_id, agent_type,
			acp_session_id, cli_session_id, resume_expiry,
			status, last_active_at,
			created_at, updated_at
		FROM session_records WHERE id = ?
	`

	row := r.db.QueryRowContext(ctx, query, id.String())
	return r.scanRecord(row)
}

// FindByThreadAndAgent 根据 Thread 和 Agent 查找
func (r *SessionRecordRepoImpl) FindByThreadAndAgent(ctx context.Context, threadID, agentID string) (*model.SessionRecord, error) {
	query := `
		SELECT id, thread_id, agent_id, agent_type,
			acp_session_id, cli_session_id, resume_expiry,
			status, last_active_at,
			created_at, updated_at
		FROM session_records
		WHERE thread_id = ? AND agent_id = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, threadID, agentID)
	return r.scanRecord(row)
}

// FindExpiredRecords 查找过期的记录
func (r *SessionRecordRepoImpl) FindExpiredRecords(ctx context.Context, expiryDuration time.Duration) ([]*model.SessionRecord, error) {
	expiryThreshold := time.Now().Add(-expiryDuration).Unix()

	query := `
		SELECT id, thread_id, agent_id, agent_type,
			acp_session_id, cli_session_id, resume_expiry,
			status, last_active_at,
			created_at, updated_at
		FROM session_records
		WHERE resume_expiry < ?
	`

	rows, err := r.db.QueryContext(ctx, query, expiryThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to find expired records: %w", err)
	}
	defer rows.Close()

	return r.scanRecords(rows)
}

// DeleteExpiredRecords 删除过期的记录
func (r *SessionRecordRepoImpl) DeleteExpiredRecords(ctx context.Context, expiryDuration time.Duration) error {
	expiryThreshold := time.Now().Add(-expiryDuration).Unix()

	query := `DELETE FROM session_records WHERE resume_expiry < ?`
	_, err := r.db.ExecContext(ctx, query, expiryThreshold)
	if err != nil {
		return fmt.Errorf("failed to delete expired records: %w", err)
	}
	return nil
}

// CountByThread 统计 Thread 的 Session 数量
func (r *SessionRecordRepoImpl) CountByThread(ctx context.Context, threadID string) (int, error) {
	query := `SELECT COUNT(*) FROM session_records WHERE thread_id = ?`
	row := r.db.QueryRowContext(ctx, query, threadID)

	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}
	return count, nil
}

// CountByAgentType 统计 AgentType 的 Session 数量
func (r *SessionRecordRepoImpl) CountByAgentType(ctx context.Context, agentType model.BaseAgentType) (int, error) {
	query := `SELECT COUNT(*) FROM session_records WHERE agent_type = ?`
	row := r.db.QueryRowContext(ctx, query, agentType)

	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions by agent type: %w", err)
	}
	return count, nil
}

// scanRecord 扫描单条记录
func (r *SessionRecordRepoImpl) scanRecord(row *sql.Row) (*model.SessionRecord, error) {
	var record model.SessionRecord
	var threadID, agentID, agentType string

	err := row.Scan(
		&record.ID, &threadID, &agentID, &agentType,
		&record.AcpSessionID, &record.CliSessionID, &record.ResumeExpiry,
		&record.Status, &record.LastActiveAt,
		&record.CreatedAt, &record.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan session record: %w", err)
	}

	record.ThreadID = uuid.MustParse(threadID)
	record.AgentID = uuid.MustParse(agentID)
	record.AgentType = model.BaseAgentType(agentType)

	return &record, nil
}

// scanRecords 扫描多条记录
func (r *SessionRecordRepoImpl) scanRecords(rows *sql.Rows) ([]*model.SessionRecord, error) {
	var records []*model.SessionRecord

	for rows.Next() {
		var record model.SessionRecord
		var threadID, agentID, agentType string

		err := rows.Scan(
			&record.ID, &threadID, &agentID, &agentType,
			&record.AcpSessionID, &record.CliSessionID, &record.ResumeExpiry,
			&record.Status, &record.LastActiveAt,
			&record.CreatedAt, &record.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan session record: %w", err)
		}

		record.ThreadID = uuid.MustParse(threadID)
		record.AgentID = uuid.MustParse(agentID)
		record.AgentType = model.BaseAgentType(agentType)

		records = append(records, &record)
	}

	return records, nil
}