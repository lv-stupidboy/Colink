// Package exec provides cross-platform command execution utilities
// This file contains Windows-specific implementations
package exec

import (
	"os/exec"
	"syscall"
)

const CREATE_NO_WINDOW = 0x08000000

// setNoWindow configures the command to not show a console window on Windows
func setNoWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	// CREATE_NO_WINDOW prevents the console window from being created
	cmd.SysProcAttr.CreationFlags = CREATE_NO_WINDOW
}