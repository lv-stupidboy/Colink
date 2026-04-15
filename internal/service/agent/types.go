package agent

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// ChunkType 输出块类型
type ChunkType string

const (
	ChunkTypeText       ChunkType = "text"
	ChunkTypeError      ChunkType = "error"
	ChunkTypeStatus     ChunkType = "status"
	ChunkTypeThinking   ChunkType = "thinking"    // 思考过程
	ChunkTypeToolUse    ChunkType = "tool_use"    // 工具调用开始
	ChunkTypeToolResult ChunkType = "tool_result" // 工具调用结果
	ChunkTypeUsage      ChunkType = "usage"       // Token 使用更新
	ChunkTypeQuestion   ChunkType = "question"    // AskUserQuestion 工具调用（需要用户输入）
)

// SessionStrategy 会话策略类型
type SessionStrategy string

const (
	SessionStrategyNew    SessionStrategy = "new"    // 新会话，不传递历史（跨角色调用）
	SessionStrategyResume SessionStrategy = "resume" // 恢复会话，传递历史（同角色调用）
)

// TokenUsage Token使用统计
type TokenUsage struct {
	InputTokens         int64   `json:"inputTokens,omitempty"`
	OutputTokens        int64   `json:"outputTokens,omitempty"`
	CacheReadTokens     int64   `json:"cacheReadTokens,omitempty"`
	CacheCreationTokens int64   `json:"cacheCreationTokens,omitempty"`
	CostUsd             float64 `json:"costUsd,omitempty"`
	DurationMs          int64   `json:"durationMs,omitempty"`
	DurationApiMs       int64   `json:"durationApiMs,omitempty"`
	NumTurns            int     `json:"numTurns,omitempty"`
}

// Chunk 流式输出块
type Chunk struct {
	Type      ChunkType
	Content   string
	ToolName  string                 // 工具名称（仅 tool_use 类型）
	ToolID    string                 // 工具ID（仅 tool_use 类型）
	ToolInput map[string]interface{} // 工具参数（仅 tool_use 类型）
	IsError   bool                   // 是否错误（仅 tool_result 类型）
	Usage     *TokenUsage            // Token使用（仅 usage 类型）
	Done      bool                   // 是否结束（thinking 完成标记）
	// AskUserQuestion 相关字段（仅 question 类型）
	Questions []QuestionItem         // 问题列表（仅 question 类型）
}

// QuestionItem AskUserQuestion 工具的问题项
type QuestionItem struct {
	Header     string            `json:"header"`     // 问题标题（最多 12 字符）
	Question   string            `json:"question"`   // 问题内容
	MultiSelect bool             `json:"multiSelect"` // 是否允许多选
	Options    []QuestionOption `json:"options"`    // 选项列表
}

// QuestionOption AskUserQuestion 工具的选项
type QuestionOption struct {
	Label       string `json:"label"`       // 选项标签
	Description string `json:"description"` // 选项描述
	Preview     string `json:"preview"`     // 预览内容（可选）
}

// ChunkListener 外部 chunk 监听器回调函数类型
type ChunkListener func(threadID, invocationID uuid.UUID, chunk Chunk, agentID, agentName string)

// ExecutionRequest 统一的执行请求
type ExecutionRequest struct {
	Config          *model.AgentRoleConfig
	BaseAgent       *model.BaseAgent
	Context         *ContextLayers
	Input           string
	WorkDir         string
	ConfigDir       string           // Agent配置目录路径（使用生成的配置）
	SessionID       string           // 会话ID（用于 --resume 复用已有会话，避免冷启动延迟）
	SessionStrategy SessionStrategy  // 会话策略：new 或 resume
	InvocationID    uuid.UUID        //Invocation ID（用于 AskUserQuestion 答案发送）
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Output    string
	SessionID string // 会话ID（用于后续 --resume 复用）
}

// SessionExecutor 会话执行器接口，扩展了AgentAdapter的会话管理能力
type SessionExecutor interface {
	StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error
	ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error
	StopSession(sessionID string) error
	GetSessionStatus(sessionID string) SessionStatus
}
