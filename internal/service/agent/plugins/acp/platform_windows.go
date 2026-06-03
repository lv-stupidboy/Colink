//go:build windows

package acp

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"go.uber.org/zap"
)

// hideCommandLineWindow 隐藏命令行窗口（Windows平台）
func hideCommandLineWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

// killProcessTree terminates the entire process tree on Windows using taskkill.
// Unlike os.Process.Kill() which only kills the parent process,
// taskkill /T kills the entire process tree including child processes
// (MCP server, tool execution processes like bun, etc.)
func killProcessTree(process *os.Process) error {
	if process == nil {
		return nil
	}

	pid := process.Pid
	LogInfo("ACP: terminating process tree on Windows", zap.Int("pid", pid))

	// Use taskkill to kill the entire process tree
	// /F = force terminate
	// /T = terminate all child processes
	// /PID = process ID
	killCmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	if err := killCmd.Run(); err != nil {
		LogError("ACP: taskkill failed, falling back to Process.Kill", zap.Error(err), zap.Int("pid", pid))
		// Fallback to direct kill if taskkill fails
		return process.Kill()
	}

	LogInfo("ACP: process tree terminated successfully", zap.Int("pid", pid))
	return nil
}