// internal/service/agent/plugins/open_claw/gateway_manager_windows.go
// Windows-specific Gateway Manager helpers
//go:build windows

package open_claw

import (
	"os/exec"
	"syscall"
)

// hideCommandLineWindow hides the command line window on Windows.
func hideCommandLineWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags = syscall.CREATE_NO_WINDOW
}