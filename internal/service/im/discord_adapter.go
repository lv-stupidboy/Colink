package im

import (
	"context"
)

// DiscordAdapter implements IMAdapter for Discord platform.
type DiscordAdapter struct{}

// NewDiscordAdapter creates a new Discord adapter.
func NewDiscordAdapter() *DiscordAdapter {
	return &DiscordAdapter{}
}

// Platform returns the platform identifier.
func (a *DiscordAdapter) Platform() string {
	return "discord"
}

// SendText sends a text message to the channel.
func (a *DiscordAdapter) SendText(ctx context.Context, chatID, text string) SendResult {
	return SendResult{
		OK:    false,
		Error: ErrNotImplemented.Error(),
	}
}

// SendCard sends a card message to the channel.
func (a *DiscordAdapter) SendCard(ctx context.Context, chatID, cardJSON string) SendResult {
	return SendResult{
		OK:    false,
		Error: ErrNotImplemented.Error(),
	}
}

// ReplyText sends a reply message to a specific message.
func (a *DiscordAdapter) ReplyText(ctx context.Context, chatID, messageID, text string) SendResult {
	return SendResult{
		OK:    false,
		Error: ErrNotImplemented.Error(),
	}
}

// CreateStreamingCard creates a streaming card message.
func (a *DiscordAdapter) CreateStreamingCard(ctx context.Context, chatID string, agentName string) (cardID string, err error) {
	return "", ErrNotImplemented
}

// UpdateStreamingCard updates a streaming card message.
func (a *DiscordAdapter) UpdateStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	return ErrNotImplemented
}

// FinalizeStreamingCard finalizes a streaming card message.
func (a *DiscordAdapter) FinalizeStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	return ErrNotImplemented
}

// CheckHealth checks the health of the Discord adapter.
func (a *DiscordAdapter) CheckHealth(ctx context.Context) error {
	return ErrNotImplemented
}

// MaxMessageLength returns the maximum message length for Discord.
func (a *DiscordAdapter) MaxMessageLength() int {
	return 2000
}

// Compile-time check to ensure DiscordAdapter implements IMAdapter.
var _ IMAdapter = (*DiscordAdapter)(nil)
