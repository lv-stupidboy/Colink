package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

type testSessionRecorder struct {
	failed     []string
	successful []string
}

func (r *testSessionRecorder) RecordFailedSession(threadID, configID, sessionID string) {
	r.failed = append(r.failed, threadID+":"+configID+":"+sessionID)
}

func (r *testSessionRecorder) RecordSuccessfulSession(threadID, configID, sessionID string) {
	r.successful = append(r.successful, threadID+":"+configID+":"+sessionID)
}

// 以下方法用于满足合并后扩展的 SessionRecorder 接口，测试中默认为 no-op。
func (r *testSessionRecorder) RecordPendingSession(threadID, configID, sessionID string) {}
func (r *testSessionRecorder) IncrementConsecutiveFailures(threadID, configID, sessionID string) {}
func (r *testSessionRecorder) ResetConsecutiveFailures(threadID, configID, sessionID string)     {}
func (r *testSessionRecorder) CheckAndSealOnOverflow(threadID, configID string) bool             { return false }

func TestExecutionErrorHelpers(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, "unknown"},
		{errors.New("timeout waiting for cli"), "cli_timeout"},
		{errors.New("session resume failed"), "session_error"},
		{errors.New("api returned 401"), "api_error"},
		{errors.New("context token overflow"), "context_overflow"},
		{errors.New("permission denied"), "permission_error"},
		{errors.New("other"), "unknown"},
	}
	for _, tc := range cases {
		if got := getErrorType(tc.err); got != tc.want {
			t.Fatalf("getErrorType(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}

	suggestions := generateErrorSuggestions(errors.New("session expired"), "session-123456789", true)
	if len(suggestions) < 2 || !strings.Contains(suggestions[len(suggestions)-1], "session-") {
		t.Fatalf("resume suggestions = %#v", suggestions)
	}
	empty := buildEmptyOutputError(strings.Repeat("x", 2100))
	if !strings.Contains(empty, "输出过长") || !strings.Contains(empty, "CLI 错误输出") {
		t.Fatalf("empty output diagnostic missing details: %s", empty)
	}
	detail := buildDetailedErrorOutput(errors.New("context token overflow"), &ExecutionRequest{
		SessionID: "1234567890abcdef",
		WorkDir:   "/tmp/work",
		ConfigDir: "/tmp/config",
		Input:     strings.Repeat("a", 10001),
		BaseAgent: &model.BaseAgent{DefaultModel: "qwen"},
		Context:   &ContextLayers{Layer1: strings.Repeat("line\n", 501)},
	}, []string{"try again"})
	for _, want := range []string{"context_overflow", "12345678...", "输入较长", "历史较长", "try again"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detailed error missing %q: %s", want, detail)
		}
	}
}

func TestExecutionA2AAndLookupHelpers(t *testing.T) {
	es := &ExecutionService{}
	fromID := uuid.New()
	input := es.buildA2AInput(context.Background(), uuid.New(), nil, &A2AContext{
		FromAgent: &AgentInfo{ID: fromID, Name: "Planner", Role: "planner"},
	}, "ignored", nil, SessionStrategyNew)
	if !strings.Contains(input, "协作规则") || !strings.Contains(input, "Direct message from Planner") {
		t.Fatalf("A2A input = %q", input)
	}
	if got := es.buildA2AInput(context.Background(), uuid.New(), nil, nil, "ignored", nil, SessionStrategyNew); got != "" {
		t.Fatalf("A2A input without source = %q", got)
	}
	if got := es.getRoleDescription(model.AgentRoleReviewer); got != "代码审查专家" {
		t.Fatalf("reviewer description = %q", got)
	}
	if got := es.getRoleDescription(model.AgentRole("custom-role")); got != "custom-role" {
		t.Fatalf("custom description = %q", got)
	}

	agentA := &model.AgentRoleConfig{ID: uuid.New(), Name: "Planner", Role: model.AgentRoleRequirement}
	agentB := &model.AgentRoleConfig{ID: uuid.New(), Name: "Coder", Role: model.AgentRoleDeveloper}
	agents := []*model.AgentRoleConfig{agentA, agentB}
	if es.findAgentByRole(agents, model.AgentRoleDeveloper) != agentB || es.findAgentByRole(agents, model.AgentRoleReviewer) != nil {
		t.Fatalf("findAgentByRole mismatch")
	}
	if es.findAgentByName(agents, "Planner") != agentA || es.findAgentByName(agents, "Missing") != nil {
		t.Fatalf("findAgentByName mismatch")
	}
}

func TestExecutionTextFilteringAndHistoryHelpers(t *testing.T) {
	es := &ExecutionService{}
	filteredMentions := es.stripPureMentionLines("keep\n@planner\n  @coder   \n@reviewer please check")
	if strings.Contains(filteredMentions, "@planner\n") || !strings.Contains(filteredMentions, "@reviewer please check") {
		t.Fatalf("stripPureMentionLines = %q", filteredMentions)
	}

	if !es.matchCondition("build passed", "") ||
		!es.matchCondition("build passed", "contains:passed") ||
		!es.matchCondition("abc-123", `regex:abc-\d+`) ||
		!es.matchCondition("hello world", "world") ||
		es.matchCondition("abc", "regex:[") {
		t.Fatalf("matchCondition behavior changed")
	}

	blockOutput := strings.Repeat("tool-output-", 30)
	structured := es.filterStructuredOutput("path: internal/main.go\n结论: 可以发布\ntrue.md", []ContentBlockData{
		{Type: "tool_use", ToolName: "grep", Output: blockOutput},
	})
	for _, want := range []string{"path: internal/main.go", "main.go", "结论: 可以发布", "[grep]"} {
		if !strings.Contains(structured, want) {
			t.Fatalf("structured output missing %q: %s", want, structured)
		}
	}
	if got := es.filterStructuredOutput("nothing useful", nil); got != "(无关键结构化信息)" {
		t.Fatalf("empty structured output = %q", got)
	}

	blocks := []ContentBlockData{
		{Type: "text", Content: "<thinking>hidden</thinking>visible"},
		{Type: "thinking", Content: strings.Repeat("think", 90)},
		{Type: "tool_use", ToolID: "tool-1", ToolName: "Read"},
		{Type: "tool_result", ToolID: "tool-1"},
		{Type: "tool_result", ToolID: "tool-2", IsError: true},
	}
	simplified := es.extractSimplifiedAgentBlocks(blocks)
	for _, want := range []string{"visible", "[思考]", "[调用工具: Read]", "[工具结果: Read - 完成]", "[工具结果: 未知工具 - 失败]"} {
		if !strings.Contains(simplified, want) {
			t.Fatalf("simplified blocks missing %q: %s", want, simplified)
		}
	}
	if got := es.extractSimplifiedAgentBlocks(nil); got != "(无可用内容)" {
		t.Fatalf("empty simplified blocks = %q", got)
	}
	if got := es.filterAndTruncateContent("<thinking>hidden</thinking>" + strings.Repeat("x", 600)); !strings.Contains(got, "已省略") {
		t.Fatalf("truncated content = %q", got)
	}
	if got := es.filterThinkingContent("## Thinking\nhidden\n\nanswer"); got != "answer" {
		t.Fatalf("filterThinkingContent = %q", got)
	}

	metadata, _ := json.Marshal(map[string]any{"agentName": "Planner"})
	contentBlocks, _ := json.Marshal(blocks)
	history := es.extractStructuredHistory([]*model.Message{
		{Role: model.MessageRoleUser, Content: "hello"},
		{Role: model.MessageRoleAgent, AgentID: "agent-1", Content: "fallback", Metadata: metadata, ContentBlocks: contentBlocks},
		{Role: model.MessageRoleAgent, AgentID: "agent-2", Content: "<thinking>hidden</thinking>answer"},
	}, 2)
	if !strings.Contains(history, "**用户**: hello") || !strings.Contains(history, "**Planner**") || strings.Contains(history, "agent-2") {
		t.Fatalf("structured history = %s", history)
	}
	if got := es.formatMessages([]*model.Message{{Role: model.MessageRoleUser, Content: "hi"}, {Role: model.MessageRoleAgent, AgentID: "agent", Content: "done"}}); !strings.Contains(got, "[用户] hi") || !strings.Contains(got, "[agent] done") {
		t.Fatalf("formatMessages = %q", got)
	}
}

func TestExecutionStateHelpers(t *testing.T) {
	recorder := &testSessionRecorder{}
	SetSessionRecorder(recorder)
	if globalSessionRecorder != recorder {
		t.Fatalf("session recorder not set")
	}
	defer SetSessionRecorder(nil)

	threadID := uuid.New()
	invocationID := uuid.New()
	agentID := uuid.New()
	es := &ExecutionService{
		runningAgents: map[uuid.UUID]*RunningAgent{
			invocationID: {
				InvocationID:      invocationID,
				ThreadID:          threadID,
				AgentConfig:       &model.AgentRoleConfig{ID: agentID, Name: "Planner"},
				StartedAt:         time.Now().Add(-2 * time.Second),
				AccumulatedOutput: "partial",
				AccumulatedContentBlocks: []ContentBlockData{
					{ID: "b1", Type: "text", Content: "partial"},
				},
			},
		},
		threadContexts: map[uuid.UUID]*ThreadContext{
			threadID: {
				Thread:  &model.Thread{ID: threadID, Name: "Thread title"},
				Project: &model.Project{Name: "Project name"},
			},
		},
		a2aContexts: map[uuid.UUID]*A2AContext{
			threadID: {CompletedAgents: map[uuid.UUID]bool{agentID: true}},
		},
		contentBlockBuffer: make([]model.InvocationContentBlock, 0, 20),
		lastFlush:          time.Now().Add(-time.Second),
	}
	es.SetAPIURL("http://127.0.0.1")
	es.SetMCPBindingRepository(nil)
	es.SetSessionManager(NewSessionManager(nil, SessionManagerConfig{}))
	if es.apiURL != "http://127.0.0.1" || es.sessionManager == nil {
		t.Fatalf("setters did not update service")
	}
	es.AddChunkListener(func(threadID, invocationID uuid.UUID, chunk Chunk, agentID, agentName string) {})
	if len(es.chunkListeners) != 1 {
		t.Fatalf("chunk listener not added")
	}
	es.NotifyChunkListeners(threadID, invocationID, Chunk{Type: ChunkTypeText, Content: "hi"}, agentID.String(), "Planner")

	states := es.GetRunningAgentsForThread(threadID)
	if len(states) != 1 || states[0].AccumulatedOutput != "partial" || states[0].Status != "running" {
		t.Fatalf("running states = %#v", states)
	}
	recovery := es.GetRunningInvocationsWithContentBlocks(context.Background(), threadID)
	recoveredBlocks, _ := recovery[0].ContentBlocks.([]ContentBlockData)
	if len(recovery) != 1 || len(recoveredBlocks) != 1 {
		t.Fatalf("recovery = %#v", recovery)
	}
	all, err := es.GetAllRunningAgents(context.Background())
	if err != nil || len(all) != 1 || all[0].ProjectName != "Project name" || all[0].ThreadTitle != "Thread title" {
		t.Fatalf("all running agents = %#v err=%v", all, err)
	}
	if !es.checkMergeCondition(threadID, []string{agentID.String(), "not-a-uuid"}) {
		t.Fatalf("merge condition should ignore invalid ids and accept completed ids")
	}
	es.ClearA2AContext(threadID)
	if es.checkMergeCondition(threadID, []string{agentID.String()}) {
		t.Fatalf("merge condition should fail after clear")
	}
	es.ClearThreadContext(threadID)
	if _, ok := es.threadContexts[threadID]; ok {
		t.Fatalf("thread context should be cleared")
	}
	if artifacts := es.getArtifacts(&model.Thread{}); artifacts != "" {
		t.Fatalf("artifacts = %q", artifacts)
	}
	if env := es.getEnvironmentInfo(&model.Thread{ID: threadID, CurrentPhase: "build", Status: model.ThreadStatusRunning}); !strings.Contains(env, "build") || !strings.Contains(env, "running") {
		t.Fatalf("environment info = %q", env)
	}
}
