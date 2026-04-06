# ISDP — Project Knowledge Base

**Generated:** 2026-04-06  
**Commit:** 90b6e50  
**Branch:** master

## Overview

Multi-agent software development platform. Go backend (Gin + raw SQL) orchestrates Claude CLI and OpenCode CLI adapters to run AI agents in workflows. React frontend (Ant Design + Zustand) provides the workbench UI. Electron installer packages Setup and Launcher as dual Windows apps. Feishu (Lark) IM integration enables users to interact with agents via Feishu chat.

## Structure

```
isdp/                          # Repository root
├── isdp/                      # Go backend module (go.mod here)
│   ├── cmd/server/            # Main entry point (main.go, ~960 lines)
│   ├── cmd/migrate_mysql/     # DB migration utility
│   ├── internal/
│   │   ├── api/               # 22 Gin HTTP handlers
│   │   ├── config/            # Logging setup
│   │   ├── middleware/        # Invite code auth
│   │   ├── model/             # 18 domain entity files
│   │   ├── parser/            # Mention parser
│   │   ├── repo/              # 31 repository files (raw SQL)
│   │   ├── service/           # 19 service packages (business logic)
│   │   │   ├── agent/         # Orchestrator + adapters (Claude/OpenCode/ACP) + ChunkListener
│   │   │   ├── a2a/           # Agent-to-Agent routing + queue
│   │   │   ├── im/            # Feishu (Lark) IM bridge service
│   │   │   ├── sandbox/       # Docker/local process execution
│   │   │   ├── workflow/      # Workflow template CRUD
│   │   │   ├── configgen/     # Per-agent config directory generation
│   │   │   ├── teampackage/   # Team package import/export
│   │   │   ├── assetpackage/  # Asset package import/export
│   │   │   └── skill/         # Skill CRUD + federated registry
│   │   └── ws/                # WebSocket hub
│   ├── pkg/config/            # YAML config loader (Viper)
│   ├── web/                   # React frontend (see web/AGENTS.md)
│   ├── sql-change/            # DB migrations (see isdp/AGENTS.md)
│   ├── configs/               # Config template + local config
│   ├── VERSION                # Base version: 0.3.0
│   └── Makefile               # build, run, test, clean
├── installer/                 # Electron installer (see installer/AGENTS.md)
├── docs/                      # Plans, specs, changelog
│   ├── superpowers/plans/     # 15 implementation plan docs
│   ├── superpowers/specs/     # 17 design spec docs
│   └── CHANGELOG.md           # Dev history
└── CLAUDE.md                  # AI guidance (naming, config, DB rules)
```

## Where to Look

| Task | Location | Notes |
|------|----------|-------|
| Add API endpoint | `isdp/internal/api/` | One handler file per resource; register routes in `cmd/server/main.go` |
| Add service logic | `isdp/internal/service/<domain>/` | New package per domain; inject via `main.go` |
| Add DB table/column | `isdp/sql-change/migrations/` | Create migration file, update model + repo |
| Add frontend page | `isdp/web/src/pages/` | Add route in `App.tsx`, API method in `api/client.ts` |
| Add frontend component | `isdp/web/src/components/` | Ant Design components; use Zustand for state |
| Change agent execution | `isdp/internal/service/agent/` | Adapter interface in `adapter.go` |
| Change A2A routing | `isdp/internal/service/a2a/` | `EnqueueA2ATargets()` in `a2a_trigger.go` |
| Change Feishu IM | `isdp/internal/service/im/` | `FeishuBridgeService` → `LarkCLIClient` → lark-cli |
| Modify installer | `installer/src/main/` | `index.ts` = Setup, `launcher-entry.ts` = Launcher |
| Update config schema | `isdp/pkg/config/config.go` + `configs/config.yaml.example` | Both files must sync |
| Run backend | `isdp/` | `make run` or `go run ./cmd/server` |
| Run frontend dev | `isdp/web/` | `npm run dev` (port 3000, proxies to 8080) |
| Run tests | `isdp/` | `make test` (Go); `cd web && npm run test:e2e` (Playwright) |
| Build release | `installer/` | `./build.sh` (Unix) or `.\build.ps1` (Windows) |

## Code Map

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `Orchestrator` | struct | `service/agent/orchestrator.go` | Central agent dispatch; spawns agents, coordinates A2A |
| `ExecutionService` | struct | `service/agent/execution_service.go` | Manages agent execution with timeout/depth/session + ChunkListener |
| `AgentAdapter` | interface | `service/agent/adapter.go` | Execute, stream, session lifecycle for any CLI |
| `ClaudeAdapter` | struct | `service/agent/claude_adapter.go` | Claude CLI integration |
| `OpenCodeAdapter` | struct | `service/agent/open_code_adapter.go` | OpenCode CLI integration |
| `EnqueueA2ATargets` | func | `service/a2a/a2a_trigger.go` | @mention → queue → spawn |
| `QueueProcessor` | struct | `service/a2a/queue_processor.go` | Auto-executes queued agents |
| `MentionParser` | struct | `parser/mention.go` | Extracts @Agent references from text |
| `SandboxService` | struct | `service/sandbox/sandbox_service.go` | Docker/local process isolation |
| `WorkflowEngine` | struct | `service/agent/workflow.go` | Phase transitions + validation |
| `FeishuBridgeService` | struct | `service/im/feishu_bridge_service.go` | Feishu IM bridge: webhook → agent → chunk forward |
| `LarkCLIClient` | struct | `service/im/lark_cli_client.go` | lark-cli external process wrapper |
| `AppState/AppActions` | interface | `web/src/store/index.ts` | Zustand global state (70+ fields, 30+ actions) |
| `APIClient` | class | `web/src/api/client.ts` | Axios client with snake→camelCase transform |

## Conventions

- **JSON fields**: camelCase everywhere (`baseAgentId`, not `base_agent_id`). See CLAUDE.md mapping table.
- **Empty arrays**: Return `[]`, never `{}` or `null`. Frontend uses `Array.isArray()` defensively.
- **Config changes**: Update BOTH `config.yaml.example` (template) AND `config.yaml` (local). Set defaults in `pkg/config/config.go:setDefaults()`.
- **DB migrations**: File per change in `sql-change/migrations/YYYYMMDDNN_desc.sql`. Never modify `init_db_mysql.sql`.
- **No ORM**: Raw SQL via `database/sql` with custom `Dialect` interface (MySQL vs SQLite).
- **Repository pattern**: Concrete structs, no interfaces. `New*Repository(db *sql.DB)` constructors.
- **Version injection**: `isdp/VERSION` → ldflags (Go) + `generate-version.js` (frontend).
- **UI terminology**: English in UI (Skills, Commands, Subagents, Rules, Settings). Chinese in code comments.

## Anti-Patterns (This Project)

- `as any`, `@ts-ignore` — forbidden in frontend code
- Direct `api/ → repo/` imports — exists in `callback_handler.go` and `registry_handler.go`; prefer routing through service layer
- Modifying `init_db_mysql.sql` — use migrations instead
- Committing `config.yaml` — contains secrets, must stay in `.gitignore`
- Empty catch blocks — always handle errors
- Docker-compose targets in Makefile — `docker-compose.yml` does not exist in repo

## Commands

```bash
# Backend
cd isdp && make run              # Dev server (port 8080)
cd isdp && make build            # Build bin/isdp-server.exe
cd isdp && make test             # Go tests with coverage

# Frontend
cd isdp/web && npm run dev       # Dev server (port 3000)
cd isdp/web && npm run build     # Production build
cd isdp/web && npm run lint      # ESLint
cd isdp/web && npm run test:e2e  # Playwright E2E tests

# Full release
cd installer && ./build.sh       # Unix: backend → frontend → installer → ZIP
```

## Notes

- **Nested `isdp/isdp/`**: Repository root contains `isdp/` which is the Go module root. `go.mod` is at `isdp/isdp/go.mod`, not repo root.
- **Dual DB support**: MySQL (production) and SQLite (development). Config `database.type` switches between them.
- **No CI/CD**: No GitHub Actions workflows in repo. Builds are local via `build.sh`/`build.ps1`.
- **A2A depth limit**: Max 15 nested agent calls (`MaxA2ADepth` constant).
- **Session resumption**: Agents support `--resume` to reuse CLI sessions, tracked via `SessionChainStore`.
- **Feishu IM**: Feishu is a frontend entry point, NOT an AgentAdapter. lark-cli is not an AI agent. Agent execution still goes through `Orchestrator → ExecutionService → Claude/OpenCode/ACP adapter`. Webhook endpoint at `POST /api/v1/feishu/webhook` (whitelisted from invite auth).
- **ChunkListener**: External chunk listeners registered via `ExecutionService.AddChunkListener()` receive ALL chunk types including usage. Used by Feishu IM for real-time message forwarding.
- **10 TODO items**: Permission validation, message dedup, merge logic, artifact retrieval, GitLab API, Git query, markdown rendering, artifact preview — all marked TODO in code.
