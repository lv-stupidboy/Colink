// internal/model/session_record.go
// Session 记录数据模型
package model

import (
	"time"

	"github.com/google/uuid"
)

// SessionRecord Session 记录
// 用于持久化 Session 信息，支持 ACP 原生 session/resume
type SessionRecord struct {
	// 基本标识
	ID       uuid.UUID      `json:"id" gorm:"primaryKey"`
	ThreadID uuid.UUID      `json:"threadId" gorm:"index;not null"`
	AgentID  uuid.UUID      `json:"agentId" gorm:"index;not null"`

	// Agent 类型
	AgentType BaseAgentType `json:"agentType" gorm:"type:varchar(50);not null"` // claude_code, open_code, code_agent

	// === ACP 原生 session/resume 模式 ===
	// 存储 ACP 协议的 session ID，用于 session/resume 或 session/load
	AcpSessionID string    `json:"acpSessionId" gorm:"type:varchar(100)"` // ACP 协议的 session ID
	ResumeExpiry int64     `json:"resumeExpiry"`                          // Resume 有效期时间戳

	// === Claude CLI resume 模式（兼容旧版本）===
	// 存储 CLI 的 session ID，用于 --resume 参数
	CliSessionID string    `json:"cliSessionId" gorm:"type:varchar(100)"` // CLI 内部的 session ID（旧模式）

	// 状态信息
	Status       string    `json:"status" gorm:"type:varchar(20)"`       // active, idle, sealed
	LastActiveAt int64     `json:"lastActiveAt"`                        // 最后活跃时间

	// 时间戳
	CreatedAt    int64     `json:"createdAt" gorm:"not null"`
	UpdatedAt    int64     `json:"updatedAt" gorm:"not null"`
}

// TableName 表名
func (SessionRecord) TableName() string {
	return "session_records"
}

// BeforeCreate 创建前钩子
func (s *SessionRecord) BeforeCreate() error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now().Unix()
	s.CreatedAt = now
	s.UpdatedAt = now
	if s.LastActiveAt == 0 {
		s.LastActiveAt = now
	}
	return nil
}

// BeforeUpdate 更新前钩子
func (s *SessionRecord) BeforeUpdate() error {
	s.UpdatedAt = time.Now().Unix()
	return nil
}

// IsExpired 检查是否过期
func (s *SessionRecord) IsExpired(expiryHours int) bool {
	if s.ResumeExpiry == 0 {
		return false
	}
	return time.Now().Unix() > s.ResumeExpiry
}

// SetResumeExpiry 设置 resume 有效期
func (s *SessionRecord) SetResumeExpiry(hours int) {
	s.ResumeExpiry = time.Now().Add(time.Duration(hours) * time.Hour).Unix()
}

// CreateSessionRecordRequest 创建 Session 记录请求
type CreateSessionRecordRequest struct {
	ThreadID      uuid.UUID
	AgentID       uuid.UUID
	AgentType     BaseAgentType
	AcpSessionID  string // ACP 协议 session ID
	CliSessionID  string // Claude CLI session ID（兼容）
}

// UpdateSessionRecordRequest 更新 Session 记录请求
type UpdateSessionRecordRequest struct {
	AcpSessionID  *string
	CliSessionID  *string
	LastActiveAt  *int64
}