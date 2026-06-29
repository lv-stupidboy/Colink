package acp

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/google/uuid"
)

func TestAdapterSessionAndRequestHelpers(t *testing.T) {
	adapter := NewBaseACPAdapter(AcpAdapterConfig{
		BuildEnv: func(req *agent.ExecutionRequest) []string {
			return []string{"ACP_TEST_ENV=adapter", "ACP_TEST_EMPTY="}
		},
	}, &model.BaseAgent{DefaultModel: "model-a"})

	if adapter.GetCurrentProcess() != nil {
		t.Fatalf("empty adapter should not expose current process")
	}
	adapter.sessions["running"] = &acpSession{status: agent.SessionStatusRunning}
	if adapter.GetCurrentProcess() != nil {
		t.Fatalf("session without process should not expose current process")
	}

	prompt := adapter.buildPromptFromRequest(&agent.ExecutionRequest{Input: "hello"})
	if !strings.Contains(prompt, "hello") {
		t.Fatalf("prompt = %q", prompt)
	}

	blocks := adapter.buildContentBlocks("text", []model.ImageContent{{MimeType: "image/png", Data: "base64"}})
	if len(blocks) != 2 || blocks[0].Type != "text" || blocks[1].Type != "image" || blocks[1].MimeType != "image/png" || blocks[1].Data != "base64" {
		t.Fatalf("content blocks = %#v", blocks)
	}

	t.Setenv("ACP_TEST_ENV", "host")
	env := adapter.buildEnv(&agent.ExecutionRequest{})
	if !containsEnv(env, "ACP_TEST_ENV=adapter") || !containsEnv(env, "ACP_TEST_EMPTY=") {
		t.Fatalf("env does not contain adapter overrides: %v", env)
	}
}

func TestBuildMCPServersIncludesManagedUserAndMemory(t *testing.T) {
	adapter := NewBaseACPAdapter(AcpAdapterConfig{
		BuildEnv: func(req *agent.ExecutionRequest) []string { return nil },
		LoadUserMCPConfig: func() map[string]interface{} {
			return map[string]interface{}{
				"user-http": map[string]interface{}{
					"type":    "http",
					"url":     "https://example.test/mcp",
					"headers": map[string]interface{}{"Authorization": "Bearer x"},
				},
				"user-stdio": map[string]interface{}{
					"command": "node",
					"args":    []interface{}{"server.js", "--debug"},
					"env":     map[string]interface{}{"TOKEN": "secret"},
				},
				"ignored": "not a map",
			}
		},
	}, nil)

	t.Setenv("ISDP_MCP_SERVER_PATH", "/tmp/isdp-mcp-server")
	invocationID := uuid.New()
	got := adapter.buildMCPServers(&agent.ExecutionRequest{
		InvocationID:  invocationID,
		CallbackToken: "callback-token",
		APIURL:        "http://api.local",
		MCPServers: []*model.MCPServer{
			{
				Name:      "managed",
				Transport: model.MCPTransportHTTP,
				URL:       "https://managed.test/mcp",
				Headers:   map[string]string{"X-Test": "1"},
				Status:    model.MCPStatusActive,
			},
		},
	})

	if len(got) != 4 {
		t.Fatalf("MCP server count = %d, servers = %#v", len(got), got)
	}
	if !hasACPServer(got, "managed") || !hasACPServer(got, "user-http") || !hasACPServer(got, "user-stdio") || !hasACPServer(got, "isdp-memory") {
		t.Fatalf("MCP servers missing expected entries: %#v", got)
	}

	memory := findACPServer(got, "isdp-memory")
	if memory["command"] != "/tmp/isdp-mcp-server" {
		t.Fatalf("memory command = %#v", memory["command"])
	}
	env := memory["env"].([]map[string]string)
	if len(env) != 3 {
		t.Fatalf("memory env = %#v", env)
	}

	noMemory := adapter.buildMCPServers(&agent.ExecutionRequest{})
	if len(noMemory) != 2 {
		t.Fatalf("expected only user MCP servers without callback fields, got %#v", noMemory)
	}

	if got := adapter.buildMCPServers(nil); len(got) != 0 {
		t.Fatalf("nil request MCP servers = %#v", got)
	}
}

func TestConvertUserMCPToACPFormatVariants(t *testing.T) {
	got := convertUserMCPToACPFormat(map[string]interface{}{
		"http": map[string]interface{}{
			"type":    "http",
			"url":     "https://example.test/http",
			"headers": map[string]interface{}{"A": "1", "B": 2},
		},
		"sse": map[string]interface{}{
			"type": "sse",
			"url":  "https://example.test/sse",
		},
		"stdio": map[string]interface{}{
			"command": "python",
			"args":    []interface{}{"server.py", 3},
			"env":     map[string]interface{}{"TOKEN": "secret", "COUNT": 2},
		},
		"bad": 7,
	})
	if len(got) != 3 {
		t.Fatalf("server count = %d: %#v", len(got), got)
	}

	httpServer := findACPServer(got, "http")
	if httpServer["type"] != "http" || httpServer["url"] != "https://example.test/http" {
		t.Fatalf("http server = %#v", httpServer)
	}
	headers := httpServer["headers"].([]map[string]string)
	if len(headers) != 1 {
		t.Fatalf("http headers = %#v", headers)
	}

	stdio := findACPServer(got, "stdio")
	if stdio["command"] != "python" {
		t.Fatalf("stdio server = %#v", stdio)
	}
	args := stdio["args"].([]interface{})
	if len(args) != 2 || args[0] != "server.py" || args[1] != 3 {
		t.Fatalf("stdio args = %#v", args)
	}
	env := stdio["env"].([]map[string]string)
	if len(env) != 1 {
		t.Fatalf("stdio env = %#v", env)
	}

	if empty := convertUserMCPToACPFormat(nil); len(empty) != 0 {
		t.Fatalf("nil user MCP = %#v", empty)
	}
}

func TestElicitationAndAnswerHelpers(t *testing.T) {
	props := map[string]json.RawMessage{
		"question_1":        json.RawMessage(`{"type":"array","title":"Files","description":"Pick files","items":{"anyOf":[{"const":"a.go","title":"a.go — source"}]}}`),
		"question_0":        json.RawMessage(`{"type":"string","title":"Mode","oneOf":[{"const":"fast","title":"fast","_meta":{"_claude/askUserQuestionOption":{"description":"Fast mode","preview":"go fast"}}}]}`),
		"question_0_custom": json.RawMessage(`{"type":"string"}`),
		"noise":             json.RawMessage(`{"type":"string"}`),
	}
	questions := parseElicitationQuestions(props, "Fallback question?")
	if len(questions) != 2 {
		t.Fatalf("questions = %#v", questions)
	}
	if questions[0].Header != "Mode" || questions[0].Question != "Fallback question?" || questions[0].Options[0].Description != "Fast mode" || questions[0].Options[0].Preview != "go fast" {
		t.Fatalf("question 0 = %#v", questions[0])
	}
	if !questions[1].MultiSelect || questions[1].Options[0].Description != "source" {
		t.Fatalf("question 1 = %#v", questions[1])
	}

	flat := flattenJSONAnswerForLegacy(`{"question_2":true,"question_0":"  A  ","question_1":["B","",3],"question_1_custom":"ignored"}`)
	if flat != "A\nB、3\ntrue" {
		t.Fatalf("flattened answer = %q", flat)
	}
	if got := flattenJSONAnswerForLegacy("plain answer"); got != "plain answer" {
		t.Fatalf("plain answer changed to %q", got)
	}
	if got := flattenJSONAnswerForLegacy(`{"other":"value"}`); got != `{"other":"value"}` {
		t.Fatalf("non-question json changed to %q", got)
	}

	content := buildElicitationContent(`{"question_0":"yes"}`, questions)
	if content["question_0"] != "yes" {
		t.Fatalf("json elicitation content = %#v", content)
	}
	content = buildElicitationContent("free text", questions)
	if content["question_0"] != "free text" {
		t.Fatalf("text elicitation content = %#v", content)
	}

	answers, err := buildOpenCodeReplyAnswers(`{"question_1":["b","c"],"question_0":"a","question_2":5,"ignored":"x","question_3":null}`)
	if err != nil {
		t.Fatalf("buildOpenCodeReplyAnswers returned error: %v", err)
	}
	if len(answers) != 4 {
		t.Fatalf("answers = %#v", answers)
	}
	if first := answers[0].([]interface{}); len(first) != 1 || first[0] != "a" {
		t.Fatalf("first answer = %#v", first)
	}
	if third := answers[2].([]interface{}); len(third) != 1 || third[0] != "5" {
		t.Fatalf("third answer = %#v", third)
	}
	if _, err := buildOpenCodeReplyAnswers(`{bad`); err == nil {
		t.Fatalf("invalid OpenCode answer should fail")
	}
}

func TestPortAndErrorHelpers(t *testing.T) {
	if got := ExtractPortFromArgs([]string{"--foo", "x", "--port", "4567"}); got != 4567 {
		t.Fatalf("ExtractPortFromArgs = %d", got)
	}
	if got := ExtractPortFromArgs([]string{"--port", "bad"}); got != 0 {
		t.Fatalf("ExtractPortFromArgs bad = %d", got)
	}
	if got := ExtractPortFromArgs([]string{"--port"}); got != 0 {
		t.Fatalf("ExtractPortFromArgs missing value = %d", got)
	}

	if !isErrorLikeMethod("notify/error") || !isErrorLikeMethod("rateLimitWarning") || isErrorLikeMethod("window/showMessage") || isErrorLikeMethod("session/update") {
		t.Fatalf("isErrorLikeMethod classification mismatch")
	}

	readable := extractReadableErrorLine(json.RawMessage(`{"message":"bad config","detail":"long"}`))
	if readable != "bad config" {
		t.Fatalf("readable error = %q", readable)
	}
	if got := extractReadableErrorLine(json.RawMessage(`"plain"`)); got != `"plain"` {
		t.Fatalf("string readable error = %q", got)
	}
	if got := extractReadableErrorLine(json.RawMessage(`{bad`)); got != "{bad" {
		t.Fatalf("bad json readable error = %q", got)
	}
}

func containsEnv(env []string, want string) bool {
	for _, item := range env {
		if item == want {
			return true
		}
	}
	return false
}

func hasACPServer(servers []interface{}, name string) bool {
	return findACPServer(servers, name) != nil
}

func findACPServer(servers []interface{}, name string) map[string]interface{} {
	for _, server := range servers {
		m, ok := server.(map[string]interface{})
		if !ok {
			continue
		}
		if m["name"] == name {
			return m
		}
	}
	return nil
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
