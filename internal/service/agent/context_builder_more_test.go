package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestContextBuilderPromptAndHistoryHelpers(t *testing.T) {
	config := &model.AgentConfig{Name: "Coder", Description: "writes code", SystemPrompt: "Follow project rules."}
	layer0 := BuildStaticLayer0(config)
	if !strings.Contains(layer0, "Coder") || !strings.Contains(layer0, "Follow project rules.") || !strings.Contains(layer0, "GOVERNANCE") {
		t.Fatalf("Layer0 missing expected content: %q", layer0)
	}
	minimal := BuildStaticLayer0Minimal(config)
	if strings.Contains(minimal, "GOVERNANCE") || !strings.Contains(minimal, "Follow project rules.") {
		t.Fatalf("minimal Layer0 unexpected: %q", minimal)
	}

	handoff, ok := ExtractHandoffBlock("before <a2a-handoff>\n### Goal\nShip it\n</a2a-handoff> after")
	if !ok || !strings.Contains(handoff, "Ship it") {
		t.Fatalf("handoff=%q ok=%v", handoff, ok)
	}
	withTags, ok := ExtractHandoffBlockWithTags("x <a2a-handoff>body</a2a-handoff> y")
	if !ok || !strings.HasPrefix(withTags, "<a2a-handoff>") {
		t.Fatalf("handoff with tags=%q ok=%v", withTags, ok)
	}
	formatted := FormatHandoffForA2A("### Task\ncontinue", "Planner")
	if !strings.Contains(formatted, "Planner") || !strings.Contains(formatted, "A2A 交接信息") {
		t.Fatalf("formatted handoff = %q", formatted)
	}

	chain := BuildChainHistoryLayer(&A2AChainContext{
		ChainIndex: 2,
		ChainTotal: 3,
		PreviousResponses: []ChainResponse{{
			AgentName: "Planner",
			Content:   "<a2a-handoff>### Goal\nImplement login</a2a-handoff>",
			Timestamp: 3600,
		}, {
			Role:    "user",
			Content: strings.Repeat("x", 900),
		}},
	})
	if !strings.Contains(chain, "第 2/3 位") || !strings.Contains(chain, "A2A 交接信息") || !strings.Contains(chain, "内容已截断") {
		t.Fatalf("chain history = %q", chain)
	}
	if BuildChainHistoryLayer(nil) != "" {
		t.Fatalf("nil chain history should be empty")
	}
}

func TestDynamicPromptBudgetAndA2AContextHelpers(t *testing.T) {
	currentID := uuid.New()
	nextID := uuid.New()
	routableID := uuid.New()
	current := &model.AgentRoleConfig{ID: currentID, Name: "Planner", Role: model.AgentRoleAgent}
	next := &model.AgentRoleConfig{ID: nextID, Name: "Coder", Role: model.AgentRoleAgent, Description: "implements", MentionPatterns: []string{"@coder"}}
	routable := &model.AgentRoleConfig{ID: routableID, Name: "Ops", Role: model.AgentRoleAgent, Description: "deploys"}

	prompt := BuildDynamicSystemPromptFromContext(&ThreadContext{
		AllowedAgents:      []*model.AgentRoleConfig{current, next},
		RoutableTeamAgents: []*model.AgentRoleConfig{routable},
		Transitions:        []model.Transition{{FromAgentID: currentID.String(), ToAgentID: nextID.String(), Type: model.TransitionTypeSequence}},
	}, current)
	if !strings.Contains(prompt, "@coder") || !strings.Contains(prompt, "跨团队协作方") || !strings.Contains(prompt, "队友名册") {
		t.Fatalf("dynamic prompt = %q", prompt)
	}

	layers := &ContextLayers{
		Layer0: "system",
		Layer1: strings.Repeat("history ", 5000),
		Layer2: "",
		Layer3: "",
	}
	tbm := NewTokenBudgetManager()
	tbm.contextWindowSizes["tiny-model"] = 200
	ApplyTokenBudgetConstraint(layers, "tiny-model", tbm)
	if len(layers.Layer1) >= len(strings.Repeat("history ", 5000)) {
		t.Fatalf("expected layer1 to be truncated")
	}

	from := &AgentInfo{ID: currentID, Name: "Planner"}
	chain := BuildA2AChainContext(&A2AContext{
		ChainIndex: 2,
		PreviousResponses: []ChainResponse{
			{AgentID: currentID, Content: "first"},
			{AgentID: nextID, Content: "second", Timestamp: 123},
		},
		OriginalMessage: "please build",
		FromAgent:       from,
		Depth:           1,
	}, SessionStrategyResume, 3, NewTokenBudgetManager())
	if chain.ChainTotal != 5 || len(chain.ActiveParticipants) != 2 || chain.TokenBudget == nil {
		t.Fatalf("unexpected chain context: %+v", chain)
	}

	stripped := StripA2AMentions("@coder\nkeep this\n  @ops please\nend")
	if strings.Contains(stripped, "@coder") || !strings.Contains(stripped, "keep this") {
		t.Fatalf("stripped mentions = %q", stripped)
	}
	digest, length := CreatePromptDigest("hello")
	if length != 5 || !strings.HasPrefix(digest, "5:") {
		t.Fatalf("digest=%q length=%d", digest, length)
	}
}

func TestStructuredHistoryExtraction(t *testing.T) {
	blocks, err := json.Marshal([]ContentBlockData{{
		Type:     "tool_use",
		ToolName: "Read",
		Input:    map[string]any{"file": "internal/api/login.go"},
		Output:   "done",
	}})
	if err != nil {
		t.Fatalf("marshal blocks: %v", err)
	}
	messages := []*model.Message{
		{Role: model.MessageRoleUser, Content: "请实现登录功能"},
		{Role: model.MessageRoleAgent, AgentID: "coder", Content: "结论：已完成 internal/api/login.go", ContentBlocks: blocks},
		{Role: model.MessageRoleAgent, Content: "", MessageType: model.MessageTypeSystem},
	}
	history := ExtractStructuredHistoryWithBudgetLimit(messages, 3000)
	for _, want := range []string{"用户请求", "结论：已完成", "internal/api/login.go", "Read", "coder"} {
		if !strings.Contains(history, want) {
			t.Fatalf("history missing %q: %s", want, history)
		}
	}
	if got := ExtractStructuredHistoryWithBudgetLimit(nil, 3000); got != "" {
		t.Fatalf("empty history = %q", got)
	}
	limited := ExtractStructuredHistory(messages, 1)
	if !strings.Contains(limited, "用户请求") {
		t.Fatalf("limited history = %q", limited)
	}
}
