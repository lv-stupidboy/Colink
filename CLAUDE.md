# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Colink - 多智能体协作平台，多 Agent 协同开发系统。

## 常用命令

```bash
# 后端开发
go run ./cmd/server             # 启动后端服务
make build                      # 构建 bin/isdp-server.exe
make genplugins                 # 生成插件注册（构建前自动执行）

# 前端开发
cd web && npm run dev           # 启动开发服务器
cd web && npm run build         # 构建生产版本
cd web && npm run lint          # ESLint 检查（max-warnings 0）

# 桌面应用开发
cd apps/desktop && npm run bundle-server    # 打包服务端资源
cd apps/desktop && npm run dev              # 启动桌面应用开发模式
cd apps/desktop && npm run dev:remote       # 远程服务端开发模式
cd apps/desktop && npm run build            # 构建桌面应用
cd apps/desktop && npm run package          # 打包当前平台
cd apps/desktop && npm run package:all      # 打包所有平台

# 安装器构建（完整发布）
pwsh -File scripts/build-release.ps1   # Windows 完整构建（7步）
./scripts/build-mac.sh                 # macOS 构建

# 测试
make test-all                    # 运行所有测试（前端 + 后端）
make test-frontend               # E2E + Vitest
make test-backend                # Go 单元测试
make test-feature F=F001         # 按特性 ID 运行测试
make test-p0                     # 只运行 P0 优先级测试

# E2E 测试
cd web && npx playwright test                # 运行所有 E2E
cd web && npx playwright test --grep "AD-01" # 按测试 ID 筛选

# Go 测试
go test ./auto-test/internal/... -v          # 所有内部测试
go test ./auto-test/internal/api/... -run API-01  # 按模式筛选

# 数据库迁移
go build -o bin/migrate.exe ./cmd/migrate    # 构建迁移工具
bin/migrate.exe up --db ./data/sqlite/colink.db --version 1.1.0   # 版本迁移
bin/migrate.exe run --db ./data/sqlite/colink.db --file xxx.sql   # 单文件执行
```

## 架构概览

```
Web前端 (React + Ant Design + Zustand)
        ↓ REST/WebSocket
后端服务 (Go + Gin)
  - cmd/server/           主入口（路由注册、依赖注入）
  - internal/api/         HTTP 处理器（22个文件）
  - internal/service/     业务逻辑层
      - agent/            Agent 调度器 + 插件适配器
      - a2a/              Agent-to-Agent 路由
      - sandbox/          Docker/本地进程隔离
      - workflow/         工作流引擎
      - im/               IM 平台集成（飞书等）
  - internal/repo/        数据访问层（33个文件，原生 SQL）
  - internal/model/       数据模型（19个文件）
  - internal/ws/          WebSocket Hub
        ↓ CLI Spawn
Agent 实例 (Claude CLI / OpenCode CLI / 其他 ACP 协议工具)
```

### 基础 Agent 插件化架构

支持多种 CLI 工具（Claude Code、OpenCode 等），通过插件机制扩展：

```
internal/service/agent/plugins/
├── all/all.go              # 自动导入所有插件（genplugins 生成）
├── claude_code/            # Claude CLI 适配器
│   ├── plugin.go           # init() 注册插件
│   └── adapter.go          # 实现 AgentAdapter 接口
└── open_code/              # OpenCode CLI 适配器
    ├── plugin.go
    └── adapter.go          # 包含标准适配器和 ACP 适配器
```

**AgentAdapter 接口**（必须实现）：
```go
type AgentAdapter interface {
    Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error)
    ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) (*ExecutionResult, error)
    ParseOutput(output string) ([]ParsedBlock, error)
    GetDefaultModel(baseAgent *model.BaseAgent) string
    IsSessionSupported() bool
    // ... 更多方法见 internal/service/agent/adapter.go
}
```

**新增插件步骤：**
1. 在 `plugins/` 下创建新目录（如 `my_cli/`）
2. 实现 `AgentAdapter` 接口的所有方法
3. 在 `plugin.go` 中调用 `RegisterPlugin()` 注册：
   ```go
   func init() {
       agent.RegisterPlugin(agent.PluginMeta{
           Type:   "my_cli",
           Factory: func(baseAgent *model.BaseAgent) agent.AgentAdapter {
               return &MyCLIAdapter{baseAgent: baseAgent}
           },
       })
   }
   ```
4. 运行 `make genplugins` 自动更新 `all/all.go`

**ACP 协议适配器**：
如果新 CLI 支持 JSON-RPC 2.0 over stdio（ACP 协议），可继承 `BaseACPAdapter`：
```go
// internal/service/agent/acp_adapter.go
type BaseACPAdapter struct {
    // 提供 JSON-RPC 2.0 通信基础实现
}

// 示例：OpenCodeACPAdapter 继承 BaseACPAdapter
type OpenCodeACPAdapter struct {
    BaseACPAdapter
    // 添加 OpenCode 特定参数
}
```

### 启动原则

**端口配置（灵活查找）**
- **后端端口**：`data/configs/config.yaml` 中 `server.port`（默认 26305）
- **前端端口**：`data/configs/config.yaml` 中 `web.port`（默认 26306）
- **前端代理目标**：`data/configs/config.yaml` 中 `web.api_url`

**启动顺序：先启动后端，再启动前端**
```bash
# 1. 启动后端（项目根目录）
go run ./cmd/server

# 2. 启动前端
cd web && npm run dev
```

**配置查找顺序（后端）**：
命令行 `-config` → 环境变量 `ISDP_CONFIG` → `data/configs/config.yaml` → `configs/config.yaml`

**前端代理规则**：
- `/api/*` 代理到后端地址（见 `web/vite.config.ts` proxy）
- WebSocket 代理已启用（`ws: true`）

**注意事项**：
- 前端 `strictPort: true`：端口冲突时报错，不会自动切换
- 后端构建自动生成插件注册（`make genplugins`）
- 前端构建先执行 TypeScript 检查（`tsc && vite build`）

## 开发工作流

### 完整发布构建

从主项目目录执行：

```powershell
pwsh -File scripts/build-release.ps1   # Windows 构建（7步）
```

**构建步骤（7步）**：
1. **ISDP 后端** - 编译 `bin/colink-server.exe` 和 `bin/migrate.exe`
2. **ISDP 前端** - 构建 `web/dist/`
3. **资源同步** - 复制到 `staging/resources/`
4. **配置文件** - 复制 `VERSION` 和 `installer-config.json`
5. **Installer 前端** - 构建 Tauri 前端 `dist/`
5.5. **图标生成** - 从 `icon.png` 生成各平台图标
6. **Tauri exe** - 编译 `Colink-Setup.exe` 和 `Colink.exe`
7. **ZIP 打包** - 输出到 `target/release/dist/Colink-Setup-{VERSION}.zip`

### MCP Server

独立 MCP（Model Context Protocol）服务，用于 Agent 工具调用：

```bash
# 构建 MCP Server
go build -o bin/mcp-server.exe ./cmd/mcp-server

# 运行（需要环境变量）
$env:ISDP_API_URL = "http://localhost:26305"
$env:ISDP_INVOCATION_ID = "<invocation-id>"
$env:ISDP_CALLBACK_TOKEN = "<token>"
bin/mcp-server.exe
```

MCP Server 通过环境变量配置 API 地址、调用 ID 和回调 token，启动后监听 JSON-RPC 请求。

### Where to Look

| 任务 | 位置 | 说明 |
|------|------|------|
| 新增 API endpoint | `internal/api/` | 每个资源一个 handler 文件；在 `cmd/server/main.go` 注册路由 |
| 新增业务逻辑 | `internal/service/<domain>/` | 新建包；在 `main.go` 中注入依赖 |
| 新增 DB 表/列 | `sql-change/v{version}/{db_type}/` | 创建迁移文件；更新 model + repo |
| 新增前端页面 | `web/src/pages/` | 在 `App.tsx` 添加路由；在 `api/client.ts` 添加 API 方法 |
| 新增前端组件 | `web/src/components/` | Ant Design 基础；Zustand 状态管理 |
| 修改 Agent 执行 | `internal/service/agent/` | Adapter 接口在 `adapter.go`；Orchestrator 在 `orchestrator.go` |
| 新增 ACP 适配器 | `internal/service/agent/acp_adapter.go` | 继承 `BaseACPAdapter`；注册到 `NewAdapter()` |
| 修改 A2A 路由 | `internal/service/a2a/` | `EnqueueA2ATargets()` 在 `a2a_trigger.go` |
| 修改飞书 IM | `internal/service/im/` | `FeishuBridgeService` → `LarkCLIClient` |
| 新增 IM 平台 | `internal/service/im/` + `api/` | 参考 Feishu 模式：handler → bridge service → chunk forward |
| 修改安装器 | `installer-tauri/src-tauri/src/` | `services/installer.rs` 安装流程，`commands/` IPC 命令 |
| 更新配置 | `pkg/config/config.go` + `configs/config.yaml.example` | 两文件必须同步 |

### 关键约束

**API 字段命名**
- **JSON 字段统一使用 camelCase**（与前端一致）
- Go 结构体 JSON tag：`json:"baseAgentId"` 而非 `json:"base_agent_id"`

**前端深色模式**
- **使用 CSS 变量**：`var(--color-primary)` 等，禁止硬编码颜色
- 变量定义：`web/src/themes/themeVariables.css`
- 主题切换：通过 `data-theme` 属性，深色主题为 `dark`
- 新增组件时确保 `[data-theme='dark']` 样式正常

```css
/* ✅ 正确 */
background: var(--bg-container);
color: var(--text-primary);

/* ❌ 错误 */
background: #ffffff;
color: #333333;
```

**配置文件**
- `configs/config.yaml.example` - 配置模板（提交到 git）
- `configs/config.yaml` - 本地配置，含敏感信息（不提交）

**数据库迁移**

迁移脚本位于 `sql-change/` 目录，按版本号组织：

```
sql-change/
├── v1.1.0/sqlite/00001_init.sql    # goose 版本: 1
├── v1.2.0/sqlite/00002_xxx.sql     # goose 版本: 2
└── v1.2.0/mysql/00002_xxx.sql
```

**Goose版本号规则（重要）**：
- 文件序号 `{序号}_xxx.sql` 是**全局递增**，不按版本目录隔离
- goose 根据文件名序号判断迁移顺序
- 不同版本目录下的文件序号不能重复（否则会跳过）

**管理规则**：
- 新版本发布时创建 `v{版本号}` 目录
- 目录下按数据库类型分 `mysql/` 和 `sqlite/`
- 文件序号必须全局递增（00001, 00002, 00003...）
- 必须包含 `-- +goose Up` 和 `-- +goose Down`

**命令对比**：
| 命令 | 执行范围 | 版本记录 | 适用场景 |
|------|----------|----------|----------|
| `up` | 版本目录下所有未执行脚本 | 记录 goose 版本 | 正式迁移、安装器自动执行 |
| `run` | 单个指定文件 | 不记录版本 | 开发协作、手动执行 |

**开发协作流程**：
1. 拉取代码后检查 `sql-change/` 是否有新增 SQL
2. 使用 `migrate run --file xxx.sql` 执行他人变更
3. 或使用 `migrate up --version xxx` 执行整个版本目录

**主要表结构**：
| 表名 | 说明 |
|------|------|
| base_agents | 基础 Agent 配置（Claude、OpenCode 等） |
| workflow_templates | 工作流模板 |
| projects | 项目信息 |
| threads | 开发会话 |
| messages | 对话消息 |
| agent_configs | Agent 角色配置 |
| agent_invocations | Agent 调用记录 |
| artifacts | 开发产物 |
| sandboxes | 沙箱容器 |

**数据库类型**
- 默认使用 SQLite（`modernc.org/sqlite` 纯 Go 驱动，无需 CGO）
- 过渡期保留 MySQL 支持（`database.type` 配置切换）

**服务端口**
- 后端端口：`configs/config.yaml` 中 `server.port`
- 前端开发端口：`configs/config.yaml` 中 `web.port`

## Data 目录

`data/` 存放用户数据，升级时保留：
- `configs/config.yaml` - 主配置
- `logs/` - 日志
- `agent-assets/` - Agent 资产
- `repos/` - 代码仓库

**配置查找顺序**：命令行 `-config` → 环境变量 `ISDP_CONFIG` → `data/configs/config.yaml` → `configs/config.yaml`

## 自动测试体系

所有测试统一放置在 `auto-test/` 目录：

```
auto-test/
├── e2e/                    # Playwright E2E 测试
│   ├── fixtures/           # 测试 fixtures
│   ├── agent-dialog/       # AD-01 ~ AD-04
│   ├── websocket/          # WS-01 ~ WS-02
│   └── ...
├── internal/               # Go 单元测试（可导入 internal 包）
│   ├── api/                # API Handler 测试
│   ├── service/            # Service 层测试
│   └── repo/               # Repo 层测试
├── vitest/                 # Vitest 组件测试
│   └── components/         # UI 组件测试
└── feature-map.yaml        # Feature ID → 测试映射
```

### 测试优先级

| 优先级 | 覆盖率要求 | 说明 |
|--------|-----------|------|
| **P0** | 必须 100% | 阻塞功能、核心流程、安全相关 |
| **P1** | ≥ 95% | 重要功能、主要路径 |
| **P2** | ≥ 80% | 一般功能、边界场景 |
| **P3** | 可选 | 体验优化、次要功能 |

### 测试 ID 格式

`{模块}-{分类}-{序号}`，示例：
- `AD-01-02`：Agent Dialog，消息输入分类，第 2 个用例
- `WS-01-03`：WebSocket，连接管理分类，第 3 个用例

每个测试文件头部需标注：
```go
// @feature F001 - Agent 对话核心
// @priority P0
// @id API-01-01
func TestAgentHandler_List(t *testing.T) { ... }
```

### 测试报告

按时间戳命名，保留历史记录：
- 运行 ID：`YYYYMMDD-HHMMSS`
- HTML 报告：`web/playwright-report/{runId}/index.html`
- JSON 结果：`web/test-results/{runId}/test-results.json`

**查看报告**：
```bash
python scripts/test-report-summary.py            # 最新摘要
python scripts/test-report-summary.py --by-feature  # 按特性分组
python scripts/test-report-summary.py --list      # 所有历史
npx playwright show-report web/playwright-report/$(ls -t web/playwright-report | head -1)
```

### Feature ID 映射

| Feature ID | 功能模块 | 关键测试 |
|------------|---------|---------|
| F001 | Agent 对话核心 | AD-01, TW-02 |
| F002 | WebSocket 流式 | WS-01 |
| F003 | 多 Agent 协作 | SV-02 |
| F004 | 团队包管理 | TP-01 |
| F005 | 线程管理 | TW-01, FT-01~06 |
| F006 | 工作流执行 | PF-01 |

## Anti-Patterns（本项目禁止）

- `as any`, `@ts-ignore`, `@ts-expect-error` — 前端代码禁止
- Direct `api/ → repo/` imports — 应通过 service 层路由（已有例外：`callback_handler.go`、`registry_handler.go`）
- Modifying `init.sql` — 使用版本化迁移
- Committing `config.yaml` — 含敏感信息，必须在 `.gitignore`
- Empty catch blocks — 必须处理错误
- `DROP COLUMN IF EXISTS` — MySQL 5.7 不兼容
- Hardcoded colors in CSS — 使用 CSS 变量
- Docker-compose targets in Makefile — 无 docker-compose.yml（dev container 使用 `.devcontainer/`）

## Key Files

| 文件 | 作用 |
|------|------|
| `cmd/server/main.go` | 主入口（~970行），路由注册、依赖注入、插件导入 |
| `internal/service/agent/adapter.go` | AgentAdapter 接口定义 |
| `internal/service/agent/orchestrator.go` | Agent 调度器，协调 A2A |
| `internal/service/agent/execution_service.go` | Agent 生命周期管理（timeout/depth/session） |
| `internal/service/a2a/a2a_trigger.go` | @mention → queue → spawn |
| `pkg/config/config.go` | 配置加载（Viper）+ 默认值设置 |
| `tools/genplugins/main.go` | 自动生成插件注册代码 |
| `VERSION` | 基础版本号（1.0.0） |
| `Makefile` | build, run, test, clean, desktop 命令 |

## Important Notes

- **Directory flattening**：之前嵌套 `isdp/isdp/...` 已扁平到项目根目录，`go.mod` 在根目录
- **Brand rename**：ISDP → Colink，部分内部引用可能仍使用 ISDP
- **Dual DB support**：MySQL（生产）和 SQLite（开发），`database.type` 配置切换
- **No CI/CD**：无 GitHub Actions，构建通过本地 `build.sh`/`build.ps1`
- **A2A depth limit**：最大 15 层嵌套 Agent 调用（`MaxA2ADepth` 常量）
- **Session resumption**：Agent 支持 `--resume` 重用 CLI 会话，通过 `SessionChainStore` 跟踪
- **Feishu IM**：飞书是前端入口，Agent 执行仍通过 `Orchestrator → ExecutionService → Adapter`
- **ChunkListener**：外部监听器通过 `ExecutionService.AddChunkListener()` 接收所有 chunk 类型
- **ACP config isolation**：`buildEnv()` 设置 `OPENCODE_PURE=1` 阻止用户级插件，`OPENCODE_CONFIG_DIR` 传递配置目录