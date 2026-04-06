# Agent Engine

Core agent execution system. Adapters wrap CLI tools (Claude, OpenCode). Orchestrator dispatches. ExecutionService manages lifecycle.

## Architecture

```
adapter.go              → AgentAdapter interface (Execute, Stream, Session lifecycle)
orchestrator.go         → Central dispatch: SpawnAgent → ExecutionService
execution_service.go    → Lifecycle: timeout (10min), depth control, session caching
claude_adapter.go       → Claude CLI: spawns process, parses streaming JSON
open_code_adapter.go    → OpenCode CLI: same pattern, different parser
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
| `Orchestrator` | Dispatches agents. Calls `NewAdapter(baseAgent)` → adapter factory selects Claude or OpenCode |
| `ExecutionService` | 1691 lines. Manages A2AContext (depth, invoked agents), ThreadContext (workflow, transitions). Caches per-thread context. |
| `Chunk` | Streaming output: text, error, status, thinking, tool_use, tool_result, usage |
| `TokenUsage` | inputTokens, outputTokens, cacheReadTokens, costUsd, durationMs |
| `SessionStatus` | idle → running → completed/failed/stopped |

## Adapter Factory

```go
func NewAdapter(baseAgent *model.BaseAgent) AgentAdapter {
    switch baseAgent.Type {
    case model.BaseAgentTypeClaudeCode:  return NewClaudeAdapter(baseAgent)
    case model.BaseAgentTypeOpenCode:    return NewOpenCodeAdapter(baseAgent)
    }
}
```

Both adapters: `sessions map[string]*session`, `sync.RWMutex`, spawn CLI via `exec.Cmd`.

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
| Change execution timeout | `execution_service.go` constants |
| Modify phase transitions | `workflow.go` valid transitions map |
| Add streaming chunk type | `types.go` ChunkType constants |
| Change context building | `context_builder.go` |
