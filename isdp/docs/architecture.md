# ISDP 架构概览

## 项目结构

```
isdp/
├── cmd/                    # 命令行工具
│   ├── server/            # 主服务入口
│   ├── update_agent_prompts/  # 更新 Agent 提示词
│   ├── setup_transitions/ # 配置工作流转换规则
│   └── show_agents/       # 查看所有 Agent 配置
├── internal/
│   ├── api/               # HTTP API 处理器
│   ├── model/             # 数据模型
│   ├── repo/              # 数据访问层
│   ├── service/
│   │   ├── agent/         # Agent 核心服务
│   │   ├── project/       # 项目服务
│   │   └── thread/        # 线程服务
│   └── ws/                # WebSocket 服务
├── web/                   # 前端应用
├── data/                  # 数据库文件
├── logs/                  # 日志文件
└── docs/                  # 文档
```

## 核心组件

### 1. Agent 服务 (`internal/service/agent/`)

| 文件 | 功能 |
|------|------|
| `execution_service.go` | 统一执行服务，处理 Agent 调用和 A2A 路由 |
| `claude_adapter.go` | Claude CLI 适配器，处理流式输出 |
| `orchestrator.go` | Agent 编排器，管理 Agent 生命周期 |
| `workflow.go` | 工作流引擎 |
| `types.go` | 类型定义 |

### 2. 数据模型 (`internal/model/`)

| 模型 | 描述 |
|------|------|
| `BaseAgent` | 基础 Agent 配置（CLI 路径、模型等） |
| `AgentRoleConfig` | Agent 角色配置（SystemPrompt） |
| `WorkflowTemplate` | 工作流模板 |
| `Transition` | Agent 间转换规则 |
| `Thread` | 开发会话 |
| `AgentInvocation` | Agent 调用记录 |

### 3. API 层 (`internal/api/`)

| 处理器 | 路由 |
|--------|------|
| `AgentHandler` | `/api/v1/agents/*` |
| `WorkflowHandler` | `/api/v1/workflows/*` |
| `ThreadHandler` | `/api/v1/threads/*` |
| `MessageHandler` | `/api/v1/messages/*` |
| `ProjectHandler` | `/api/v1/projects/*` |

## 数据流

### 用户消息处理流程

```
用户发送消息
      │
      ▼
MessageHandler.Create()
      │
      ▼
Orchestrator.SpawnAgentForUserMessage()
      │
      ├──▶ 获取 Thread 的 WorkflowTemplateID
      │
      ├──▶ 获取工作流模板中的第一个 Agent
      │
      ▼
ExecutionService.SpawnAgent()
      │
      ├──▶ 创建 AgentInvocation 记录
      │
      ├──▶ 构建 ContextLayers (系统提示、历史消息等)
      │
      ▼
ClaudeAdapter.ExecuteWithStream()
      │
      ├──▶ 调用 Claude CLI
      │
      ├──▶ 解析流式输出
      │
      ├──▶ 广播 WebSocket 消息
      │
      ▼
ExecutionService.checkRouting()
      │
      ├──▶ 检查 @mention
      │
      ├──▶ 检查 Transitions
      │
      ▼
触发下一个 Agent (如果有)
```

### 流式输出流程

```
Claude CLI (--output-format stream-json)
      │
      ▼
stdout (JSON Lines)
      │
      ▼
parseStreamJSONLine()
      │
      ├──▶ stream_event.content_block_start
      │         └── Chunk{Type: "thinking" | "tool_use"}
      │
      ├──▶ stream_event.content_block_delta
      │         └── Chunk{Type: "text", Content: "..."}
      │
      ▼
broadcastChunk() ──▶ WebSocket ──▶ 前端实时显示
```

## WebSocket 消息类型

| 类型 | 描述 |
|------|------|
| `agent_status` | Agent 状态变更 (started/completed/failed) |
| `agent_output_chunk` | 流式输出块 |
| `agent_message` | 完整的 Agent 消息 |

## 配置管理

### 环境变量

| 变量 | 描述 |
|------|------|
| `ANTHROPIC_API_KEY` | Claude API 密钥 |
| `ANTHROPIC_API_URL` | API 端点（可选） |
| `CLAUDE_CODE_GIT_BASH_PATH` | Git Bash 路径（Windows） |

### Claude CLI 参数

```go
args := []string{
    "--print",
    "--output-format", "stream-json",
    "--verbose",
    "--include-partial-messages",    // 真正的流式输出
    "--dangerously-skip-permissions", // 跳过权限检查
    "--no-session-persistence",       // 禁用会话持久化
    "--model", model,
}
```

## 扩展指南

### 添加新的 Agent 类型

1. 在 `internal/model/base_agent.go` 添加类型定义
2. 在 `internal/service/agent/adapter.go` 实现适配器
3. 在 `NewAdapter()` 工厂方法中注册

### 添加新的路由规则

1. 扩展 `Transition` 模型
2. 修改 `checkRouting()` 或 `checkSignalRouting()` 方法
3. 更新工作流模板配置

## 相关文档

- [A2A 协同机制](./a2a-collaboration.md)
- [API 文档](./api.md) (待补充)
- [开发指南](./development.md) (待补充)