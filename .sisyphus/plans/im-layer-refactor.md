# Colink IM Layer Refactoring — Reliability, Abstraction, Extensibility

## TL;DR

> **Quick Summary**: Refactor the Colink IM integration layer from a Feishu-only monolith into a reliable, platform-agnostic bridge with adapter pattern. Phase 1 fixes bugs and adds safety nets (dedup, rate limiting, retry). Phase 2 extracts a platform interface and delivery layer. Phase 3 scaffolds multi-platform support.
>
> **Deliverables**:
> - Phase 1: Files moved to correct path, 3 bugs fixed, dedup + rate limiter + retry added, full test coverage
> - Phase 2: IMAdapter interface, generic IMBridgeService, Delivery Layer, FeishuAdapter, session locks, streaming card support
> - Phase 3: Adapter registry, Slack/Discord stubs, input validation, multi-platform config
>
> **Estimated Effort**: Large (5-7 days)
> **Parallel Execution**: YES — 6 waves
> **Critical Path**: T1 → T3 → T6 → T10 → T14 → T17 → T19 → Final

---

## Context

### Original Request
Refactor the Colink IM integration layer for reliability, platform abstraction, and extensibility, referencing patterns from the open-source [Claude-to-IM](https://github.com/op7418/Claude-to-IM) project. Three-phase approach agreed with user.

### Current State
- **Stranded files**: 4 IM service files at wrong path `isdp/internal/service/im/` (nested `isdp/` directory)
- **Correct-path files**: Webhook handler, model, repo already at `internal/`
- **LSP errors**: `main.go` and `feishu_webhook_handler.go` have broken imports due to stranded package
- **Test coverage**: Only `feishu_types_test.go` (type parsing) and `im_session_repo_test.go` (CRUD). Zero tests for bridge service, CLI client, webhook handler, chunk buffering.
- **Confirmed bugs**: Context leak in webhook handler (line 65), buffer flush race in bridge service
- **No reliability features**: No dedup, no rate limiting, no retry, no error classification

### Research Findings

**Codebase Patterns** (from explore agents):
- Agent adapters use factory function with type switch (`NewAdapter()`), not global registry
- Error handling: `fmt.Errorf("context: %w", err)` + zap structured logging
- Tests: standard Go `testing`, table-driven tests, in-memory SQLite for DB
- ChunkListener: `func(threadID, invocationID uuid.UUID, chunk Chunk, agentID, agentName string)` — FROZEN
- WebSocket hub: channel-based broadcast per threadID

**Claude-to-IM Patterns** (from librarian agent):
- Adapter Registry: `sync.Map` + factory functions with self-registration
- Delivery Layer: `chunkText()` (split at newlines), `classifyError()` (5 categories), exponential backoff with jitter, HTML→plaintext fallback
- Session Lock: Promise chaining → Go equivalent: per-session channel serialization
- LRU Dedup: 1000-entry in-memory Map with oldest-first eviction
- Rate Limiter: Sliding window (timestamp array) per chat, blocking acquire
- Streaming Cards: CardKit v2 with 200ms throttle timer, sequence numbering, trailing-edge flush
- Input Validation: regex patterns for null bytes, path traversal, command injection

### Self-Performed Gap Analysis (Metis unavailable)

**Gaps identified and addressed:**
1. **Graceful shutdown**: IM service must drain in-flight messages on SIGTERM → Added to Phase 1
2. **Metrics/observability**: No mention of metrics for dedup hits, rate limit blocks, retry attempts → Added counters via zap logging (structured fields for future Prometheus export)
3. **Config hot-reload**: Rate limit params should be configurable without restart → Deferred (config is YAML-loaded at startup per codebase convention)
4. **lark-cli connection pooling**: Per-message process spawn (~100ms) identified as problem #9 → Phase 2 addresses via persistent process option in delivery layer
5. **Streaming card error recovery**: What if card creation succeeds but update fails mid-stream? → Added fallback: stop updates, send final text message
6. **Multi-adapter ChunkListener routing**: When multiple IM platforms are active, same ChunkListener fires for ALL → Need routing by threadID→platform in bridge service
7. **Database migration for Phase 2**: IMSession table may need `platform_adapter_config` column → Evaluated: not needed, config lives in YAML per platform
8. **Backward compatibility**: Phase 2 refactor must not break existing Feishu webhook URL or behavior → Explicit guardrail added

---

## Work Objectives

### Core Objective
Transform the IM integration layer from a single-platform monolith into a reliable, testable, platform-agnostic bridge that handles message delivery failures gracefully and supports adding new IM platforms with minimal code.

### Concrete Deliverables

**Phase 1 — Reliability Foundation:**
- Files moved from `isdp/internal/service/im/` to `internal/service/im/`
- Context bug fixed (detached context for goroutine)
- Buffer flush race fixed (copy-on-flush pattern)
- LRU dedup cache (1000 entries, thread-safe)
- Sliding window rate limiter (configurable msg/min/session)
- Retry with exponential backoff + 5-class error classification
- Tests for all new code + existing untested code

**Phase 2 — Platform Abstraction:**
- `IMAdapter` interface definition
- Generic `IMBridgeService` (platform-agnostic orchestrator)
- `DeliveryService` (chunkText, classifyError, retry, fallback)
- `FeishuAdapter` implementing `IMAdapter`
- Per-session locks (channel-based serialization)
- Streaming card support (CardKit v2 with throttle)
- Config schema for multi-platform IM

**Phase 3 — Extensibility Scaffolding:**
- Adapter factory with type-based dispatch (matching codebase pattern)
- `SlackAdapter` stub (interface satisfied, methods return `ErrNotImplemented`)
- `DiscordAdapter` stub (same)
- Input validation module (dangerous patterns, max length)
- Updated config.yaml.example for multi-platform

### Definition of Done
- [ ] `make test` passes with zero failures
- [ ] `go vet ./...` reports zero issues
- [ ] All new code has corresponding `_test.go` files
- [ ] Feishu webhook → agent → Feishu reply flow works end-to-end
- [ ] No LSP import errors in `main.go` or `feishu_webhook_handler.go`
- [ ] Existing API contract unchanged (webhook URL, request/response format)

### Must Have
- Zero downtime for existing Feishu integration during refactoring
- ChunkListener signature unchanged (`types.go:47-48`)
- All imports compile (no stranded path references remain)
- Tests for every new public function
- Atomic commits per logical change (see Commit Strategy)

### Must NOT Have (Guardrails)
- **No external test dependencies**: Do NOT add testify, gomock, or any third-party test library — match existing codebase patterns (standard `testing` package)
- **No ORM**: All new DB queries must use raw SQL via `database/sql`
- **No global mutable state**: No package-level `var` maps without sync protection; use constructor injection
- **No breaking API changes**: Webhook URL `/api/v1/feishu/webhook` must remain unchanged
- **No Slack/Discord implementation**: Phase 3 stubs are interface-only, returning `ErrNotImplemented`
- **No premature abstraction**: Don't abstract things that only have one implementation yet (except the IMAdapter interface which has a clear second-platform purpose)
- **No lark-cli SDK replacement**: Keep lark-cli as external process for Feishu (too much scope to replace with SDK)
- **No frontend changes**: This is backend-only
- **No changes to `execution_service.go` ChunkListener mechanism**: Use `AddChunkListener()` as-is
- **No `as any` or `@ts-ignore` equivalents**: No `interface{}` abuse, no skipping error checks

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (standard Go `testing`, `make test`)
- **Automated tests**: TDD — write failing test first, then implement
- **Framework**: Standard `testing` package, table-driven tests
- **DB tests**: In-memory SQLite `:memory:` (matching existing `im_session_repo_test.go` pattern)

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Bug fixes**: Write regression test that fails before fix, passes after
- **New modules** (dedup, rate limiter, delivery): Table-driven unit tests + integration test
- **Refactors** (Phase 2): Verify existing Feishu flow still works via curl to webhook endpoint
- **Config changes**: Verify `go build ./...` succeeds and config loads

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — start immediately, MAX PARALLEL):
├── T1: Move stranded files + fix imports [quick]
├── T2: Fix webhook context bug [quick]
├── T3: Fix buffer flush race condition [quick]
├── T4: LRU dedup cache module [quick]
├── T5: Sliding window rate limiter module [quick]
└── T6: Error classification module [quick]

Wave 2 (Integration — after Wave 1):
├── T7: Retry + delivery reliability (depends: T6) [deep]
├── T8: Integrate dedup into bridge service (depends: T1, T4) [quick]
├── T9: Integrate rate limiter into bridge service (depends: T1, T5) [quick]
└── T10: Add bridge service + webhook handler tests (depends: T1, T2, T3) [unspecified-high]

Wave 3 (Abstraction — after Wave 2):
├── T11: Define IMAdapter interface + types (depends: T10) [deep]
├── T12: Build DeliveryService (depends: T7) [deep]
└── T13: Per-session lock module (depends: T10) [quick]

Wave 4 (Platform refactor — after Wave 3):
├── T14: Extract FeishuAdapter from bridge service (depends: T11, T12, T13) [deep]
├── T15: Build generic IMBridgeService (depends: T11, T13) [deep]
└── T16: Streaming card support for Feishu (depends: T14) [unspecified-high]

Wave 5 (Rewire + config — after Wave 4):
├── T17: Rewire main.go to new architecture (depends: T14, T15) [deep]
├── T18: Multi-platform config schema (depends: T17) [quick]
└── T19: Integration tests for full flow (depends: T17) [unspecified-high]

Wave 6 (Extensibility — after Wave 5):
├── T20: Adapter factory + registry (depends: T17, T18) [quick]
├── T21: Slack adapter stub (depends: T11, T20) [quick]
├── T22: Discord adapter stub (depends: T11, T20) [quick]
└── T23: Input validation module (depends: T17) [quick]

Wave FINAL (after ALL tasks — 4 parallel reviews, then user okay):
├── TF1: Plan compliance audit (oracle)
├── TF2: Code quality review (unspecified-high)
├── TF3: Real manual QA (unspecified-high)
└── TF4: Scope fidelity check (deep)
→ Present results → Get explicit user okay
```

**Critical Path**: T1 → T3 → T10 → T11 → T14 → T17 → T19 → TF1-TF4 → user okay
**Parallel Speedup**: ~65% faster than sequential
**Max Concurrent**: 6 (Wave 1)

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | T8, T9, T10 | 1 |
| T2 | — | T10 | 1 |
| T3 | — | T10 | 1 |
| T4 | — | T8 | 1 |
| T5 | — | T9 | 1 |
| T6 | — | T7 | 1 |
| T7 | T6 | T12 | 2 |
| T8 | T1, T4 | T14 | 2 |
| T9 | T1, T5 | T14 | 2 |
| T10 | T1, T2, T3 | T11, T13 | 2 |
| T11 | T10 | T14, T15, T21, T22 | 3 |
| T12 | T7 | T14 | 3 |
| T13 | T10 | T14, T15 | 3 |
| T14 | T11, T12, T13 | T16, T17 | 4 |
| T15 | T11, T13 | T17 | 4 |
| T16 | T14 | T17 | 4 |
| T17 | T14, T15, T16 | T18, T19, T20, T23 | 5 |
| T18 | T17 | T20 | 5 |
| T19 | T17 | — | 5 |
| T20 | T17, T18 | T21, T22 | 6 |
| T21 | T11, T20 | — | 6 |
| T22 | T11, T20 | — | 6 |
| T23 | T17 | — | 6 |

### Agent Dispatch Summary

| Wave | Tasks | Categories |
|------|-------|-----------|
| 1 | 6 | T1-T6 → `quick` |
| 2 | 4 | T7 → `deep`, T8-T9 → `quick`, T10 → `unspecified-high` |
| 3 | 3 | T11 → `deep`, T12 → `deep`, T13 → `quick` |
| 4 | 3 | T14 → `deep`, T15 → `deep`, T16 → `unspecified-high` |
| 5 | 3 | T17 → `deep`, T18 → `quick`, T19 → `unspecified-high` |
| 6 | 4 | T20-T23 → `quick` |
| FINAL | 4 | TF1 → `oracle`, TF2 → `unspecified-high`, TF3 → `unspecified-high`, TF4 → `deep` |

---

## TODOs

### Phase 1 — Reliability Foundation

- [x] 1. Move Stranded IM Files to Correct Path

  **What to do**:
  - Move all 4 files from `isdp/internal/service/im/` to `internal/service/im/`:
    - `feishu_bridge_service.go`
    - `feishu_types.go`
    - `feishu_types_test.go`
    - `lark_cli_client.go`
  - Update package import path in every file that references the old path:
    - `cmd/server/main.go` (line 23): change import from `github.com/anthropic/isdp/isdp/internal/service/im` → `github.com/anthropic/isdp/internal/service/im`
    - `internal/api/feishu_webhook_handler.go` (line 8): same import fix
    - Check for any other files importing the stranded path
  - Remove the now-empty `isdp/internal/service/im/` directory
  - Verify `go build ./...` succeeds with zero errors

  **Must NOT do**:
  - Do NOT modify any logic inside the moved files — this is a pure path move
  - Do NOT rename any structs, functions, or types
  - Do NOT change package name (it should remain `package im`)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2, T3, T4, T5, T6)
  - **Blocks**: T8, T9, T10
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `isdp/internal/service/im/` — the 4 stranded files to move (verify exact contents before moving)

  **API/Type References**:
  - `cmd/server/main.go:23` — import that references stranded path (currently broken, LSP confirms)
  - `internal/api/feishu_webhook_handler.go:8` — import that references stranded path (currently broken)

  **External References**:
  - None

  **WHY Each Reference Matters**:
  - The LSP diagnostics confirm both `main.go` and `feishu_webhook_handler.go` have broken imports due to the stranded path. Moving files fixes compilation.
  - The `isdp/` prefix directory is a historical artifact from directory flattening (documented in AGENTS.md).

  **Acceptance Criteria**:
  - [ ] `go build ./...` succeeds with zero errors
  - [ ] `ls isdp/internal/service/im/` returns "no such file or directory"
  - [ ] `ls internal/service/im/` shows all 4 files
  - [ ] `go test ./internal/service/im/...` passes (feishu_types_test.go)

  **QA Scenarios**:

  ```
  Scenario: Build succeeds after file move
    Tool: Bash
    Preconditions: Files at stranded path, broken LSP imports
    Steps:
      1. Run `go build ./...`
      2. Assert exit code is 0
      3. Run `go vet ./...`
      4. Assert exit code is 0
    Expected Result: Both commands succeed with zero output
    Failure Indicators: Any "cannot find package" or import errors
    Evidence: .sisyphus/evidence/task-1-build-success.txt

  Scenario: Stranded directory is removed
    Tool: Bash
    Preconditions: Files moved to correct path
    Steps:
      1. Run `ls isdp/internal/service/im/ 2>&1`
      2. Assert output contains "No such file or directory"
      3. Run `ls internal/service/im/`
      4. Assert output contains feishu_bridge_service.go, feishu_types.go, feishu_types_test.go, lark_cli_client.go
    Expected Result: Old directory gone, new directory has all files
    Failure Indicators: Old directory still exists or files missing from new location
    Evidence: .sisyphus/evidence/task-1-directory-check.txt

  Scenario: No remaining references to stranded path
    Tool: Bash
    Preconditions: All imports updated
    Steps:
      1. Run `grep -r "isdp/internal/service/im" --include="*.go" .`
      2. Assert zero matches (exit code 1)
    Expected Result: No Go file references the old path
    Failure Indicators: Any grep match
    Evidence: .sisyphus/evidence/task-1-no-stale-refs.txt
  ```

  **Commit**: YES (C1)
  - Message: `refactor(im): move stranded IM files to correct path`
  - Files: `internal/service/im/*.go`, `cmd/server/main.go`, `internal/api/feishu_webhook_handler.go`
  - Pre-commit: `go build ./...`

- [x] 2. Fix Webhook Context Bug

  **What to do**:
  - In `internal/api/feishu_webhook_handler.go`, line 65: `go h.bridgeSvc.HandleMessageEvent(c.Request.Context(), msgEvent)` — the goroutine captures the request context which gets cancelled when the HTTP handler returns 200.
  - **Fix**: Create a detached context that inherits no cancellation from the request:
    ```go
    // Replace: go h.bridgeSvc.HandleMessageEvent(c.Request.Context(), msgEvent)
    // With:
    detachedCtx := context.WithoutCancel(c.Request.Context())
    go h.bridgeSvc.HandleMessageEvent(detachedCtx, msgEvent)
    ```
  - `context.WithoutCancel` is available in Go 1.21+ (we're on 1.25). It returns a context that is never cancelled but carries the same values.
  - Write a test that verifies the handler returns 200 immediately and the bridge service is invoked asynchronously.

  **Must NOT do**:
  - Do NOT use `context.Background()` — it loses request-scoped values (like trace IDs)
  - Do NOT add a timeout here — timeout belongs in the bridge service, not the handler
  - Do NOT change the webhook response format

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T4, T5, T6)
  - **Blocks**: T10
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/api/feishu_webhook_handler.go:65` — the line with the context bug
  - `internal/api/feishu_webhook_handler.go:31-71` — full `HandleWebhook()` method for context

  **API/Type References**:
  - Go stdlib `context.WithoutCancel()` — available since Go 1.21

  **WHY Each Reference Matters**:
  - Line 65 is the exact bug location. The goroutine outlives the request but its context is tied to the request lifecycle.
  - `context.WithoutCancel` is the idiomatic Go 1.21+ solution — creates a derived context that carries values but ignores cancellation.

  **Acceptance Criteria**:
  - [ ] `HandleWebhook()` returns 200 before bridge service completes
  - [ ] Bridge service goroutine's context is not cancelled when handler returns
  - [ ] Test file created: `internal/api/feishu_webhook_handler_test.go`

  **QA Scenarios**:

  ```
  Scenario: Handler returns 200 immediately
    Tool: Bash
    Preconditions: Webhook handler compiled, test written
    Steps:
      1. Run `go test ./internal/api/ -run TestHandleWebhook -v`
      2. Assert test passes
      3. Verify test asserts HTTP 200 response
    Expected Result: Test passes, handler returns 200 without waiting for bridge service
    Failure Indicators: Test fails or handler blocks on bridge service completion
    Evidence: .sisyphus/evidence/task-2-handler-test.txt

  Scenario: Context is not cancelled after handler returns
    Tool: Bash
    Preconditions: Test uses a mock bridge service that checks context cancellation
    Steps:
      1. Run `go test ./internal/api/ -run TestContextNotCancelled -v`
      2. Assert test passes
    Expected Result: Detached context remains valid after HTTP handler returns
    Failure Indicators: context.Err() returns non-nil in the goroutine
    Evidence: .sisyphus/evidence/task-2-context-test.txt
  ```

  **Commit**: YES (groups with T3 → C2)
  - Message: `fix(im): resolve webhook context leak and buffer flush race`
  - Files: `internal/api/feishu_webhook_handler.go`, `internal/api/feishu_webhook_handler_test.go`
  - Pre-commit: `go test ./internal/api/... ./internal/service/im/...`

- [x] 3. Fix Buffer Flush Race Condition

  **What to do**:
  - In `internal/service/im/feishu_bridge_service.go`, the `accumulateAndFlush()` method (lines 248-283) has a race condition: the timer callback captures a pointer to the buffer (`buf`), but the buffer may be reused or mutated between timer creation and callback execution.
  - **Fix**: Use copy-on-flush pattern — when the timer fires, lock the mutex, copy the buffer content, reset the buffer, unlock, then send the copy:
    ```go
    func (s *FeishuBridgeService) accumulateAndFlush(chatID, invocationID, text string) {
        s.bufMu.Lock()
        key := chatID + ":" + invocationID
        buf, exists := s.buffers[key]
        if !exists {
            buf = &chunkBuffer{content: strings.Builder{}}
            s.buffers[key] = buf
        }
        buf.content.WriteString(text)

        // Cancel existing timer
        if buf.timer != nil {
            buf.timer.Stop()
        }

        // Copy content for threshold check
        currentLen := buf.content.Len()
        
        if currentLen >= 200 {
            // Flush immediately: copy and reset under lock
            flushed := buf.content.String()
            buf.content.Reset()
            s.bufMu.Unlock()
            s.sendText(chatID, flushed)
            return
        }

        // Set debounce timer: capture content snapshot in closure
        buf.timer = time.AfterFunc(500*time.Millisecond, func() {
            s.bufMu.Lock()
            b, ok := s.buffers[key]
            if !ok || b.content.Len() == 0 {
                s.bufMu.Unlock()
                return
            }
            flushed := b.content.String()
            b.content.Reset()
            s.bufMu.Unlock()
            s.sendText(chatID, flushed)
        })
        s.bufMu.Unlock()
    }
    ```
  - Add a dedicated `sync.Mutex` field `bufMu` to `FeishuBridgeService` struct if not already present (check current implementation for existing synchronization)
  - Write tests that exercise concurrent buffer writes and verify no data loss or panic

  **Must NOT do**:
  - Do NOT change the flush thresholds (200 chars / 500ms) — keep existing behavior
  - Do NOT change the `chunkBuffer` struct layout unless needed for the fix
  - Do NOT add channel-based buffering yet — that's Phase 2

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T4, T5, T6)
  - **Blocks**: T10
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/service/im/feishu_bridge_service.go:248-283` — current `accumulateAndFlush()` implementation (AFTER T1 moves the file)
  - `internal/service/im/feishu_bridge_service.go:42-47` — `chunkBuffer` struct definition
  - `internal/service/im/feishu_bridge_service.go:286-310` — current `flushBufferLocked()` (may be folded into new implementation)

  **WHY Each Reference Matters**:
  - Lines 248-283 contain the exact race: timer callback captures `buf` pointer but doesn't lock before accessing content
  - The fix must preserve existing debounce behavior (200-char threshold + 500ms timer) while eliminating the race

  **Acceptance Criteria**:
  - [ ] No data race detected by `go test -race ./internal/service/im/...`
  - [ ] Buffer flush produces correct output under concurrent writes
  - [ ] Test file: `internal/service/im/feishu_bridge_service_test.go` with race tests

  **QA Scenarios**:

  ```
  Scenario: No data race under concurrent writes
    Tool: Bash
    Preconditions: Buffer race fix applied
    Steps:
      1. Run `go test -race ./internal/service/im/ -run TestAccumulateAndFlush -v -count=5`
      2. Assert exit code 0
      3. Assert no "DATA RACE" in output
    Expected Result: 5 runs with zero race detector warnings
    Failure Indicators: "WARNING: DATA RACE" in output
    Evidence: .sisyphus/evidence/task-3-race-test.txt

  Scenario: Buffer content not lost under concurrent flush
    Tool: Bash
    Preconditions: Test writes 10 chunks concurrently, verifies all content delivered
    Steps:
      1. Run `go test ./internal/service/im/ -run TestBufferNoDataLoss -v`
      2. Assert all 10 chunk contents appear in flushed output
    Expected Result: Total flushed content equals total written content
    Failure Indicators: Missing chunks or garbled content
    Evidence: .sisyphus/evidence/task-3-no-data-loss.txt
  ```

  **Commit**: YES (groups with T2 → C2)
  - Message: `fix(im): resolve webhook context leak and buffer flush race`
  - Files: `internal/service/im/feishu_bridge_service.go`, `internal/service/im/feishu_bridge_service_test.go`
  - Pre-commit: `go test -race ./internal/service/im/...`

- [x] 4. LRU Dedup Cache Module

  **What to do**:
  - Create new file `internal/service/im/dedup.go` containing a thread-safe LRU dedup cache:
    ```go
    type DedupCache struct {
        mu   sync.RWMutex
        data map[string]struct{}
        keys []string // ordered by insertion time (oldest first)
        max  int
    }

    func NewDedupCache(maxEntries int) *DedupCache
    func (c *DedupCache) IsDuplicate(key string) bool    // Check + insert atomically; returns true if already seen
    func (c *DedupCache) Len() int
    ```
  - `IsDuplicate(key)`: If key exists → return true (duplicate). If key doesn't exist → insert it, evict oldest if over max, return false (new).
  - Eviction: when `len(keys) > max`, remove `keys[0]` from both map and slice.
  - Thread-safety: `sync.RWMutex` — `RLock` for read-only check (fast path), `Lock` for insert+evict.
  - Create test file `internal/service/im/dedup_test.go` with table-driven tests:
    - Basic dedup (same key returns true on second call)
    - Eviction (insert max+1 entries, first key no longer detected)
    - Concurrent access (goroutines calling IsDuplicate simultaneously)
    - Empty/zero max edge case

  **Must NOT do**:
  - Do NOT use external LRU library (e.g., `hashicorp/golang-lru`) — keep zero dependencies
  - Do NOT persist to database — this is in-memory only
  - Do NOT integrate into bridge service yet — that's T8

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T5, T6)
  - **Blocks**: T8
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - Claude-to-IM `feishu-adapter.ts:111-136` — `seenMessageIds` Map with LRU eviction pattern
  - Claude-to-IM `delivery-layer.ts:147-157` — `checkDedup()` / `insertDedup()` store interface

  **Test References**:
  - `internal/service/a2a/invocation_queue_test.go` — table-driven test pattern with edge cases (codebase example)
  - `internal/repo/im_session_repo_test.go` — concurrent access testing pattern

  **WHY Each Reference Matters**:
  - Claude-to-IM's dedup pattern is the design reference — 1000 max entries, oldest-first eviction
  - Existing test files show the project's preferred testing style (table-driven, `t.Run`, manual assertions)

  **Acceptance Criteria**:
  - [ ] `internal/service/im/dedup.go` exists with `DedupCache` struct
  - [ ] `internal/service/im/dedup_test.go` passes with `go test -race`
  - [ ] At least 4 test cases: basic, eviction, concurrent, edge case

  **QA Scenarios**:

  ```
  Scenario: Dedup correctly identifies duplicates
    Tool: Bash
    Preconditions: dedup.go and dedup_test.go exist
    Steps:
      1. Run `go test ./internal/service/im/ -run TestDedupCache -v`
      2. Assert all subtests pass
    Expected Result: IsDuplicate returns false first time, true second time for same key
    Failure Indicators: Test failures or unexpected return values
    Evidence: .sisyphus/evidence/task-4-dedup-test.txt

  Scenario: No race conditions under concurrent access
    Tool: Bash
    Preconditions: Concurrent test case exists
    Steps:
      1. Run `go test -race ./internal/service/im/ -run TestDedupConcurrent -v -count=3`
      2. Assert zero "DATA RACE" warnings
    Expected Result: 3 runs with no race detector warnings
    Failure Indicators: "WARNING: DATA RACE"
    Evidence: .sisyphus/evidence/task-4-dedup-race.txt
  ```

  **Commit**: YES (groups with T5, T6 → C3)
  - Message: `feat(im): add dedup cache, rate limiter, and error classification`
  - Files: `internal/service/im/dedup.go`, `internal/service/im/dedup_test.go`
  - Pre-commit: `go test -race ./internal/service/im/...`

- [x] 5. Sliding Window Rate Limiter Module

  **What to do**:
  - Create new file `internal/service/im/rate_limiter.go`:
    ```go
    type RateLimiter struct {
        mu          sync.Mutex
        buckets     map[string]*bucket
        maxMessages int
        window      time.Duration
    }

    type bucket struct {
        timestamps []int64 // Unix milliseconds
    }

    func NewRateLimiter(maxMessages int, window time.Duration) *RateLimiter
    func (r *RateLimiter) Acquire(chatID string)  // Blocks until allowed; prunes expired timestamps
    func (r *RateLimiter) TryAcquire(chatID string) bool  // Non-blocking; returns false if rate limited
    ```
  - `Acquire(chatID)`: prune timestamps older than window, if under limit → append and return, if at limit → calculate wait time until oldest expires → `time.Sleep(waitMs)` → re-prune → append.
  - `TryAcquire(chatID)`: same logic but returns `false` instead of blocking.
  - Default: 20 messages per 60 seconds per chat (configurable via constructor).
  - Create test file `internal/service/im/rate_limiter_test.go`:
    - Under-limit: 5 calls succeed without blocking
    - At-limit: 21st call blocks (use short window for test speed)
    - TryAcquire returns false at limit
    - Different chatIDs have independent buckets
    - Window expiry: old timestamps pruned correctly

  **Must NOT do**:
  - Do NOT use a third-party rate limiter library
  - Do NOT use token bucket algorithm — sliding window is simpler and sufficient
  - Do NOT integrate into bridge service yet — that's T9

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T4, T6)
  - **Blocks**: T9
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - Claude-to-IM `rate-limiter.ts:12-52` — `ChatRateLimiter` with sliding window, `acquire()` blocking pattern, `pruneOld()` cleanup

  **WHY Each Reference Matters**:
  - Claude-to-IM's rate limiter is the exact design reference. Go translation needs `sync.Mutex` instead of JS single-thread, and `time.Sleep` instead of `setTimeout`.

  **Acceptance Criteria**:
  - [ ] `internal/service/im/rate_limiter.go` exists with `RateLimiter` struct
  - [ ] `internal/service/im/rate_limiter_test.go` passes
  - [ ] At least 5 test cases covering all behaviors
  - [ ] Uses short windows (10ms) in tests for speed

  **QA Scenarios**:

  ```
  Scenario: Rate limiter allows messages under limit
    Tool: Bash
    Preconditions: rate_limiter.go and test exist
    Steps:
      1. Run `go test ./internal/service/im/ -run TestRateLimiter -v`
      2. Assert all subtests pass
    Expected Result: Messages under limit proceed without blocking
    Failure Indicators: Unexpected blocking or test timeout
    Evidence: .sisyphus/evidence/task-5-ratelimit-test.txt

  Scenario: Rate limiter blocks at capacity
    Tool: Bash
    Preconditions: Test uses short window (e.g., 50ms window, 3 max)
    Steps:
      1. Run `go test ./internal/service/im/ -run TestRateLimiterBlocking -v -timeout 10s`
      2. Assert 4th call blocks for approximately the window duration
    Expected Result: 4th call delayed until window expires
    Failure Indicators: No blocking occurs or test times out
    Evidence: .sisyphus/evidence/task-5-ratelimit-blocking.txt
  ```

  **Commit**: YES (groups with T4, T6 → C3)
  - Message: `feat(im): add dedup cache, rate limiter, and error classification`
  - Files: `internal/service/im/rate_limiter.go`, `internal/service/im/rate_limiter_test.go`
  - Pre-commit: `go test ./internal/service/im/...`

- [x] 6. Error Classification Module

  **What to do**:
  - Create new file `internal/service/im/errors.go`:
    ```go
    type ErrorCategory int
    const (
        ErrCategoryRateLimit   ErrorCategory = iota // HTTP 429 or "rate limit" in message
        ErrCategoryServerError                       // HTTP 5xx or "internal server error"
        ErrCategoryClientError                       // HTTP 4xx (non-429) or "bad request"
        ErrCategoryParseError                        // JSON parse failure, invalid response format
        ErrCategoryNetwork                           // Timeout, connection refused, DNS failure
    )

    func (c ErrorCategory) String() string // Stringer for logging
    func (c ErrorCategory) ShouldRetry() bool // rate_limit, server_error, network → true; client_error, parse_error → false

    type SendResult struct {
        OK         bool
        Error      string
        HTTPStatus int
    }

    func ClassifyError(result SendResult) ErrorCategory
    ```
  - `ClassifyError` logic:
    - `HTTPStatus == 429` → RateLimit
    - `HTTPStatus >= 500` → ServerError
    - `HTTPStatus >= 400 && HTTPStatus < 500` → ClientError
    - `Error` contains "parse", "json", "unmarshal" → ParseError
    - `Error` contains "timeout", "connection refused", "no such host" → Network
    - Default → Network
  - `ShouldRetry()`: RateLimit, ServerError, Network → true; ClientError, ParseError → false
  - Create test file `internal/service/im/errors_test.go`:
    - Table-driven tests covering all 5 categories
    - ShouldRetry returns correct values
    - Edge cases: zero status, empty error string, combined signals

  **Must NOT do**:
  - Do NOT integrate retry logic here — that's T7
  - Do NOT import from external error libraries
  - Keep this module pure (no side effects, no logging — caller decides)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T4, T5)
  - **Blocks**: T7
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - Claude-to-IM `delivery-layer.ts:78-101` — `classifyError()` with 5 categories, HTTP status mapping, regex-based pattern matching

  **WHY Each Reference Matters**:
  - Direct design reference. Go version uses constants instead of string enum, and `strings.Contains` instead of regex for simpler patterns.

  **Acceptance Criteria**:
  - [ ] `internal/service/im/errors.go` exists with `ClassifyError()` and `ErrorCategory`
  - [ ] `internal/service/im/errors_test.go` passes with all categories covered
  - [ ] At least 8 test cases (one per category + edge cases)
  - [ ] `ShouldRetry()` correctly maps all categories

  **QA Scenarios**:

  ```
  Scenario: All error categories correctly classified
    Tool: Bash
    Preconditions: errors.go and errors_test.go exist
    Steps:
      1. Run `go test ./internal/service/im/ -run TestClassifyError -v`
      2. Assert all subtests pass
    Expected Result: Each HTTP status / error pattern maps to correct category
    Failure Indicators: Wrong category returned for any test case
    Evidence: .sisyphus/evidence/task-6-errors-test.txt
  ```

  **Commit**: YES (groups with T4, T5 → C3)
  - Message: `feat(im): add dedup cache, rate limiter, and error classification`
  - Files: `internal/service/im/errors.go`, `internal/service/im/errors_test.go`
  - Pre-commit: `go test ./internal/service/im/...`

### Wave 2 — Integration

- [x] 7. Retry with Exponential Backoff

  **What to do**:
  - Create new file `internal/service/im/retry.go`:
    ```go
    type RetryConfig struct {
        MaxAttempts    int           // Default: 3
        BaseDelay      time.Duration // Default: 1s
        MaxDelay       time.Duration // Default: 30s
        JitterMax      time.Duration // Default: 500ms
        InterMsgDelay  time.Duration // Default: 300ms (delay between sequential sends)
    }

    func DefaultRetryConfig() RetryConfig

    // RetryableSend wraps a send function with retry logic.
    // Classifies errors and only retries retryable categories.
    // Returns the final SendResult after all attempts exhausted.
    func RetryableSend(ctx context.Context, cfg RetryConfig, logger *zap.Logger, sendFn func() SendResult) SendResult
    ```
  - `RetryableSend` logic:
    1. Call `sendFn()`
    2. If OK → return immediately
    3. Classify error via `ClassifyError(result)`
    4. If `!category.ShouldRetry()` → return immediately (no retry for client/parse errors)
    5. If parse_error → caller handles fallback (not retry's job)
    6. Calculate delay: `min(baseDelay * 2^attempt + random(0, jitterMax), maxDelay)`
    7. Sleep, then retry
    8. After maxAttempts → return last result with all errors logged
  - Log each retry attempt with structured fields: `attempt`, `category`, `delay`, `error`
  - Create `internal/service/im/retry_test.go`:
    - Succeeds on first try (no retry)
    - Retries on server error, succeeds on 2nd attempt
    - Gives up after maxAttempts
    - Does NOT retry client error
    - Does NOT retry parse error
    - Backoff increases between attempts
    - Context cancellation stops retry loop

  **Must NOT do**:
  - Do NOT add HTML→plaintext fallback here — that's in the DeliveryService (T12)
  - Do NOT import external retry libraries
  - Do NOT use global retry config — pass via constructor/parameter

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Retry logic requires careful concurrency handling and multiple interacting edge cases
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T8, T9, T10)
  - **Blocks**: T12
  - **Blocked By**: T6 (uses `ClassifyError` and `SendResult`)

  **References**:

  **Pattern References**:
  - `internal/service/im/errors.go` — `ClassifyError()`, `SendResult`, `ShouldRetry()` (created in T6)
  - Claude-to-IM `delivery-layer.ts:64-68` — `backoffDelay()` with base * 2^attempt + jitter
  - Claude-to-IM `delivery-layer.ts:200-270` — retry loop with error classification and early exit

  **WHY Each Reference Matters**:
  - `errors.go` provides the `ClassifyError`/`ShouldRetry` that retry depends on — tight coupling
  - Claude-to-IM's retry loop is the design blueprint; Go version adds context cancellation

  **Acceptance Criteria**:
  - [ ] `internal/service/im/retry.go` exists with `RetryableSend()`
  - [ ] `internal/service/im/retry_test.go` passes with at least 7 test cases
  - [ ] Backoff delay doubles between attempts (verified in test)
  - [ ] Context cancellation respected (test verifies early exit)

  **QA Scenarios**:

  ```
  Scenario: Retry succeeds on transient failure
    Tool: Bash
    Preconditions: retry.go and retry_test.go exist
    Steps:
      1. Run `go test ./internal/service/im/ -run TestRetry -v`
      2. Assert TestRetrySucceedsOnSecondAttempt passes
    Expected Result: First call fails (server error), second succeeds, function returns OK
    Failure Indicators: Function gives up prematurely or doesn't retry
    Evidence: .sisyphus/evidence/task-7-retry-test.txt

  Scenario: No retry for non-retryable errors
    Tool: Bash
    Preconditions: Test includes client_error and parse_error cases
    Steps:
      1. Run `go test ./internal/service/im/ -run TestRetryNoRetryClientError -v`
      2. Assert send function called exactly once
    Expected Result: Client errors return immediately without retry
    Failure Indicators: Send function called more than once
    Evidence: .sisyphus/evidence/task-7-retry-no-retry.txt
  ```

  **Commit**: YES (groups with T8, T9 → C4)
  - Message: `feat(im): integrate retry delivery, dedup, and rate limiting into bridge`
  - Files: `internal/service/im/retry.go`, `internal/service/im/retry_test.go`
  - Pre-commit: `go test ./internal/service/im/...`

- [x] 8. Integrate Dedup into Bridge Service

  **What to do**:
  - Add `dedupCache *DedupCache` field to `FeishuBridgeService` struct
  - Initialize in `NewFeishuBridgeService()` with `NewDedupCache(1000)`
  - In `HandleMessageEvent()`, after parsing the Feishu event, check dedup:
    ```go
    messageID := msgEvent.Event.Message.MessageID
    if s.dedupCache.IsDuplicate(messageID) {
        s.logger.Debug("duplicate message, skipping", zap.String("messageID", messageID))
        return
    }
    ```
  - This prevents duplicate webhook deliveries from Feishu (which retries on non-200 or timeout)
  - Update existing tests or add new test verifying duplicate messages are skipped

  **Must NOT do**:
  - Do NOT modify `DedupCache` itself (already created in T4)
  - Do NOT add dedup to the outbound (chunk→Feishu) path — that's a different concern
  - Do NOT persist dedup state to DB — in-memory is sufficient

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7, T9, T10)
  - **Blocks**: T14
  - **Blocked By**: T1 (files at correct path), T4 (dedup module exists)

  **References**:

  **Pattern References**:
  - `internal/service/im/dedup.go` — `DedupCache` (created in T4)
  - `internal/service/im/feishu_bridge_service.go:81-108` — `HandleMessageEvent()` where dedup check goes
  - `internal/service/im/feishu_types.go:44-50` — `FeishuMessage.MessageID` field for dedup key

  **WHY Each Reference Matters**:
  - Dedup check must happen early in `HandleMessageEvent()` before any session lookup or agent trigger
  - `MessageID` from Feishu is globally unique per message — ideal dedup key

  **Acceptance Criteria**:
  - [ ] `FeishuBridgeService` has `dedupCache` field
  - [ ] Duplicate messages logged at DEBUG and skipped
  - [ ] Test verifies second call with same messageID is no-op

  **QA Scenarios**:

  ```
  Scenario: Duplicate webhook ignored
    Tool: Bash
    Preconditions: Dedup integrated into bridge service
    Steps:
      1. Run `go test ./internal/service/im/ -run TestDedupIntegration -v`
      2. Assert second HandleMessageEvent call with same messageID does NOT trigger agent
    Expected Result: Agent spawned exactly once for duplicate messages
    Failure Indicators: Agent spawned twice
    Evidence: .sisyphus/evidence/task-8-dedup-integration.txt
  ```

  **Commit**: YES (groups with T7, T9 → C4)
  - Message: `feat(im): integrate retry delivery, dedup, and rate limiting into bridge`
  - Files: `internal/service/im/feishu_bridge_service.go`
  - Pre-commit: `go test ./internal/service/im/...`

- [x] 9. Integrate Rate Limiter into Bridge Service

  **What to do**:
  - Add `rateLimiter *RateLimiter` field to `FeishuBridgeService` struct
  - Initialize in `NewFeishuBridgeService()` with `NewRateLimiter(20, 60*time.Second)` (configurable via FeishuConfig later)
  - In `OnAgentChunk()`, before sending any message to Feishu (text, card, error), call:
    ```go
    if !s.rateLimiter.TryAcquire(chatID) {
        s.logger.Warn("rate limited, dropping message", zap.String("chatID", chatID))
        return
    }
    ```
  - Use `TryAcquire` (non-blocking) in the chunk handler to avoid blocking agent execution. Messages that exceed the rate are logged and dropped.
  - Update tests to verify rate limiting is applied

  **Must NOT do**:
  - Do NOT use blocking `Acquire()` in the chunk handler — this would block the ChunkListener goroutine
  - Do NOT modify `RateLimiter` itself (already created in T5)
  - Do NOT rate limit at the webhook inbound level — only outbound sends

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7, T8, T10)
  - **Blocks**: T14
  - **Blocked By**: T1 (files at correct path), T5 (rate limiter module exists)

  **References**:

  **Pattern References**:
  - `internal/service/im/rate_limiter.go` — `RateLimiter` with `TryAcquire()` (created in T5)
  - `internal/service/im/feishu_bridge_service.go:197-245` — `OnAgentChunk()` where rate limit check goes

  **WHY Each Reference Matters**:
  - Rate limiting must be at the OUTBOUND path (chunk→Feishu), not inbound (webhook→handler)
  - `TryAcquire` is critical — blocking `Acquire` would stall the ChunkListener goroutine

  **Acceptance Criteria**:
  - [ ] `FeishuBridgeService` has `rateLimiter` field
  - [ ] Messages exceeding rate are logged and dropped (not queued)
  - [ ] Test verifies rate limiting triggers after configured threshold

  **QA Scenarios**:

  ```
  Scenario: Rate limiter drops excess messages
    Tool: Bash
    Preconditions: Rate limiter integrated into bridge
    Steps:
      1. Run `go test ./internal/service/im/ -run TestRateLimitIntegration -v`
      2. Assert messages beyond rate limit are dropped with warning log
    Expected Result: First N messages sent, subsequent ones dropped
    Failure Indicators: All messages sent regardless of rate
    Evidence: .sisyphus/evidence/task-9-ratelimit-integration.txt
  ```

  **Commit**: YES (groups with T7, T8 → C4)
  - Message: `feat(im): integrate retry delivery, dedup, and rate limiting into bridge`
  - Files: `internal/service/im/feishu_bridge_service.go`
  - Pre-commit: `go test ./internal/service/im/...`

- [x] 10. Comprehensive Tests for Bridge Service and Webhook Handler

  **What to do**:
  - Create/expand `internal/service/im/feishu_bridge_service_test.go` with thorough tests covering the EXISTING bridge service logic (these were missing — zero bridge service tests existed before):
    - `TestHandleMessageEvent_NewSession`: first message creates session + triggers agent
    - `TestHandleMessageEvent_ExistingSession`: subsequent message reuses session
    - `TestHandleMessageEvent_EmptyText`: empty message is rejected gracefully
    - `TestOnAgentChunk_TextBuffering`: text chunks accumulated and flushed
    - `TestOnAgentChunk_ToolUse`: tool_use chunk sends card immediately
    - `TestOnAgentChunk_Error`: error chunk sends red card
    - `TestOnAgentChunk_Status`: status chunk sends completion card
    - `TestOnAgentChunk_Thinking`: thinking chunk is silently ignored
  - Create/expand `internal/api/feishu_webhook_handler_test.go` (started in T2, now comprehensive):
    - `TestHandleWebhook_URLVerification`: returns challenge for verification
    - `TestHandleWebhook_ValidMessage`: returns 200, triggers async handling
    - `TestHandleWebhook_InvalidToken`: returns 200 but logs warning (Feishu expects 200 always)
    - `TestHandleWebhook_InvalidJSON`: returns 400
    - `TestHandleWebhook_UnknownEventType`: returns 200, no processing
  - Use mock/fake implementations for dependencies:
    - Fake `LarkCLIClient` that records sent messages instead of spawning processes
    - Fake `Orchestrator` that records spawn calls
    - In-memory SQLite for session repo (existing pattern)

  **Must NOT do**:
  - Do NOT use gomock or testify — create simple fakes as struct implementations
  - Do NOT test external lark-cli process execution — test the bridge service logic only
  - Do NOT modify production code to accommodate tests (except adding interfaces if needed for faking)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Writing comprehensive test suite requires understanding multiple interacting components
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7, T8, T9)
  - **Blocks**: T11, T13
  - **Blocked By**: T1 (files at correct path), T2 (context fix), T3 (buffer fix)

  **References**:

  **Pattern References**:
  - `internal/service/im/feishu_bridge_service.go` — all public methods to test (after T1 move, T2+T3 fixes)
  - `internal/api/feishu_webhook_handler.go` — handler methods to test
  - `internal/repo/im_session_repo_test.go:203-224` — `createTable()` helper pattern for SQLite test setup

  **Test References**:
  - `internal/service/a2a/mention_parser_test.go` — table-driven test pattern
  - `internal/service/command/service_test.go` — service-level testing pattern

  **WHY Each Reference Matters**:
  - `im_session_repo_test.go` has the exact pattern for creating test tables in SQLite `:memory:` — reuse it
  - Existing service tests show how to test services without mocking frameworks

  **Acceptance Criteria**:
  - [ ] `feishu_bridge_service_test.go` has ≥8 test functions
  - [ ] `feishu_webhook_handler_test.go` has ≥5 test functions
  - [ ] All tests pass with `go test -race`
  - [ ] Fake implementations created for `LarkCLIClient` and orchestrator interaction

  **QA Scenarios**:

  ```
  Scenario: All bridge service tests pass
    Tool: Bash
    Preconditions: Test files created with fakes
    Steps:
      1. Run `go test -race ./internal/service/im/ -v -count=1`
      2. Assert all test functions pass
      3. Assert zero race conditions
    Expected Result: All ≥8 tests pass, no races
    Failure Indicators: Any FAIL or DATA RACE
    Evidence: .sisyphus/evidence/task-10-bridge-tests.txt

  Scenario: All webhook handler tests pass
    Tool: Bash
    Preconditions: Test files created
    Steps:
      1. Run `go test -race ./internal/api/ -run TestHandleWebhook -v`
      2. Assert all ≥5 tests pass
    Expected Result: All pass
    Failure Indicators: Any FAIL
    Evidence: .sisyphus/evidence/task-10-handler-tests.txt
  ```

  **Commit**: YES (C5)
  - Message: `test(im): add comprehensive tests for bridge service and webhook handler`
  - Files: `internal/service/im/feishu_bridge_service_test.go`, `internal/api/feishu_webhook_handler_test.go`
  - Pre-commit: `make test`

### Phase 2 — Platform Abstraction

### Wave 3 — Abstraction Definitions

- [x] 11. Define IMAdapter Interface and Types

  **What to do**:
  - Create new file `internal/service/im/adapter.go`:
    ```go
    // IMAdapter defines the contract for IM platform adapters.
    // Each platform (Feishu, Slack, Discord) implements this interface.
    type IMAdapter interface {
        // Platform returns the platform identifier (e.g., "feishu", "slack")
        Platform() string

        // SendText sends a plain text message to the given chat.
        SendText(ctx context.Context, chatID, text string) SendResult

        // SendCard sends a structured card/rich message.
        SendCard(ctx context.Context, chatID, cardJSON string) SendResult

        // ReplyText replies to a specific message.
        ReplyText(ctx context.Context, chatID, messageID, text string) SendResult

        // CreateStreamingCard creates a streaming card and returns its ID.
        // Returns ("", err) if the platform doesn't support streaming cards.
        CreateStreamingCard(ctx context.Context, chatID string) (cardID string, err error)

        // UpdateStreamingCard updates the content of a streaming card.
        UpdateStreamingCard(ctx context.Context, cardID string, content string, sequence int) error

        // FinalizeStreamingCard closes streaming mode and sets final content.
        FinalizeStreamingCard(ctx context.Context, cardID string, content string, sequence int) error

        // CheckHealth verifies the adapter's external dependencies are available.
        CheckHealth(ctx context.Context) error

        // MaxMessageLength returns the platform's max message length.
        MaxMessageLength() int
    }
    ```
  - Create new file `internal/service/im/types.go` (shared types for all adapters):
    ```go
    // IMPlatform type already exists in model/im_session.go — reuse it
    // Add new platform constants here as needed

    // ChunkMessage represents a processed chunk ready for delivery.
    type ChunkMessage struct {
        ChatID       string
        InvocationID string
        Type         string // "text", "card", "error", "status"
        Content      string
        CardJSON     string // For structured messages
        DedupKey     string // For dedup checking
    }

    // DeliveryResult tracks the outcome of message delivery.
    type DeliveryResult struct {
        OK           bool
        Attempts     int
        FinalError   string
        Category     ErrorCategory
    }
    ```
  - Create `internal/service/im/adapter_test.go` — compile-time interface satisfaction checks:
    ```go
    // Verify FeishuAdapter (T14) will satisfy IMAdapter
    // var _ IMAdapter = (*FeishuAdapter)(nil) // Uncomment when FeishuAdapter exists
    ```
  - Write tests for `ChunkMessage` construction helpers if any

  **Must NOT do**:
  - Do NOT implement the interface yet — just define it (FeishuAdapter is T14)
  - Do NOT over-abstract: no generic "metadata" map, no "options" pattern
  - Do NOT change existing `FeishuBridgeService` in this task
  - Do NOT add methods that only one platform needs — keep the interface minimal

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Interface design requires careful thought about multi-platform compatibility
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T12, T13)
  - **Blocks**: T14, T15, T21, T22
  - **Blocked By**: T10 (tests confirm current behavior before abstracting)

  **References**:

  **Pattern References**:
  - `internal/service/agent/adapter.go` — `AgentAdapter` interface with 8 methods (existing adapter pattern to follow)
  - `internal/service/im/feishu_bridge_service.go:197-245` — `OnAgentChunk()` reveals what the adapter needs to support (text, card, error, status)
  - `internal/service/im/lark_cli_client.go` — current Feishu-specific send methods that will map to adapter interface
  - Claude-to-IM `channel-adapter.ts:121-136` — `BaseChannelAdapter` with `send()` method

  **API/Type References**:
  - `internal/model/im_session.go:12-16` — existing `IMPlatform` type to reuse
  - `internal/service/im/errors.go` — `SendResult` already defined in T6

  **WHY Each Reference Matters**:
  - `AgentAdapter` is the closest existing pattern — follow its style (method names, context passing, error returns)
  - `lark_cli_client.go` methods directly map to the adapter interface: `SendTextMessage→SendText`, `SendCardMessage→SendCard`, `ReplyMessage→ReplyText`
  - Streaming card methods are based on Claude-to-IM's CardKit v2 pattern

  **Acceptance Criteria**:
  - [ ] `internal/service/im/adapter.go` defines `IMAdapter` interface
  - [ ] `internal/service/im/types.go` defines shared types
  - [ ] Interface has clear godoc comments
  - [ ] `go build ./...` succeeds (interface compiles)

  **QA Scenarios**:

  ```
  Scenario: Interface compiles and is well-defined
    Tool: Bash
    Preconditions: adapter.go and types.go exist
    Steps:
      1. Run `go build ./internal/service/im/...`
      2. Assert exit code 0
      3. Run `go vet ./internal/service/im/...`
      4. Assert exit code 0
    Expected Result: Clean build with no warnings
    Failure Indicators: Compilation errors or vet warnings
    Evidence: .sisyphus/evidence/task-11-interface-build.txt
  ```

  **Commit**: YES (groups with T12, T13 → C6)
  - Message: `refactor(im): define IMAdapter interface, delivery service, session locks`
  - Files: `internal/service/im/adapter.go`, `internal/service/im/types.go`, `internal/service/im/adapter_test.go`
  - Pre-commit: `go build ./...`

- [x] 12. Build DeliveryService

  **What to do**:
  - Create new file `internal/service/im/delivery.go`:
    ```go
    // DeliveryService handles reliable message delivery to IM platforms.
    // It wraps an IMAdapter with chunking, retry, rate limiting, and dedup.
    type DeliveryService struct {
        adapter     IMAdapter
        retryCfg    RetryConfig
        rateLimiter *RateLimiter
        dedupCache  *DedupCache
        logger      *zap.Logger
    }

    func NewDeliveryService(adapter IMAdapter, retryCfg RetryConfig, rateLimiter *RateLimiter, dedupCache *DedupCache, logger *zap.Logger) *DeliveryService

    // DeliverText delivers a text message, chunking if needed, with retry and dedup.
    func (d *DeliveryService) DeliverText(ctx context.Context, chatID, text, dedupKey string) DeliveryResult

    // DeliverCard delivers a card message with retry.
    func (d *DeliveryService) DeliverCard(ctx context.Context, chatID, cardJSON, dedupKey string) DeliveryResult
    ```
  - `DeliverText` logic:
    1. Dedup check: if `dedupKey != ""` and `dedupCache.IsDuplicate(dedupKey)` → return OK
    2. Rate limit: if `!rateLimiter.TryAcquire(chatID)` → return rate limited result
    3. Chunk: `chunkText(text, adapter.MaxMessageLength())` → split into chunks
    4. For each chunk: `RetryableSend(ctx, retryCfg, logger, func() SendResult { return adapter.SendText(ctx, chatID, chunk) })`
    5. Add 300ms delay between chunks (inter-message delay from RetryConfig)
    6. Return aggregate result
  - Create `chunkText(text string, maxLen int) []string` function:
    - If `len(text) <= maxLen` → return `[]string{text}`
    - Split at last `\n` before maxLen (prefer clean line breaks)
    - If no `\n` found in first half → split at maxLen
    - Trim leading `\n` from remainder
  - Create `internal/service/im/delivery_test.go`:
    - `TestChunkText_ShortMessage`: no chunking needed
    - `TestChunkText_LongMessage`: splits at newline
    - `TestChunkText_NoNewlines`: splits at maxLen
    - `TestDeliverText_Success`: sends and returns OK
    - `TestDeliverText_Dedup`: duplicate dedupKey skips send
    - `TestDeliverText_RateLimited`: returns rate limit result
    - `TestDeliverText_ChunkedMessage`: long text split and sent as multiple
    - `TestDeliverText_RetryOnError`: transient failure retried

  **Must NOT do**:
  - Do NOT add HTML→plaintext fallback yet (only relevant when we have platforms that support HTML)
  - Do NOT add streaming card delivery here — that's a separate method in FeishuAdapter (T16)
  - Keep this module testable with a fake `IMAdapter`

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Delivery service is the central reliability layer; must orchestrate retry, dedup, rate limit, and chunking correctly
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T11, T13)
  - **Blocks**: T14
  - **Blocked By**: T7 (retry module)

  **References**:

  **Pattern References**:
  - `internal/service/im/retry.go` — `RetryableSend()` (created in T7)
  - `internal/service/im/rate_limiter.go` — `TryAcquire()` (created in T5)
  - `internal/service/im/dedup.go` — `IsDuplicate()` (created in T4)
  - `internal/service/im/errors.go` — `SendResult`, `ClassifyError()` (created in T6)
  - Claude-to-IM `delivery-layer.ts:36-59` — `chunkText()` with newline-preferred splitting
  - Claude-to-IM `delivery-layer.ts:130-270` — full delivery flow: dedup → send → classify → retry → fallback

  **WHY Each Reference Matters**:
  - This task COMPOSES modules from T4-T7 into a single cohesive delivery pipeline
  - Claude-to-IM's delivery layer is the architecture blueprint; we adapt the flow but use our own modules

  **Acceptance Criteria**:
  - [ ] `internal/service/im/delivery.go` exists with `DeliveryService`
  - [ ] `chunkText()` function handles all splitting edge cases
  - [ ] `internal/service/im/delivery_test.go` has ≥8 test functions
  - [ ] Tests use a fake `IMAdapter` (no external process)

  **QA Scenarios**:

  ```
  Scenario: Text chunking splits correctly at newlines
    Tool: Bash
    Preconditions: delivery.go and delivery_test.go exist
    Steps:
      1. Run `go test ./internal/service/im/ -run TestChunkText -v`
      2. Assert all chunk test cases pass
    Expected Result: Long text split at newlines, short text returned as-is
    Failure Indicators: Chunks exceed maxLen or content lost
    Evidence: .sisyphus/evidence/task-12-chunk-test.txt

  Scenario: Full delivery pipeline works end-to-end
    Tool: Bash
    Preconditions: Fake IMAdapter configured
    Steps:
      1. Run `go test ./internal/service/im/ -run TestDeliverText -v`
      2. Assert success, dedup, rate limit, and retry cases all pass
    Expected Result: All delivery scenarios handled correctly
    Failure Indicators: Any test failure
    Evidence: .sisyphus/evidence/task-12-delivery-test.txt
  ```

  **Commit**: YES (groups with T11, T13 → C6)
  - Message: `refactor(im): define IMAdapter interface, delivery service, session locks`
  - Files: `internal/service/im/delivery.go`, `internal/service/im/delivery_test.go`
  - Pre-commit: `go test ./internal/service/im/...`

- [x] 13. Per-Session Lock Module

  **What to do**:
  - Create new file `internal/service/im/session_lock.go`:
    ```go
    // SessionLock provides per-session serialization for IM message processing.
    // Different sessions run concurrently; same-session operations serialize.
    type SessionLock struct {
        mu     sync.Mutex
        chains map[string]chan struct{} // sessionID → done channel
    }

    func NewSessionLock() *SessionLock

    // Acquire blocks until the session is available, then returns a release function.
    // Usage:
    //   release := lock.Acquire(sessionID)
    //   defer release()
    //   // ... process message ...
    func (sl *SessionLock) Acquire(sessionID string) func()
    ```
  - Implementation:
    1. Lock mutex, check if session has an existing chain
    2. If yes → save the `done` channel, create new channel, unlock, wait on old channel
    3. If no → create new channel, store it, unlock, return immediately
    4. Return release function that closes the current channel and cleans up under lock
  - Create `internal/service/im/session_lock_test.go`:
    - `TestSessionLock_NoContention`: single session, no blocking
    - `TestSessionLock_Serialization`: two goroutines for same session serialize
    - `TestSessionLock_DifferentSessions`: two goroutines for different sessions run concurrently
    - `TestSessionLock_ChainCleanup`: after release, map is cleaned up
    - `TestSessionLock_HighContention`: 50 goroutines for same session, all complete

  **Must NOT do**:
  - Do NOT use a global mutex (that would serialize ALL sessions)
  - Do NOT add timeout to the lock — caller manages timeouts via context
  - Do NOT over-engineer — this is a simple per-key serializer

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T11, T12)
  - **Blocks**: T14, T15
  - **Blocked By**: T10 (confirm current concurrent behavior before adding locks)

  **References**:

  **Pattern References**:
  - Claude-to-IM `bridge-manager.ts:171-213` — `processWithSessionLock()` with Promise chaining and cleanup

  **WHY Each Reference Matters**:
  - The Promise chaining pattern translates to Go channels: each session has a "done" channel, next operation waits on it

  **Acceptance Criteria**:
  - [ ] `internal/service/im/session_lock.go` exists with `SessionLock`
  - [ ] `internal/service/im/session_lock_test.go` passes with `-race`
  - [ ] At least 5 test cases including high-contention
  - [ ] Map cleaned up after all operations complete

  **QA Scenarios**:

  ```
  Scenario: Same-session operations serialize correctly
    Tool: Bash
    Preconditions: session_lock.go and test exist
    Steps:
      1. Run `go test -race ./internal/service/im/ -run TestSessionLock -v -count=3`
      2. Assert all tests pass with zero race conditions
    Expected Result: Serialization verified, no races
    Failure Indicators: DATA RACE or incorrect ordering
    Evidence: .sisyphus/evidence/task-13-session-lock-test.txt
  ```

  **Commit**: YES (groups with T11, T12 → C6)
  - Message: `refactor(im): define IMAdapter interface, delivery service, session locks`
  - Files: `internal/service/im/session_lock.go`, `internal/service/im/session_lock_test.go`
  - Pre-commit: `go test -race ./internal/service/im/...`

### Wave 4 — Platform Refactor

- [x] 14. Extract FeishuAdapter from Bridge Service

  **What to do**:
  - Create new file `internal/service/im/feishu_adapter.go` implementing `IMAdapter`:
    ```go
    type FeishuAdapter struct {
        client  *LarkCLIClient
        logger  *zap.Logger
        healthy bool
    }

    func NewFeishuAdapter(client *LarkCLIClient, logger *zap.Logger) *FeishuAdapter

    // Implement all IMAdapter methods:
    func (a *FeishuAdapter) Platform() string                    // "feishu"
    func (a *FeishuAdapter) SendText(ctx, chatID, text) SendResult
    func (a *FeishuAdapter) SendCard(ctx, chatID, cardJSON) SendResult
    func (a *FeishuAdapter) ReplyText(ctx, chatID, msgID, text) SendResult
    func (a *FeishuAdapter) CreateStreamingCard(ctx, chatID) (string, error) // stub for T16
    func (a *FeishuAdapter) UpdateStreamingCard(ctx, cardID, content, seq) error // stub for T16
    func (a *FeishuAdapter) FinalizeStreamingCard(ctx, cardID, content, seq) error // stub for T16
    func (a *FeishuAdapter) CheckHealth(ctx) error
    func (a *FeishuAdapter) MaxMessageLength() int               // 4000 for Feishu
    ```
  - Each send method wraps `LarkCLIClient` and returns `SendResult`:
    - Call `larkClient.SendTextMessage(ctx, chatID, text)`
    - If error → `SendResult{OK: false, Error: err.Error()}`
    - If success → `SendResult{OK: true}`
  - Extract card-building logic from `FeishuBridgeService` into `feishu_adapter.go`:
    - `buildSimpleCard()` (lines 352-365 of old bridge service)
    - `sendCompletionCard()` logic (lines 313-349)
  - Streaming card methods return `ErrNotImplemented` for now (T16 fills them in)
  - Add compile-time check: `var _ IMAdapter = (*FeishuAdapter)(nil)`
  - Create `internal/service/im/feishu_adapter_test.go`:
    - Test each method with a fake `LarkCLIClient` (mock that records calls)
    - Test `MaxMessageLength()` returns 4000
    - Test `Platform()` returns "feishu"
    - Test error wrapping in SendResult

  **Must NOT do**:
  - Do NOT delete `FeishuBridgeService` yet — it's refactored into `IMBridgeService` in T15
  - Do NOT implement streaming card methods yet — that's T16
  - Do NOT change `LarkCLIClient` interface — adapt around it

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Extracting an adapter from an existing service requires careful method mapping and ensuring no behavior changes
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T15, T16)
  - **Blocks**: T16, T17
  - **Blocked By**: T11 (interface definition), T12 (delivery service), T13 (session locks)

  **References**:

  **Pattern References**:
  - `internal/service/im/adapter.go` — `IMAdapter` interface (created in T11)
  - `internal/service/im/feishu_bridge_service.go:352-365` — `buildSimpleCard()` to extract
  - `internal/service/im/feishu_bridge_service.go:313-349` — `sendCompletionCard()` logic to extract
  - `internal/service/im/lark_cli_client.go` — underlying client that adapter wraps
  - `internal/service/agent/claude_adapter.go` — example of adapter wrapping CLI process (pattern reference)

  **API/Type References**:
  - `internal/service/im/errors.go` — `SendResult` struct for return values

  **WHY Each Reference Matters**:
  - `adapter.go` is the contract this must satisfy — verify all methods match
  - Bridge service methods map 1:1 to adapter methods; extraction must preserve behavior
  - `claude_adapter.go` shows how existing adapters in this codebase wrap CLI processes

  **Acceptance Criteria**:
  - [ ] `internal/service/im/feishu_adapter.go` exists, compiles, satisfies `IMAdapter`
  - [ ] `var _ IMAdapter = (*FeishuAdapter)(nil)` compiles
  - [ ] All non-streaming methods are fully implemented
  - [ ] Streaming methods return `ErrNotImplemented`
  - [ ] `feishu_adapter_test.go` has ≥6 test functions

  **QA Scenarios**:

  ```
  Scenario: FeishuAdapter satisfies IMAdapter interface
    Tool: Bash
    Preconditions: feishu_adapter.go exists
    Steps:
      1. Run `go build ./internal/service/im/...`
      2. Assert compile-time check passes
    Expected Result: No compilation errors
    Failure Indicators: "does not implement IMAdapter"
    Evidence: .sisyphus/evidence/task-14-adapter-build.txt

  Scenario: FeishuAdapter send methods work with fake client
    Tool: Bash
    Preconditions: feishu_adapter_test.go exists with fake LarkCLIClient
    Steps:
      1. Run `go test ./internal/service/im/ -run TestFeishuAdapter -v`
      2. Assert all tests pass
    Expected Result: SendText records call, SendCard records call, errors wrapped correctly
    Failure Indicators: Test failures
    Evidence: .sisyphus/evidence/task-14-adapter-test.txt
  ```

  **Commit**: YES (groups with T15 → C7)
  - Message: `refactor(im): extract FeishuAdapter and generic IMBridgeService`
  - Files: `internal/service/im/feishu_adapter.go`, `internal/service/im/feishu_adapter_test.go`
  - Pre-commit: `go test ./internal/service/im/...`

- [x] 15. Build Generic IMBridgeService

  **What to do**:
  - Create new file `internal/service/im/bridge_service.go` — the platform-agnostic bridge:
    ```go
    // IMBridgeService is the platform-agnostic bridge between agent execution and IM platforms.
    // It replaces the Feishu-specific FeishuBridgeService as the ChunkListener and message handler.
    type IMBridgeService struct {
        adapters     map[string]IMAdapter     // platform → adapter
        delivery     map[string]*DeliveryService // platform → delivery
        sessionRepo  *repo.IMSessionRepository
        threadRepo   *repo.ThreadRepository
        projectRepo  *repo.ProjectRepository
        orchestrator interface{ SpawnAgentForUserMessage(...) } // minimal interface
        wsHub        *ws.Hub
        sessionLock  *SessionLock
        logger       *zap.Logger
    }

    func NewIMBridgeService(...) *IMBridgeService

    // RegisterAdapter adds a platform adapter with its delivery service.
    func (s *IMBridgeService) RegisterAdapter(adapter IMAdapter, delivery *DeliveryService)

    // HandleInboundMessage handles an incoming message from any IM platform.
    func (s *IMBridgeService) HandleInboundMessage(ctx context.Context, platform, chatID, chatType, userID, userName, messageID, text string) error

    // OnAgentChunk is the ChunkListener callback — routes chunks to the correct platform adapter.
    func (s *IMBridgeService) OnAgentChunk(threadID, invocationID uuid.UUID, chunk agent.Chunk, agentID, agentName string)
    ```
  - `HandleInboundMessage` logic (generalized from `FeishuBridgeService.HandleMessageEvent`):
    1. Session lock: `release := s.sessionLock.Acquire(chatID); defer release()`
    2. Get or create IMSession: lookup by (platform, chatID) → create thread if new
    3. Trigger agent: `orchestrator.SpawnAgentForUserMessage(threadID, text)`
  - `OnAgentChunk` logic (generalized from `FeishuBridgeService.OnAgentChunk`):
    1. Look up IMSession by threadID → get platform + chatID
    2. Get adapter for platform: `s.adapters[session.Platform]`
    3. Get delivery for platform: `s.delivery[session.Platform]`
    4. Route by chunk type → `delivery.DeliverText()` or `delivery.DeliverCard()`
  - Keep `FeishuBridgeService` file but mark methods as deprecated (or delete if all callers moved)
  - Create `internal/service/im/bridge_service_test.go`:
    - Test with fake adapter + in-memory SQLite
    - Test inbound message creates session and triggers agent
    - Test chunk routing to correct platform adapter
    - Test session lock serialization

  **Must NOT do**:
  - Do NOT delete `feishu_bridge_service.go` entirely yet — T17 will update `main.go` to use the new service
  - Do NOT change `ChunkListener` signature
  - Do NOT import platform-specific packages in bridge_service.go — it must be platform-agnostic

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: This is the central orchestration component — must correctly generalize from Feishu-specific to platform-agnostic
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T14, T16)
  - **Blocks**: T17
  - **Blocked By**: T11 (IMAdapter interface), T13 (session locks)

  **References**:

  **Pattern References**:
  - `internal/service/im/feishu_bridge_service.go:81-108` — `HandleMessageEvent()` to generalize
  - `internal/service/im/feishu_bridge_service.go:197-245` — `OnAgentChunk()` to generalize
  - `internal/service/im/feishu_bridge_service.go:111-166` — `getOrCreateSession()` to reuse
  - `internal/service/im/adapter.go` — `IMAdapter` interface (T11)
  - `internal/service/im/delivery.go` — `DeliveryService` (T12)
  - `internal/service/im/session_lock.go` — `SessionLock` (T13)
  - Claude-to-IM `bridge-manager.ts` — central orchestrator that routes between adapters

  **API/Type References**:
  - `internal/model/im_session.go` — `IMSession` struct with Platform field for routing
  - `internal/repo/im_session_repo.go` — `FindByChatID()`, `FindByThreadID()` for lookups

  **WHY Each Reference Matters**:
  - The old bridge service methods are being generalized — reference them to ensure no behavior is lost
  - `IMSession.Platform` field enables routing chunks to the correct adapter
  - `FindByThreadID()` is critical for chunk routing: threadID → session → platform → adapter

  **Acceptance Criteria**:
  - [ ] `internal/service/im/bridge_service.go` exists with `IMBridgeService`
  - [ ] `RegisterAdapter()` accepts any `IMAdapter` implementation
  - [ ] `OnAgentChunk()` routes to correct adapter based on session platform
  - [ ] `bridge_service_test.go` has ≥4 test functions
  - [ ] No platform-specific imports in `bridge_service.go`

  **QA Scenarios**:

  ```
  Scenario: Inbound message creates session and triggers agent
    Tool: Bash
    Preconditions: bridge_service_test.go with fake adapter and in-memory DB
    Steps:
      1. Run `go test ./internal/service/im/ -run TestBridgeHandleInbound -v`
      2. Assert session created in DB and agent spawned
    Expected Result: New IMSession record created, orchestrator called
    Failure Indicators: No session created or agent not triggered
    Evidence: .sisyphus/evidence/task-15-bridge-inbound.txt

  Scenario: Chunk routed to correct platform adapter
    Tool: Bash
    Preconditions: Two fake adapters registered (feishu, slack)
    Steps:
      1. Run `go test ./internal/service/im/ -run TestBridgeChunkRouting -v`
      2. Assert text chunk for Feishu session goes to Feishu adapter, not Slack
    Expected Result: Correct adapter receives the message
    Failure Indicators: Wrong adapter receives message or message lost
    Evidence: .sisyphus/evidence/task-15-bridge-routing.txt
  ```

  **Commit**: YES (groups with T14 → C7)
  - Message: `refactor(im): extract FeishuAdapter and generic IMBridgeService`
  - Files: `internal/service/im/bridge_service.go`, `internal/service/im/bridge_service_test.go`
  - Pre-commit: `make test`

- [x] 16. Streaming Card Support for Feishu

  **What to do**:
  - Implement the streaming card methods in `FeishuAdapter` (stubbed in T14):
    - `CreateStreamingCard()`: Call lark-cli to create a CardKit v2 streaming card, return card ID
    - `UpdateStreamingCard()`: Call lark-cli to update card content with sequence number
    - `FinalizeStreamingCard()`: Disable streaming mode, set final content with footer
  - Add streaming card state tracking to `FeishuAdapter`:
    ```go
    type cardState struct {
        mu            sync.Mutex
        cardID        string
        messageID     string
        sequence      int
        startTime     time.Time
        pendingText   string
        lastUpdateAt  time.Time
        throttleTimer *time.Timer
    }

    const cardThrottleMs = 200
    ```
  - Add `UpdateCardContent(chatID, text string)` internal method with throttle logic:
    - If elapsed since lastUpdate < 200ms → schedule trailing-edge timer
    - If elapsed >= 200ms → flush immediately
    - Timer callback: lock, check pending, flush, unlock
  - Add `FinalizeCard(chatID, status, responseText)` method:
    - Cancel any pending throttle timer
    - Disable streaming mode via lark-cli
    - Send final card with footer (status, duration, tool calls count)
  - Add lark-cli commands to `LarkCLIClient`:
    - `CreateCard(ctx, chatID) (cardID, messageID, error)`
    - `UpdateCardContent(ctx, cardID, content, sequence) error`
    - `SetStreamingMode(ctx, cardID, enabled, sequence) error`
  - Create tests with fake lark-cli calls

  **Must NOT do**:
  - Do NOT make streaming cards mandatory — if creation fails, fall back to regular text messages
  - Do NOT change the IMAdapter interface — streaming methods already defined in T11
  - Do NOT add streaming logic to `bridge_service.go` — it stays in the adapter

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Streaming card requires understanding of Feishu CardKit v2 API, throttle timing, and state management
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T14, T15 — but starts after T14 completes its stubs)
  - **Blocks**: T17
  - **Blocked By**: T14 (FeishuAdapter exists with stubs)

  **References**:

  **Pattern References**:
  - `internal/service/im/feishu_adapter.go` — stub methods to implement (created in T14)
  - `internal/service/im/lark_cli_client.go` — where to add new lark-cli commands
  - Claude-to-IM `feishu-adapter.ts:52-66` — `FeishuCardState` struct and throttle constant
  - Claude-to-IM `feishu-adapter.ts:454-482` — `updateCardContent()` with trailing-edge throttle
  - Claude-to-IM `feishu-adapter.ts:522-578` — `finalizeCard()` with streaming mode close + final update

  **External References**:
  - Feishu CardKit v2 API — streaming card create/update/finalize endpoints via lark-cli

  **WHY Each Reference Matters**:
  - Claude-to-IM's implementation is the exact blueprint for the throttle and finalize logic
  - `lark_cli_client.go` needs new methods for card operations — follow existing method pattern

  **Acceptance Criteria**:
  - [ ] Streaming card methods no longer return `ErrNotImplemented`
  - [ ] Throttle timer correctly debounces updates at 200ms
  - [ ] Fallback to text if card creation fails
  - [ ] Tests cover create → update → finalize lifecycle
  - [ ] Tests cover throttle behavior (rapid updates → single flush)

  **QA Scenarios**:

  ```
  Scenario: Streaming card lifecycle works
    Tool: Bash
    Preconditions: Fake lark-cli client that simulates card operations
    Steps:
      1. Run `go test ./internal/service/im/ -run TestStreamingCard -v`
      2. Assert create → update → finalize sequence
    Expected Result: Card created, content updated, streaming mode closed, final content set
    Failure Indicators: Any step fails or sequence incorrect
    Evidence: .sisyphus/evidence/task-16-streaming-card.txt

  Scenario: Throttle debounces rapid updates
    Tool: Bash
    Preconditions: Test sends 10 updates in 50ms
    Steps:
      1. Run `go test ./internal/service/im/ -run TestStreamingThrottle -v`
      2. Assert fewer than 10 actual API calls made
    Expected Result: Throttle reduces calls to ~2-3 (200ms intervals)
    Failure Indicators: All 10 updates sent immediately
    Evidence: .sisyphus/evidence/task-16-throttle.txt
  ```

  **Commit**: YES (C8)
  - Message: `feat(im/feishu): add streaming card support with throttle`
  - Files: `internal/service/im/feishu_adapter.go`, `internal/service/im/lark_cli_client.go`
  - Pre-commit: `go test ./internal/service/im/...`

### Wave 5 — Rewire and Config

- [x] 17. Rewire main.go to New IM Architecture

  **What to do**:
  - Replace the IM initialization block in `cmd/server/main.go` (lines 331-351) with the new architecture:
    ```go
    // ========== IM Integration ==========
    var imBridgeSvc *im.IMBridgeService
    if cfg.Feishu.Enabled {
        imSessionRepo := repo.NewIMSessionRepository(db)
        
        // Create Feishu adapter
        larkClient := im.NewLarkCLIClient(cfg.Feishu.LarkCLIPath, logger)
        feishuAdapter := im.NewFeishuAdapter(larkClient, logger)
        
        // Health check
        if err := feishuAdapter.CheckHealth(context.Background()); err != nil {
            logger.Warn("Feishu adapter health check failed, send disabled", zap.Error(err))
        }
        
        // Create delivery service for Feishu
        retryCfg := im.DefaultRetryConfig()
        rateLimiter := im.NewRateLimiter(20, 60*time.Second)
        dedupCache := im.NewDedupCache(1000)
        feishuDelivery := im.NewDeliveryService(feishuAdapter, retryCfg, rateLimiter, dedupCache, logger)
        
        // Create generic bridge service
        imBridgeSvc = im.NewIMBridgeService(
            imSessionRepo, threadRepo, projectRepo,
            orchestrator, wsHub, logger,
        )
        imBridgeSvc.RegisterAdapter(feishuAdapter, feishuDelivery)
        
        // Register as ChunkListener
        orchestrator.GetExecutionService().AddChunkListener(imBridgeSvc.OnAgentChunk)
        logger.Info("IM integration enabled", zap.String("platform", "feishu"))
    }
    ```
  - Update webhook handler registration to use new bridge service:
    - `FeishuWebhookHandler` now calls `imBridgeSvc.HandleInboundMessage()` instead of `feishuBridgeSvc.HandleMessageEvent()`
    - Update `internal/api/feishu_webhook_handler.go` to accept `*im.IMBridgeService` instead of `*im.FeishuBridgeService`
    - Parse Feishu-specific event format in handler, pass generic fields to bridge
  - Mark `FeishuBridgeService` as deprecated or remove it if no longer referenced
  - Verify `go build ./...` and `make test` pass

  **Must NOT do**:
  - Do NOT change the webhook URL — `/api/v1/feishu/webhook` stays the same
  - Do NOT change the HTTP response format — always return 200
  - Do NOT remove the `feishu` config section — keep backward compatible
  - Do NOT break the middleware whitelist for the webhook endpoint

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Rewiring initialization is the riskiest step — must maintain backward compatibility while switching internals
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential within Wave 5
  - **Blocks**: T18, T19, T20, T23
  - **Blocked By**: T14 (FeishuAdapter), T15 (IMBridgeService), T16 (streaming cards)

  **References**:

  **Pattern References**:
  - `cmd/server/main.go:331-351` — current IM init block to replace
  - `internal/api/feishu_webhook_handler.go` — handler to update
  - `internal/service/im/bridge_service.go` — new `IMBridgeService` (T15)
  - `internal/service/im/feishu_adapter.go` — new `FeishuAdapter` (T14)
  - `internal/service/im/delivery.go` — new `DeliveryService` (T12)
  - `cmd/server/main.go:200-230` — agent adapter initialization pattern (for consistency)

  **WHY Each Reference Matters**:
  - Lines 331-351 are the exact block being replaced — must be read carefully to ensure nothing is lost
  - The agent adapter init pattern (lines 200-230) shows how this codebase wires up services in main.go
  - Webhook handler needs type signature change for the new bridge service

  **Acceptance Criteria**:
  - [ ] `go build ./...` succeeds
  - [ ] `make test` passes
  - [ ] Feishu webhook still works (handler returns 200, triggers agent)
  - [ ] No references to old `FeishuBridgeService` in main.go
  - [ ] ChunkListener registered on new `IMBridgeService`

  **QA Scenarios**:

  ```
  Scenario: Application builds and starts
    Tool: Bash
    Preconditions: All Phase 2 code merged
    Steps:
      1. Run `go build ./cmd/server/...`
      2. Assert exit code 0
      3. Run `make test`
      4. Assert all tests pass
    Expected Result: Clean build, all tests pass
    Failure Indicators: Compilation errors or test failures
    Evidence: .sisyphus/evidence/task-17-build.txt

  Scenario: Old FeishuBridgeService no longer referenced
    Tool: Bash
    Preconditions: main.go updated
    Steps:
      1. Run `grep -rn "FeishuBridgeService" --include="*.go" cmd/ internal/api/`
      2. Assert zero matches (only allowed in deprecated file or tests)
    Expected Result: No production code references old bridge service
    Failure Indicators: Active references found
    Evidence: .sisyphus/evidence/task-17-no-old-refs.txt
  ```

  **Commit**: YES (groups with T18 → C9)
  - Message: `refactor(im): rewire main.go to new IM architecture with multi-platform config`
  - Files: `cmd/server/main.go`, `internal/api/feishu_webhook_handler.go`
  - Pre-commit: `make test`

- [x] 18. Multi-Platform Config Schema

  **What to do**:
  - Update `pkg/config/config.go` to support multi-platform IM configuration:
    ```go
    type IMConfig struct {
        Platforms []IMPlatformConfig `mapstructure:"platforms"`
    }

    type IMPlatformConfig struct {
        Type            string        `mapstructure:"type"`     // "feishu", "slack", "discord"
        Enabled         bool          `mapstructure:"enabled"`
        // Feishu-specific
        AppID           string        `mapstructure:"app_id"`
        AppSecret       string        `mapstructure:"app_secret"`
        VerificationToken string      `mapstructure:"verification_token"`
        EncryptKey      string        `mapstructure:"encrypt_key"`
        LarkCLIPath     string        `mapstructure:"lark_cli_path"`
        DefaultProjectID string       `mapstructure:"default_project_id"`
        // Rate limiting
        RateLimitMax    int           `mapstructure:"rate_limit_max"`    // Default: 20
        RateLimitWindow time.Duration `mapstructure:"rate_limit_window"` // Default: 60s
        // Retry
        MaxRetries      int           `mapstructure:"max_retries"`      // Default: 3
    }
    ```
  - Keep existing `FeishuConfig` for backward compatibility, but add `IMConfig` as the new canonical config
  - Add `setDefaults()` entries for new IM config fields
  - Update `configs/config.yaml.example` with the new IM config section (commented out):
    ```yaml
    im:
      platforms:
        - type: feishu
          enabled: false
          app_id: ""
          app_secret: ""
          # ... etc
    ```
  - Add validation in `validateConfig()` for required IM fields when enabled

  **Must NOT do**:
  - Do NOT remove existing `feishu:` config section — maintain backward compatibility
  - Do NOT make `im.platforms` required — it's optional, Feishu can still use old config path
  - Do NOT put secrets in example file — use empty strings

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (after T17)
  - **Blocks**: T20
  - **Blocked By**: T17 (main.go rewired)

  **References**:

  **Pattern References**:
  - `pkg/config/config.go` — existing config structure with `FeishuConfig`, `setDefaults()`, `validateConfig()`
  - `configs/config.yaml.example` — existing example config to update

  **WHY Each Reference Matters**:
  - Must follow exact config conventions: `mapstructure` tags, `setDefaults()` for fallbacks, both files updated in sync

  **Acceptance Criteria**:
  - [ ] `pkg/config/config.go` has `IMConfig` and `IMPlatformConfig` structs
  - [ ] `configs/config.yaml.example` has new IM section
  - [ ] Defaults set for rate_limit_max (20), rate_limit_window (60s), max_retries (3)
  - [ ] Backward compatible: old `feishu:` config still works

  **QA Scenarios**:

  ```
  Scenario: Config loads with new IM section
    Tool: Bash
    Preconditions: config.go updated with new structs
    Steps:
      1. Run `go build ./pkg/config/...`
      2. Assert compiles
      3. Run `go test ./pkg/config/... -v` (if tests exist)
    Expected Result: Config struct compiles and loads correctly
    Failure Indicators: Compilation errors or config parse failures
    Evidence: .sisyphus/evidence/task-18-config.txt
  ```

  **Commit**: YES (groups with T17 → C9)
  - Message: `refactor(im): rewire main.go to new IM architecture with multi-platform config`
  - Files: `pkg/config/config.go`, `configs/config.yaml.example`
  - Pre-commit: `go build ./...`

- [x] 19. Integration Tests for Full Flow

  **What to do**:
  - Create `internal/service/im/integration_test.go` — end-to-end test of the full IM flow:
    ```go
    // TestFullIMFlow tests:
    //   webhook event → bridge.HandleInboundMessage() → session created → agent triggered
    //   agent chunk → bridge.OnAgentChunk() → delivery → adapter.SendText()
    func TestFullIMFlow(t *testing.T)
    ```
  - Use fakes for all external dependencies:
    - Fake `IMAdapter` that records sent messages
    - In-memory SQLite for repos
    - Fake orchestrator that immediately fires chunk callbacks
  - Test scenarios:
    - New user message → session created → agent spawned → text chunk → message sent to adapter
    - Duplicate message → dedup skips it
    - Rate limited message → dropped with warning
    - Error chunk → error card sent
    - Status chunk → completion card sent
    - Multi-platform routing: Feishu session → Feishu adapter, not Slack adapter
  - Verify cleanup: session lock released, buffers flushed

  **Must NOT do**:
  - Do NOT call actual lark-cli or any external process
  - Do NOT require network access
  - Do NOT test streaming cards here (separate unit tests in T16)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Integration testing requires wiring all components together with fakes
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (after T17)
  - **Blocks**: —
  - **Blocked By**: T17 (all components wired up)

  **References**:

  **Pattern References**:
  - `internal/service/im/bridge_service.go` — the service being integration tested
  - `internal/service/im/bridge_service_test.go` — unit tests to complement
  - `internal/repo/im_session_repo_test.go:203-224` — SQLite test setup pattern

  **WHY Each Reference Matters**:
  - Integration tests exercise the full pipeline that unit tests cover in isolation
  - SQLite pattern from repo tests is reused for DB setup

  **Acceptance Criteria**:
  - [ ] `internal/service/im/integration_test.go` has ≥6 test scenarios
  - [ ] All tests pass with `go test -race`
  - [ ] No external dependencies required (pure in-process)

  **QA Scenarios**:

  ```
  Scenario: Full IM flow integration tests pass
    Tool: Bash
    Preconditions: integration_test.go exists
    Steps:
      1. Run `go test -race ./internal/service/im/ -run TestFullIMFlow -v`
      2. Assert all sub-tests pass
    Expected Result: Complete webhook→agent→delivery flow verified
    Failure Indicators: Any sub-test failure
    Evidence: .sisyphus/evidence/task-19-integration.txt
  ```

  **Commit**: YES (C10)
  - Message: `test(im): add integration tests for full webhook-to-delivery flow`
  - Files: `internal/service/im/integration_test.go`
  - Pre-commit: `make test`

### Phase 3 — Extensibility Scaffolding

### Wave 6 — Registry, Stubs, Validation

- [x] 20. Adapter Factory and Registry

  **What to do**:
  - Create new file `internal/service/im/registry.go`:
    ```go
    // AdapterFactory creates an IMAdapter for a given platform config.
    type AdapterFactory func(cfg IMPlatformConfig, logger *zap.Logger) (IMAdapter, error)

    // Registry maps platform types to their factory functions.
    // Follows the same factory pattern as agent adapters (NewAdapter in adapter.go).
    type Registry struct {
        factories map[string]AdapterFactory
    }

    func NewRegistry() *Registry

    // Register adds a factory for a platform type.
    func (r *Registry) Register(platformType string, factory AdapterFactory)

    // Create instantiates an adapter for the given config.
    func (r *Registry) Create(cfg IMPlatformConfig, logger *zap.Logger) (IMAdapter, error)
    ```
  - Register Feishu factory explicitly in `main.go` (NO global `DefaultRegistry`, NO `init()`):
    ```go
    imRegistry := im.NewRegistry()
    imRegistry.Register("feishu", func(cfg im.IMPlatformConfig, logger *zap.Logger) (im.IMAdapter, error) {
        client := im.NewLarkCLIClient(cfg.LarkCLIPath, logger)
        return im.NewFeishuAdapter(client, logger), nil
    })
    ```
  - Update `main.go` (T17 code) to use explicit registry:
    ```go
    for _, platformCfg := range cfg.IM.Platforms {
        if !platformCfg.Enabled { continue }
        adapter, err := imRegistry.Create(platformCfg, logger)
        // ... create delivery, register with bridge ...
    }
    ```
  - Create `internal/service/im/registry_test.go`:
    - Test registering and creating adapter
    - Test unknown platform returns error
    - Test explicit registry wiring pattern

  **Must NOT do**:
  - Do NOT use `sync.Map` — regular map with explicit locking
  - Do NOT use package-level `var DefaultRegistry` — this violates the "no global mutable state" guardrail
  - Do NOT use `init()` for registration — all registration happens explicitly in `main.go`, matching the codebase's DI pattern

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with T21, T22, T23)
  - **Blocks**: T21, T22
  - **Blocked By**: T17 (main.go rewired), T18 (config schema)

  **References**:

  **Pattern References**:
  - `internal/service/agent/adapter.go:51-66` — `NewAdapter()` factory with type switch (existing pattern)
  - Claude-to-IM `channel-adapter.ts:121-136` — `registerAdapterFactory()` and `createAdapter()`

  **WHY Each Reference Matters**:
  - Agent adapter uses type switch; IM adapter uses map-based registry — both are valid, map is more extensible for plugins
  - Claude-to-IM's registry is the blueprint; Go version uses a struct instead of module-level Map

  **Acceptance Criteria**:
  - [ ] `internal/service/im/registry.go` exists with `Registry`
  - [ ] Feishu factory registered
  - [ ] Unknown platform returns descriptive error
  - [ ] `registry_test.go` passes

  **QA Scenarios**:

  ```
  Scenario: Registry creates Feishu adapter
    Tool: Bash
    Preconditions: registry.go exists with Feishu registered
    Steps:
      1. Run `go test ./internal/service/im/ -run TestRegistry -v`
      2. Assert Feishu adapter created successfully
      3. Assert unknown platform "telegram" returns error
    Expected Result: Factory dispatch works correctly
    Failure Indicators: Wrong adapter type or no error for unknown
    Evidence: .sisyphus/evidence/task-20-registry.txt
  ```

  **Commit**: YES (groups with T21, T22, T23 → C11)
  - Message: `feat(im): add adapter registry, platform stubs, input validation`
  - Files: `internal/service/im/registry.go`, `internal/service/im/registry_test.go`
  - Pre-commit: `go test ./internal/service/im/...`

- [x] 21. Slack Adapter Stub

  **What to do**:
  - Create new file `internal/service/im/slack_adapter.go`:
    ```go
    var ErrNotImplemented = errors.New("not implemented")

    // SlackAdapter is a placeholder for future Slack integration.
    type SlackAdapter struct{}

    func NewSlackAdapter() *SlackAdapter

    func (a *SlackAdapter) Platform() string { return "slack" }
    func (a *SlackAdapter) SendText(ctx context.Context, chatID, text string) SendResult {
        return SendResult{OK: false, Error: ErrNotImplemented.Error()}
    }
    // ... all other IMAdapter methods return ErrNotImplemented ...
    func (a *SlackAdapter) MaxMessageLength() int { return 4000 } // Slack limit
    func (a *SlackAdapter) CheckHealth(ctx context.Context) error { return ErrNotImplemented }
    ```
  - Add compile-time check: `var _ IMAdapter = (*SlackAdapter)(nil)`
  - Do NOT register in any global — registration happens in `main.go` via the explicit `Registry` instance (T20)
  - Create minimal test: `TestSlackAdapter_SatisfiesInterface`

  **Must NOT do**:
  - Do NOT implement actual Slack API calls
  - Do NOT add Slack SDK dependencies to `go.mod`
  - Do NOT add Slack webhook handler

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with T20, T22, T23)
  - **Blocks**: —
  - **Blocked By**: T11 (IMAdapter interface), T20 (registry)

  **References**:

  **Pattern References**:
  - `internal/service/im/adapter.go` — `IMAdapter` interface to satisfy
  - `internal/service/im/feishu_adapter.go` — pattern to follow for method signatures

  **Acceptance Criteria**:
  - [ ] `internal/service/im/slack_adapter.go` compiles
  - [ ] `var _ IMAdapter = (*SlackAdapter)(nil)` passes
  - [ ] All methods return `ErrNotImplemented`
  - [ ] Registered in default registry

  **QA Scenarios**:

  ```
  Scenario: Slack adapter satisfies interface
    Tool: Bash
    Steps:
      1. Run `go build ./internal/service/im/...`
      2. Assert compiles
    Expected Result: Compile-time interface check passes
    Failure Indicators: "does not implement IMAdapter"
    Evidence: .sisyphus/evidence/task-21-slack-stub.txt
  ```

  **Commit**: YES (groups with T20, T22, T23 → C11)
  - Files: `internal/service/im/slack_adapter.go`
  - Pre-commit: `go build ./...`

- [x] 22. Discord Adapter Stub

  **What to do**:
  - Create new file `internal/service/im/discord_adapter.go`:
    - Same pattern as Slack stub (T21)
    - `Platform()` returns `"discord"`
    - `MaxMessageLength()` returns `2000` (Discord limit)
    - All methods return `ErrNotImplemented`
  - Add compile-time check: `var _ IMAdapter = (*DiscordAdapter)(nil)`
  - Do NOT register in any global — registration happens in `main.go` via the explicit `Registry` instance (T20)

  **Must NOT do**:
  - Same constraints as T21

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with T20, T21, T23)
  - **Blocks**: —
  - **Blocked By**: T11 (IMAdapter interface), T20 (registry)

  **References**:
  - Same as T21, with Discord-specific max length (2000 chars)

  **Acceptance Criteria**:
  - [ ] `internal/service/im/discord_adapter.go` compiles and satisfies `IMAdapter`

  **QA Scenarios**:

  ```
  Scenario: Discord adapter satisfies interface
    Tool: Bash
    Steps:
      1. Run `go build ./internal/service/im/...`
      2. Assert compiles
    Expected Result: Compile-time check passes
    Evidence: .sisyphus/evidence/task-22-discord-stub.txt
  ```

  **Commit**: YES (groups with T20, T21, T23 → C11)
  - Files: `internal/service/im/discord_adapter.go`
  - Pre-commit: `go build ./...`

- [x] 23. Input Validation Module

  **What to do**:
  - Create new file `internal/service/im/validation.go`:
    ```go
    const MaxMessageLength = 10000 // Global maximum inbound message length

    // DangerousPattern describes a potentially dangerous input pattern.
    type DangerousPattern struct {
        Pattern *regexp.Regexp
        Reason  string
    }

    var dangerousPatterns = []DangerousPattern{
        {regexp.MustCompile(`\x00`), "null byte"},
        {regexp.MustCompile(`\.\.[/\\]`), "path traversal"},
        {regexp.MustCompile(`\$\(`), "command substitution $()"},
        {regexp.MustCompile("`[^`]*`"), "backtick command substitution"},
        {regexp.MustCompile(`;\s*(rm|cat|curl|wget|chmod|chown)\b`), "chained dangerous command"},
        {regexp.MustCompile(`\|\s*(bash|sh|zsh|exec)\b`), "pipe to shell"},
    }

    // ValidateInboundMessage checks an incoming message for dangerous patterns and length.
    // Returns (sanitized string, error). Error is non-nil if message should be rejected.
    func ValidateInboundMessage(text string) (string, error)

    // IsDangerous checks if text contains any dangerous patterns.
    // Returns (true, reason) if dangerous, (false, "") if safe.
    func IsDangerous(text string) (bool, string)
    ```
  - `ValidateInboundMessage` logic:
    1. Check empty → return error
    2. Check length > MaxMessageLength → return error
    3. Check `IsDangerous()` → log warning but do NOT reject (user messages should flow through, just sanitize null bytes)
    4. Remove null bytes: `strings.ReplaceAll(text, "\x00", "")`
    5. Trim whitespace
    6. Return sanitized text
  - Integrate into `IMBridgeService.HandleInboundMessage()`: validate text before processing
  - Create `internal/service/im/validation_test.go`:
    - Normal text passes through unchanged
    - Null bytes removed
    - Overlength rejected
    - Each dangerous pattern detected
    - Empty string rejected

  **Must NOT do**:
  - Do NOT reject messages with command-like patterns (users discuss code!) — only log warning
  - Do NOT strip Unicode or non-ASCII characters
  - Do NOT add rate limiting here (already in DeliveryService)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with T20, T21, T22)
  - **Blocks**: —
  - **Blocked By**: T17 (bridge service wired, integration point exists)

  **References**:

  **Pattern References**:
  - Claude-to-IM `validators.ts:21-30` — dangerous pattern definitions
  - Claude-to-IM `validators.ts:38-61` — `validateWorkingDirectory()` validation flow

  **WHY Each Reference Matters**:
  - Pattern definitions adapted from Claude-to-IM, but behavior differs: we log and sanitize, not reject (Colink handles user messages about code, which may contain command-like text)

  **Acceptance Criteria**:
  - [ ] `internal/service/im/validation.go` exists
  - [ ] Null bytes removed from messages
  - [ ] Overlength messages rejected
  - [ ] Dangerous patterns detected and logged (not rejected)
  - [ ] `validation_test.go` has ≥6 test cases

  **QA Scenarios**:

  ```
  Scenario: Validation handles all edge cases
    Tool: Bash
    Preconditions: validation.go and validation_test.go exist
    Steps:
      1. Run `go test ./internal/service/im/ -run TestValidat -v`
      2. Assert all tests pass
    Expected Result: Normal text passes, null bytes stripped, dangerous patterns logged
    Failure Indicators: Test failures
    Evidence: .sisyphus/evidence/task-23-validation.txt
  ```

  **Commit**: YES (groups with T20, T21, T22 → C11)
  - Message: `feat(im): add adapter registry, platform stubs, input validation`
  - Files: `internal/service/im/validation.go`, `internal/service/im/validation_test.go`
  - Pre-commit: `go test ./internal/service/im/...`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] TF1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in `.sisyphus/evidence/`. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] TF2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...` + `go build ./...` + `make test`. Review all changed files for: `interface{}` abuse, empty catches, fmt.Println in prod, commented-out code, unused imports. Check for AI slop: excessive comments, over-abstraction, generic names (data/result/item/temp). Verify all error returns are checked.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] TF3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration: webhook → bridge → adapter → delivery → lark-cli flow. Test edge cases: empty message, very long message, rapid messages, invalid JSON. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] TF4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (`git log`/`git diff`). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

| Commit | Tasks | Message | Pre-commit |
|--------|-------|---------|------------|
| C1 | T1 | `refactor(im): move stranded IM files to correct path` | `go build ./...` |
| C2 | T2, T3 | `fix(im): resolve webhook context leak and buffer flush race` | `go test ./internal/service/im/...` |
| C3 | T4, T5, T6 | `feat(im): add dedup cache, rate limiter, and error classification` | `go test ./internal/service/im/...` |
| C4 | T7, T8, T9 | `feat(im): integrate retry delivery, dedup, and rate limiting into bridge` | `go test ./internal/service/im/...` |
| C5 | T10 | `test(im): add comprehensive tests for bridge service and webhook handler` | `make test` |
| C6 | T11, T12, T13 | `refactor(im): define IMAdapter interface, delivery service, session locks` | `go test ./internal/service/im/...` |
| C7 | T14, T15 | `refactor(im): extract FeishuAdapter and generic IMBridgeService` | `make test` |
| C8 | T16 | `feat(im/feishu): add streaming card support with throttle` | `go test ./internal/service/im/...` |
| C9 | T17, T18 | `refactor(im): rewire main.go to new IM architecture with multi-platform config` | `make test` |
| C10 | T19 | `test(im): add integration tests for full webhook-to-delivery flow` | `make test` |
| C11 | T20, T21, T22, T23 | `feat(im): add adapter registry, platform stubs, input validation` | `make test` |

---

## Success Criteria

### Verification Commands
```bash
go build ./...                    # Expected: zero errors
go vet ./...                      # Expected: zero warnings
make test                         # Expected: all pass, coverage > existing baseline
go test ./internal/service/im/... -v -cover  # Expected: >80% coverage for IM package
```

### Final Checklist
- [ ] All "Must Have" items present and verified
- [ ] All "Must NOT Have" items absent (searched and confirmed)
- [ ] All tests pass (`make test`)
- [ ] Feishu webhook flow works end-to-end
- [ ] No stranded imports referencing `isdp/internal/service/im`
- [ ] Config schema documented in `config.yaml.example`
- [ ] Each phase independently functional (Phase 1 works without Phase 2)
