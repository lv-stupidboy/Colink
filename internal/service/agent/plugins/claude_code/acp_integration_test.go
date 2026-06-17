package claude_code

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/google/uuid"
)

func TestClaudeACPStartSessionWithManagedMCP(t *testing.T) {
	if _, err := exec.LookPath("claude-agent-acp"); err != nil {
		t.Skip("claude-agent-acp is not installed")
	}

	adapter := NewClaudeACPAdapter(&model.BaseAgent{
		ID:           uuid.New(),
		Type:         model.BaseAgentType("claude_code"),
		DefaultModel: "claude-sonnet-4-20250514",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sessionID := "claude-acp-mcp-integration-" + uuid.NewString()
	req := &agent.ExecutionRequest{
		WorkDir: t.TempDir(),
		MCPServers: []*model.MCPServer{
			{
				ID:        uuid.New(),
				Name:      "colink-test-mcp",
				Transport: model.MCPTransportStdio,
				Command:   "node",
				Args:      []string{"-e", "process.exit(0)"},
				Env:       map[string]string{"COLINK_MCP_TEST": "1"},
				Status:    model.MCPStatusActive,
			},
		},
	}

	if err := adapter.StartSession(ctx, sessionID, req); err != nil {
		t.Fatalf("failed to start Claude ACP session with managed MCP: %v", err)
	}
	defer adapter.StopSession(sessionID)

	if status := adapter.GetSessionStatus(sessionID); status != agent.SessionStatusRunning {
		t.Fatalf("expected running session, got %s", status)
	}
}
