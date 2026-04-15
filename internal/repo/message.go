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

// MessageRepository Message数据访问
type MessageRepository struct {
	BaseRepository
}

// NewMessageRepository 创建Message Repository
func NewMessageRepository(db *sql.DB, dbType DBType) *MessageRepository {
	return &MessageRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建消息
func (r *MessageRepository) Create(ctx context.Context, msg *model.Message) error {
	query := `
		INSERT INTO messages (id, thread_id, role, agent_id, content, content_blocks, message_type, metadata, created_at, mentions, origin, reply_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	msg.ID = uuid.New()
	msg.CreatedAt = time.Now()

	var replyTo interface{}
	if msg.ReplyTo != nil {
		replyTo = msg.ReplyTo.String()
	}

	_, err := r.DB().ExecContext(ctx, query,
		msg.ID.String(), msg.ThreadID.String(), msg.Role, msg.AgentID, msg.Content, msg.ContentBlocks, msg.MessageType, msg.Metadata, msg.CreatedAt,
		serializeStrings(msg.Mentions), msg.Origin, replyTo,
	)
	return err
}

// FindByThreadID 根据ThreadID查找消息
func (r *MessageRepository) FindByThreadID(ctx context.Context, threadID uuid.UUID, limit int) ([]*model.Message, error) {
	query := `
		SELECT id, thread_id, role, agent_id, content, content_blocks, message_type, metadata, created_at, mentions, origin, reply_to
		FROM messages WHERE thread_id = ? ORDER BY created_at ASC LIMIT ?
	`
	rows, err := r.DB().QueryContext(ctx, query, threadID.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages = make([]*model.Message, 0) // 初始化为空数组，避免 JSON null
	for rows.Next() {
		msg, err := r.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// GetRecent 获取最近消息
func (r *MessageRepository) GetRecent(ctx context.Context, threadID uuid.UUID, limit int) ([]*model.Message, error) {
	query := `
		SELECT id, thread_id, role, agent_id, content, content_blocks, message_type, metadata, created_at, mentions, origin, reply_to
		FROM messages WHERE thread_id = ? ORDER BY created_at DESC LIMIT ?
	`
	rows, err := r.DB().QueryContext(ctx, query, threadID.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages = make([]*model.Message, 0) // 初始化为空数组，避免 JSON null
	for rows.Next() {
		msg, err := r.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// FindMentionsForAgent 获取指定 Agent 被 @mention 的消息
func (r *MessageRepository) FindMentionsForAgent(ctx context.Context, threadID uuid.UUID, catID string, limit int) ([]*model.Message, error) {
	// 使用 JSON 数组查询：检查 mentions 数组中是否包含 catID
	// SQLite/MySQL JSON 函数: JSON_CONTAINS 或 LIKE
	query := `
		SELECT id, thread_id, role, agent_id, content, content_blocks, message_type, metadata, created_at, mentions, origin, reply_to
		FROM messages
		WHERE thread_id = ? AND mentions LIKE ?
		ORDER BY created_at DESC
		LIMIT ?
	`
	pattern := fmt.Sprintf("%%\"%s\"%%", catID)
	rows, err := r.DB().QueryContext(ctx, query, threadID.String(), pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages = make([]*model.Message, 0)
	for rows.Next() {
		msg, err := r.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// GetByID 根据ID获取消息
func (r *MessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Message, error) {
	query := `
		SELECT id, thread_id, role, agent_id, content, content_blocks, message_type, metadata, created_at, mentions, origin, reply_to
		FROM messages WHERE id = ?
	`
	row := r.DB().QueryRowContext(ctx, query, id.String())
	return r.scanMessageRow(row)
}

// scanMessage 扫描消息行
func (r *MessageRepository) scanMessage(rows *sql.Rows) (*model.Message, error) {
	msg := &model.Message{}
	var idStr, threadIDStr string
	var contentBlocks []byte
	var metadata []byte
	var mentionsJSON []byte
	var origin sql.NullString
	var replyTo sql.NullString
	var createdAt SQLiteTimeScanner

	err := rows.Scan(
		&idStr, &threadIDStr, &msg.Role, &msg.AgentID, &msg.Content, &contentBlocks, &msg.MessageType, &metadata, &createdAt,
		&mentionsJSON, &origin, &replyTo,
	)
	if err != nil {
		return nil, err
	}

	msg.ID, _ = uuid.Parse(idStr)
	msg.ThreadID, _ = uuid.Parse(threadIDStr)
	msg.ContentBlocks = json.RawMessage(contentBlocks)
	msg.Metadata = json.RawMessage(metadata)
	msg.Mentions = deserializeStrings(mentionsJSON)
	msg.Origin = origin.String
	msg.CreatedAt = createdAt.Time
	if replyTo.Valid {
		replyToID, _ := uuid.Parse(replyTo.String)
		msg.ReplyTo = &replyToID
	}

	return msg, nil
}

// scanMessageRow 扫描消息行（单行）
func (r *MessageRepository) scanMessageRow(row *sql.Row) (*model.Message, error) {
	msg := &model.Message{}
	var idStr, threadIDStr string
	var contentBlocks []byte
	var metadata []byte
	var mentionsJSON []byte
	var origin sql.NullString
	var replyTo sql.NullString
	var createdAt SQLiteTimeScanner

	err := row.Scan(
		&idStr, &threadIDStr, &msg.Role, &msg.AgentID, &msg.Content, &contentBlocks, &msg.MessageType, &metadata, &createdAt,
		&mentionsJSON, &origin, &replyTo,
	)
	if err != nil {
		return nil, err
	}

	msg.ID, _ = uuid.Parse(idStr)
	msg.ThreadID, _ = uuid.Parse(threadIDStr)
	msg.ContentBlocks = json.RawMessage(contentBlocks)
	msg.Metadata = json.RawMessage(metadata)
	msg.Mentions = deserializeStrings(mentionsJSON)
	msg.Origin = origin.String
	msg.CreatedAt = createdAt.Time
	if replyTo.Valid {
		replyToID, _ := uuid.Parse(replyTo.String)
		msg.ReplyTo = &replyToID
	}

	return msg, nil
}

// serializeStrings 序列化字符串数组为 JSON
func serializeStrings(arr []string) interface{} {
	if len(arr) == 0 {
		return nil
	}
	data, _ := json.Marshal(arr)
	return string(data)
}

// deserializeStrings 反序列化 JSON 为字符串数组
func deserializeStrings(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	var arr []string
	json.Unmarshal(data, &arr)
	return arr
}
// Update 更新消息（用于更新 content_blocks 等字段）
func (r *MessageRepository) Update(ctx context.Context, msg *model.Message) error {
	query := `
		UPDATE messages 
		SET content_blocks = ?, metadata = ?, content = ?
		WHERE id = ?
	`
	_, err := r.DB().ExecContext(ctx, query,
		msg.ContentBlocks, msg.Metadata, msg.Content, msg.ID.String(),
	)
	return err
}
