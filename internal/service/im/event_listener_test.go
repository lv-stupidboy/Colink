package im

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestEventListenerLifecycleGuardsAndStreamParsing(t *testing.T) {
	listener := NewEventListener("missing-lark-cli", nil, zap.NewNop())
	if listener.IsRunning() {
		t.Fatal("new listener should not be running")
	}
	listener.SetProfile("dev")
	if listener.profile != "dev" {
		t.Fatalf("profile = %q, want dev", listener.profile)
	}
	listener.Stop()
	if err := listener.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "bridge service is nil") {
		t.Fatalf("Start(nil bridge) error = %v, want bridge error", err)
	}

	listener.running = true
	if err := listener.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("Start(already running) error = %v, want running error", err)
	}
	listener.running = false

	stream := strings.NewReader("not-json\n" +
		`{"type":"unknown.event","message_type":"text","content":"ignored"}` + "\n" +
		`{"type":"im.message.receive_v1","message_type":"image","content":"ignored"}` + "\n" +
		`{"type":"im.message.receive_v1","message_type":"text","content":""}` + "\n")
	listener.processStream(context.Background(), stream)
	listener.drainStderr(strings.NewReader("status line\n"))
}

func TestEventListenerStartWithShortLivedCLI(t *testing.T) {
	cliPath := writeFakeLarkCLI(t)
	listener := NewEventListener(cliPath, &IMBridgeService{}, zap.NewNop())
	listener.SetProfile("dev")

	if err := listener.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !listener.IsRunning() {
		t.Fatal("listener should be running immediately after Start")
	}

	select {
	case <-listener.done:
	case <-time.After(time.Second):
		t.Fatal("listener subprocess did not exit")
	}
	waitUntil(t, func() bool { return !listener.IsRunning() })
	listener.Stop()
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not satisfied")
}
