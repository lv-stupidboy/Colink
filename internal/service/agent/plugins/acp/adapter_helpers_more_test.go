package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	port, err := FindFreePort()
	if err != nil {
		t.Skipf("FindFreePort unavailable in this sandbox: %v", err)
	}
	if port <= 0 {
		t.Fatalf("FindFreePort() = %d", port)
	}
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

func TestBaseACPAdapterCheckHealth(t *testing.T) {
	adapter := NewBaseACPAdapter(AcpAdapterConfig{
		CliPath: writeFakeACPHealthCLI(t),
		BuildArgs: func(req *agent.ExecutionRequest) []string {
			return []string{"--health"}
		},
		BuildEnv: func(req *agent.ExecutionRequest) []string {
			return []string{"ACP_HEALTH_TEST=1"}
		},
	}, &model.BaseAgent{DefaultModel: "test-model"})
	if err := adapter.CheckHealth(context.Background()); err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}

	bad := NewBaseACPAdapter(AcpAdapterConfig{
		CliPath: filepath.Join(t.TempDir(), "missing-cli"),
		BuildArgs: func(req *agent.ExecutionRequest) []string {
			return nil
		},
		BuildEnv: func(req *agent.ExecutionRequest) []string {
			return nil
		},
	}, nil)
	if err := bad.CheckHealth(context.Background()); err == nil {
		t.Fatal("CheckHealth(missing cli) error = nil, want error")
	}
}

func TestBaseACPAdapterConfigureSessionVariants(t *testing.T) {
	adapter := NewBaseACPAdapter(AcpAdapterConfig{
		ModelRef: func() string { return "provider/model-b" },
	}, &model.BaseAgent{DefaultModel: "model-a"})
	transport, calls, closeTransport := newRecordingACPTransport(t, nil)
	defer closeTransport()

	err := adapter.configureSession(transport, &acpSession{id: "acp-session"}, &acpNewSessionResult{
		ConfigOptions: []acpSessionConfigOpt{{ConfigID: "model", Name: "Model", Type: "select"}},
	}, &agent.ExecutionRequest{})
	if err != nil {
		t.Fatalf("configureSession configOptions returned error: %v", err)
	}
	call := readRecordedACPCall(t, calls)
	if call.Method != "session/set_config_option" || !strings.Contains(string(call.Params), `"value":"provider/model-b"`) {
		t.Fatalf("config option call = %#v", call)
	}

	legacy := NewBaseACPAdapter(AcpAdapterConfig{LegacyModelConfig: true}, &model.BaseAgent{DefaultModel: "legacy-model"})
	err = legacy.configureSession(transport, &acpSession{id: "legacy-session"}, &acpNewSessionResult{
		ConfigOptions: []acpSessionConfigOpt{{ConfigID: "model"}},
	}, &agent.ExecutionRequest{})
	if err != nil {
		t.Fatalf("configureSession legacy returned error: %v", err)
	}
	call = readRecordedACPCall(t, calls)
	if call.Method != "session/set_model" || !strings.Contains(string(call.Params), `"modelId":"legacy-model"`) {
		t.Fatalf("legacy call = %#v", call)
	}

	skipping := NewBaseACPAdapter(AcpAdapterConfig{
		SkipModelConfig: func(req *agent.ExecutionRequest) bool { return true },
	}, &model.BaseAgent{DefaultModel: "skip-model"})
	if err := skipping.configureSession(transport, &acpSession{id: "skip-session"}, &acpNewSessionResult{
		ConfigOptions: []acpSessionConfigOpt{{ConfigID: "model"}},
	}, &agent.ExecutionRequest{}); err != nil {
		t.Fatalf("configureSession skip returned error: %v", err)
	}
	assertNoRecordedACPCall(t, calls)
}

func TestBaseACPAdapterConfigureSessionAndAuthenticateErrors(t *testing.T) {
	transport, _, closeTransport := newRecordingACPTransport(t, func(method string, params json.RawMessage) (any, *jsonrpcError) {
		if method == "session/set_config_option" {
			return nil, &jsonrpcError{Code: -32602, Message: "bad model"}
		}
		return map[string]any{}, nil
	})
	defer closeTransport()

	adapter := NewBaseACPAdapter(AcpAdapterConfig{}, &model.BaseAgent{DefaultModel: "bad-model"})
	err := adapter.configureSession(transport, &acpSession{id: "session"}, &acpNewSessionResult{
		ConfigOptions: []acpSessionConfigOpt{{ConfigID: "model"}},
	}, &agent.ExecutionRequest{})
	if err == nil || !strings.Contains(err.Error(), "bad model") {
		t.Fatalf("configureSession error = %v", err)
	}

	noGateway := NewBaseACPAdapter(AcpAdapterConfig{}, nil)
	if err := noGateway.sendAuthenticate(transport); err != nil {
		t.Fatalf("sendAuthenticate without gateway returned error: %v", err)
	}

	authTransport, authCalls, closeAuthTransport := newRecordingACPTransport(t, nil)
	defer closeAuthTransport()
	gateway := NewBaseACPAdapter(AcpAdapterConfig{
		GatewayBaseURL: "https://gateway.example.test",
		GatewayHeaders: map[string]string{"x-api-key": "secret"},
	}, nil)
	if err := gateway.sendAuthenticate(authTransport); err != nil {
		t.Fatalf("sendAuthenticate returned error: %v", err)
	}
	call := readRecordedACPCall(t, authCalls)
	if call.Method != "authenticate" || !strings.Contains(string(call.Params), "https://gateway.example.test") || !strings.Contains(string(call.Params), "x-api-key") {
		t.Fatalf("authenticate call = %#v", call)
	}
}

type recordedACPCall struct {
	Method string
	Params json.RawMessage
}

func newRecordingACPTransport(t *testing.T, respond func(method string, params json.RawMessage) (any, *jsonrpcError)) (*acpTransport, <-chan recordedACPCall, func()) {
	t.Helper()
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	calls := make(chan recordedACPCall, 16)
	done := make(chan struct{})
	transport := newACPTransport(clientToServerW, serverToClientR, nil)
	if err := transport.Start(); err != nil {
		t.Fatalf("transport.Start: %v", err)
	}

	go func() {
		defer close(done)
		scanner := bufio.NewScanner(clientToServerR)
		for scanner.Scan() {
			var msg struct {
				ID     uint64          `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				continue
			}
			calls <- recordedACPCall{Method: msg.Method, Params: append(json.RawMessage(nil), msg.Params...)}
			var result any = map[string]any{}
			var rpcErr *jsonrpcError
			if respond != nil {
				result, rpcErr = respond(msg.Method, msg.Params)
			}
			if rpcErr != nil {
				writeRPCLine(t, serverToClientW, map[string]any{"jsonrpc": "2.0", "id": msg.ID, "error": rpcErr})
				continue
			}
			writeRPCLine(t, serverToClientW, map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": result})
		}
	}()

	closeFn := func() {
		_ = transport.Close()
		_ = clientToServerR.Close()
		_ = serverToClientW.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatalf("recording ACP server did not stop")
		}
	}
	return transport, calls, closeFn
}

func readRecordedACPCall(t *testing.T, calls <-chan recordedACPCall) recordedACPCall {
	t.Helper()
	select {
	case call := <-calls:
		return call
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for recorded ACP call")
		return recordedACPCall{}
	}
}

func assertNoRecordedACPCall(t *testing.T, calls <-chan recordedACPCall) {
	t.Helper()
	select {
	case call := <-calls:
		t.Fatalf("unexpected ACP call: %#v", call)
	case <-time.After(50 * time.Millisecond):
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

func writeFakeACPHealthCLI(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-acp-health")
	script := `#!/bin/sh
read line
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":2025,"serverCapabilities":{}}}'
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake ACP CLI: %v", err)
	}
	return path
}
