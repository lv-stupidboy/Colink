// Package exec provides cross-platform command execution utilities
// This file contains non-Windows implementations (no-op)
//go:build !windows

package exec

import (
	"os/exec"
)

// setNoWindow is a no-op on non-Windows platforms
func setNoWindow(cmd *exec.Cmd) {
	// No special handling needed on non-Windows platforms
}