package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

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