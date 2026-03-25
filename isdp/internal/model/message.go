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
	MessageType MessageType     `json:"messageType"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
}

func (m *Message) TableName() string {
	return "messages"
}