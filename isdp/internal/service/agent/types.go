package agent

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
)

// ChunkType 输出块类型
type ChunkType string

const (
	ChunkTypeText   ChunkType = "text"
	ChunkTypeError  ChunkType = "error"
	ChunkTypeStatus ChunkType = "status"
)

// Chunk 流式输出块
type Chunk struct {
	Type    ChunkType
	Content string
}

// ExecutionRequest 统一的执行请求
type ExecutionRequest struct {
	Config     *model.AgentRoleConfig
	BaseAgent  *model.BaseAgent
	Context    *ContextLayers
	Input      string
	WorkDir    string
	SessionKey string // 用于会话恢复（空表示新会话）
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Output     string
	SessionKey string // 返回的会话标识（用于后续恢复）
}

// SessionExecutor 会话执行器接口，扩展了AgentAdapter的会话管理能力
type SessionExecutor interface {
	StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error
	ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error
	StopSession(sessionID string) error
	GetSessionStatus(sessionID string) SessionStatus
}