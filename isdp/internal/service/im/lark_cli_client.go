// Package im 提供飞书即时通讯集成功能
package im

import (
	"context"
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
