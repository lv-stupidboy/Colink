// Package exec provides cross-platform command execution utilities
package exec

import (
	"context"
	"os/exec"
)

// CommandContext creates an exec.Cmd with context that suppresses console window on Windows
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	setNoWindow(cmd)
	return cmd
}

// Command creates an exec.Cmd that suppresses console window on Windows
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	setNoWindow(cmd)
	return cmd
}