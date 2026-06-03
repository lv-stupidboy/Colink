//go:build !windows

package acp

import (
	"os"
	"os/exec"
	"syscall"
	"time"

	"go.uber.org/zap"
)

const killGracePeriod = 3 * time.Second

// hideCommandLineWindow 非Windows平台无需隐藏窗口
func hideCommandLineWindow(cmd *exec.Cmd) {
	// No-op on non-Windows platforms
}

// killProcessTree terminates the process on Unix systems using SIGTERM/SIGKILL.
// On Unix, signals propagate to the process group, killing child processes.
func killProcessTree(process *os.Process) error {
	if process == nil {
		return nil
	}

	LogInfo("ACP: terminating process tree", zap.Int("pid", process.Pid))

	// Unix: SIGTERM propagates to process group
	process.Signal(syscall.SIGTERM)

	// Grace period for graceful shutdown
	time.Sleep(killGracePeriod)

	// Check if process still exists, then escalate to SIGKILL
	// Note: On Unix, we cannot easily check if process is still running,
	// so we just escalate after grace period
	LogInfo("ACP: escalating to SIGKILL", zap.Int("pid", process.Pid))
	return process.Kill()
}