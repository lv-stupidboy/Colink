package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// MessageRepository Message数据访问
type MessageRepository struct {
	db *sql.DB
}

// NewMessageRepository 创建Message Repository
func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Create 创建消息
func (r *MessageRepository) Create(ctx context.Context, msg *model.Message) error {
	query := `
		INSERT INTO messages (id, thread_id, role, agent_id, content, message_type, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	msg.ID = uuid.New()
	msg.CreatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, query,
		msg.ID.String(), msg.ThreadID.String(), msg.Role, msg.AgentID, msg.Content, msg.MessageType, msg.Metadata, msg.CreatedAt,
	)
	return err
}

// FindByThreadID 根据ThreadID查找消息
func (r *MessageRepository) FindByThreadID(ctx context.Context, threadID uuid.UUID, limit int) ([]*model.Message, error) {
	query := `
		SELECT id, thread_id, role, agent_id, content, message_type, metadata, created_at
		FROM messages WHERE thread_id = ? ORDER BY created_at ASC LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, threadID.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*model.Message
	for rows.Next() {
		msg := &model.Message{}
		var idStr, threadIDStr string
		var metadata []byte
		err := rows.Scan(
			&idStr, &threadIDStr, &msg.Role, &msg.AgentID, &msg.Content, &msg.MessageType, &metadata, &msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		msg.ID, _ = uuid.Parse(idStr)
		msg.ThreadID, _ = uuid.Parse(threadIDStr)
		msg.Metadata = json.RawMessage(metadata)
		messages = append(messages, msg)
	}
	return messages, nil
}

// GetRecent 获取最近消息
func (r *MessageRepository) GetRecent(ctx context.Context, threadID uuid.UUID, limit int) ([]*model.Message, error) {
	query := `
		SELECT id, thread_id, role, agent_id, content, message_type, metadata, created_at
		FROM messages WHERE thread_id = ? ORDER BY created_at DESC LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, threadID.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*model.Message
	for rows.Next() {
		msg := &model.Message{}
		var idStr, threadIDStr string
		var metadata []byte
		err := rows.Scan(
			&idStr, &threadIDStr, &msg.Role, &msg.AgentID, &msg.Content, &msg.MessageType, &metadata, &msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		msg.ID, _ = uuid.Parse(idStr)
		msg.ThreadID, _ = uuid.Parse(threadIDStr)
		msg.Metadata = json.RawMessage(metadata)
		messages = append(messages, msg)
	}
	return messages, nil
}