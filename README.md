<p align="center">
  <h1 align="center">Colink</h1>
</p>

<p align="center">
  Multi-agent software development workbench for Claude, OpenCode, ACP agents, workflows, teams, and IM collaboration.
</p>

<p align="center">
  Agent Orchestration | Workflow Automation | A2A Collaboration | Skills & Commands | Feishu Integration | Web Workbench
</p>

<p align="center">
  <img alt="Version" src="https://img.shields.io/badge/version-1.0.0-blue">
  <img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-green">
  <img alt="Backend" src="https://img.shields.io/badge/backend-Go-00ADD8">
  <img alt="Frontend" src="https://img.shields.io/badge/frontend-React-61DAFB">
  <img alt="Database" src="https://img.shields.io/badge/database-SQLite%20%7C%20MySQL-orange">
</p>

---

## Quick Navigation

[What is Colink?](#what-is-colink) | [Core Capabilities](#core-capabilities) | [Architecture](#architecture) | [Quick Start](#quick-start) | [Database Migration](#database-migration) | [Development](#development) | [Q&A](#quick-qa)

---

## What is Colink?

**Colink is a multi-agent software development platform.** It provides a Go backend and React workbench UI for running, coordinating, and managing AI agents across development workflows.

Instead of treating an agent as a single chat window, Colink organizes agents as reusable execution units with skills, commands, workflow phases, A2A routing, WebSocket streaming, and optional IM access through Feishu.

| Capability | Traditional AI Chat | Colink |
| :-- | :-- | :-- |
| Agent execution | Single conversation | Multi-agent orchestration |
| Collaboration | Manual handoff | A2A routing and workflow coordination |
| Runtime | Chat-only | CLI adapters, ACP adapters, sandbox execution |
| UI | Simple chat | React workbench with teams, skills, commands, settings |
| Integration | Limited | Feishu webhook and IM bridge |
| Database | App-specific | SQLite local development, MySQL production support |


---

## Quick Start

### System Requirements

- **Go**: 1.25 or later
- **Node.js**: 20 or later
- **npm**
- **Make**
- **Database**: SQLite for local development, MySQL for production deployment

On Windows, use PowerShell or another terminal that can run `make`.

### 1. Enter the Project

```powershell
cd Colink
```

### 2. Build Backend Resources

```powershell
make build
make genplugins
go build -o bin/mcp-server.exe ./cmd/mcp-server
```

`make build` already depends on plugin generation, but running `make genplugins` explicitly is useful when you want to refresh the local plugin registry before starting the application. Build `mcp-server.exe` before starting the backend so MCP-related functionality is available at runtime.

### 3. Prepare Configuration

Create a local config file from the example when the template exists:

```powershell
if (Test-Path configs\config.yaml.example) {
  Copy-Item configs\config.yaml.example configs\config.yaml -Force
}
```

Then edit:

```text
configs\config.yaml
```

Update database, server, agent, IM, and other runtime settings as needed. If this workspace already provides `configs\config.yaml`, edit that file directly.

### 4. Run Database Migration

Build the migration tool:

```powershell
go build -o bin/migrate.exe ./cmd/migrate
```

Run migrations in order:

```powershell
.\bin\migrate.exe up --db .\data\sqlite\colink.db --version 1.1.0
.\bin\migrate.exe up --db .\data\sqlite\colink.db --version 1.2.0
.\bin\migrate.exe up --db .\data\sqlite\colink.db --version 1.2.2
.\bin\migrate.exe up --db .\data\sqlite\colink.db --version 1.2.3
.\bin\migrate.exe up --db .\data\sqlite\colink.db --version 1.2.4
.\bin\migrate.exe up --db .\data\sqlite\colink.db --version 1.2.5
.\bin\migrate.exe up --db .\data\sqlite\colink.db --version 1.2.7
```

Check current status:

```powershell
.\bin\migrate.exe status --db .\data\sqlite\colink.db
```

### 5. Start Backend

Development mode:

```powershell
make run
```

Or run the compiled server:

```powershell
.\bin\isdp-server
```

### 6. Start Frontend

Open another terminal:

```powershell
cd web
npm install
npm run dev
```

The frontend development server starts on port `3000` by default.

---

## Database Migration

Migration scripts live under:

```text
sql-change/v{version}/sqlite
```

Common commands:

```powershell
.\bin\migrate.exe status --db .\data\sqlite\colink.db
.\bin\migrate.exe version --db .\data\sqlite\colink.db
.\bin\migrate.exe up --db .\data\sqlite\colink.db --version 1.2.7
.\bin\migrate.exe down --db .\data\sqlite\colink.db --version 1.2.7
```

Add new migration directories for schema changes. Do not modify migration files that have already been applied in shared environments.

---

## Development

### Backend Commands

```powershell
make genplugins
make build
go build -o bin/mcp-server.exe ./cmd/mcp-server
make run
make test
```

### Frontend Commands

```powershell
cd web
npm install
npm run dev
npm run build
npm run lint
npm run test:e2e
```

### Auto-Test Commands

```powershell
make test-backend
make test-frontend
make test-all
python scripts/run-feature-tests.py --feature F001
python scripts/run-feature-tests.py --priority P0
```

---

## Configuration Notes

- Runtime config file: `configs\config.yaml`
- Local SQLite database: `data\sqlite\colink.db`
- Backend server entry: `cmd/server`
- Migration tool entry: `cmd/migrate`
- Frontend app: `web`

When changing the config schema, keep the Go config definitions and example configuration in sync.

---

## Quick Q&A

**Q: Do I need MySQL for local development?**  
A: No. Local development can use SQLite through `data\sqlite\colink.db`. MySQL is supported for production-style deployments.

**Q: Why do I need to run migrations manually?**  
A: The SQLite database must be upgraded through versioned migration directories under `sql-change/` so the schema matches the current backend.

**Q: Does `make build` generate plugins?**  
A: Yes. The `build` target depends on `genplugins`. You can still run `make genplugins` manually to refresh plugin registry files.

**Q: Which ports are used by default?**  
A: The frontend runs on `3000` by default. The backend port is controlled by `configs\config.yaml`.

**Q: Is Feishu an agent adapter?**  
A: No. Feishu is an IM entry point. Agent execution still goes through `Orchestrator -> ExecutionService -> Adapter`.

---

## License

This project is licensed under [Apache-2.0](LICENSE).
