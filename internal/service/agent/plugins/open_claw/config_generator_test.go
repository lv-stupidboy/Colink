package open_claw

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
)

func TestBuildOpenClawConfigCustomProvider(t *testing.T) {
	cfg := buildOpenClawConfig(&model.BaseAgent{
		ApiURL:       "https://example.test/v1",
		ApiToken:     "secret",
		DefaultModel: "qwen3.7-plus",
	}, 18789)

	if cfg.Gateway == nil || cfg.Gateway.Port != 18789 || cfg.Gateway.Auth.Mode != "none" {
		t.Fatalf("unexpected gateway config: %#v", cfg.Gateway)
	}
	provider := cfg.Models.Providers["custom"]
	if provider.BaseURL != "https://example.test/v1" || provider.APIKey != "secret" {
		t.Fatalf("unexpected provider: %#v", provider)
	}
	if len(provider.Models) != 1 || provider.Models[0].ID != "qwen3.7-plus" {
		t.Fatalf("unexpected models: %#v", provider.Models)
	}
}

func TestBuildOpenClawConfigDefaultProviderReasoning(t *testing.T) {
	cfg := buildOpenClawConfig(&model.BaseAgent{DefaultModel: "claude-3-7-sonnet"}, 19000)
	models := cfg.Models.Providers["anthropic"].Models
	if len(models) != 1 || !models[0].Reasoning {
		t.Fatalf("expected claude model to enable reasoning, got %#v", models)
	}

	if isReasoningModel("glm-5") {
		t.Fatal("glm-5 should not be classified as a reasoning model by the OpenClaw heuristic")
	}
}

func TestGenerateOpenClawConfigWritesJSON(t *testing.T) {
	dir := t.TempDir()
	baseAgent := &model.BaseAgent{ApiURL: "https://example.test/v1", ApiToken: "secret", DefaultModel: "glm-5"}

	generateOpenClawConfig(baseAgent, &agent.ExecutionRequest{ConfigDir: dir}, 18888)

	content, err := os.ReadFile(filepath.Join(dir, "openclaw.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg OpenClawConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Gateway.Port != 18888 || cfg.Models.Providers["custom"].Models[0].ID != "glm-5" {
		t.Fatalf("unexpected generated config: %#v", cfg)
	}
}

func TestBuildOpenClawEnv(t *testing.T) {
	got := buildOpenClawEnv(&model.BaseAgent{DefaultModel: "glm-5"}, &agent.ExecutionRequest{ConfigDir: "/tmp/openclaw"}, "token")
	want := []string{
		"OPENCLAW_STATE_DIR=/tmp/openclaw",
		"OPENCLAW_CONFIG_PATH=/tmp/openclaw/openclaw.json",
		"OPENCLAW_HIDE_BANNER=1",
		"OPENCLAW_SUPPRESS_NOTES=1",
		"OPENCLAW_GATEWAY_TOKEN=token",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("env=%v, want %v", got, want)
	}
}
