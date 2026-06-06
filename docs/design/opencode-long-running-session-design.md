# OpenCode 长连接 Session 管理设计

## 一、架构概览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           ExecutionService                                    │
│                                                                              │
│  ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────────────┐   │
│  │ SessionPool     │   │ Conversation    │   │ RecoveryManager         │   │
│  │                 │──▶│ Persister       │──▶│                         │   │
│  │ - session pool  │   │                 │   │ - 断连检测              │   │
│  │ - idle timeout  │   │ - 定期持久化     │   │ - 历史压缩              │   │
│  │ - lifecycle     │   │ - 消息累积       │   │ - prompt 注入恢复       │   │
│  └─────────────────┘   └─────────────────┘   └─────────────────────────┘   │
│         │                      │                        │                   │
│         ▼                      ▼                        ▼                   │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    LongRunningSession                                │   │
│  │                                                                      │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────────┐ │   │
│  │  │ Process     │  │ Transport   │  │ Conversation│  │ State      │ │   │
│  │  │ (OpenCode)  │  │ (JSON-RPC)  │  │ Buffer      │  │ Machine    │ │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  ┌────────────┘ │   │
│  │                                                      │ Active      │ │   │
│  │                                                      │ Idle        │ │   │
│  │                                                      │ Sealing     │ │   │
│  │                                                      │ Sealed      │ │   │
│  │                                                      │ Recovering  │ │   │
│  │                                                      └────────────┘ │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘

数据流向：
                                                    ┌──────────────┐
  用户输入 → SessionPool.GetOrCreate() → Process    │   SQLite     │
                 ↓                        ↓         │              │
          Transport.SendPrompt() → Conversation ←──│ messages     │
                 ↓                        ↓        │ conversations│
          onChunk(chunk) → Buffer.Append() → Persist│              │
                                                    └──────────────┘
```

## 二、核心数据结构

### 2.1 SessionPool

```go
// internal/service/agent/session_pool.go

type SessionPool struct {
    sessions     map[string]*LongRunningSession  // key: "threadID:agentID"
    idleTimeout  time.Duration                   // 空闲超时（默认 10 分钟）
    maxSessions  int                             // 最大并发数（默认 20）
    persistInterval int                          // 每几轮对话后持久化（默认 3）

    // 依赖注入
    repo         *repo.SessionConversationRepo
    compressor   *HistoryCompressor
    wsHub        *ws.Hub

    mu           sync.RWMutex
    stopCh       chan struct{}                   // 停止信号
}

type SessionPoolConfig struct {
    Enabled         bool    `yaml:"enabled"`          // 是否启用长连接
    IdleTimeout     int     `yaml:"idleTimeout"`      // 空闲超时（秒）
    MaxSessions     int     `yaml:"maxSessions"`      // 最大并发数
    PersistInterval int     `yaml:"persistInterval"`  // 持久化间隔（轮）
    MaxHistoryTokens int    `yaml:"maxHistoryTokens"` // 恢复历史最大 Token
}
```

### 2.2 LongRunningSession

```go
// internal/service/agent/long_running_session.go

type SessionStatus string

const (
    SessionStatusActive     SessionStatus = "active"      // 正在执行
    SessionStatusIdle       SessionStatus = "idle"        // 空闲等待
    SessionStatusSealing    SessionStatus = "sealing"     // 正在封存
    SessionStatusSealed     SessionStatus = "sealed"      // 已封存（可恢复）
    SessionStatusRecovering SessionStatus = "recovering"  // 正在恢复
    SessionStatusError      SessionStatus = "error"       // 异常状态
)

type LongRunningSession struct {
    // 标识
    ID           string        `json:"id"`           // 内部 session ID
    AcpSessionID string        `json:"acpSessionId"` // ACP 协议的 session ID
    ThreadID     string        `json:"threadId"`
    AgentID      string        `json:"agentId"`
    InvocationID string        `json:"invocationId"` // 当前执行的 invocation ID

    // 进程管理
    Process      *exec.Cmd     `json:"-"`
    Transport    *acpTransport `json:"-"`
    StdinPipe    io.WriteCloser `json:"-"`
    StdoutPipe   io.Reader     `json:"-"`
    StderrBuffer strings.Builder `json:"-"`

    // 状态
    Status       SessionStatus `json:"status"`
    LastActiveAt time.Time     `json:"lastActiveAt"`
    CreatedAt    time.Time     `json:"createdAt"`
    TurnCount    int           `json:"turnCount"`    // 对话轮数

    // 对话累积
    Conversation *ConversationBuffer `json:"conversation"`

    // 回调
    onChunk      func(Chunk)   `json:"-"`

    // 并发控制
    mu           sync.RWMutex  `json:"-"`
    cancel       context.CancelFunc `json:"-"`
}

// ConversationBuffer 对话缓冲区
type ConversationBuffer struct {
    Turns        []ConversationTurn `json:"turns"`
    TotalTokens  int                `json:"totalTokens"`
    KeyEntities  []KeyEntity        `json:"keyEntities"` // 关键实体追踪（问题单号等）
    mu           sync.RWMutex       `json:"-"`
}

type ConversationTurn struct {
    TurnID       string                 `json:"turnId"`       // 唯一 ID
    Role         string                 `json:"role"`         // "user" 或 "agent"
    Content      string                 `json:"content"`      // 消息内容
    Timestamp    time.Time              `json:"timestamp"`
    TokenCount   int                    `json:"tokenCount"`   // Token 计数
    ContentBlocks []ContentBlockData    `json:"contentBlocks"` // 结构化内容块
    Metadata     map[string]string      `json:"metadata"`     // agentID, toolCalls 等
}

// KeyEntity 关键实体追踪（如问题单号）
type KeyEntity struct {
    Type         string   `json:"type"`         // "issue_id", "project_name", "requirement"
    Value        string   `json:"value"`        // "BUG-12345"
    MentionedAt  []int    `json:"mentionedAt"`  // 出现在哪些 turnID
    LastUpdateAt int      `json:"lastUpdateAt"` // 最后提到的 turn 序号
}
```

### 2.3 数据库模型

```go
// internal/model/session_conversation.go

type SessionConversation struct {
    ID           uuid.UUID `json:"id" gorm:"primaryKey"`
    ThreadID     uuid.UUID `json:"threadId" gorm:"index"`
    AgentID      uuid.UUID `json:"agentId" gorm:"index"`
    SessionID    string    `json:"sessionId" gorm:"uniqueIndex:thread_agent_session"`

    // 状态
    Status       string    `json:"status"`       // active, idle, sealed
    TurnCount    int       `json:"turnCount"`
    TotalTokens  int       `json:"totalTokens"`

    // 对话内容（JSON）
    Conversation []byte    `json:"conversation"` // ConversationBuffer 序列化
    KeyEntities  []byte    `json:"keyEntities"`  // KeyEntity 列表序列化

    // 进程信息（用于恢复诊断）
    ProcessPID   int       `json:"processPid"`   // 进程 PID（已失效，仅供参考）
    LastActiveAt int64     `json:"lastActiveAt"` // 最后活跃时间

    // 创建/更新时间
    CreatedAt    int64     `json:"createdAt"`
    UpdatedAt    int64     `json:"updatedAt"`
    SealedAt     int64     `json:"sealedAt,omitempty"` // 封存时间
}
```

## 三、状态机设计

```
                    ┌─────────────────────────────────────────────────────────────┐
                    │                     Session Status Machine                   │
                    └─────────────────────────────────────────────────────────────┘

                                      ┌──────────────┐
                                      │   (创建)     │
                                      └──────────────┘
                                             │
                                             │ GetOrCreate()
                                             ▼
    ┌────────────────────────────────────────────────────────────────────────────┐
    │                              Active                                          │
    │  - Process 正在执行                                                          │
    │  - Transport 连接活跃                                                        │
    │  - 正在接收 chunks                                                           │
    │  - Conversation 实时累积                                                     │
    │                                                                              │
    │  触发事件：                                                                   │
    │  - user_input: 继续执行                                                      │
    │  - execution_complete → Idle                                                │
    │  - user_cancel → Sealing                                                    │
    │  - process_crash → Recovering                                               │
    │  - timeout → Sealing                                                        │
    └────────────────────────────────────────────────────────────────────────────┘
                                             │
                              execution_complete│
                                             ▼
    ┌────────────────────────────────────────────────────────────────────────────┐
    │                              Idle                                            │
    │  - Process 存活但空闲                                                        │
    │  - Transport 连接保持                                                        │
    │  - 等待下一次用户输入                                                        │
    │  - 定期持久化 Conversation                                                   │
    │                                                                              │
    │  触发事件：                                                                   │
    │  - user_input → Active                                                       │
    │  - idle_timeout → Sealing                                                    │
    │  - process_crash → Recovering                                               │
    │  - server_shutdown → Sealing                                                │
    │                                                                              │
    │  空闲计时器：10 分钟                                                          │
    └────────────────────────────────────────────────────────────────────────────┘
                                             │
                              idle_timeout    │
                                             ▼
    ┌────────────────────────────────────────────────────────────────────────────┐
    │                              Sealing                                         │
    │  - 正在封存对话                                                              │
    │  - 持久化 Conversation 到数据库                                             │
    │  - 提取 KeyEntities                                                         │
    │  - 终止 Process                                                             │
    │                                                                              │
    │  触发事件：                                                                   │
    │  - persist_complete → Sealed                                                │
    │  - persist_failed → Error                                                   │
    │                                                                              │
    │  封存超时：30 秒                                                              │
    └────────────────────────────────────────────────────────────────────────────┘
                                             │
                              persist_complete│
                                             ▼
    ┌────────────────────────────────────────────────────────────────────────────┐
    │                              Sealed                                          │
    │  - Process 已终止                                                           │
    │  - Conversation 已持久化                                                    │
    │  - 可以通过 Recovery 恢复                                                   │
    │                                                                              │
    │  触发事件：                                                                   │
    │  - user_input → Recovering                                                  │
    │  - conversation_expired → 删除                                              │
    │                                                                              │
    │  封存有效期：24 小时                                                          │
    └────────────────────────────────────────────────────────────────────────────┘
                                             │
                              user_input      │
                                             ▼
    ┌────────────────────────────────────────────────────────────────────────────┐
    │                              Recovering                                      │
    │  - 加载持久化的 Conversation                                                │
    │  - 压缩历史（Token 预算控制）                                                │
    │  - 启动新 Process                                                           │
    │  - 注入恢复 Prompt                                                          │
    │                                                                              │
    │  触发事件：                                                                   │
    │  - recovery_success → Active                                               │
    │  - recovery_failed → Error (fallback to new session)                       │
    │                                                                              │
    │  恢复超时：60 秒                                                              │
    └────────────────────────────────────────────────────────────────────────────┘
                                             │
                              recovery_success│
                                             ▼
                                      [回到 Active]

    ┌────────────────────────────────────────────────────────────────────────────┐
    │                              Error                                           │
    │  - 异常状态                                                                  │
    │  - 尝试清理资源                                                              │
    │  - 降级：创建新 Session（不恢复历史）                                        │
    └────────────────────────────────────────────────────────────────────────────┘
```

## 四、断连场景处理详解

### 4.1 场景一：用户主动取消

**触发条件**：用户点击前端"取消"按钮

**处理流程**：

```
用户点击取消
    │
    ▼
ExecutionService.CancelAgent(invocationID)
    │
    ├── 1. 查询 invocation 状态
    │      - 如果已是终态（completed/failed/cancelled）→ 直接返回
    │
    ├── 2. 更新 invocation 状态为 cancelled
    │
    ├── 3. 广播 cancelled 状态（通知前端）
    │
    ├── 4. 获取 SessionPool 中的 session
    │      - 调用 session.Cancel()
    │      - 设置 session.Status = Sealing
    │
    ├── 5. 封存流程（异步执行）
    │      ├── 持久化 Conversation（当前已累积的内容）
    │      ├── 提取 KeyEntities
    │      ├── 终止 Process（killProcessTree）
    │      └── 更新 Status = Sealed
    │
    └── 6. 清理资源
           - 关闭 Transport
           - 从 SessionPool 移除
           - 保留 Sealed 记录在数据库（可恢复）
```

**关键代码**：

```go
func (s *LongRunningSession) Cancel(reason string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.Status != SessionStatusActive {
        return nil // 非活跃状态无需处理
    }

    s.Status = SessionStatusSealing

    // 1. 取消当前执行
    if s.cancel != nil {
        s.cancel()
    }

    // 2. 异步封存（不阻塞用户响应）
    go s.sealSession(SealReasonManual, reason)

    return nil
}

func (s *LongRunningSession) sealSession(reason SealReason, detail string) {
    // 1. 持久化当前对话
    s.Conversation.Persist()

    // 2. 提取关键实体
    s.Conversation.ExtractKeyEntities()

    // 3. 终止进程
    killProcessTree(s.Process.Process)

    // 4. 更新状态
    s.Status = SessionStatusSealed
    s.repo.UpdateSessionStatus(s.ID, SessionStatusSealed)
}
```

**恢复策略**：
- 用户下次输入时，检测到 Sealed session
- 进入 Recovering 状态
- 加载历史 + 压缩 + 注入恢复 prompt

---

### 4.2 场景二：Agent 执行完成（正常结束）

**触发条件**：OpenCode 完成 prompt 执行，返回 `stopReason: "end_turn"`

**处理流程**：

```
OpenCode 返回 end_turn
    │
    ▼
Transport 收到 promptResult
    │
    ├── 1. 更新 invocation 状态为 completed
    │
    ├── 2. 保存 agent 消息到 messages 表
    │
    ├── 3. 更新 session 状态
    │      - session.Status = Idle
    │      - session.LastActiveAt = now
    │      - session.TurnCount++
    │
    ├── 4. Conversation 累积
    │      - 添加 agent turn 到 buffer
    │      - 计算并更新 TotalTokens
    │
    ├── 5. 定期持久化检查
    │      - if TurnCount % persistInterval == 0:
    │      -   持久化 Conversation 到数据库
    │
    ├── 6. 启动空闲计时器
    │      - idleTimer.Reset(idleTimeout)
    │      - 等待下一次用户输入或超时
    │
    └── 7. 从 runningAgents 移除（但保留在 SessionPool）
```

**关键代码**：

```go
func (p *SessionPool) OnExecutionComplete(sessionKey string, output string) {
    session := p.Get(sessionKey)
    if session == nil {
        return
    }

    session.mu.Lock()
    defer session.mu.Unlock()

    // 1. 更新状态
    session.Status = SessionStatusIdle
    session.LastActiveAt = time.Now()
    session.TurnCount++

    // 2. 累积对话
    session.Conversation.AppendTurn(ConversationTurn{
        Role:      "agent",
        Content:   output,
        Timestamp: time.Now(),
    })

    // 3. 定期持久化
    if session.TurnCount % p.persistInterval == 0 {
        go p.persistConversation(session)
    }

    // 4. 重置空闲计时器
    p.resetIdleTimer(sessionKey)
}
```

**恢复策略**：
- Process 保持存活
- 用户下次输入直接发送 prompt（无需恢复）
- 如果空闲超时 → 进入 Sealing → 后续 Recovering

---

### 4.3 场景三：进程意外崩溃

**触发条件**：OpenCode CLI 进程异常退出（OOM、崩溃、被系统杀掉）

**检测机制**：

```go
// 进程存活监控 goroutine
func (s *LongRunningSession) monitorProcessHealth() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if s.Process == nil || s.Process.Process == nil {
                continue
            }

            // 检查进程是否存活
            if err := s.Process.Process.Signal(syscall.Signal(0)); err != nil {
                // 进程已死
                s.onProcessDeath("signal_check_failed")
                return
            }

        case <-s.ctx.Done():
            return
        }
    }
}

// stdout/stderr 读取中断检测
func (s *LongRunningSession) readStdout() {
    defer func() {
        // stdout 关闭意味着进程退出
        if s.Status == SessionStatusActive {
            s.onProcessDeath("stdout_closed")
        }
    }()

    scanner := bufio.NewScanner(s.StdoutPipe)
    for scanner.Scan() {
        // ... 处理输出
    }
}
```

**处理流程**：

```
检测到进程崩溃
    │
    ▼
onProcessDeath(trigger)
    │
    ├── 1. 确认进程状态
    │      - 尝试 Process.Wait() 获取退出码
    │      - 记录崩溃原因到日志
    │
    ├── 2. 更新 session 状态
    │      - session.Status = Recovering
    │
    ├── 3. 持久化当前对话（紧急保存）
    │      - Conversation.Persist() (同步执行)
    │
    ├── 4. 清理残留资源
    │      - 关闭 Transport
    │      - 清理 stdin/stdout pipes
    │
    ├── 5. 判断是否可恢复
    │      - if TurnCount > 0 && Conversation 已持久化:
    │      -   启动恢复流程
    │      - else:
    │      -   降级：创建新 session（不恢复历史）
    │
    ├── 6. 恢复流程
    │      ├── 加载持久化历史
    │      ├── 压缩历史（Token 预算）
    │      ├── 启动新 Process
    │      ├── 注入恢复 Prompt
    │      └── 更新 Status = Active
    │
    └── 7. 广播恢复事件
           - WebSocket: session_recovered
           - 通知前端继续等待响应
```

**关键代码**：

```go
func (s *LongRunningSession) onProcessDeath(trigger string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    logInfo("Process death detected",
        zap.String("sessionID", s.ID),
        zap.String("trigger", trigger),
        zap.String("status", string(s.Status)))

    // 1. 紧急持久化
    s.Conversation.mu.RLock()
    conversationCopy := s.Conversation.DeepCopy()
    s.Conversation.mu.RUnlock()

    p.repo.SaveSessionConversation(s.ThreadID, s.AgentID, conversationCopy)

    // 2. 清理
    if s.Transport != nil {
        s.Transport.Close()
    }

    // 3. 判断恢复策略
    if s.TurnCount > 0 {
        s.Status = SessionStatusRecovering
        go s.recoverSession()
    } else {
        // 无历史，降级为新 session
        s.Status = SessionStatusError
        p.wsHub.Broadcast(s.ThreadID, ws.WSMessage{
            Type: "session_recovery_failed",
            Payload: map[string]interface{}{
                "reason": "no_history",
                "fallback": "new_session",
            },
        })
    }
}

func (s *LongRunningSession) recoverSession() error {
    // 1. 加载历史
    history, err := p.repo.LoadSessionConversation(s.ThreadID, s.AgentID)
    if err != nil {
        return s.fallbackToNewSession()
    }

    // 2. 压缩历史
    compressed := p.compressor.Compress(history, s.baseAgent.DefaultModel)

    // 3. 启动新进程
    newSession, err := p.startNewProcess(s.ctx, s.req)
    if err != nil {
        return err
    }

    // 4. 注入恢复 prompt
    recoveryPrompt := p.buildRecoveryPrompt(compressed, s.pendingInput)
    newSession.Transport.SendRequest("session/prompt", &acpPromptParams{
        SessionID: newSession.AcpSessionID,
        Prompt:    []acpContentBlock{{Type: "text", Text: recoveryPrompt}},
    })

    // 5. 更新状态
    s.Process = newSession.Process
    s.Transport = newSession.Transport
    s.Status = SessionStatusActive

    return nil
}
```

---

### 4.4 场景四：服务重启/崩溃

**触发条件**：后端服务重启（升级、崩溃、手动重启）

**问题**：所有内存中的 session 都丢失

**设计原则**：
- 服务启动时，不自动恢复所有 session
- 用户下次输入时，触发按需恢复

**处理流程**：

```
服务启动
    │
    ▼
SessionPool.Initialize()
    │
    ├── 1. 加载配置
    │      - idleTimeout, maxSessions, persistInterval
    │
    ├── 2. 清理残留的 Sealed sessions（可选）
    │      - 删除超过 24 小时的 Sealed 记录
    │
    ├── 3. 不主动启动任何 session
    │      - 等待用户触发
    │
    └── 4. 启动后台清理 goroutine
           - 定期清理过期 Sealed 记录

用户发送新消息
    │
    ▼
ExecutionService.SpawnAgent()
    │
    ├── 1. 检查 SessionPool 是否有活跃 session
    │      - 内存中无（服务刚重启）
    │
    ├── 2. 检查数据库是否有 Sealed session
    │      - if exists && SealedAt < 24小时:
    │      -   进入 Recovering 流程
    │
    ├── 3. 恢复流程
    │      ├── 加载历史
    │      ├── 压缩 + 注入恢复 prompt
    │      ├── 启动新 Process
    │      └── 更新 Status = Active
    │
    └── 4. 如果无 Sealed 记录
           - 创建新 session（无历史）
```

**关键代码**：

```go
func (p *SessionPool) GetOrCreate(ctx context.Context, threadID, agentID string, req *ExecutionRequest) (*LongRunningSession, error) {
    sessionKey := fmt.Sprintf("%s:%s", threadID, agentID)

    // 1. 检查内存中是否有活跃 session
    p.mu.RLock()
    session := p.sessions[sessionKey]
    p.mu.RUnlock()

    if session != nil && session.Status == SessionStatusActive {
        return session, nil // 直接复用
    }

    // 2. 检查数据库是否有可恢复的 Sealed session
    sealedRecord, err := p.repo.FindSealedSession(threadID, agentID)
    if err == nil && sealedRecord != nil {
        // 有历史，尝试恢复
        return p.recoverFromSealed(ctx, sealedRecord, req)
    }

    // 3. 无历史，创建新 session
    return p.createNewSession(ctx, sessionKey, req)
}

func (p *SessionPool) recoverFromSealed(ctx context.Context, sealed *model.SessionConversation, req *ExecutionRequest) (*LongRunningSession, error) {
    // 1. 解析历史
    var conversation ConversationBuffer
    json.Unmarshal(sealed.Conversation, &conversation)

    // 2. 压缩历史（Token 预算控制）
    compressed := p.compressor.Compress(&conversation, req.BaseAgent.DefaultModel)

    // 3. 启动新进程
    session, err := p.startNewProcess(ctx, req)
    if err != nil {
        return nil, err
    }

    session.Status = SessionStatusRecovering

    // 4. 构建恢复 prompt
    recoveryPrompt := p.buildRecoveryPrompt(compressed, req.Input)

    // 5. 发送恢复 prompt
    _, err = session.Transport.SendRequest("session/prompt", &acpPromptParams{
        SessionID: session.AcpSessionID,
        Prompt:    []acpContentBlock{{Type: "text", Text: recoveryPrompt}},
    })
    if err != nil {
        // 恢复失败，降级为新 session
        session.Process.Process.Kill()
        return p.createNewSession(ctx, sessionKey, req)
    }

    // 6. 更新状态
    session.Status = SessionStatusActive
    session.TurnCount = conversation.TurnCount

    // 7. 更新数据库记录
    p.repo.UpdateSessionStatus(sealed.ID, SessionStatusActive)

    return session, nil
}
```

---

### 4.5 场景五：空闲超时

**触发条件**：用户长时间无输入，session 空闲超过配置的 timeout（默认 10 分钟）

**目的**：释放资源，避免无限占用进程

**处理流程**：

```
空闲计时器触发
    │
    ▼
onIdleTimeout(sessionKey)
    │
    ├── 1. 检查 session 状态
    │      - 确认是 Idle 状态
    │      - 确认无正在执行的任务
    │
    ├── 2. 进入 Sealing 流程
    │      ├── 持久化 Conversation（完整历史）
    │      ├── 提取 KeyEntities
    │      ├── 终止 Process
    │      └── 更新 Status = Sealed
    │
    ├── 3. 从 SessionPool 移除
    │
    └── 4. 广播状态变更
           - WebSocket: session_sealed
           - 前端可显示 "会话已保存，下次继续时将恢复上下文"
```

**关键代码**：

```go
func (p *SessionPool) startIdleMonitor() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            p.checkIdleSessions()
        case <-p.stopCh:
            return
        }
    }
}

func (p *SessionPool) checkIdleSessions() {
    p.mu.RLock()
    sessions := make([]*LongRunningSession, 0)
    for _, s := range p.sessions {
        sessions = append(sessions, s)
    }
    p.mu.RUnlock()

    now := time.Now()
    for _, session := range sessions {
        session.mu.RLock()
        status := session.Status
        lastActive := session.LastActiveAt
        session.mu.RUnlock()

        if status == SessionStatusIdle && now.Sub(lastActive) > p.idleTimeout {
            p.sealIdleSession(session)
        }
    }
}

func (p *SessionPool) sealIdleSession(session *LongRunningSession) {
    session.mu.Lock()
    session.Status = SessionStatusSealing
    session.mu.Unlock()

    // 异步封存（不阻塞清理循环）
    go func() {
        session.sealSession(SealReasonTimeout, "idle_timeout")

        // 从 pool 移除
        p.mu.Lock()
        delete(p.sessions, session.ID)
        p.mu.Unlock()

        // 广播
        p.wsHub.Broadcast(session.ThreadID, ws.WSMessage{
            Type: "session_sealed",
            Payload: map[string]interface{}{
                "sessionId": session.ID,
                "reason": "idle_timeout",
                "recoverable": true,
            },
        })
    }()
}
```

---

### 4.6 场景六：WebSocket 断连

**触发条件**：前端 WebSocket 连接断开（网络问题、用户关闭页面）

**检测机制**：

```go
// ws/hub.go 中的断连检测
func (h *Hub) OnClientDisconnect(clientID string) {
    // 查找该 client 关联的 threads
    threads := h.getClientThreads(clientID)

    for _, threadID := range threads {
        // 检查是否有活跃 session
        session := h.sessionPool.GetByThread(threadID)
        if session != nil && session.Status == SessionStatusActive {
            // 用户断连，但 session 可能还在执行
            // 等待执行完成后再处理
            session.mu.Lock()
            session.clientDisconnected = true
            session.mu.Unlock()
        }
    }
}
```

**处理策略**：

```
WebSocket 断连
    │
    ▼
标记 session.clientDisconnected = true
    │
    ├── 1. 如果 session 正在执行
    │      - 继续执行（不中断）
    │      - 执行完成后检查 clientDisconnected
    │
    ├── 2. 执行完成后
    │      - if clientDisconnected:
    │      -   进入 Idle 状态
    │      -   启动空闲计时器
    │      -   不广播（用户已断连）
    │
    ├── 3. 空闲超时后
    │      - 正常 Sealing 流程
    │      - 持久化历史
    │
    └── 4. 用户重新连接
           - 加载 Sealed session 状态
           - 显示历史对话
           - 用户可继续输入（触发 Recovering）
```

---

### 4.7 场景七：AskUserQuestion 等待

**特殊情况**：Agent 发起 AskUserQuestion，等待用户响应

**问题**：用户可能长时间不响应

**处理策略**：

```
AskUserQuestion 触发
    │
    ▼
收到 session/request_user_input notification
    │
    ├── 1. 设置特殊状态
    │      - session.Status = Active (但 subState = waiting_for_input)
    │      - session.PendingQuestion = chunk
    │
    ├── 2. 广播 question_ready
    │      - 前端显示问题
    │
    ├── 3. 特殊空闲计时
    │      - AskUserQuestion 状态下，空闲超时延长（30 分钟）
    │      - 给用户足够时间思考
    │
    ├── 4. 用户响应
    │      - SendToolResult()
    │      - 继续执行 → Active → Idle
    │
    ├── 5. 超时未响应
    │      - sealSession(SealReasonAskUserQuestionTimeout)
    │      - 保存当前状态（包括 PendingQuestion）
    │      - 用户下次连接时，恢复问题状态
```

---

## 五、恢复 Prompt 构建策略

### 5.1 历史压缩算法

```go
func (c *HistoryCompressor) Compress(buffer *ConversationBuffer, model string) string {
    windowSize := c.getWindowSize(model)
    maxHistoryTokens := int(windowSize * 0.15) // 15% 用于历史

    totalTokens := buffer.TotalTokens

    if totalTokens <= maxHistoryTokens {
        // 无需压缩，返回完整历史
        return c.formatFullHistory(buffer)
    }

    // 需要压缩
    return c.compressWithBudget(buffer, maxHistoryTokens)
}

func (c *HistoryCompressor) compressWithBudget(buffer *ConversationBuffer, budget int) string {
    var sb strings.Builder
    usedTokens := 0

    // 1. 始终保留 KeyEntities（优先级最高）
    sb.WriteString("## 关键信息\n\n")
    for _, entity := range buffer.KeyEntities {
        line := fmt.Sprintf("- **%s**: %s\n", entity.Type, entity.Value)
        sb.WriteString(line)
        usedTokens += EstimateTokens(line)
    }
    sb.WriteString("\n")

    // 2. 保留最近 N 轮完整对话（优先级高）
    recentTurns := buffer.GetRecentTurns(5)
    sb.WriteString("## 最近对话\n\n")
    for _, turn := range recentTurns {
        content := turn.Content
        if len(content) > 500 {
            content = TruncateHeadTail(content, 500)
        }
        line := fmt.Sprintf("**%s**: %s\n\n", turn.Role, content)
        sb.WriteString(line)
        usedTokens += EstimateTokens(line)
    }

    // 3. 剩余预算用于早期对话摘要
    remainingBudget := budget - usedTokens
    if remainingBudget > 200 {
        oldTurns := buffer.GetOldTurns(len(buffer.Turns) - 5)
        sb.WriteString("## 早期对话摘要\n\n")
        summary := c.summarizeTurns(oldTurns, remainingBudget)
        sb.WriteString(summary)
    }

    return sb.String()
}

func (c *HistoryCompressor) summarizeTurns(turns []ConversationTurn, budget int) string {
    // 提取关键决策和结论
    var sb strings.Builder
    usedTokens := 0

    for _, turn := range turns {
        if usedTokens >= budget {
            break
        }

        // 只提取结论性内容
        conclusions := ExtractConclusions(turn.Content)
        for _, conclusion := range conclusions {
            line := fmt.Sprintf("- %s\n", conclusion)
            sb.WriteString(line)
            usedTokens += EstimateTokens(line)
        }
    }

    return sb.String()
}
```

### 5.2 恢复 Prompt 模板

```go
func (p *SessionPool) buildRecoveryPrompt(history string, newInput string) string {
    return fmt.Sprintf(`
## 会话恢复

你正在继续一个之前中断的对话。以下是历史上下文，请在此基础上继续回答。

%s

---

## 当前请求

%s

**注意**：
1. 请基于历史上下文理解当前请求
2. 如果历史中提到的关键信息（如问题单号）与当前请求相关，请继续使用
3. 如果当前请求是新的话题，可以独立处理
`, history, newInput)
}
```

---

## 六、配置与监控

### 6.1 配置项

```yaml
# configs/config.yaml
session:
  longRunning:
    enabled: true              # 是否启用长连接模式
    idleTimeout: 600           # 空闲超时（秒），默认 10 分钟
    maxSessions: 20            # 最大并发 session 数
    persistInterval: 3         # 每几轮对话后持久化
    maxHistoryTokens: 4000     # 恢复历史最大 Token 数
    sealedExpireHours: 24      # Sealed 记录过期时间（小时）
    askUserQuestionTimeout: 1800  # AskUserQuestion 等待超时（秒）

  recovery:
    maxAttempts: 3             # 恢复尝试次数
    fallbackToNewSession: true # 恢复失败时是否降级为新 session
    compressionEnabled: true   # 是否启用历史压缩
```

### 6.2 监控指标

```go
// internal/service/agent/session_metrics.go

type SessionMetrics struct {
    // 计数器
    ActiveSessions     int `json:"activeSessions"`
    IdleSessions       int `json:"idleSessions"`
    SealedSessions     int `json:"sealedSessions"`
    RecoveringSessions int `json:"recoveringSessions"`

    // 累计
    TotalSessionsCreated int `json:"totalSessionsCreated"`
    TotalSessionsSealed  int `json:"totalSessionsSealed"`
    TotalSessionsRecovered int `json:"totalSessionsRecovered"`
    TotalRecoveryFailures int `json:"totalRecoveryFailures"`

    // 性能
    AvgSessionLifetime    float64 `json:"avgSessionLifetime"`   // 平均存活时间（秒）
    AvgRecoveryTime       float64 `json:"avgRecoveryTime"`      // 平均恢复时间（秒）
    AvgConversationTokens float64 `json:"avgConversationTokens"` // 平均对话 Token 数

    // 资源
    TotalProcessMemoryMB  float64 `json:"totalProcessMemoryMB"` // 进程总内存占用
}

func (p *SessionPool) GetMetrics() SessionMetrics {
    // 实现指标收集
}
```

---

## 七、实施计划

### Phase 1：核心框架（Week 1）

1. 创建 `session_pool.go`、`long_running_session.go`
2. 修改 `acp/adapter_base.go`，支持长连接模式
3. 实现 Active → Idle → Sealing → Sealed 状态流转
4. 单元测试覆盖基本场景

### Phase 2：持久化与恢复（Week 2）

1. 创建 `model/session_conversation.go`、`repo/`
2. 实现 ConversationBuffer 和定期持久化
3. 实现 Recovering 状态和恢复 prompt 构建
4. 集成 HistoryCompressor

### Phase 3：断连处理（Week 3）

1. 实现进程崩溃检测和自动恢复
2. 实现空闲超时和 AskUserQuestion 特殊处理
3. WebSocket 断连处理
4. 服务重启后的按需恢复

### Phase 4：监控与优化（Week 4）

1. 添加 SessionMetrics 和监控 API
2. 性能优化：历史压缩效率、恢复延迟
3. 前端集成：显示 session 状态、恢复进度
4. 端到端测试覆盖所有断连场景