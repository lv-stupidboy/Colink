package agent_test

import (
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/service/agent"
)

// @feature F001 - Agent 对话核心
// @priority P0
// @id SV-01-02
func TestBuildPromptIncludesMemoryContext(t *testing.T) {
	prompt := agent.BuildPrompt(&agent.ContextLayers{
		Layer0:        "system prompt",
		MemoryContext: "<memory-context>\n## Project Memory\n- port-8080-unavailable: avoid 8080\n</memory-context>",
	}, "请启动服务")

	if !strings.Contains(prompt, "<memory-context>") {
		t.Fatalf("expected memory context to be included in prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, "<user>") {
		t.Fatalf("expected regular user tag, got:\n%s", prompt)
	}
}

// @feature F003 - 多 Agent 协作 (A2A)
// @priority P0
// @id SV-03-06
func TestBuildMemoryToolGovernanceUsesUnifiedMemoryAdd(t *testing.T) {
	governance := agent.BuildMemoryToolGovernance()

	for _, expected := range []string{
		"memory.add",
		"type=team",
		"type=project",
		"团队记忆",
		"项目记忆",
		"Agent 角色",
		"当前 workspace",
		"topic",
		"facts",
		"usage",
	} {
		if !strings.Contains(governance, expected) {
			t.Fatalf("expected memory tool governance to contain %q, got:\n%s", expected, governance)
		}
	}
	if strings.Contains(governance, "project_memory") {
		t.Fatalf("expected project_memory to be removed from governance, got:\n%s", governance)
	}
}
