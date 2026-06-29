package open_code

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/google/uuid"
)

func TestNewOpenCodeAdapterDefaultsAndArgs(t *testing.T) {
	baseAgent := &model.BaseAgent{DefaultModel: "qwen3.7-plus"}
	adapter, ok := NewOpenCodeAdapter(baseAgent).(*OpenCodeAdapter)
	if !ok {
		t.Fatalf("expected *OpenCodeAdapter")
	}
	if adapter.BaseACPAdapter.Config.CliPath != "opencode" {
		t.Fatalf("expected default cli path opencode, got %q", adapter.BaseACPAdapter.Config.CliPath)
	}

	args := adapter.BaseACPAdapter.Config.BuildArgs(&agent.ExecutionRequest{})
	if len(args) == 1 && args[0] == "acp" {
		return
	}
	if len(args) != 3 || args[0] != "acp" || args[1] != "--port" {
		t.Fatalf("unexpected args=%v", args)
	}
	port, err := strconv.Atoi(args[2])
	if err != nil || port <= 0 {
		t.Fatalf("expected positive dynamic port, got args=%v err=%v", args, err)
	}
}

func TestNewOpenCodeAdapterCustomCLIAndEnv(t *testing.T) {
	baseAgent := &model.BaseAgent{
		CliPath:      "/bin/opencode",
		ApiURL:       "https://example.test/v1",
		ApiToken:     "secret",
		DefaultModel: "qwen3.7-plus",
	}
	adapter := NewOpenCodeAdapter(baseAgent).(*OpenCodeAdapter)
	if adapter.BaseACPAdapter.Config.CliPath != "/bin/opencode" {
		t.Fatalf("expected custom cli path, got %q", adapter.BaseACPAdapter.Config.CliPath)
	}

	env := adapter.BaseACPAdapter.Config.BuildEnv(&agent.ExecutionRequest{ConfigDir: t.TempDir(), InvocationID: uuid.New()})
	if len(env) == 0 {
		t.Fatal("expected opencode env to be generated")
	}
}

func TestOpenCodeModelRefAndConfigContent(t *testing.T) {
	if got := openCodeModelRef(""); got != "" {
		t.Fatalf("empty model ref=%q", got)
	}
	if got := openCodeModelRef("qwen3.7-plus"); got != "colink/qwen3.7-plus" {
		t.Fatalf("model ref=%q", got)
	}
	if hasConfigContent(&model.BaseAgent{}) {
		t.Fatal("empty base agent should not have config content")
	}

	content := buildOpenCodeConfigContent(&model.BaseAgent{
		ApiURL:       "https://example.test/v1",
		ApiToken:     "secret",
		DefaultModel: "qwen3.7-plus",
	})
	if content == "" {
		t.Fatal("expected config content")
	}
	var cfg openCodeConfig
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "colink/qwen3.7-plus" {
		t.Fatalf("model=%q", cfg.Model)
	}
	modelCfg := cfg.Provider[openCodeProviderID].Models["qwen3.7-plus"]
	if !modelCfg.Attachment || modelCfg.Modalities == nil {
		t.Fatalf("expected multimodal model config, got %#v", modelCfg)
	}
}
