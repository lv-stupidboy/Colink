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
- **10 TODO items**: Permission validation, message dedup, merge logic, artifact retrieval, GitLab API, Git query, markdown rendering, artifact preview — all marked TODO in code.
