# Agent Engine

Core agent execution system. Adapters wrap CLI tools (Claude, OpenCode via ACP protocol). Orchestrator dispatches. ExecutionService manages lifecycle.

## Architecture

```
adapter.go              → AgentAdapter interface (Execute, Stream, Session lifecycle)
adapter_registry.go     → Global plugin registry for adapter types
plugin_types.go         → PluginMeta, AdapterFactory types
orchestrator.go         → Central dispatch: SpawnAgent → ExecutionService
execution_service.go    → Lifecycle: timeout (10min), depth control, session caching, content block flush
claude_adapter.go       → Claude CLI: spawns process, parses streaming JSON
plugins/open_code/      → OpenCode ACP plugin package:
  - plugin.go           → Plugin registration (init)
  - adapter.go          → OpenCodeAdapter: renamed from OpenCodeACPAdapter
  - acp_adapter.go      → BaseACPAdapter: ACP protocol base
  - acp_jsonrpc.go      → acpTransport: bidirectional JSON-RPC 2.0 communication
  - acp_event_parser.go → Parses ACP session notifications into Chunk types
  - acp_types.go        → ACP protocol type definitions
  - logger.go           → Log helper delegation to agent package
workflow.go             → WorkflowEngine: phase transitions + validation
context_builder.go      → Builds ContextLayers for agent input
types.go                → Chunk, TokenUsage, ExecutionRequest/Result
tracker.go              → Tracks active agent invocations
logger.go               → Structured logging helpers (exported for plugins)
base_agent_service.go   → BaseAgent CRUD
config_service.go       → AgentRoleConfig CRUD
debug_thread_manager.go → Solo/debug mode thread management
```

## Key Types

| Type | Role |
|------|------|
| `AgentAdapter` | Interface: `Execute`, `ExecuteWithStream`, `StartSession`, `ResumeSession`, `StopSession`, `CheckHealth` |
| `ToolResultSender` | Interface: `SendToolResult` for AskUserQuestion handling |
| `Orchestrator` | Dispatches agents. Calls `GetAdapter(baseAgent)` → plugin registry selects adapter |
| `ExecutionService` | Manages A2AContext (depth, invoked agents), ThreadContext (workflow, transitions). Caches per-thread context. Content block flush with throttling. |
| `Chunk` | Streaming output: text, error, status, thinking, tool_use, tool_result, usage. Includes `Done` (thinking completion) and `IsError` fields. |
| `TokenUsage` | inputTokens, outputTokens, cacheReadTokens, costUsd, durationMs |
| `SessionStatus` | idle → running → completed/failed/stopped |
| `PluginMeta` | Plugin metadata: Type, Name, Description, Factory, ConfigDir, DefaultPath |

## Plugin Registry

```go
// Register plugin (in plugin init())
func RegisterPlugin(meta PluginMeta)

// Get adapter (in orchestrator)
func GetAdapter(baseAgent *model.BaseAgent) AgentAdapter

// Get plugin types (for API)
func GetTypes() []PluginTypeInfo
```

Plugins are registered via `init()` functions in `plugins/*/plugin.go`.
The global registry `globalRegistry` manages all adapter types.

## ACP Protocol (OpenCode Plugin)

The OpenCode plugin implements ACP (Agent Client Protocol) over stdio:

- `BaseACPAdapter`: ACP protocol base. JSON-RPC 2.0 over stdio. Session management, notification parsing.
- `OpenCodeAdapter`: Wraps BaseACPAdapter with OpenCode-specific CLI args and env vars.
- Env isolation: `OPENCODE_PURE=1` blocks user plugins; `OPENCODE_CONFIG_DIR` passes agent-specific configs.

## Execution Constants

- `agentExecutionTimeout` = 10 minutes
- `toolHeartbeatInterval` = 30 seconds
- `MaxA2ADepth` = 15

## Session Resumption

Agents support `--resume` to reuse CLI sessions, avoiding cold-start cost. `SessionID` flows through `ExecutionRequest` → adapter → CLI flag. Tracked via `SessionChainStore` in `a2a/` package.

## Workflow Phases

```
requirement → design → development → review → test → merge → complete
```

Transitions validated in `workflow.go`. Each phase restricts which next phases are valid.

## Where to Change

| Task | File |
|------|------|
| Add new CLI adapter | Create new plugin package in `plugins/*/`; implement `AgentAdapter` interface; call `RegisterPlugin()` in `init()` |
| Change execution timeout | `execution_service.go` constants |
| Modify phase transitions | `workflow.go` valid transitions map |
| Add streaming chunk type | `types.go` ChunkType constants |
| Change context building | `context_builder.go` |
| Modify content block flush | `execution_service.go` — `flushContentBlocks()`, `addToContentBlockBuffer()` |