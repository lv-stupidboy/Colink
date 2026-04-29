# Colink — Project Knowledge Base

**Generated:** 2026-04-09  
**Commit:** 72baf2f  
**Branch:** master

## Overview

Multi-agent software development platform (formerly ISDP). Go backend (Gin + raw SQL) orchestrates Claude CLI, OpenCode CLI, and ACP protocol adapters to run AI agents in workflows. React frontend (Ant Design + Zustand) provides the workbench UI. Electron installer packages Setup and Launcher as dual Windows apps. Feishu (Lark) IM integration enables users to interact with agents via chat.

## Structure

```
isdp/                              # Repository root (Go module root — go.mod here)
├── cmd/server/                    # Main entry point (main.go, ~970 lines)
├── cmd/migrate_mysql/             # DB migration utility
├── internal/
│   ├── api/                       # 22 Gin HTTP handlers
│   ├── config/                    # Logging setup (Zap + Lumberjack)
│   ├── middleware/                 # Invite code auth
│   ├── model/                     # 19 domain entity files
│   ├── parser/                    # Mention parser
│   ├── repo/                      # 33 repository files (raw SQL)
│   ├── service/                   # 20 service packages (business logic)
│   │   ├── agent/                 # Orchestrator + adapters (Claude/OpenCode/ACP) + ChunkListener
│   │   ├── a2a/                   # Agent-to-Agent routing + queue
│   │   ├── im/                    # Feishu (Lark) IM bridge service
│   │   ├── sandbox/               # Docker/local process execution
│   │   ├── workflow/              # Workflow template CRUD
│   │   ├── configgen/             # Per-agent config directory generation
│   │   ├── teampackage/           # Team package import/export
│   │   ├── assetpackage/          # Asset package import/export
│   │   ├── skill/                 # Skill CRUD + federated registry
│   │   └── knowledge/             # Knowledge base CRUD
│   └── ws/                        # WebSocket hub
├── pkg/config/                    # YAML config loader (Viper)
├── web/                           # React frontend (see web/AGENTS.md)
├── sql-change/                    # DB migrations (see sql-change/AGENTS.md)
├── configs/                       # Config template + local config
├── .devcontainer/                 # DevContainer (Go 1.25 + Node 20 + MySQL + Redis)
├── installer/                     # Electron installer (see installer/AGENTS.md)
├── docs/                          # Plans, specs, changelog
│   ├── superpowers/plans/         # Implementation plan docs
│   ├── superpowers/specs/         # Design spec docs
│   └── CHANGELOG.md               # Dev history
├── auto-test/                     # Unified test directory (340 cases)
│   ├── e2e/                       # Playwright E2E tests
│   ├── internal/                  # Go unit tests (imports internal/)
│   ├── vitest/                    # Vitest component tests
│   └── feature-map.yaml           # Feature ID → test mapping
├── scripts/                       # Test runners, utilities
├── VERSION                        # Base version: 1.0.0
├── Makefile                       # build, run, test, clean
└── CLAUDE.md                      # AI guidance (naming, config, DB rules)
```

## Where to Look

| Task | Location | Notes |
|------|----------|-------|
| Add API endpoint | `internal/api/` | One handler file per resource; register routes in `cmd/server/main.go` |
| Add service logic | `internal/service/<domain>/` | New package per domain; inject via `main.go` |
| Add DB table/column | `sql-change/migrations/` | Create migration file, update model + repo |
| Add frontend page | `web/src/pages/` | Add route in `App.tsx`, API method in `api/client.ts` |
| Add frontend component | `web/src/components/` | Ant Design components; use Zustand for state |
| Change agent execution | `internal/service/agent/` | Adapter interface in `adapter.go` |
| Add ACP-based adapter | `internal/service/agent/acp_adapter.go` + `adapter.go` | Extend `BaseACPAdapter`, register in `NewAdapter()` |
| Change A2A routing | `internal/service/a2a/` | `EnqueueA2ATargets()` in `a2a_trigger.go` |
| Change Feishu IM | `internal/service/im/` | `FeishuBridgeService` → `LarkCLIClient` → lark-cli |
| Add IM platform | `internal/service/im/` + `model/im_session.go` + `api/feishu_webhook_handler.go` | Follow Feishu pattern: handler → bridge service → chunk forward |
| Modify installer | `installer/src/main/` | `index.ts` = Setup, `launcher-entry.ts` = Launcher |
| Update config schema | `pkg/config/config.go` + `configs/config.yaml.example` | Both files must sync |
| Run backend | `.` | `make run` or `go run ./cmd/server` |
| Run frontend dev | `web/` | `npm run dev` (port 3000, proxies to 8080) |
| Run tests | `.` | `make test` (Go); `cd web && npm run test:e2e` (Playwright) |
| Run auto-tests | `auto-test/` | `make test-all`; `python scripts/run-feature-tests.py --feature F001` |
| Build release | `installer/` | `./build.sh` (Unix) or `.\build.ps1` (Windows) |
| DevContainer | `.devcontainer/` | MySQL 8 + Redis 7 + Go 1.25 + Node 20 + Playwright |

## Code Map

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `Orchestrator` | struct | `service/agent/orchestrator.go` | Central agent dispatch; spawns agents, coordinates A2A |
| `ExecutionService` | struct | `service/agent/execution_service.go` | Agent lifecycle with timeout/depth/session + ChunkListener + content block flush |
| `AgentAdapter` | interface | `service/agent/adapter.go` | Execute, stream, session lifecycle for any CLI |
| `ClaudeAdapter` | struct | `service/agent/claude_adapter.go` | Claude CLI integration |
| `OpenCodeAdapter` | struct | `service/agent/open_code_adapter.go` | OpenCode CLI integration |
| `BaseACPAdapter` | struct | `service/agent/acp_adapter.go` | ACP protocol (JSON-RPC 2.0 over stdio) base adapter |
| `OpenCodeACPAdapter` | struct | `service/agent/open_code_acp_adapter.go` | OpenCode ACP wrapper (extends BaseACPAdapter) |
| `FeishuBridgeService` | struct | `service/im/feishu_bridge_service.go` | Feishu IM: webhook → agent → chunk buffering → Feishu forward |
| `LarkCLIClient` | struct | `service/im/lark_cli_client.go` | lark-cli external process wrapper |
| `IMSession` | struct | `model/im_session.go` | Maps Feishu chat_id → ISDP thread_id |
| `FeishuWebhookHandler` | struct | `api/feishu_webhook_handler.go` | POST /api/v1/feishu/webhook (whitelisted from invite auth) |
| `EnqueueA2ATargets` | func | `service/a2a/a2a_trigger.go` | @mention → queue → spawn |
| `QueueProcessor` | struct | `service/a2a/queue_processor.go` | Auto-executes queued agents |
| `MentionParser` | struct | `parser/mention.go` | Extracts @Agent references from text |
| `SandboxService` | struct | `service/sandbox/sandbox_service.go` | Docker/local process isolation |
| `WorkflowEngine` | struct | `service/agent/workflow.go` | Phase transitions + validation |
| `AppState/AppActions` | interface | `web/src/store/index.ts` | Zustand global state (70+ fields, 30+ actions) |
| `APIClient` | class | `web/src/api/client.ts` | Axios client with snake→camelCase transform |

## Conventions

- **JSON fields**: camelCase everywhere (`baseAgentId`, not `base_agent_id`). See CLAUDE.md mapping table.
- **Empty arrays**: Return `[]`, never `{}` or `null`. Frontend uses `Array.isArray()` defensively.
- **Config changes**: Update BOTH `config.yaml.example` (template) AND `config.yaml` (local). Set defaults in `pkg/config/config.go:setDefaults()`.
- **DB migrations**: Versioned directories in `sql-change/migrations/v{version}/`. Never modify `init.sql`.
- **No ORM**: Raw SQL via `database/sql` with custom `Dialect` interface (MySQL vs SQLite).
- **Repository pattern**: Concrete structs, no interfaces. `New*Repository(db *sql.DB)` constructors.
- **Version injection**: `VERSION` → ldflags (Go) + `generate-version.js` (frontend).
- **UI terminology**: English in UI (Skills, Commands, Subagents, Rules, Settings). Chinese in code comments.
- **CSS theming**: Use CSS variables (`var(--color-primary)`), never hardcoded colors. Dark mode via `[data-theme='dark']`.
- **TypeScript strictness**: `strict: true`, `noUnusedLocals`, `noUnusedParameters`.

## Anti-Patterns (This Project)

- `as any`, `@ts-ignore`, `@ts-expect-error` — forbidden in frontend code
- Direct `api/ → repo/` imports — exists in `callback_handler.go` and `registry_handler.go`; prefer routing through service layer
- Modifying `init.sql` — use versioned migrations instead
- Committing `config.yaml` — contains secrets, must stay in `.gitignore`
- Empty catch blocks — always handle errors
- `DROP COLUMN IF EXISTS` — MySQL 5.7 incompatible
- Hardcoded colors in CSS — use CSS variables from `web/src/themes/themeVariables.css`
- Docker-compose targets in Makefile — `docker-compose.yml` does not exist in repo (dev container uses `.devcontainer/docker-compose.dev.yml`)

## Commands

```bash
# Backend
make run                # Dev server (port 8080)
make build              # Build bin/isdp-server.exe
make test               # Go tests with coverage

# Frontend
cd web && npm run dev       # Dev server (port 3000)
cd web && npm run build     # Production build
cd web && npm run lint      # ESLint (max-warnings 0)
cd web && npm run test:e2e  # Playwright E2E tests
cd web && npm run test:e2e:headed  # Playwright visible browser

# Auto-test (unified test directory)
make test-all                    # Run all tests (frontend + backend)
make test-frontend               # E2E + Vitest tests
make test-backend                # Go unit tests
python scripts/run-feature-tests.py --feature F001  # By Feature ID
python scripts/run-feature-tests.py --priority P0   # By priority

# Full release
cd installer && ./build.sh       # Unix: backend → frontend → installer → ZIP
cd installer && .\build.ps1      # Windows: same flow

# DevContainer
# Open in VS Code: Reopen in Container (auto-runs post-create.sh)
```

## Notes

- **Directory flattening**: Previously nested `isdp/isdp/...` flattened to project root. `go.mod` at repo root.
- **Brand rename**: ISDP → Colink (commit cdeb065). Some internal references may still say ISDP.
- **Stranded `isdp/` directory**: `isdp/internal/service/im/` still contains IM service files at old path. Should be moved to `internal/service/im/`.
- **Dual DB support**: MySQL (production) and SQLite (development). Config `database.type` switches between them.
- **No CI/CD**: No GitHub Actions workflows. Builds are local via `build.sh`/`build.ps1`.
- **A2A depth limit**: Max 15 nested agent calls (`MaxA2ADepth` constant).
- **Session resumption**: Agents support `--resume` to reuse CLI sessions, tracked via `SessionChainStore`.
- **Feishu IM**: Feishu is a frontend entry point, NOT an AgentAdapter. Agent execution still goes through `Orchestrator → ExecutionService → Claude/OpenCode/ACP adapter`.
- **ChunkListener**: External chunk listeners registered via `ExecutionService.AddChunkListener()` receive ALL chunk types including usage and status.
- **ACP protocol**: JSON-RPC 2.0 over stdio. `BaseACPAdapter` is the base; `OpenCodeACPAdapter` adds OpenCode-specific CLI args. New agent types can extend `BaseACPAdapter`.
- **ACP config isolation**: `buildEnv()` sets `OPENCODE_PURE=1` to block user-level OpenCode plugins (e.g. oh-my-openagent) that set `default_agent` to a plugin-defined agent unavailable in ACP context. `OPENCODE_CONFIG_DIR` passes per-agent config directories (agents/skills/rules/commands) which load independently and are NOT gated by OPENCODE_PURE — only `plugin/` subdirs are blocked.
- **10 TODO items**: Permission validation, message dedup, merge logic, artifact retrieval, GitLab API, Git query, markdown rendering, artifact preview — all marked TODO in code.

## Auto-Test Guidelines

测试统一放置在 `auto-test/` 目录。编写新测试时遵循以下规范：

### 测试 ID 格式

```
{模块}-{分类}-{序号}
```

示例：
- `AD-01-02`：Agent Dialog，消息输入分类，第 2 个用例
- `WS-01-03`：WebSocket，连接管理分类，第 3 个用例
- `API-01-05`：API Handler，Agent CRUD 分类，第 5 个用例

### 优先级定义

| 优先级 | 覆盖率要求 | 适用场景 |
|--------|-----------|----------|
| P0 | 100% 必须通过 | 核心流程阻塞、安全相关、数据完整性 |
| P1 | ≥95% | 主要功能、正常/异常路径 |
| P2 | ≥80% | 边界场景、性能测试 |
| P3 | 可选 | 体验优化、次要功能 |

### 注释格式

每个测试需在头部标注：

```go
// @feature F001 - Agent 对话核心
// @priority P0
// @id API-01-01
func TestAgentHandler_List(t *testing.T) { ... }
```

```typescript
// @feature F001 - Agent 对话核心
// @priority P0
test('AD-01-01: 输入框正常显示 [F001]', async ({ page }) => { ... });
```

### Feature ID 映射

Feature ID 定义在 `auto-test/feature-map.yaml`：

| Feature ID | 功能模块 |
|------------|---------|
| F001 | Agent 对话核心 |
| F002 | WebSocket 流式 |
| F003 | 多 Agent 协作 (A2A) |
| F004 | 团队包管理 |
| F005 | 工作流执行 |
| F006 | 沙箱隔离 |
| F007 | 消息存储 |
| F008 | IM 集成 |
| F009 | 系统性能与稳定性 |
| F010 | 安装器与启动 |

### Go 测试命名

- 文件：`{module}_test.go`，放在 `auto-test/internal/{path}/`
- 包名：`{module}_test`（允许导入 `internal` 包）
- 函数：`Test{Category}_{TestCase}` 或直接使用测试 ID

### E2E 测试命名

- 文件：`{category}.spec.ts`，放在 `auto-test/e2e/{module}/`
- describe：`'{Module}-{Category} [P0/P1]'`
- test：`'{TestID}: {描述} [{FeatureID}]'`

### Vitest 测试命名

- 文件：`{Component}.test.ts`，放在 `auto-test/vitest/{type}/`
- describe：`'{Module}: {Component} [P0/P1]'`
- it：`'{TestID}: {描述} [{FeatureID}]'`
