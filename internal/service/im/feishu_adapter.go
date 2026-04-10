package im

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// LarkClient defines the interface for Lark CLI operations.
type LarkClient interface {
	SendTextMessage(ctx context.Context, chatID, text string) error
	SendCardMessage(ctx context.Context, chatID, cardJSON string) error
	ReplyMessage(ctx context.Context, chatID, messageID, text string) error
	CheckHealth(ctx context.Context) error
	// Streaming card methods
	CreateCard(ctx context.Context, chatID string) (cardID, messageID string, err error)
	UpdateCardContent(ctx context.Context, cardID, content string, sequence int) error
	SetStreamingMode(ctx context.Context, cardID string, enabled bool, sequence int) error
}

const (
	streamingThrottleDelay = 200 * time.Millisecond
)

type cardState struct {
	mu            sync.Mutex
	cardID        string
	messageID     string
	sequence      int
	startTime     time.Time
	pendingText   string
	lastUpdateAt  time.Time
	throttleTimer *time.Timer
}

// FeishuAdapter implements IMAdapter for Feishu (Lark) platform.
type FeishuAdapter struct {
	client     LarkClient
	logger     *zap.Logger
	cardStates sync.Map
}

// NewFeishuAdapter creates a new Feishu adapter.
func NewFeishuAdapter(client LarkClient, logger *zap.Logger) *FeishuAdapter {
	return &FeishuAdapter{
		client:     client,
		logger:     logger,
		cardStates: sync.Map{},
	}
}

// Platform returns the platform identifier.
func (a *FeishuAdapter) Platform() string {
	return "feishu"
}

// SendText sends a text message to the chat.
func (a *FeishuAdapter) SendText(ctx context.Context, chatID, text string) SendResult {
	err := a.client.SendTextMessage(ctx, chatID, text)
	if err != nil {
		a.logger.Error("failed to send text message",
			zap.String("chatID", chatID),
			zap.Error(err))
		return SendResult{
			OK:    false,
			Error: err.Error(),
		}
	}
	return SendResult{OK: true}
}

// SendCard sends a card message to the chat.
func (a *FeishuAdapter) SendCard(ctx context.Context, chatID, cardJSON string) SendResult {
	err := a.client.SendCardMessage(ctx, chatID, cardJSON)
	if err != nil {
		a.logger.Error("failed to send card message",
			zap.String("chatID", chatID),
			zap.Error(err))
		return SendResult{
			OK:    false,
			Error: err.Error(),
		}
	}
	return SendResult{OK: true}
}

// ReplyText sends a reply message to a specific message.
func (a *FeishuAdapter) ReplyText(ctx context.Context, chatID, messageID, text string) SendResult {
	err := a.client.ReplyMessage(ctx, chatID, messageID, text)
	if err != nil {
		a.logger.Error("failed to reply message",
			zap.String("chatID", chatID),
			zap.String("messageID", messageID),
			zap.Error(err))
		return SendResult{
			OK:    false,
			Error: err.Error(),
		}
	}
	return SendResult{OK: true}
}

// CreateStreamingCard creates a streaming card and returns the card ID.
func (a *FeishuAdapter) CreateStreamingCard(ctx context.Context, chatID string) (string, error) {
	cardID, messageID, err := a.client.CreateCard(ctx, chatID)
	if err != nil {
		a.logger.Error("failed to create streaming card",
			zap.String("chatID", chatID),
			zap.Error(err))
		return "", fmt.Errorf("failed to create streaming card: %w", err)
	}

	state := &cardState{
		cardID:       cardID,
		messageID:    messageID,
		sequence:     0,
		startTime:    time.Now(),
		lastUpdateAt: time.Now(),
	}
	a.cardStates.Store(cardID, state)

	if err := a.client.SetStreamingMode(ctx, cardID, true, 0); err != nil {
		a.logger.Warn("failed to enable streaming mode",
			zap.String("cardID", cardID),
			zap.Error(err))
	}

	a.logger.Debug("created streaming card",
		zap.String("cardID", cardID),
		zap.String("messageID", messageID))

	return cardID, nil
}

// UpdateStreamingCard updates a streaming card with throttling.
func (a *FeishuAdapter) UpdateStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	val, ok := a.cardStates.Load(cardID)
	if !ok {
		return fmt.Errorf("card state not found for cardID: %s", cardID)
	}

	state := val.(*cardState)
	state.mu.Lock()
	defer state.mu.Unlock()

	state.pendingText = content
	state.sequence = sequence

	elapsed := time.Since(state.lastUpdateAt)

	if elapsed >= streamingThrottleDelay {
		return a.flushCardUpdate(ctx, state)
	}

	if state.throttleTimer != nil {
		state.throttleTimer.Stop()
	}

	state.throttleTimer = time.AfterFunc(streamingThrottleDelay, func() {
		state.mu.Lock()
		defer state.mu.Unlock()

		if err := a.flushCardUpdate(context.Background(), state); err != nil {
			a.logger.Error("failed to flush throttled card update",
				zap.String("cardID", state.cardID),
				zap.Error(err))
		}
	})

	return nil
}

// FinalizeStreamingCard finalizes a streaming card with completion footer.
func (a *FeishuAdapter) FinalizeStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	val, ok := a.cardStates.Load(cardID)
	if !ok {
		return fmt.Errorf("card state not found for cardID: %s", cardID)
	}

	state := val.(*cardState)
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.throttleTimer != nil {
		state.throttleTimer.Stop()
		state.throttleTimer = nil
	}

	duration := time.Since(state.startTime)

	finalContent := a.buildFinalCardContent(content, duration)

	if err := a.client.UpdateCardContent(ctx, cardID, finalContent, sequence); err != nil {
		a.logger.Error("failed to finalize card content",
			zap.String("cardID", cardID),
			zap.Error(err))
		return fmt.Errorf("failed to finalize card content: %w", err)
	}

	if err := a.client.SetStreamingMode(ctx, cardID, false, sequence+1); err != nil {
		a.logger.Warn("failed to disable streaming mode",
			zap.String("cardID", cardID),
			zap.Error(err))
	}

	a.cardStates.Delete(cardID)

	a.logger.Debug("finalized streaming card",
		zap.String("cardID", cardID),
		zap.Int("sequence", sequence),
		zap.Duration("duration", duration))

	return nil
}

func (a *FeishuAdapter) flushCardUpdate(ctx context.Context, state *cardState) error {
	if state.pendingText == "" {
		return nil
	}

	if err := a.client.UpdateCardContent(ctx, state.cardID, state.pendingText, state.sequence); err != nil {
		a.logger.Error("failed to update card content",
			zap.String("cardID", state.cardID),
			zap.Int("sequence", state.sequence),
			zap.Error(err))
		return fmt.Errorf("failed to update card content: %w", err)
	}

	state.lastUpdateAt = time.Now()
	state.pendingText = ""

	return nil
}

func (a *FeishuAdapter) buildFinalCardContent(content string, duration time.Duration) string {
	footer := fmt.Sprintf("\n\n---\n✅ 完成 | 用时: %s", duration.Round(time.Millisecond))
	return content + footer
}

// CheckHealth checks if the Feishu adapter is healthy.
func (a *FeishuAdapter) CheckHealth(ctx context.Context) error {
	return a.client.CheckHealth(ctx)
}

// MaxMessageLength returns the maximum message length for Feishu.
func (a *FeishuAdapter) MaxMessageLength() int {
	return 4000
}

// buildSimpleCard builds a simple Feishu card message.
func (a *FeishuAdapter) buildSimpleCard(content, color string) string {
	return fmt.Sprintf(`{
		"config": {
			"wide_screen_mode": true
		},
		"header": {
			"title": {
				"tag": "plain_text",
				"content": "%s"
			},
			"template": "%s"
		}
	}`, content, color)
}

// buildCompletionCard builds a completion summary card.
func (a *FeishuAdapter) buildCompletionCard(status string) string {
	headerColor := "green"
	if status == "failed" || status == "stopped" {
		headerColor = "red"
	}

	title := "✅ 执行完成"
	if status == "failed" {
		title = "❌ 执行失败"
	} else if status == "stopped" {
		title = "⏹️ 执行终止"
	}

	return fmt.Sprintf(`{
		"config": {
			"wide_screen_mode": true
		},
		"header": {
			"title": {
				"tag": "plain_text",
				"content": "%s"
			},
			"template": "%s"
		},
		"elements": [
			{
				"tag": "div",
				"text": {
					"tag": "plain_text",
					"content": "状态: %s"
				}
			}
		]
	}`, title, headerColor, status)
}

// Compile-time check that FeishuAdapter implements IMAdapter.
var _ IMAdapter = (*FeishuAdapter)(nil)
