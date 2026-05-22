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
// taskkill /T kills the entire process tree including child processes
// (MCP server, tool execution processes, etc.)
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