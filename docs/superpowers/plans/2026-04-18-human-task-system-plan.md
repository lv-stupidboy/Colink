# Human Task System 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 Human Task 待办任务系统，Agent 等待用户输入时自动创建待办任务，用户回复后自动关闭。

**Architecture:** 
- 触发：解析 AskUserQuestion 工具调用，创建待办任务
- 关闭：用户回复时通过 invocation_id 精确匹配关闭
- 前端：待办任务页面 `/tasks`，WebSocket 实时更新

**Tech Stack:** Go (后端), React + Ant Design (前端), SQLite (数据库)

---

## 文件结构

### 后端文件
| 文件 | 负责内容 | 操作 |
|------|----------|------|
| `internal/model/human_task.go` | HumanTask 数据模型 | 修改 |
| `internal/repo/human_task.go` | 数据访问层 | 修改 |
| `internal/repo/db_sqlite.go` | 时间解析（本地时区） | 修改 |
| `internal/service/humantask/service.go` | 业务逻辑层 | 修改 |
| `internal/api/callback_handler.go` | AskUserQuestion 解析 | 修改 |
| `internal/api/human_task_handler.go` | API 处理器 | 修改 |
| `sql-change/v1.3.0/sqlite/*.sql` | 数据库迁移 | 创建 |

### 前端文件
| 文件 | 负责内容 | 操作 |
|------|----------|------|
| `web/src/pages/Tasks.tsx` | 待办任务页面 | 创建 |
| `web/src/api/client.ts` | API 客户端 | 修改 |
| `web/src/pages/ThreadView.tsx` | 对话页面关闭任务 | 修改 |
| `web/src/types/humanTask.ts` | 类型定义 | 创建 |

---

## Task 1: HumanTask 模型扩展

**Files:**
- Modify: `internal/model/human_task.go`

- [x] **Step 1: 添加项目信息字段**

```go
type HumanTask struct {
    ID             uuid.UUID       `json:"id"`
    ThreadID       uuid.UUID       `json:"threadId"`
    InvocationID   uuid.UUID       `json:"invocationId"`
    AgentConfigID  uuid.UUID       `json:"agentConfigId"`
    AgentName      string          `json:"agentName"`
    WaitReason     string          `json:"waitReason"`
    ProjectID      uuid.UUID       `json:"projectId"`      // 新增
    ProjectName    string          `json:"projectName"`    // 新增
    ThreadName     string          `json:"threadName"`     // 新增
    Status         HumanTaskStatus `json:"status"`
    CreatedAt      time.Time       `json:"createdAt"`
    CompletedAt    *time.Time      `json:"completedAt"`
}
```

- [x] **Step 2: Commit**

```bash
git add internal/model/human_task.go
git commit -m "feat: HumanTask 添加项目信息字段"
```

---

## Task 2: 数据库迁移脚本

**Files:**
- Create: `sql-change/v1.3.0/sqlite/00006_human_task_fields.sql`
- Create: `sql-change/v1.3.0/sqlite/00008_human_task_project_info.sql`

- [x] **Step 1: 创建迁移脚本**

```sql
-- 00006: 新增 invocation_id, wait_reason 字段
-- 00008: 新增 project_id, project_name, thread_name 字段
```

- [x] **Step 2: Commit**

```bash
git add sql-change/v1.3.0/sqlite/*.sql
git commit -m "feat: human_tasks 表字段迁移"
```

---

## Task 3: Repository 方法更新

**Files:**
- Modify: `internal/repo/human_task.go`

- [x] **Step 1: 添加 FindByInvocation 方法**

```go
// FindByInvocation 根据 invocation_id 查找 pending 状态的任务
func (r *HumanTaskRepository) FindByInvocation(ctx context.Context, invocationID uuid.UUID) (*model.HumanTask, error) {
    // 查询 invocation_id + status='pending'
}
```

- [x] **Step 2: 添加 CompleteByInvocation 方法**

```go
// CompleteByInvocation 根据 invocation_id 完成 pending 任务
func (r *HumanTaskRepository) CompleteByInvocation(ctx context.Context, invocationID uuid.UUID) error {
    // UPDATE status='completed' WHERE invocation_id=? AND status='pending'
}
```

- [x] **Step 3: Commit**

```bash
git add internal/repo/human_task.go
git commit -m "feat: HumanTaskRepository 新增 invocation 相关方法"
```

---

## Task 4: SQLite 时间解析修复

**Files:**
- Modify: `internal/repo/db_sqlite.go`

- [x] **Step 1: 使用本地时区解析时间**

```go
// parseSQLiteTime 使用 time.ParseInLocation 解析本地时区时间
for _, item := range layoutsUTC {
    if item.useUTC {
        t, err = time.ParseInLocation(item.layout, s, localLocation)
    } else {
        t, err = time.Parse(item.layout, s)
    }
}
```

- [x] **Step 2: Commit**

```bash
git add internal/repo/db_sqlite.go
git commit -m "fix: SQLite 时间解析使用本地时区"
```

---

## Task 5: Service 业务逻辑

**Files:**
- Modify: `internal/service/humantask/service.go`

- [x] **Step 1: CreateTaskFromWaiting 方法**

```go
// CreateTaskFromWaiting 从 Agent waiting 状态创建待办任务
func (s *Service) CreateTaskFromWaiting(
    ctx context.Context,
    threadID uuid.UUID,
    invocationID uuid.UUID,
    agentConfigID uuid.UUID,
    agentName string,
    waitReason string,
) (*model.HumanTask, error) {
    // 幂等检查：是否已有 pending 任务
    // 创建任务，获取项目信息
    // 广播 human_task_created 事件
}
```

- [x] **Step 2: CompleteTaskFromReply 方法**

```go
// CompleteTaskFromReply 用户回复后关闭待办任务
func (s *Service) CompleteTaskFromReply(ctx context.Context, invocationID uuid.UUID) error {
    // CompleteByInvocation
    // 广播 human_task_completed 事件
}
```

- [x] **Step 3: Commit**

```bash
git add internal/service/humantask/service.go
git commit -m "feat: HumanTaskService 新增 CreateTaskFromWaiting, CompleteTaskFromReply"
```

---

## Task 6: AskUserQuestion 解析触发

**Files:**
- Modify: `internal/api/callback_handler.go`

- [x] **Step 1: 解析 AskUserQuestion 工具调用**

```go
// 在 output chunk 解析中检测 AskUserQuestion
if toolName == "AskUserQuestion" {
    // 提取问题内容作为 wait_reason
    // 调用 humanTaskSvc.CreateTaskFromWaiting
}
```

- [x] **Step 2: Commit**

```bash
git add internal/api/callback_handler.go
git commit -m "feat: AskUserQuestion 解析触发待办任务创建"
```

---

## Task 7: API Handler 扩展

**Files:**
- Modify: `internal/api/human_task_handler.go`

- [x] **Step 1: 添加 CompleteByInvocation Handler**

```go
// CompleteByInvocation 按 invocation 关闭任务
// PUT /api/human-tasks/invocation/:invocationId/complete
func (h *HumanTaskHandler) CompleteByInvocation(c *gin.Context) {
    invocationID := c.Param("invocationId")
    h.svc.CompleteTaskFromReply(ctx, invocationID)
}
```

- [x] **Step 2: Commit**

```bash
git add internal/api/human_task_handler.go
git commit -m "feat: CompleteByInvocation API"
```

---

## Task 8: 前端类型定义

**Files:**
- Modify: `web/src/types/humanTask.ts`

- [x] **Step 1: 扩展 HumanTask 类型**

```typescript
interface HumanTask {
  id: string;
  threadId: string;
  invocationId: string;
  agentConfigId: string;
  agentName: string;
  waitReason: string;
  projectId: string;      // 新增
  projectName: string;    // 新增
  threadName: string;     // 新增
  status: HumanTaskStatus;
  createdAt: string;
  completedAt?: string;
}
```

- [x] **Step 2: Commit**

```bash
git add web/src/types/humanTask.ts
git commit -m "feat: HumanTask 类型添加项目信息"
```

---

## Task 9: API 客户端扩展

**Files:**
- Modify: `web/src/api/client.ts`

- [x] **Step 1: 添加 completeByInvocation 方法**

```typescript
humanTasks = {
  list: (status?: HumanTaskStatus) => request(`/human-tasks?status=${status}`),
  complete: (id: string) => request(`/human-tasks/${id}/complete`, 'PUT'),
  cancel: (id: string) => request(`/human-tasks/${id}/cancel`, 'PUT'),
  completeByInvocation: (invocationId: string) => 
    request(`/human-tasks/invocation/${invocationId}/complete`, 'PUT'),
};
```

- [x] **Step 2: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat: humanTasks API 添加 completeByInvocation"
```

---

## Task 10: 待办任务页面

**Files:**
- Create: `web/src/pages/Tasks.tsx`

- [x] **Step 1: 创建 Tasks 页面组件**

```tsx
// 状态 Tabs：待处理 / 已完成 / 已取消
// 任务列表：显示 Agent 名称、项目、任务、等待原因、时间
// 手动操作：标记完成、取消
// 点击进入：跳转到 /threads/:threadId
// WebSocket 监听：human_task_created, human_task_completed, human_task_cancelled
```

- [x] **Step 2: 添加路由**

```tsx
<Route path="/tasks" element={<Tasks />} />
```

- [x] **Step 3: Commit**

```bash
git add web/src/pages/Tasks.tsx web/src/App.tsx
git commit -m "feat: 待办任务页面 /tasks"
```

---

## Task 11: 用户回复关闭任务

**Files:**
- Modify: `web/src/pages/ThreadView.tsx`

- [x] **Step 1: 提交答案后关闭任务**

```typescript
// handleInlineQuestionSubmit 中
// 提交答案成功后
await api.humanTasks.completeByInvocation(invocationId);
```

- [x] **Step 2: Commit**

```bash
git add web/src/pages/ThreadView.tsx
git commit -m "feat: 用户回复 AskUserQuestion 后自动关闭待办任务"
```

---

## Task 12: WebSocket 全局广播

**Files:**
- Modify: `internal/ws/hub.go`

- [x] **Step 1: BroadcastGlobal 方法**

```go
// BroadcastGlobal 向所有客户端广播消息
func (h *Hub) BroadcastGlobal(message WSMessage) {
    // 遍历所有 Thread 的所有客户端
}
```

- [x] **Step 2: Commit**

```bash
git add internal/ws/hub.go
git commit -m "feat: WebSocket BroadcastGlobal 全局广播"
```

---

## 验证清单

| 功能 | 验证方式 | 状态 |
|------|---------|------|
| AskUserQuestion 触发创建 | Agent 提问后检查待办任务列表 | ✅ |
| 待办任务页面 | `/tasks` 页面正常显示 | ✅ |
| 状态筛选 | Tabs 切换筛选 | ✅ |
| 手动完成/取消 | 点击按钮操作 | ✅ |
| 进入对话 | 点击跳转到 Thread | ✅ |
| 用户回复关闭 | 回答后任务自动消失 | ✅ |
| WebSocket 实时更新 | 新任务自动出现，关闭自动消失 | ✅ |
| 时间显示正确 | 显示相对时间（几分钟前） | ✅ |
| 项目信息显示 | 显示项目名和任务名 | ✅ |

---

## 已完成 Commit

所有任务已完成，最终提交：

```
f285f0e feat: 完善待办任务管理功能
```