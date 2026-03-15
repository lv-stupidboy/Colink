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

// OpenCodeAdapter OpenCode CLI适配器
type OpenCodeAdapter struct {
	cliPath    string
	apiURL     string
	apiToken   string
	maxRetries int
	timeout    time.Duration
}

// NewOpenCodeAdapter 创建OpenCode适配器
func NewOpenCodeAdapter(baseAgent *model.BaseAgent) *OpenCodeAdapter {
	cliPath := baseAgent.CliPath
	if cliPath == "" {
		cliPath = "opencode"
	}

	timeout := time.Duration(baseAgent.TimeoutMinutes) * time.Minute
	if timeout == 0 {
		timeout = 30 * time.Minute
	}

	return &OpenCodeAdapter{
		cliPath:    cliPath,
		apiURL:     baseAgent.ApiURL,
		apiToken:   baseAgent.ApiToken,
		maxRetries: 3,
		timeout:    timeout,
	}
}

// Execute 执行OpenCode CLI
func (a *OpenCodeAdapter) Execute(ctx context.Context, config *model.AgentRoleConfig, layers *ContextLayers, input string, workDir string) (string, error) {
	// 构建提示词
	prompt := a.buildPrompt(config, layers, input)

	// 准备命令参数
	modelName := config.ModelName
	if modelName == "" {
		modelName = "gpt-4"
	}

	args := []string{
		"run",
		"--model", modelName,
		"--non-interactive",
	}

	if config.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", config.MaxTokens))
	}

	// 执行CLI
	output, err := a.runCLI(ctx, args, prompt, workDir)
	if err != nil {
		return "", fmt.Errorf("opencode CLI execution failed: %w", err)
	}

	return output, nil
}

// ExecuteWithStream 流式执行
func (a *OpenCodeAdapter) ExecuteWithStream(ctx context.Context, config *model.AgentRoleConfig, layers *ContextLayers, input string, workDir string, onChunk func(string)) error {
	prompt := a.buildPrompt(config, layers, input)

	modelName := config.ModelName
	if modelName == "" {
		modelName = "gpt-4"
	}

	args := []string{
		"run",
		"--model", modelName,
		"--stream",
		"--non-interactive",
	}

	if config.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", config.MaxTokens))
	}

	cmd := exec.CommandContext(ctx, a.cliPath, args...)
	cmd.Stdin = strings.NewReader(prompt)

	// 设置工作目录
	if workDir != "" {
		cmd.Dir = workDir
	}

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
		var chunk OpenCodeStreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			// 如果不是JSON，直接作为文本处理
			onChunk(line + "\n")
			continue
		}
		if chunk.Content != "" {
			onChunk(chunk.Content)
		}
	}

	return cmd.Wait()
}

// buildPrompt 构建提示词
func (a *OpenCodeAdapter) buildPrompt(config *model.AgentRoleConfig, layers *ContextLayers, input string) string {
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
func (a *OpenCodeAdapter) runCLI(ctx context.Context, args []string, input string, workDir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.cliPath, args...)
	cmd.Stdin = strings.NewReader(input)

	// 设置工作目录
	if workDir != "" {
		cmd.Dir = workDir
	}

	// 设置环境变量
	env := os.Environ()

	// 如果配置了API URL和Token，添加到环境变量
	if a.apiURL != "" {
		env = append(env, fmt.Sprintf("OPENCODE_API_URL=%s", a.apiURL))
	}
	if a.apiToken != "" {
		env = append(env, fmt.Sprintf("OPENCODE_API_KEY=%s", a.apiToken))
	}

	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("CLI error (exit %d): %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", err
	}

	return string(output), nil
}

// OpenCodeStreamChunk OpenCode流式响应块
type OpenCodeStreamChunk struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

// CheckHealth 检查CLI健康状态
func (a *OpenCodeAdapter) CheckHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.cliPath, "--version")
	return cmd.Run()
}

// GetAvailableModels 获取可用模型列表
func (a *OpenCodeAdapter) GetAvailableModels(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, a.cliPath, "models", "--list")
	output, err := cmd.Output()
	if err != nil {
		// 如果命令不存在，返回默认模型列表
		return []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo", "claude-3-opus", "claude-3-sonnet"}, nil
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