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
	ThreadID    uuid.UUID       `json:"thread_id"`
	Role        MessageRole     `json:"role"`
	AgentID     string          `json:"agent_id,omitempty"`
	Content     string          `json:"content"`
	MessageType MessageType     `json:"message_type"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

func (m *Message) TableName() string {
	return "messages"
}