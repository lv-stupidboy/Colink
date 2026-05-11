//go:build windows

package acp

import (
	"os/exec"
	"syscall"
)

// hideCommandLineWindow 隐藏命令行窗口（Windows平台）
func hideCommandLineWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}