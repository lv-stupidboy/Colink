# Agent-to-Agent (A2A) 协同机制

## 概述

ISDP 实现了灵活的 Agent-to-Agent (A2A) 协同机制，允许多个 Agent 按照预定义的工作流协作完成任务。A2A 协同支持多种触发方式和工作流模式：

1. **@mention 路由**：Agent 在输出中主动 `@目标Agent` 触发协作
2. **Transitions 自动路由**：基于工作流模板配置，Agent 完成后自动触发下一个 Agent

## 工作流模式

### 支持的模式

| 模式 | 类型 | 描述 |
|------|------|------|
| 顺序执行 | `sequence` | A → B → C，一个 Agent 完成后触发下一个 |
| 并行执行（分支） | `parallel` | A → B, A → C，一个 Agent 同时触发多个 Agent |
| 汇聚执行 | `merge` | A → C, B → C，等待多个上游 Agent 完成后再执行 |
| 条件路由 | - | 基于输出内容决定是否触发 |

### 流程图

```
┌─────────────────────────────────────────────────────────────────────┐
│                         顺序执行 (sequence)                          │
│                                                                     │
│   需求分析师 ──────▶ 架构师 ──────▶ 开发工程师 ──────▶ 测试工程师    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                         并行执行 (parallel)                          │
│                                                                     │
│                           ┌──▶ 前端工程师                           │
│                          /                                          │
│         架构师 ──────────┤                                           │
│                          \                                          │
│                           └──▶ 后端工程师                           │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                           汇聚执行 (merge)                           │
│                                                                     │
│         前端工程师 ─────┐                                            │
│                          \                                          │
│                           ├──▶ 集成测试工程师                        │
│                          /                                          │
│         后端工程师 ─────┘                                            │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## 架构设计

### 核心模型

```
┌─────────────────────────────────────────────────────────────┐
│                    WorkflowTemplate                         │
│  ┌─────────────────┐    ┌─────────────────────────────┐    │
│  │    AgentIDs     │    │       Transitions           │    │
│  │  [Agent1,       │    │  from_agent_id: Agent1.ID   │    │
│  │   Agent2,       │───▶│  to_agent_id: Agent2.ID     │    │
│  │   Agent3]       │    │  type: "parallel"           │    │
│  └─────────────────┘    │  condition: "contains:测试" │    │
│                         │  wait_for: [Agent1, Agent2]  │    │
│                         └─────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                         Thread                              │
│  workflow_template_id → 关联的工作流模板                     │
│  当前执行中的 Agent 调用链                                   │
└─────────────────────────────────────────────────────────────┘
```

### 数据模型

#### Transition（转换规则）

```go
type TransitionType string

const (
    TransitionTypeSequence TransitionType = "sequence" // 顺序执行
    TransitionTypeParallel TransitionType = "parallel" // 并行执行（分支）
    TransitionTypeMerge    TransitionType = "merge"    // 汇聚执行
)

type Transition struct {
    FromAgentID     string         `json:"from_agent_id"`     // 源 Agent ID
    Trigger         string         `json:"trigger"`           // 触发条件描述
    ToAgentID       string         `json:"to_agent_id"`       // 目标 Agent ID
    MessageTemplate string         `json:"message_template"`  // 消息模板 (可选)
    Description     string         `json:"description"`       // 转换描述
    Type            TransitionType `json:"type"`              // 转换类型
    Condition       string         `json:"condition"`         // 条件表达式 (可选)
    WaitFor         []string       `json:"wait_for"`          // 等待的 Agent ID 列表 (汇聚)
}
```

#### WorkflowTemplate（工作流模板）

```go
type WorkflowTemplate struct {
    ID            uuid.UUID       `json:"id"`
    Name          string          `json:"name"`
    AgentIDs      json.RawMessage `json:"agent_ids"`      // Agent 实例 ID 列表
    Transitions   json.RawMessage `json:"transitions"`    // Agent 间转换规则
    // ...
}
```

#### A2AContext（A2A 上下文）

```go
type A2AContext struct {
    Depth           int                // 当前深度
    InvokedAgents   map[uuid.UUID]bool // 已调用的 Agent ID 集合
    CompletedAgents map[uuid.UUID]bool // 已完成的 Agent ID 集合（用于汇聚判断）
}
```

## A2A 路由流程

### 1. Agent 执行完成后的路由检查

```
Agent 执行完成
      │
      ▼
checkRouting()
      │
      ├──▶ parseMentions()  ──▶ 发现 @mention?
      │                              │
      │                              ├── Yes ──▶ checkMentionRouting()
      │                              │                  │
      │                              │                  ▼
      │                              │           验证目标 Agent 在工作流中
      │                              │                  │
      │                              │                  ▼
      │                              │           触发目标 Agent
      │                              │
      │                              └── No ──┐
      │                                       │
      ◀───────────────────────────────────────┘
      │
      ▼
checkSignalRouting()
      │
      ├──▶ 获取 Transitions
      │
      ├──▶ 记录当前 Agent 已完成 (CompletedAgents)
      │
      ▼
遍历所有匹配的 Transitions
      │
      ├──▶ 检查条件路由 (Condition)
      │         │
      │         └── 条件不匹配 → 跳过
      │
      ├──▶ 检查汇聚条件 (WaitFor)
      │         │
      │         └── 上游未完成 → 等待
      │
      ├──▶ 去重检查
      │         │
      │         └── 已调用过 → 跳过
      │
      ▼
批量触发所有目标 Agent（并行）
```

### 2. 条件路由

支持基于输出内容的条件匹配：

| 条件格式 | 描述 | 示例 |
|----------|------|------|
| `contains:关键词` | 输出包含指定关键词 | `contains:测试通过` |
| `regex:正则表达式` | 正则表达式匹配 | `regex:状态:.*成功` |
| `关键词` | 默认使用 contains 匹配 | `测试通过` |

```go
func (es *ExecutionService) matchCondition(output, condition string) bool {
    if strings.HasPrefix(condition, "contains:") {
        keyword := strings.TrimPrefix(condition, "contains:")
        return strings.Contains(output, keyword)
    }
    if strings.HasPrefix(condition, "regex:") {
        pattern := strings.TrimPrefix(condition, "regex:")
        matched, _ := regexp.MatchString(pattern, output)
        return matched
    }
    return strings.Contains(output, condition)
}
```

### 3. 汇聚机制

当目标 Agent 需要等待多个上游 Agent 完成时：

```json
{
  "type": "merge",
  "from_agent_id": "前端工程师ID",
  "to_agent_id": "集成测试ID",
  "wait_for": ["前端工程师ID", "后端工程师ID"]
}
```

系统会检查 `CompletedAgents` 集合，只有当所有 `wait_for` 中的 Agent 都已完成时，才会触发目标 Agent。

## 配置示例

### 1. 顺序工作流

```json
{
  "name": "标准开发流程",
  "agent_ids": [
    "需求分析师ID",
    "架构师ID",
    "开发工程师ID",
    "测试工程师ID"
  ],
  "transitions": [
    {
      "from_agent_id": "需求分析师ID",
      "to_agent_id": "架构师ID",
      "type": "sequence",
      "trigger": "需求完成"
    },
    {
      "from_agent_id": "架构师ID",
      "to_agent_id": "开发工程师ID",
      "type": "sequence",
      "trigger": "设计完成"
    },
    {
      "from_agent_id": "开发工程师ID",
      "to_agent_id": "测试工程师ID",
      "type": "sequence",
      "trigger": "开发完成"
    }
  ]
}
```

### 2. 并行工作流（分支）

架构师完成后，同时触发前端和后端工程师：

```json
{
  "name": "前后端并行开发",
  "agent_ids": [
    "架构师ID",
    "前端工程师ID",
    "后端工程师ID",
    "集成测试ID"
  ],
  "transitions": [
    {
      "from_agent_id": "架构师ID",
      "to_agent_id": "前端工程师ID",
      "type": "parallel",
      "trigger": "设计完成"
    },
    {
      "from_agent_id": "架构师ID",
      "to_agent_id": "后端工程师ID",
      "type": "parallel",
      "trigger": "设计完成"
    }
  ]
}
```

### 3. 汇聚工作流

前端和后端都完成后，触发集成测试：

```json
{
  "name": "集成测试流程",
  "agent_ids": [
    "前端工程师ID",
    "后端工程师ID",
    "集成测试ID"
  ],
  "transitions": [
    {
      "from_agent_id": "前端工程师ID",
      "to_agent_id": "集成测试ID",
      "type": "merge",
      "wait_for": ["前端工程师ID", "后端工程师ID"]
    },
    {
      "from_agent_id": "后端工程师ID",
      "to_agent_id": "集成测试ID",
      "type": "merge",
      "wait_for": ["前端工程师ID", "后端工程师ID"]
    }
  ]
}
```

### 4. 条件路由

根据输出内容决定路由：

```json
{
  "name": "条件测试流程",
  "agent_ids": [
    "开发工程师ID",
    "单元测试ID",
    "代码审查ID"
  ],
  "transitions": [
    {
      "from_agent_id": "开发工程师ID",
      "to_agent_id": "单元测试ID",
      "type": "sequence",
      "condition": "contains:单元测试通过"
    },
    {
      "from_agent_id": "开发工程师ID",
      "to_agent_id": "代码审查ID",
      "type": "sequence",
      "condition": "contains:代码审查通过"
    }
  ]
}
```

## 关键实现

### ExecutionService 核心方法

| 方法 | 功能 |
|------|------|
| `checkRouting()` | 路由入口，检查是否需要触发下一个 Agent |
| `parseMentions()` | 解析输出中的 @mention |
| `checkSignalRouting()` | 基于 Transitions 的自动路由 |
| `getTransitionsForAgent()` | 获取当前 Agent 的转换规则 |
| `getAllowedAgentsFromWorkflow()` | 获取工作流中允许的 Agent 列表 |
| `matchCondition()` | 条件表达式匹配 |
| `checkMergeCondition()` | 汇聚条件检查 |

### 防护机制

**防止无限循环**：
- 最大深度限制：`MaxA2ADepth = 15`
- Agent 去重：同一 Agent 不会重复调用

```go
// 深度检查
if a2aCtx.Depth >= MaxA2ADepth {
    logInfo("A2A 深度达到上限，停止自动路由")
    return
}

// 去重检查
if a2aCtx.InvokedAgents[targetConfig.ID] {
    logInfo("A2A 去重：Agent 已被调用过")
    continue
}
```

## SystemPrompt 设计

为了引导 Agent 正确协作，SystemPrompt 需要包含：

### 需求分析师示例

```
你是一个需求分析师，负责对用户的原始输入进行理解分析，并提供需求设计。

## 你的职责范围
- 理解和分析用户需求
- 编写需求文档和设计文档
- 明确功能点和技术方案

## 重要限制（必须遵守）
- 你**只负责需求分析**，不负责代码实现
- 当你完成需求分析后，**必须停止**，不要写任何代码
- 在输出末尾添加一行："@开发工程师 请根据以上需求进行开发实现"
```

### 开发工程师示例

```
你是一个开发工程师，负责将需求分析师提供的需求文档进行实现。

## 你的职责范围
- 根据需求文档编写代码
- 创建和修改项目文件
- 执行必要的命令和脚本

## 工作流程
1. 首先阅读需求分析师提供的需求文档
2. 分析需要实现的功能
3. 编写代码实现
4. 测试验证功能
```

## 数据库结构

### workflow_templates 表

```sql
CREATE TABLE workflow_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    agent_ids TEXT,          -- JSON 数组
    transitions TEXT,        -- JSON 数组
    checkpoints TEXT,        -- JSON 数组
    estimated_time TEXT,
    is_system INTEGER,
    is_default INTEGER,
    created_at DATETIME,
    updated_at DATETIME
);
```

### threads 表

```sql
ALTER TABLE threads ADD COLUMN workflow_template_id TEXT;
```

## API 端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/workflows` | GET | 获取所有工作流模板 |
| `/api/v1/workflows` | POST | 创建工作流模板 |
| `/api/v1/workflows/:id` | GET | 获取指定工作流模板 |
| `/api/v1/workflows/:id` | PUT | 更新工作流模板 |
| `/api/v1/workflows/:id/default` | PUT | 设置为默认工作流 |

## 调试与日志

关键日志点：

```
INFO  getTransitionsForAgent: starting
INFO  getTransitionsForAgent: found thread with WorkflowTemplateID
INFO  getTransitionsForAgent: raw Transitions JSON
INFO  getTransitionsForAgent: parsed transitions
INFO  A2A 自动路由触发（基于Transitions）
INFO  A2A 条件路由：条件不匹配，跳过
INFO  A2A 汇聚：等待上游 Agent 完成
INFO  A2A 去重：Agent 已被调用过
```

## 最佳实践

1. **明确职责边界**：每个 Agent 的 SystemPrompt 应清晰定义职责范围
2. **合理设置 Transitions**：避免复杂的循环依赖
3. **监控深度**：注意 A2A 深度，防止无限循环
4. **日志追踪**：通过日志监控 Agent 协作流程
5. **测试验证**：新工作流上线前进行端到端测试
6. **并行设计**：使用 `parallel` 类型实现并行执行，提高效率
7. **汇聚控制**：使用 `merge` 类型确保依赖关系正确