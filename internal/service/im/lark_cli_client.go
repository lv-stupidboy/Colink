// Package im 提供飞书即时通讯集成功能
package im

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"go.uber.org/zap"
)

// LarkCLIClient 包装 lark-cli 外部进程
type LarkCLIClient struct {
	cliPath string
	timeout time.Duration
	logger  *zap.Logger
}

// NewLarkCLIClient 创建新的 Lark CLI 客户端
func NewLarkCLIClient(cliPath string, logger *zap.Logger) *LarkCLIClient {
	return &LarkCLIClient{
		cliPath: cliPath,
		timeout: 30 * time.Second,
		logger:  logger,
	}
}

// SendTextMessage 发送文本消息
func (c *LarkCLIClient) SendTextMessage(ctx context.Context, chatID, text string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.cliPath, "im", "messages-send", "--chat-id", chatID, "--text", text)

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to send text message",
			zap.String("chatID", chatID),
			zap.Error(err))
		return fmt.Errorf("failed to send text message: %w", err)
	}

	return nil
}

// SendCardMessage 发送卡片消息
func (c *LarkCLIClient) SendCardMessage(ctx context.Context, chatID, cardJSON string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.cliPath, "im", "messages-send", "--chat-id", chatID, "--card", cardJSON)

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to send card message",
			zap.String("chatID", chatID),
			zap.Error(err))
		return fmt.Errorf("failed to send card message: %w", err)
	}

	return nil
}

// ReplyMessage 回复消息
func (c *LarkCLIClient) ReplyMessage(ctx context.Context, chatID, messageID, text string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.cliPath, "im", "messages-send", "--chat-id", chatID, "--reply-in-thread", messageID, "--text", text)

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to reply message",
			zap.String("chatID", chatID),
			zap.String("messageID", messageID),
			zap.Error(err))
		return fmt.Errorf("failed to reply message: %w", err)
	}

	return nil
}

// CheckHealth 检查 lark-cli 是否可用
func (c *LarkCLIClient) CheckHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.cliPath, "--version")

	if err := cmd.Run(); err != nil {
		c.logger.Error("lark-cli not available",
			zap.String("cliPath", c.cliPath),
			zap.Error(err))
		return fmt.Errorf("lark-cli not installed or not in PATH: %w", err)
	}

	return nil
}

// CreateCard 创建流式卡片
func (c *LarkCLIClient) CreateCard(ctx context.Context, chatID string) (cardID, messageID string, err error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.cliPath, "im", "card-create", "--chat-id", chatID, "--output", "json")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to create card",
			zap.String("chatID", chatID),
			zap.Error(err))
		return "", "", fmt.Errorf("failed to create card: %w", err)
	}

	var result struct {
		CardID    string `json:"card_id"`
		MessageID string `json:"message_id"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		c.logger.Error("failed to parse card creation response",
			zap.String("output", stdout.String()),
			zap.Error(err))
		return "", "", fmt.Errorf("failed to parse card creation response: %w", err)
	}

	return result.CardID, result.MessageID, nil
}

// UpdateCardContent 更新卡片内容
func (c *LarkCLIClient) UpdateCardContent(ctx context.Context, cardID, content string, sequence int) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.cliPath, "im", "card-update",
		"--card-id", cardID,
		"--content", content,
		"--sequence", fmt.Sprintf("%d", sequence))

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to update card content",
			zap.String("cardID", cardID),
			zap.Int("sequence", sequence),
			zap.Error(err))
		return fmt.Errorf("failed to update card content: %w", err)
	}

	return nil
}

// SetStreamingMode 设置卡片流式模式
func (c *LarkCLIClient) SetStreamingMode(ctx context.Context, cardID string, enabled bool, sequence int) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	mode := "false"
	if enabled {
		mode = "true"
	}

	cmd := exec.CommandContext(ctx, c.cliPath, "im", "card-streaming",
		"--card-id", cardID,
		"--enabled", mode,
		"--sequence", fmt.Sprintf("%d", sequence))

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to set streaming mode",
			zap.String("cardID", cardID),
			zap.Bool("enabled", enabled),
			zap.Int("sequence", sequence),
			zap.Error(err))
		return fmt.Errorf("failed to set streaming mode: %w", err)
	}

	return nil
}
