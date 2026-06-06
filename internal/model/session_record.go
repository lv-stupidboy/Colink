// internal/model/session_record.go
// Session 记录数据模型
package model

import (
	"time"

	"github.com/google/uuid"
)

// SessionRecord Session 记录
// 用于持久化 Session 信息，支持两种模式：
// 1. Claude CLI resume 模式：存储 CliSessionID，用于 --resume
// 2. OpenCode/CodeAgent 长连接模式：存储对话历史和状态
type SessionRecord struct {
	// 基本标识
	ID       uuid.UUID      `json:"id" gorm:"primaryKey"`
	ThreadID uuid.UUID      `json:"threadId" gorm:"index;not null"`
	AgentID  uuid.UUID      `json:"agentId" gorm:"index;not null"`

	// Agent 类型
	AgentType BaseAgentType `json:"agentType" gorm:"type:varchar(50);not null"` // claude_code, open_code, code_agent

	// === Claude CLI resume 模式 ===
	// 存储 CLI 的 session ID，用于 --resume 参数
	CliSessionID string    `json:"cliSessionId" gorm:"type:varchar(100)"` // CLI 内部的 session ID
	ResumeExpiry int64     `json:"resumeExpiry"`                          // Resume 有效期时间戳

	// === OpenCode/CodeAgent 长连接模式 ===
	// 存储 session 状态和对话历史
	Status       string    `json:"status" gorm:"type:varchar(20)"`       // active, idle, sealed, recovering
	TurnCount    int       `json:"turnCount"`                           // 对话轮数
	TotalTokens  int       `json:"totalTokens"`                         // Token 总数
	Conversation []byte    `json:"conversation" gorm:"type:blob"`        // ConversationBuffer 序列化
	KeyEntities  []byte    `json:"keyEntities" gorm:"type:blob"`         // KeyEntity 列表序列化

	// 进程信息（长连接模式）
	ProcessPID   int       `json:"processPid"`                          // 进程 PID（仅供参考）

	// 时间戳
	CreatedAt    int64     `json:"createdAt" gorm:"not null"`
	UpdatedAt    int64     `json:"updatedAt" gorm:"not null"`
	LastActiveAt int64     `json:"lastActiveAt"`                        // 最后活跃时间
	SealedAt     int64     `json:"sealedAt"`                            // 封存时间（Sealed 状态才有）

	// 错误信息
	LastError    string    `json:"lastError" gorm:"type:text"`          // 最后错误信息
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

// IsExpired 检查是否过期（Claude CLI resume 模式）
func (s *SessionRecord) IsExpired(expiryHours int) bool {
	if s.ResumeExpiry == 0 {
		return false
	}
	return time.Now().Unix() > s.ResumeExpiry
}

// IsSealed 检查是否已封存（长连接模式）
func (s *SessionRecord) IsSealed() bool {
	return s.Status == "sealed"
}

// IsRecoverable 检查是否可恢复
func (s *SessionRecord) IsRecoverable() bool {
	// Sealed 状态且有对话历史，可以恢复
	return s.Status == "sealed" && s.TurnCount > 0 && len(s.Conversation) > 0
}

// SetResumeExpiry 设置 resume 有效期
func (s *SessionRecord) SetResumeExpiry(hours int) {
	s.ResumeExpiry = time.Now().Add(time.Duration(hours) * time.Hour).Unix()
}

// Seal 封存 session
func (s *SessionRecord) Seal() {
	s.Status = "sealed"
	s.SealedAt = time.Now().Unix()
	s.UpdatedAt = time.Now().Unix()
}

// Activate 激活 session
func (s *SessionRecord) Activate() {
	s.Status = "active"
	s.LastActiveAt = time.Now().Unix()
	s.UpdatedAt = time.Now().Unix()
}

// CreateSessionRecordRequest 创建 Session 记录请求
type CreateSessionRecordRequest struct {
	ThreadID      uuid.UUID
	AgentID       uuid.UUID
	AgentType     BaseAgentType
	CliSessionID  string // Claude CLI 模式
	Status        string // 长连接模式
	Conversation  []byte // 长连接模式
	KeyEntities   []byte // 长连接模式
}

// UpdateSessionRecordRequest 更新 Session 记录请求
type UpdateSessionRecordRequest struct {
	CliSessionID  *string
	Status        *string
	TurnCount     *int
	TotalTokens   *int
	Conversation  []byte
	KeyEntities   []byte
	LastActiveAt  *int64
	SealedAt      *int64
	LastError     *string
}