# 自由协作模式设计方案

## 一、背景与目标

### 问题
ISDP 当前只支持工作流模式（WorkflowTemplate），Agent 按预设的 Transitions 顺序执行。但有时需要：
- 多 Agent 并行讨论同一问题
- 自由发起 A2A 协作，不受固定流程约束
- 类似 Clowder AI 的 multi_mention 机制

### 目标
在现有架构基础上，新增"自由协作模式"团队类型，支持：
1. 多 Agent 并行讨论（参考 multi_mention）
2. Agent 间自由 @触发（A2A）
3. 项目级团队绑定
4. 完整的 MCP 工具链支持

## 二、现有架构分析

### 已有的基础设施
| 组件 | 文件 | 说明 |
|------|------|------|
| InvocationRegistry | `internal/service/a2a/invocation_registry.go` | 调用注册表，支持 invocationId + callbackToken |
| InvocationQueue | `internal/service/a2a/invocation_queue.go` | 调用队列，支持 A2A 排队 |
| WorklistRegistry | `internal/service/a2a/worklist_registry.go` | 工作链管理（串行 A2A）|
| EnqueueA2ATargets | `internal/service/a2a/a2a_trigger.go` | A2A 触发函数 |
| AgentRoleConfig | `internal/model/agent_config.go` | Agent 角色配置，含 MentionPatterns |
| WorkflowTemplate | `internal/model/workflow_template.go` | 工作流模板（当前只有工作流模式）|
| **Skill 系统** | `internal/service/configgen/` | **已有完整 Skill 系统** ⭐ |
| **configgen.Service** | `internal/service/configgen/service.go` | **生成 Agent 配置到 {configDir}/skills/** |
| **Downloader** | `internal/service/configgen/downloader.go` | **复制 Skill 目录到配置目录** |

### Skill 加载机制
ISDP 已有完整的 Skill 加载链路：

1. **存储**：Skill 存放在 `{skillStoragePath}/{skillName}/SKILL.md`
2. **绑定**：通过 `agent_skill_bindings` 表关联 AgentRole 和 Skill
3. **生成**：`configgen.GenerateAgentConfig()` 复制 Skill 到 `{configDir}/skills/`
4. **加载**：Agent CLI 通过 `CLAUDE_CONFIG_DIR` 环境变量自动加载

**无需新建加载机制**，只需创建 Skill 文件并绑定。

## 三、架构设计

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        前端层                                │
│  ProjectDetail: "自由讨论"按钮                               │
│  ThreadView: 自由模式 UI、TeammateRoster                     │
│  MultiMentionModal: Agent 多选、发起讨论                     │
└─────────────────────────────────────────────────────────────┘
                              ↓ REST API
┌─────────────────────────────────────────────────────────────┐
│                        API 层                                │
│  POST /callbacks/multi-mention                              │
│  GET  /callbacks/multi-mention-status                       │
│  GET  /callbacks/teammate-roster                            │
│  Thread API 扩展 (type 参数)                                 │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                     MCP 工具层                               │
│  isdp_multi_mention (MCP Tool)                              │
│  isdp_get_teammate_roster (MCP Tool)                        │
│  工具 Handler → 服务层调用                                   │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                     核心服务层                               │
│  MultiMentionOrchestrator: 状态机、聚合、级联防护             │
│  TeammateRosterBuilder: 队友列表构建                         │
│  ContextBuilder 扩展: 协作信息注入                           │
│  ExecutionService 集成: 任务分发、回调触发                   │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                        数据层                                │
│  Thread: type, availableAgents                              │
│  WorkflowTemplate: mode                                     │
│  MultiMentionRequest, MultiMentionResponse                  │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                      Skill 层                                │
│  free-collaboration/SKILL.md                                │
│  协作原则、工具使用说明                                       │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 Skill 与 MCP 工具的关系

```
SKILL.md（指导文档）
     ↓ 告诉 Agent 何时使用什么工具
MCP Tool（isdp_multi_mention）
     ↓ 通过 API 端点调用
API Handler（/callbacks/multi-mention）
     ↓ 业务逻辑
MultiMentionOrchestrator
```

**重要**：Skill 是**指导文档**，告诉 Agent 如何协作；MCP 工具是**执行接口**，实现具体功能。两者是分离的。

## 四、数据模型设计

### 4.1 Thread 扩展

```go
// internal/model/thread.go

type ThreadType string

const (
    ThreadTypeWorkflow       ThreadType = "workflow"        // 工作流模式（默认）
    ThreadTypeFreeDiscussion ThreadType = "free_discussion" // 自由协作模式
)

type Thread struct {
    // ... existing fields
    Type            ThreadType `json:"type"`                         // 会话类型
    AvailableAgents []string   `json:"availableAgents,omitempty"`    // 可用 Agent 范围（自由模式）
}
```

### 4.2 WorkflowTemplate 扩展

```go
// internal/model/workflow_template.go

type TeamMode string

const (
    TeamModeWorkflow TeamMode = "workflow" // 工作流模式（顺序执行）
    TeamModeFree     TeamMode = "free"     // 自由模式（并行协作）
)

type WorkflowTemplate struct {
    // ... existing fields
    Mode TeamMode `json:"mode"` // 团队模式
}
```

### 4.3 MultiMentionRequest

```go
// internal/model/multi_mention.go

type MultiMentionStatus string

const (
    MultiMentionStatusPending   MultiMentionStatus = "pending"
    MultiMentionStatusRunning   MultiMentionStatus = "running"
    MultiMentionStatusPartial   MultiMentionStatus = "partial"
    MultiMentionStatusDone      MultiMentionStatus = "done"
    MultiMentionStatusTimeout   MultiMentionStatus = "timeout"
    MultiMentionStatusFailed    MultiMentionStatus = "failed"
)

type MultiMentionRequest struct {
    ID              uuid.UUID          `json:"id"`
    ThreadID        uuid.UUID          `json:"threadId"`
    Initiator       string             `json:"initiator"`       // 发起者 AgentID
    CallbackTo      string             `json:"callbackTo"`      // 回调给谁
    Targets         []string           `json:"targets"`         // 目标 AgentIDs (1-3)
    Question        string             `json:"question"`
    Context         string             `json:"context,omitempty"`
    Status          MultiMentionStatus `json:"status"`
    TimeoutMinutes  int                `json:"timeoutMinutes"`
    SearchEvidence  []string           `json:"searchEvidence,omitempty"`
    OverrideReason  string             `json:"overrideReason,omitempty"`
    CreatedAt       time.Time          `json:"createdAt"`
}

type MultiMentionResponse struct {
    ID        uuid.UUID `json:"id"`
    RequestID uuid.UUID `json:"requestId"`
    AgentID   string    `json:"agentId"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"createdAt"`
}
```

## 五、API 设计

### 5.1 Multi-Mention API

#### POST /api/v1/callbacks/multi-mention

**请求**：
```json
{
  "threadId": "uuid",
  "targets": ["AgentA", "AgentB"],
  "question": "架构选型建议",
  "callbackTo": "InitiatorAgent",
  "searchEvidenceRefs": ["已查看 auth 模块"],
  "overrideReason": null,
  "timeoutMinutes": 8
}
```

**响应**：
```json
{
  "invocationId": "uuid",
  "callbackToken": "token",
  "status": "running"
}
```

**错误**：
- 400: targets 数量超限（>3 或 <1）
- 400: 无 searchEvidenceRefs 且无 overrideReason（"先搜后问"原则）
- 400: Agent 在 activeTargets 中（级联防护）
- 404: threadId 不存在

#### GET /api/v1/callbacks/multi-mention-status

**请求**：`?id=<invocationId>`

**响应**：
```json
{
  "id": "uuid",
  "status": "done",
  "responses": [
    { "agentId": "AgentA", "content": "建议 JWT" },
    { "agentId": "AgentB", "content": "建议 Session" }
  ]
}
```

### 5.2 Teammate Roster API

#### GET /api/v1/callbacks/teammate-roster

**请求**：`?threadId=<threadId>`

**响应**：
```json
{
  "agents": [
    { "id": "AgentA", "name": "架构师", "role": "architect", "skills": ["架构设计"] },
    { "id": "AgentB", "name": "前端开发", "role": "frontend", "skills": ["React", "Vue"] }
  ]
}
```

## 六、核心服务设计

### 6.1 MultiMentionOrchestrator 状态机

```
状态流转：
pending → running → done
                  ↘ partial (部分响应)
                  ↘ timeout
                  ↘ failed (执行错误)
```

**核心方法**：

| 方法 | 功能 |
|------|------|
| `Create(params)` | 创建请求，校验参数（targets 数量、证据规则） |
| `Start(requestID)` | 启动执行，状态 pending → running |
| `RecordResponse(requestID, agentID, content)` | 记录响应，判断是否全部收到 |
| `HandleTimeout(requestID)` | 超时处理，已收到响应仍聚合 |
| `GetResult(requestID)` | 获取聚合结果 |
| `IsActiveTarget(threadID, agentID)` | 判断是否为活跃目标（级联防护） |

**级联防护**：
- 维护 `activeTargets` map（threadID → agentIDs）
- Agent 在 activeTargets 中调用 multi_mention 时返回错误
- multi_mention 完成后移除 activeTargets

### 6.2 TeammateRosterBuilder

| 模式 | 数据来源 |
|------|----------|
| 自由模式 | Thread.availableAgents |
| 工作流模式 | WorkflowTemplate.agentIds（通过 Thread.workflowTemplateId 查询） |

**输出格式**：Agent ID、名称、角色、擅长领域

### 6.3 ContextBuilder 扩展

在 Layer0 系统提示词中注入：

1. **队友名册**：`BuildTeammateRoster()` - 当前可用 Agent 列表
2. **协作规则**："先搜后问"原则、级联防护规则
3. **工作流触发点**（工作流模式）：`BuildWorkflowTriggers()`

## 七、Skill 包设计

### 7.1 目录结构

```
{skillStoragePath}/
└── free-collaboration/
    └── SKILL.md
```

### 7.2 SKILL.md 内容

```markdown
---
name: free-collaboration
description: >
  多 Agent 并行讨论、独立思考、结果聚合。
  Use when: 需要多 Agent 视角、架构选型、跨领域问题、复杂决策。
triggers:
  - "多Agent讨论"
  - "并行思考"
  - "multi mention"
  - "需要多方意见"
---

# 自由协作

## 协作原则

### 先搜后问
调用 multi_mention 前，必须先搜索相关资料（代码、文档、知识库）。
提供 `searchEvidenceRefs` 参数，说明你找到了什么证据。

**例外情况**：如果确实无法搜索（如全新概念），必须提供 `overrideReason` 说明理由。

### 防止级联
被召唤的 Agent 不得再次发起 multi_mention，避免无限级联。

## 可用的 MCP 工具

### isdp_multi_mention
并行邀请 1-3 个 Agent 讨论同一问题。

**参数**：
- `targets`: 目标 Agent ID 列表（1-3 个）
- `question`: 问题内容
- `callbackTo`: 回调 Agent（通常是自己）
- `searchEvidenceRefs`: 搜索证据引用（必须）
- `overrideReason`: 跳过搜索的理由（可选）

**使用示例**：
```json
{
  "targets": ["架构师", "安全专家"],
  "question": "用户认证方案选型：JWT vs Session？",
  "callbackTo": "前端开发",
  "searchEvidenceRefs": [
    "已查看 auth/auth.go 现有实现",
    "已搜索 OWASP 认证最佳实践"
  ]
}
```

### isdp_get_teammate_roster
获取当前团队中可 @ 的队友列表及其擅长领域。

## 协作流程

1. **发现问题** → 判断是否需要多方意见
2. **搜索证据** → 查看代码、文档、历史记录
3. **发起讨论** → 调用 multi_mention（带证据）
4. **等待聚合** → 系统自动收集各 Agent 响应
5. **收到回调** → 整合意见，做出决策

## 注意事项

- 目标 Agent 数量限制为 1-3 个
- 默认超时 8 分钟
- 被召唤的 Agent 只能响应，不能继续召唤
```

## 八、完整测试矩阵

### 8.1 测试覆盖图

```
E2E 测试        │ 完整业务流程验证
────────────────┼──────────────────────
API 集成测试    │ 路由、数据持久化、跨服务调用
────────────────┼──────────────────────
服务单元测试    │ MultiMentionOrchestrator、ContextBuilder
────────────────┼──────────────────────
MCP 工具测试    │ 工具注册、参数验证、错误处理
────────────────┼──────────────────────
Skill 加载测试  │ 文件存在、绑定关系、CLI 加载
────────────────┼──────────────────────
数据库迁移测试  │ 字段新增、表创建、约束验证
────────────────┼──────────────────────
前端组件测试    │ 入口、UI、交互
```

### 8.2 数据库迁移测试

| 测试 | 验证点 |
|------|--------|
| `TestMigrationThreadType` | threads 表新增 type、available_agents 字段，默认值为 workflow |
| `TestMigrationTemplateMode` | workflow_templates 表新增 mode 字段，默认值为 workflow |
| `TestMigrationMultiMentionTables` | multi_mention_requests、multi_mention_responses 表创建成功 |
| `TestMigrationConstraints` | 外键约束正确（thread_id → threads.id） |
| `TestMigrationRollback` | 回滚脚本能正确删除新增字段和表 |

### 8.3 MultiMentionOrchestrator 状态机测试

| 测试方法 | 输入状态 | 操作 | 验证输出状态 |
|----------|----------|------|-------------|
| `TestStateTransition_PendingToRunning` | pending | Start() | running |
| `TestStateTransition_RunningToDone` | running | RecordResponse(所有目标) | done |
| `TestStateTransition_RunningToPartial` | running | RecordResponse(部分目标) | partial |
| `TestStateTransition_RunningToTimeout` | running | HandleTimeout() | timeout |
| `TestStateTransition_RunningToFailed` | running | Agent 执行失败 | failed |
| `TestStateTransition_InvalidStart` | done | Start() | 错误（已完成不能再启动） |
| `TestStateTransition_InvalidRecord` | pending | RecordResponse() | 错误（未启动不能记录） |

### 8.4 边界条件测试

| 测试方法 | 输入 | 验证点 |
|----------|------|--------|
| `TestTargetsMin` | targets=["A"] | 1 个目标合法 |
| `TestTargetsMax` | targets=["A","B","C"] | 3 个目标合法 |
| `TestTargetsExceed` | targets=["A","B","C","D"] | 返回错误 |
| `TestTargetsEmpty` | targets=[] | 返回错误 |
| `TestTargetsInvalidAgent` | targets=["不存在Agent"] | 返回错误（不在 availableAgents） |
| `TestEvidenceRequired` | 无 searchEvidenceRefs 且无 overrideReason | 返回错误 |
| `TestEvidenceProvided` | searchEvidenceRefs=["证据"] | 合法 |
| `TestOverrideReason` | overrideReason="全新概念无资料" | 合法 |
| `TestCallbackToInvalid` | callbackTo="不存在Agent" | 返回错误 |
| `TestDuplicateTargets` | targets=["A","A"] | 去重处理 |

### 8.5 级联防护测试

| 测试方法 | 场景 | 验证点 |
|----------|------|--------|
| `TestCascadePrevention_ActiveTarget` | Agent 在 activeTargets 中调用 multi_mention | 返回错误 |
| `TestCascadePrevention_NormalAgent` | 正常 Agent 调用 multi_mention | 成功 |
| `TestCascadePrevention_MultiLevel` | 二级被召唤 Agent 再次召唤 | 返回错误 |
| `TestIsActiveTarget_AfterDone` | multi_mention 完成后 | Agent 不再是 activeTarget |

### 8.6 超时机制测试

| 测试方法 | 场景 | 验证点 |
|----------|------|--------|
| `TestTimeout_Default` | 未指定 timeoutMinutes | 默认 8 分钟 |
| `TestTimeout_Custom` | timeoutMinutes=5 | 5 分钟后触发超时 |
| `TestTimeout_AfterPartialResponse` | 超时前收到部分响应 | 已收到响应仍聚合回调 |
| `TestTimeout_NoResponse` | 超时前无响应 | 回调空结果，标记 timeout |

### 8.7 E2E 测试闭环

```gherkin
Feature: 自由协作完整流程

Scenario: 完整流程闭环
  # Step 1: 创建自由讨论会话
  Given 用户在项目详情页
  When 用户点击"自由讨论"按钮
  Then 创建 Thread（type=free_discussion）
  And Thread.availableAgents 正确填充
  
  # Step 2: Agent 发起 multi_mention
  Given Agent A1 执行中
  When A1 调用 isdp_multi_mention（targets=["A2","A3"]）
  Then 创建 MultiMentionRequest（status=running）
  And A2、A3 收到任务
  
  # Step 3: 响应聚合
  Given A2、A3 执行任务
  When A2、A3 返回响应
  Then status=done
  And A1 收到聚合回调
  
Scenario: 级联防护闭环
  Given A2 被召唤执行
  When A2 尝试调用 isdp_multi_mention
  Then 返回错误 "当前是被召唤状态，禁止级联"

Scenario: 超时处理闭环
  Given MultiMentionRequest 运行中
  And A2 已响应，A3 未响应
  When 8 分钟后
  Then status=timeout
  And A1 收到 A2 响应（部分聚合）
```

### 8.8 Skill 加载闭环测试

```gherkin
Feature: Skill 加载验证

Scenario: Skill 从存储到 CLI 加载闭环
  Given free-collaboration Skill 存在于 {skillStoragePath}
  When 创建自由模式团队
  Then 系统自动绑定 Skill 到所有 Agent
  
  Given Agent 执行启动
  And configgen.GenerateAgentConfig() 执行
  Then Skill 复制到 {configDir}/skills/
  
  Given CLI 启动
  And CLAUDE_CONFIG_DIR={configDir}
  Then CLI 加载 SKILL.md
  And Agent 可调用 isdp_multi_mention
```

## 九、验收标准

### 9.1 功能验收标准

| 功能点 | 验收标准 | 验证方法 |
|--------|----------|----------|
| 自由讨论会话创建 | 用户可创建 type=free_discussion 的 Thread | E2E + 前端验证 |
| 队友名册显示 | ThreadView 显示可 @ 队友列表 | 前端组件测试 |
| multi_mention 发起 | Agent 可发起 1-3 目标的并行讨论 | MCP 工具测试 |
| 响应聚合 | 响应自动聚合回调给发起 Agent | 状态机测试 |
| 超时处理 | 超时后已收到响应仍聚合 | 超时测试 |
| 级联防护 | 被召唤 Agent 无法再次发起 | 边界测试 |
| Skill 加载 | CLI 正确加载 Skill | 加载闭环测试 |

### 9.2 非功能验收标准

| 类别 | 验收标准 | 验证方法 |
|------|----------|----------|
| 性能 | multi_mention 创建 < 200ms；状态查询 < 50ms | API 响应测试 |
| 并发 | 同一 Thread 可同时运行多个 multi_mention | 并发测试 |
| 可观测性 | 关键操作有日志记录 | 日志检查 |
| 安全 | callbackToken 校验；targets 范围校验 | 安全测试 |

### 9.3 发布前验收检查清单

```markdown
## 发布验收检查清单

### Phase 1: 数据层
- [ ] 数据库迁移脚本执行成功
- [ ] 新字段有正确默认值
- [ ] 外键约束正确
- [ ] 原有数据未丢失

### Phase 2: 服务层
- [ ] MultiMentionOrchestrator 所有状态流转测试通过
- [ ] 边界条件测试全部通过
- [ ] 级联防护测试通过
- [ ] 超时机制测试通过
- [ ] 聚合回调测试通过

### Phase 3: API 层
- [ ] multi-mention 创建 API 测试通过
- [ ] multi-mention-status 查询 API 测试通过
- [ ] teammate-roster API 测试通过
- [ ] 向后兼容测试通过

### Phase 4: MCP 工具层
- [ ] isdp_multi_mention Schema 验证通过
- [ ] isdp_multi_mention Handler 测试通过
- [ ] isdp_get_teammate_roster 测试通过

### Phase 5: Skill 层
- [ ] SKILL.md 文件创建成功
- [ ] Skill 绑定记录正确
- [ ] CLI 加载 Skill 成功

### Phase 6: 前端层
- [ ] 自由讨论按钮渲染正确
- [ ] ThreadView 自由模式 UI 正确
- [ ] Agent 多选组件功能正确

### Phase 7: E2E 闭环
- [ ] 完整流程测试通过
- [ ] 级联防护闭环测试通过
- [ ] 超时处理闭环测试通过
- [ ] Skill 加载闭环测试通过
```

## 十、实现子任务分解

### 10.1 子任务总览

```
Phase 1: 数据层（3 tasks）
Phase 2: 核心服务层（4 tasks）
Phase 3: API 层（3 tasks）
Phase 4: MCP 工具层（2 tasks）
Phase 5: Skill 包（2 tasks）
Phase 6: 前端层（4 tasks）
Phase 7: 集成测试（2 tasks）
Phase 8: E2E 测试（1 task）
```

### 10.2 详细子任务

| 序号 | Task ID | 任务名称 | 文件 | 前置依赖 | 参考（Clowder AI） |
|------|---------|----------|------|----------|-------------------|
| 1 | 1.1 | 数据库迁移脚本 | `sql-change/migrations/202603310001_add_free_discussion_fields.sql` | 无 | - |
| 2 | 1.2 | Go 数据模型 | `internal/model/multi_mention.go`（新增） | 1.1 | `db/schema.ts` |
| 3 | 1.3 | 数据仓库层 | `internal/repo/multi_mention_repo.go`（新增） | 1.2 | ISDP repo 模式 |
| 4 | 2.1 | MultiMentionOrchestrator | `internal/service/a2a/multi_mention_orchestrator.go`（新增） | 1.3 | `callback-multi-mention-routes.ts` |
| 5 | 2.2 | TeammateRosterBuilder | `internal/service/agent/teammate_roster_builder.go`（新增） | 1.2 | `callback-teammate-roster-routes.ts` |
| 6 | 2.3 | ContextBuilder 扩展 | `internal/service/agent/context_builder.go`（修改） | 2.2 | `SystemPromptBuilder.ts` |
| 7 | 2.4 | Agent 执行集成 | `internal/service/agent/execution_service.go`（修改） | 2.1 | ISDP A2A 机制 |
| 8 | 3.1 | Multi-Mention API | `internal/api/handlers/callback_multi_mention.go`（新增） | 2.1 | `callback-multi-mention-routes.ts` |
| 9 | 3.2 | Teammate Roster API | `internal/api/handlers/callback_teammate_roster.go`（新增） | 2.2 | `callback-teammate-roster-routes.ts` |
| 10 | 3.3 | Thread API 扩展 | `internal/api/handlers/thread.go`（修改） | 1.2 | ISDP Thread API |
| 11 | 4.1 | MCP 工具定义 | `internal/mcp/tools/callback_tools.go`（新增/修改） | 无 | `callback-tools.ts` |
| 12 | 4.2 | MCP 工具 Handler | `internal/mcp/handlers/callback_handlers.go`（新增/修改） | 4.1, 2.1 | `callback-tools.ts` |
| 13 | 5.1 | SKILL.md 文件 | `{skillStoragePath}/free-collaboration/SKILL.md`（新增） | 无 | `collaborative-thinking/SKILL.md` |
| 14 | 5.2 | Skill 初始化脚本 | `sql-change/migrations/202603310002_init_free_collaboration_skill.sql`（新增） | 5.1 | ISDP Skill 初始化 |
| 15 | 6.1 | 自由讨论入口 | `web/src/pages/ProjectDetail/index.tsx`（修改） | 3.3 | ISDP Thread 创建 |
| 16 | 6.2 | ThreadView UI | `web/src/pages/ThreadView.tsx`（修改） | 6.1 | ISDP ThreadView |
| 17 | 6.3 | TeammateRoster 组件 | `web/src/components/TeammateRoster/index.tsx`（新增） | 3.2 | Clowder UI |
| 18 | 6.4 | MultiMentionModal 组件 | `web/src/components/MultiMentionModal/index.tsx`（新增） | 3.1, 6.3 | Clowder UI |
| 19 | 7.1 | 后端集成测试 | `internal/api/handlers/*_test.go`（新增） | 3.1-3.3 | ISDP 测试模式 |
| 20 | 7.2 | 服务单元测试 | `internal/service/a2a/*_test.go`（新增） | 2.1 | 测试矩阵 |
| 21 | 8.1 | E2E 测试 | `tests/e2e/free_discussion_test.go`（新增） | 所有前置 | 测试矩阵 |

### 10.3 开发过程验证机制

每个 Task 完成后执行 Checkpoint：

```
1. 代码实现完成
2. 单元测试编写并通过
3. 对照设计文档验证实现
4. 对照 Clowder AI 实现验证（如有参考）
5. 更新 CHANGELOG
6. Git commit（commit message 包含 Task ID）
```

**对照 Clowder AI 验证要点**：

| 功能 | Clowder AI 参考文件 | 验证要点 |
|------|---------------------|----------|
| MultiMention 状态流转 | `callback-multi-mention-routes.ts` | 状态定义、超时处理、聚合逻辑 |
| MCP 工具定义 | `callback-tools.ts` | 参数 Schema、"先搜后问"规则 |
| Skill 内容 | `collaborative-thinking/SKILL.md` | 协作原则、工具使用说明 |
| Teammate Roster | `callback-teammate-roster-routes.ts` | 返回格式、Agent 信息 |
| 系统提示词注入 | `SystemPromptBuilder.ts` | 提示词格式、注入位置 |

## 十一、风险与缓解

| 风险 | 缓解措施 |
|------|---------|
| Token 消耗大 | 限制并行 Agent 数量（最多 3 个） |
| 响应超时 | 默认 8 分钟超时，支持配置 |
| 级联调用 | isActiveTarget 检查防止级联 |
| 资源竞争 | InvocationQueue 去重保护 |
| Skill 未绑定 | 自由模式团队默认绑定内置 Skill |

## 十二、关键文件列表

### 新增文件
| 文件 | 说明 |
|------|------|
| `internal/model/multi_mention.go` | 多讨论请求模型 |
| `internal/repo/multi_mention_repo.go` | 数据仓库 |
| `internal/service/a2a/multi_mention_orchestrator.go` | 核心编排器 |
| `internal/service/agent/teammate_roster_builder.go` | 队友列表构建 |
| `internal/api/handlers/callback_multi_mention.go` | API 处理器 |
| `internal/api/handlers/callback_teammate_roster.go` | API 处理器 |
| `internal/mcp/tools/callback_tools.go` | MCP 工具定义 |
| `internal/mcp/handlers/callback_handlers.go` | MCP Handler |
| `{skillStoragePath}/free-collaboration/SKILL.md` | 内置 Skill 文件 |
| `sql-change/migrations/202603310001_add_free_discussion_fields.sql` | 数据库迁移 |
| `sql-change/migrations/202603310002_init_free_collaboration_skill.sql` | Skill 初始化 |
| `web/src/components/TeammateRoster/index.tsx` | 队友名册组件 |
| `web/src/components/MultiMentionModal/index.tsx` | 多讨论发起弹窗 |

### 修改文件
| 文件 | 改动 |
|------|------|
| `internal/model/thread.go` | 新增 Type、AvailableAgents 字段（已完成） |
| `internal/model/workflow_template.go` | 新增 Mode 字段（已完成） |
| `internal/service/agent/context_builder.go` | 注入协作信息 |
| `internal/service/agent/execution_service.go` | 集成 multi_mention 分发 |
| `internal/api/handlers/thread.go` | 支持创建自由讨论会话 |
| `web/src/pages/ProjectDetail/index.tsx` | 新增自由讨论入口 |
| `web/src/pages/ThreadView.tsx` | 支持自由模式 UI |

---

**文档版本**: v1.0
**创建日期**: 2026-03-31
**作者**: Claude Code