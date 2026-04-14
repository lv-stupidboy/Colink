package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== IM Session Models ==========

// IMPlatform IM平台类型
type IMPlatform string

const (
	IMPlatformFeishu IMPlatform = "feishu"
)

// IMSession IM会话映射
type IMSession struct {
	ID            uuid.UUID  `json:"id"`
	Platform      IMPlatform `json:"platform"`
	ChatID        string     `json:"chatId"`
	ChatType      string     `json:"chatType"`
	ThreadID      uuid.UUID  `json:"threadId"`
	ProjectID     uuid.UUID  `json:"projectId"`
	UserID        string     `json:"userId,omitempty"`
	UserName      string     `json:"userName,omitempty"`
	LastMessageAt *time.Time `json:"lastMessageAt,omitempty"`
	IsActive      bool       `json:"isActive"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}
