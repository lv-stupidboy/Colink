# ISDP ACP Adapter MCP 配置传递分析

## 概述

本文档记录了 ISDP 项目如何给 ACP adapter 传入 MCP 配置的完整分析，基于与 Clowder-AI 项目 A2A 机制的对比研究。

## 参考文档

- [Clowder-AI A2A 机制深度分析](./a2a-clowder-ai-analysis.md)
- [A2A 对比报告](./a2a-comparison-report.md)

## 当前状态分析

### 1. ACP 协议已支持 MCP 字段

**文件**: `internal/service/agent/acp_types.go:39`

```go
type acpNewSessionParams struct {
    CWD        string        `json:"cwd"`
    MCPServers []interface{} `json:"mcpServers"`  // ✓ 字段已定义
}
```

ACP 协议的 `session/new` 方法支持 `mcpServers` 参数，用于传入 MCP 服务器配置。

### 2. 当前传入空数组

**文件**: `internal/service/agent/acp_adapter.go`

```go
// Line 132-134 (ExecuteWithStream)
sessionNewResult, err := transport.SendRequest("session/new", &acpNewSessionParams{
    CWD:        req.WorkDir,
    MCPServers: []interface{}{},  // ❌ 空数组，未传入实际配置
})

// Line 262 (StartSession)
sessionNewResult, err := transport.SendRequest("session/new", &acpNewSessionParams{
    CWD:        req.WorkDir,
    MCPServers: []interface{}{},  // ❌ 空数组
})
```

两处调用都传入空数组，导致 CLI 无法获得 MCP 工具能力。

### 3. Callback Token 生成但未传递

**文件**: `internal/service/a2a/invocation_registry.go`

```go
func (r *InvocationRegistry) Register(record *InvocationRecord) (string, error) {
    // 生成 32 字节随机 token
    tokenBytes := make([]byte, 32)
    if _, err := rand.Read(tokenBytes); err != nil {
        return "", fmt.Errorf("failed to generate callback token: %w", err)
    }
    callbackToken := hex.EncodeToString(tokenBytes)
    record.CallbackToken = callbackToken
    return callbackToken, nil  // Token 存储在数据库，但未传递给 CLI
}
```

Token 生成逻辑存在，但未通过环境变量或 MCP 配置传递给 Agent CLI。

### 4. ExecutionRequest 缺少 MCP 相关字段

**文件**: `internal/service/agent/types.go`

```go
type ExecutionRequest struct {
    Config          *model.AgentRoleConfig
    BaseAgent       *model.BaseAgent
    Context         *ContextLayers
    Input           string
    WorkDir         string
    ConfigDir       string
    SessionID       string
    SessionStrategy SessionStrategy
    // ❌ 缺少 InvocationID、CallbackToken、MCPServers 字段
}
```

## 需要实现的完整方案

### 步骤 1：扩展 ExecutionRequest 结构

**修改文件**: `internal/service/agent/types.go`

```go
type ExecutionRequest struct {
    Config          *model.AgentRoleConfig
    BaseAgent       *model.BaseAgent
    Context         *ContextLayers
    Input           string
    WorkDir         string
    ConfigDir       string
    SessionID       string
    SessionStrategy SessionStrategy
    
    // 新增 MCP 相关字段
    InvocationID    string            // 调用 ID（用于认证）
    CallbackToken   string            // 回调 Token（用于认证）
    MCPServers      []MCPServerConfig // MCP 服务器配置
}

// MCPServerConfig MCP 服务器描述符（符合 ACP 协议格式）
type MCPServerConfig struct {
    Name    string            `json:"name"`
    Command string            `json:"command"`
    Args    []string          `json:"args"`
    Env     map[string]string `json:"env"`
}
```

### 步骤 2：ExecutionService 构建配置

**修改文件**: `internal/service/agent/execution_service.go`

在 `SpawnAgent` 方法中添加 MCP 配置构建逻辑：

```go
func (es *ExecutionService) SpawnAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
    // ... 现有配置解析逻辑 ...
    
    // 生成 Callback Token（32 字节随机）
    tokenBytes := make([]byte, 32)
    rand.Read(tokenBytes)
    callbackToken := hex.EncodeToString(tokenBytes)
    
    // 存储 Token 到 Invocation 记录
    invocation.CallbackToken = callbackToken
    es.invocationRepo.Update(ctx, invocation)
    
    // 构建 MCP 服务器配置
    mcpServers := []MCPServerConfig{
        {
            Name:    "isdp-callback",
            Command: "node",  // 或 "python" 根据实现选择
            Args:    []string{es.mcpServerPath},  // MCP 服务器脚本路径
            Env: map[string]string{
                "ISDP_API_URL":        es.config.APIURL,        // 回调 API 地址
                "ISDP_INVOCATION_ID":  invocation.ID.String(),  // 调用 ID
                "ISDP_CALLBACK_TOKEN": callbackToken,           // 认证 Token
            },
        },
    }
    
    // 构建执行请求
    execReq := &ExecutionRequest{
        Input:           buildAgentInput(ctx, req.ThreadID, req.Input),
        WorkDir:         projectPath,
        Config:          config,
        BaseAgent:       baseAgent,
        Context:         layers,
        InvocationID:    invocation.ID.String(),
        CallbackToken:   callbackToken,
        MCPServers:      mcpServers,  // ✓ 传入 MCP 配置
    }
    
    // 执行 Agent...
}
```

### 步骤 3：ACP Adapter 使用配置

**修改文件**: `internal/service/agent/acp_adapter.go`

添加辅助函数并修改 `ExecuteWithStream` 和 `StartSession`：

```go
// buildMCPServerDescriptors 构建 ACP 协议格式的 MCP 服务器描述符
func buildMCPServerDescriptors(servers []MCPServerConfig) []interface{} {
    descriptors := make([]interface{}, 0, len(servers))
    for _, server := range servers {
        descriptors = append(descriptors, map[string]interface{}{
            "name":    server.Name,
            "command": server.Command,
            "args":    server.Args,
            "env":     server.Env,
        })
    }
    return descriptors
}

func (a *BaseACPAdapter) ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) (*ExecutionResult, error) {
    // ... 现有握手逻辑 ...
    
    sessionNewResult, err := transport.SendRequest("session/new", &acpNewSessionParams{
        CWD:        req.WorkDir,
        MCPServers: buildMCPServerDescriptors(req.MCPServers),  // ✓ 传入实际配置
    })
    // ... 后续逻辑 ...
}

func (a *BaseACPAdapter) StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error {
    // ... 现有握手逻辑 ...
    
    sessionNewResult, err := transport.SendRequest("session/new", &acpNewSessionParams{
        CWD:        req.WorkDir,
        MCPServers: buildMCPServerDescriptors(req.MCPServers),  // ✓ 传入实际配置
    })
    // ... 后续逻辑 ...
}
```

### 步骤 4：创建 MCP Server 包

需要新建 MCP 服务器实现（类似 Clowder-AI 的 `packages/mcp-server`）：

```
internal/mcp-server/
├── server.go              # MCP 服务器入口
├── tools/
│   ├── post_message.go    # A2A 消息发送工具
│   ├── thread_context.go  # 获取对话上下文
│   └── update_task.go     # 更新任务状态
│   └── request_permission.go  # 请求危险操作授权
└── auth.go                # Callback Token 验证
```

**MCP 工具设计**（参考 PRD 文档）：

| 工具名 | 功能 | Clowder-AI 等价工具 |
|--------|------|---------------------|
| `post_message` | 发送 A2A 消息（含 @mention） | `cat_cafe_post_message` |
| `thread_context` | 获取对话历史 | `cat_cafe_thread_history` |
| `update_task` | 更新任务状态 | `cat_cafe_update_status` |
| `request_permission` | 请求危险操作授权 | `cat_cafe_request_permission` |

**认证机制**（参考 PRD 文档）：

```go
// internal/mcp-server/auth.go

// VerifyCallbackToken 验证回调 Token
func VerifyCallbackToken(invocationID, callbackToken string) error {
    // 1. 从数据库查询 invocation 记录
    invocation, err := invocationRepo.FindByID(ctx, invocationID)
    if err != nil {
        return ErrInvalidInvocation
    }
    
    // 2. 验证 Token 匹配
    if invocation.CallbackToken != callbackToken {
        return ErrInvalidToken
    }
    
    // 3. 验证 Invocation 状态（必须是 running）
    if invocation.Status != model.InvocationStatusRunning {
        return ErrInvocationNotActive
    }
    
    return nil
}
```

## 数据流对比

### Clowder-AI 的完整流程

```
invokeSingleCat → buildInvocationContext → invoke CLI
                                      ↓
                          环境变量注入:
                          CAT_CAFE_INVOCATION_ID=xxx
                          CAT_CAFE_CALLBACK_TOKEN=yyy
                                      ↓
                          CLI 启动，加载 MCP tools
                                      ↓
                          Agent 通过 MCP tool 调用 cat_cafe_post_message
                                      ↓
                          callback-a2a-trigger.ts 验证 token
                                      ↓
                          enqueueA2ATargets → pushToWorklist
                                      ↓
                          WorklistRegistry 扩展工作列表
                                      ↓
                          routeSerial 继续执行下一个 Agent
```

### ISDP 需要实现

```
ExecutionService.SpawnAgent → 生成 callbackToken → 构建 MCPServers config
                                      ↓
                          ExecutionRequest.MCPServers = [{
                              name: "isdp-callback",
                              env: {
                                  ISDP_INVOCATION_ID: xxx,
                                  ISDP_CALLBACK_TOKEN: yyy
                              }
                          }]
                                      ↓
                          ACP Adapter → session/new → CLI 启动
                                      ↓
                          CLI 加载 MCP tools（通过 ACP mcpServers 参数）
                                      ↓
                          Agent 通过 MCP tool 调用 post_message
                                      ↓
                          callback_handler.go 验证 invocationID + callbackToken
                                      ↓
                          触发 A2A 路由逻辑
                                      ↓
                          ExecutionService 继续执行下游 Agent
```

## 实现优先级

| 优先级 | 组件 | 工作量 | 说明 |
|--------|------|--------|------|
| P0 | ExecutionRequest 扩展 | 小 | 基础结构，必须先完成 |
| P0 | ACP Adapter 修改 | 小 | 传入 MCPServers 配置 |
| P1 | ExecutionService 构建 | 中 | 生成 token、构建 config |
| P1 | MCP Server 包 | 大 | 需实现多个工具 |
| P2 | 数据库字段更新 | 小 | AgentInvocation 表添加 CallbackToken 字段 |

## 关键差异总结

| 方面 | Clowder-AI | ISDP（当前） | ISDP（目标） |
|------|------------|--------------|--------------|
| MCP 配置传递 | 环境变量注入 | 空数组 | ACP mcpServers 参数 |
| Token 生成时机 | invoke 时 | Register 时 | SpawnAgent 时 |
| Token 存储位置 | 内存（WorklistRegistry） | 数据库 | 数据库 + ExecutionRequest |
| 认证字段名 | CAT_CAFE_* | 无 | ISDP_* |
| 工具数量 | 20+ | 0（端点未使用） | 4（基础） |

## 相关文件路径

| 文件 | 职责 |
|------|------|
| `internal/service/agent/types.go` | ExecutionRequest 结构定义 |
| `internal/service/agent/acp_types.go` | ACP 协议类型定义 |
| `internal/service/agent/acp_adapter.go` | ACP adapter 实现 |
| `internal/service/agent/execution_service.go` | Agent 执行服务 |
| `internal/service/a2a/invocation_registry.go` | Invocation 记录管理 |
| `internal/api/callback_handler.go` | Callback 端点处理器 |

## 后续工作

1. **MCP Server 实现**: 创建 `internal/mcp-server/` 包，实现基础工具
2. **数据库变更**: `agent_invocations` 表添加 `callback_token` 字段
3. **配置管理**: MCP 服务器脚本路径配置化
4. **测试验证**: 确保 A2A 消息能正确触发下游 Agent

---

**文档创建日期**: 2026-04-14  
**参考项目**: Clowder-AI (猫咖系统)