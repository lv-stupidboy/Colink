# 开发变更记录

本文件记录项目的开发变更历史，用于后期复盘和追溯。

---

## 2026-03-19 项目清理与前端布局优化

### 背景

项目完成了从 SQLite 到 MySQL 的数据库迁移，需要清理迁移相关的临时文件。同时前端布局存在高度溢出问题，页面级滚动导致交互体验不佳。

### 目标

1. 清理数据库迁移相关的临时文件和脚本
2. 重构前端布局，解决 100vh 溢出问题
3. 优化前端样式，提取内联样式到 CSS 类
4. 修复潜在的空值引用错误

### 核心变更

#### 后端清理

##### 删除迁移相关文件
- 移除 `cmd/migrate/main.go` - SQLite 到 MySQL 数据迁移工具
- 移除 `isdp/DB_MIGRATION_GUIDE.md` 和 `isdp/docs/DB_MIGRATION_GUIDE.md` - 数据库迁移指南
- 删除 `scripts/` 目录下所有 SQL 脚本：
  - `init_db.sql`, `init_db_mysql.sql`, `init_db_sqlite.sql`
  - `initial_schema.sql`
  - `migrate.sh`, `schema.sh`
  - `202403160001_remove_model_name_field.sql`
  - `202403170003_remove_is_active_field.sql`
- 删除 `test-results/.last-run.json` - 测试结果缓存

##### 配置文件调整
- 删除旧配置 `configs/config.yaml`
- 新增 `configs/config.yaml` - 新版配置文件
- 新增 `configs/config.yaml.example` - 配置示例

##### SQL 变更目录
- 新增 `sql-change/` 目录，包含：
  - `README.md` - SQL 变更说明
  - `init_db_mysql.sql` - MySQL 初始化脚本
  - `migrations/` - 迁移脚本目录

#### 后端依赖升级

##### go.mod 更新
- 升级 SQLite 驱动：`modernc.org/sqlite` 1.29.10 → 1.47.0
- 新增 `github.com/mattn/go-sqlite3` 依赖
- 升级相关依赖版本（`golang.org/x/exp`, `golang.org/x/sys`, `modernc.org/*`）

##### MySQL 字符集修复
- 在 `db_mysql.go` 中添加 `SET NAMES utf8mb4`，确保中文字符正确存储和读取

#### 前端布局重构

##### 全局高度修复 (index.css)
```css
/* 修改前 */
#root {
  min-height: 100vh;
}

/* 修改后 */
html, body {
  height: 100%;
  margin: 0;
  padding: 0;
  overflow: hidden;
}
#root {
  height: 100%;
}
```

##### 主布局 Flex 改造 (MainLayout.tsx)
```tsx
// 修改前：整体页面滚动，内容区域有 margin
<Layout style={{ minHeight: '100vh' }}>
  <Content style={{ margin: 16, borderRadius: 8, padding: 24 }}>

// 修改后：分区独立滚动，Header 固定，Content 自适应
<Layout style={{ height: '100vh', overflow: 'hidden' }}>
  <Sider style={{ height: '100vh', overflow: 'auto' }} />
  <Layout style={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
    <Header style={{ flexShrink: 0 }} />
    <Content style={{ flex: 1, margin: 0, padding: 16, overflow: 'auto' }} />
  </Layout>
</Layout>
```

#### 前端样式优化

##### SandboxPanel 组件重构
- 将内联样式提取为 CSS 类：
  - `.sandbox-control-bar` - 控制栏样式
  - `.sandbox-preview-bar` - 预览栏样式
  - `.sandbox-iframe-container` - iframe 容器样式
  - `.sandbox-empty` - 空状态样式
- 简化 `ThreadView.tsx` 中 @mention 列表项的交互，移除内联事件处理

##### CSS 文件更新
- `SandboxPanel.css` - 新增沙箱面板样式
- `ThreadView.css` - 新增 `.mention-list-item` 悬停样式
- `FileTree.css` - 文件树样式优化
- `MessageInput.css` - 消息输入组件样式优化

#### 前端 Bug 修复

##### Workflow 页面空值保护
```tsx
// 修改前：可能因 undefined 导致渲染错误
const templateAgents = agents.filter(a => template.agentIds?.includes(a.id));

// 修改后：添加空值默认值
const templateAgents = (agents || []).filter(a => template.agentIds?.includes(a.id));
```

涉及数组：`agents`, `workflowTemplates`，所有 `.map()` 和 `.filter()` 调用都添加了 `|| []` 保护。

### 修改文件统计

| 类型 | 数量 | 说明 |
|------|------|------|
| 删除文件 | 13 | 迁移工具、SQL脚本、旧配置 |
| 新增目录 | 2 | `configs/`, `sql-change/` |
| 修改文件 | 10 | 依赖升级、布局优化、样式重构 |

### 详细文件列表

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `isdp/cmd/migrate/main.go` | 删除 | 迁移工具 |
| `isdp/DB_MIGRATION_GUIDE.md` | 删除 | 迁移指南 |
| `isdp/docs/DB_MIGRATION_GUIDE.md` | 删除 | 迁移指南（重复） |
| `isdp/configs/config.yaml` | 删除 | 旧配置文件 |
| `isdp/scripts/*` | 删除 | 所有 SQL 脚本和 shell 脚本 |
| `isdp/test-results/.last-run.json` | 删除 | 测试缓存 |
| `isdp/go.mod` / `go.sum` | 修改 | 依赖升级 |
| `isdp/internal/repo/db_mysql.go` | 修改 | 字符集设置 |
| `isdp/web/src/index.css` | 修改 | 全局高度修复 |
| `isdp/web/src/layouts/MainLayout.tsx` | 修改 | Flex 布局改造 |
| `isdp/web/src/pages/ThreadView.tsx` | 修改 | 移除内联事件 |
| `isdp/web/src/pages/Workflow/index.tsx` | 修改 | 空值保护 |
| `isdp/web/src/components/thread/SandboxPanel.tsx` | 修改 | 样式类提取 |
| `isdp/web/src/components/thread/SandboxPanel.css` | 修改 | 新增样式 |
| `isdp/web/src/pages/ThreadView.css` | 修改 | mention 样式 |
| `isdp/web/src/components/FileTree/FileTree.css` | 修改 | 样式优化 |
| `isdp/web/src/components/thread/MessageInput.css` | 修改 | 样式优化 |

### 验证方法

1. 启动后端服务：`go run ./cmd/server`
2. 启动前端服务：`cd web && npm run dev`
3. 验证页面布局：
   - 整体页面不滚动
   - 侧边栏和内容区域独立滚动
   - Header 固定高度
4. 验证中文存储：创建/编辑包含中文的内容，刷新页面检查显示
5. 验证 Workflow 页面：无报错，正常显示 Agent 列表和模板列表

### 影响范围

- 后端：依赖版本、MySQL 字符集
- 前端：整体布局、样式结构
- 数据：不影响现有数据
- 配置：清理旧配置，新增示例配置

---

## 2026-03-18 Agent调试功能重构与状态管理优化

### 背景

Agent调试功能需要进行重构，将调试模式与工作流模式分离，并优化状态管理和线程安全。同时清理了冗余代码，统一了日志管理。

### 目标

1. 实现调试模式与工作流模式的独立状态管理
2. 增强调试线程管理的线程安全性
3. 重构WebSocket连接，简化接口并添加自动重连
4. 清理冗余代码，统一日志工具

### 核心变更

#### 后端改动

##### Orchestrator 调试功能增强
- 新增 `SpawnDebugAgent` 方法 - 调试模式启动Agent
- 新增 `ContinueDebugAgent` 方法 - 继续调试会话
- 新增 `SetDebugThreadManager` 方法 - 注入调试线程管理器

##### DebugThreadManager 线程安全增强
- 新增状态常量：`DebugThreadStatusIdle`, `DebugThreadStatusRunning`, `DebugThreadStatusCompleted`, `DebugThreadStatusError`
- 新增 `CompareAndSwapStatus` 方法 - 原子状态比较交换
- 新增 `TryStartExecution` 方法 - 原子启动执行
- 新增 `GetProjectPath` 方法 - 获取工作目录
- 新增 `ProjectPath` 字段 - 存储工作目录路径
- 使用 `sync.Once` 保护 `Stop()` 方法，防止多次调用 panic
- `GetMessages` 返回副本，避免外部修改影响内部状态

##### 日志工具统一
- 新增 `internal/service/agent/logger.go` - 统一的日志辅助函数
- 提供 `logInfo`, `logError`, `logDebug`, `logWarn` 等函数

#### 前端改动

##### 新增调试状态管理
- 新增 `web/src/store/debugThread.ts` - 调试模式专用 Zustand store
- 独立管理：threadId, status, messages, streamingContent, sandboxServer 等
- 与工作流模式状态完全隔离

##### WebSocket Hook 重构
- 简化接口签名：`useWebSocket(threadId, options)`
- 添加自动重连机制（默认 3 秒间隔）
- 添加 `onConnect`, `onDisconnect` 回调
- 新增 `disconnect` 方法用于主动断开

##### ThreadView 页面重构
- 支持调试模式和工作流模式分离
- 根据 URL 中的 `agentId` 参数判断模式
- 调试模式使用本地 WebSocket 状态
- 工作流模式使用全局 store 状态
- 新增沙箱侧边栏支持
- 新增 `ThreadView.css` 样式文件

##### 类型定义扩展
- 新增 `WSMessage`, `WSMessageDebug`, `WSMessageType` 类型
- 新增 `AgentOutputChunk`, `AgentMessage`, `SystemMessage` 类型
- 新增 `SandboxServer`, `SandboxReady` 类型

#### 删除的文件

| 文件 | 说明 |
|------|------|
| `internal/service/agent/session_manager.go` | 会话管理器，功能已整合到其他模块 |
| `internal/service/agent/interactive_session.go` | 交互式会话，功能已整合到其他模块 |
| `internal/service/agent/execution_context.go` | 部分代码删除，剩余整合到 execution_service |
| `cmd/test/test_opencode.go` | 测试文件移除 |
| `isdp/docs/*` | 设计文档移动到项目根目录 `docs/` |

### 修改文件统计

| 类型 | 数量 |
|------|------|
| 新增文件 | 4 |
| 修改文件 | 30+ |
| 删除文件 | 10+ |

### 提交记录

```
aa33c50 chore: allow debugThread.ts in gitignore
dabcce1 feat(web): add debug thread Zustand store
fb11f5d feat(web): add useWebSocket hook for real-time updates
6ef13df feat(web): add TypeScript types for debug functionality
ef5e40b feat(server): initialize DebugThreadManager in main
1f9bb10 refactor(api): inject DebugThreadManager into AgentHandler
90d8d5b fix: improve debug thread safety and preserve ProjectPath
5abbf26 feat(agent): add SpawnDebugAgent and ContinueDebugAgent to Orchestrator
6dfb6da fix: improve thread safety in GetMessages and add status constants
2fae2d2 fix: add sync.Once protection to Stop() to prevent panic on multiple calls
```

### 数据流

```
调试模式启动:
  前端 → api.agents.debug() → 后端 AgentHandler → DebugThreadManager.CreateThread()
      → Orchestrator.SpawnDebugAgent() → adapter.ExecuteWithStream()
      → WebSocket 广播流式输出 → 前端 debugThread store 更新

调试模式继续:
  前端 → api.agents.continueDebug() → Orchestrator.ContinueDebugAgent()
      → 获取最后 Agent 配置 → SpawnDebugAgent()
```

### 验证方法

1. 启动后端服务：`go run ./cmd/server`
2. 启动前端服务：`cd web && npm run dev`
3. 打开 Agent 调试页面
4. 选择一个 Agent 配置进行调试
5. 验证流式输出正常显示
6. 继续对话，验证上下文保持正确

### 影响范围

- 后端：Orchestrator, DebugThreadManager, AgentHandler
- 前端：ThreadView, useWebSocket, debugThread store
- 删除：session_manager, interactive_session 等冗余模块

---

## 2026-03-17 Agent执行功能重构与团队协作增强

### 背景

项目在团队协作开发中遇到了数据库变更同步困难的问题，需要建立一套完善的数据库版本控制机制。同时，对Agent执行与调试功能进行了重构，并对日志管理和无用文件进行了清理和优化。

### 目标

1. 实施数据库版本控制机制，确保变更同步
2. 重构Agent执行与调试的底层功能
3. 清理无用文件，优化项目结构
4. 建立日志管理规范

### 核心变更

#### Agent执行功能重构

##### 新增ExecutionService
- 创建 `internal/service/agent/execution_service.go` - 统一执行服务
- 整合 Orchestrator 和 InteractiveSession 的功能
- 实现 ExecutionContext（执行上下文）概念

##### Session管理增强
- 创建 `internal/service/agent/session_manager.go` - 会话管理器
- 创建 `internal/service/agent/execution_context.go` - 执行上下文定义

#### 数据库协作机制
- 创建 `scripts/migrate.sh` - 数据库迁移管理脚本
- 创建 `DB_MIGRATION_GUIDE.md` - 数据库迁移指南文档
- 实现数据库版本控制与团队协作方案

#### 日志管理改进
- 创建 `internal/config/logging.go` - 集中式日志配置
- 在 `cmd/server/main.go` 中集成日志管理功能
- 添加自动日志维护机制，定期清理过期日志

#### A2A交互协议完善
- 在 Orchestrator 中完善 Agent 路由验证逻辑
- 优化消息驱动的Agent交互流程
- 添加项目路径绑定功能，确保Agent在正确的工作目录下执行

#### 文件清理
删除了以下无用文件：
- `add_debug_project.go` - 调试项目添加脚本
- `add_debug_project.sql` - 调试项目SQL脚本
- `check_data.go` - 数据检查脚本
- `check_data2.go` - 数据检查脚本2
- `check_schema.go` - 模式检查脚本
- `check_current_schema.go` - 当前模式检查脚本
- `server.log` - 服务器日志文件
- `main.exe` - 旧可执行文件
- `server.exe` - 旧可执行文件

---

## 2026-03-17 修复项目路径绑定问题

### 背景

进行新项目开发时，Agent 的工作目录应该是绑定的项目路径，而不是当前工程路径。

### 问题

1. **SpawnRequest.ProjectPath 未设置**：`invocation_handler.go` 中的 Spawn 函数没有传递项目路径
2. **A2A 路由缺少项目路径**：`checkRouting` 和 `checkSignalRouting` 触发新 Agent 时没有传递项目路径
3. **用户消息触发缺少项目路径**：`SpawnAgentForUserMessage` 触发 Agent 时没有传递项目路径

### 目标

确保所有 Agent 触发场景都正确传递绑定项目的 `LocalPath` 作为工作目录。

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/api/invocation_handler.go` | 修改 | 添加 projectRepo 依赖，Spawn 时获取项目路径 |
| `internal/service/agent/orchestrator.go` | 修改 | 添加 projectRepo 依赖，多处 SpawnAgent 调用添加 ProjectPath |
| `cmd/server/main.go` | 修改 | 传递 projectRepo 给 InvocationHandler 和 Orchestrator |

### 详细改动

#### 1. InvocationHandler 添加项目路径获取

```go
// NewInvocationHandler 创建处理器
func NewInvocationHandler(orchestrator *agent.Orchestrator, mcpAuth *a2a.MCPAuthService, projectRepo *repo.ProjectRepository) *InvocationHandler

// Spawn 启动Agent
func (h *InvocationHandler) Spawn(c *gin.Context) {
    // 获取绑定的项目路径
    var projectPath string
    if h.projectRepo != nil {
        project, err := h.projectRepo.GetByThreadID(c.Request.Context(), threadID)
        if err == nil && project != nil {
            projectPath = project.LocalPath
        }
    }
    spawnReq := &agent.SpawnRequest{
        // ...
        ProjectPath: projectPath,
    }
}
```

#### 2. Orchestrator 添加 projectRepo 依赖

```go
type Orchestrator struct {
    // ...
    projectRepo *repo.ProjectRepository // 新增：项目仓库
}

func NewOrchestrator(..., projectRepo *repo.ProjectRepository, ...) *Orchestrator
```

#### 3. checkRouting 添加项目路径

```go
func (o *Orchestrator) checkRouting(...) {
    // 获取项目路径
    var projectPath string
    if o.projectRepo != nil {
        project, err := o.projectRepo.GetByThreadID(ctx, threadID)
        if err == nil && project != nil {
            projectPath = project.LocalPath
        }
    }
    o.SpawnAgent(ctx, &SpawnRequest{
        // ...
        ProjectPath: projectPath,
    })
}
```

#### 4. checkSignalRouting 添加项目路径

```go
func (o *Orchestrator) checkSignalRouting(...) {
    // 获取项目路径
    var projectPath string
    if o.projectRepo != nil {
        project, err := o.projectRepo.GetByThreadID(ctx, threadID)
        if err == nil && project != nil {
            projectPath = project.LocalPath
        }
    }
    o.SpawnAgent(ctx, &SpawnRequest{
        // ...
        ProjectPath: projectPath,
    })
}
```

#### 5. SpawnAgentForUserMessage 添加项目路径

```go
func (o *Orchestrator) SpawnAgentForUserMessage(...) error {
    // 获取项目路径
    var projectPath string
    if o.projectRepo != nil {
        project, err := o.projectRepo.GetByThreadID(ctx, threadID)
        if err == nil && project != nil {
            projectPath = project.LocalPath
        }
    }
    // 两处 SpawnAgent 调用都添加 ProjectPath
}
```

### 数据流

```
Thread → ProjectID → Project → LocalPath → SpawnRequest.ProjectPath
```

### 验证方法

1. 创建一个项目，设置 `local_path` 为目标开发目录
2. 创建 Thread 绑定该项目
3. 触发 Agent 执行
4. 验证 Agent 的工作目录为绑定的项目路径

### 影响范围

- 后端：Agent 触发逻辑
- 数据：不影响现有数据

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