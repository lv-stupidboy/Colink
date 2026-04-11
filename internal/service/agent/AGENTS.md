# Agent Engine

Core agent execution system. Adapters wrap CLI tools (Claude, OpenCode, ACP protocol). Orchestrator dispatches. ExecutionService manages lifecycle.

## Architecture

```
adapter.go              → AgentAdapter interface (Execute, Stream, Session lifecycle)
orchestrator.go         → Central dispatch: SpawnAgent → ExecutionService
execution_service.go    → Lifecycle: timeout (10min), depth control, session caching, content block flush
claude_adapter.go       → Claude CLI: spawns process, parses streaming JSON
open_code_adapter.go    → OpenCode CLI: same pattern, different parser
acp_adapter.go          → BaseACPAdapter: ACP protocol (JSON-RPC 2.0 over stdio)
acp_jsonrpc.go          → acpTransport: bidirectional JSON-RPC 2.0 communication
acp_event_parser.go     → Parses ACP session notifications into Chunk types
acp_types.go            → ACP protocol type definitions
open_code_acp_adapter.go → OpenCodeACPAdapter: extends BaseACPAdapter with OpenCode CLI args
workflow.go             → WorkflowEngine: phase transitions + validation
context_builder.go      → Builds ContextLayers for agent input
types.go                → Chunk, TokenUsage, ExecutionRequest/Result
tracker.go              → Tracks active agent invocations
logger.go               → Structured logging helpers
base_agent_service.go   → BaseAgent CRUD
config_service.go       → AgentRoleConfig CRUD
debug_thread_manager.go → Solo/debug mode thread management
```

## Key Types

| Type | Role |
|------|------|
| `AgentAdapter` | Interface: `Execute`, `ExecuteWithStream`, `StartSession`, `ResumeSession`, `StopSession`, `CheckHealth` |
| `Orchestrator` | Dispatches agents. Calls `NewAdapter(baseAgent)` → adapter factory selects Claude, OpenCode, or OpenCodeACP |
| `ExecutionService` | 2303 lines. Manages A2AContext (depth, invoked agents), ThreadContext (workflow, transitions). Caches per-thread context. Content block flush with throttling. |
| `Chunk` | Streaming output: text, error, status, thinking, tool_use, tool_result, usage. Includes `Done` (thinking completion) and `IsError` fields. |
| `TokenUsage` | inputTokens, outputTokens, cacheReadTokens, costUsd, durationMs |
| `SessionStatus` | idle → running → completed/failed/stopped |
| `BaseACPAdapter` | ACP protocol base. JSON-RPC 2.0 over stdio. Session management, notification parsing. Sets `OPENCODE_PURE=1` in `buildEnv()` to isolate from user-level plugins. |
| `OpenCodeACPAdapter` | Wraps BaseACPAdapter with OpenCode-specific CLI args and `OPENCODE_CONFIG_DIR` for per-agent config (agents/skills/rules/commands). |

## Adapter Factory

```go
func NewAdapter(baseAgent *model.BaseAgent) AgentAdapter {
    switch baseAgent.Type {
    case model.BaseAgentTypeClaudeCode:  return NewClaudeAdapter(baseAgent)
    case model.BaseAgentTypeOpenCode:    return NewOpenCodeAdapter(baseAgent)
    case model.BaseAgentTypeOpenCodeACP: return NewOpenCodeACPAdapter(baseAgent)
    }
}
```

Claude/OpenCode adapters: `sessions map[string]*session`, `sync.RWMutex`, spawn CLI via `exec.Cmd`.
ACP adapters: `BaseACPAdapter` manages stdio transport, `OpenCodeACPAdapter` adds OpenCode CLI configuration. Env isolation: `OPENCODE_PURE=1` blocks user plugins; `OPENCODE_CONFIG_DIR` passes agent-specific configs that bypass the PURE gate.

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
| Add new CLI adapter | Implement `AgentAdapter` interface; register in `NewAdapter()` |
| Add ACP-based adapter | Extend `BaseACPAdapter` (see `open_code_acp_adapter.go` as example); register in `NewAdapter()` |
| Change execution timeout | `execution_service.go` constants |
| Modify phase transitions | `workflow.go` valid transitions map |
| Add streaming chunk type | `types.go` ChunkType constants |
| Change context building | `context_builder.go` |
| Modify content block flush | `execution_service.go` — `flushContentBlocks()`, `addToContentBlockBuffer()` |
