// Package im provides Feishu (Lark) IM integration
package im

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	pkgexec "github.com/anthropic/isdp/pkg/exec"
	"go.uber.org/zap"
)

// LarkCLIClient wraps lark-cli external process for Feishu IM operations.
// It uses lark-cli shortcuts for basic messaging and the generic `api` command
// for Cardkit v1 streaming card operations.
type LarkCLIClient struct {
	cliPath string
	timeout time.Duration
	logger  *zap.Logger
}

// NewLarkCLIClient creates a new Lark CLI client.
func NewLarkCLIClient(cliPath string, logger *zap.Logger) *LarkCLIClient {
	return &LarkCLIClient{
		cliPath: cliPath,
		timeout: 30 * time.Second,
		logger:  logger,
	}
}

// SendTextMessage sends a plain text message.
func (c *LarkCLIClient) SendTextMessage(ctx context.Context, chatID, text string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := pkgexec.CommandContext(ctx, c.cliPath, "im", "+messages-send", "--chat-id", chatID, "--text", text)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to send text message",
			zap.String("chatID", chatID),
			zap.String("stderr", stderr.String()),
			zap.Error(err))
		return fmt.Errorf("failed to send text message: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

// SendCardMessage sends an interactive card message using the old-style inline card JSON.
// For streaming cards, use CreateStreamingCardEntity + SendCardEntityMessage instead.
func (c *LarkCLIClient) SendCardMessage(ctx context.Context, chatID, cardJSON string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := pkgexec.CommandContext(ctx, c.cliPath, "im", "+messages-send",
		"--chat-id", chatID,
		"--msg-type", "interactive",
		"--content", cardJSON)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to send card message",
			zap.String("chatID", chatID),
			zap.String("stderr", stderr.String()),
			zap.Error(err))
		return fmt.Errorf("failed to send card message: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

// ReplyMessage replies to a message (thread reply).
func (c *LarkCLIClient) ReplyMessage(ctx context.Context, chatID, messageID, text string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := pkgexec.CommandContext(ctx, c.cliPath, "im", "+messages-send",
		"--chat-id", chatID,
		"--reply-in-thread", messageID,
		"--text", text)

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to reply message",
			zap.String("chatID", chatID),
			zap.String("messageID", messageID),
			zap.Error(err))
		return fmt.Errorf("failed to reply message: %w", err)
	}

	return nil
}

// CheckHealth checks if lark-cli is available.
func (c *LarkCLIClient) CheckHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := pkgexec.CommandContext(ctx, c.cliPath, "--version")

	if err := cmd.Run(); err != nil {
		c.logger.Error("lark-cli not available",
			zap.String("cliPath", c.cliPath),
			zap.Error(err))
		return fmt.Errorf("lark-cli not installed or not in PATH: %w", err)
	}

	return nil
}

// --- Cardkit v1 Streaming Card API (via lark-cli api) ---

// cardkitCreateResponse is the response from POST /open-apis/cardkit/v1/cards.
type cardkitCreateResponse struct {
	Code int `json:"code"`
	Data struct {
		CardID string `json:"card_id"`
	} `json:"data"`
	Msg string `json:"msg"`
}

// imMessageResponse is the response from POST /open-apis/im/v1/messages.
type imMessageResponse struct {
	Code int `json:"code"`
	Data struct {
		MessageID string `json:"message_id"`
		ChatID    string `json:"chat_id"`
	} `json:"data"`
	Msg string `json:"msg"`
}

// cardkitGenericResponse is the generic response from cardkit API calls.
type cardkitGenericResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// CreateStreamingCardEntity creates a Cardkit v1 card entity with streaming enabled.
// Returns card_id for subsequent streaming operations.
//
// Flow: Create entity → Send as message → Enable streaming → Update elements → Disable streaming
func (c *LarkCLIClient) CreateStreamingCardEntity(ctx context.Context, title string, elementID string) (cardID string, err error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Build card JSON 2.0 with streaming config
	cardData := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{
			"title": map[string]string{
				"content": title,
				"tag":     "plain_text",
			},
		},
		"config": map[string]interface{}{
			"streaming_mode": true,
			"streaming_config": map[string]interface{}{
				"print_frequency_ms": map[string]int{"default": 70},
				"print_step":         map[string]int{"default": 1},
				"print_strategy":     "fast",
			},
		},
		"body": map[string]interface{}{
			"elements": []map[string]string{
				{
					"tag":        "markdown",
					"content":    "",
					"element_id": elementID,
				},
			},
		},
	}

	cardJSONBytes, err := json.Marshal(cardData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal card data: %w", err)
	}

	// The `data` field must be a JSON-escaped string
	reqBody := map[string]string{
		"type": "card_json",
		"data": string(cardJSONBytes),
	}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	cmd := pkgexec.CommandContext(ctx, c.cliPath, "api",
		"POST", "/open-apis/cardkit/v1/cards",
		"--data", string(reqJSON))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	c.logger.Debug("CreateStreamingCardEntity exec",
		zap.String("cmd", c.cliPath),
		zap.Int("dataLen", len(reqJSON)))

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to create streaming card entity",
			zap.String("title", title),
			zap.String("stderr", stderr.String()),
			zap.String("stdout", stdout.String()),
			zap.Error(err))
		return "", fmt.Errorf("failed to create streaming card entity: %w (stderr: %s)", err, stderr.String())
	}

	var resp cardkitCreateResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		c.logger.Error("failed to parse card entity response",
			zap.String("output", stdout.String()),
			zap.Error(err))
		return "", fmt.Errorf("failed to parse card entity response: %w", err)
	}

	if resp.Code != 0 {
		return "", fmt.Errorf("card entity creation failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return resp.Data.CardID, nil
}

// SendCardEntityMessage sends a previously created card entity as a message.
// Returns the message_id.
func (c *LarkCLIClient) SendCardEntityMessage(ctx context.Context, chatID, cardID string) (messageID string, err error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Build content JSON: {"type":"card","data":{"card_id":"..."}}
	contentMap := map[string]interface{}{
		"type": "card",
		"data": map[string]string{
			"card_id": cardID,
		},
	}
	contentJSON, err := json.Marshal(contentMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal content: %w", err)
	}

	reqBody := map[string]string{
		"receive_id": chatID,
		"msg_type":   "interactive",
		"content":    string(contentJSON),
	}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	cmd := pkgexec.CommandContext(ctx, c.cliPath, "api",
		"POST", "/open-apis/im/v1/messages",
		"--params", `{"receive_id_type":"chat_id"}`,
		"--data", string(reqJSON))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to send card entity message",
			zap.String("chatID", chatID),
			zap.String("cardID", cardID),
			zap.String("stderr", stderr.String()),
			zap.String("stdout", stdout.String()),
			zap.Error(err))
		return "", fmt.Errorf("failed to send card entity message: %w (stderr: %s)", err, stderr.String())
	}

	var resp imMessageResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		c.logger.Error("failed to parse send card message response",
			zap.String("output", stdout.String()),
			zap.Error(err))
		return "", fmt.Errorf("failed to parse send card message response: %w", err)
	}

	if resp.Code != 0 {
		return "", fmt.Errorf("send card message failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return resp.Data.MessageID, nil
}

// UpdateStreamingElement updates the content of a streaming card element.
// The content parameter should be the FULL text (not incremental); the platform
// calculates the delta and renders the typewriter effect.
func (c *LarkCLIClient) UpdateStreamingElement(ctx context.Context, cardID, elementID, content string, sequence int) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	reqBody := map[string]interface{}{
		"content":  content,
		"sequence": sequence,
	}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	path := fmt.Sprintf("/open-apis/cardkit/v1/cards/%s/elements/%s/content", cardID, elementID)
	cmd := pkgexec.CommandContext(ctx, c.cliPath, "api",
		"PUT", path,
		"--data", string(reqJSON))

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to update streaming element",
			zap.String("cardID", cardID),
			zap.String("elementID", elementID),
			zap.Int("sequence", sequence),
			zap.Error(err))
		return fmt.Errorf("failed to update streaming element: %w", err)
	}

	var resp cardkitGenericResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return fmt.Errorf("failed to parse update response: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("update streaming element failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// SetCardStreamingMode enables or disables streaming mode on a card entity.
func (c *LarkCLIClient) SetCardStreamingMode(ctx context.Context, cardID string, enabled bool, sequence int) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	streamingMode := "false"
	if enabled {
		streamingMode = "true"
	}

	// The `settings` field must be a JSON-escaped string
	settingsMap := map[string]interface{}{
		"config": map[string]interface{}{
			"streaming_mode": streamingMode == "true",
			"update_multi":   true,
		},
	}
	settingsJSON, err := json.Marshal(settingsMap)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	reqBody := map[string]interface{}{
		"settings": string(settingsJSON),
		"sequence": sequence,
	}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	path := fmt.Sprintf("/open-apis/cardkit/v1/cards/%s/settings", cardID)
	cmd := pkgexec.CommandContext(ctx, c.cliPath, "api",
		"PATCH", path,
		"--data", string(reqJSON))

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		c.logger.Error("failed to set card streaming mode",
			zap.String("cardID", cardID),
			zap.Bool("enabled", enabled),
			zap.Int("sequence", sequence),
			zap.Error(err))
		return fmt.Errorf("failed to set card streaming mode: %w", err)
	}

	var resp cardkitGenericResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return fmt.Errorf("failed to parse streaming mode response: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("set streaming mode failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return nil
}
