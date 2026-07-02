# A2A (Agent-to-Agent) Routing

Handles @Agent mention routing, invocation queuing, and session chain management. Enables multi-agent collaboration within threads.

## Flow

```
Agent output contains @AgentName
  → MCP callback: /callbacks/post-message
  → MentionParser extracts targets
  → EnqueueA2ATargets() checks depth + dedup
  → InvocationQueue.Enqueue()
  → QueueProcessor.TryAutoExecute()
  → Orchestrator.SpawnAgent()
  → On completion: QueueProcessor.OnInvocationComplete() → next agent
```

## Files

| File | Role |
|------|------|
| `a2a_trigger.go` | `EnqueueA2ATargets()` — entry point. Depth check, dedup, queue/fallback |
| `mention_parser.go` | Parses @Agent patterns from message text |
| `invocation_queue.go` | Per-thread queue (max 5 entries per user). Enqueue/dequeue/peek |
| `queue_processor.go` | Auto-executes queued agents. Handles pause/resume |
| `invocation_registry.go` | Tracks active invocations. Slot-based mutex for concurrency |
| `session_chain_store.go` | Maps (threadID, agentCatID) → session chain. Supports sealing |
| `session_sealer.go` | Archives sessions when context is exhausted |
| `mcp_auth.go` | Token-based auth for MCP callbacks (ThreadID + InvocationID) |
| `worklist.go` | Worklist data structure for agent task tracking |
| `worklist_registry.go` | Registry of worklists across threads |

## Key Types

| Type | Role |
|------|------|
| `A2ATriggerDeps` | Dependencies: Orchestrator, WSHub, Queue |
| `A2ATriggerOptions` | Input: TargetCats, Content, ThreadID, CallerCatID, ParentInvocationID |
| `InvocationQueue` | Thread-scoped queue. `Enqueue()`, `Dequeue()`, `Peek()` |
| `QueueProcessor` | Monitors completion, triggers next. `TryAutoExecute()`, `OnInvocationComplete()` |
| `SessionChainStore` | Maps agent+thread to session chains for `--resume` support |
| `MCPAuthService` | `GenerateToken()` / `ValidateToken()` for callback auth |

## Constraints

- **Max depth**: 15 nested A2A calls (`MaxA2ADepth` in `execution_service.go`)
- **Max queue**: 5 entries per user per thread
- **Deduplication**: Same agent not re-queued if already active/queued for thread
- **Token auth**: MCP callbacks require one-time token (ThreadID + InvocationID pair)

## Where to Change

| Task | File |
|------|------|
| Modify routing logic | `a2a_trigger.go` — `EnqueueA2ATargets()` |
| Change queue limits | `invocation_queue.go` |
| Add mention patterns | `mention_parser.go` (patterns loaded from DB at startup) |
| Modify auto-execution | `queue_processor.go` — `TryAutoExecute()` |
| Change session sealing | `session_sealer.go` |
