package im

import "context"

// IMAdapter defines the contract for IM platform adapters.
type IMAdapter interface {
	Platform() string // e.g., "feishu", "slack"
	SendText(ctx context.Context, chatID, text string) SendResult
	SendCard(ctx context.Context, chatID, cardJSON string) SendResult
	ReplyText(ctx context.Context, chatID, messageID, text string) SendResult
	CreateStreamingCard(ctx context.Context, chatID string) (cardID string, err error)
	UpdateStreamingCard(ctx context.Context, cardID string, content string, sequence int) error
	FinalizeStreamingCard(ctx context.Context, cardID string, content string, sequence int) error
	CheckHealth(ctx context.Context) error
	MaxMessageLength() int
}
