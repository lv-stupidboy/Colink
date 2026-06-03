---
name: agent-cancel-process-tree-bug
description: Windows 上取消 Agent 时进程树未被完全终止的问题分析
metadata:
  type: project
---

# Bug 分析：Agent 取消后进程残留

**问题描述**：在对话页面，当 Agent 回答未完成时，用户点击"当前调用"栏中的结束会话按钮，前端会话条目会成功结束，但后端运行的 agent 调用未停止，agent 仍会继续回答，后端会有 nodejs 进程残留。

## 根因分析

### 1. 代码流程追踪

**前端调用链**：
```
AgentStatusCard.tsx:handleCancel()
  → useAppStore.cancelAgent(invocationId)
    → api.invocations.cancel(invocationId)
      → POST /api/invocations/:id/cancel
```

**后端调用链**：
```
invocation_handler.go:Cancel()
  → Orchestrator.CancelAgent()
    → ExecutionService.CancelAgent()
      → killChild(cmd, cmdMu)  // Windows: cmd.Process.Kill()
```

### 2. 问题根源：Windows 进程树管理缺失

**关键代码位置**：`internal/service/agent/execution_service.go:37-66`

```go
func killChild(cmd *exec.Cmd, cmdMu *sync.Mutex) {
    // Windows 不支持 SIGTERM，直接用 Kill
    if runtime.GOOS == "windows" {
        cmd.Process.Kill()  // ❌ 只杀死父进程，不杀死进程树
        return
    }
    // Unix: SIGTERM 会传播到整个进程组
    cmd.Process.Signal(syscall.SIGTERM)
}
```

**Windows 特有问题**：
- Go 的 `cmd.Process.Kill()` 只杀死直接进程（PID 对应的进程）
- **不会杀死子进程树**（CLI spawn 的 MCP server、工具执行进程等）
- Windows 没有 Unix 的进程组（PGID）概念

### 3. 进程架构

```
Go Backend
  └── cmd.Process (node.exe - Claude CLI)  ← Kill() 只杀死这个
        └── MCP Server (mcp-server.exe)     ← 残留
        └── Tool processes (nodejs)         ← 残留
        └── Sub-agent processes             ← 残留
```

### 4. 现有 SysProcAttr 分析

**文件位置**：
- `internal/service/agent/plugins/claude_code/adapter_windows.go`
- `internal/service/agent/plugins/acp/platform_windows.go`

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    HideWindow:    true,
    CreationFlags: 0x08000000, // CREATE_NO_WINDOW - 只隐藏窗口
}
```

**问题**：`CREATE_NO_WINDOW` 只控制窗口显示，**不管理进程生命周期**。

## 解决方案

### 方案 A：Windows Job Object（推荐）

**原理**：将进程放入 Job Object，Job 关闭时所有关联进程一起终止。

**优点**：
- 系统级保证，进程退出时 Job 自动关闭
- 无需手动追踪子进程 PID

**实现要点**：
```go
import "golang.org/x/sys/windows"

// 创建 Job Object
job, _ := windows.CreateJobObject(nil, nil)

// 设置 Job 限制：进程退出时杀死所有子进程
var info windows.JOBOBJECT_BASIC_LIMIT_INFORMATION
info.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
windows.SetInformationJobObject(job, windows.JobObjectBasicLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))

// 将进程分配到 Job
windows.AssignProcessToJobObject(job, windows.Handle(cmd.Process.Pid))
```

### 方案 B：taskkill 命令

**原理**：使用 Windows 内置命令杀死进程树。

**优点**：
- 实现简单，无需额外依赖

**缺点**：
- 需要调用外部命令
- 可能受权限限制

**实现要点**：
```go
func killProcessTree(pid int) error {
    cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
    return cmd.Run()
}
```

### 方案 C：进程 PID 追踪

**原理**：CLI 进程启动后，通过 API 获取所有子进程 PID，取消时逐个杀死。

**缺点**：
- 需要额外进程追踪机制
- 可能遗漏动态创建的进程

## 推荐方案

**方案 A（Job Object）** 是最佳选择：
1. 系统级保证，不会遗漏子进程
2. Go 标准库支持（`golang.org/x/sys/windows`）
3. 性能开销小

**Why:** Windows 上进程残留会导致资源泄漏和 Agent 继续执行，Job Object 是 Windows 官方推荐的进程组管理机制。

**How to apply:**
1. 在 `execution_service.go` 中添加 Job Object 管理
2. 修改 `hideCommandLineWindow` 函数，同时创建 Job Object
3. 将 CLI 进程分配到 Job Object
4. 取消时关闭 Job Object（自动杀死所有子进程）

## 相关文件

| 文件 | 作用 |
|------|------|
| `internal/service/agent/execution_service.go:37-66` | `killChild` 函数 |
| `internal/service/agent/plugins/claude_code/adapter_windows.go` | Claude CLI Windows 配置 |
| `internal/service/agent/plugins/acp/platform_windows.go` | ACP Windows 配置 |

## 下一步

需要 @SuperPowers全栈开发工程师 实现修复方案。

[[agent-cancel-process-tree-bug]]

---

## Implementation Notes

**Chosen approach:** taskkill command (方案 B)

**Reason:** Simpler than Job Object, sufficient for the problem, no need to modify RunningAgent struct or add Job Object lifecycle management.

**Changes made:**
1. Created `execution_service_windows.go` with `taskkill /F /T /PID` implementation
2. Created `execution_service_other.go` with existing Unix SIGTERM/SIGKILL logic
3. Removed duplicate `killChild` and `killGracePeriod` from `execution_service.go`
4. Commit: `04bc1c1`

**Testing:** Pending manual verification on Windows.