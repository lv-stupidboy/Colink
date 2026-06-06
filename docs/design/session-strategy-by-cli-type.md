# Session 恢复策略：按 CLI 类型差异化处理

## 一、核心原则

**不同 CLI 工具的 session 能力不同，需要差异化处理：**

| CLI 类型 | Session 能力 | 恢复策略 |
|----------|-------------|---------|
| **Claude CLI** (`claude_code`) | 原生 `--resume` 支持 | 使用 CLI 原生 resume（优先） |
| **OpenCode ACP** (`open_code`) | 无 session 支持 | 长连接 + Prompt 注入恢复 |
| **CodeAgent** (`code_agent`) | 无 session 支持（基于 OpenCode） | 长连接 + Prompt 注入恢复（同 OpenCode） |
| **Hermes** (`hermes`) | 待确认 | 待定 |
| **OpenClaw** (`open_claw`) | 待确认 | 待定 |

> **注意**：CodeAgent 是公司内部基于 OpenCode 定制开发的 CLI 工具，adapter 代码不在当前仓库。
> CodeAgent 使用和 OpenCode 相同的 session 处理机制（长连接 + Prompt 注入恢复）。

## 二、Claude CLI 策略：原生 Resume

### 2.1 现有机制（保持不变）

Claude CLI adapter 已支持 `--resume`：

```go
// plugins/claude_code/adapter.go (现有代码)
if req.SessionID != "" {
    sessionID = req.SessionID
    args = append(args, "--resume", sessionID)  // 使用 CLI 原生 resume
}
```

### 2.2 Session ID 管理（需要增强）

**现状问题**：
- Session ID 存储在内存（`cliSessions` map），服务重启后丢失
- 应该持久化到数据库，确保服务重启后仍可 resume

**改进方案**：

```go
// ExecutionService 增强 session ID 持久化

func (es *ExecutionService) executeAgent(...) {
    // ...

    // 1. 查询数据库中是否有该 thread+agent 的 session ID
    if req.SessionStrategy == SessionStrategyResume {
        sessionRecord, err := es.repo.FindSessionRecord(req.ThreadID, config.ID)
        if err == nil && sessionRecord.CliSessionID != "" {
            sessionID = sessionRecord.CliSessionID
            logInfo("Claude: 从数据库恢复 session ID", zap.String("sessionId", sessionID))
        }
    }

    // 2. 执行完成后，持久化 session ID
    if result != nil && result.SessionID != "" {
        es.repo.SaveSessionRecord(req.ThreadID, config.ID, result.SessionID, model.AgentTypeClaudeCode)
    }

    // ...
}
```

### 2.3 Session 过期管理

Claude CLI 的 session 有有效期（默认 7 天），需要管理：

```go
// 定期清理过期的 session records
func (es *ExecutionService) cleanupExpiredSessions() {
    expiredRecords := es.repo.FindExpiredSessionRecords(7 * 24 * time.Hour)
    for _, record := range expiredRecords {
        es.repo.DeleteSessionRecord(record.ID)
        logInfo("Session expired, cleaned up", zap.String("sessionId", record.CliSessionID))
    }
}
```

### 2.4 Claude 不使用长连接

**原因**：
- Claude CLI 的 `--resume` 已经高效（约 2-3 秒恢复）
- 保持进程存活不如 resume 灵活（服务重启后进程丢失）
- Prompt 注入恢复不如 CLI 内部恢复完整

**处理方式**：
- 每次 prompt 都启动新进程（使用 `--resume` 恢复上下文）
- 进程执行完成后立即退出
- Session ID 持久化到数据库

---

## 三、OpenCode ACP 策略：长连接 + Prompt 注入恢复

### 3.1 适用场景

OpenCode ACP 不支持 `--resume`，需要完整的长连接方案：

1. **长连接**：保持进程存活，避免上下文丢失
2. **定期持久化**：进程存活期间定期保存对话内容
3. **断连恢复**：进程意外断连时，通过 prompt 注入恢复

### 3.2 实现要点（保持之前的设计）

```
SessionPool → LongRunningSession → Process (保持存活)
     ↓              ↓
  空闲超时      ConversationBuffer
     ↓              ↓
  Sealing       定期持久化
     ↓              ↓
  Sealed        断连时 → Recovering → Prompt 注入
```

---

## 四、CodeAgent 类型扩展设计

### 4.1 CodeAgent 简介

**CodeAgent** 是公司内部基于 OpenCode 定制开发的 CLI 工具：
- 基于 OpenCode 源码二次开发
- 添加公司特定的功能和配置
- 使用 ACP 协议（与 OpenCode 相同）
- **不支持原生 session resume**

### 4.2 Adapter 代码位置

CodeAgent adapter 代码不在当前仓库（isdp），位于公司内部私有仓库：
- 仓库地址：`git.company.com/code-agent/adapter`（示例）
- 实现方式：继承或复制 OpenCode adapter，添加定制逻辑

### 4.3 扩展机制设计

**目标**：即使 adapter 代码不在当前仓库，也能正确识别和处理 CodeAgent

```go
// internal/service/agent/session_strategy.go

// SessionStrategyConfig 扩展字段
type SessionStrategyConfig struct {
    UseLongRunning   bool           // 是否使用长连接
    UseNativeResume  bool           // 是否使用 CLI 原生 resume
    IdleTimeout      int            // 空闲超时（长连接模式）
    ResumeExpiry     int            // Resume 有效期（原生 resume 模式）
    ParentType       string         // 父类型（继承策略，如 "open_code"）
    ACPProtocol      bool           // 是否使用 ACP 协议
}

// 预定义的 OpenCode 兼容类型列表
// 这些类型使用和 OpenCode 相同的 session 策略
var openCodeCompatibleTypes = []model.BaseAgentType{
    model.BaseAgentType("open_code"),
    model.BaseAgentType("code_agent"),
    // 未来可能添加更多基于 OpenCode 的衍生类型
}

// IsOpenCodeCompatible 判断类型是否使用 OpenCode 兼容策略
func IsOpenCodeCompatible(agentType model.BaseAgentType) bool {
    for _, t := range openCodeCompatibleTypes {
        if t == agentType {
            return true
        }
    }
    return false
}

// GetSessionStrategy 获取指定类型的 session 策略
// 支持动态扩展：如果类型未在 defaultStrategies 中定义，检查是否为 OpenCode 兼容类型
func GetSessionStrategy(agentType model.BaseAgentType) SessionStrategyConfig {
    // 1. 查找预定义策略
    if config, exists := defaultStrategies[agentType]; exists {
        return config
    }

    // 2. 检查是否为 OpenCode 兼容类型
    if IsOpenCodeCompatible(agentType) {
        return defaultStrategies[model.BaseAgentType("open_code")]
    }

    // 3. 默认策略：不使用长连接，不使用 resume（每次新进程）
    return SessionStrategyConfig{
        UseLongRunning:  false,
        UseNativeResume: false,
    }
}
```

### 4.4 CodeAgent Plugin 注册（外部仓库）

CodeAgent adapter 在公司私有仓库中注册：

```go
// 公司私有仓库: code_agent/plugin.go

package code_agent

import (
    "github.com/anthropic/isdp/internal/model"
    "github.com/anthropic/isdp/internal/service/agent"
    "github.com/anthropic/isdp/internal/service/agent/plugins/acp"
)

const Type model.BaseAgentType = "code_agent"

func init() {
    // CodeAgent 使用 ACP 协议，继承 OpenCode 的基本配置
    agent.RegisterPlugin(agent.PluginMeta{
        Type:   Type,
        Name:   "CodeAgent",
        Factory: func(baseAgent *model.BaseAgent) agent.AgentAdapter {
            // CodeAgent adapter 实现公司定制逻辑
            return NewCodeAgentAdapter(baseAgent)
        },
        ConfigGeneratorFactory: func() agent.ConfigGenerator {
            // 使用 OpenCode 的配置生成器（或定制版本）
            return NewCodeAgentConfigGenerator()
        },
    })
}

// CodeAgentAdapter 继承 BaseACPAdapter
type CodeAgentAdapter struct {
    *acp.BaseACPAdapter
    // 公司定制字段
    companySpecificConfig CompanyConfig
}

func NewCodeAgentAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
    // 使用 OpenCode 兼容的 ACP 配置
    config := acp.AcpAdapterConfig{
        CliPath: baseAgent.CliPath,
        BuildArgs: func(req *agent.ExecutionRequest) []string {
            return []string{"acp"}  // ACP 协议
        },
        BuildEnv: func(req *agent.ExecutionRequest) []string {
            // 公司特定环境变量
            env := []string{
                "CODE_AGENT_MODE=enterprise",
                // ... 其他公司定制配置
            }
            return env
        },
    }

    return &CodeAgentAdapter{
        BaseACPAdapter: acp.NewBaseACPAdapter(config, baseAgent),
    }
}
```

### 4.5 自动类型识别

**SessionManager 在运行时自动识别类型**：

```go
func (sm *SessionManager) GetOrCreateSession(ctx context.Context, req *SpawnRequest, baseAgent *model.BaseAgent) (SessionHandle, error) {
    strategy := GetSessionStrategy(baseAgent.Type)

    if strategy.UseNativeResume {
        // Claude CLI: 使用原生 resume
        return sm.getOrCreateResumeSession(ctx, req, baseAgent)
    }

    if strategy.UseLongRunning {
        // OpenCode / CodeAgent: 使用长连接
        // 使用相同的 SessionPool 和 Recovery 逻辑
        return sm.getOrCreateLongRunningSession(ctx, req, baseAgent)
    }

    // 默认：无 session 支持
    return sm.createNewSession(ctx, req, baseAgent)
}
```

---

## 五、统一 Session 管理

### 5.1 SessionManager（统一入口）

```go
// internal/service/agent/session_manager.go

type SessionManager struct {
    pool       *SessionPool        // 长连接池（OpenCode ACP 使用）
    repo       *SessionRecordRepo  // Session ID 持久化（Claude 使用）
    strategies map[model.AgentType]SessionStrategyConfig
}

type SessionStrategyConfig struct {
    UseLongRunning   bool   // 是否使用长连接
    UseNativeResume  bool   // 是否使用 CLI 原生 resume
    IdleTimeout      int    // 空闲超时（长连接模式）
    ResumeExpiry     int    // Resume 有效期（原生 resume 模式）
}

// 默认策略配置
var defaultStrategies = map[model.BaseAgentType]SessionStrategyConfig{
    model.BaseAgentType("claude_code"): {
        UseLongRunning:   false,  // 不使用长连接
        UseNativeResume:  true,   // 使用原生 resume
        ResumeExpiry:     168,    // 7 天 = 168 小时
    },
    model.BaseAgentType("open_code"): {
        UseLongRunning:   true,   // 使用长连接
        UseNativeResume:  false,  // 不使用原生 resume
        IdleTimeout:      600,    // 10 分钟
    },
    model.BaseAgentType("code_agent"): {
        UseLongRunning:   true,   // 使用长连接（同 OpenCode）
        UseNativeResume:  false,  // 不使用原生 resume
        IdleTimeout:      600,    // 10 分钟
        ParentType:       "open_code", // 继承 OpenCode 的处理逻辑
    },
}

// IsOpenCodeCompatible 判断是否为 OpenCode 兼容类型（使用相同 session 策略）
func (c SessionStrategyConfig) IsOpenCodeCompatible() bool {
    return c.UseLongRunning && !c.UseNativeResume
}
```

### 5.2 GetOrCreateSession（统一入口）

```go
func (sm *SessionManager) GetOrCreateSession(ctx context.Context, req *SpawnRequest, baseAgent *model.BaseAgent) (SessionHandle, error) {
    strategy := sm.strategies[baseAgent.Type]

    if strategy.UseNativeResume {
        // Claude CLI: 使用原生 resume
        return sm.getOrCreateResumeSession(ctx, req, baseAgent)
    }

    if strategy.UseLongRunning {
        // OpenCode ACP: 使用长连接
        return sm.getOrCreateLongRunningSession(ctx, req, baseAgent)
    }

    // 默认：无 session 支持（每次新进程）
    return sm.createNewSession(ctx, req, baseAgent)
}

func (sm *SessionManager) getOrCreateResumeSession(ctx context.Context, req *SpawnRequest, baseAgent *model.BaseAgent) (*ResumeSessionHandle, error) {
    // 1. 从数据库查询 session ID
    record, err := sm.repo.FindByThreadAndAgent(req.ThreadID, req.ConfigID)
    if err == nil && record != nil && record.CliSessionID != "" {
        // 有历史 session，检查是否过期
        if time.Since(time.Unix(record.LastActiveAt, 0)) < time.Duration(strategy.ResumeExpiry)*time.Hour {
            return &ResumeSessionHandle{
                SessionID: record.CliSessionID,
                Strategy:  SessionStrategyResume,
            }, nil
        }
    }

    // 2. 无有效历史 session，创建新 session
    return &ResumeSessionHandle{
        SessionID: uuid.New().String(),
        Strategy:  SessionStrategyNew,
    }, nil
}

func (sm *SessionManager) getOrCreateLongRunningSession(ctx context.Context, req *SpawnRequest, baseAgent *model.BaseAgent) (*LongRunningSession, error) {
    sessionKey := fmt.Sprintf("%s:%s", req.ThreadID, req.ConfigID)

    // 1. 检查 SessionPool 是否有活跃 session
    session := sm.pool.Get(sessionKey)
    if session != nil && session.Status == SessionStatusActive {
        return session, nil  // 直接复用
    }

    // 2. 检查数据库是否有 Sealed session（可恢复）
    sealed, err := sm.repo.FindSealedSession(req.ThreadID, req.ConfigID)
    if err == nil && sealed != nil {
        return sm.pool.RecoverFromSealed(ctx, sealed, req)
    }

    // 3. 创建新 session
    return sm.pool.CreateNew(ctx, sessionKey, req)
}
```

### 5.3 SessionHandle 接口

```go
// SessionHandle 会话句柄接口
type SessionHandle interface {
    GetSessionID() string
    GetStrategy() SessionStrategy
    IsLongRunning() bool
}

// ResumeSessionHandle Claude CLI 的 resume session
type ResumeSessionHandle struct {
    SessionID string
    Strategy  SessionStrategy  // new 或 resume
}

func (h *ResumeSessionHandle) GetSessionID() string { return h.SessionID }
func (h *ResumeSessionHandle) GetStrategy() SessionStrategy { return h.Strategy }
func (h *ResumeSessionHandle) IsLongRunning() bool { return false }

// LongRunningSession OpenCode ACP 的长连接 session
type LongRunningSession struct {
    // ... 之前的设计
}

func (s *LongRunningSession) GetSessionID() string { return s.AcpSessionID }
func (s *LongRunningSession) GetStrategy() SessionStrategy { return SessionStrategyResume }
func (s *LongRunningSession) IsLongRunning() bool { return true }
```

---

## 六、ExecutionService 改造

### 6.1 使用 SessionManager

```go
func (es *ExecutionService) executeAgent(...) {
    // ...

    // 1. 通过 SessionManager 获取 session
    handle, err := es.sessionManager.GetOrCreateSession(ctx, req, baseAgent)
    if err != nil {
        return nil, err
    }

    // 2. 根据 handle 类型决定执行方式
    if handle.IsLongRunning() {
        // OpenCode ACP: 长连接模式
        session := handle.(*LongRunningSession)
        return es.executeWithLongRunning(ctx, session, invocation, config, baseAgent, req)
    } else {
        // Claude CLI: 原生 resume 模式
        resumeHandle := handle.(*ResumeSessionHandle)
        req.SessionID = resumeHandle.GetSessionID()
        req.SessionStrategy = resumeHandle.GetStrategy()
        return es.executeWithResume(ctx, invocation, config, baseAgent, req)
    }
}
```

### 6.2 执行完成后保存 Session ID

```go
func (es *ExecutionService) onExecutionComplete(...) {
    // ...

    // 保存 session ID 到数据库（Claude CLI）
    if !baseAgent.Type.UseLongRunning() && result.SessionID != "" {
        es.sessionManager.SaveSessionRecord(threadID, config.ID, result.SessionID, baseAgent.Type)
    }

    // 长连接 session 进入 Idle 状态（OpenCode ACP）
    if baseAgent.Type.UseLongRunning() {
        es.sessionManager.MarkIdle(sessionKey)
    }
}
```

---

## 七、数据模型调整

### 7.1 SessionRecord 表（统一存储）

```go
type SessionRecord struct {
    ID            uuid.UUID         `json:"id" gorm:"primaryKey"`
    ThreadID      uuid.UUID         `json:"threadId" gorm:"index"`
    AgentID       uuid.UUID         `json:"agentId" gorm:"index"`
    AgentType     model.BaseAgentType `json:"agentType"` // claude_code, open_code, code_agent

    // Claude CLI resume 模式
    CliSessionID  string            `json:"cliSessionId"` // CLI 的 session ID
    ResumeExpiry  int64             `json:"resumeExpiry"` // 过期时间戳

    // OpenCode ACP / CodeAgent 长连接模式
    Status        SessionStatus     `json:"status"`       // active, idle, sealed
    Conversation  []byte            `json:"conversation"` // 对话历史（Sealed 状态才有）
    KeyEntities   []byte            `json:"keyEntities"`  // 关键实体

    // 进程信息（长连接模式）
    ProcessPID    int               `json:"processPid"`   // 进程 PID（仅供参考）

    // 时间
    CreatedAt     int64             `json:"createdAt"`
    LastActiveAt  int64             `json:"lastActiveAt"`
    SealedAt      int64             `json:"sealedAt,omitempty"`
}
```

---

## 八、总结

| CLI 类型 | Session 策略 | 需要改动 |
|----------|-------------|---------|
| **Claude CLI** (`claude_code`) | 原生 `--resume` | Session ID 持久化到数据库 |
| **OpenCode ACP** (`open_code`) | 长连接 + Prompt 注入恢复 | 完整实现之前的设计 |
| **CodeAgent** (`code_agent`) | 长连接 + Prompt 注入恢复（同 OpenCode） | 无需额外改动，自动继承 OpenCode 策略 |
| **其他** | 按能力选择策略 | 待确认 |

**核心改动**：
1. 创建 `SessionManager` 统一管理不同 CLI 的 session 策略
2. Claude CLI：增强 session ID 持久化（不使用长连接）
3. OpenCode ACP / CodeAgent：完整实现长连接方案
4. 数据模型：`SessionRecord` 表支持两种模式
5. **扩展机制**：通过 `openCodeCompatibleTypes` 列表自动识别 CodeAgent 等衍生类型