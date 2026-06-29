package exec

import (
	"context"
	"strings"
	"testing"
)

func TestCommandConstructors(t *testing.T) {
	cmd := Command("echo", "hello")
	if cmd.Path == "" || len(cmd.Args) != 2 || cmd.Args[1] != "hello" {
		t.Fatalf("unexpected command: path=%q args=%v", cmd.Path, cmd.Args)
	}

	ctxCmd := CommandContext(context.Background(), "echo", "hello")
	if ctxCmd.Path == "" || len(ctxCmd.Args) != 2 || ctxCmd.Args[1] != "hello" {
		t.Fatalf("unexpected context command: path=%q args=%v", ctxCmd.Path, ctxCmd.Args)
	}
}

func TestGitCommandInjectsSSHConfig(t *testing.T) {
	cmd := GitCommand("git", "status")
	assertGitSSHCommand(t, cmd.Env)

	ctxCmd := GitCommandContext(context.Background(), "git", "status")
	assertGitSSHCommand(t, ctxCmd.Env)
}

func TestInjectGitSSHConfigReplacesExistingEnv(t *testing.T) {
	cmd := Command("git")
	cmd.Env = []string{"GIT_SSH_COMMAND=old", "OTHER=value"}
	injectGitSSHConfig(cmd)
	if len(cmd.Env) != 2 {
		t.Fatalf("expected env length preserved, got %v", cmd.Env)
	}
	assertGitSSHCommand(t, cmd.Env)
}

func assertGitSSHCommand(t *testing.T, env []string) {
	t.Helper()
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_SSH_COMMAND=") {
			if entry != "GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=no" {
				t.Fatalf("unexpected git ssh command %q", entry)
			}
			return
		}
	}
	t.Fatalf("missing GIT_SSH_COMMAND in env %v", env)
}
