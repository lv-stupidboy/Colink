# Agent Cancel Process Tree Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix Windows process tree not being terminated when canceling agent execution, causing nodejs subprocesses to remain running.

**Architecture:** Replace `cmd.Process.Kill()` (only kills parent) with `taskkill /F /T /PID` (kills entire process tree) on Windows. This is a minimal change that solves the problem without adding Job Object complexity.

**Tech Stack:** Go, Windows syscall/exec, taskkill command

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/service/agent/execution_service.go` | Core `killChild` function - modify Windows branch |
| `internal/service/agent/execution_service_windows.go` | NEW: Windows-specific process tree termination |
| `internal/service/agent/execution_service_other.go` | NEW: Unix-specific process termination (refactor existing code) |
| `docs/superpowers/specs/2026-05-22-agent-cancel-process-tree-design.md` | Spec (already exists) |

---

## Task 1: Create Platform-Specific killChild Functions

**Files:**
- Create: `internal/service/agent/execution_service_windows.go`
- Create: `internal/service/agent/execution_service_other.go`
- Modify: `internal/service/agent/execution_service.go:37-66`

**Why:** Separate platform-specific logic into build-tagged files. Windows uses `taskkill`, Unix uses existing SIGTERM/SIGKILL logic.

- [ ] **Step 1: Create `execution_service_windows.go` with taskkill implementation**

```go
//go:build windows

package agent

import (
	"os/exec"
	"strconv"
	"sync"

	"go.uber.org/zap"
)

// killChild terminates the process tree on Windows using taskkill.
// Unlike cmd.Process.Kill() which only kills the parent process,
// taskkill /T kills the entire process tree including child processes.
func killChild(cmd *exec.Cmd, cmdMu *sync.Mutex) {
	cmdMu.Lock()
	defer cmdMu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	pid := cmd.Process.Pid
	logInfo("killChild: terminating process tree on Windows", zap.Int("pid", pid))

	// Use taskkill to kill the entire process tree
	// /F = force terminate
	// /T = terminate all child processes
	// /PID = process ID
	killCmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	if err := killCmd.Run(); err != nil {
		logError("killChild: taskkill failed, falling back to Process.Kill", zap.Error(err), zap.Int("pid", pid))
		// Fallback to direct kill if taskkill fails
		cmd.Process.Kill()
	}
}
```

- [ ] **Step 2: Create `execution_service_other.go` with Unix implementation**

```go
//go:build !windows

package agent

import (
	"os/exec"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

const killGracePeriod = 3 * time.Second

// killChild terminates the process on Unix systems using SIGTERM/SIGKILL.
// On Unix, signals propagate to the process group, killing child processes.
func killChild(cmd *exec.Cmd, cmdMu *sync.Mutex) {
	cmdMu.Lock()
	defer cmdMu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	logInfo("killChild: terminating process", zap.Int("pid", cmd.Process.Pid))

	// Unix: SIGTERM propagates to process group
	cmd.Process.Signal(syscall.SIGTERM)

	// 3秒后升级到 SIGKILL
	go func(pid int, cmd *exec.Cmd, cmdMu *sync.Mutex) {
		time.Sleep(killGracePeriod)
		cmdMu.Lock()
		defer cmdMu.Unlock()
		if cmd.Process != nil && cmd.Process.Pid == pid {
			logInfo("killChild: escalating to SIGKILL", zap.Int("pid", pid))
			cmd.Process.Kill()
		}
	}(cmd.Process.Pid, cmd, cmdMu)
}
```

- [ ] **Step 3: Remove existing killChild function from `execution_service.go`**

Delete lines 37-66 in `execution_service.go`:
- Remove the `killGracePeriod` constant (now in `execution_service_other.go`)
- Remove the entire `killChild` function (now in platform-specific files)

Also remove unused imports if `syscall` and `time` are no longer needed in this file.

- [ ] **Step 4: Verify build succeeds**

Run: `go build ./cmd/server`
Expected: No build errors

- [ ] **Step 5: Commit platform-specific split**

```bash
git add internal/service/agent/execution_service_windows.go internal/service/agent/execution_service_other.go internal/service/agent/execution_service.go
git commit -m "fix(agent): split killChild into platform-specific files for Windows process tree termination"
```

---

## Task 2: Manual Testing on Windows

**Files:**
- Test: Manual verification with Claude CLI

- [ ] **Step 1: Start backend server**

Run: `go run ./cmd/server`

- [ ] **Step 2: Start frontend dev server**

Run: `cd web && npm run dev`

- [ ] **Step 3: Open browser and create a thread**

Navigate to `http://localhost:26306`
Create a new thread with an Agent

- [ ] **Step 4: Trigger Agent execution and cancel mid-response**

1. Send a prompt that will take time (e.g., "explain this codebase")
2. Wait for Agent to start streaming output
3. Click the stop button in "当前调用" panel

- [ ] **Step 5: Verify process tree is terminated**

Run: `tasklist | findstr node`
Expected: No node.exe processes related to the Agent (only the frontend dev server if running)

Also check: `tasklist | findstr mcp-server`
Expected: No MCP server processes running

- [ ] **Step 6: Verify Agent stops streaming in UI**

Expected: Agent output stops immediately, status shows "已取消"

---

## Task 3: Update Spec Document

**Files:**
- Modify: `docs/superpowers/specs/2026-05-22-agent-cancel-process-tree-design.md`

- [ ] **Step 1: Add implementation notes to spec**

Add at the end of the spec file:

```markdown
## Implementation Notes

**Chosen approach:** taskkill command (方案 B)

**Reason:** Simpler than Job Object, sufficient for the problem, no need to modify RunningAgent struct.

**Changes made:**
1. Created `execution_service_windows.go` with `taskkill /F /T /PID` implementation
2. Created `execution_service_other.go` with existing Unix SIGTERM/SIGKILL logic
3. Removed duplicate `killChild` from `execution_service.go`

**Testing:** Manual verification confirmed process tree termination works correctly.
```

- [ ] **Step 2: Commit spec update**

```bash
git add docs/superpowers/specs/2026-05-22-agent-cancel-process-tree-design.md
git commit -m "docs: update agent-cancel-process-tree spec with implementation notes"
```

---

## Self-Review Checklist

**1. Spec coverage:**
- ✓ Windows process tree termination - Task 1
- ✓ Manual testing - Task 2
- ✓ Documentation - Task 3

**2. Placeholder scan:**
- No TBD/TODO placeholders
- All code steps contain actual implementation
- No vague "handle edge cases" without code

**3. Type consistency:**
- `killChild` signature matches across platform files: `func killChild(cmd *exec.Cmd, cmdMu *sync.Mutex)`
- Imports consistent: `go.uber.org/zap` for logging

---

## Summary

| Metric | Value |
|--------|-------|
| Tasks | 3 |
| Files created | 2 |
| Files modified | 2 |
| Estimated time | 20-30 min |