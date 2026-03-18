# Agent执行架构统一设计

## 背景

在多Agent开发平台中，存在两套Agent执行入口：

1. **工作流场景**：项目任务触发Agent执行
2. **调试场景**：Agent管理中的调试功能触发

这两部分调用了不同的方法，存在割裂。调试本身是为了工作流服务的，应该使用相同的方法。同时，需要将Agent类型（ClaudeCode、OpenCode）的差异在工作流层面屏蔽，抽象出统一的接口，便于后续扩展其他类型的Agent。

## 目标

1. **统一执行入口**：工作流和调试使用相同的Agent执行方法
2. **抽象统一接口**：在Adapter层屏蔽Agent类型差异
3. **简化扩展流程**：新增Agent类型只需实现Adapter接口

## 现状分析

### 当前架构问题

```
工作流场景:
  ExecutionService.spawnWorkflowAgent() → adapter.ExecuteWithStream()

调试场景:
  Orchestrator.StartInteractiveSession() → InteractiveSessionManager → 自己管理的CLI进程
```

**问题点**：

1. **执行路径割裂**：两套完全不同的执行逻辑
2. **会话管理重复**：
   - `InteractiveSessionManager`（调试场景）
   - `SessionManager`（尝试统一但未完成）
   - `ExecutionService.sessionIDs` map（工作流场景）
3. **Agent类型判断分散**：在 `adapter.go`、`session_manager.go`、`interactive_session.go` 等多处

### 决策记录

| 问题 | 决策 |
|-----|------|
| 调试是否支持路由等高级特性 | 保留部分差异：调试跳过路由逻辑 |
| 会话是否需要持久化 | 内存管理即可 |
| 架构方案 | Adapter层完全封装会话管理 |

## 设计方案

### 核心思路

采用**Adapter层完全封装**方案：

- 所有会话管理逻辑下沉到各Adapter内部
- `ExecutionService` 作为统一调用入口
- 工作流和调试使用完全相同的执行路径，差异仅在是否有工作流模板

### 新的AgentAdapter接口

```go
// AgentAdapter 统一的Agent适配器接口
type AgentAdapter interface {
    // Execute 执行单次任务（无会话上下文）
    Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error)

    // ExecuteWithStream 流式执行，实时回调输出
    ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) error

    // StartSession 启动交互式会话
    StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error

    // ResumeSession 恢复会话，发送新消息
    ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error

    // StopSession 停止会话
    StopSession(sessionID string) error

    // GetSessionStatus 获取会话状态
    GetSessionStatus(sessionID string) SessionStatus

    // CheckHealth 检查CLI健康状态
    CheckHealth(ctx context.Context) error
}

// ExecutionRequest 统一的执行请求
type ExecutionRequest struct {
    Config      *model.AgentRoleConfig
    BaseAgent   *model.BaseAgent
    Context     *ContextLayers   // 上下文层
    Input       string           // 用户输入
    WorkDir     string           // 工作目录
    SessionKey  string           // 用于会话恢复（空表示新会话）
}

// ExecutionResult 执行结果
type ExecutionResult struct {
    Output      string
    SessionKey  string  // 返回的会话标识（用于后续恢复）
}

// Chunk 流式输出块
type Chunk struct {
    Type    ChunkType  // Text, Error, Status
    Content string
}

// ChunkType 输出块类型
type ChunkType string

const (
    ChunkTypeText   ChunkType = "text"
    ChunkTypeError  ChunkType = "error"
    ChunkTypeStatus ChunkType = "status"
)
```

### ExecutionService统一调用层

**核心原则**：工作流和调试使用完全相同的执行逻辑，路由检查自然依赖于是否有工作流模板。

```go
// ExecutionService 统一执行服务
type ExecutionService struct {
    invocationRepo   *repo.AgentInvocationRepository
    threadRepo       *repo.ThreadRepository
    msgRepo          *repo.MessageRepository
    configSvc        *ConfigService
    baseAgentSvc     *BaseAgentService
    tracker          *InvocationTracker
    workflow         *WorkflowEngine
    workflowRepo     *repo.WorkflowTemplateRepository
    projectRepo      *repo.ProjectRepository
    wsHub            *ws.Hub
    defaultAdapter   AgentAdapter

    // Adapter缓存
    adapters      map[string]AgentAdapter
    adapterMu     sync.RWMutex
}

// SpawnAgent 统一入口 - 工作流和调试都走这里
func (es *ExecutionService) SpawnAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
    // 1. 获取配置和BaseAgent
    config, baseAgent := es.resolveConfig(ctx, req)

    // 2. 获取对应的Adapter
    adapter := es.getOrCreateAdapter(baseAgent)

    // 3. 构建执行请求
    execReq := &ExecutionRequest{
        Config:    config,
        BaseAgent: baseAgent,
        Input:     req.Input,
        WorkDir:   req.ProjectPath,
    }

    // 4. 创建调用记录
    invocation := es.createInvocation(ctx, req, config)

    // 5. 异步执行Agent（统一的执行路径）
    go es.executeAgent(ctx, adapter, execReq, invocation)

    return invocation, nil
}

// executeAgent 统一的Agent执行逻辑
func (es *ExecutionService) executeAgent(ctx context.Context, adapter AgentAdapter, req *ExecutionRequest, invocation *model.AgentInvocation) {
    defer func() {
        if r := recover(); r != nil {
            es.handleAgentError(ctx, invocation, fmt.Errorf("panic: %v", r))
        }
        es.cleanupRunningAgent(invocation.ID)
    }()

    // 执行并流式输出
    var outputBuilder strings.Builder
    err := adapter.ExecuteWithStream(ctx, req, func(chunk Chunk) {
        if chunk.Type == ChunkTypeText {
            outputBuilder.WriteString(chunk.Content)
        }
        es.broadcastChunk(invocation.ThreadID, invocation.ID, chunk)
    })

    if err != nil {
        es.handleAgentError(ctx, invocation, err)
        return
    }

    output := outputBuilder.String()

    // 更新调用状态
    es.completeInvocation(ctx, invocation, output)

    // 路由检查 - 如果有工作流模板则尝试路由
    // 调试场景没有模板，自然不会触发路由
    es.tryRouteToNextAgent(ctx, invocation.ThreadID, config, output)
}

// tryRouteToNextAgent 尝试路由到下一个Agent
func (es *ExecutionService) tryRouteToNextAgent(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string) {
    // 获取工作流模板中的Agent列表
    allowedAgents := es.getAllowedAgentsFromWorkflow(ctx, threadID)
    if len(allowedAgents) == 0 {
        // 没有工作流模板，不进行路由
        return
    }

    // 解析 @mention 并路由
    mentions := es.parseMentions(output)
    for _, mention := range mentions {
        targetConfig := es.findTargetAgent(allowedAgents, mention)
        if targetConfig != nil {
            es.SpawnAgent(ctx, &SpawnRequest{
                ThreadID:    threadID,
                ConfigID:    targetConfig.ID,
                Input:       output,
                // ...
            })
        }
    }
}
```

### Adapter实现示例

#### ClaudeAdapter

```go
type ClaudeAdapter struct {
    cliPath     string
    baseAgent   *model.BaseAgent

    // 会话管理
    sessions    map[string]*claudeSession
    mu          sync.RWMutex
}

type claudeSession struct {
    id          string
    sessionKey  string          // CLI的session-id
    cmd         *exec.Cmd
    ctx         context.Context
    cancel      context.CancelFunc
    status      SessionStatus
    stdout      io.Reader
}

func (a *ClaudeAdapter) StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error {
    session := &claudeSession{
        id:     sessionID,
        status: SessionStatusRunning,
    }

    // 构建命令参数
    args := []string{
        "--print",
        "--output-format", "stream-json",
        "--permission-mode", "auto",
    }
    if a.baseAgent.DefaultModel != "" {
        args = append(args, "--model", a.baseAgent.DefaultModel)
    }

    // 新会话用 --session-id
    sessionKey := uuid.New().String()
    session.sessionKey = sessionKey
    args = append(args, "--session-id", sessionKey)

    // 启动进程
    cmd := exec.CommandContext(ctx, a.cliPath, args...)
    cmd.Stdin = strings.NewReader(a.buildPrompt(req))
    cmd.Dir = req.WorkDir
    cmd.Env = a.buildEnv()

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return err
    }
    session.stdout = stdout
    session.cmd = cmd

    if err := cmd.Start(); err != nil {
        return err
    }

    // 启动输出读取协程
    go a.readOutput(session)

    a.mu.Lock()
    a.sessions[sessionID] = session
    a.mu.Unlock()
    return nil
}

func (a *ClaudeAdapter) ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error {
    a.mu.RLock()
    session := a.sessions[sessionID]
    a.mu.RUnlock()

    if session == nil {
        return fmt.Errorf("session not found: %s", sessionID)
    }

    // 使用 --resume 恢复会话
    args := []string{
        "--print",
        "--output-format", "stream-json",
        "--resume", session.sessionKey,
    }

    // 启动新进程，流式回调
    // ... 实现细节
}
```

#### OpenCodeAdapter

```go
type OpenCodeAdapter struct {
    cliPath     string
    baseAgent   *model.BaseAgent

    sessions    map[string]*openCodeSession
    mu          sync.RWMutex
}

type openCodeSession struct {
    id          string
    sessionKey  string          // 从CLI输出提取的sessionID
    status      SessionStatus
}

func (a *OpenCodeAdapter) StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error {
    // OpenCode首次不传 --session，从输出提取 sessionID
    args := []string{
        "run",
        "--format", "json",
        "--model", a.baseAgent.DefaultModel,
        req.Input,
    }

    // 启动进程，解析输出提取 sessionKey
    // ... 实现细节
}

func (a *OpenCodeAdapter) ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error {
    a.mu.RLock()
    session := a.sessions[sessionID]
    a.mu.RUnlock()

    // 使用 --session 恢复
    args := []string{
        "run",
        "--format", "json",
        "--session", session.sessionKey,
        input,
    }
    // ... 实现细节
}
```

## 需要变更的文件

### 需要删除

| 文件 | 原因 |
|-----|------|
| `internal/service/agent/interactive_session.go` | 会话逻辑移入Adapter |
| `internal/service/agent/session_manager.go` | 会话逻辑移入Adapter |
| `internal/service/agent/execution_context.go` | 不再需要区分执行上下文 |

### 需要修改

| 文件 | 修改内容 |
|-----|---------|
| `internal/service/agent/adapter.go` | 重定义 `AgentAdapter` 接口，添加会话管理方法 |
| `internal/service/agent/claude_adapter.go` | 实现完整的会话管理 |
| `internal/service/agent/open_code_adapter.go` | 实现完整的会话管理 |
| `internal/service/agent/execution_service.go` | 统一执行逻辑，删除模式判断，删除sessionIDs map |
| `internal/service/agent/orchestrator.go` | 删除 `interactiveManager` 字段，简化为委托给 `ExecutionService` |
| `internal/api/agent_handler.go` | 调试入口调用 `ExecutionService.SpawnAgent()` |

## 扩展新Agent类型指南

当需要添加新的Agent类型（如 `GeminiCode`）时，只需以下步骤：

### 1. 定义类型常量

在 `internal/model/base_agent.go` 添加：

```go
const (
    BaseAgentTypeClaudeCode BaseAgentType = "claude_code"
    BaseAgentTypeOpenCode   BaseAgentType = "opencode"
    BaseAgentTypeGeminiCode BaseAgentType = "gemini_code"  // 新增
)
```

### 2. 实现Adapter

创建 `internal/service/agent/gemini_adapter.go`：

```go
type GeminiAdapter struct {
    cliPath   string
    baseAgent *model.BaseAgent
    sessions  map[string]*geminiSession
    mu        sync.RWMutex
}

func NewGeminiAdapter(baseAgent *model.BaseAgent) *GeminiAdapter {
    return &GeminiAdapter{
        cliPath:   "gemini",
        baseAgent: baseAgent,
        sessions:  make(map[string]*geminiSession),
    }
}

// 实现 AgentAdapter 接口的所有方法
func (a *GeminiAdapter) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) { ... }
func (a *GeminiAdapter) ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) error { ... }
func (a *GeminiAdapter) StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error { ... }
func (a *GeminiAdapter) ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error { ... }
func (a *GeminiAdapter) StopSession(sessionID string) error { ... }
func (a *GeminiAdapter) GetSessionStatus(sessionID string) SessionStatus { ... }
func (a *GeminiAdapter) CheckHealth(ctx context.Context) error { ... }
```

### 3. 注册到工厂方法

在 `internal/service/agent/adapter.go` 的 `NewAdapter()` 添加case：

```go
func NewAdapter(baseAgent *model.BaseAgent) AgentAdapter {
    switch baseAgent.Type {
    case model.BaseAgentTypeClaudeCode:
        return NewClaudeAdapter(baseAgent)
    case model.BaseAgentTypeOpenCode:
        return NewOpenCodeAdapter(baseAgent)
    case model.BaseAgentTypeGeminiCode:
        return NewGeminiAdapter(baseAgent)  // 新增
    default:
        return nil
    }
}
```

**完成！** 上层代码（`ExecutionService`、`agent_handler`）无需任何修改。

## 架构图

```
┌─────────────────────────────────────────────────────────────┐
│                      API Layer                               │
│  ┌─────────────────┐    ┌─────────────────────────────────┐ │
│  │ workflow_handler │    │       agent_handler            │ │
│  └────────┬────────┘    └───────────────┬─────────────────┘ │
└───────────┼─────────────────────────────┼───────────────────┘
            │                             │
            └──────────────┬──────────────┘
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                    ExecutionService                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              SpawnAgent (统一入口)                     │   │
│  │  - resolveConfig()                                    │   │
│  │  - getOrCreateAdapter()                               │   │
│  │  - executeAgent()                                     │   │
│  │  - tryRouteToNextAgent() (有模板时执行)               │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────┬───────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    AgentAdapter 接口                         │
│  Execute | ExecuteWithStream | StartSession | ResumeSession │
│  StopSession | GetSessionStatus | CheckHealth               │
└──────────┬──────────────────────┬───────────────────────────┘
           ▼                      ▼
┌─────────────────────┐  ┌─────────────────────┐
│   ClaudeAdapter     │  │   OpenCodeAdapter   │
│  - sessions map     │  │  - sessions map     │
│  - CLI进程管理      │  │  - CLI进程管理      │
│  - 会话恢复         │  │  - 会话恢复         │
└─────────────────────┘  └─────────────────────┘
```

## 测试计划

1. **单元测试**
   - 各Adapter的会话管理方法测试
   - ExecutionService的统一执行逻辑测试

2. **集成测试**
   - 工作流场景Agent执行
   - 调试场景Agent执行
   - 会话恢复功能

3. **回归测试**
   - 确保现有功能不受影响