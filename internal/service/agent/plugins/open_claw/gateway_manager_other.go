// internal/service/agent/plugins/open_claw/gateway_manager_other.go
// Non-Windows Gateway Manager helpers
//go:build !windows

package open_claw

import "os/exec"

// hideCommandLineWindow is a no-op on non-Windows platforms.
func hideCommandLineWindow(cmd *exec.Cmd) {
	// No special handling needed on Unix/macOS
}