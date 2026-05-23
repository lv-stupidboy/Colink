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