# 开发变更记录

本文件记录项目的开发变更历史，用于后期复盘和追溯。

---

## 2026-03-16 A2A @mention 路由验证功能

### 背景

当前 A2A @mention 路由存在以下问题：

1. **路由验证逻辑未生效**：`ValidateRouting` 和 `CanRouteTo` 配置存在但未被调用
2. **路由范围不受控**：Agent 可以 @mention 任意角色，不受工作流模板限制
3. **重复代码**：`getAllowedRoutes` 和 `getDefaultRouting` 定义了相同的路由规则

### 目标

修复 @mention 路由验证逻辑，使 Agent 只能路由到工作流模板中已配置的 Agent 实例。支持两种格式：
- `@role`（角色别名，如 `@developer`）
- `@agent-name`（实例名称，如 `@前端开发`）

### 设计文档

`isdp/docs/superpowers/specs/2026-03-16-a2a-mention-routing-validation-design.md`

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/service/agent/orchestrator.go` | 修改 | 修改 `parseMentions`、`checkRouting`，新增辅助函数 |
| `internal/service/a2a/mention_parser.go` | 删除 | 整个文件可删除，代码未被使用 |

### 详细改动

#### 1. 新增数据结构

**File:** `internal/service/agent/orchestrator.go`

```go
// ParsedMention @mention 解析结果
type ParsedMention struct {
    Role      model.AgentRole // 角色类型（可能为空）
    AgentName string          // Agent 实例名称（可能为空）
    Raw       string          // 原始 mention 文本
}
```

#### 2. 修改 parseMentions 函数

**改动:** 返回类型从 `[]model.AgentRole` 改为 `[]ParsedMention`

```go
// 修改前
func parseMentions(content string) []model.AgentRole

// 修改后
func (o *Orchestrator) parseMentions(content string) []ParsedMention
```

#### 3. 修改 checkRouting 函数

**核心逻辑变更:**

```go
// 修改前：直接按 role 触发，无验证
for _, role := range mentions {
    if role != "" {
        o.SpawnAgent(ctx, &SpawnRequest{
            ThreadID: threadID,
            Role:     role,
            Input:    output,
        })
    }
}

// 修改后：验证目标是否在工作流模板中
allowedAgents := o.getAllowedAgentsFromWorkflow(ctx, threadID)
for _, mention := range mentions {
    var targetConfig *model.AgentRoleConfig
    if mention.Role != "" {
        targetConfig = o.findAgentByRole(allowedAgents, mention.Role)
    } else {
        targetConfig = o.findAgentByName(allowedAgents, mention.AgentName)
    }
    if targetConfig == nil {
        logInfo("路由被拒绝：目标不在工作流模板中", ...)
        continue
    }
    o.SpawnAgent(ctx, &SpawnRequest{
        ThreadID: threadID,
        ConfigID: targetConfig.ID,
        Role:     targetConfig.Role,
        Input:    output,
    })
}
```

#### 4. 新增辅助函数

```go
// getAllowedAgentsFromWorkflow 从工作流模板获取允许路由的 Agent 列表
// 数据流: Thread → WorkflowTemplate → AgentIDs → AgentConfigs
func (o *Orchestrator) getAllowedAgentsFromWorkflow(ctx context.Context, threadID uuid.UUID) []*model.AgentRoleConfig

// findAgentByRole 在 Agent 列表中按角色查找
func (o *Orchestrator) findAgentByRole(agents []*model.AgentRoleConfig, role model.AgentRole) *model.AgentRoleConfig

// findAgentByName 在 Agent 列表中按名称查找
func (o *Orchestrator) findAgentByName(agents []*model.AgentRoleConfig, name string) *model.AgentRoleConfig

// checkSignalRouting 检查信号路由（原有逻辑提取）
func (o *Orchestrator) checkSignalRouting(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string)
```

#### 5. 删除未使用代码

**File:** `internal/service/a2a/mention_parser.go` - 整个文件删除

删除内容：
- `MentionParser` 结构体和 `NewMentionParser` 函数
- `ParsedMention` 结构体（旧版，字段为 `{Role, Content}`）
- `ParseMentions` 方法
- `ParseAgentRole` 函数（与 `orchestrator.go` 中的重复）
- `ExtractRouting` 方法和 `RoutingInfo` 结构体
- `ValidateRouting` 方法
- `getAllowedRoutes` 函数（与 `config_service.go` 中的 `getDefaultRouting` 重复）
- `FormatMention` 函数和 `roleToString` 函数

### 数据流

```
Agent 执行完成 → checkRouting() → parseMentions(output)
    ↓
getAllowedAgentsFromWorkflow(threadID)
    → threadRepo.FindByID(threadID)
    → workflowRepo.FindByID(templateID)
    → configSvc.GetByID() for each agent ID
    ↓
匹配 @mention 与 allowedAgents
    → findAgentByRole() 或 findAgentByName()
    ↓
SpawnAgent(ConfigID: targetConfig.ID)
```

### 边界情况处理

| 场景 | 处理方式 |
|------|----------|
| Thread 未绑定工作流模板 | 返回 nil，所有 @mention 被记录为"路由被拒绝"并跳过 |
| @mention 角色不在模板中 | 记录日志"路由被拒绝：目标不在工作流模板中"，跳过 |
| @mention 名称不在模板中 | 记录日志"路由被拒绝：目标不在工作流模板中"，跳过 |
| Agent 配置被删除 | `GetByID` 失败，跳过该 Agent |
| 工作流模板 AgentIDs 为空 | 返回 nil，所有 @mention 被记录为"路由被拒绝"并跳过 |

### 回退方法

如需回退此功能：

1. 恢复 `orchestrator.go` 中的 `parseMentions` 函数为返回 `[]model.AgentRole`
2. 恢复 `checkRouting` 函数为原来的直接触发逻辑
3. 删除新增的辅助函数：`getAllowedAgentsFromWorkflow`、`findAgentByRole`、`findAgentByName`、`checkSignalRouting`
4. 删除新增的 `ParsedMention` 结构体
5. 恢复 `a2a/mention_parser.go` 文件（如需）

### 验证方法

1. 启动服务，创建一个绑定了工作流模板的 Thread
2. 让 Agent 输出 `@前端开发 请实现登录页面`
3. 验证：
   - 如果"前端开发"在模板中 → 触发该 Agent
   - 如果"前端开发"不在模板中 → 日志记录"路由被拒绝"
4. 测试 `@developer` 等角色别名格式

### 影响范围

- 后端：`orchestrator.go` 路由逻辑
- 删除：`a2a/mention_parser.go` 未使用代码
- 数据：不影响现有数据

---

## 2026-03-15 工作流阶段配置改为Agent实例选择

### 背景

工作流页面的"阶段配置"原来选择的是阶段名称（需求分析、架构设计等），但实际应该配置的是具体的 Agent 实例。

### 目标

将"阶段配置"改为选择 Agent 实例，Agent 实例从后端 API 动态获取。

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `isdp/web/src/pages/Workflow/index.tsx` | 修改 | 主要修改文件 |

### 详细改动

#### 1. 新增导入

```tsx
import React, { useState, useEffect } from 'react';
// 新增 Spin 组件
import { ..., Spin } from 'antd';
// 新增 API 客户端
import { api } from '@/api/client';
// 新增类型导入
import type { AgentConfig } from '@/types';
import { AgentRoleLabels } from '@/types';
```

#### 2. 接口定义修改

**WorkflowTemplate 接口**：

```typescript
// 修改前
interface WorkflowTemplate {
  phases: string[];  // 阶段名称列表
  ...
}

// 修改后
interface WorkflowTemplate {
  agentIds: string[];  // Agent 实例 ID 列表
  ...
}
```

#### 3. 新增状态和 API 调用

```tsx
const [agents, setAgents] = useState<AgentConfig[]>([]);
const [loadingAgents, setLoadingAgents] = useState(false);

useEffect(() => {
  setLoadingAgents(true);
  api.agents.list()
    .then(setAgents)
    .catch((error) => {
      console.error('Failed to fetch agents:', error);
      message.error('获取Agent列表失败');
    })
    .finally(() => setLoadingAgents(false));
}, []);
```

#### 4. 删除硬编码数据

删除了静态的 `agentRoles` 数组，改为从 API 动态获取：

```tsx
// 已删除
const agentRoles = [
  { id: 'requirement', name: '需求分析师', ... },
  { id: 'architect', name: '架构师', ... },
  ...
];
```

#### 5. 模板数据结构调整

```tsx
// 修改前
const workflowTemplates = [
  {
    id: 'standard',
    phases: ['需求分析', '架构设计', '代码实现', ...],
    ...
  }
];

// 修改后
const workflowTemplates = [
  {
    id: 'standard',
    agentIds: [], // 将根据角色动态匹配
    ...
  }
];
```

#### 6. 表单字段修改

```tsx
// 修改前
<Form.Item name="phases" label="阶段配置">
  <Select mode="multiple">
    <Option value="requirement">需求分析</Option>
    ...
  </Select>
</Form.Item>

// 修改后
<Form.Item name="agentIds" label="Agent配置">
  <Select mode="multiple" loading={loadingAgents}>
    {agents.map((agent) => (
      <Option key={agent.id} value={agent.id}>
        {agent.name} ({AgentRoleLabels[agent.role]})
      </Option>
    ))}
  </Select>
</Form.Item>
```

#### 7. UI 显示更新

- 模板卡片：从显示"阶段流程"改为显示"Agent配置"
- 右侧卡片：标题从"Agent 角色"改为"Agent 实例"
- Agent 列表：从静态数据改为动态数据，增加了加载状态和空状态处理

### 数据流变化

```
修改前:
  硬编码 agentRoles → 渲染 UI

修改后:
  API /agents → agents state → 渲染 UI
```

### 验证方法

1. 启动前后端服务
2. 打开工作流页面 http://localhost:3004/workflow
3. 点击"自定义工作流"按钮
4. 验证"Agent配置"下拉列表显示从后端获取的 Agent 实例
5. 选择多个 Agent 实例并提交表单

### 影响范围

- 仅影响前端 Workflow 页面
- 不涉及后端 API 修改
- 不影响其他页面功能

### 备注

- 预设工作流模板的 `agentIds` 目前为空数组，后续可根据实际业务需求进行配置
- Agent 实例数据来自 `api.agents.list()` 接口，返回 `AgentConfig[]` 类型数据

---

## 2026-03-15 工作流模板持久化功能实现

### 背景

工作流创建后未正常保存，`handleCreateWorkflow` 函数只是打印日志，没有实际调用后端 API 保存数据。需要实现完整的工作流模板持久化功能。

### 目标

实现工作流模板的完整 CRUD 功能：
- 后端：创建模型、Repository、Service、Handler
- 前端：API 客户端、页面交互

### 新增文件

| 文件 | 说明 |
|------|------|
| `internal/model/workflow_template.go` | 工作流模板数据模型 |
| `internal/repo/workflow_template.go` | 工作流模板数据访问层 |
| `internal/service/workflow/service.go` | 工作流模板业务逻辑层 |
| `internal/api/workflow_handler.go` | 工作流模板 API 处理器 |

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `cmd/server/main.go` | 修改 | 添加 workflow 相关初始化和路由注册 |
| `web/src/api/client.ts` | 修改 | 添加 workflows API 方法 |
| `web/src/api/transform.ts` | 修改 | 添加 workflow 数据转换函数 |
| `web/src/types/index.ts` | 修改 | 添加 WorkflowTemplate 类型定义 |
| `web/src/pages/Workflow/index.tsx` | 修改 | 使用 API 实现模板的增删改查 |

### 详细改动

#### 1. 后端模型定义 (workflow_template.go)

```go
type WorkflowTemplate struct {
    ID            uuid.UUID       `json:"id"`
    Name          string          `json:"name"`
    Description   string          `json:"description"`
    AgentIDs      json.RawMessage `json:"agent_ids"`      // Agent实例ID列表
    Checkpoints   json.RawMessage `json:"checkpoints"`    // 人工检查点列表
    EstimatedTime string          `json:"estimated_time"`
    IsSystem      bool            `json:"is_system"`      // 是否系统预设
    CreatedAt     time.Time       `json:"created_at"`
    UpdatedAt     time.Time       `json:"updated_at"`
}
```

#### 2. 数据库表结构 (main.go)

```sql
CREATE TABLE IF NOT EXISTS workflow_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    agent_ids TEXT DEFAULT '[]',
    checkpoints TEXT DEFAULT '[]',
    estimated_time TEXT,
    is_system INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### 3. API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/workflows` | 获取所有工作流模板 |
| POST | `/api/v1/workflows` | 创建工作流模板 |
| GET | `/api/v1/workflows/:id` | 获取单个工作流模板 |
| PUT | `/api/v1/workflows/:id` | 更新工作流模板 |
| DELETE | `/api/v1/workflows/:id` | 删除工作流模板（仅非系统模板） |

#### 4. 前端 API 客户端 (client.ts)

```typescript
workflows = {
  list: (): Promise<WorkflowTemplate[]> => this.request('/workflows', 'GET'),
  get: (id: string): Promise<WorkflowTemplate> => this.request(`/workflows/${id}`, 'GET'),
  create: (data: Partial<WorkflowTemplate>): Promise<WorkflowTemplate> =>
    this.request('/workflows', 'POST', data),
  update: (id: string, data: Partial<WorkflowTemplate>): Promise<WorkflowTemplate> =>
    this.request(`/workflows/${id}`, 'PUT', data),
  delete: (id: string): Promise<void> => this.request(`/workflows/${id}`, 'DELETE'),
};
```

#### 5. 前端类型定义 (types/index.ts)

```typescript
export interface WorkflowTemplate {
  id: string;
  name: string;
  description: string;
  agentIds: string[];
  checkpoints: string[];
  estimatedTime: string;
  isSystem: boolean;
  createdAt: string;
  updatedAt: string;
}
```

#### 6. 页面交互改进

- 从 API 获取工作流模板列表，替代硬编码数据
- 创建工作流时调用 `api.workflows.create()` 保存到后端
- 添加删除功能，支持删除非系统预设模板
- 添加加载状态和提交状态显示
- 系统预设模板显示"系统预设"标签，不可删除

### 系统预设模板

服务启动时自动初始化 4 个系统预设模板：

1. **标准开发流程** - 完整的软件开发流程，从需求到部署
2. **快速原型流程** - 快速构建原型，验证想法
3. **代码重构流程** - 优化现有代码结构和质量
4. **问题修复流程** - 快速定位和修复问题

### 数据流

```
创建工作流:
  前端表单 → api.workflows.create() → 后端 Handler → Service → Repository → SQLite

获取工作流列表:
  页面加载 → api.workflows.list() → 后端 Handler → Service → Repository → 返回数据

删除工作流:
  点击删除 → Popconfirm确认 → api.workflows.delete() → 后端 Handler → Service → Repository
```

### 验证方法

1. 启动后端服务：`go run ./cmd/server`
2. 启动前端服务：`cd web && npm run dev`
3. 打开工作流页面 http://localhost:3004/workflow
4. 验证页面显示系统预设的 4 个工作流模板
5. 点击"自定义工作流"，填写表单并提交
6. 验证新创建的工作流模板出现在列表中
7. 刷新页面，验证数据持久化成功
8. 测试删除功能，验证非系统模板可删除

### 影响范围

- 后端：新增工作流模板相关的完整 CRUD 功能
- 前端：Workflow 页面实现完整的增删改查交互
- 数据库：新增 `workflow_templates` 表

### 备注

- 系统预设模板（`is_system = true`）不可删除
- 删除操作有二次确认（Popconfirm）
- 表单提交有防重复提交保护（`submitting` 状态）

---

## 2026-03-15 工作流模板功能Bug修复

### 背景

工作流编排页面打开报错，创建自定义工作流也报错。经排查发现以下问题：

1. **JSON字段存储问题**：`json.RawMessage` 类型未正确存储到 SQLite
2. **空值处理问题**：前端 transform 函数未正确处理 `null` 值
3. **布尔值转换问题**：SQLite 存储 `is_system` 为 INTEGER (0/1)，但前端期望布尔值
4. **系统模板重复初始化**：服务重启时可能重复创建系统预设模板

### 目标

修复以上问题，确保工作流模板功能正常运行。

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/repo/workflow_template.go` | 修复 | JSON字段存储转换为 `[]byte` |
| `internal/service/workflow/service.go` | 修复 | 添加系统模板存在性检查 |
| `web/src/api/transform.ts` | 修复 | 增强空值处理和布尔值转换 |

### 详细改动

#### 1. Repository JSON字段存储修复

**问题**：`json.RawMessage` 直接传递给 SQL Exec 时存储失败

**修复**：转换为 `[]byte` 后存储

```go
// 修改前
_, err := r.db.ExecContext(ctx, query,
    template.AgentIDs,    // json.RawMessage
    template.Checkpoints, // json.RawMessage
    ...
)

// 修改后
_, err := r.db.ExecContext(ctx, query,
    []byte(template.AgentIDs),      // 转换为 []byte
    []byte(template.Checkpoints),   // 转换为 []byte
    ...
)
```

#### 2. Service 系统模板初始化修复

**问题**：服务重启时重复创建系统预设模板

**修复**：初始化前检查是否已存在系统模板

```go
func (s *Service) InitSystemTemplates(ctx context.Context) error {
    // 先检查是否已有系统模板
    existingTemplates, err := s.repo.FindAll(ctx)
    if err != nil {
        return err
    }

    // 如果已有系统模板，跳过初始化
    for _, t := range existingTemplates {
        if t.IsSystem {
            return nil
        }
    }

    // 创建系统模板...
}
```

#### 3. 前端 Transform 函数修复

**问题**：
- `agentIds` 和 `checkpoints` 可能为 `null`，导致前端解析失败
- `isSystem` 从后端返回为数字 `0/1`，前端期望布尔值

**修复**：增强空值处理和类型转换

```typescript
export function transformWorkflowTemplate(data: any): any {
  if (!data) return data;
  const result = snakeToCamel(data);

  // 确保 agentIds 是数组
  if (result.agentIds == null) {
    result.agentIds = [];
  } else if (typeof result.agentIds === 'string') {
    try {
      result.agentIds = JSON.parse(result.agentIds);
    } catch {
      result.agentIds = [];
    }
  }

  // 确保 checkpoints 是数组
  if (result.checkpoints == null) {
    result.checkpoints = [];
  } else if (typeof result.checkpoints === 'string') {
    try {
      result.checkpoints = JSON.parse(result.checkpoints);
    } catch {
      result.checkpoints = [];
    }
  }

  // 确保 isSystem 是布尔值
  if (typeof result.isSystem === 'number') {
    result.isSystem = result.isSystem === 1;
  }

  return result;
}
```

### 修复前后对比

| 问题 | 修复前 | 修复后 |
|------|--------|--------|
| 创建工作流 | 报错，数据未保存 | 正常创建并保存 |
| 页面加载 | 报错，无法显示模板 | 正常显示所有模板 |
| 服务重启 | 可能产生重复模板 | 跳过已存在的模板 |
| 系统模板标识 | 显示为数字 | 正确显示为布尔值 |

### 验证方法

1. 重新编译后端：`go build -o bin/server.exe ./cmd/server`
2. 重启后端服务
3. 重启前端服务：`cd web && npm run dev`
4. 打开工作流页面，验证无报错
5. 创建自定义工作流，验证保存成功
6. 刷新页面，验证数据持久化

### 影响范围

- 后端：JSON字段存储逻辑
- 前端：数据转换逻辑
- 数据：现有数据不受影响

---