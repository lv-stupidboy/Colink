package agent

import (
	"context"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// ChunkType 输出块类型
type ChunkType string

const (
	ChunkTypeText           ChunkType = "text"
	ChunkTypeError          ChunkType = "error"
	ChunkTypeStatus         ChunkType = "status"
	ChunkTypeThinking       ChunkType = "thinking"         // 思考过程
	ChunkTypeToolUse        ChunkType = "tool_use"         // 工具调用开始
	ChunkTypeToolResult     ChunkType = "tool_result"      // 工具调用结果
	ChunkTypeInputJSONDelta ChunkType = "input_json_delta" // 工具参数增量更新
	ChunkTypeUsage          ChunkType = "usage"            // Token 使用更新
	ChunkTypeQuestion       ChunkType = "question"         // AskUserQuestion 工具调用（需要用户输入）
)

// SessionStrategy 会话策略类型
type SessionStrategy string

const (
	SessionStrategyNew    SessionStrategy = "new"    // 新会话，不传递历史（跨角色调用）
	SessionStrategyResume SessionStrategy = "resume" // 恢复会话，传递历史（同角色调用）
)

// MaxPreviousResponses PreviousResponses 最大长度限制
// 防止多轮对话导致 PreviousResponses 无限增长
const MaxPreviousResponses = 20

// A2AInputOptions A2A 输入构建选项
type A2AInputOptions struct {
	IncludeTokenBudget bool // 是否注入 Token 预算信息
	MaxSummaryLength   int  // 前序摘要最大长度（默认 500）
}

// ChainResponse 链路中的响应记录
type ChainResponse struct {
	AgentID   uuid.UUID // Agent ID
	AgentName string    // Agent 名称
	Content   string    // 响应内容（可能截断）
	Role      string    // 角色
	Timestamp int64     // 时间戳
}

// A2AChainContext A2A 链路追踪上下文（参考 clowder-ai route-serial previousResponses）
type A2AChainContext struct {
	ChainIndex        int             // 当前在链路中的位置（从 1 开始）
	ChainTotal        int             // 链路总长度（预计）
	Teammates         []uuid.UUID     // 链路中的队友列表
	PreviousResponses []ChainResponse // 前序响应累积（按时间顺序）
	OriginalMessage   string          // 原始用户消息
	FromAgent         *AgentInfo      // 直接触发者
	SessionStrategy   SessionStrategy // 会话策略
	Depth             int             // A2A 深度

	// clowder-ai 对齐新增字段
	A2AEnabled         bool                // 是否允许继续 A2A（参考 clowder-ai invocationContext.a2aEnabled）
	TokenBudget        *TokenBudgetInfo    // Token 预算信息
	ActiveParticipants []ActiveParticipant // 活跃参与者列表（参考 clowder-ai activeParticipants）
}

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
	ContextUsed         int64   `json:"contextUsed,omitempty"` // ACP: 已使用的 context tokens
	ContextSize         int64   `json:"contextSize,omitempty"` // ACP: context 总容量
}

// Chunk 流式输出块
type Chunk struct {
	Type        ChunkType
	Content     string
	ToolName    string                 // 工具名称（仅 tool_use 类型）
	ToolID      string                 // 工具ID（仅 tool_use 类型）
	ToolInput   map[string]interface{} // 工具参数（仅 tool_use 类型）
	ToolIndex   int                    // 工具在消息中的索引（用于 input_json_delta 定位）
	PartialJSON string                 // 增量 JSON（input_json_delta 类型，需要累积解析）
	IsError     bool                   // 是否错误（仅 tool_result 类型）
	Usage       *TokenUsage            // Token使用（仅 usage 类型）
	Done        bool                   // 是否结束（thinking 完成标记）
	// AskUserQuestion 相关字段（仅 question 类型）
	Questions []QuestionItem // 问题列表（仅 question 类型）
}

// QuestionItem AskUserQuestion 工具的问题项
type QuestionItem struct {
	Header      string           `json:"header"`      // 问题标题（最多 12 字符）
	Question    string           `json:"question"`    // 问题内容
	MultiSelect bool             `json:"multiSelect"` // 是否允许多选
	Options     []QuestionOption `json:"options"`     // 选项列表
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
	Images          []model.ImageContent // 多模态输入：图片列表
	WorkDir         string
	ConfigDir       string             // Agent配置目录路径（使用生成的配置）
	MCPServers      []*model.MCPServer // 显式绑定到 Agent 角色的 MCP Servers
	SessionID       string             // 会话ID（用于 --resume 复用已有会话，避免冷启动延迟）
	SessionStrategy SessionStrategy    // 会话策略：new 或 resume
	InvocationID    uuid.UUID          // Invocation ID（用于 AskUserQuestion 答案发送）
	CallbackToken   string             // MCP server 回调认证 Token
	APIURL          string             // MCP server 回调 API URL

	// OnSessionIDAcquired 在 adapter 拿到 session ID 时立即回调（不等进程退出）
	// 用于提前持久化 session ID，确保取消/崩溃后仍可 resume
	OnSessionIDAcquired func(sessionID string)
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

// RunningAgentInfo 运行中的Agent信息（用于API返回）
type RunningAgentInfo struct {
	InvocationID           uuid.UUID `json:"invocationId"`
	AgentName              string    `json:"agentName"`
	ProjectName            string    `json:"projectName"`
	ThreadTitle            string    `json:"threadTitle"`
	StartedAt              time.Time `json:"startedAt"`
	RunningDurationSeconds int       `json:"runningDurationSeconds"`
}
