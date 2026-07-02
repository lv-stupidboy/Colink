package claude_code

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestParseStreamJSONLineCoversEventsAndMessages(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		streaming bool
		assert    func(t *testing.T, chunks []agent.Chunk)
	}{
		{
			name: "text delta",
			line: `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}}`,
			assert: func(t *testing.T, chunks []agent.Chunk) {
				assertChunk(t, chunks, agent.ChunkTypeText, "hello")
			},
		},
		{
			name: "thinking lifecycle",
			line: `{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"thinking"}}}`,
			assert: func(t *testing.T, chunks []agent.Chunk) {
				assertChunk(t, chunks, agent.ChunkTypeThinking, "")
			},
		},
		{
			name: "tool use",
			line: `{"type":"stream_event","event":{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","name":"Read","id":"tool-1","input":{"file":"main.go"}}}}`,
			assert: func(t *testing.T, chunks []agent.Chunk) {
				assertChunk(t, chunks, agent.ChunkTypeToolUse, "")
				if chunks[0].ToolName != "Read" || chunks[0].ToolID != "tool-1" || chunks[0].ToolIndex != 2 {
					t.Fatalf("tool chunk = %#v", chunks[0])
				}
			},
		},
		{
			name: "question tool",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"AskUserQuestion","id":"q1","input":{"questions":[{"header":"确认","question":"继续吗","options":[{"label":"继续","description":"go"}]}]}}]}}`,
			streaming: true,
			assert: func(t *testing.T, chunks []agent.Chunk) {
				assertChunk(t, chunks, agent.ChunkTypeQuestion, "")
				if len(chunks[0].Questions) != 1 || chunks[0].Questions[0].Header != "确认" {
					t.Fatalf("question chunk = %#v", chunks[0])
				}
			},
		},
		{
			name: "input json delta",
			line: `{"type":"stream_event","event":{"type":"content_block_delta","index":4,"delta":{"type":"input_json_delta","partial_json":"{\"a\":"}}}`,
			assert: func(t *testing.T, chunks []agent.Chunk) {
				assertChunk(t, chunks, agent.ChunkTypeInputJSONDelta, "")
				if chunks[0].ToolIndex != 4 || chunks[0].PartialJSON == "" {
					t.Fatalf("input delta = %#v", chunks[0])
				}
			},
		},
		{
			name: "user tool result",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool-1","content":[{"type":"text","text":"done"}],"is_error":true}]}}`,
			assert: func(t *testing.T, chunks []agent.Chunk) {
				assertChunk(t, chunks, agent.ChunkTypeToolResult, "done")
				if !chunks[0].IsError || chunks[0].ToolID != "tool-1" {
					t.Fatalf("tool result = %#v", chunks[0])
				}
			},
		},
		{
			name: "usage result",
			line: `{"type":"result","result":"final","usage":{"input_tokens":10,"output_tokens":3,"cache_read_input_tokens":2},"cost_usd":0.1,"duration_ms":20,"duration_api_ms":10,"num_turns":1}`,
			assert: func(t *testing.T, chunks []agent.Chunk) {
				if len(chunks) != 2 || chunks[0].Type != agent.ChunkTypeText || chunks[1].Type != agent.ChunkTypeUsage {
					t.Fatalf("chunks = %#v", chunks)
				}
				if chunks[1].Usage.InputTokens != 10 || chunks[1].Usage.CostUsd != 0.1 {
					t.Fatalf("usage = %#v", chunks[1].Usage)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(t, parseStreamJSONLine(tc.line, tc.streaming))
		})
	}
	if chunks := parseStreamJSONLine("{not-json", false); chunks != nil {
		t.Fatalf("bad json chunks = %#v", chunks)
	}
	if got := parseToolResultContent(json.RawMessage(`{"unexpected":true}`)); !strings.Contains(got, "unexpected") {
		t.Fatalf("fallback tool result = %q", got)
	}
}

func TestClaudeAdapterHelpers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ISDP_MCP_SERVER_PATH", "/tmp/isdp-mcp-server")
	writeTestFile(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{"custom":{"command":"custom-mcp"}}}`)

	base := &model.BaseAgent{
		CliPath:        "claude-bin",
		ApiURL:         "https://anthropic.example",
		ApiToken:       "sk-test-token",
		GitBashPath:    "/usr/bin/bash",
		DefaultModel:   "claude-sonnet",
		TimeoutMinutes: 1,
	}
	adapter := NewClaudeCLIAdapter(base).(*ClaudeCLIAdapter)
	if adapter.cliPath != "claude-bin" || adapter.timeout == 0 || adapter.GetCurrentProcess() != nil {
		t.Fatalf("adapter defaults = %#v", adapter)
	}

	env := envMap(adapter.buildEnv(&agent.ExecutionRequest{ConfigDir: "/tmp/claude-config"}))
	for key, want := range map[string]string{
		"ANTHROPIC_BASE_URL":       "https://anthropic.example",
		"ANTHROPIC_AUTH_TOKEN":     "sk-test-token",
		"ANTHROPIC_MODEL":          "claude-sonnet",
		"CLAUDE_CONFIG_DIR":        "/tmp/claude-config",
		"CLAUDE_CODE_GIT_BASH_PATH": "/usr/bin/bash",
		"CLAUDE_NO_INTERACTIVE":     "1",
	} {
		if env[key] != want {
			t.Fatalf("%s = %q, want %q", key, env[key], want)
		}
	}

	mcp := adapter.buildMCPConfig(&agent.ExecutionRequest{CallbackToken: "0123456789abcdef-token", APIURL: "http://api", InvocationID: uuid.New()})
	if !strings.Contains(mcp, "custom") || !strings.Contains(mcp, "isdp-memory") || !strings.Contains(mcp, "ISDP_CALLBACK_TOKEN") {
		t.Fatalf("mcp config = %s", mcp)
	}
	if got := adapter.buildMCPConfig(&agent.ExecutionRequest{}); !strings.Contains(got, "custom") {
		t.Fatalf("user-only mcp config = %s", got)
	}

	input := adapter.buildMultimodalInput("describe", []model.ImageContent{{MimeType: "image/png", Data: "abc"}})
	if !strings.HasSuffix(input, "\n") || !strings.Contains(input, `"type":"image"`) || !strings.Contains(input, `"describe"`) {
		t.Fatalf("multimodal input = %s", input)
	}
	prompt := adapter.buildPromptFromRequest(&agent.ExecutionRequest{Input: "hello\nworld"})
	if !strings.Contains(prompt, "hello") || !strings.Contains(prompt, "world") {
		t.Fatalf("prompt = %q", prompt)
	}
	if got := maskToken("1234567890"); got != "1234****7890" {
		t.Fatalf("maskToken = %q", got)
	}
	if err := adapter.SendToolResult(uuid.New(), "tool-1", "answer"); err == nil {
		t.Fatalf("SendToolResult without stdin should fail")
	}
	if err := adapter.StopSession("missing"); err != nil {
		t.Fatalf("StopSession missing: %v", err)
	}
	if status := adapter.GetSessionStatus("missing"); status != agent.SessionStatusStopped {
		t.Fatalf("missing status = %s", status)
	}
}

func TestClaudeConfigGeneratorCopiesAssets(t *testing.T) {
	root := t.TempDir()
	skillStorage := filepath.Join(root, "skills-store")
	commandStorage := filepath.Join(root, "commands-store")
	subagentStorage := filepath.Join(root, "subagents-store")
	ruleStorage := filepath.Join(root, "rules-store")
	configPath := filepath.Join(root, "config")
	settingsDir := filepath.Join(root, "settings")

	skillID := uuid.New()
	writeTestFile(t, filepath.Join(skillStorage, skillID.String(), "SKILL.md"), "# Skill")
	writeTestFile(t, filepath.Join(skillStorage, skillID.String(), "nested", "note.md"), "nested")
	writeTestFile(t, filepath.Join(commandStorage, "Build.md"), "# Build")
	writeTestFile(t, filepath.Join(subagentStorage, "Code-Reviewer.md"), "# Reviewer")
	writeTestFile(t, filepath.Join(ruleStorage, "Secure.md"), "# Secure")
	writeTestFile(t, filepath.Join(settingsDir, "settings.json"), `{"theme":"dark"}`)

	generator := NewClaudeConfigGenerator(skillStorage, subagentStorage, commandStorage, ruleStorage, zap.NewNop())
	result, err := generator.GenerateConfig(context.Background(), &agent.ConfigGenerateRequest{
		AgentRoleID:   uuid.New(),
		ConfigPath:    configPath,
		CleanExisting: true,
		Skills:        []*model.Skill{{ID: skillID, Name: "review"}},
		Commands:      []*model.Command{{Name: "Build"}},
		Subagents:     []*model.Subagent{{Name: "Code Reviewer", Description: "reviews", Content: "fallback"}},
		Rules:         []*model.Rule{{Name: "Secure"}},
		Settings:      []*model.Settings{{Name: "defaults", DirectoryPath: settingsDir}},
	})
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}
	if result.SkillsCount != 1 || result.CommandsCount != 1 || result.SubagentsCount != 1 || result.RulesCount != 1 || result.SettingsCount != 1 {
		t.Fatalf("result = %#v", result)
	}
	for _, path := range []string{
		filepath.Join(configPath, "skills", "review", "SKILL.md"),
		filepath.Join(configPath, "skills", "review", "nested", "note.md"),
		filepath.Join(configPath, "commands", "Build.md"),
		filepath.Join(configPath, "agents", "Code-Reviewer.md"),
		filepath.Join(configPath, "rules", "Secure.md"),
		filepath.Join(configPath, "settings.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %s: %v", path, err)
		}
	}
	preview, err := generator.PreviewConfig(context.Background(), &agent.ConfigPreviewRequest{ConfigPath: configPath})
	if err != nil || len(preview.Files) != 0 {
		t.Fatalf("PreviewConfig = %#v err=%v", preview, err)
	}

	fallbackConfig := filepath.Join(root, "fallback-config")
	fallbackResult, err := generator.GenerateConfig(context.Background(), &agent.ConfigGenerateRequest{
		AgentRoleID:   uuid.New(),
		ConfigPath:    fallbackConfig,
		CleanExisting: true,
		Subagents:     []*model.Subagent{{Name: "Inline Agent", Description: "inline", Content: "body"}},
		Settings:      []*model.Settings{{Name: "bad"}},
		Skills:        []*model.Skill{{ID: uuid.New(), Name: "missing"}},
		Commands:      []*model.Command{{Name: "Missing"}},
		Rules:         []*model.Rule{{Name: "MissingRule"}},
	})
	if err != nil {
		t.Fatalf("GenerateConfig fallback: %v", err)
	}
	if fallbackResult.SubagentsCount != 1 || fallbackResult.SettingsCount != 0 || fallbackResult.SkillsCount != 0 || fallbackResult.CommandsCount != 0 || fallbackResult.RulesCount != 0 {
		t.Fatalf("fallback result = %#v", fallbackResult)
	}
	content, err := os.ReadFile(filepath.Join(fallbackConfig, "agents", "Inline-Agent.md"))
	if err != nil || !strings.Contains(string(content), "body") {
		t.Fatalf("fallback subagent content = %q err=%v", content, err)
	}
}

func assertChunk(t *testing.T, chunks []agent.Chunk, typ agent.ChunkType, content string) {
	t.Helper()
	if len(chunks) != 1 || chunks[0].Type != typ || chunks[0].Content != content {
		t.Fatalf("chunks = %#v, want one %s %q", chunks, typ, content)
	}
}

func envMap(env []string) map[string]string {
	result := make(map[string]string, len(env))
	for _, item := range env {
		if key, value, ok := strings.Cut(item, "="); ok {
			result[key] = value
		}
	}
	return result
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
