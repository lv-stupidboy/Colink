# 自由协作模式实现计划

## 设计文档

**Spec**: `docs/superpowers/specs/2026-03-31-free-collaboration-design.md`

## 实现目标

实现 ISDP 自由协作模式，支持：
- 多 Agent 并行讨论（multi_mention）
- 自由 A2A 协作
- 项目级团队绑定
- 完整 MCP 工具链

## 总体策略

**分阶段实现，每阶段独立可验证**：
- Phase 1-2: 后端基础设施（数据层 + 服务层）
- Phase 3-4: API + MCP 工具层
- Phase 5: Skill 包
- Phase 6: 前端 UI
- Phase 7-8: 测试闭环

**每个任务完成后执行 Checkpoint**：
1. 代码实现
2. 单元测试通过
3. 对照设计文档验证
4. 对照 Clowder AI 实现（如有参考）
5. Git commit

---

## Phase 1: 数据层

### Task 1.1: 数据库迁移脚本

**目标**: 创建自由协作所需的数据库表和字段

**文件**: `isdp/sql-change/migrations/202603310001_add_free_discussion_fields.sql`

**内容**:
```sql
-- threads 表扩展
ALTER TABLE threads ADD COLUMN type VARCHAR(20) DEFAULT 'workflow';
ALTER TABLE threads ADD COLUMN available_agents JSON DEFAULT NULL;

-- workflow_templates 表扩展  
ALTER TABLE workflow_templates ADD COLUMN mode VARCHAR(20) DEFAULT 'workflow';

-- multi_mention_requests 表
CREATE TABLE multi_mention_requests (
  id UUID PRIMARY KEY,
  thread_id UUID NOT NULL,
  initiator VARCHAR(100) NOT NULL,
  callback_to VARCHAR(100) NOT NULL,
  targets JSON NOT NULL,
  question TEXT NOT NULL,
  context TEXT,
  status VARCHAR(20) DEFAULT 'pending',
  timeout_minutes INT DEFAULT 8,
  search_evidence JSON,
  override_reason TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (thread_id) REFERENCES threads(id)
);

-- multi_mention_responses 表
CREATE TABLE multi_mention_responses (
  id UUID PRIMARY KEY,
  request_id UUID NOT NULL,
  agent_id VARCHAR(100) NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (request_id) REFERENCES multi_mention_requests(id)
);

-- 紧急索引
CREATE INDEX idx_mm_requests_thread ON multi_mention_requests(thread_id);
CREATE INDEX idx_mm_requests_status ON multi_mention_requests(status);
CREATE INDEX idx_mm_responses_request ON multi_mention_responses(request_id);
```

**验证 Checkpoint**:
- [ ] 脚本语法正确
- [ ] 执行迁移成功
- [ ] 字段默认值正确（type='workflow', mode='workflow'）
- [ ] 外键约束生效
- [ ] 原有数据未受影响

**Clowder AI 参考**: 无对应表（ISDP 新设计）

---

### Task 1.2: Go 数据模型定义

**目标**: 定义 Go 结构体映射数据库表

**文件**:
- `internal/model/thread.go` - 扩展（已有基础修改）
- `internal/model/workflow_template.go` - 扩展（已有基础修改）
- `internal/model/multi_mention.go` - 新增

**新增内容** (`internal/model/multi_mention.go`):
```go
package model

import (
    "time"
    "github.com/google/uuid"
)

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
    ID             uuid.UUID          `json:"id"`
    ThreadID       uuid.UUID          `json:"threadId"`
    Initiator      string             `json:"initiator"`
    CallbackTo     string             `json:"callbackTo"`
    Targets        []string           `json:"targets"`
    Question       string             `json:"question"`
    Context        string             `json:"context,omitempty"`
    Status         MultiMentionStatus `json:"status"`
    TimeoutMinutes int                `json:"timeoutMinutes"`
    SearchEvidence []string           `json:"searchEvidence,omitempty"`
    OverrideReason string             `json:"overrideReason,omitempty"`
    CreatedAt      time.Time          `json:"createdAt"`
}

type MultiMentionResponse struct {
    ID        uuid.UUID `json:"id"`
    RequestID uuid.UUID `json:"requestId"`
    AgentID   string    `json:"agentId"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"createdAt"`
}
```

**验证 Checkpoint**:
- [ ] 结构体编译通过
- [ ] JSON tag 使用 camelCase（符合 CLAUDE.md 规范）
- [ ] 字段与数据库列对应
- [ ] 常量枚举完整

**Clowder AI 参考**: `packages/api/src/db/schema.ts`

---

### Task 1.3: 数据仓库层

**目标**: 实现 MultiMention 的 CRUD 操作

**文件**: `internal/repo/multi_mention_repo.go` - 新增

**内容**:
```go
package repo

import (
    "context"
    "github.com/google/uuid"
    "isdp/internal/model"
)

type MultiMentionRepo interface {
    CreateRequest(ctx context.Context, req *model.MultiMentionRequest) error
    GetRequestByID(ctx context.Context, id uuid.UUID) (*model.MultiMentionRequest, error)
    UpdateRequestStatus(ctx context.Context, id uuid.UUID, status model.MultiMentionStatus) error
    CreateResponse(ctx context.Context, resp *model.MultiMentionResponse) error
    GetResponsesByRequestID(ctx context.Context, requestID uuid.UUID) ([]*model.MultiMentionResponse, error)
    GetActiveRequestsByThread(ctx context.Context, threadID uuid.UUID) ([]*model.MultiMentionRequest, error)
}

type multiMentionRepo struct {
    db *sql.DB
}

// 实现 CRUD 方法...
```

**验证 Checkpoint**:
- [ ] 接口定义完整
- [ ] CRUD 方法实现正确
- [ ] 事务处理正确
- [ ] 单元测试通过（mock DB）

**Clowder AI 参考**: ISDP 现有 repo 模式（如 `agent_config_repo.go`）

---

## Phase 2: 核心服务层

### Task 2.1: MultiMentionOrchestrator

**目标**: 实现状态机管理和多讨论编排

**文件**: `internal/service/a2a/multi_mention_orchestrator.go` - 新增

**核心职责**:
1. 创建请求（参数校验：targets 数量、证据规则）
2. 状态流转（pending → running → done/partial/timeout/failed）
3. 响应记录与聚合
4. 超时处理
5. 级联防护（activeTargets 维护）

**关键方法**:
```go
type MultiMentionOrchestrator struct {
    repo           repo.MultiMentionRepo
    activeTargets  map[uuid.UUID][]string // threadID -> agentIDs
    timeoutManager *TimeoutManager
}

func (o *MultiMentionOrchestrator) Create(ctx context.Context, params CreateParams) (*CreateResult, error)
func (o *MultiMentionOrchestrator) Start(ctx context.Context, requestID uuid.UUID) error
func (o *MultiMentionOrchestrator) RecordResponse(ctx context.Context, requestID uuid.UUID, agentID string, content string) error
func (o *MultiMentionOrchestrator) HandleTimeout(ctx context.Context, requestID uuid.UUID) error
func (o *MultiMentionOrchestrator) GetResult(ctx context.Context, requestID uuid.UUID) (*AggregatedResult, error)
func (o *MultiMentionOrchestrator) IsActiveTarget(threadID uuid.UUID, agentID string) bool
```

**参数校验规则**:
- `targets`: 1-3 个，必须在 availableAgents 范围内
- `searchEvidenceRefs`: 必须提供，除非有 `overrideReason`
- `callbackTo`: 必须在 availableAgents 范围内
- 级联防护：调用者不能在 activeTargets 中

**验证 Checkpoint**:
- [ ] 状态流转完整（7 种状态）
- [ ] 参数校验正确（targets 数量、证据规则）
- [ ] 级联防护有效
- [ ] 超时机制正确
- [ ] 聚合逻辑正确
- [ ] 单元测试覆盖率 > 90%

**Clowder AI 参考**: `packages/api/src/routes/callback-multi-mention-routes.ts`
- 重点对照：状态定义、超时处理、"先搜后问"规则

---

### Task 2.2: TeammateRosterBuilder

**目标**: 构建当前 Thread 可用的队友列表

**文件**: `internal/service/agent/teammate_roster_builder.go` - 新增

**内容**:
```go
type TeammateRosterBuilder struct {
    threadRepo     repo.ThreadRepo
    templateRepo   repo.WorkflowTemplateRepo
    agentRepo      repo.AgentConfigRepo
}

type TeammateInfo struct {
    ID     string   `json:"id"`
    Name   string   `json:"name"`
    Role   string   `json:"role"`
    Skills []string `json:"skills"`
}

func (b *TeammateRosterBuilder) Build(ctx context.Context, threadID uuid.UUID) ([]TeammateInfo, error)
```

**逻辑**:
- 自由模式（Thread.type=free_discussion）：返回 Thread.availableAgents
- 工作流模式：通过 Thread.workflowTemplateId 查询 WorkflowTemplate.agentIds

**验证 Checkpoint**:
- [ ] 自由模式返回 availableAgents
- [ ] 工作流模式返回 template.agentIds
- [ ] Agent 信息完整（ID、Name、Role、Skills）
- [ ] 空列表处理正确
- [ ] 单元测试通过

**Clowder AI 参考**: `packages/api/src/routes/callback-teammate-roster-routes.ts`

---

### Task 2.3: ContextBuilder 扩展

**目标**: 在系统提示词中注入协作信息

**文件**: `internal/service/agent/context_builder.go` - 修改

**新增方法**:
```go
func (b *ContextBuilder) BuildTeammateRosterPrompt(availableAgents []TeammateInfo) string
func (b *ContextBuilder) BuildGovernancePrompt() string
```

**注入位置**: Layer0 系统提示词

**内容**:
- 队友名册："当前团队包含：架构师（擅长架构设计）、前端开发（擅长 React）..."
- 协作规则："先搜后问原则"、"级联防护规则"
- MCP 工具说明：isdp_multi_mention、isdp_get_teammate_roster

**验证 Checkpoint**:
- [ ] 提示词格式正确
- [ ] 注入位置正确（Layer0）
- [ ] 内容包含队友信息 + 协作规则
- [ ] 单元测试通过

**Clowder AI 参考**: `packages/api/src/services/SystemPromptBuilder.ts`

---

### Task 2.4: Agent 执行集成

**目标**: 将 multi_mention 集成到 Agent 执行流程

**文件**: 
- `internal/service/agent/execution_service.go` - 修改
- `internal/service/agent/orchestrator.go` - 修改

**改动点**:
1. ExecutionService 在执行前检查 Agent 是否在 activeTargets（级联防护）
2. Orchestrator 处理 multi_mention 任务分发
3. 状态 done 时触发 callbackTo Agent

**分发逻辑**:
```go
// 在 checkRouting() 中增加 multi_mention 处理
if mmRequest.Status == MultiMentionStatusDone {
    // 触发 callbackTo Agent
    o.EnqueueA2ATargets(ctx, threadID, []string{mmRequest.CallbackTo}, aggregatedContent)
}
```

**验证 Checkpoint**:
- [ ] 分发逻辑正确
- [ ] 回调触发正确
- [ ] 级联防护检查点正确
- [ ] 与现有 A2A 机制兼容

**Clowder AI 参考**: ISDP 现有 A2A 触发机制

---

## Phase 3: API 层

### Task 3.1: Multi-Mention API Handler

**目标**: 提供 multi_mention REST API

**文件**: `internal/api/handlers/callback_multi_mention.go` - 新增

**API 端点**:
- `POST /api/v1/callbacks/multi-mention` - 创建请求
- `GET /api/v1/callbacks/multi-mention-status?id=<invocationId>` - 查询状态

**请求/响应格式**（见设计文档第五部分）

**验证 Checkpoint**:
- [ ] 参数校验正确
- [ ] 调用 Orchestrator 正确
- [ ] 响应格式正确（camelCase）
- [ ] 错误处理完整（400/404/500）
- [ ] API 集成测试通过

**Clowder AI 参考**: `packages/api/src/routes/callback-multi-mention-routes.ts`

---

### Task 3.2: Teammate Roster API Handler

**目标**: 提供队友名册 REST API

**文件**: `internal/api/handlers/callback_teammate_roster.go` - 新增

**API 端点**:
- `GET /api/v1/callbacks/teammate-roster?threadId=<threadId>`

**验证 Checkpoint**:
- [ ] 调用 TeammateRosterBuilder 正确
- [ ] 响应格式正确
- [ ] 错误处理完整
- [ ] API 成测试通过

**Clowder AI 参考**: `packages/api/src/routes/callback-teammate-roster-routes.ts`

---

### Task 3.3: Thread API 扩展

**目标**: 支持创建自由讨论会话

**文件**: `internal/api/handlers/thread.go` - 修改

**改动**:
- Create Thread 接受 `type` 参数
- Create Thread 接受 `availableAgents` 参数
- 返回数据包含新字段

**验证 Checkpoint**:
- [ ] 参数处理正确
- [ ] 默认值正确（type='workflow'）
- [ ] 向后兼容（不传 type 时行为不变）
- [ ] API 测试通过

**Clowder AI 参考**: ISDP 现有 Thread API

---

## Phase 4: MCP 工具层

### Task 4.1: MCP 工具定义

**目标**: 定义 MCP 工具 Schema

**文件**: `internal/mcp/tools/callback_tools.go` - 新增/修改

**工具定义**:
```go
// isdp_multi_mention
{
    Name: "isdp_multi_mention",
    Description: "并行邀请 1-3 个 Agent 讨论同一问题...",
    InputSchema: {
        type: "object",
        properties: {
            targets: { type: "array", items: { type: "string" }, minItems: 1, maxItems: 3 },
            question: { type: "string" },
            callbackTo: { type: "string" },
            searchEvidenceRefs: { type: "array", items: { type: "string" } },
            overrideReason: { type: "string" },
            timeoutMinutes: { type: "integer", default: 8 }
        },
        required: ["targets", "question", "callbackTo"]
    }
}

// isdp_get_teammate_roster
{
    Name: "isdp_get_teammate_roster",
    Description: "获取当前团队可用队友列表",
    InputSchema: { type: "object" }
}
```

**验证 Checkpoint**:
- [ ] Schema 定义正确
- [ ] 参数约束正确（targets minItems/maxItems）
- [ ] Description 清晰
- [ ] 注册到 MCP server

**Clowder AI 参考**: `packages/mcp-server/src/tools/callback-tools.ts`
- 重点对照：参数名称、"先搜后问"规则说明

---

### Task 4.2: MCP 工具 Handler

**目标**: 实现 MCP 工具执行逻辑

**文件**: `internal/mcp/handlers/callback_handlers.go` - 新增/修改

**Handler 实现**:
```go
func handleMultiMention(ctx context.Context, params map[string]interface{}) (*ToolResult, error)
func handleGetTeammateRoster(ctx context.Context, params map[string]interface{}) (*ToolResult, error)
```

**逻辑**:
- handleMultiMention: 校验参数 → 检查级联防护 → 调用 Orchestrator.Create
- handleGetTeammateRoster: 获取 threadId → 调用 TeammateRosterBuilder.Build

**验证 Checkpoint**:
- [ ] 参数解析正确
- [ ] 级联防护检查正确
- [ ] 调用服务层正确
- [ ] 返回格式正确（invocationId + callbackToken）
- [ ] Handler 单元测试通过

**Clowder AI 参考**: `packages/mcp-server/src/tools/callback-tools.ts`

---

## Phase 5: Skill 包

### Task 5.1: 创建 SKILL.md 文件

**目标**: 创建自由协作技能指导文档

**文件**: `{skillStoragePath}/free-collaboration/SKILL.md` - 新增

**内容**: 见设计文档第七部分

**关键内容**:
- 协作原则（先搜后问、防止级联）
- MCP 工具使用说明
- 协作流程
- 注意事项

**验证 Checkpoint**:
- [ ] 文件路径正确
- [ ] frontmatter 正确（name、description、triggers）
- [ ] 内容完整
- [ ] 格式符合 Skill 规范

**Clowder AI 参考**: `cat-cafe-skills/collaborative-thinking/SKILL.md`

---

### Task 5.2: Skill 初始化脚本

**目标**: 初始化 Skill 记录和绑定

**文件**: `isdp/sql-change/migrations/202603310002_init_free_collaboration_skill.sql` - 新增

**内容**:
```sql
-- 插入 Skill 记录
INSERT INTO skills (id, name, description, storage_path, is_system, created_at)
VALUES (
    UUID(),
    'free-collaboration',
    '自由协作技能 - 多Agent并行讨论',
    'free-collaboration',
    1,
    NOW()
);

-- 注意：绑定逻辑需要在应用层处理
-- 当创建自由模式团队时，自动绑定此 Skill 到所有 Agent
```

**应用层绑定逻辑**（在 Team 创建 API 中）:
```go
if template.Mode == TeamModeFree {
    // 为所有 agentIds 绑定 free-collaboration Skill
    for _, agentID := range template.AgentIds {
        skillBinding := &model.AgentSkillBinding{
            AgentRoleID: agentID,
            SkillID:     freeCollaborationSkillID,
        }
        skillBindingRepo.Create(ctx, skillBinding)
    }
}
```

**验证 Checkpoint**:
- [ ] Skill 记录正确
- [ ] 绑定逻辑正确（应用层）
- [ ] configgen 复制 Skill 成功
- [ ] CLI 加载 Skill 成功

**Clowder AI 参考**: ISDP 现有 Skill 初始化逻辑

---

## Phase 6: 前端层

### Task 6.1: 自由讨论入口

**目标**: 项目详情页添加"自由讨论"按钮

**文件**: `isdp/web/src/pages/ProjectDetail/index.tsx` - 修改

**改动**:
- 新增"自由讨论"按钮（与"开始工作流"并列）
- 点击创建 type=free_discussion Thread
- 获取项目绑定的自由模式团队，填充 availableAgents
- 跳转到 ThreadView

**验证 Checkpoint**:
- [ ] 按钮 UI 正确
- [ ] API 调用正确（POST /threads with type=free_discussion）
- [ ] 跳转正确
- [ ] 前端组件测试通过

**Clowder AI 参考**: ISDP 现有 Thread 创建流程

---

### Task 6.2: ThreadView 自由模式 UI

**目标**: ThreadView 根据类型显示不同 UI

**文件**: `isdp/web/src/pages/ThreadView.tsx` - 修改

**改动**:
- 根据 thread.type 判断模式
- 自由模式显示 TeammateRoster 组件
- 自由模式隐藏工作流 Transitions UI

**验证 Checkpoint**:
- [ ] 类型判断正确
- [ ] UI 切换正确
- [ ] 组件渲染正确

**Clowder AI 参考**: ISDP 现有 ThreadView

---

### Task 6.3: TeammateRoster 组件

**目标**: 显示可 @ 队友列表

**文件**: `isdp/web/src/components/TeammateRoster/index.tsx` - 新增

**功能**:
- 显示队友列表（名称、角色、擅长）
- 点击队友快速 @提及
- 显示 MultiMentionModal 发起讨论

**验证 Checkpoint**:
- [ ] 组件渲染正确
- [ ] 数据获取正确（GET /callbacks/teammate-roster）
- [ ] 点击交互正确
- [ ] 样式符合 Ant Design

**Clowder AI 参考**: Clowder AI 前端 UI

---

### Task 6.4: MultiMentionModal 组件

**目标**: 发起多 Agent 讨论的弹窗

**文件**: `isdp/web/src/components/MultiMentionModal/index.tsx` - 新增

**功能**:
- Agent 多选（1-3 个，复选框或标签选择）
- 问题输入（TextArea）
- 搜索证据输入（TextArea）
- 提交按钮（调用 POST /callbacks/multi-mention）

**验证 Checkpoint**:
- [ ] 多选限制正确（1-3）
- [ ] 必填校验正确
- [ ] API 调用正确
- [ ] 样式符合 Ant Design

**Clowder AI 参考**: Clowder AI multi_mention UI

---

## Phase 7: 集成测试

### Task 7.1: 后端集成测试

**目标**: API 端点和数据持久化测试

**文件**: 
- `internal/api/handlers/callback_multi_mention_test.go` - 新增
- `internal/api/handlers/callback_teammate_roster_test.go` - 新增

**测试内容**:
- API 端点测试（请求/响应格式）
- 参数校验测试（targets 数量、证据规则）
- 数据持久化测试（数据库 CRUD）
- 错误处理测试（400/404/500）

**验证 Checkpoint**:
- [ ] API 测试覆盖率 > 80%
- [ ] 所有边界条件测试通过
- [ ] 向后兼容测试通过

**Clowder AI 参考**: 见设计文档测试矩阵

---

### Task 7.2: 服务单元测试

**目标**: 核心服务逻辑测试

**文件**: `internal/service/a2a/multi_mention_orchestrator_test.go` - 新增

**测试内容**:
- 状态流转测试（7 种状态）
- 边界条件测试（targets 数量、证据规则）
- 级联防护测试
- 超时测试
- 聚合测试

**验证 Checkpoint**:
- [ ] 测试覆盖率 > 90%
- [ ] 所有状态流转测试通过
- [ ] 边界条件全覆盖

**Clowder AI 参考**: 见设计文档测试矩阵

---

## Phase 8: E2E 测试

### Task 8.1: E2E 完整流程测试

**目标**: 完整业务流程闭环验证

**文件**: `isdp/tests/e2e/free_discussion_test.go` - 新增

**测试场景**:
1. 创建自由讨论会话 → Thread.type=free_discussion
2. 发起 multi_mention → targets 收到任务
3. 响应聚合 → callbackTo 收到聚合结果
4. 级联防护 → 被召唤 Agent 无法再次发起
5. 超时处理 → 部分响应仍聚合
6. Skill 加载 → CLI 正确加载 SKILL.md

**验证 Checkpoint**:
- [ ] 所有 E2E scenario 通过
- [ ] 完整流程闭环验证
- [ ] Skill 加载闭环验证

**Clowder AI 参考**: 见设计文档 E2E 测试

---

## 执行顺序总结

```
Phase 1: 数据层 (Task 1.1 → 1.2 → 1.3)
    ↓
Phase 2: 服务层 (Task 2.1 → 2.2 → 2.3 → 2.4)
    ↓
Phase 3: API层 (Task 3.1 → 3.2 → 3.3)
    ↓
Phase 4: MCP工具 (Task 4.1 → 4.2)
    ↓
Phase 5: Skill包 (Task 5.1 → 5.2) [可并行于 Phase 1-2]
    ↓
Phase 6: 前端 (Task 6.1 → 6.2 → 6.3 → 6.4)
    ↓
Phase 7: 集成测试 (Task 7.1 → 7.2)
    ↓
Phase 8: E2E测试 (Task 8.1)
```

**并行机会**:
- Task 5.1（SKILL.md）可独立于 Phase 1-2 并行执行
- Task 4.1（MCP 定义）可与 Phase 2 并行

---

## Clowder AI 对照检查表

| Task | Clowder AI 参考文件 | 对照要点 |
|------|---------------------|----------|
| 2.1 | `callback-multi-mention-routes.ts` | 状态定义、超时处理、"先搜后问" |
| 2.2 | `callback-teammate-roster-routes.ts` | 返回格式、Agent 信息字段 |
| 2.3 | `SystemPromptBuilder.ts` | 提示词格式、注入位置 |
| 4.1 | `callback-tools.ts` | 参数 Schema、工具名称 |
| 4.2 | `callback-tools.ts` | Handler 逻辑、错误处理 |
| 5.1 | `collaborative-thinking/SKILL.md` | frontmatter、协作原则 |

---

## 验收标准（发布前必须通过）

参见设计文档第九部分验收检查清单。

---

**计划版本**: v1.0
**创建日期**: 2026-03-31
**设计文档**: `docs/superpowers/specs/2026-03-31-free-collaboration-design.md`