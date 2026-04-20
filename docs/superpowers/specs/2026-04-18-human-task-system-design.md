# Human Task System 设计文档

> **状态**：已实现
> **日期**：2026-04-18

---

## 背景

ISDP 需要支持 Agent 与用户的协作交互。当 Agent 执行过程中需要用户输入（如 AskUserQuestion）时，系统自动创建待办任务，用户可通过待办任务页面快速进入对话进行处理。

### 设计目标

- **Agent 主导**：Agent 团队保持纯粹，不将 Human 作为角色类型混入
- **原子幂等**：对话处理任务 → 待办任务自动关闭，精确匹配避免误关闭
- **精确关联**：某个任务的某个 Agent 因什么事情而等待，精确追踪

---

## 核心设计

### 1. 触发机制

Agent 等待用户输入时自动创建待办任务，主要触发场景：

**AskUserQuestion 工具调用**

当 Agent 调用 AskUserQuestion 工具提出问题时，后端解析输出并创建待办任务：

```
1. Agent 输出包含 tool_use: AskUserQuestion
2. 后端解析输出，识别 AskUserQuestion
3. 创建待办任务，关联 invocation_id
4. 记录 Agent 当前输出内容作为 wait_reason
5. 广播 human_task_created 事件
```

### 2. 待办任务数据模型

**表：`human_tasks`**

```sql
CREATE TABLE human_tasks (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,           -- 关联的 Thread
    invocation_id TEXT NOT NULL,       -- 关联的 Invocation（精确匹配）
    agent_config_id TEXT NOT NULL,     -- 等待用户的 Agent 配置 ID
    agent_name TEXT,                   -- Agent 名称（便于显示）
    wait_reason TEXT,                  -- 等待原因（Agent 当前输出内容摘要）
    project_id TEXT,                   -- 项目 ID
    project_name TEXT,                 -- 项目名称
    thread_name TEXT,                  -- 任务名称
    status TEXT DEFAULT 'pending',     -- pending/completed/cancelled
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    completed_at TEXT
);
```

**字段说明**：

| 字段 | 说明 |
|------|------|
| `invocation_id` | Agent 调用实例 ID，唯一标识一次 Agent 执行 |
| `agent_config_id` | 等待用户的 Agent 配置 ID |
| `wait_reason` | Agent 当前输出内容摘要，用于前端展示 |
| `project_id`, `project_name` | 项目信息，用于展示 |
| `thread_name` | 任务名称，用于展示 |

### 3. 原子幂等关闭机制

**核心原则**：用户在对话中回复 → 匹配的待办任务自动关闭

**唯一标识**：
```
(invocation_id, status='pending') 唯一匹配
```

**关闭流程**：

```
1. 用户在对话中回复 Agent（提交 AskUserQuestion 答案）
2. 后端接收用户消息，调用 completeByInvocation API
3. 查询该 invocation_id 下 pending 状态的待办任务
4. 更新待办任务状态为 completed，记录 completed_at
5. 广播 human_task_completed 事件到前端
```

**幂等保证**：
- 同一 invocation 只能有一个 pending 待办任务
- 关闭时通过 invocation_id + status='pending' 精确查找
- 已关闭的任务不会被再次关闭

### 4. 待办任务页面

**路由**：`/tasks`

**页面功能**：

- 状态筛选：待处理 / 已完成 / 已取消
- 任务卡片显示：Agent 名称、项目、任务、等待原因、创建时间
- 手动操作：标记完成、取消任务
- 点击进入：跳转到对应 Thread 对话页面

**卡片布局**：

```
┌─────────────────────────────────────────────────────────┐
│ 🤖 架构师                                    [待处理]   │
│ [项目名] [任务名]                                       │
│ 等待原因: "请确认是否采用 OAuth 2.0 方案..."            │
│ 10 分钟前                          [✓] [✗] [→]        │
└─────────────────────────────────────────────────────────┘
```

---

## API 设计

### REST API

| API | 方法 | 说明 |
|-----|------|------|
| `/api/human-tasks` | GET | 获取待办任务列表（支持 status 筛选） |
| `/api/human-tasks/:id/complete` | PUT | 手动完成待办任务 |
| `/api/human-tasks/:id/cancel` | PUT | 取消待办任务 |
| `/api/human-tasks/invocation/:invocationId/complete` | PUT | 按 invocation 关闭任务 |

### WebSocket 事件

| 事件 | 说明 |
|------|------|
| `human_task_created` | 新待办任务创建（全局广播） |
| `human_task_completed` | 待办任务完成 |
| `human_task_cancelled` | 待办任务取消 |

---

## 实现细节

### 后端触发点

**callback_handler.go**：解析 AskUserQuestion 工具调用

```go
// 当检测到 AskUserQuestion 工具调用时
if toolName == "AskUserQuestion" {
    // 创建待办任务
    humanTaskSvc.CreateTaskFromWaiting(ctx, threadID, invocationID, agentConfigID, agentName, waitReason)
}
```

### 前端关闭点

**ThreadView.tsx**：用户提交 AskUserQuestion 答案后

```typescript
// 用户提交答案后
await api.humanTasks.completeByInvocation(invocationId);
```

### 时间显示

**SQLite 时间解析**：使用本地时区解析不带时区的时间格式

```go
// parseSQLiteTime 使用 time.ParseInLocation 解析本地时区时间
t, err := time.ParseInLocation(layout, s, time.Local)
```

---

## 数据流图

```
┌──────────────────────────────────────────────────────────────┐
│ Agent 输出 AskUserQuestion                                    │
└──────────────────────────────────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ 后端解析输出，识别 AskUserQuestion                            │
│ CreateTaskFromWaiting                                         │
└──────────────────────────────────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ 创建 HumanTask 记录                                           │
│ 广播 human_task_created 事件                                  │
└──────────────────────────────────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ 前端展示                                                      │
│   /tasks 页面：任务列表                                       │
│   WebSocket 实时更新                                          │
└──────────────────────────────────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ 用户进入对话，回复 Agent                                      │
└──────────────────────────────────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ completeByInvocation                                          │
│ 更新任务状态 → completed                                      │
│ 广播 human_task_completed 事件                                │
└──────────────────────────────────────────────────────────────┘
```

---

## 已实现功能清单

| 功能 | 状态 |
|------|------|
| AskUserQuestion 解析创建待办任务 | ✅ 已实现 |
| 待办任务页面 `/tasks` | ✅ 已实现 |
| 状态筛选（待处理/已完成/已取消） | ✅ 已实现 |
| 手动标记完成/取消 | ✅ 已实现 |
| 点击进入对话 | ✅ 已实现 |
| 用户回复自动关闭任务 | ✅ 已实现 |
| WebSocket 实时更新 | ✅ 已实现 |
| 项目/任务信息显示 | ✅ 已实现 |
| 时间正确显示（本地时区） | ✅ 已实现 |

---

## 变更文件清单

### 后端

| 文件 | 变更内容 |
|------|---------|
| `internal/model/human_task.go` | HumanTask 模型（含 projectId, projectName, threadName） |
| `internal/repo/human_task.go` | 数据访问层（FindByInvocation, CompleteByInvocation 等） |
| `internal/repo/db_sqlite.go` | parseSQLiteTime 本地时区解析 |
| `internal/service/humantask/service.go` | CreateTaskFromWaiting, CompleteTaskFromReply |
| `internal/api/callback_handler.go` | AskUserQuestion 解析触发 |
| `internal/api/human_task_handler.go` | CompleteByInvocation API |

### 前端

| 文件 | 变更内容 |
|------|---------|
| `web/src/pages/Tasks.tsx` | 待办任务页面 |
| `web/src/api/client.ts` | humanTasks API（list, complete, cancel, completeByInvocation） |
| `web/src/pages/ThreadView.tsx` | 用户回复后关闭任务 |
| `web/src/types/humanTask.ts` | HumanTask 类型定义 |

### 数据库迁移

| 文件 | 内容 |
|------|------|
| `sql-change/v1.3.0/sqlite/00006_human_task_fields.sql` | 新增字段 |
| `sql-change/v1.3.0/sqlite/00007_human_task_fix_null.sql` | 修复 NULL 约束 |
| `sql-change/v1.3.0/sqlite/00008_human_task_project_info.sql` | 项目信息字段 |