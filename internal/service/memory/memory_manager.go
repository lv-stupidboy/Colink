package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
)

// MemoryManager 记忆管理器，协调内置和外部提供者
// 设计约束：单一外部提供者强制（参考 hermes-agent MemoryManager）
type MemoryManager struct {
	builtin   *BuiltinProvider
	external  MemoryProvider // 单一外部提供者
	hasExternal bool          // 外部提供者标记

	mu        sync.RWMutex
	config    ProviderConfig
}

// NewMemoryManager 创建记忆管理器
func NewMemoryManager(db *sql.DB) *MemoryManager {
	return &MemoryManager{
		builtin: NewBuiltinProvider(db),
	}
}

// RegisterExternalProvider 注册外部提供者（单一强制）
// 如果已有外部提供者，返回错误
func (m *MemoryManager) RegisterExternalProvider(provider MemoryProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.hasExternal {
		return ErrExternalProviderAlreadyRegistered
	}

	if !provider.IsAvailable() {
		return ErrProviderNotAvailable
	}

	m.external = provider
	m.hasExternal = true
	return nil
}

// Initialize 初始化所有提供者
func (m *MemoryManager) Initialize(sessionID string, opts ...Option) error {
	m.config = ProviderConfig{SessionID: sessionID}
	for _, opt := range opts {
		opt(&m.config)
	}

	// 初始化内置提供者
	if err := m.builtin.Initialize(sessionID, opts...); err != nil {
		return err
	}

	// 初始化外部提供者（如果存在）
	m.mu.RLock()
	ext := m.external
	m.mu.RUnlock()

	if ext != nil {
		if err := ext.Initialize(sessionID, opts...); err != nil {
			return err
		}
	}

	return nil
}

// ========== Tool Schema Aggregation ==========

// GetToolSchemas 返回聚合的工具 Schema
// 内置提供者始终提供 memory 工具
// 外部提供者可扩展额外工具
func (m *MemoryManager) GetToolSchemas() []map[string]any {
	schemas := m.builtin.GetToolSchemas()

	m.mu.RLock()
	ext := m.external
	m.mu.RUnlock()

	if ext != nil {
		schemas = append(schemas, ext.GetToolSchemas()...)
	}

	return schemas
}

// HandleToolCall 处理工具调用
// 路由到内置提供者
func (m *MemoryManager) HandleToolCall(ctx context.Context, name string, args map[string]any) (string, error) {
	// team_memory 和 project_memory 由内置提供者处理
	if name == "team_memory" || name == "project_memory" {
		resp, err := m.builtin.HandleToolCall(ctx, name, args)

		// 镜像写入到外部提供者（如果存在）
		m.mu.RLock()
		ext := m.external
		m.mu.RUnlock()

		if ext != nil && err == nil {
			action, _ := args["action"].(string)
			content, _ := args["content"].(string)
			ext.OnMemoryWrite(ctx, action, name, content)
		}

		return resp, err
	}

	// 其他工具路由到外部提供者
	m.mu.RLock()
	ext := m.external
	m.mu.RUnlock()

	if ext != nil {
		return ext.HandleToolCall(ctx, name, args)
	}

	// 无匹配提供者
	resp := model.MemoryToolResponse{
		Success: false,
		Error:   "Unknown tool: " + name,
	}
	data, _ := json.Marshal(resp)
	return string(data), nil
}

// ========== Hooks ==========

// Prefetch 对话前预取相关记忆
// 同时查询内置和外部提供者，聚合结果
func (m *MemoryManager) Prefetch(ctx context.Context, query string, scope model.MemoryScope, scopeID string) string {
	var parts []string

	// 内置提供者预取
	builtinResult := m.builtin.Prefetch(ctx, query, scope, scopeID)
	if builtinResult != "" {
		parts = append(parts, builtinResult)
	}

	// 外部提供者预取（如果存在）
	m.mu.RLock()
	ext := m.external
	m.mu.RUnlock()

	if ext != nil {
		extResult := ext.Prefetch(ctx, query, scope, scopeID)
		if extResult != "" {
			parts = append(parts, extResult)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n")
}

// PrefetchMultiScope 多层级预取 - 分层设计版本
// 只预取 ISDP 管理的 team + project 级记忆，注入到 Prompt
// CLI 管理 session/agent 级记忆，不在此预取（CLI memory 不注入 Prompt）
// 参数说明：teamID = WorkflowTemplateID，userID 复用为 projectID（后续需改进签名）
func (m *MemoryManager) PrefetchMultiScope(ctx context.Context, threadID, agentID, teamID, projectID string) string {
	// 内置提供者多层级预取（team + project）
	builtinResult := m.builtin.PrefetchMultiScope(ctx, threadID, agentID, teamID, projectID)

	// 外部提供者预取（如果存在）
	m.mu.RLock()
	ext := m.external
	m.mu.RUnlock()

	var extResult string
	if ext != nil {
		// 外部提供者预取（传入空 scope，由外部提供者自行决定）
		extResult = ext.Prefetch(ctx, "", model.MemoryScope(""), "")
	}

	// 合并结果
	if builtinResult != "" && extResult != "" {
		return builtinResult + "\n\n" + extResult
	}
	if builtinResult != "" {
		return builtinResult
	}
	return extResult
}

// BuildMemoryContextBlock 包装记忆内容为 fenced block（供 ExecutionService 调用）
func (m *MemoryManager) BuildMemoryContextBlock(rawContext string) string {
	return BuildMemoryContextBlock(rawContext)
}

// SyncTurn 对话后同步（必须非阻塞）
// 参考 hermes-agent：goroutine + context.WithTimeout(30s)
func (m *MemoryManager) SyncTurn(ctx context.Context, userContent, assistantContent string) {
	m.mu.RLock()
	ext := m.external
	m.mu.RUnlock()

	if ext == nil {
		return // 内置提供者不自动同步
	}

	// 异步执行，带超时保护
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		ext.SyncTurn(syncCtx, userContent, assistantContent)
	}()
}

// OnSessionEnd 会话结束时调用
func (m *MemoryManager) OnSessionEnd(ctx context.Context, messages []map[string]any) {
	// 内置提供者不自动提取
	m.mu.RLock()
	ext := m.external
	m.mu.RUnlock()

	if ext != nil {
		ext.OnSessionEnd(ctx, messages)
	}
}

// OnThreadEnd 线程结束时通知外部提供者（内置提供者不处理）
func (m *MemoryManager) OnThreadEnd(ctx context.Context, threadID string) error {
	m.mu.RLock()
	ext := m.external
	m.mu.RUnlock()

	if ext != nil {
		return ext.OnThreadEnd(ctx, threadID)
	}

	return nil
}

// Shutdown 清理所有提供者资源
func (m *MemoryManager) Shutdown() {
	m.builtin.Shutdown()

	m.mu.RLock()
	ext := m.external
	m.mu.RUnlock()

	if ext != nil {
		ext.Shutdown()
	}
}

// ========== Errors ==========

var (
	ErrExternalProviderAlreadyRegistered = errors.New("external provider already registered")
	ErrProviderNotAvailable              = errors.New("provider not available")
)