package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
)

// ClaudeAdapter Claude CLI适配器
type ClaudeAdapter struct {
	cliPath    string
	maxRetries int
	timeout    time.Duration
}

// NewClaudeAdapter 创建Claude适配器
func NewClaudeAdapter(cliPath string) *ClaudeAdapter {
	if cliPath == "" {
		cliPath = "claude" // 默认使用PATH中的claude
	}
	return &ClaudeAdapter{
		cliPath:    cliPath,
		maxRetries: 3,
		timeout:    30 * time.Minute,
	}
}

// Execute 执行Claude CLI
func (a *ClaudeAdapter) Execute(ctx context.Context, config *model.AgentConfig, layers *ContextLayers, input string) (string, error) {
	// 构建提示词
	prompt := a.buildPrompt(config, layers, input)

	// 准备命令参数
	args := []string{
		"--print",
		"--model", config.ModelName,
	}

	if config.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", config.MaxTokens))
	}

	// 执行CLI
	output, err := a.runCLI(ctx, args, prompt)
	if err != nil {
		return "", fmt.Errorf("claude CLI execution failed: %w", err)
	}

	return output, nil
}

// ExecuteWithStream 流式执行
func (a *ClaudeAdapter) ExecuteWithStream(ctx context.Context, config *model.AgentConfig, layers *ContextLayers, input string, onChunk func(string)) error {
	prompt := a.buildPrompt(config, layers, input)

	args := []string{
		"--stream-json",
		"--model", config.ModelName,
	}

	if config.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", config.MaxTokens))
	}

	cmd := exec.CommandContext(ctx, a.cliPath, args...)
	cmd.Stdin = strings.NewReader(prompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		var chunk StreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if chunk.Content != "" {
			onChunk(chunk.Content)
		}
	}

	return cmd.Wait()
}

// buildPrompt 构建提示词
func (a *ClaudeAdapter) buildPrompt(config *model.AgentConfig, layers *ContextLayers, input string) string {
	var sb strings.Builder

	// Layer 0: 系统提示
	sb.WriteString("<system>\n")
	sb.WriteString(layers.Layer0)
	sb.WriteString("\n</system>\n\n")

	// Layer 1: Thread历史
	if layers.Layer1 != "" {
		sb.WriteString("<conversation>\n")
		sb.WriteString(layers.Layer1)
		sb.WriteString("\n</conversation>\n\n")
	}

	// Layer 2: 工作产物
	if layers.Layer2 != "" {
		sb.WriteString("<artifacts>\n")
		sb.WriteString(layers.Layer2)
		sb.WriteString("\n</artifacts>\n\n")
	}

	// Layer 3: 环境信息
	if layers.Layer3 != "" {
		sb.WriteString("<environment>\n")
		sb.WriteString(layers.Layer3)
		sb.WriteString("\n</environment>\n\n")
	}

	// 用户输入
	sb.WriteString("<user>\n")
	sb.WriteString(input)
	sb.WriteString("\n</user>\n")

	return sb.String()
}

// runCLI 运行CLI
func (a *ClaudeAdapter) runCLI(ctx context.Context, args []string, input string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.cliPath, args...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "CLAUDE_NO_INTERACTIVE=1")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("CLI error (exit %d): %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", err
	}

	return string(output), nil
}

// StreamChunk 流式响应块
type StreamChunk struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

// GetAvailableModels 获取可用模型列表
func (a *ClaudeAdapter) GetAvailableModels(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, a.cliPath, "--list-models")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var models []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			models = append(models, line)
		}
	}
	return models, nil
}

// CheckHealth 检查CLI健康状态
func (a *ClaudeAdapter) CheckHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.cliPath, "--version")
	return cmd.Run()
}