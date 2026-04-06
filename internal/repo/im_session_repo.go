package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// IMSessionRepository IM会话数据访问
type IMSessionRepository struct {
	db *sql.DB
}

// NewIMSessionRepository 创建IMSession Repository
func NewIMSessionRepository(db *sql.DB) *IMSessionRepository {
	return &IMSessionRepository{db: db}
}

// Create 创建IMSession
func (r *IMSessionRepository) Create(ctx context.Context, session *model.IMSession) error {
	query := `
		INSERT INTO im_sessions (id, platform, chat_id, chat_type, thread_id, project_id, user_id, user_name, last_message_at, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	var lastMessageAt interface{}
	if session.LastMessageAt != nil {
		lastMessageAt = *session.LastMessageAt
	}
	_, err := r.db.ExecContext(ctx, query,
		session.ID.String(), session.Platform, session.ChatID, session.ChatType, session.ThreadID.String(), session.ProjectID.String(),
		session.UserID, session.UserName, lastMessageAt, session.IsActive, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create im_session: %w", err)
	}
	session.CreatedAt = now
	session.UpdatedAt = now
	return nil
}

// FindByID 根据ID查找IMSession
func (r *IMSessionRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.IMSession, error) {
	query := `
		SELECT id, platform, chat_id, chat_type, thread_id, project_id, user_id, user_name, last_message_at, is_active, created_at, updated_at
		FROM im_sessions WHERE id = ?
	`
	session := &model.IMSession{}
	var idStr, threadIDStr, projectIDStr string
	var lastMessageAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &session.Platform, &session.ChatID, &session.ChatType, &threadIDStr, &projectIDStr,
		&session.UserID, &session.UserName, &lastMessageAt, &session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find im_session: %w", err)
	}
	session.ID, _ = uuid.Parse(idStr)
	session.ThreadID, _ = uuid.Parse(threadIDStr)
	session.ProjectID, _ = uuid.Parse(projectIDStr)
	if lastMessageAt.Valid {
		session.LastMessageAt = &lastMessageAt.Time
	}
	return session, nil
}

// FindByChatID 根据平台和ChatID查找IMSession
func (r *IMSessionRepository) FindByChatID(ctx context.Context, platform string, chatID string) (*model.IMSession, error) {
	query := `
		SELECT id, platform, chat_id, chat_type, thread_id, project_id, user_id, user_name, last_message_at, is_active, created_at, updated_at
		FROM im_sessions WHERE platform = ? AND chat_id = ?
	`
	session := &model.IMSession{}
	var idStr, threadIDStr, projectIDStr string
	var lastMessageAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, platform, chatID).Scan(
		&idStr, &session.Platform, &session.ChatID, &session.ChatType, &threadIDStr, &projectIDStr,
		&session.UserID, &session.UserName, &lastMessageAt, &session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find im_session by chat_id: %w", err)
	}
	session.ID, _ = uuid.Parse(idStr)
	session.ThreadID, _ = uuid.Parse(threadIDStr)
	session.ProjectID, _ = uuid.Parse(projectIDStr)
	if lastMessageAt.Valid {
		session.LastMessageAt = &lastMessageAt.Time
	}
	return session, nil
}

// FindByThreadID 根据ThreadID查找IMSession
func (r *IMSessionRepository) FindByThreadID(ctx context.Context, threadID uuid.UUID) (*model.IMSession, error) {
	query := `
		SELECT id, platform, chat_id, chat_type, thread_id, project_id, user_id, user_name, last_message_at, is_active, created_at, updated_at
		FROM im_sessions WHERE thread_id = ?
	`
	session := &model.IMSession{}
	var idStr, threadIDStr, projectIDStr string
	var lastMessageAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, threadID.String()).Scan(
		&idStr, &session.Platform, &session.ChatID, &session.ChatType, &threadIDStr, &projectIDStr,
		&session.UserID, &session.UserName, &lastMessageAt, &session.IsActive, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find im_session by thread_id: %w", err)
	}
	session.ID, _ = uuid.Parse(idStr)
	session.ThreadID, _ = uuid.Parse(threadIDStr)
	session.ProjectID, _ = uuid.Parse(projectIDStr)
	if lastMessageAt.Valid {
		session.LastMessageAt = &lastMessageAt.Time
	}
	return session, nil
}

// FindActiveByPlatform 根据平台查找活跃IMSession列表
func (r *IMSessionRepository) FindActiveByPlatform(ctx context.Context, platform string) ([]*model.IMSession, error) {
	query := `
		SELECT id, platform, chat_id, chat_type, thread_id, project_id, user_id, user_name, last_message_at, is_active, created_at, updated_at
		FROM im_sessions WHERE platform = ? AND is_active = true ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, platform)
	if err != nil {
		return nil, fmt.Errorf("failed to find active im_sessions: %w", err)
	}
	defer rows.Close()

	var sessions = make([]*model.IMSession, 0) // 初始化为空数组，避免 JSON null
	for rows.Next() {
		session := &model.IMSession{}
		var idStr, threadIDStr, projectIDStr string
		var lastMessageAt sql.NullTime
		err := rows.Scan(
			&idStr, &session.Platform, &session.ChatID, &session.ChatType, &threadIDStr, &projectIDStr,
			&session.UserID, &session.UserName, &lastMessageAt, &session.IsActive, &session.CreatedAt, &session.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan im_session: %w", err)
		}
		session.ID, _ = uuid.Parse(idStr)
		session.ThreadID, _ = uuid.Parse(threadIDStr)
		session.ProjectID, _ = uuid.Parse(projectIDStr)
		if lastMessageAt.Valid {
			session.LastMessageAt = &lastMessageAt.Time
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// UpdateLastMessageAt 更新最后消息时间
func (r *IMSessionRepository) UpdateLastMessageAt(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE im_sessions
		SET last_message_at = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, now, now, id.String())
	if err != nil {
		return fmt.Errorf("failed to update last_message_at: %w", err)
	}
	return nil
}

// Deactivate 停用IMSession
func (r *IMSessionRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE im_sessions
		SET is_active = false, updated_at = ?
		WHERE id = ?
	`
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, now, id.String())
	if err != nil {
		return fmt.Errorf("failed to deactivate im_session: %w", err)
	}
	return nil
}
