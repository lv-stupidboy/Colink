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

// FindByThreadID 根据ThreadID查找消息（取最新的N条，按时间正序返回）
func (r *MessageRepository) FindByThreadID(ctx context.Context, threadID uuid.UUID, limit int) ([]*model.Message, error) {
	// 先用 DESC 取最新的 N 条，然后反转顺序
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

	// 反转顺序，使旧消息在前、新消息在后（符合聊天显示习惯）
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// FindByThreadIDBeforeCursor 根据ThreadID查找指定cursor之前的消息（用于向上滚动加载历史）
// cursor 是消息ID，返回比该消息更早的消息，按时间正序返回
func (r *MessageRepository) FindByThreadIDBeforeCursor(ctx context.Context, threadID uuid.UUID, cursor string, limit int) ([]*model.Message, error) {
	// 先获取 cursor 消息的 created_at，然后查询比它更早的消息
	cursorQuery := `SELECT created_at FROM messages WHERE id = ?`
	var cursorTime SQLiteTimeScanner
	err := r.DB().QueryRowContext(ctx, cursorQuery, cursor).Scan(&cursorTime)
	if err != nil {
		return nil, fmt.Errorf("cursor message not found: %w", err)
	}

	// 查询比 cursor 更早的消息，用 DESC 取最新的 N 条，然后反转
	query := `
		SELECT id, thread_id, role, agent_id, content, content_blocks, message_type, metadata, created_at, mentions, origin, reply_to
		FROM messages WHERE thread_id = ? AND created_at < ? ORDER BY created_at DESC LIMIT ?
	`
	rows, err := r.DB().QueryContext(ctx, query, threadID.String(), cursorTime.Time, limit)
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

	// 反转顺序，使旧消息在前、新消息在后
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// CountByThreadID 获取消息总数（用于判断是否还有更多历史）
func (r *MessageRepository) CountByThreadID(ctx context.Context, threadID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM messages WHERE thread_id = ?`
	var count int
	err := r.DB().QueryRowContext(ctx, query, threadID.String()).Scan(&count)
	return count, err
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

// FindUnreportedForReporting 查询未上报的消息（用于消息上报功能）
// 只查询 role='user' 和 role='agent' 的消息，排除 system 消息
// 按 created_at 升序排列，限制单次上报数量
func (r *MessageRepository) FindUnreportedForReporting(ctx context.Context, limit int) ([]*model.Message, error) {
	query := `
		SELECT id, thread_id, role, agent_id, content, content_blocks, message_type, metadata, created_at, reported_at, mentions, origin, reply_to
		FROM messages
		WHERE reported_at IS NULL AND role IN ('user', 'agent')
		ORDER BY created_at ASC
		LIMIT ?
	`
	rows, err := r.DB().QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages = make([]*model.Message, 0)
	for rows.Next() {
		msg, err := r.scanMessageWithReported(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// scanMessageWithReported 扫描消息行（包含 reported_at 字段）
func (r *MessageRepository) scanMessageWithReported(rows *sql.Rows) (*model.Message, error) {
	msg := &model.Message{}
	var idStr, threadIDStr string
	var contentBlocks []byte
	var metadata []byte
	var mentionsJSON []byte
	var origin sql.NullString
	var replyTo sql.NullString
	var createdAt SQLiteTimeScanner
	var reportedAt sql.NullTime // 使用 sql.NullTime 处理 NULL 值

	err := rows.Scan(
		&idStr, &threadIDStr, &msg.Role, &msg.AgentID, &msg.Content, &contentBlocks, &msg.MessageType, &metadata, &createdAt,
		&reportedAt, // 新增字段
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
	if reportedAt.Valid {
		msg.ReportedAt = &reportedAt.Time
	}
	if replyTo.Valid {
		replyToID, _ := uuid.Parse(replyTo.String)
		msg.ReplyTo = &replyToID
	}

	return msg, nil
}

// BatchUpdateReportedAt 批量更新消息的上报时间
// 使用事务保证数据一致性
func (r *MessageRepository) BatchUpdateReportedAt(ctx context.Context, messageIDs []uuid.UUID, reportedAt time.Time) error {
	if len(messageIDs) == 0 {
		return nil
	}

	// 构建 IN 查询的参数
	idStrs := make([]string, len(messageIDs))
	for i, id := range messageIDs {
		idStrs[i] = id.String()
	}

	// 构建 IN 子句
	inQuery := `UPDATE messages SET reported_at = ? WHERE id IN (`
	for i := range idStrs {
		if i > 0 {
			inQuery += `,`
		}
		inQuery += `?`
	}
	inQuery += `)`

	// 构建参数列表
	args := make([]interface{}, len(idStrs)+1)
	args[0] = reportedAt
	for i, idStr := range idStrs {
		args[i+1] = idStr
	}

	_, err := r.DB().ExecContext(ctx, inQuery, args...)
	return err
}
