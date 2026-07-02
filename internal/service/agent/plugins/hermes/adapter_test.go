package hermes

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
)

func TestGenerateHermesConfigCustomProvider(t *testing.T) {
	dir := t.TempDir()
	baseAgent := &model.BaseAgent{
		ApiURL:       "https://example.test/v1",
		ApiToken:     "secret",
		DefaultModel: "qwen3.7-plus",
	}

	generateHermesConfig(baseAgent, &agent.ExecutionRequest{ConfigDir: dir})

	content, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := `model:
  default: "qwen3.7-plus"
  provider: custom
  base_url: "https://example.test/v1"
`
	if string(content) != want {
		t.Fatalf("unexpected config:\n%s", content)
	}
}

func TestGenerateHermesConfigDefaultProviderAndNoDir(t *testing.T) {
	generateHermesConfig(&model.BaseAgent{DefaultModel: "claude-sonnet"}, nil)

	dir := t.TempDir()
	generateHermesConfig(&model.BaseAgent{DefaultModel: "claude-sonnet"}, &agent.ExecutionRequest{ConfigDir: dir})

	content, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := `model:
  default: "claude-sonnet"
`
	if string(content) != want {
		t.Fatalf("unexpected config:\n%s", content)
	}
}

func TestBuildHermesEnv(t *testing.T) {
	baseAgent := &model.BaseAgent{
		ApiURL:       "https://example.test/v1",
		ApiToken:     "secret",
		DefaultModel: "qwen3.7-plus",
	}
	req := &agent.ExecutionRequest{ConfigDir: "/tmp/hermes-home"}

	got := buildHermesEnv(baseAgent, req)
	want := []string{
		"HERMES_INFERENCE_PROVIDER=custom",
		"CUSTOM_BASE_URL=https://example.test/v1",
		"OPENAI_API_KEY=secret",
		"HERMES_HOME=/tmp/hermes-home",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("env=%v, want %v", got, want)
	}

	if got := buildHermesEnv(&model.BaseAgent{DefaultModel: "local"}, &agent.ExecutionRequest{}); len(got) != 0 {
		t.Fatalf("expected empty env without custom api/config dir, got %v", got)
	}
}

func TestHermesMasks(t *testing.T) {
	if got := maskToken(""); got != "<empty>" {
		t.Fatalf("empty token mask=%q", got)
	}
	if got := maskToken("short"); got != "****" {
		t.Fatalf("short token mask=%q", got)
	}
	if got := maskToken("sk-123456789"); got != "sk-1****6789" {
		t.Fatalf("long token mask=%q", got)
	}
	if got := maskURL(""); got != "<empty>" {
		t.Fatalf("empty url mask=%q", got)
	}
	if got := maskURL("https://coding.dashscope.aliyuncs.com/v1"); got != "https://co****ncs.com/v1" {
		t.Fatalf("long url mask=%q", got)
	}
}
