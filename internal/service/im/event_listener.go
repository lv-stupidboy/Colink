package im

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EventListener receives Feishu events via lark-cli's WebSocket-based event subscription.
// It spawns `lark-cli event +subscribe --compact` as a subprocess and reads NDJSON from stdout.
// This approach does NOT require a public webhook URL — lark-cli establishes an outbound
// WebSocket connection to Feishu's cloud.
type EventListener struct {
	cliPath string
	profile string // optional lark-cli profile
	bridge  *IMBridgeService
	logger  *zap.Logger
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.Mutex
	running bool
}

// NewEventListener creates a new event listener.
func NewEventListener(cliPath string, bridge *IMBridgeService, logger *zap.Logger) *EventListener {
	return &EventListener{
		cliPath: cliPath,
		bridge:  bridge,
		logger:  logger,
		done:    make(chan struct{}),
	}
}

// SetProfile sets the lark-cli profile to use (optional).
func (l *EventListener) SetProfile(profile string) {
	l.profile = profile
}

// Start begins listening for events via lark-cli event +subscribe.
// This call is non-blocking; it spawns a goroutine that reads the subprocess stdout.
func (l *EventListener) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return fmt.Errorf("event listener is already running")
	}

	if l.bridge == nil {
		return fmt.Errorf("bridge service is nil")
	}

	listenerCtx, cancel := context.WithCancel(ctx)
	l.cancel = cancel

	args := []string{"event", "+subscribe", "--compact", "--quiet"}
	if l.profile != "" {
		args = append([]string{"--profile", l.profile}, args...)
	}

	l.cmd = exec.CommandContext(listenerCtx, l.cliPath, args...)

	stdout, err := l.cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := l.cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := l.cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start lark-cli event listener: %w", err)
	}

	l.running = true
	l.done = make(chan struct{})

	// Drain stderr in background (lark-cli writes status messages there)
	go l.drainStderr(stderr)

	// Read NDJSON lines from stdout
	go l.processStream(listenerCtx, stdout)

	// Wait for subprocess exit
	go func() {
		defer close(l.done)
		if err := l.cmd.Wait(); err != nil {
			l.mu.Lock()
			if l.running {
				l.logger.Warn("lark-cli event listener exited with error", zap.Error(err))
			}
			l.mu.Unlock()
		}
		l.mu.Lock()
		l.running = false
		l.mu.Unlock()
	}()

	l.logger.Info("IM event listener started via lark-cli WebSocket")
	return nil
}

// Stop gracefully shuts down the event listener.
func (l *EventListener) Stop() {
	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()
		return
	}
	l.logger.Info("Stopping IM event listener...")
	l.running = false
	l.cancel()
	l.mu.Unlock()

	// Wait for subprocess to exit (with timeout)
	select {
	case <-l.done:
		l.logger.Info("IM event listener stopped")
	case <-time.After(5 * time.Second):
		l.logger.Warn("IM event listener stop timed out, killing process")
		if l.cmd != nil && l.cmd.Process != nil {
			l.cmd.Process.Kill()
		}
	}
}

// IsRunning returns whether the listener is currently active.
func (l *EventListener) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

// processStream reads NDJSON lines from lark-cli stdout and dispatches events.
func (l *EventListener) processStream(ctx context.Context, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	// Allow lines up to 1MB (large messages)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		l.handleLine(ctx, line)
	}

	if err := scanner.Err(); err != nil {
		l.mu.Lock()
		if l.running {
			l.logger.Error("lark-cli event stream read error", zap.Error(err))
		}
		l.mu.Unlock()
	}
}

// larkCLICompactEvent represents a simplified event from `lark-cli event +subscribe --compact`.
// With --compact, lark-cli outputs flat key-value NDJSON. We map known fields here.
// Actual compact output example:
//
//	{"chat_id":"oc_xxx","chat_type":"p2p","content":"？","message_id":"om_xxx",
//	 "message_type":"text","sender_id":"ou_xxx","type":"im.message.receive_v1"}
type larkCLICompactEvent struct {
	EventType string `json:"type"` // "im.message.receive_v1" etc.
	EventID   string `json:"id"`   // event ID (same as message_id for IM events)

	// im.message.receive_v1 fields (compact mode)
	ChatID      string `json:"chat_id"`
	ChatType    string `json:"chat_type"`
	MessageID   string `json:"message_id"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`   // compact mode: plain text directly (not JSON-encoded)
	SenderID    string `json:"sender_id"` // sender open_id
}

// handleLine parses a single NDJSON line and dispatches to the bridge service.
func (l *EventListener) handleLine(ctx context.Context, line string) {
	var evt larkCLICompactEvent
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		l.logger.Debug("skipping non-JSON event line", zap.String("line", line))
		return
	}

	// Route by event type
	switch evt.EventType {
	case "im.message.receive_v1":
		l.handleMessageEvent(ctx, &evt)
	default:
		l.logger.Debug("ignoring event type", zap.String("event_type", evt.EventType))
	}
}

// handleMessageEvent processes an im.message.receive_v1 event from the compact stream.
func (l *EventListener) handleMessageEvent(ctx context.Context, evt *larkCLICompactEvent) {
	if evt.MessageType != "text" {
		l.logger.Debug("skipping non-text message",
			zap.String("message_type", evt.MessageType),
			zap.String("chat_id", evt.ChatID))
		return
	}

	// In compact mode, "content" is already plain text (not JSON-encoded)
	text := evt.Content
	if text == "" {
		return
	}

	userID := evt.SenderID

	chatType := evt.ChatType
	if chatType == "" {
		chatType = "p2p"
	}

	l.logger.Info("received IM message via lark-cli event stream",
		zap.String("chat_id", evt.ChatID),
		zap.String("user_id", userID),
		zap.String("message_id", evt.MessageID),
		zap.Int("text_len", len(text)))

	if err := l.bridge.HandleInboundMessage(
		ctx,
		"feishu",
		evt.ChatID,
		chatType,
		userID,
		"", // userName not available in compact mode
		evt.MessageID,
		text,
	); err != nil {
		l.logger.Error("failed to handle inbound message from event stream",
			zap.String("chat_id", evt.ChatID),
			zap.String("message_id", evt.MessageID),
			zap.Error(err))
	}
}

// drainStderr reads and logs stderr output from the subprocess.
func (l *EventListener) drainStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			l.logger.Debug("lark-cli stderr", zap.String("line", line))
		}
	}
}
