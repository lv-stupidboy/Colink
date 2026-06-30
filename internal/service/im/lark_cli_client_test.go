package im

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestLarkCLIClientBasicCommands(t *testing.T) {
	cliPath := writeFakeLarkCLI(t)
	client := NewLarkCLIClient(cliPath, zap.NewNop())
	client.timeout = time.Second

	if client.cliPath != cliPath {
		t.Fatalf("cliPath = %q, want %q", client.cliPath, cliPath)
	}

	ctx := context.Background()
	if err := client.SendTextMessage(ctx, "chat-1", "hello"); err != nil {
		t.Fatalf("SendTextMessage() error = %v", err)
	}
	if err := client.SendCardMessage(ctx, "chat-1", `{"card":true}`); err != nil {
		t.Fatalf("SendCardMessage() error = %v", err)
	}
	if err := client.ReplyMessage(ctx, "chat-1", "msg-1", "reply"); err != nil {
		t.Fatalf("ReplyMessage() error = %v", err)
	}
	if err := client.CheckHealth(ctx); err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
}

func TestLarkCLIClientBasicCommandErrors(t *testing.T) {
	cliPath := writeFakeLarkCLI(t)
	client := NewLarkCLIClient(cliPath, zap.NewNop())
	client.timeout = time.Second
	t.Setenv("LARK_CLI_MODE", "fail")

	ctx := context.Background()
	for name, run := range map[string]func() error{
		"SendTextMessage": func() error { return client.SendTextMessage(ctx, "chat-1", "hello") },
		"SendCardMessage": func() error { return client.SendCardMessage(ctx, "chat-1", "{}") },
		"ReplyMessage":    func() error { return client.ReplyMessage(ctx, "chat-1", "msg-1", "reply") },
		"CheckHealth":     func() error { return client.CheckHealth(ctx) },
	} {
		if err := run(); err == nil {
			t.Fatalf("%s() error = nil, want error", name)
		}
	}
}

func TestLarkCLIClientStreamingCardLifecycle(t *testing.T) {
	cliPath := writeFakeLarkCLI(t)
	client := NewLarkCLIClient(cliPath, zap.NewNop())
	client.timeout = time.Second

	ctx := context.Background()
	cardID, err := client.CreateStreamingCardEntity(ctx, "Agent", "streaming_content")
	if err != nil {
		t.Fatalf("CreateStreamingCardEntity() error = %v", err)
	}
	if cardID != "card-123" {
		t.Fatalf("cardID = %q, want card-123", cardID)
	}

	messageID, err := client.SendCardEntityMessage(ctx, "chat-1", cardID)
	if err != nil {
		t.Fatalf("SendCardEntityMessage() error = %v", err)
	}
	if messageID != "msg-123" {
		t.Fatalf("messageID = %q, want msg-123", messageID)
	}

	if err := client.UpdateStreamingElement(ctx, cardID, "streaming_content", "hello", 2); err != nil {
		t.Fatalf("UpdateStreamingElement() error = %v", err)
	}
	if err := client.SetCardStreamingMode(ctx, cardID, true, 3); err != nil {
		t.Fatalf("SetCardStreamingMode(true) error = %v", err)
	}
	if err := client.SetCardStreamingMode(ctx, cardID, false, 4); err != nil {
		t.Fatalf("SetCardStreamingMode(false) error = %v", err)
	}
}

func TestLarkCLIClientStreamingCardCommandFailures(t *testing.T) {
	cliPath := writeFakeLarkCLI(t)
	client := NewLarkCLIClient(cliPath, zap.NewNop())
	client.timeout = time.Second
	t.Setenv("LARK_CLI_MODE", "fail")

	ctx := context.Background()
	if _, err := client.CreateStreamingCardEntity(ctx, "Agent", "element"); err == nil {
		t.Fatal("CreateStreamingCardEntity() error = nil, want error")
	}
	if _, err := client.SendCardEntityMessage(ctx, "chat-1", "card-1"); err == nil {
		t.Fatal("SendCardEntityMessage() error = nil, want error")
	}
	if err := client.UpdateStreamingElement(ctx, "card-1", "element", "content", 1); err == nil {
		t.Fatal("UpdateStreamingElement() error = nil, want error")
	}
	if err := client.SetCardStreamingMode(ctx, "card-1", true, 1); err == nil {
		t.Fatal("SetCardStreamingMode() error = nil, want error")
	}
}

func TestLarkCLIClientStreamingCardBadResponses(t *testing.T) {
	tests := []struct {
		name string
		mode string
		run  func(context.Context, *LarkCLIClient) error
	}{
		{
			name: "create bad json",
			mode: "badjson",
			run: func(ctx context.Context, client *LarkCLIClient) error {
				_, err := client.CreateStreamingCardEntity(ctx, "Agent", "element")
				return err
			},
		},
		{
			name: "create nonzero code",
			mode: "nonzero",
			run: func(ctx context.Context, client *LarkCLIClient) error {
				_, err := client.CreateStreamingCardEntity(ctx, "Agent", "element")
				return err
			},
		},
		{
			name: "send bad json",
			mode: "badjson",
			run: func(ctx context.Context, client *LarkCLIClient) error {
				_, err := client.SendCardEntityMessage(ctx, "chat-1", "card-1")
				return err
			},
		},
		{
			name: "send nonzero code",
			mode: "nonzero",
			run: func(ctx context.Context, client *LarkCLIClient) error {
				_, err := client.SendCardEntityMessage(ctx, "chat-1", "card-1")
				return err
			},
		},
		{
			name: "update bad json",
			mode: "badjson",
			run: func(ctx context.Context, client *LarkCLIClient) error {
				return client.UpdateStreamingElement(ctx, "card-1", "element", "content", 1)
			},
		},
		{
			name: "update nonzero code",
			mode: "nonzero",
			run: func(ctx context.Context, client *LarkCLIClient) error {
				return client.UpdateStreamingElement(ctx, "card-1", "element", "content", 1)
			},
		},
		{
			name: "streaming mode bad json",
			mode: "badjson",
			run: func(ctx context.Context, client *LarkCLIClient) error {
				return client.SetCardStreamingMode(ctx, "card-1", false, 1)
			},
		},
		{
			name: "streaming mode nonzero code",
			mode: "nonzero",
			run: func(ctx context.Context, client *LarkCLIClient) error {
				return client.SetCardStreamingMode(ctx, "card-1", false, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cliPath := writeFakeLarkCLI(t)
			client := NewLarkCLIClient(cliPath, zap.NewNop())
			client.timeout = time.Second
			t.Setenv("LARK_CLI_MODE", tt.mode)

			if err := tt.run(context.Background(), client); err == nil {
				t.Fatal("error = nil, want error")
			}
		})
	}
}

func writeFakeLarkCLI(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "lark-cli")
script := `#!/bin/sh
if [ -n "$LARK_CLI_LOG" ]; then
  printf '%s\n' "$*" >> "$LARK_CLI_LOG"
fi

if [ "$LARK_CLI_MODE" = "fail" ]; then
  echo "fake lark-cli failed" >&2
  exit 7
fi

if [ "$1" = "--version" ]; then
  echo "lark-cli test"
  exit 0
fi

if [ "$LARK_CLI_MODE" = "badjson" ]; then
  echo "{bad-json"
  exit 0
fi

if [ "$LARK_CLI_MODE" = "nonzero" ]; then
  echo '{"code":177,"msg":"denied"}'
  exit 0
fi

if [ "$1" = "api" ]; then
  case "$3" in
    "/open-apis/cardkit/v1/cards")
      echo '{"code":0,"data":{"card_id":"card-123"}}'
      ;;
    "/open-apis/im/v1/messages")
      echo '{"code":0,"data":{"message_id":"msg-123","chat_id":"chat-1"}}'
      ;;
    *)
      echo '{"code":0,"msg":"ok"}'
      ;;
  esac
  exit 0
fi

if [ "$1" = "im" ]; then
  exit 0
fi

echo "unexpected args: $*" >&2
exit 9
`
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(script, "\r\n", "\n")), 0o755); err != nil {
		t.Fatalf("write fake lark-cli: %v", err)
	}
	return path
}
