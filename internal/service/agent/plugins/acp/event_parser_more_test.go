package acp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/service/agent"
)

func TestParseACPSessionUpdateMessageThoughtUsagePlanAndUnknown(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantTyp agent.ChunkType
		want    string
	}{
		{
			name:    "message",
			raw:     `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hello"}}`,
			wantTyp: agent.ChunkTypeText,
			want:    "hello",
		},
		{
			name:    "thought",
			raw:     `{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"thinking"}}`,
			wantTyp: agent.ChunkTypeThinking,
			want:    "thinking",
		},
		{
			name:    "plan",
			raw:     `{"sessionUpdate":"plan","entries":[{"content":"step one","status":"pending","priority":2}]}`,
			wantTyp: agent.ChunkTypeStatus,
			want:    "[2] [pending] step one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := parseACPSessionUpdate(json.RawMessage(tt.raw), nil)
			if err != nil {
				t.Fatalf("parseACPSessionUpdate returned error: %v", err)
			}
			if len(chunks) != 1 {
				t.Fatalf("chunk count = %d", len(chunks))
			}
			if chunks[0].Type != tt.wantTyp || chunks[0].Content != tt.want {
				t.Fatalf("chunk = %#v", chunks[0])
			}
		})
	}

	usageChunks, err := parseACPSessionUpdate(json.RawMessage(`{"sessionUpdate":"usage_update","used":12,"size":99}`), nil)
	if err != nil {
		t.Fatalf("usage parse error: %v", err)
	}
	if len(usageChunks) != 1 || usageChunks[0].Type != agent.ChunkTypeUsage || usageChunks[0].Usage.ContextUsed != 12 || usageChunks[0].Usage.ContextSize != 99 {
		t.Fatalf("usage chunk = %#v", usageChunks)
	}

	unknown, err := parseACPSessionUpdate(json.RawMessage(`{"sessionUpdate":"new_future_event"}`), nil)
	if err != nil {
		t.Fatalf("unknown update returned error: %v", err)
	}
	if unknown != nil {
		t.Fatalf("unknown update chunks = %#v", unknown)
	}
}

func TestParseACPToolCallStoresNameAndDetectsQuestion(t *testing.T) {
	session := &acpSession{}
	raw := json.RawMessage(`{
		"sessionUpdate":"tool_call",
		"toolCallId":"tool-1",
		"title":"Read",
		"status":"pending",
		"rawInput":{"path":"README.md"}
	}`)

	chunks, err := parseACPToolCall(raw, session)
	if err != nil {
		t.Fatalf("parseACPToolCall returned error: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != agent.ChunkTypeToolUse {
		t.Fatalf("tool chunks = %#v", chunks)
	}
	if chunks[0].ToolName != "Read" || chunks[0].ToolInput["path"] != "README.md" {
		t.Fatalf("tool chunk = %#v", chunks[0])
	}
	if got := session.toolCallNames["tool-1"]; got != "Read" {
		t.Fatalf("stored tool name = %q", got)
	}

	askInitial := json.RawMessage(`{"sessionUpdate":"tool_call","toolCallId":"q0","title":"question","rawInput":{}}`)
	suppressed, err := parseACPToolCall(askInitial, session)
	if err != nil {
		t.Fatalf("empty question tool returned error: %v", err)
	}
	if suppressed != nil {
		t.Fatalf("empty question tool should be suppressed: %#v", suppressed)
	}

	questionRaw := json.RawMessage(`{
		"sessionUpdate":"tool_call",
		"toolCallId":"q1",
		"title":"AskUserQuestion",
		"rawInput":{"questions":[{"header":"Mode","question":"Pick one","options":[{"label":"A","description":"Alpha","preview":"a"}]}]}
	}`)
	questionChunks, err := parseACPToolCall(questionRaw, session)
	if err != nil {
		t.Fatalf("question parse returned error: %v", err)
	}
	if len(questionChunks) != 1 || questionChunks[0].Type != agent.ChunkTypeQuestion {
		t.Fatalf("question chunks = %#v", questionChunks)
	}
	if session.pendingQuestion == nil || session.pendingQuestion.ToolID != "q1" {
		t.Fatalf("pendingQuestion = %#v", session.pendingQuestion)
	}
	if got := questionChunks[0].Questions[0]; got.Header != "Mode" || got.Question != "Pick one" || len(got.Options) != 2 {
		t.Fatalf("question item = %#v", got)
	}

	metaAsk := json.RawMessage(`{"sessionUpdate":"tool_call","toolCallId":"q2","_meta":{"claudeCode":{"toolName":"AskUserQuestion"}}}`)
	metaChunks, err := parseACPToolCall(metaAsk, session)
	if err != nil {
		t.Fatalf("meta ask parse returned error: %v", err)
	}
	if metaChunks != nil {
		t.Fatalf("meta AskUserQuestion should be skipped: %#v", metaChunks)
	}
}

func TestParseACPToolCallUpdateVariants(t *testing.T) {
	session := &acpSession{toolCallNames: map[string]string{"tool-1": "Read"}}

	startRaw := json.RawMessage(`{"sessionUpdate":"tool_call_update","toolCallId":"tool-1","status":"in_progress","rawInput":{"path":"main.go"}}`)
	startChunks, err := parseACPToolCallUpdate(startRaw, session)
	if err != nil {
		t.Fatalf("start update parse returned error: %v", err)
	}
	if len(startChunks) != 1 || startChunks[0].Type != agent.ChunkTypeToolUse || startChunks[0].ToolName != "Read" {
		t.Fatalf("start chunks = %#v", startChunks)
	}

	questionRaw := json.RawMessage(`{
		"sessionUpdate":"tool_call_update",
		"toolCallId":"q1",
		"status":"pending",
		"kind":"question",
		"rawInput":{"questions":[{"header":"Targets","question":"Choose","multiple":true,"options":[{"label":"one"}]}]}
	}`)
	questionChunks, err := parseACPToolCallUpdate(questionRaw, session)
	if err != nil {
		t.Fatalf("question update parse returned error: %v", err)
	}
	if len(questionChunks) != 1 || questionChunks[0].Type != agent.ChunkTypeQuestion || !questionChunks[0].Questions[0].MultiSelect {
		t.Fatalf("question update chunks = %#v", questionChunks)
	}

	doneRaw := json.RawMessage(`{
		"sessionUpdate":"tool_call_update",
		"toolCallId":"tool-1",
		"status":"completed",
		"content":[{"type":"content","content":{"type":"text","text":"done"}}]
	}`)
	doneChunks, err := parseACPToolCallUpdate(doneRaw, session)
	if err != nil {
		t.Fatalf("done update parse returned error: %v", err)
	}
	if len(doneChunks) != 1 || doneChunks[0].Type != agent.ChunkTypeToolResult || doneChunks[0].Content != "done" || doneChunks[0].IsError {
		t.Fatalf("done chunks = %#v", doneChunks)
	}

	failedRaw := json.RawMessage(`{"sessionUpdate":"tool_call_update","toolCallId":"tool-1","status":"failed","content":[{"type":"text","text":"boom"}]}`)
	failedChunks, err := parseACPToolCallUpdate(failedRaw, session)
	if err != nil {
		t.Fatalf("failed update parse returned error: %v", err)
	}
	if len(failedChunks) != 1 || !failedChunks[0].IsError || failedChunks[0].Content != "boom" {
		t.Fatalf("failed chunks = %#v", failedChunks)
	}

	metaAsk := json.RawMessage(`{"sessionUpdate":"tool_call_update","toolCallId":"q2","_meta":{"claudeCode":{"toolName":"AskUserQuestion"}}}`)
	metaChunks, err := parseACPToolCallUpdate(metaAsk, session)
	if err != nil {
		t.Fatalf("meta ask update returned error: %v", err)
	}
	if metaChunks != nil {
		t.Fatalf("meta AskUserQuestion update should be skipped: %#v", metaChunks)
	}
}

func TestACPQuestionAndContentHelpers(t *testing.T) {
	if !detectQuestionTool("Ask User", "", nil) || !detectQuestionTool("", "ask_user", nil) || !detectQuestionTool("", "", map[string]interface{}{"questions": []interface{}{}}) {
		t.Fatalf("detectQuestionTool should detect question signals")
	}
	if detectQuestionTool("Read", "tool", map[string]interface{}{"path": "x"}) {
		t.Fatalf("detectQuestionTool should ignore normal tools")
	}

	content := extractToolCallContent([]acpContentBlock{
		{Type: "content", Content: json.RawMessage(`{"type":"text","text":"nested"}`)},
	})
	if content != "nested" {
		t.Fatalf("nested content = %q", content)
	}
	if got := extractToolCallContent([]acpContentBlock{{Type: "content", Content: json.RawMessage(`{"text":"map text"}`)}}); got != "map text" {
		t.Fatalf("map content = %q", got)
	}
	if got := extractToolCallContent([]acpContentBlock{{Type: "content", Content: json.RawMessage(`{bad`)}}); got != "" {
		t.Fatalf("bad content = %q", got)
	}

	reqChunk := parseACPUserInputRequest(acpUserInputRequest{
		ToolCallID: "q1",
		ToolName:   "AskUserQuestion",
		Input: map[string]interface{}{
			"questions": []interface{}{map[string]interface{}{
				"header":      "H",
				"question":    "Q",
				"multiSelect": true,
				"options":     []interface{}{map[string]interface{}{"label": "L"}},
			}},
		},
	})
	if reqChunk.Type != agent.ChunkTypeQuestion || reqChunk.ToolID != "q1" || !reqChunk.Questions[0].MultiSelect {
		t.Fatalf("user input chunk = %#v", reqChunk)
	}

	if getStringFromMap(map[string]interface{}{"x": 1}, "x") != "" || getBoolFromMap(map[string]interface{}{"x": "true"}, "x") {
		t.Fatalf("typed map helpers accepted wrong types")
	}
}

func TestParseACPErrors(t *testing.T) {
	badJSON := json.RawMessage(`{bad`)
	parsers := []func(json.RawMessage) ([]agent.Chunk, error){
		func(raw json.RawMessage) ([]agent.Chunk, error) { return parseACPAgentMessageChunk(raw) },
		func(raw json.RawMessage) ([]agent.Chunk, error) { return parseACPAgentThoughtChunk(raw) },
		func(raw json.RawMessage) ([]agent.Chunk, error) { return parseACPToolCall(raw, nil) },
		func(raw json.RawMessage) ([]agent.Chunk, error) { return parseACPToolCallUpdate(raw, nil) },
		func(raw json.RawMessage) ([]agent.Chunk, error) { return parseACPUsageUpdate(raw) },
		func(raw json.RawMessage) ([]agent.Chunk, error) { return parseACPPlanUpdate(raw) },
	}
	for i, parser := range parsers {
		if _, err := parser(badJSON); err == nil {
			t.Fatalf("parser %d expected error", i)
		}
	}
	if _, err := parseACPSessionUpdate(badJSON, nil); err == nil || !strings.Contains(err.Error(), "parse session update header") {
		t.Fatalf("parseACPSessionUpdate bad error = %v", err)
	}
}
