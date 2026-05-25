// Package exec provides cross-platform command execution utilities
package exec

import (
	"context"
	"os"
	"os/exec"
	"strings"
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

// GitCommand creates an exec.Cmd for git with SSH auto-accept enabled.
// Sets GIT_SSH_COMMAND to skip host key verification, avoiding interactive
// prompts on first SSH connection to a new host.
func GitCommand(name string, args ...string) *exec.Cmd {
	cmd := Command(name, args...)
	injectGitSSHConfig(cmd)
	return cmd
}

// GitCommandContext creates an exec.Cmd for git with context and SSH auto-accept enabled.
func GitCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := CommandContext(ctx, name, args...)
	injectGitSSHConfig(cmd)
	return cmd
}

// injectGitSSHConfig sets GIT_SSH_COMMAND to auto-accept new SSH host keys
func injectGitSSHConfig(cmd *exec.Cmd) {
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	sshCmd := "ssh -o StrictHostKeyChecking=no"
	found := false
	for i, env := range cmd.Env {
		if strings.HasPrefix(env, "GIT_SSH_COMMAND=") {
			cmd.Env[i] = "GIT_SSH_COMMAND=" + sshCmd
			found = true
			break
		}
	}
	if !found {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND="+sshCmd)
	}
}