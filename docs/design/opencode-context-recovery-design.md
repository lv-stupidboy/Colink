# OpenCode 上下文记忆恢复方案设计

## 问题背景

OpenCode ACP Agent 在多轮对话后无法记住之前的上下文（如问题单号），原因：
1. 每次 `ExecuteWithStream` 启动新进程，进程退出后 CLI 内部上下文丢失
2. Layer1 只提取摘要（关键结论、文件引用），而非完整对话
3. ACP 协议不支持 `--resume` 真正的 session 恢复

## 设计方案：长连接 + 定期持久化 + Prompt 注入恢复

### 核心思路

```
┌─────────────────────────────────────────────────────────────────┐
│                      ExecutionService                            │
│  ┌────────────────┐    ┌────────────────┐    ┌────────────────┐ │
│  │ LongRunning    │    │ SessionState   │    │ History        │ │
│  │ SessionManager │───▶│ Persister      │───▶│ Compressor     │ │
│  │ (保持进程存活)  │    │ (定期持久化)    │    │ (Token 预算)   │ │
│  └────────────────┘    └────────────────┘    └────────────────┘ │
│           │                    │                    │            │
│           ▼                    ▼                    ▼            │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                 ACP Session Pool                           │ │
│  │  session-1: OpenCode process (active)                      │ │
│  │  session-2: OpenCode process (idle, 5min timeout)          │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### Phase 1: 长连接 Session 管理

**目标**：保持 OpenCode 进程存活，避免每次都重新启动

**改动点**：

1. **Session Pool 设计**（`internal/service/agent/session_pool.go`）

```go
type SessionPool struct {
    sessions    map[string]*LongRunningSession  // threadID:agentID -> session
    idleTimeout time.Duration                   // 空闲超时（默认 10 分钟）
    maxSessions int                             // 最大并发 session 数
    mu          sync.RWMutex
}

type LongRunningSession struct {
    ID           string
    ThreadID     string
    AgentID      string
    Process      *exec.Cmd
    Transport    *acpTransport
    LastActiveAt time.Time
    Status       SessionStatus  // active, idle, sealed
    Conversation *ConversationBuffer  // 对话累积
}
```

2. **修改 ACP Adapter**（`plugins/acp/adapter_base.go`）

```go
// 新增：StartOrResumeSession 方法
func (a *BaseACPAdapter) StartOrResumeSession(ctx context.Context, sessionKey string, req *agent.ExecutionRequest) (*LongRunningSession, error) {
    // 1. 检查是否已有活跃 session
    session := a.sessionPool.Get(sessionKey)
    if session != nil && session.Status == SessionStatusActive {
        return session, nil  // 直接复用
    }

    // 2. 无活跃 session，创建新进程
    session = a.startNewProcess(ctx, req)
    a.sessionPool.Register(sessionKey, session)
    return session, nil
}

// 新增：SendPrompt 方法（复用已有 session）
func (a *BaseACPAdapter) SendPrompt(session *LongRunningSession, prompt string, onChunk func(Chunk)) error {
    return session.Transport.SendRequest("session/prompt", &acpPromptParams{
        SessionID: session.AcpSessionID,
        Prompt:    []acpContentBlock{{Type: "text", Text: prompt}},
    })
}
```

### Phase 2: 对话历史持久化

**目标**：定期持久化对话内容，支持断连后恢复

**改动点**：

1. **ConversationBuffer 设计**

```go
type ConversationBuffer struct {
    Turns []ConversationTurn  // 对话回合列表
    mu    sync.RWMutex
}

type ConversationTurn struct {
    Role      string    // "user" 或 "agent"
    Content   string    // 消息内容
    Timestamp time.Time
    TokenCount int      // Token 计数（用于预算控制）
    Metadata  map[string]string  // agentID, toolCalls 等
}
```

2. **定期持久化机制**

```go
// 每 N 次对话后自动持久化（或在空闲时持久化）
func (s *LongRunningSession) PersistConversation() error {
    // 1. 序列化 ConversationBuffer
    data, _ := json.Marshal(s.Conversation)

    // 2. 写入数据库（messages 表或新增 session_conversations 表）
    return s.repo.SaveSessionConversation(s.ThreadID, s.AgentID, data)
}
```

### Phase 3: 断连恢复（Prompt 注入）

**目标**：进程意外断连时，通过 prompt 注入恢复上下文

**改动点**：

1. **恢复逻辑**

```go
func (a *BaseACPAdapter) RecoverSession(ctx context.Context, sessionKey string, req *agent.ExecutionRequest) (*LongRunningSession, error) {
    // 1. 从数据库加载持久化的对话历史
    history, err := a.repo.LoadSessionConversation(req.ThreadID, req.AgentID)
    if err != nil {
        return nil, err  // 无历史，启动新 session
    }

    // 2. 解压历史（Token 预算控制）
    compressedHistory := a.compressor.Compress(history, req.Model)

    // 3. 启动新进程，在第一个 prompt 中注入历史
    session := a.startNewProcess(ctx, req)

    // 4. 发送恢复 prompt
    recoveryPrompt := a.buildRecoveryPrompt(compressedHistory, req.Input)
    session.Transport.SendRequest("session/prompt", &acpPromptParams{
        SessionID: session.AcpSessionID,
        Prompt:    []acpContentBlock{{Type: "text", Text: recoveryPrompt}},
    })

    return session, nil
}

func (a *BaseACPAdapter) buildRecoveryPrompt(history string, newInput string) string {
    return fmt.Sprintf(`
## 对话历史恢复

以下是之前的对话摘要，请继续基于此上下文回答：

%s

---

## 当前请求

%s
`, history, newInput)
}
```

### Phase 4: Token 预算控制

**目标**：防止历史过长导致上下文溢出

**改动点**：

1. **历史压缩策略**

```go
type HistoryCompressor struct {
    MaxHistoryTokens int  // 最大历史 Token 数（默认 4000）
}

func (c *HistoryCompressor) Compress(buffer *ConversationBuffer, model string) string {
    // 1. 计算当前 Token 总数
    totalTokens := buffer.TotalTokens()

    // 2. 如果超出预算，从旧对话开始压缩
    if totalTokens > c.MaxHistoryTokens {
        return c.compressOldTurns(buffer, model)
    }

    // 3. 未超出预算，保留完整历史
    return buffer.FormatFull()
}

func (c *HistoryCompressor) compressOldTurns(buffer *ConversationBuffer, model string) string {
    var sb strings.Builder

    // 保留最近的 N 轮完整对话
    recentTurns := buffer.GetRecentTurns(5)

    // 压缩旧对话为摘要
    oldTurns := buffer.GetOldTurns(len(buffer.Turns) - 5)
    sb.WriteString("## 早期对话摘要\n")
    sb.WriteString(c.summarizeOldTurns(oldTurns))

    // 保留最近对话完整
    sb.WriteString("\n## 最近对话（完整）\n")
    for _, turn := range recentTurns {
        sb.WriteString(fmt.Sprintf("[%s] %s\n", turn.Role, turn.Content))
    }

    return sb.String()
}
```

## 实施步骤

### Step 1: 实现 SessionPool（优先级最高）

1. 新建 `internal/service/agent/session_pool.go`
2. 修改 `plugins/acp/adapter_base.go`，添加 `StartOrResumeSession` 方法
3. 修改 `execution_service.go`，在 `executeAgent` 中使用 session pool

### Step 2: 实现对话持久化

1. 新建 `internal/model/session_conversation.go`（数据模型）
2. 新建 `internal/repo/session_conversation_repo.go`（持久化）
3. 在 `LongRunningSession` 中添加定期持久化逻辑

### Step 3: 实现断连恢复

1. 在 `SessionPool.Get` 中添加断连检测
2. 实现 `RecoverSession` 方法
3. 修改 `buildContextLayers`，支持注入恢复历史

### Step 4: Token 预算控制

1. 新建 `internal/service/agent/history_compressor.go`
2. 集成到 `RecoverSession` 流程

## 配置项

```yaml
# configs/config.yaml
session:
  longRunning:
    enabled: true           # 是否启用长连接
    idleTimeout: 600        # 空闲超时（秒）
    maxSessions: 10         # 最大并发 session 数
    persistInterval: 3      # 每几轮对话后持久化

  recovery:
    maxHistoryTokens: 4000  # 恢复历史最大 Token 数
    compressionEnabled: true # 是否启用历史压缩
```

## 兜底策略

如果长连接实现复杂度较高，可以先实现 **"Prompt 注入恢复"** 作为独立方案：

```go
// 在 buildContextLayers 中增强 Layer1
func (es *ExecutionService) buildContextLayers(...) {
    // ...

    // 增强 Layer1：完整对话历史（而非摘要）
    if req.SessionStrategy == SessionStrategyResume {
        // 加载完整历史（而非 extractStructuredHistory 摘要）
        layers.Layer1 = es.loadFullConversationHistory(ctx, threadID, config.ID)
    }

    // ...
}
```

这种方式不需要修改 ACP Adapter，只需增强 `context_builder.go` 的历史加载逻辑。