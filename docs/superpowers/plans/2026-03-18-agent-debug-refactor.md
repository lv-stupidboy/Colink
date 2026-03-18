# Agent调试功能重构实施计划

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复调试功能的500错误，实现内存管理的调试线程，统一前后端调试与正式执行的体验。

**Architecture:** 后端使用`DebugThreadManager`在内存中管理调试线程，完全隔离数据库依赖；前端提取`MessageCard`、`MessageInput`、`SandboxPanel`共享组件，调试页面和正式执行页面复用组件但使用不同状态管理（Zustand store）。

**Tech Stack:** Go (Gin, sync.RWMutex), React, TypeScript, Zustand, WebSocket

---

## 文件结构

### 后端文件

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/service/agent/debug_thread_manager.go` | 创建 | 内存调试线程管理，生命周期，WebSocket广播 |
| `internal/service/agent/orchestrator.go` | 修改 | 添加调试支持方法 |
| `internal/api/agent_handler.go` | 修改 | 注入DebugThreadManager，简化调试API |
| `cmd/server/main.go` | 修改 | 初始化DebugThreadManager |

### 前端文件

| 文件 | 操作 | 职责 |
|------|------|------|
| `src/types/index.ts` | 创建 | 类型定义 |
| `src/hooks/useWebSocket.ts` | 创建 | WebSocket连接hook |
| `src/store/debugThread.ts` | 创建 | 调试线程状态管理 |
| `src/components/thread/MessageCard.tsx` | 创建 | 消息卡片组件 |
| `src/components/thread/MessageCard.css` | 创建 | 消息卡片样式 |
| `src/components/thread/MessageInput.tsx` | 创建 | 消息输入组件 |
| `src/components/thread/MessageInput.css` | 创建 | 消息输入样式 |
| `src/components/thread/SandboxPanel.tsx` | 创建 | 沙箱面板组件 |
| `src/components/thread/SandboxPanel.css` | 创建 | 沙箱面板样式 |
| `src/components/thread/index.ts` | 创建 | 组件导出 |
| `src/pages/AgentDebug.tsx` | 修改 | 重构为对话形式 |
| `src/pages/ThreadView.tsx` | 修改 | 使用共享组件 |
| `src/api/agents.ts` | 修改 | 添加调试API方法 |

---

## Task 1: 创建DebugThreadManager

**Files:**
- Create: `isdp/internal/service/agent/debug_thread_manager.go`
- Test: `isdp/internal/service/agent/debug_thread_manager_test.go`

- [ ] **Step 1: Write the failing test for DebugThreadManager**

```go
// isdp/internal/service/agent/debug_thread_manager_test.go
package agent

import (
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestNewDebugThreadManager(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	if mgr == nil {
		t.Error("Expected non-nil manager")
	}
	if mgr.threads == nil {
		t.Error("Expected initialized threads map")
	}
	if mgr.maxAge != 2*time.Hour {
		t.Errorf("Expected maxAge 2h, got %v", mgr.maxAge)
	}
}

func TestDebugThreadManager_CreateThread(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread()

	if thread == nil {
		t.Error("Expected non-nil thread")
	}
	if thread.ID == uuid.Nil {
		t.Error("Expected non-nil thread ID")
	}
	if thread.Status != "idle" {
		t.Errorf("Expected status 'idle', got %s", thread.Status)
	}
	if len(thread.Messages) != 0 {
		t.Error("Expected empty messages slice")
	}

	// Verify thread is stored
	retrieved := mgr.GetThread(thread.ID)
	if retrieved == nil || retrieved.ID != thread.ID {
		t.Error("Thread not stored correctly")
	}
}

func TestDebugThreadManager_AddMessage(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread()

	msg := &model.Message{
		ID:      uuid.New(),
		Role:    "user",
		Content: "test message",
	}
	mgr.AddMessage(thread.ID, msg)

	messages := mgr.GetMessages(thread.ID)
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "test message" {
		t.Errorf("Expected 'test message', got %s", messages[0].Content)
	}
}

func TestDebugThreadManager_SetStatus(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread()

	mgr.SetStatus(thread.ID, "running")
	if thread.Status != "running" {
		t.Errorf("Expected status 'running', got %s", thread.Status)
	}
}

func TestDebugThreadManager_DeleteThread(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread()

	mgr.DeleteThread(thread.ID)
	if mgr.GetThread(thread.ID) != nil {
		t.Error("Expected thread to be deleted")
	}
}

func TestDebugThreadManager_ConcurrentAccess(t *testing.T) {
	mgr := NewDebugThreadManager(nil)

	// Concurrent thread creation
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			thread := mgr.CreateThread()
			if thread == nil {
				t.Error("Failed to create thread concurrently")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd isdp && go test ./internal/service/agent/... -run TestDebugThreadManager -v`
Expected: FAIL (undefined: DebugThreadManager, NewDebugThreadManager)

- [ ] **Step 3: Write DebugThreadManager implementation**

```go
// isdp/internal/service/agent/debug_thread_manager.go
package agent

import (
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// DebugThread 内存中的调试线程
type DebugThread struct {
	ID        uuid.UUID
	Status    string // idle, running, completed, error
	CreatedAt time.Time
	Messages  []*model.Message
}

// DebugThreadManager 调试线程管理器
type DebugThreadManager struct {
	threads         map[uuid.UUID]*DebugThread
	mu              sync.RWMutex
	wsHub           *ws.Hub
	maxAge          time.Duration // 线程最大存活时间
	cleanupInterval time.Duration // 清理间隔
	stopCleanup     chan struct{} // 停止清理信号
}

// NewDebugThreadManager 创建调试线程管理器
func NewDebugThreadManager(wsHub *ws.Hub) *DebugThreadManager {
	m := &DebugThreadManager{
		threads:         make(map[uuid.UUID]*DebugThread),
		wsHub:           wsHub,
		maxAge:          2 * time.Hour,
		cleanupInterval: 30 * time.Minute,
		stopCleanup:     make(chan struct{}),
	}
	go m.startCleanupRoutine()
	return m
}

// startCleanupRoutine 定期清理过期线程
func (m *DebugThreadManager) startCleanupRoutine() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredThreads()
		case <-m.stopCleanup:
			return
		}
	}
}

// cleanupExpiredThreads 清理过期线程
func (m *DebugThreadManager) cleanupExpiredThreads() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, thread := range m.threads {
		if now.Sub(thread.CreatedAt) > m.maxAge {
			// 广播线程关闭消息
			if m.wsHub != nil {
				m.wsHub.BroadcastToThread(id.String(), ws.WSMessage{
					Type: "thread_expired",
					Payload: map[string]interface{}{
						"threadId": id.String(),
						"message":  "调试会话已过期，请重新开始",
					},
					ThreadID:  id.String(),
					Timestamp: now.Unix(),
				})
			}
			delete(m.threads, id)
		}
	}
}

// Stop 停止清理协程（用于优雅关闭）
func (m *DebugThreadManager) Stop() {
	close(m.stopCleanup)
}

// CreateThread 创建调试线程
func (m *DebugThreadManager) CreateThread() *DebugThread {
	m.mu.Lock()
	defer m.mu.Unlock()

	thread := &DebugThread{
		ID:        uuid.New(),
		Status:    "idle",
		CreatedAt: time.Now(),
		Messages:  make([]*model.Message, 0),
	}
	m.threads[thread.ID] = thread
	return thread
}

// GetThread 获取调试线程
func (m *DebugThreadManager) GetThread(id uuid.UUID) *DebugThread {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.threads[id]
}

// AddMessage 添加消息
func (m *DebugThreadManager) AddMessage(threadID uuid.UUID, msg *model.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if thread, ok := m.threads[threadID]; ok {
		thread.Messages = append(thread.Messages, msg)
	}
}

// GetMessages 获取消息列表
func (m *DebugThreadManager) GetMessages(threadID uuid.UUID) []*model.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if thread, ok := m.threads[threadID]; ok {
		return thread.Messages
	}
	return nil
}

// SetStatus 设置线程状态
func (m *DebugThreadManager) SetStatus(threadID uuid.UUID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if thread, ok := m.threads[threadID]; ok {
		thread.Status = status
	}
}

// DeleteThread 删除调试线程（清理资源）
func (m *DebugThreadManager) DeleteThread(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.threads, id)
}

// Broadcast 向线程广播消息（WebSocket集成）
func (m *DebugThreadManager) Broadcast(threadID uuid.UUID, msgType string, payload map[string]interface{}) {
	if m.wsHub == nil {
		return
	}

	msg := ws.WSMessage{
		Type:      msgType,
		Payload:   payload,
		ThreadID:  threadID.String(),
		Timestamp: time.Now().Unix(),
	}

	m.wsHub.BroadcastToThread(threadID.String(), msg)
}

// BroadcastChunk 广播流式输出块
func (m *DebugThreadManager) BroadcastChunk(threadID, invocationID, agentID, agentName, chunk string) {
	m.Broadcast(threadID, "agent_output_chunk", map[string]interface{}{
		"chunk":        chunk,
		"invocationId": invocationID,
		"agentId":      agentID,
		"agentName":    agentName,
	})
}

// BroadcastMessage 广播完整消息
func (m *DebugThreadManager) BroadcastMessage(threadID, messageID, agentID, agentName, agentRole, content string) {
	m.Broadcast(threadID, "agent_message", map[string]interface{}{
		"messageId":  messageID,
		"agentId":    agentID,
		"agentName":  agentName,
		"agentRole":  agentRole,
		"content":    content,
	})
}

// BroadcastSandboxReady 广播沙箱就绪
func (m *DebugThreadManager) BroadcastSandboxReady(threadID, sandboxURL string) {
	m.Broadcast(threadID, "sandbox_ready", map[string]interface{}{
		"url": sandboxURL,
	})
}

// BroadcastError 广播错误消息
func (m *DebugThreadManager) BroadcastError(threadID, errorMsg string) {
	m.Broadcast(threadID, "system_message", map[string]interface{}{
		"content": errorMsg,
		"level":   "error",
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd isdp && go test ./internal/service/agent/... -run TestDebugThreadManager -v`
Expected: PASS (all tests pass)

- [ ] **Step 5: Commit**

```bash
cd isdp && git add internal/service/agent/debug_thread_manager.go internal/service/agent/debug_thread_manager_test.go
git commit -m "feat(agent): add DebugThreadManager for in-memory debug threads"
```

---

## Task 2: Orchestrator添加调试支持方法

**Files:**
- Modify: `isdp/internal/service/agent/orchestrator.go`

- [ ] **Step 1: Read current orchestrator.go to understand structure**

Run: Read file `isdp/internal/service/agent/orchestrator.go`
Expected: Understand current imports, struct definition, existing methods

- [ ] **Step 2: Add debugThreadMgr field to Orchestrator struct**

Find the `Orchestrator` struct and add the new field:

```go
type Orchestrator struct {
	// ... existing fields ...
	debugThreadMgr *DebugThreadManager // 新增
}
```

- [ ] **Step 3: Add SetDebugThreadManager method**

```go
// SetDebugThreadManager 设置调试线程管理器
func (o *Orchestrator) SetDebugThreadManager(mgr *DebugThreadManager) {
	o.debugThreadMgr = mgr
}
```

- [ ] **Step 4: Add SpawnDebugAgent method**

Add necessary imports at top of file if not present:
```go
import (
	// ... existing imports ...
	"fmt"
	"strings"
)
```

Add the method:

```go
// SpawnDebugAgent 调试模式启动Agent
func (o *Orchestrator) SpawnDebugAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
	if o.debugThreadMgr == nil {
		return nil, fmt.Errorf("debug thread manager not initialized")
	}

	// 验证调试线程存在
	debugThread := o.debugThreadMgr.GetThread(req.ThreadID)
	if debugThread == nil {
		return nil, fmt.Errorf("debug thread not found: %s", req.ThreadID)
	}

	// 获取Agent配置
	config, err := o.configService.GetByID(ctx, req.ConfigID)
	if err != nil {
		return nil, fmt.Errorf("agent config not found: %w", err)
	}

	// 获取基础Agent
	baseAgent, err := o.baseAgentService.GetByID(ctx, config.BaseAgentID)
	if err != nil {
		return nil, fmt.Errorf("base agent not found: %w", err)
	}

	// 创建适配器
	adapter := NewAdapter(baseAgent)
	if adapter == nil {
		return nil, fmt.Errorf("unsupported agent type: %s", baseAgent.Type)
	}

	// 更新调试线程状态
	o.debugThreadMgr.SetStatus(req.ThreadID, "running")

	// 添加用户消息到内存
	userMsg := &model.Message{
		ID:        uuid.New(),
		ThreadID:  req.ThreadID,
		Role:      "user",
		Content:   req.Input,
		CreatedAt: time.Now(),
	}
	o.debugThreadMgr.AddMessage(req.ThreadID, userMsg)

	// 创建调用记录（内存中，不写数据库）
	invocation := &model.AgentInvocation{
		ID:        uuid.New(),
		ThreadID:  req.ThreadID,
		ConfigID:  req.ConfigID,
		Role:      req.Role,
		Status:    "running",
		Input:     req.Input,
		StartedAt: time.Now(),
	}

	// 启动goroutine执行Agent
	go o.executeDebugAgent(req.ThreadID, invocation, adapter, config, req)

	return invocation, nil
}
```

- [ ] **Step 5: Add executeDebugAgent method**

```go
// executeDebugAgent 执行调试Agent（异步）
func (o *Orchestrator) executeDebugAgent(
	threadID uuid.UUID,
	invocation *model.AgentInvocation,
	adapter AgentAdapter,
	config *model.AgentRoleConfig,
	req *SpawnRequest,
) {
	ctx := context.Background()
	invocationID := invocation.ID.String()

	// 构建执行上下文
	execReq := &ExecutionRequest{
		Input:       req.Input,
		ProjectPath: req.ProjectPath,
		Context: &ContextLayers{
			Layer0: config.SystemPrompt,
		},
	}

	// 创建输出收集器
	var outputBuilder strings.Builder
	agentID := config.ID.String()
	agentName := config.Name
	agentRole := string(config.Role)

	// 执行Agent并收集输出
	err := adapter.Execute(ctx, execReq, func(chunk string) {
		outputBuilder.WriteString(chunk)
		// 广播流式输出
		o.debugThreadMgr.BroadcastChunk(threadID, invocationID, agentID, agentName, chunk)
	})

	if err != nil {
		o.debugThreadMgr.SetStatus(threadID, "error")
		o.debugThreadMgr.BroadcastError(threadID, fmt.Sprintf("Agent执行失败: %v", err))
		return
	}

	// 添加Agent消息到内存
	agentMsg := &model.Message{
		ID:        uuid.New(),
		ThreadID:  threadID,
		Role:      "agent",
		AgentID:   &config.ID,
		Content:   outputBuilder.String(),
		CreatedAt: time.Now(),
	}
	o.debugThreadMgr.AddMessage(threadID, agentMsg)

	// 广播完整消息
	o.debugThreadMgr.BroadcastMessage(threadID, agentMsg.ID.String(), agentID, agentName, agentRole, agentMsg.Content)

	// 更新线程状态
	o.debugThreadMgr.SetStatus(threadID, "idle")
}
```

- [ ] **Step 6: Add ContinueDebugAgent method**

```go
// ContinueDebugAgent 继续调试会话
func (o *Orchestrator) ContinueDebugAgent(ctx context.Context, threadID uuid.UUID, message string) error {
	if o.debugThreadMgr == nil {
		return fmt.Errorf("debug thread manager not initialized")
	}

	// 验证调试线程存在
	debugThread := o.debugThreadMgr.GetThread(threadID)
	if debugThread == nil {
		return fmt.Errorf("debug thread not found: %s", threadID)
	}

	// 检查线程状态
	if debugThread.Status == "running" {
		return fmt.Errorf("agent is still running, please wait")
	}

	// 获取最后一条Agent消息确定配置
	var lastConfigID uuid.UUID
	for i := len(debugThread.Messages) - 1; i >= 0; i-- {
		if debugThread.Messages[i].Role == "agent" && debugThread.Messages[i].AgentID != nil {
			lastConfigID = *debugThread.Messages[i].AgentID
			break
		}
	}

	if lastConfigID == uuid.Nil {
		return fmt.Errorf("no previous agent context found")
	}

	// 使用相同的配置继续执行
	req := &SpawnRequest{
		ThreadID:    threadID,
		ConfigID:    lastConfigID,
		Input:       message,
		ProjectPath: "",
	}

	_, err := o.SpawnDebugAgent(ctx, req)
	return err
}
```

- [ ] **Step 7: Verify code compiles**

Run: `cd isdp && go build ./internal/service/agent/...`
Expected: No compilation errors

- [ ] **Step 8: Write test for SpawnDebugAgent**

Create test file `isdp/internal/service/agent/orchestrator_debug_test.go`:

```go
package agent

import (
	"context"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestOrchestrator_SpawnDebugAgent_NilManager(t *testing.T) {
	o := &Orchestrator{}
	_, err := o.SpawnDebugAgent(context.Background(), &SpawnRequest{})
	if err == nil {
		t.Error("Expected error when debugThreadMgr is nil")
	}
}

func TestOrchestrator_SpawnDebugAgent_ThreadNotFound(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	o := &Orchestrator{debugThreadMgr: mgr}

	_, err := o.SpawnDebugAgent(context.Background(), &SpawnRequest{
		ThreadID: uuid.New(),
	})
	if err == nil {
		t.Error("Expected error when thread not found")
	}
}

func TestOrchestrator_ContinueDebugAgent_ThreadNotFound(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	o := &Orchestrator{debugThreadMgr: mgr}

	err := o.ContinueDebugAgent(context.Background(), uuid.New(), "test")
	if err == nil {
		t.Error("Expected error when thread not found")
	}
}

func TestOrchestrator_ContinueDebugAgent_ThreadRunning(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread()
	mgr.SetStatus(thread.ID, "running")

	o := &Orchestrator{debugThreadMgr: mgr}

	err := o.ContinueDebugAgent(context.Background(), thread.ID, "test")
	if err == nil {
		t.Error("Expected error when thread is running")
	}
}
```

- [ ] **Step 9: Run tests**

Run: `cd isdp && go test ./internal/service/agent/... -run TestOrchestrator -v`
Expected: All tests pass

- [ ] **Step 10: Commit**

```bash
cd isdp && git add internal/service/agent/orchestrator.go
git commit -m "feat(agent): add SpawnDebugAgent and ContinueDebugAgent to Orchestrator"
```

---

## Task 3: 修改AgentHandler注入DebugThreadManager

**Files:**
- Modify: `isdp/internal/api/agent_handler.go`

- [ ] **Step 1: Read current agent_handler.go**

Run: Read file `isdp/internal/api/agent_handler.go`
Expected: Understand current AgentHandler struct and methods

- [ ] **Step 2: Add debugThreadMgr field to AgentHandler**

Modify the AgentHandler struct:

```go
// AgentHandler Agent配置API处理器
type AgentHandler struct {
	configSvc      *agent.ConfigService
	baseAgentSvc   *agent.BaseAgentService
	orchestrator   *agent.Orchestrator
	threadRepo     *repo.ThreadRepository
	debugThreadMgr *agent.DebugThreadManager // 新增
}
```

- [ ] **Step 3: Update NewAgentHandler constructor**

```go
// NewAgentHandler 创建处理器
func NewAgentHandler(
	configSvc *agent.ConfigService,
	baseAgentSvc *agent.BaseAgentService,
	orchestrator *agent.Orchestrator,
	threadRepo *repo.ThreadRepository,
	debugThreadMgr *agent.DebugThreadManager, // 新增
) *AgentHandler {
	return &AgentHandler{
		configSvc:      configSvc,
		baseAgentSvc:   baseAgentSvc,
		orchestrator:   orchestrator,
		threadRepo:     threadRepo,
		debugThreadMgr: debugThreadMgr,
	}
}
```

- [ ] **Step 4: Simplify CreateDebugThread to use memory**

Replace the existing `CreateDebugThread` method:

```go
// CreateDebugThread 预创建调试Thread - 完全内存操作
func (h *AgentHandler) CreateDebugThread(c *gin.Context) {
	thread := h.debugThreadMgr.CreateThread()
	c.JSON(http.StatusOK, &CreateDebugThreadResponse{
		ThreadID: thread.ID.String(),
	})
}
```

- [ ] **Step 5: Update Debug method**

Replace the existing `Debug` method:

```go
// Debug 调试Agent - 启动交互式会话
func (h *AgentHandler) Debug(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req DebugRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取Agent配置
	config, err := h.configSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
		return
	}

	// 解析或创建调试线程
	var debugThreadID uuid.UUID
	if req.ThreadID != "" {
		debugThreadID, err = uuid.Parse(req.ThreadID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid threadId"})
			return
		}
		// 验证线程存在
		if h.debugThreadMgr.GetThread(debugThreadID) == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "debug thread not found"})
			return
		}
	} else {
		thread := h.debugThreadMgr.CreateThread()
		debugThreadID = thread.ID
	}

	// 启动Agent执行
	invocation, err := h.orchestrator.SpawnDebugAgent(c.Request.Context(), &agent.SpawnRequest{
		ThreadID:    debugThreadID,
		ConfigID:    config.ID,
		Role:        config.Role,
		Input:       req.Input,
		ProjectPath: req.ProjectPath,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, &DebugResponse{
		InvocationID: invocation.ID.String(),
		ThreadID:     debugThreadID.String(),
	})
}
```

- [ ] **Step 6: Update ContinueDebug method**

Replace the existing `ContinueDebug` method:

```go
// ContinueDebug 继续调试会话 - 发送消息到正在运行的会话
func (h *AgentHandler) ContinueDebug(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	// 验证是调试线程
	if h.debugThreadMgr.GetThread(threadID) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "debug thread not found"})
		return
	}

	var req ContinueDebugRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.orchestrator.ContinueDebugAgent(c.Request.Context(), threadID, req.Message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}
```

- [ ] **Step 7: Verify code compiles**

Run: `cd isdp && go build ./internal/api/...`
Expected: No compilation errors

- [ ] **Step 8: Commit**

```bash
cd isdp && git add internal/api/agent_handler.go
git commit -m "refactor(api): inject DebugThreadManager into AgentHandler"
```

---

## Task 4: 更新main.go初始化DebugThreadManager

**Files:**
- Modify: `isdp/cmd/server/main.go`

- [ ] **Step 1: Read current main.go initialization section**

Run: Read file `isdp/cmd/server/main.go` (lines 80-130)
Expected: Understand current initialization order

- [ ] **Step 2: Add DebugThreadManager initialization**

After the wsHub initialization (around line 82-84), add:

```go
// 初始化调试线程管理器
debugThreadMgr := agent.NewDebugThreadManager(wsHub)
```

- [ ] **Step 3: Update AgentHandler initialization**

Find the `agentHandler := api.NewAgentHandler(...)` line and add `debugThreadMgr`:

```go
agentHandler := api.NewAgentHandler(configService, baseAgentService, orchestrator, threadRepo, debugThreadMgr)
```

- [ ] **Step 4: Set DebugThreadManager in Orchestrator**

After orchestrator creation, add:

```go
// 在Orchestrator中设置调试管理器
orchestrator.SetDebugThreadManager(debugThreadMgr)
```

- [ ] **Step 5: Add defer for graceful shutdown**

Add near the top of the function where other defer statements are:

```go
// 优雅关闭调试线程管理器
defer debugThreadMgr.Stop()
```

- [ ] **Step 6: Verify code compiles and runs**

Run: `cd isdp && go build ./cmd/server/...`
Expected: No compilation errors

- [ ] **Step 7: Commit**

```bash
cd isdp && git add cmd/server/main.go
git commit -m "feat(server): initialize DebugThreadManager in main"
```

---

## Task 5: 创建前端类型定义

**Files:**
- Create: `isdp/web/src/types/index.ts`

- [ ] **Step 1: Check if types file already exists**

Run: `ls -la isdp/web/src/types/` or use Glob
Expected: Determine if file exists or needs creation

- [ ] **Step 2: Write types file**

```typescript
// isdp/web/src/types/index.ts

// 消息类型
export interface Message {
  id: string;
  threadId?: string;
  role: 'user' | 'agent' | 'system';
  agentId?: string;
  content: string;
  createdAt: Date;
  metadata?: Record<string, unknown>;
}

// Agent配置类型
export interface AgentConfig {
  id: string;
  name: string;
  role: string;
  description?: string;
  systemPrompt?: string;
  baseAgentId?: string;
  maxTokens: number;
  temperature: number;
  isDefault: boolean;
}

// WebSocket消息类型
export interface WSMessage {
  type: 'agent_output_chunk' | 'agent_message' | 'system_message' | 'sandbox_ready' | 'thread_expired';
  payload: Record<string, unknown>;
  threadId?: string;
  timestamp: number; // Unix timestamp from backend (int64)
}

// Agent输出块
export interface AgentOutputChunk {
  chunk: string;
  invocationId: string;
  agentId: string;
  agentName: string;
}

// Agent完整消息
export interface AgentMessage {
  messageId: string;
  agentId: string;
  agentName: string;
  agentRole: string;
  content: string;
}

// 系统消息
export interface SystemMessage {
  content: string;
  level?: 'info' | 'warning' | 'error';
}

// 沙箱就绪消息
export interface SandboxReady {
  url: string;
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd isdp/web && npx tsc --noEmit src/types/index.ts` (if tsc available)
Expected: No type errors

- [ ] **Step 4: Commit**

```bash
cd isdp && git add web/src/types/index.ts
git commit -m "feat(web): add TypeScript types for debug functionality"
```

---

## Task 6: 创建useWebSocket hook

**Files:**
- Create: `isdp/web/src/hooks/useWebSocket.ts`

- [ ] **Step 1: Check if hooks directory exists**

Run: `ls -la isdp/web/src/hooks/` or use Glob
Expected: Create directory if needed

- [ ] **Step 2: Write useWebSocket hook**

```typescript
// isdp/web/src/hooks/useWebSocket.ts
import { useEffect, useRef, useCallback } from 'react';
import { WSMessage } from '@/types';

interface UseWebSocketOptions {
  onMessage?: (data: WSMessage) => void;
  onConnect?: () => void;
  onDisconnect?: () => void;
  reconnectInterval?: number;
}

export function useWebSocket(
  threadId: string | null,
  options: UseWebSocketOptions = {}
) {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout>();
  const { onMessage, onConnect, onDisconnect, reconnectInterval = 3000 } = options;

  const connect = useCallback(() => {
    if (!threadId) return;

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/v1/ws?threadId=${threadId}`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('WebSocket connected');
      onConnect?.();
    };

    ws.onmessage = (event) => {
      try {
        const data: WSMessage = JSON.parse(event.data);
        onMessage?.(data);
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e);
      }
    };

    ws.onclose = () => {
      console.log('WebSocket disconnected');
      onDisconnect?.();
      // 自动重连
      reconnectTimeoutRef.current = setTimeout(connect, reconnectInterval);
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }, [threadId, onMessage, onConnect, onDisconnect, reconnectInterval]);

  useEffect(() => {
    connect();
    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      wsRef.current?.close();
    };
  }, [connect]);

  const sendMessage = useCallback((data: unknown) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data));
    }
  }, []);

  return {
    sendMessage,
    connected: wsRef.current?.readyState === WebSocket.OPEN,
  };
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd isdp/web && npx tsc --noEmit src/hooks/useWebSocket.ts` (if tsc available)
Expected: No type errors

- [ ] **Step 4: Commit**

```bash
cd isdp && git add web/src/hooks/useWebSocket.ts
git commit -m "feat(web): add useWebSocket hook for debug thread connection"
```

---

## Task 7: 创建调试专用Store

**Files:**
- Create: `isdp/web/src/store/debugThread.ts`

- [ ] **Step 1: Check if store directory exists**

Run: `ls -la isdp/web/src/store/` or use Glob
Expected: Create directory if needed

- [ ] **Step 2: Write debugThread store**

```typescript
// isdp/web/src/store/debugThread.ts
import { create } from 'zustand';
import { Message } from '@/types';

interface DebugThreadState {
  threadId: string | null;
  messages: Message[];
  streamingContent: string;
  status: 'idle' | 'running' | 'completed' | 'error';
  sandboxUrl: string | null;
  projectPath: string; // 新增：项目路径

  // Actions
  setThreadId: (id: string) => void;
  addMessage: (msg: Message) => void;
  appendStreamChunk: (chunk: string) => void;
  clearStreamContent: () => void;
  setStatus: (status: DebugThreadState['status']) => void;
  setSandboxUrl: (url: string | null) => void;
  setProjectPath: (path: string) => void; // 新增
  clearAll: () => void;
}

export const useDebugThreadStore = create<DebugThreadState>((set) => ({
  threadId: null,
  messages: [],
  streamingContent: '',
  status: 'idle',
  sandboxUrl: null,
  projectPath: '',

  setThreadId: (id) => set({ threadId: id }),
  addMessage: (msg) => set((state) => ({
    messages: [...state.messages, msg]
  })),
  appendStreamChunk: (chunk) => set((state) => ({
    streamingContent: state.streamingContent + chunk
  })),
  clearStreamContent: () => set({ streamingContent: '' }),
  setStatus: (status) => set({ status }),
  setSandboxUrl: (url) => set({ sandboxUrl: url }),
  setProjectPath: (path) => set({ projectPath: path }),
  clearAll: () => set({
    threadId: null,
    messages: [],
    streamingContent: '',
    status: 'idle',
    sandboxUrl: null,
    projectPath: '',
  }),
}));
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd isdp/web && npx tsc --noEmit src/store/debugThread.ts` (if tsc available)
Expected: No type errors

- [ ] **Step 4: Commit**

```bash
cd isdp && git add web/src/store/debugThread.ts
git commit -m "feat(web): add debugThread Zustand store for debug state management"
```

---

## Task 8: 创建MessageCard共享组件

**Files:**
- Create: `isdp/web/src/components/thread/MessageCard.tsx`
- Create: `isdp/web/src/components/thread/MessageCard.css`
- Create: `isdp/web/src/components/thread/index.ts`

- [ ] **Step 1: Create components directory**

Run: `mkdir -p isdp/web/src/components/thread`
Expected: Directory created

- [ ] **Step 2: Write MessageCard component**

```tsx
// isdp/web/src/components/thread/MessageCard.tsx
import React from 'react';
import { Message } from '@/types';
import './MessageCard.css';

interface MessageCardProps {
  message: Message;
  isStreaming?: boolean;
  agentName?: string;
  agentRole?: string;
}

export const MessageCard: React.FC<MessageCardProps> = ({
  message,
  isStreaming,
  agentName,
  agentRole,
}) => {
  const isUser = message.role === 'user';

  return (
    <div className={`message-card ${isUser ? 'message-user' : 'message-agent'}`}>
      {!isUser && (
        <div className="message-header">
          <span className="agent-name">{agentName || 'Agent'}</span>
          {agentRole && <span className="agent-role">{agentRole}</span>}
        </div>
      )}
      <div className="message-content">
        {message.content}
        {isStreaming && <span className="streaming-cursor">▊</span>}
      </div>
    </div>
  );
};
```

- [ ] **Step 3: Write MessageCard CSS**

```css
/* isdp/web/src/components/thread/MessageCard.css */
.message-card {
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 12px;
  max-width: 80%;
}

.message-user {
  background-color: #007bff;
  color: white;
  margin-left: auto;
  text-align: right;
}

.message-agent {
  background-color: #f1f3f4;
  color: #333;
  margin-right: auto;
}

.message-header {
  display: flex;
  gap: 8px;
  margin-bottom: 4px;
  font-size: 0.85em;
}

.agent-name {
  font-weight: 600;
}

.agent-role {
  color: #666;
}

.message-content {
  white-space: pre-wrap;
  word-break: break-word;
}

.streaming-cursor {
  animation: blink 1s infinite;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}
```

- [ ] **Step 4: Commit**

```bash
cd isdp && git add web/src/components/thread/MessageCard.tsx web/src/components/thread/MessageCard.css
git commit -m "feat(web): add MessageCard shared component"
```

---

## Task 9: 创建MessageInput共享组件

**Files:**
- Create: `isdp/web/src/components/thread/MessageInput.tsx`
- Create: `isdp/web/src/components/thread/MessageInput.css`

- [ ] **Step 1: Write MessageInput component**

```tsx
// isdp/web/src/components/thread/MessageInput.tsx
import React, { useState, useRef } from 'react';
import { AgentConfig } from '@/types';
import './MessageInput.css';

interface MessageInputProps {
  onSend: (message: string) => void;
  disabled?: boolean;
  placeholder?: string;
  agents?: AgentConfig[];
}

export const MessageInput: React.FC<MessageInputProps> = ({
  onSend,
  disabled,
  placeholder = '输入消息...',
  agents = [],
}) => {
  const [input, setInput] = useState('');
  const [showMentions, setShowMentions] = useState(false);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const handleSend = () => {
    if (input.trim() && !disabled) {
      onSend(input.trim());
      setInput('');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleMentionSelect = (agent: AgentConfig) => {
    setInput(prev => prev + `@${agent.name} `);
    setShowMentions(false);
    inputRef.current?.focus();
  };

  return (
    <div className="message-input-container">
      <div className="input-wrapper">
        <textarea
          ref={inputRef}
          value={input}
          onChange={(e) => {
            setInput(e.target.value);
            if (e.target.value.endsWith('@')) {
              setShowMentions(true);
            }
          }}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={disabled}
          rows={1}
        />
        {showMentions && agents.length > 0 && (
          <div className="mention-dropdown">
            {agents.map(agent => (
              <div
                key={agent.id}
                className="mention-item"
                onClick={() => handleMentionSelect(agent)}
              >
                <span className="mention-name">{agent.name}</span>
                <span className="mention-role">{agent.role}</span>
              </div>
            ))}
          </div>
        )}
      </div>
      <button onClick={handleSend} disabled={disabled || !input.trim()}>
        发送
      </button>
    </div>
  );
};
```

- [ ] **Step 2: Write MessageInput CSS**

```css
/* isdp/web/src/components/thread/MessageInput.css */
.message-input-container {
  display: flex;
  gap: 8px;
  padding: 12px;
  background: #fff;
  border-top: 1px solid #e0e0e0;
}

.input-wrapper {
  flex: 1;
  position: relative;
}

.input-wrapper textarea {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #ddd;
  border-radius: 8px;
  resize: none;
  font-size: 14px;
}

.input-wrapper textarea:focus {
  outline: none;
  border-color: #007bff;
}

.mention-dropdown {
  position: absolute;
  bottom: 100%;
  left: 0;
  background: white;
  border: 1px solid #ddd;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
  max-height: 200px;
  overflow-y: auto;
}

.mention-item {
  padding: 8px 12px;
  cursor: pointer;
  display: flex;
  gap: 8px;
}

.mention-item:hover {
  background: #f5f5f5;
}

.mention-name {
  font-weight: 500;
}

.mention-role {
  color: #666;
  font-size: 0.9em;
}

.message-input-container button {
  padding: 10px 20px;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
}

.message-input-container button:disabled {
  background: #ccc;
  cursor: not-allowed;
}
```

- [ ] **Step 3: Commit**

```bash
cd isdp && git add web/src/components/thread/MessageInput.tsx web/src/components/thread/MessageInput.css
git commit -m "feat(web): add MessageInput shared component with @mention support"
```

---

## Task 10: 创建SandboxPanel共享组件

**Files:**
- Create: `isdp/web/src/components/thread/SandboxPanel.tsx`
- Create: `isdp/web/src/components/thread/SandboxPanel.css`

- [ ] **Step 1: Write SandboxPanel component**

```tsx
// isdp/web/src/components/thread/SandboxPanel.tsx
import React from 'react';
import './SandboxPanel.css';

interface SandboxPanelProps {
  url?: string;
  visible: boolean;
  onClose: () => void;
}

export const SandboxPanel: React.FC<SandboxPanelProps> = ({
  url,
  visible,
  onClose,
}) => {
  if (!visible) return null;

  return (
    <div className="sandbox-panel">
      <div className="sandbox-header">
        <span>沙箱预览</span>
        <button onClick={onClose}>收起 ▼</button>
      </div>
      <div className="sandbox-content">
        {url ? (
          <iframe src={url} title="Sandbox Preview" />
        ) : (
          <div className="sandbox-empty">
            暂无沙箱运行
          </div>
        )}
      </div>
    </div>
  );
};
```

- [ ] **Step 2: Write SandboxPanel CSS**

```css
/* isdp/web/src/components/thread/SandboxPanel.css */
.sandbox-panel {
  position: fixed;
  bottom: 0;
  right: 0;
  width: 50%;
  height: 50%;
  background: #fff;
  border-left: 1px solid #e0e0e0;
  border-top: 1px solid #e0e0e0;
  display: flex;
  flex-direction: column;
  z-index: 1000;
}

.sandbox-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f5f5f5;
  border-bottom: 1px solid #e0e0e0;
}

.sandbox-header button {
  padding: 4px 12px;
  background: transparent;
  border: 1px solid #ddd;
  border-radius: 4px;
  cursor: pointer;
}

.sandbox-content {
  flex: 1;
  position: relative;
}

.sandbox-content iframe {
  width: 100%;
  height: 100%;
  border: none;
}

.sandbox-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #999;
}
```

- [ ] **Step 3: Commit**

```bash
cd isdp && git add web/src/components/thread/SandboxPanel.tsx web/src/components/thread/SandboxPanel.css
git commit -m "feat(web): add SandboxPanel shared component"
```

---

## Task 11: 创建组件导出文件

**Files:**
- Create: `isdp/web/src/components/thread/index.ts`

- [ ] **Step 1: Write index.ts**

```typescript
// isdp/web/src/components/thread/index.ts
export { MessageCard } from './MessageCard';
export { MessageInput } from './MessageInput';
export { SandboxPanel } from './SandboxPanel';
```

- [ ] **Step 2: Commit**

```bash
cd isdp && git add web/src/components/thread/index.ts
git commit -m "feat(web): add thread components barrel export"
```

---

## Task 12: 重构AgentDebug页面

**Files:**
- Modify: `isdp/web/src/pages/AgentDebug.tsx`

- [ ] **Step 1: Read current AgentDebug.tsx**

Run: Read file `isdp/web/src/pages/AgentDebug.tsx`
Expected: Understand current implementation

- [ ] **Step 2: Rewrite AgentDebug page**

```tsx
// isdp/web/src/pages/AgentDebug.tsx
import React, { useEffect, useState } from 'react';
import { MessageCard, MessageInput, SandboxPanel } from '@/components/thread';
import { useDebugThreadStore } from '@/store/debugThread';
import { useWebSocket } from '@/hooks/useWebSocket';
import { agentsApi } from '@/api/agents';
import { AgentConfig } from '@/types';
import './AgentDebug.css';

export const AgentDebug: React.FC = () => {
  const [selectedAgentId, setSelectedAgentId] = useState<string>();
  const [sandboxVisible, setSandboxVisible] = useState(false);
  const [agents, setAgents] = useState<AgentConfig[]>([]);

  const {
    threadId,
    messages,
    streamingContent,
    status,
    sandboxUrl,
    projectPath,
    setThreadId,
    addMessage,
    appendStreamChunk,
    clearStreamContent,
    setStatus,
    setSandboxUrl,
    setProjectPath,
    clearAll,
  } = useDebugThreadStore();

  // WebSocket连接
  useWebSocket(threadId, {
    onMessage: (data) => {
      switch (data.type) {
        case 'agent_output_chunk':
          appendStreamChunk(data.payload.chunk as string);
          break;
        case 'agent_message':
          clearStreamContent();
          addMessage({
            id: data.payload.messageId as string,
            role: 'agent',
            agentId: data.payload.agentId as string,
            content: data.payload.content as string,
            createdAt: new Date(data.timestamp * 1000), // Unix timestamp to Date
          });
          setStatus('idle');
          break;
        case 'system_message':
          addMessage({
            id: Date.now().toString(),
            role: 'system',
            content: data.payload.content as string,
            createdAt: new Date(),
          });
          break;
        case 'sandbox_ready':
          setSandboxUrl(data.payload.url as string);
          break;
        case 'thread_expired':
          clearAll();
          alert('调试会话已过期，请重新开始');
          break;
      }
    },
  });

  // 加载Agent列表
  useEffect(() => {
    agentsApi.list().then(setAgents).catch(console.error);
  }, []);

  // 创建调试线程
  useEffect(() => {
    agentsApi.createDebugThread().then(({ threadId }) => {
      setThreadId(threadId);
    }).catch(console.error);
  }, []);

  const handleSend = async (message: string) => {
    if (!selectedAgentId || !threadId) return;

    // 添加用户消息
    addMessage({
      id: Date.now().toString(),
      role: 'user',
      content: message,
      createdAt: new Date(),
    });

    setStatus('running');

    // 发送到后端
    try {
      await agentsApi.debug(selectedAgentId, {
        input: message,
        threadId,
        projectPath: projectPath || undefined, // 传递项目路径
      });
    } catch (error) {
      console.error('Failed to send debug request:', error);
      setStatus('error');
    }
  };

  const handleClear = () => {
    clearAll();
    // 重新创建线程
    agentsApi.createDebugThread().then(({ threadId }) => {
      setThreadId(threadId);
    }).catch(console.error);
  };

  return (
    <div className="agent-debug">
      <div className="debug-header">
        <h2>Agent调试</h2>
        <select
          value={selectedAgentId || ''}
          onChange={(e) => setSelectedAgentId(e.target.value)}
        >
          <option value="">选择Agent</option>
          {agents.map(agent => (
            <option key={agent.id} value={agent.id}>
              {agent.name} ({agent.role})
            </option>
          ))}
        </select>
        <input
          type="text"
          placeholder="项目路径（可选）"
          value={projectPath}
          onChange={(e) => setProjectPath(e.target.value)}
          className="project-path-input"
        />
        <button onClick={handleClear}>清空对话</button>
        <button onClick={() => setSandboxVisible(!sandboxVisible)}>
          {sandboxVisible ? '隐藏沙箱' : '沙箱预览'}
        </button>
      </div>

      <div className="debug-content">
        <div className="messages-area">
          {messages.map((msg, idx) => (
            <MessageCard
              key={idx}
              message={msg}
              agentName={msg.agentId}
            />
          ))}
          {streamingContent && (
            <MessageCard
              message={{
                id: 'streaming',
                role: 'agent',
                content: streamingContent,
                createdAt: new Date(),
              }}
              isStreaming
            />
          )}
        </div>

        <SandboxPanel
          url={sandboxUrl || undefined}
          visible={sandboxVisible}
          onClose={() => setSandboxVisible(false)}
        />
      </div>

      <div className="debug-input">
        <MessageInput
          onSend={handleSend}
          disabled={status === 'running' || !selectedAgentId}
          agents={agents}
          placeholder="输入消息调试Agent... 使用 @ 提及特定Agent"
        />
      </div>
    </div>
  );
};
```

- [ ] **Step 3: Write/update AgentDebug.css**

```css
/* isdp/web/src/pages/AgentDebug.css */
.agent-debug {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.debug-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: #f5f5f5;
  border-bottom: 1px solid #e0e0e0;
}

.debug-header h2 {
  margin: 0;
}

.debug-header select {
  padding: 6px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
}

.debug-header button {
  padding: 6px 12px;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.debug-header .project-path-input {
  flex: 1;
  max-width: 300px;
  padding: 6px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
}

.debug-content {
  flex: 1;
  display: flex;
  overflow: hidden;
  position: relative;
}

.messages-area {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.debug-input {
  background: #fff;
}
```

- [ ] **Step 4: Commit**

```bash
cd isdp && git add web/src/pages/AgentDebug.tsx web/src/pages/AgentDebug.css
git commit -m "refactor(web): rewrite AgentDebug page with shared components"
```

---

## Task 13: 更新ThreadView页面使用共享组件

**Files:**
- Modify: `isdp/web/src/pages/ThreadView.tsx`

- [ ] **Step 1: Read current ThreadView.tsx**

Run: Read file `isdp/web/src/pages/ThreadView.tsx`
Expected: Understand current implementation and message rendering

- [ ] **Step 2: Add shared component imports**

Add imports at the top of the file:

```typescript
import { MessageCard, MessageInput, SandboxPanel } from '@/components/thread';
```

- [ ] **Step 3: Add sandbox state**

Add state variables for sandbox functionality:

```typescript
const [sandboxVisible, setSandboxVisible] = useState(false);
const [sandboxUrl, setSandboxUrl] = useState<string>();
```

- [ ] **Step 4: Add sandbox toggle button to header**

Find the header section and add a sandbox toggle button:

```tsx
<button onClick={() => setSandboxVisible(!sandboxVisible)}>
  {sandboxVisible ? '隐藏沙箱' : '沙箱预览'}
</button>
```

- [ ] **Step 5: Add SandboxPanel component**

Add SandboxPanel near the messages area:

```tsx
<SandboxPanel
  url={sandboxUrl}
  visible={sandboxVisible}
  onClose={() => setSandboxVisible(false)}
/>
```

- [ ] **Step 6: Handle sandbox_ready WebSocket message**

In the WebSocket message handler, add case for sandbox_ready:

```typescript
case 'sandbox_ready':
  setSandboxUrl(data.payload.url as string);
  break;
```

- [ ] **Step 7: Replace custom message rendering with MessageCard**

Replace existing message rendering with MessageCard component:

```tsx
{messages.map((msg, idx) => (
  <MessageCard
    key={idx}
    message={msg}
    agentName={msg.agentId}
  />
))}
```

- [ ] **Step 8: Verify TypeScript compiles**

Run: `cd isdp/web && npx tsc --noEmit src/pages/ThreadView.tsx` (if tsc available)
Expected: No type errors

- [ ] **Step 9: Commit**

```bash
cd isdp && git add web/src/pages/ThreadView.tsx
git commit -m "refactor(web): update ThreadView to use shared components"
```

---

## Task 14: 更新agents API

**Files:**
- Modify: `isdp/web/src/api/agents.ts`

- [ ] **Step 1: Read current agents.ts**

Run: Read file `isdp/web/src/api/agents.ts`
Expected: Understand current API methods

- [ ] **Step 2: Add debug API methods**

Add the following methods to the agentsApi object:

```typescript
// Add to the agentsApi object

createDebugThread: async (): Promise<{ threadId: string }> => {
  const response = await fetch('/api/v1/agents/debug/thread', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({}),
  });
  if (!response.ok) {
    const errData = await response.json().catch(() => ({}));
    throw new Error(errData.error || 'Failed to create debug thread');
  }
  return response.json();
},

debug: async (agentId: string, data: { input: string; threadId: string; projectPath?: string }): Promise<{ invocationId: string; threadId: string }> => {
  const response = await fetch(`/api/v1/agents/${agentId}/debug`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!response.ok) {
    const errData = await response.json().catch(() => ({}));
    throw new Error(errData.error || 'Failed to start debug');
  }
  return response.json();
},

continueDebug: async (threadId: string, message: string): Promise<void> => {
  const response = await fetch(`/api/v1/agents/debug/${threadId}/continue`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message }),
  });
  if (!response.ok) {
    const errData = await response.json().catch(() => ({}));
    throw new Error(errData.error || 'Failed to continue debug');
  }
},
```

- [ ] **Step 3: Commit**

```bash
cd isdp && git add web/src/api/agents.ts
git commit -m "feat(web): add debug API methods to agentsApi"
```

---

## Task 15: 集成测试与验证

**Files:**
- None (verification only)

- [ ] **Step 1: Build backend**

Run: `cd isdp && go build ./cmd/server/...`
Expected: Build succeeds

- [ ] **Step 2: Run backend tests**

Run: `cd isdp && go test ./internal/service/agent/... -v`
Expected: All tests pass

- [ ] **Step 3: Build frontend**

Run: `cd isdp/web && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Start server manually and test debug flow**

Run: `cd isdp && go run ./cmd/server/...`
Expected: Server starts without errors

Manual test steps:
1. Open browser to Agent Debug page
2. Select an Agent
3. Send a message
4. Verify WebSocket connection established
5. Verify message appears in conversation
6. Verify streaming output works
7. Test sandbox toggle button

- [ ] **Step 5: Final commit if needed**

```bash
cd isdp && git add -A && git commit -m "chore: final integration adjustments"
```

---

## 完成检查清单

- [ ] 后端: DebugThreadManager测试通过
- [ ] 后端: Orchestrator编译无错误
- [ ] 后端: AgentHandler编译无错误
- [ ] 后端: main.go编译无错误
- [ ] 前端: TypeScript类型编译通过
- [ ] 前端: 所有组件编译通过
- [ ] 前端: 页面正常运行
- [ ] 集成: 调试流程端到端可用