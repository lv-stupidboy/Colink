# 人角色设计 - 团队协作中的人工节点

> **目标:** 将人作为团队成员角色纳入工作流，支持 Agent 与人的混编协作。Agent 可将无法完成的任务分发给对应的人角色，人提交交付物后自动流转下游。

## 背景

当前团队工作流中所有角色都是 Agent（通过 CLI 执行）。实际运作中存在 Agent 无法完成的任务：
- 业务决策（需要人工判断优先级、方案选择）
- 安全/合规审核（需要人工签字确认）
- 外部操作（线下采购、外部系统沟通）

需要将"人"作为特殊角色纳入团队，通过 Transition 定义触发条件，实现人-Agent 混编协作。

---

## 设计概览

### 核心变更

1. **角色类型简化** - `Role` 字段从细分类型改为大类区分（`agent` / `human`）
2. **触发机制扩展** - Mention 解析支持人角色，创建任务卡片而非启动 CLI
3. **任务卡片系统** - Thread 内展示 + 任务中心汇总
4. **交付物提交** - 支持文本 + 文件上传，提交后自动流转下游

### 三种场景支持

| 场景 | 说明 |
|------|------|
| 任务分发（核心） | Agent 将无法完成的任务分发给指定的人角色 |
| 审核决策 | 工作流节点需要人工审核后才能继续 |
| 人工确认 | 关键操作需要人工确认（如合并、部署） |

---

## Part 1: 角色配置模型变更

### AgentRoleConfig 结构变更

```go
// AgentRole 角色大类
type AgentRole string

const (
    AgentRoleAgent AgentRole = "agent"  // Agent 角色（CLI 执行）
    AgentRoleHuman AgentRole = "human"  // 人角色（任务卡片）
)

// AgentRoleConfig 角色配置（Agent 和人共用）
type AgentRoleConfig struct {
    ID           uuid.UUID   `json:"id"`
    Name         string      `json:"name"`           // 角色名称，如"人工审核员"
    Role         AgentRole   `json:"role"`           // 大类：agent 或 human
    BaseAgentID  uuid.UUID   `json:"baseAgentId,omitempty"` // Agent 角色需要，人角色不需要
    Description  string      `json:"description"`    // 角色描述
    SystemPrompt string      `json:"systemPrompt"`   // Agent: 执行指令；人: 职责+交付物期望
    MaxTokens    int         `json:"maxTokens"`      // Agent 角色需要
    Temperature  float64     `json:"temperature"`    // Agent 角色需要
    IsDefault    bool        `json:"isDefault"`
    IsSystem     bool        `json:"isSystem"`
    MentionPatterns []string `json:"mentionPatterns"` // @触发模式
    CreatedAt    time.Time   `json:"createdAt"`
    UpdatedAt    time.Time   `json:"updatedAt"`
}
```

### 人角色配置示例

```json
{
  "id": "uuid-xxx",
  "name": "人工审核员",
  "role": "human",
  "description": "代码安全审核",
  "systemPrompt": "职责：审查代码安全性\n交付物：审查报告（漏洞列表 + 修复建议 + 优先级）",
  "mentionPatterns": ["@人工审核员", "@审核", "@安全审核"]
}
```

### 数据迁移

现有 Agent 角色的 `Role` 字段值需要迁移：
- `"requirement"` → `"agent"`
- `"architect"` → `"agent"`
- `"developer"` → `"agent"`
- ...所有现有值 → `"agent"`

具体职责通过 `Name` + `SystemPrompt` 区分。

### extractExpectedOutput 函数

从人角色 SystemPrompt 中提取期望交付物描述：

```go
func extractExpectedOutput(systemPrompt string) string {
    // 解析格式：职责：xxx\n交付物：yyy
    lines := strings.Split(systemPrompt, "\n")
    for _, line := range lines {
        if strings.HasPrefix(line, "交付物：") || strings.HasPrefix(line, "交付物:") {
            return strings.TrimPrefix(line, "交付物：")
        }
        if strings.HasPrefix(line, "期望交付物：") || strings.HasPrefix(line, "期望交付物:") {
            return strings.TrimPrefix(line, "期望交付物：")
        }
    }
    // 未找到明确交付物描述，返回完整 SystemPrompt
    return systemPrompt
}
```

人角色 SystemPrompt 推荐格式：
```
职责：[角色职责描述]
交付物：[期望交付物格式，如：审查报告（漏洞列表 + 修复建议）]
```

---

## Part 2: 触发机制扩展

### Mention 解析扩展

现有逻辑：
```
Agent 输出: "@前端开发工程师 请实现登录页面"
    ↓
MentionParser 解析 → 匹配 AgentRoleConfig.MentionPatterns
    ↓
SpawnAgent（启动 CLI 执行）
```

新增逻辑：
```
Agent 输出: "@人工审核员 请审查登录模块安全性"
    ↓
MentionParser 解析 → 匹配 AgentRoleConfig.MentionPatterns
    ↓
判断 targetConfig.Role
    ↓
Role == "agent" → SpawnAgent（CLI 执行）
Role == "human" → CreateHumanTask（创建任务卡片）
```

### 核心代码变更位置

| 文件 | 变更 |
|------|------|
| `internal/service/a2a/a2a_trigger.go` | `EnqueueA2ATargets()` 增加 Role 判断分支 |
| `internal/service/a2a/queue_processor.go` | `TryAutoExecute()` 区分 Agent 和人角色的执行方式 |
| `internal/service/agent/mention/parser.go` | 解析逻辑不变，匹配 MentionPatterns |

### 任务创建函数

```go
// CreateHumanTask 创建人工任务
func (q *QueueProcessor) CreateHumanTask(ctx context.Context, target *A2ATarget) error {
    // 1. 获取人角色配置
    humanConfig := target.AgentConfig
    
    // 2. 创建任务卡片记录
    task := &model.HumanTask{
        ID:             uuid.New(),
        ThreadID:       target.ThreadID,
        RoleConfigID:   humanConfig.ID,
        RoleName:       humanConfig.Name,
        TaskType:       "task_dispatch", // task_dispatch / review / confirm
        TaskContent:    target.Content,  // Agent 的任务描述
        ExpectedOutput: extractExpectedOutput(humanConfig.SystemPrompt),
        SourceAgentID:  target.ParentInvocationID,
        Status:         "pending",
        CreatedAt:      time.Now(),
    }
    
    // 3. 存储任务
    if err := q.taskRepo.Create(ctx, task); err != nil {
        return err
    }
    
    // 4. 广播任务创建事件（前端展示）
    q.wsHub.BroadcastTaskCreated(task)
    
    // 5. Thread 内插入任务卡片消息
    q.msgRepo.Create(ctx, &model.Message{
        ThreadID: target.ThreadID,
        Role:     "system",
        Content:  buildTaskCardContent(task),
        MessageType: "human_task",
        Metadata: map[string]interface{}{
            "taskId": task.ID.String(),
            "taskStatus": "pending",
        },
    })
    
    return nil
}
```

---

## Part 3: 任务卡片系统

### 数据模型

```go
// HumanTask 人工任务
type HumanTask struct {
    ID             uuid.UUID   `json:"id"`
    ThreadID       uuid.UUID   `json:"threadId"`
    RoleConfigID   uuid.UUID   `json:"roleConfigId"`
    RoleName       string      `json:"roleName"`       // 角色名称
    TaskType       string      `json:"taskType"`       // task_dispatch / review / confirm
    TaskContent    string      `json:"taskContent"`    // 任务描述
    ExpectedOutput string      `json:"expectedOutput"` // 期望交付物
    SourceAgentID  uuid.UUID   `json:"sourceAgentId"`  // 来源 Agent invocation ID
    SourceAgentName string     `json:"sourceAgentName"`// 来源 Agent 名称
    Status         string      `json:"status"`         // pending / in_progress / completed / rejected / failed
    SubmittedAt    *time.Time  `json:"submittedAt"`    // 提交时间
    SubmittedBy    string      `json:"submittedBy"`    // 提交人（当前系统为用户自己）
    OutputContent  string      `json:"outputContent"`  // 交付物内容（文本）
    OutputFiles    []string    `json:"outputFiles"`    // 交付物文件路径列表
    TargetAgentID  uuid.UUID   `json:"targetAgentId"`  // 下游目标 Agent ID（自动流转）
    CreatedAt      time.Time   `json:"createdAt"`
    UpdatedAt      time.Time   `json:"updatedAt"`
}

func (t *HumanTask) TableName() string {
    return "human_tasks"
}
```

### Thread 内展示

任务卡片以 Message 形式嵌入 Thread 消息流：

```json
{
  "id": "msg-xxx",
  "threadId": "thread-xxx",
  "role": "system",
  "messageType": "human_task",
  "content": "",
  "metadata": {
    "taskId": "task-xxx",
    "taskStatus": "pending",
    "taskType": "task_dispatch",
    "roleName": "人工审核员",
    "taskContent": "请审查登录模块安全性...",
    "expectedOutput": "审查报告（漏洞列表 + 修复建议）",
    "sourceAgentName": "代码审查员"
  }
}
```

前端渲染为任务卡片组件。

### 任务中心页面

新增路由 `/tasks`，汇总展示所有状态的任务：

```
我的任务
├── ⏳ 待处理 (2)
│   ├── 代码安全审核 - 来自 @代码审查员
│   └── 业务优先级判定 - 来自 @需求分析师
├── 🔄 进行中 (1)
│   └── 文档编写 - 来自 @开发工程师
├── ✅ 已完成 (5)
│   ├── ...
└── ❌ 已拒绝 (0)
```

---

## Part 4: 交付物提交与自动流转

### 提交入口

两种入口：
1. **Thread 内** - 点击任务卡片"执行任务"按钮，在当前 Thread 回复
2. **任务中心** - 点击任务进入详情页，提交交付物

### 提交 API

```
POST /api/v1/human-tasks/{taskId}/submit

Request:
{
  "outputContent": "审查完成，发现 3 个安全问题：\n1. 密码未加密...\n2. 缺少 CSRF...",
  "outputFiles": [
    "/data/artifacts/security-review-report.md"
  ]
}

Response:
{
  "success": true,
  "nextAgent": {
    "id": "agent-xxx",
    "name": "开发工程师"
  },
  "triggered": true
}
```

### 自动流转逻辑

```go
// SubmitHumanTask 提交人工任务并触发下游
func (s *HumanTaskService) Submit(ctx context.Context, taskID uuid.UUID, output *SubmitOutput) error {
    // 1. 更新任务状态
    task, err := s.taskRepo.FindByID(ctx, taskID)
    if err != nil {
        return err
    }
    task.Status = "completed"
    task.OutputContent = output.Content
    task.OutputFiles = output.Files
    task.SubmittedAt = time.Now()
    s.taskRepo.Update(ctx, task)

    // 2. 创建交付物消息（在 Thread 内）
    msg := &model.Message{
        ThreadID: task.ThreadID,
        Role:     "human",
        AgentID:  task.RoleConfigID.String(),
        Content:  output.Content,
        MessageType: "human_output",
        Metadata: map[string]interface{}{
            "taskId": taskID.String(),
            "outputFiles": output.Files,
        },
    }
    s.msgRepo.Create(ctx, msg)

    // 3. 查找下游目标（根据 Transition 定义）
    // Transition 使用 RoleConfigID 作为节点标识（人角色和 Agent 角色共用 RoleConfig 表）
    transitions := s.workflowRepo.GetTransitionsFromRole(ctx, task.RoleConfigID)
    if len(transitions) > 0 {
        // 取第一个下游目标
        nextTransition := transitions[0]
        nextAgentID := nextTransition.ToAgentID
        
        // 4. 触发下游 Agent
        target := &A2ATarget{
            ThreadID:         task.ThreadID,
            AgentConfigID:    nextAgentID,
            Content:          buildTriggerContent(task, output),
            ParentInvocationID: task.SourceAgentID,
        }
        s.a2aTrigger.EnqueueA2ATargets(ctx, target)
    }

    // 5. 广播任务完成事件
    s.wsHub.BroadcastTaskCompleted(task)

    return nil
}
```

---

## Part 5: 前端组件

### 任务卡片组件 (HumanTaskCard)

```
┌─────────────────────────────────────────────────────────────┐
│ 📋 任务: 代码安全审核                                       │
│                                                             │
│ 来源: @代码审查员                                           │
│ Thread: 登录功能开发                                         │
│                                                             │
│ 任务描述:                                                   │
│ ─────────────────────────────────────────                  │
│ 请审查登录模块的安全性，重点关注密码存储、                   │
│ Session 管理、CSRF 保护等方面。                             │
│ ─────────────────────────────────────────                  │
│                                                             │
│ 期望交付物:                                                 │
│ 审查报告（漏洞列表 + 修复建议 + 优先级）                      │
│                                                             │
│ ─────────────────────────────────────────                  │
│                                                             │
│ [执行任务] [查看上下文]                                      │
└─────────────────────────────────────────────────────────────┘
```

### 任务执行弹窗

点击"执行任务"后弹出：

```
┌─────────────────────────────────────────────────────────────┐
│ 执行任务: 代码安全审核                                      │
│                                                             │
│ 交付内容:                                                   │
│ ┌───────────────────────────────────────────────────────┐   │
│ │ [文本输入框 - 多行]                                    │   │
│ │                                                       │   │
│ │ 审查完成，发现 3 个安全问题：                           │   │
│ │ 1. 密码未加密存储...                                   │   │
│ │ 2. 缺少 CSRF 保护...                                  │   │
│ │ 3. Session 超时未设置...                              │   │
│ └───────────────────────────────────────────────────────┘   │
│                                                             │
│ 上传文件:                                                   │
│ [选择文件] 已选择: security-review-report.md                │
│                                                             │
│ ─────────────────────────────────────────                  │
│                                                             │
│ [提交] [取消]                                               │
└─────────────────────────────────────────────────────────────┘
```

### 任务中心页面 (MyTasks)

新增页面路由 `/tasks`：

```
┌─────────────────────────────────────────────────────────────┐
│ 我的任务                                                    │
│                                                             │
│ Tabs: [待处理] [进行中] [已完成] [已拒绝]                    │
│                                                             │
│ 待处理任务列表:                                             │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ 📋 代码安全审核                                          │ │
│ │ 来源: @代码审查员 | Thread: 登录功能开发                  │ │
│ │ 创建: 10分钟前                                           │ │
│ │                                           [执行任务]     │ │
│ └─────────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ 📋 业务优先级判定                                        │ │
│ │ 来源: @需求分析师 | Thread: 产品迭代                      │ │
│ │ 创建: 1小时前                                            │ │
│ │                                           [执行任务]     │ │
│ └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

---

## Part 6: 数据库变更

### 新增表

```sql
-- human_tasks 人工任务表
CREATE TABLE human_tasks (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    role_config_id TEXT NOT NULL,
    role_name TEXT NOT NULL,
    task_type TEXT NOT NULL DEFAULT 'task_dispatch',
    task_content TEXT NOT NULL,
    expected_output TEXT,
    source_agent_id TEXT,
    source_agent_name TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    submitted_at TEXT,
    submitted_by TEXT,
    output_content TEXT,
    output_files TEXT,  -- JSON 数组
    target_agent_id TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_human_tasks_thread ON human_tasks(thread_id);
CREATE INDEX idx_human_tasks_status ON human_tasks(status);
CREATE INDEX idx_human_tasks_role_config ON human_tasks(role_config_id);
```

### 现有表变更

```sql
-- agent_configs 表 Role 字段值迁移
-- 现有值: requirement, architect, developer, reviewer, testengineer, devops, custom
-- 新值: agent, human

-- 迁移脚本
UPDATE agent_configs SET role = 'agent' WHERE role IN ('requirement', 'architect', 'developer', 'reviewer', 'testengineer', 'devops', 'fullstack_engineer', 'custom');
```

---

## Part 7: API 接口

### 新增接口

| 路由 | 方法 | 说明 |
|------|------|------|
| `/api/v1/human-tasks` | GET | 获取任务列表（支持 status 筛选） |
| `/api/v1/human-tasks/{id}` | GET | 获取任务详情 |
| `/api/v1/human-tasks/{id}/submit` | POST | 提交交付物 |
| `/api/v1/human-tasks/{id}/start` | PUT | 开始执行（状态改为 in_progress） |
| `/api/v1/human-tasks/{id}/reject` | PUT | 拒绝任务 |

### Transition 扩展说明

现有 Transition 定义使用 `FromAgentID` 和 `ToAgentID`，字段名虽为 Agent 但实际指向 `agent_configs` 表的 ID。人角色也存储在同一张表中，因此可直接使用 RoleConfigID 作为节点标识。

需要修改的点：
- Transition 字段名建议改为语义更准确的 `FromRoleID` / `ToRoleID`（向后兼容保留旧字段名）
- WorkflowTemplate 支持创建指向人角色的 Transition

```go
// Transition 转换规则（支持 Agent 和人角色）
type Transition struct {
    FromRoleID string `json:"fromRoleId"` // 来源角色（agent_configs.id）
    ToRoleID   string `json:"toRoleId"`   // 目标角色（agent_configs.id）
    Type       string `json:"type"`       // handover / review / confirm
    TriggerHint string `json:"triggerHint"` // 触发提示语
}
```

---

## Part 8: 错误处理

### 任务创建失败

- 记录日志，不阻断上游 Agent 执行
- 任务状态设为 "failed"，记录失败原因到 OutputContent
- 前端展示错误提示，用户可手动重试或忽略

### 提交超时

- 人角色任务无超时限制（等待人工响应）
- 可手动取消/拒绝任务

### 下游触发失败

- 交付物提交成功，但下游 Agent 启动失败
- 任务状态仍为 "completed"，记录触发失败原因
- 用户可手动触发下游

---

## 数据流图

```
┌──────────────────────────────────────────────────────────────┐
│ Agent 输出                                                   │
│   "@人工审核员 请审查登录模块安全性"                          │
└──────────────────────────────────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ MentionParser 解析                                           │
│   匹配 MentionPatterns: ["@人工审核员", "@审核"]             │
└──────────────────────────────────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ 查找 AgentRoleConfig                                         │
│   RoleConfig.Role == "human"                                 │
└──────────────────────────────────────────────────────────────┐
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ CreateHumanTask                                              │
│   创建 HumanTask 记录                                        │
│   Thread 内插入任务卡片消息                                   │
│   广播 WebSocket 事件                                        │
└──────────────────────────────────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ 前端展示                                                      │
│   Thread 内: 任务卡片组件                                    │
│   任务中心: 任务列表                                         │
└──────────────────────────────────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ 用户执行任务                                                  │
│   点击"执行任务" → 填写交付物 → 提交                          │
└──────────────────────────────────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ SubmitHumanTask                                              │
│   更新任务状态 → 创建交付物消息                              │
│   查找下游 Transition → 自动触发下游 Agent                   │
└──────────────────────────────────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│ 下游 Agent 执行                                              │
│   收到交付物 → 继续工作流                                    │
└──────────────────────────────────────────────────────────────┘
```

---

## 测试要点

### 角色配置

1. 创建人角色配置（Role="human"）
2. 人角色配置缺少 MentionPatterns 时无法被触发
3. 人角色配置的 SystemPrompt 正确展示期望交付物

### 触发机制

1. Agent @人角色 → 任务卡片正确创建
2. Agent @不存在的人角色 → 无响应（不影响 Agent 继行）
3. 多个 Agent 同时 @同一人角色 → 不合并，创建多个独立任务
   - 每个任务独立记录来源 Agent
   - 前端展示时可按 RoleName 分组显示
   - 用户逐个处理或批量处理（后续扩展）

### 任务卡片

1. Thread 内任务卡片正确渲染
2. 任务中心列表正确展示所有状态任务
3. 任务状态变更后 WebSocket 实时更新

### 交付物提交

1. 提交文本交付物 → 下游 Agent 正确接收
2. 提交文件交付物 → 文件路径正确存储
3. 拒绝任务 → 不触发下游，通知上游 Agent

### 自动流转

1. 提交后自动触发下游 Agent
2. 下游 Agent 启动失败 → 错误记录，任务仍标记完成
3. 无下游 Transition → 任务完成但不触发

---

## 变更文件清单

### Go 后端

| 文件 | 变更类型 |
|------|----------|
| `internal/model/agent_config.go` | 修改 AgentRole 常量 |
| `internal/model/human_task.go` | 新增模型 |
| `internal/repo/human_task.go` | 新增仓库 |
| `internal/service/humantask/service.go` | 新增服务 |
| `internal/service/a2a/a2a_trigger.go` | 增加 Role 判断分支 |
| `internal/service/a2a/queue_processor.go` | 增加 CreateHumanTask |
| `internal/api/human_task_handler.go` | 新增接口 |
| `sql-change/v1.3.0/sqlite/00003_human_tasks.sql` | 新增表 |
| `sql-change/v1.3.0/sqlite/00004_role_migration.sql` | 数据迁移 |

### 前端

| 文件 | 变更类型 |
|------|----------|
| `web/src/pages/MyTasks/index.tsx` | 新增任务中心页面 |
| `web/src/components/HumanTaskCard/index.tsx` | 新增任务卡片组件 |
| `web/src/components/HumanTaskCard/TaskExecuteModal.tsx` | 新增执行弹窗 |
| `web/src/api/client.ts` | 新增 API 调用 |
| `web/src/types/index.ts` | 新增 HumanTask 类型 |
| `web/src/layouts/MainLayout.tsx` | 新增路由 |

---

## 实施顺序

1. **Phase 1: 数据层** - 新增表 + 数据迁移
2. **Phase 2: 角色配置** - AgentRole 变更 + 创建人角色接口
3. **Phase 3: 触发机制** - Mention 解析扩展 + CreateHumanTask
4. **Phase 4: 前端展示** - 任务卡片组件 + Thread 内展示
5. **Phase 5: 提交流转** - 提交接口 + 自动触发下游
6. **Phase 6: 任务中心** - 独立任务汇总页面

---

## 后续扩展（SaaS 版本）

当前设计针对"本地个人工具"场景（用户就是唯一的人角色）。未来 SaaS 版本可扩展：

1. **用户身份绑定** - HumanTask.SubmittedBy 记录实际用户 ID
2. **角色池分配** - 人角色绑定多个用户，任务分配给其中一人
3. **外部通知** - 任务触发时推送飞书/钉钉通知
4. **任务权限** - 不同用户只能看到分配给自己的任务