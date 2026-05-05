//go:build !windows

package open_code

import (
	"os/exec"
)

// hideCommandLineWindow 非Windows平台无需隐藏窗口
func hideCommandLineWindow(cmd *exec.Cmd) {
	// No-op on non-Windows platforms
}