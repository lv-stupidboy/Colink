package agent

import (
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestNewAdapter_NilBaseAgent(t *testing.T) {
	adapter := NewAdapter(nil)
	if adapter != nil {
		t.Error("Expected nil adapter for nil base agent")
	}
}

func TestNewAdapter_ClaudeCode(t *testing.T) {
	baseAgent := &model.BaseAgent{
		ID:           uuid.New(),
		Name:         "Test Claude",
		Type:         model.BaseAgentTypeClaudeCode,
		DefaultModel: "claude-sonnet-4-6",
		CliPath:      "claude",
	}

	adapter := NewAdapter(baseAgent)
	if adapter == nil {
		t.Error("Expected non-nil adapter for claude_code type")
	}

	// Verify it implements the interface
	_, ok := adapter.(*ClaudeAdapter)
	if !ok {
		t.Error("Expected ClaudeAdapter type")
	}
}

func TestNewAdapter_OpenCode(t *testing.T) {
	baseAgent := &model.BaseAgent{
		ID:           uuid.New(),
		Name:         "Test OpenCode",
		Type:         model.BaseAgentTypeOpenCode,
		DefaultModel: "gpt-4",
		CliPath:      "opencode",
	}

	adapter := NewAdapter(baseAgent)
	if adapter == nil {
		t.Error("Expected non-nil adapter for open_code type")
	}

	// Verify it implements the interface
	_, ok := adapter.(*OpenCodeAdapter)
	if !ok {
		t.Error("Expected OpenCodeAdapter type")
	}
}

func TestClaudeAdapter_BuildPrompt(t *testing.T) {
	baseAgent := &model.BaseAgent{
		ID:           uuid.New(),
		Name:         "Test Claude",
		Type:         model.BaseAgentTypeClaudeCode,
		DefaultModel: "claude-sonnet-4-6",
		CliPath:      "claude",
	}
	adapter := NewClaudeAdapter(baseAgent)

	layers := &ContextLayers{
		Layer0: "System prompt here",
		Layer1: "Previous conversation",
		Layer2: "Artifacts context",
		Layer3: "Environment info",
	}

	req := &ExecutionRequest{
		Context: layers,
		Input:   "Hello, world!",
	}

	prompt := adapter.buildPromptFromRequest(req)

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}

	// Check that all layers are included
	if !contains(prompt, "<system>") {
		t.Error("Expected <system> tag in prompt")
	}
	if !contains(prompt, "<conversation>") {
		t.Error("Expected <conversation> tag in prompt")
	}
	if !contains(prompt, "<artifacts>") {
		t.Error("Expected <artifacts> tag in prompt")
	}
	if !contains(prompt, "<environment>") {
		t.Error("Expected <environment> tag in prompt")
	}
	if !contains(prompt, "<user>") {
		t.Error("Expected <user> tag in prompt")
	}
}

func TestOpenCodeAdapter_BuildPrompt(t *testing.T) {
	baseAgent := &model.BaseAgent{
		ID:           uuid.New(),
		Type:         model.BaseAgentTypeOpenCode,
		DefaultModel: "gpt-4",
		CliPath:      "opencode",
	}
	adapter := NewOpenCodeAdapter(baseAgent)

	layers := &ContextLayers{
		Layer0: "System prompt",
		Layer1: "History",
		Layer2: "Files",
		Layer3: "Env",
	}

	req := &ExecutionRequest{
		Context: layers,
		Input:   "Test input",
	}

	prompt := adapter.buildPromptFromRequest(req)

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}