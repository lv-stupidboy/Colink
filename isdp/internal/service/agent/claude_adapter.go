package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
)

// ClaudeAdapter Claude CLI适配器
type ClaudeAdapter struct {
	cliPath     string
	apiURL      string
	apiToken    string
	gitBashPath string // Windows下git-bash路径
	maxRetries  int
	timeout     time.Duration
}

// NewClaudeAdapter 创建Claude适配器（旧版兼容）
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

// NewClaudeAdapterFromBaseAgent 从BaseAgent配置创建Claude适配器
func NewClaudeAdapterFromBaseAgent(baseAgent *model.BaseAgent) *ClaudeAdapter {
	cliPath := baseAgent.CliPath
	if cliPath == "" {
		cliPath = "claude"
	}

	timeout := time.Duration(baseAgent.TimeoutMinutes) * time.Minute
	if timeout == 0 {
		timeout = 30 * time.Minute
	}

	return &ClaudeAdapter{
		cliPath:     cliPath,
		apiURL:      baseAgent.ApiURL,
		apiToken:    baseAgent.ApiToken,
		gitBashPath: baseAgent.GitBashPath,
		maxRetries:  3,
		timeout:     timeout,
	}
}

// Execute 执行Claude CLI
func (a *ClaudeAdapter) Execute(ctx context.Context, config *model.AgentRoleConfig, layers *ContextLayers, input string, workDir string) (string, error) {
	// 构建提示词
	prompt := a.buildPrompt(config, layers, input)

	// 准备命令参数
	args := []string{
		"--print",
	}

	// 只有在指定了模型名称时才添加 --model 参数
	modelName := config.ModelName
	if modelName != "" {
		args = append(args, "--model", modelName)
	}

	// 注意: --max-tokens 选项在某些CLI版本中不支持，暂时移除
	// 如果需要，可以通过环境变量或其他方式配置

	// 执行CLI
	output, err := a.runCLI(ctx, args, prompt, workDir)
	if err != nil {
		return "", fmt.Errorf("claude CLI execution failed: %w", err)
	}

	return output, nil
}

// ExecuteWithStream 流式执行
func (a *ClaudeAdapter) ExecuteWithStream(ctx context.Context, config *model.AgentRoleConfig, layers *ContextLayers, input string, workDir string, onChunk func(string)) error {
	prompt := a.buildPrompt(config, layers, input)

	args := []string{
		"--print", // 使用 --print 模式，更稳定
	}

	// 只有在指定了模型名称时才添加 --model 参数
	modelName := config.ModelName
	if modelName != "" {
		args = append(args, "--model", modelName)
	}

	// 注意: --max-tokens 选项在某些CLI版本中不支持，暂时移除

	cmd := exec.CommandContext(ctx, a.cliPath, args...)
	cmd.Stdin = strings.NewReader(prompt)

	// 设置工作目录
	if workDir != "" {
		cmd.Dir = workDir
	}

	// 设置环境变量
	env := os.Environ()
	env = append(env, "CLAUDE_NO_INTERACTIVE=1")
	if a.apiURL != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_API_URL=%s", a.apiURL))
	}
	if a.apiToken != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", a.apiToken))
	}
	if a.gitBashPath != "" {
		env = append(env, fmt.Sprintf("CLAUDE_CODE_GIT_BASH_PATH=%s", a.gitBashPath))
	}
	cmd.Env = env

	// 获取输出和错误
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("CLI error (exit %d): %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return fmt.Errorf("CLI execution failed: %w", err)
	}

	// 将整个输出作为一个块发送
	if onChunk != nil {
		onChunk(string(output))
	}

	return nil
}

// buildPrompt 构建提示词
func (a *ClaudeAdapter) buildPrompt(config *model.AgentRoleConfig, layers *ContextLayers, input string) string {
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
func (a *ClaudeAdapter) runCLI(ctx context.Context, args []string, input string, workDir string) (string, error) {
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
	env = append(env, "CLAUDE_NO_INTERACTIVE=1")

	// 如果配置了API URL和Token，添加到环境变量
	if a.apiURL != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_API_URL=%s", a.apiURL))
	}
	if a.apiToken != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", a.apiToken))
	}

	// 如果配置了Git-Bash路径，添加到环境变量（Windows下Claude CLI需要）
	if a.gitBashPath != "" {
		env = append(env, fmt.Sprintf("CLAUDE_CODE_GIT_BASH_PATH=%s", a.gitBashPath))
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