# Agent调试功能重构设计

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复调试功能的500错误，并将调试页面改造为与工作流执行一致的对话形式，实现调试与正式执行的后端隔离但前端体验统一。

**Architecture:** 后端使用内存管理调试线程，与数据库完全隔离；前端提取共享UI组件，调试页面和正式执行页面复用相同组件但使用不同的状态管理。

**Tech Stack:** Go (Gin), React, TypeScript, Zustand, WebSocket, SQLite

---

## 背景与问题

### 当前问题

1. **调试功能500错误**：重构后`CreateDebugThread`方法尝试将调试线程关联到不存在的project_id，导致外键约束失败
2. **前端体验不一致**：调试页面使用简单的文本输出，而正式执行使用对话卡片形式，两者差异大
3. **代码耦合**：调试逻辑与正式执行逻辑混在一起，难以独立维护

### 设计目标

1. 调试线程不需要真实项目，完全在内存中管理
2. 调试页面UI与工作流执行页面保持一致
3. 支持完整的调试功能：对话、@mention、产物展示、沙箱预览
4. 前后端逻辑统一，调试与真实执行更接近

---

## 架构设计

### 后端架构

```
┌─────────────────────────────────────────────────────────┐
│                      Orchestrator                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│   ┌─────────────────────┐   ┌─────────────────────┐    │
│   │ ExecutionService    │   │ DebugThreadManager  │    │
│   │ (正式执行)           │   │ (调试专用)           │    │
│   │                     │   │                     │    │
│   │ - 操作数据库         │   │ - 内存map管理        │    │
│   │ - Thread表          │   │ - 无数据库依赖        │    │
│   │ - AgentInvocation表 │   │ - 临时会话          │    │
│   └─────────────────────┘   └─────────────────────┘    │
│              │                        │                 │
│              └────────────┬───────────┘                 │
│                           │                             │
│                    ┌──────▼──────┐                      │
│                    │  ws.Hub     │                      │
│                    │ (共享)       │                      │
│                    └─────────────┘                      │
└─────────────────────────────────────────────────────────┘
```

### 前端架构

```
┌─────────────────────────────────────────────────────────┐
│                    共享UI组件                            │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐          │
│  │MessageCard │ │MessageInput│ │SandboxPanel│ ...      │
│  └────────────┘ └────────────┘ └────────────┘          │
└─────────────────────────────────────────────────────────┘
              ▲                    ▲
              │                    │
    ┌─────────┴────────┐  ┌───────┴─────────┐
    │   ThreadView     │  │   AgentDebug    │
    │   (正式执行)      │  │   (调试页面)     │
    │                  │  │                 │
    │ - 数据库消息      │  │ - 内存消息       │
    │ - workflow store  │  │ - debug store   │
    └──────────────────┘  └─────────────────┘
```

---

## 后端实现

### 1. DebugThreadManager

**文件:** `internal/service/agent/debug_thread_manager.go`

```go
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
    threads map[uuid.UUID]*DebugThread
    mu      sync.RWMutex
    wsHub   *ws.Hub
}

// NewDebugThreadManager 创建调试线程管理器
func NewDebugThreadManager(wsHub *ws.Hub) *DebugThreadManager {
    return &DebugThreadManager{
        threads: make(map[uuid.UUID]*DebugThread),
        wsHub:   wsHub,
    }
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
```

### 2. 修改AgentHandler

**文件:** `internal/api/agent_handler.go`

修改要点：
- 注入`DebugThreadManager`
- `CreateDebugThread`不再操作数据库
- `Debug`方法使用内存线程

```go
// AgentHandler 添加字段
type AgentHandler struct {
    configSvc         *agent.ConfigService
    baseAgentSvc      *agent.BaseAgentService
    orchestrator      *agent.Orchestrator
    threadRepo        *repo.ThreadRepository
    debugThreadMgr    *agent.DebugThreadManager  // 新增
}

// NewAgentHandler 更新构造函数
func NewAgentHandler(
    configSvc *agent.ConfigService,
    baseAgentSvc *agent.BaseAgentService,
    orchestrator *agent.Orchestrator,
    threadRepo *repo.ThreadRepository,
    debugThreadMgr *agent.DebugThreadManager,  // 新增
) *AgentHandler {
    return &AgentHandler{
        configSvc:      configSvc,
        baseAgentSvc:   baseAgentSvc,
        orchestrator:   orchestrator,
        threadRepo:     threadRepo,
        debugThreadMgr: debugThreadMgr,
    }
}

// CreateDebugThread 简化为内存操作
func (h *AgentHandler) CreateDebugThread(c *gin.Context) {
    thread := h.debugThreadMgr.CreateThread()
    c.JSON(http.StatusOK, &CreateDebugThreadResponse{
        ThreadID: thread.ID.String(),
    })
}

// Debug 更新为使用内存线程
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

    // 启动Agent执行（传入标记表示调试模式）
    invocation, err := h.orchestrator.SpawnDebugAgent(c.Request.Context(), &agent.SpawnRequest{
        ThreadID:    debugThreadID,
        ConfigID:    config.ID,
        Role:        config.Role,
        Input:       req.Input,
        ProjectPath: req.ProjectPath,
        IsDebug:     true,  // 新增标记
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

// ContinueDebug 继续调试
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

### 3. Orchestrator添加调试支持

**文件:** `internal/service/agent/orchestrator.go`

添加调试专用方法和字段：

```go
type Orchestrator struct {
    // ... 现有字段
    debugThreadMgr *DebugThreadManager  // 新增
}

// SpawnDebugAgent 调试模式启动Agent
func (o *Orchestrator) SpawnDebugAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
    // 类似SpawnAgent，但消息存储到内存而非数据库
    // 广播使用相同的wsHub
}

// ContinueDebugAgent 继续调试会话
func (o *Orchestrator) ContinueDebugAgent(ctx context.Context, threadID uuid.UUID, message string) error {
    // 添加用户消息到内存
    // 触发Agent响应
}
```

### 4. main.go初始化

**文件:** `cmd/server/main.go`

```go
// 初始化调试线程管理器
debugThreadMgr := agent.NewDebugThreadManager(wsHub)

// 更新AgentHandler初始化
agentHandler := api.NewAgentHandler(configService, baseAgentService, orchestrator, threadRepo, debugThreadMgr)
```

---

## 前端实现

### 1. 共享UI组件

#### MessageCard组件

**文件:** `src/components/thread/MessageCard.tsx`

```tsx
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

#### MessageInput组件

**文件:** `src/components/thread/MessageInput.tsx`

```tsx
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
            // 检测@符号触发mention列表
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

#### SandboxPanel组件

**文件:** `src/components/thread/SandboxPanel.tsx`

```tsx
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

#### 组件导出

**文件:** `src/components/thread/index.ts`

```ts
export { MessageCard } from './MessageCard';
export { MessageInput } from './MessageInput';
export { SandboxPanel } from './SandboxPanel';
```

### 2. 调试专用Store

**文件:** `src/store/debugThread.ts`

```ts
import { create } from 'zustand';
import { Message } from '@/types';

interface DebugThreadState {
  threadId: string | null;
  messages: Message[];
  streamingContent: string;
  status: 'idle' | 'running' | 'completed' | 'error';
  sandboxUrl: string | null;

  // Actions
  setThreadId: (id: string) => void;
  addMessage: (msg: Message) => void;
  appendStreamChunk: (chunk: string) => void;
  clearStreamContent: () => void;
  setStatus: (status: DebugThreadState['status']) => void;
  setSandboxUrl: (url: string | null) => void;
  clearAll: () => void;
}

export const useDebugThreadStore = create<DebugThreadState>((set) => ({
  threadId: null,
  messages: [],
  streamingContent: '',
  status: 'idle',
  sandboxUrl: null,

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
  clearAll: () => set({
    threadId: null,
    messages: [],
    streamingContent: '',
    status: 'idle',
    sandboxUrl: null,
  }),
}));
```

### 3. 重构AgentDebug页面

**文件:** `src/pages/AgentDebug.tsx`

```tsx
import React, { useEffect, useState } from 'react';
import { MessageCard, MessageInput, SandboxPanel } from '@/components/thread';
import { useDebugThreadStore } from '@/store/debugThread';
import { useWebSocket } from '@/hooks/useWebSocket';
import { agentsApi } from '@/api/agents';
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
    setThreadId,
    addMessage,
    appendStreamChunk,
    clearStreamContent,
    setStatus,
    setSandboxUrl,
    clearAll,
  } = useDebugThreadStore();

  // WebSocket连接
  const { sendMessage, connected } = useWebSocket(threadId, {
    onMessage: (data) => {
      switch (data.type) {
        case 'agent_output_chunk':
          appendStreamChunk(data.payload.chunk);
          break;
        case 'agent_message':
          clearStreamContent();
          addMessage({
            id: data.payload.messageId,
            role: 'agent',
            agentId: data.payload.agentId,
            content: data.payload.content,
            createdAt: new Date(data.timestamp),
          });
          setStatus('idle');
          break;
        case 'system_message':
          addMessage({
            id: Date.now().toString(),
            role: 'system',
            content: data.payload.content,
            createdAt: new Date(),
          });
          break;
        case 'sandbox_ready':
          setSandboxUrl(data.payload.url);
          break;
      }
    },
  });

  // 加载Agent列表
  useEffect(() => {
    agentsApi.list().then(setAgents);
  }, []);

  // 创建调试线程
  useEffect(() => {
    agentsApi.createDebugThread().then(({ threadId }) => {
      setThreadId(threadId);
    });
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
    await agentsApi.debug(selectedAgentId, {
      input: message,
      threadId,
    });
  };

  const handleClear = () => {
    clearAll();
    // 重新创建线程
    agentsApi.createDebugThread().then(({ threadId }) => {
      setThreadId(threadId);
    });
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

### 4. 更新ThreadView页面

**文件:** `src/pages/ThreadView.tsx`

修改要点：
- 导入共享组件
- 添加沙箱预览按钮和面板
- 保持现有的数据库交互逻辑

```tsx
// 导入共享组件
import { MessageCard, MessageInput, SandboxPanel } from '@/components/thread';

// 添加沙箱状态
const [sandboxVisible, setSandboxVisible] = useState(false);
const [sandboxUrl, setSandboxUrl] = useState<string>();

// 在header添加沙箱按钮
<button onClick={() => setSandboxVisible(!sandboxVisible)}>
  {sandboxVisible ? '隐藏沙箱' : '沙箱预览'}
</button>

// 在消息区域旁添加SandboxPanel
<SandboxPanel
  url={sandboxUrl}
  visible={sandboxVisible}
  onClose={() => setSandboxVisible(false)}
/>

// WebSocket消息处理中添加sandbox_ready
case 'sandbox_ready':
  setSandboxUrl(data.payload.url);
  break;
```

---

## API接口

### 新增/修改接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/agents/debug/thread` | 创建内存调试线程 |
| POST | `/api/v1/agents/:id/debug` | 启动调试Agent |
| POST | `/api/v1/agents/debug/:threadId/continue` | 继续调试对话 |

### 响应格式

**创建调试线程：**
```json
{
  "threadId": "uuid-string"
}
```

**启动调试：**
```json
{
  "invocationId": "uuid-string",
  "threadId": "uuid-string"
}
```

---

## WebSocket消息类型

所有消息类型在调试和正式执行中保持一致：

| 类型 | 说明 | Payload |
|------|------|---------|
| `agent_output_chunk` | 流式输出块 | `{ chunk, invocationId, agentId, agentName }` |
| `agent_message` | 完整Agent消息 | `{ messageId, agentId, content, agentName, agentRole }` |
| `system_message` | 系统消息 | `{ content }` |
| `sandbox_ready` | 沙箱就绪 | `{ url }` |

---

## 文件变更清单

### 后端新增文件
- `internal/service/agent/debug_thread_manager.go`

### 后端修改文件
- `internal/api/agent_handler.go` - 注入DebugThreadManager，修改调试方法
- `internal/service/agent/orchestrator.go` - 添加调试支持方法
- `cmd/server/main.go` - 初始化DebugThreadManager

### 前端新增文件
- `src/components/thread/MessageCard.tsx`
- `src/components/thread/MessageCard.css`
- `src/components/thread/MessageInput.tsx`
- `src/components/thread/MessageInput.css`
- `src/components/thread/SandboxPanel.tsx`
- `src/components/thread/SandboxPanel.css`
- `src/components/thread/index.ts`
- `src/store/debugThread.ts`

### 前端修改文件
- `src/pages/AgentDebug.tsx` - 重构为对话形式
- `src/pages/ThreadView.tsx` - 使用共享组件，添加沙箱预览
- `src/api/agents.ts` - 添加调试相关API方法（如有需要）

---

## 测试要点

### 后端测试
1. 调试线程创建不依赖数据库
2. WebSocket消息正确广播到调试线程
3. 多个调试线程并发隔离
4. 调试线程内存正确清理

### 前端测试
1. 调试页面消息正确显示
2. @mention功能正常工作
3. 沙箱面板正确显示/隐藏
4. 页面刷新后调试数据清空
5. 流式输出正确追加显示