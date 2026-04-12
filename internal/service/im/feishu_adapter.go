package im

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	streamingElementID = "content_1"
	streamingThrottle  = 200 * time.Millisecond
)

type LarkClient interface {
	SendTextMessage(ctx context.Context, chatID, text string) error
	SendCardMessage(ctx context.Context, chatID, cardJSON string) error
	ReplyMessage(ctx context.Context, chatID, messageID, text string) error
	CheckHealth(ctx context.Context) error
	CreateStreamingCardEntity(ctx context.Context, title, elementID string) (cardID string, err error)
	SendCardEntityMessage(ctx context.Context, chatID, cardID string) (messageID string, err error)
	UpdateStreamingElement(ctx context.Context, cardID, elementID, content string, sequence int) error
	SetCardStreamingMode(ctx context.Context, cardID string, enabled bool, sequence int) error
}

type streamingCardState struct {
	mu          sync.Mutex
	cardID      string
	messageID   string
	sequence    int
	startTime   time.Time
	accumulated string
	lastSent    string
	lastSentAt  time.Time
	done        chan struct{}
	dirty       chan struct{}
}

type FeishuAdapter struct {
	client     LarkClient
	logger     *zap.Logger
	cardStates sync.Map
}

func NewFeishuAdapter(client LarkClient, logger *zap.Logger) *FeishuAdapter {
	return &FeishuAdapter{
		client:     client,
		logger:     logger,
		cardStates: sync.Map{},
	}
}

func (a *FeishuAdapter) Platform() string { return "feishu" }

func (a *FeishuAdapter) SendText(ctx context.Context, chatID, text string) SendResult {
	if err := a.client.SendTextMessage(ctx, chatID, text); err != nil {
		a.logger.Error("failed to send text", zap.String("chatID", chatID), zap.Error(err))
		return SendResult{OK: false, Error: err.Error()}
	}
	return SendResult{OK: true}
}

func (a *FeishuAdapter) SendCard(ctx context.Context, chatID, cardJSON string) SendResult {
	if err := a.client.SendCardMessage(ctx, chatID, cardJSON); err != nil {
		a.logger.Error("failed to send card", zap.String("chatID", chatID), zap.Error(err))
		return SendResult{OK: false, Error: err.Error()}
	}
	return SendResult{OK: true}
}

func (a *FeishuAdapter) ReplyText(ctx context.Context, chatID, messageID, text string) SendResult {
	if err := a.client.ReplyMessage(ctx, chatID, messageID, text); err != nil {
		a.logger.Error("failed to reply", zap.String("chatID", chatID), zap.String("messageID", messageID), zap.Error(err))
		return SendResult{OK: false, Error: err.Error()}
	}
	return SendResult{OK: true}
}

func (a *FeishuAdapter) CreateStreamingCard(ctx context.Context, chatID string, agentName string) (string, error) {
	title := agentName
	if title == "" {
		title = "Agent"
	}
	cardID, err := a.client.CreateStreamingCardEntity(ctx, title, streamingElementID)
	if err != nil {
		return "", fmt.Errorf("create card entity: %w", err)
	}

	messageID, err := a.client.SendCardEntityMessage(ctx, chatID, cardID)
	if err != nil {
		return "", fmt.Errorf("send card message: %w", err)
	}

	state := &streamingCardState{
		cardID:     cardID,
		messageID:  messageID,
		sequence:   1,
		startTime:  time.Now(),
		lastSentAt: time.Now(),
		done:       make(chan struct{}),
		dirty:      make(chan struct{}, 1),
	}
	a.cardStates.Store(cardID, state)

	if err := a.client.SetCardStreamingMode(ctx, cardID, true, state.sequence); err != nil {
		a.logger.Warn("failed to enable streaming mode via settings", zap.String("cardID", cardID), zap.Error(err))
	}
	state.sequence++

	go a.flushLoop(state)

	return cardID, nil
}

func (a *FeishuAdapter) UpdateStreamingCard(ctx context.Context, cardID, content string, _ int) error {
	val, ok := a.cardStates.Load(cardID)
	if !ok {
		return fmt.Errorf("card state not found: %s", cardID)
	}

	state := val.(*streamingCardState)
	state.mu.Lock()
	state.accumulated += content
	state.mu.Unlock()

	select {
	case state.dirty <- struct{}{}:
	default:
	}

	return nil
}

func (a *FeishuAdapter) flushLoop(state *streamingCardState) {
	for {
		select {
		case <-state.done:
			return
		case <-state.dirty:
		}

		state.mu.Lock()
		elapsed := time.Since(state.lastSentAt)
		state.mu.Unlock()

		if elapsed < streamingThrottle {
			select {
			case <-time.After(streamingThrottle - elapsed):
			case <-state.done:
				return
			}
		}

		state.mu.Lock()
		text := state.accumulated
		lastSent := state.lastSent
		seq := state.sequence
		state.mu.Unlock()

		if text == "" || text == lastSent {
			continue
		}

		if err := a.client.UpdateStreamingElement(context.Background(), state.cardID, streamingElementID, text, seq); err != nil {
			a.logger.Error("failed to update streaming element",
				zap.String("cardID", state.cardID),
				zap.Int("sequence", seq),
				zap.Int("textLen", len(text)),
				zap.Error(err))
			continue
		}

		preview := text
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		a.logger.Info("streaming update sent",
			zap.String("cardID", state.cardID),
			zap.Int("sequence", seq),
			zap.Int("textLen", len(text)),
			zap.String("preview", preview))

		state.mu.Lock()
		state.sequence++
		state.lastSent = text
		state.lastSentAt = time.Now()
		state.mu.Unlock()
	}
}

func (a *FeishuAdapter) FinalizeStreamingCard(ctx context.Context, cardID, content string, _ int) error {
	val, ok := a.cardStates.Load(cardID)
	if !ok {
		return fmt.Errorf("card state not found: %s", cardID)
	}

	state := val.(*streamingCardState)
	close(state.done)

	state.mu.Lock()
	defer state.mu.Unlock()

	state.accumulated += content

	duration := time.Since(state.startTime)
	finalContent := state.accumulated + fmt.Sprintf("\n\n---\n✅ 完成 | 用时: %s", duration.Round(time.Millisecond))

	if err := a.client.UpdateStreamingElement(ctx, state.cardID, streamingElementID, finalContent, state.sequence); err != nil {
		a.logger.Error("failed to finalize card content", zap.String("cardID", cardID), zap.Error(err))
		return fmt.Errorf("finalize card content: %w", err)
	}
	state.sequence++

	if err := a.client.SetCardStreamingMode(ctx, state.cardID, false, state.sequence); err != nil {
		a.logger.Warn("failed to disable streaming mode", zap.String("cardID", cardID), zap.Error(err))
	}
	state.sequence++

	a.cardStates.Delete(cardID)

	a.logger.Info("finalized streaming card",
		zap.String("cardID", cardID),
		zap.Int("sequence", state.sequence),
		zap.Duration("duration", duration))

	return nil
}

func (a *FeishuAdapter) CheckHealth(ctx context.Context) error {
	return a.client.CheckHealth(ctx)
}

func (a *FeishuAdapter) MaxMessageLength() int { return 4000 }

func (a *FeishuAdapter) buildSimpleCard(content, color string) string {
	return fmt.Sprintf(`{
		"config": {"wide_screen_mode": true},
		"header": {
			"title": {"tag": "plain_text", "content": %q},
			"template": %q
		}
	}`, content, color)
}

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
		"config": {"wide_screen_mode": true},
		"header": {
			"title": {"tag": "plain_text", "content": "%s"},
			"template": "%s"
		},
		"elements": [
			{"tag": "div", "text": {"tag": "plain_text", "content": "状态: %s"}}
		]
	}`, title, headerColor, status)
}

var _ IMAdapter = (*FeishuAdapter)(nil)
