package memory

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
)

// ========== MemoryProvider Interface ==========

// Option Provider 配置选项
type Option func(*ProviderConfig)

// ProviderConfig Provider 配置
type ProviderConfig struct {
	SessionID   string
	HermesHome  string
	Platform    string
	UserID      string
	ThreadID    string
	TeamID      string
	ProjectID   string // 新增项目 ID
	BaseAgentID string
}

// WithSessionID 设置会话 ID
func WithSessionID(id string) Option {
	return func(c *ProviderConfig) { c.SessionID = id }
}

// WithUserID 设置用户 ID
func WithUserID(id string) Option {
	return func(c *ProviderConfig) { c.UserID = id }
}

// WithThreadID 设置线程 ID
func WithThreadID(id string) Option {
	return func(c *ProviderConfig) { c.ThreadID = id }
}

// WithTeamID 设置团队 ID
func WithTeamID(id string) Option {
	return func(c *ProviderConfig) { c.TeamID = id }
}

// WithProjectID 设置项目 ID
func WithProjectID(id string) Option {
	return func(c *ProviderConfig) { c.ProjectID = id }
}

// WithBaseAgentID 设置 BaseAgent ID
func WithBaseAgentID(id string) Option {
	return func(c *ProviderConfig) { c.BaseAgentID = id }
}

// MemoryProvider 记忆提供者抽象接口
// 参考 hermes-agent MemoryProvider ABC 设计
type MemoryProvider interface {
	// Name 返回提供者唯一标识
	Name() string

	// IsAvailable 检查提供者是否可用（只做本地检查，禁止网络调用）
	IsAvailable() bool

	// Initialize 初始化提供者
	Initialize(sessionID string, opts ...Option) error

	// GetToolSchemas 返回暴露给 Agent 的工具 Schema
	GetToolSchemas() []map[string]any

	// HandleToolCall 处理工具调用
	HandleToolCall(ctx context.Context, name string, args map[string]any) (string, error)

	// ========== Optional Hooks ==========

	// Prefetch 对话前预取相关记忆
	Prefetch(ctx context.Context, query string, scope model.MemoryScope, scopeID string) string

	// SyncTurn 对话后同步（必须非阻塞）
	SyncTurn(ctx context.Context, userContent, assistantContent string)

	// OnSessionEnd 会话结束时调用
	OnSessionEnd(ctx context.Context, messages []map[string]any)

	// OnThreadEnd 线程结束时设置 TTL
	OnThreadEnd(ctx context.Context, threadID string) error

	// OnMemoryWrite 内置记忆写入时镜像到外部
	OnMemoryWrite(ctx context.Context, action, scope, content string)

	// Shutdown 清理资源
	Shutdown()
}