---
title: Agent 调用排队与消息合并
date: 2026-05-16
status: draft
updated: 2026-05-16
clarification_rounds: 1
final_ambiguity: 22.4%
---

# Agent 调用排队与消息合并设计

## 背景

当前 isdp 项目在 A2A 触发时，如果同一个 Agent 正在执行，再次 @mention 该 Agent 会直接跳过（`a2a_trigger.go` line 162-163）。用户期望这些重复调用能够入队等待，并在上一个完成后合并执行。

## 澄清决策（Deep Interview Round 1）

| 问题 | 决策 | 理由 |
|------|------|------|
| Slot 状态追踪 | 增强 InvocationRegistry，不新增 AgentSlotTracker | InvocationRegistry 已有 HasActiveSlot、catLatest map |
| 合并时机 | 执行前合并（slot idle 后合并所有 queued） | 用户明确选择 |
| 合并实现位置 | 增强 QueueProcessor.OnInvocationComplete | 不新增独立组件 |
| 回调触发方式 | ExecutionService 回调 | 保持现有方式 |

## 目标

1. **入队策略**：所有重复调用都入队，不跳过
2. **执行顺序**：执行前合并消息，减少重复调用开销
3. **用户体验**：显示排队数量、合并预览、支持取消特定消息
4. **作用域**：Thread 级别隔离（每个 Thread 内同一 Agent 只能有一个活跃调用）

## 架构设计

### 核心组件（复用现有）

| 组件 | 文件位置 | 职责 | 变化 |
|------|----------|------|------|
| `InvocationRegistry` | `internal/service/a2a/invocation_registry.go` | Slot 状态追踪 | **增强**：新增 SetSlotBusy/SetSlotIdle |
| `InvocationQueue` | `internal/service/a2a/invocation_queue.go` | 存储排队请求 | **增强**：保留现有结构 |
| `QueueProcessor` | `internal/service/a2a/queue_processor.go` | slot 完成时触发执行 | **增强**：新增合并逻辑 |

### 数据结构

#### InvocationRegistry 增强

```go
// InvocationRegistry 已有字段（复用）
// - catLatest map[string]uuid.UUID  // threadID:catID -> latest invocationID
// - HasActiveSlot(threadID, catID)  // 已有方法

// 新增方法
func (r *InvocationRegistry) SetSlotBusy(threadID uuid.UUID, catID string, invocationID uuid.UUID)
func (r *InvocationRegistry) SetSlotIdle(threadID uuid.UUID, catID string)
func (r *InvocationRegistry) GetSlotStatus(threadID uuid.UUID, catID string) SlotStatus

type SlotStatus struct {
    Status            string    // "idle" | "busy"
    ActiveInvocationID uuid.UUID
    StartedAt         time.Time
    QueueLength       int       // 关联队列长度
}
```

#### QueueEntry 增强

```go
type QueueEntry struct {
    // ...现有字段保持不变（已有 MergedMsgIDs）
    // 执行前合并时填充 MergedInputs
}

// InvocationQueue 已有字段足够支持合并
// - canMerge() 函数已存在，需调整调用时机
// - HasQueuedAgent() 已存在
```

### 关键方法

#### InvocationRegistry（增强）

```go
// SetSlotBusy 标记 slot 为忙碌（SpawnAgent 前调用）
func (r *InvocationRegistry) SetSlotBusy(threadID uuid.UUID, catID string, invocationID uuid.UUID)

// SetSlotIdle 标记 slot 为空闲（OnInvocationComplete 中调用）
func (r *InvocationRegistry) SetSlotIdle(threadID uuid.UUID, catID string)

// GetSlotStatus 获取 slot 详情（用于 WebSocket 广播）
func (r *InvocationRegistry) GetSlotStatus(threadID uuid.UUID, catID string) SlotStatus
```

#### QueueProcessor（增强）

```go
// MergeQueuedForAgent 合并同 Agent 的所有排队消息（OnInvocationComplete 中调用）
func (p *QueueProcessor) MergeQueuedForAgent(ctx context.Context, threadID uuid.UUID, catID string) string

// BuildMergePreview 构建合并预览（用于前端显示）
func (p *QueueProcessor) BuildMergePreview(threadID uuid.UUID, catID string) *MergePreview

type MergePreview struct {
    CatID           string
    AgentName       string
    MergedContent   string
    OriginalInputs  []MergedInput
}
```

## 数据流

### 入队流程

```
用户消息 → EnqueueA2ATargets()
    ↓
检查 InvocationRegistry.HasActiveSlot(threadID, catID)
    ↓
┌─ busy ──────────────────────┐
│  入队到 InvocationQueue     │
│  WebSocket 广播 slot_status │
└─────────────────────────────┘
┌─ idle ─────────────────────┐
│  InvocationRegistry.SetSlotBusy() │
│  直接触发 SpawnAgent        │
└─────────────────────────────┘
```

### 完成流程

```
ExecutionService.executeAgent 完成
    ↓
回调 QueueProcessor.OnInvocationComplete()
    ↓
InvocationRegistry.SetSlotIdle(threadID, catID)
    ↓
检查 InvocationQueue.HasQueuedAgent(threadID, catID)
    ↓
┌─ 有排队 ──────────────────────┐
│  QueueProcessor.MergeQueuedForAgent() │
│  WebSocket 广播 merge_preview │
│  InvocationRegistry.SetSlotBusy() │
│  触发 SpawnAgent(合并input)  │
└──────────────────────────────┘
┌─ 无排队 ────────────────────┐
│  结束，slot 保持 idle        │
└─────────────────────────────┘
```

### 取消流程

```
前端取消请求 → API: DELETE /api/threads/:id/queue/:entryId
    ↓
InvocationQueue.Remove(threadID, userID, entryID)
    ↓
WebSocket 广播 queue_updated
```

## API 设计

### 新增 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/threads/:id/queue` | 获取线程的排队状态 |
| DELETE | `/api/threads/:id/queue/:entryId` | 取消特定排队请求 |

### API Handler 实现

在 `internal/api/` 新增 `queue_handler.go`：

```go
// GetThreadQueue 获取线程排队状态
func (h *QueueHandler) GetThreadQueue(c *gin.Context)

// CancelQueueEntry 取消排队请求
func (h *QueueHandler) CancelQueueEntry(c *gin.Context)
```

路由注册在 `cmd/server/main.go`。

## WebSocket 消息

### 新增消息类型

| Type | 说明 | Payload |
|------|------|---------|
| `slot_status` | slot 状态变化 | `{catId, agentName, status, queueLength, activeInvocationId}` |
| `merge_preview` | 合并消息预览 | `{catId, mergedContent, originalInputs}` |

### slot_status Payload

```json
{
  "type": "slot_status",
  "threadId": "...",
  "timestamp": 1234567890,
  "payload": {
    "catId": "...",
    "agentName": "Backend Developer",
    "status": "busy",
    "queueLength": 2,
    "activeInvocationId": "..."
  }
}
```

### merge_preview Payload

```json
{
  "type": "merge_preview",
  "threadId": "...",
  "timestamp": 1234567890,
  "payload": {
    "catId": "...",
    "mergedContent": "请依次完成以下任务:\n1. 修复登录bug\n2. 添加日志功能",
    "originalInputs": [
      {"originalMessageId": "...", "originalContent": "修复登录bug", "enqueuedAt": "..."},
      {"originalMessageId": "...", "originalContent": "添加日志功能", "enqueuedAt": "..."}
    ]
  }
}
```

## 前端交互

### 排队状态显示

```
┌─────────────────────────────────────────┐
│ 🔄 Backend Developer 正在执行...        │
│ 排队中: 2 条消息                         │
│ ─────────────────────────────────────    │
│ #1: "修复登录bug"    [取消]              │
│ #2: "添加日志功能"   [取消]              │
└─────────────────────────────────────────┘
```

### 合并执行前预览

```
┌─────────────────────────────────────────┐
│ ⚡ 合并执行 Backend Developer           │
│ ─────────────────────────────────────    │
│ 将执行以下合并请求:                       │
│ • "修复登录bug"                          │
│ • "添加日志功能"                         │
│ ─────────────────────────────────────    │
│ 合并内容:                                 │
│ 请依次完成以下任务:                       │
│ 1. 修复登录bug                           │
│ 2. 添加日志功能                           │
└─────────────────────────────────────────┘
```

## 错误处理

| 场景 | 处理方式 |
|------|----------|
| Agent 执行失败 | InvocationRegistry.SetSlotIdle，队列中的请求可选择继续或暂停（复用现有 `pausedSlots` 机制） |
| 用户取消所有排队请求 | 清空队列，slot 保持 idle |
| 合并消息过长（超 token） | 截断或提示用户，最大合并长度由配置控制 |
| 跨用户排队（多人协作） | 按时间顺序 FIFO，不区分用户优先级 |
| Agent 被禁用 | 清空该 agent 的所有排队请求，WebSocket 通知 |

## 配置项

在 `configs/config.yaml` 新增：

```yaml
a2a:
  queue:
    enabled: true           # 默认开启
    maxMergeLength: 4000    # 合并消息最大字符数，默认 4000
    mergeDelay: 0           # 合并延迟秒数（执行前等待后续消息），默认 0（立即合并）
```

## 合并消息模板

合并后的消息格式：

```
请依次完成以下任务：

1. {第一条消息内容}
2. {第二条消息内容}
...

完成后请汇报结果。
```

如果只有一条消息，不添加序号格式，直接使用原内容。

## 与现有代码的集成点

| 现有组件 | 集成方式 |
|----------|----------|
| `InvocationRegistry.HasActiveSlot()` | 入队前检查，复用现有方法 |
| `InvocationRegistry.Register()` | SpawnAgent 时调用，已有 |
| `InvocationRegistry.Complete()` | 执行完成时调用，已有 |
| `InvocationQueue.Enqueue()` | 入队逻辑，已有 |
| `InvocationQueue.HasQueuedAgent()` | 检查队列，已有 |
| `QueueProcessor.OnInvocationComplete()` | 完成回调，增强 |
| `QueueProcessor.pausedSlots` | 失败场景处理，复用 |

## 测试要点

1. **入队测试**：验证 busy 时入队逻辑
2. **合并测试**：验证多条消息合并格式
3. **取消测试**：验证取消特定消息后队列状态
4. **并发测试**：验证多用户同时 @mention 同一 agent
5. **失败测试**：验证执行失败后队列处理

## 实现计划

实现将拆分为以下阶段：

1. **阶段一**：InvocationRegistry 增强（SetSlotBusy/SetSlotIdle/GetSlotStatus）
2. **阶段二**：QueueProcessor 合并逻辑增强
3. **阶段三**：WebSocket 消息集成（slot_status/merge_preview）
4. **阶段四**：API Handler 实现
5. **阶段五**：前端组件实现

详细实现计划将由 `writing-plans` skill 生成。