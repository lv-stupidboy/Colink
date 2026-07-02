package open_claw

import (
	"context"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/google/uuid"
)

func TestNewOpenClawAdapterDefaultsAndBuildArgs(t *testing.T) {
	baseAgent := &model.BaseAgent{DefaultModel: "glm-5"}
	adapter, ok := NewOpenClawAdapter(baseAgent).(*OpenClawAdapter)
	if !ok {
		t.Fatalf("expected *OpenClawAdapter")
	}
	if adapter.BaseACPAdapter.Config.CliPath != "openclaw" {
		t.Fatalf("expected default cli path openclaw, got %q", adapter.BaseACPAdapter.Config.CliPath)
	}

	invocationID := uuid.New()
	args := adapter.BaseACPAdapter.Config.BuildArgs(&agent.ExecutionRequest{InvocationID: invocationID})
	assertArgValue(t, args, "--url", "ws://127.0.0.1:18789")
	assertArgValue(t, args, "--session", "agent:main:"+invocationID.String())
	if !containsArg(args, "acp") || !containsArg(args, "--reset-session") {
		t.Fatalf("expected acp and --reset-session in args, got %v", args)
	}
}

func TestNewOpenClawAdapterCustomCLIAndBuildEnv(t *testing.T) {
	baseAgent := &model.BaseAgent{CliPath: "/bin/openclaw", DefaultModel: "glm-5"}
	adapter := NewOpenClawAdapter(baseAgent).(*OpenClawAdapter)
	if adapter.BaseACPAdapter.Config.CliPath != "/bin/openclaw" {
		t.Fatalf("expected custom cli path, got %q", adapter.BaseACPAdapter.Config.CliPath)
	}

	env := adapter.BaseACPAdapter.Config.BuildEnv(&agent.ExecutionRequest{ConfigDir: t.TempDir()})
	if len(env) < 4 {
		t.Fatalf("expected OpenClaw env, got %v", env)
	}
}

func TestBuildSessionKeyFallback(t *testing.T) {
	key := buildSessionKey(&agent.ExecutionRequest{})
	if len(key) <= len("agent:main:") || key[:len("agent:main:")] != "agent:main:" {
		t.Fatalf("unexpected session key %q", key)
	}
}

func TestOpenClawMaskURLAndGatewayPort(t *testing.T) {
	if got := maskURL(""); got != "<empty>" {
		t.Fatalf("empty url mask=%q", got)
	}
	if got := maskURL("short"); got != "sh****" {
		t.Fatalf("short url mask=%q", got)
	}
	if got := maskURL("https://coding.dashscope.aliyuncs.com/v1"); got != "https://co****ncs.com/v1" {
		t.Fatalf("long url mask=%q", got)
	}
	if got := GetGatewayPort(&agent.ExecutionRequest{}); got != DefaultGatewayPort {
		t.Fatalf("gateway port=%d, want %d", got, DefaultGatewayPort)
	}
}

func TestGatewayManagerConfigurationHelpers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager := NewGatewayManager(19001, "")
	if manager.GetGatewayURL() != "ws://127.0.0.1:19001" {
		t.Fatalf("unexpected gateway url %q", manager.GetGatewayURL())
	}
	if token := manager.GetGatewayToken(); token != "" {
		t.Fatalf("expected no token without config, got %q", token)
	}
	env := manager.buildGatewayEnv()
	if !containsArg(env, "OPENCLAW_STATE_DIR="+home+"/.openclaw") ||
		!containsArg(env, "OPENCLAW_CONFIG_PATH="+home+"/.openclaw/openclaw.json") ||
		!containsArg(env, "OPENCLAW_HIDE_BANNER=1") ||
		!containsArg(env, "OPENCLAW_SUPPRESS_NOTES=1") {
		t.Fatalf("missing gateway env entries in %v", env)
	}
}

func TestGatewayManagerReadsTokenAndMasks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mustWrite(t, home+"/.openclaw/openclaw.json", `{"gateway":{"auth":{"mode":"token","token":"1234567890abcdef"}}}`)

	manager := NewGatewayManager(19002, "")
	if token := manager.GetGatewayToken(); token != "1234567890abcdef" {
		t.Fatalf("token=%q", token)
	}
	if got := maskToken(""); got != "<empty>" {
		t.Fatalf("empty token mask=%q", got)
	}
	if got := maskToken("short"); got != "****" {
		t.Fatalf("short token mask=%q", got)
	}
	if got := maskToken("1234567890abcdef"); got != "1234****cdef" {
		t.Fatalf("long token mask=%q", got)
	}
}

func TestGatewayManagerReadyChecks(t *testing.T) {
	manager := NewGatewayManager(1, t.TempDir())
	if manager.isGatewayRunning() {
		t.Fatal("port 1 should not be treated as a running OpenClaw gateway in tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := manager.WaitForReady(ctx, time.Hour); err == nil {
		t.Fatal("expected context timeout while waiting for unavailable gateway")
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func assertArgValue(t *testing.T, args []string, key string, want string) {
	t.Helper()
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key {
			if args[i+1] != want {
				t.Fatalf("%s=%q, want %q in args %v", key, args[i+1], want, args)
			}
			return
		}
	}
	t.Fatalf("missing %s in args %v", key, args)
}
