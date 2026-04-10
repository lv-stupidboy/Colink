package im

import (
	"context"
	"errors"
)

var ErrNotImplemented = errors.New("not implemented")

// SlackAdapter implements IMAdapter for Slack platform.
type SlackAdapter struct{}

// NewSlackAdapter creates a new Slack adapter.
func NewSlackAdapter() *SlackAdapter {
	return &SlackAdapter{}
}

// Platform returns the platform identifier.
func (a *SlackAdapter) Platform() string {
	return "slack"
}

// SendText sends a text message to the channel.
func (a *SlackAdapter) SendText(ctx context.Context, chatID, text string) SendResult {
	return SendResult{
		OK:    false,
		Error: ErrNotImplemented.Error(),
	}
}

// SendCard sends a card message to the channel.
func (a *SlackAdapter) SendCard(ctx context.Context, chatID, cardJSON string) SendResult {
	return SendResult{
		OK:    false,
		Error: ErrNotImplemented.Error(),
	}
}

// ReplyText sends a reply message to a specific message.
func (a *SlackAdapter) ReplyText(ctx context.Context, chatID, messageID, text string) SendResult {
	return SendResult{
		OK:    false,
		Error: ErrNotImplemented.Error(),
	}
}

// CreateStreamingCard creates a streaming card message.
func (a *SlackAdapter) CreateStreamingCard(ctx context.Context, chatID string) (cardID string, err error) {
	return "", ErrNotImplemented
}

// UpdateStreamingCard updates a streaming card message.
func (a *SlackAdapter) UpdateStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	return ErrNotImplemented
}

// FinalizeStreamingCard finalizes a streaming card message.
func (a *SlackAdapter) FinalizeStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	return ErrNotImplemented
}

// CheckHealth checks the health of the Slack adapter.
func (a *SlackAdapter) CheckHealth(ctx context.Context) error {
	return ErrNotImplemented
}

// MaxMessageLength returns the maximum message length for Slack.
func (a *SlackAdapter) MaxMessageLength() int {
	return 4000
}

// Compile-time check to ensure SlackAdapter implements IMAdapter.
var _ IMAdapter = (*SlackAdapter)(nil)
