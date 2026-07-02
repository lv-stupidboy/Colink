package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ========== Image Content Model ==========

// ImageContent 图片内容（用于多模态输入）
type ImageContent struct {
	MimeType string `json:"mimeType"` // MIME类型：image/png, image/jpeg, image/gif
	Data     string `json:"data"`     // base64数据（不含 data:image/xxx;base64, 前缀）
}

// ========== Message Models ==========

type MessageRole string

const (
	MessageRoleUser   MessageRole = "user"
	MessageRoleAgent  MessageRole = "agent"
	MessageRoleSystem MessageRole = "system"
)

type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeArtifact MessageType = "artifact"
	MessageTypeSystem   MessageType = "system"
)

// Message 消息模型
type Message struct {
	ID          uuid.UUID       `json:"id"`
	ThreadID    uuid.UUID       `json:"threadId"`
	Role        MessageRole     `json:"role"`
	AgentID     string          `json:"agentId,omitempty"`
	Content     string          `json:"content"`
	ContentBlocks json.RawMessage `json:"contentBlocks,omitempty"` // 结构化内容块
	MessageType MessageType     `json:"messageType"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	ReportedAt  *time.Time      `json:"reportedAt,omitempty"` // 上报时间，NULL 表示未上报

	// SortableID 字典序单调 ID，供 A2A 拉模式 cursor 使用
	// 格式：{ts_ms_16padded}-{seq_6padded}-{uuid_prefix_8}
	// 由 repo.MessageRepository.Create 内部生成（保证与 CreatedAt 单调对齐），
	// 上层不需要也不应该自己填。
	SortableID string `json:"sortableId,omitempty"`

	// A2A 相关字段
	Mentions     []string    `json:"mentions,omitempty"`      // 被 @mention 的 Agent IDs
	MentionsUser bool        `json:"mentionsUser,omitempty"` // 是否 @用户
	Origin       string      `json:"origin,omitempty"`        // "user", "callback", "stream"
	ReplyTo      *uuid.UUID  `json:"replyTo,omitempty"`       // 回复的消息 ID
}

func (m *Message) TableName() string {
	return "messages"
}

// Now 返回当前时间的毫秒时间戳
func Now() int64 {
	return time.Now().UnixMilli()
}