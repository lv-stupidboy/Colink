package agent

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
)

// ChunkType 输出块类型
type ChunkType string

const (
	ChunkTypeText       ChunkType = "text"
	ChunkTypeError      ChunkType = "error"
	ChunkTypeStatus     ChunkType = "status"
	ChunkTypeThinking   ChunkType = "thinking"   // 思考过程
	ChunkTypeToolUse    ChunkType = "tool_use"  // 工具调用开始
	ChunkTypeToolResult ChunkType = "tool_result" // 工具调用结果
)

// Chunk 流式输出块
type Chunk struct {
	Type     ChunkType
	Content  string
	ToolName string                 // 工具名称（仅 tool_use 类型）
	ToolID   string                 // 工具ID（仅 tool_use 类型）
	ToolInput map[string]interface{} // 工具参数（仅 tool_use 类型）
	IsError  bool                   // 是否错误（仅 tool_result 类型）
}

// ExecutionRequest 统一的执行请求
type ExecutionRequest struct {
	Config    *model.AgentRoleConfig
	BaseAgent *model.BaseAgent
	Context   *ContextLayers
	Input     string
	WorkDir   string
	ConfigDir string // Agent配置目录路径（使用生成的配置）
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Output string
}

// SessionExecutor 会话执行器接口，扩展了AgentAdapter的会话管理能力
type SessionExecutor interface {
	StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error
	ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error
	StopSession(sessionID string) error
	GetSessionStatus(sessionID string) SessionStatus
}