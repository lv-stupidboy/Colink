# A2A 路由改进变更记录

**变更日期**: 2026-03-19
**变更原因**: 参考 Cat Café 的 A2A 实践，修复深度限制缺失、重复触发、链条断裂等问题

## 变更概要

| 变更项 | 文件 | 变更类型 |
|--------|------|----------|
| 添加 Transitions 字段 | `model/workflow_template.go` | 新增字段 |
| A2A 上下文追踪 | `service/agent/execution_service.go` | 新增逻辑 |
| 动态 SystemPrompt 注入 | `service/agent/execution_service.go` | 修改方法 |
| 统一路由路径 | `service/agent/orchestrator.go` | 删除重复代码 |

---

## 变更 1: WorkflowTemplate 添加 Transitions 字段

**文件**: `isdp/internal/model/workflow_template.go`

### 新增类型定义 (第 10 行后插入)

```go
// Transition Agent间协作转换规则
type Transition struct {
	FromAgentID     string `json:"from_agent_id"`     // 源 Agent ID
	Trigger         string `json:"trigger"`           // 触发条件描述
	ToAgentID       string `json:"to_agent_id"`       // 目标 Agent ID
	MessageTemplate string `json:"message_template"`  // 消息模板 (可选)
	Description     string `json:"description"`       // 转换描述
}
```

### WorkflowTemplate 结构体添加字段 (第 21 行后插入)

```go
	Transitions    json.RawMessage `json:"transitions"`    // Agent间转换规则 (JSON数组)
```

### 回滚方法

删除新增的 `Transition` 类型定义和 `Transitions` 字段。

---

## 变更 2: A2A 上下文追踪 (深度限制 + 去重)

**文件**: `isdp/internal/service/agent/execution_service.go`

### 新增常量和类型 (第 18 行后插入)

```go
// MaxA2ADepth A2A 最大深度限制
const MaxA2ADepth = 15

// A2AContext A2A 上下文，用于追踪深度和去重
type A2AContext struct {
	Depth         int               // 当前深度
	InvokedAgents map[uuid.UUID]bool // 已调用的 Agent ID 集合
}
```

### ExecutionService 结构体添加字段 (第 35 行后插入)

```go
	// A2A 上下文追踪
	a2aContexts    map[uuid.UUID]*A2AContext // threadID -> A2AContext
	a2aMu          sync.RWMutex
```

### NewExecutionService 初始化 (第 63 行后插入)

```go
		a2aContexts:    make(map[uuid.UUID]*A2AContext),
```

### checkRouting 方法修改 (替换原方法)

新增深度检查和去重逻辑。

### 回滚方法

删除新增的常量、类型、字段，恢复原 `checkRouting` 方法。

---

## 变更 3: 动态 SystemPrompt 注入

**文件**: `isdp/internal/service/agent/execution_service.go`

### buildContextLayers 方法修改

Layer 0 从静态的 `config.SystemPrompt` 改为动态注入：
- 工作流触发点提示
- 出口检查提示

### 新增辅助方法

- `getTransitionsForAgent()` - 获取当前 Agent 的转换规则
- `buildWorkflowTriggerPrompt()` - 构建工作流触发点提示
- `buildExitCheckPrompt()` - 构建出口检查提示

### 回滚方法

恢复原 `buildContextLayers` 方法，删除新增辅助方法。

---

## 变更 4: 统一路由路径

**文件**: `isdp/internal/service/agent/orchestrator.go`

### 删除重复方法

删除以下方法 (委托给 ExecutionService):
- `checkRouting()` (第 172-239 行)
- `parseMentions()` (第 243-275 行)
- `getAllowedAgentsFromWorkflow()` (第 298-335 行)
- `findAgentByRole()` (第 338-345 行)
- `findAgentByName()` (第 348-355 行)
- `checkSignalRouting()` (第 358-382 行)

### 回滚方法

恢复被删除的方法。

---

## 验证清单

- [x] 编译通过
- [ ] 现有功能不受影响
- [ ] A2A 深度超过 15 时停止
- [ ] 同一 Agent 不会被重复调用
- [ ] Agent SystemPrompt 包含工作流触发点
- [ ] Agent SystemPrompt 包含出口检查提示

---

## 完整回滚步骤

```bash
# 1. 恢复 model/workflow_template.go
git checkout HEAD -- isdp/internal/model/workflow_template.go

# 2. 恢复 service/agent/execution_service.go
git checkout HEAD -- isdp/internal/service/agent/execution_service.go

# 3. 恢复 service/agent/orchestrator.go
git checkout HEAD -- isdp/internal/service/agent/orchestrator.go
```