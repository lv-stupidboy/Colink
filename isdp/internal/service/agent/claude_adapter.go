package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
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

	// 使用 stream-json 模式实现真正的流式输出
	// 注意: --verbose 是 --output-format stream-json 必需的
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--verbose",
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

	// 使用管道获取stdout实现真正的流式输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// 使用WaitGroup等待goroutine完成
	var wg sync.WaitGroup
	var stderrOutput strings.Builder
	var streamErr error

	// 读取stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			stderrOutput.WriteString(scanner.Text())
			stderrOutput.WriteString("\n")
		}
	}()

	// 读取stdout并解析 stream-json 格式
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		// 设置更大的缓冲区，因为单行可能很长
		scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			// 解析 stream-json 格式
			chunk, err := parseStreamJSONLine(line, onChunk)
			if err != nil {
				// 如果解析失败，直接发送原始内容
				if onChunk != nil {
					onChunk(line)
				}
			}
			_ = chunk // chunk 已在 parseStreamJSONLine 中通过 onChunk 发送
		}
		if err := scanner.Err(); err != nil {
			streamErr = err
		}
	}()

	// 等待所有输出读取完成
	wg.Wait()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		if stderrOutput.Len() > 0 {
			return fmt.Errorf("CLI error: %s", stderrOutput.String())
		}
		return fmt.Errorf("CLI execution failed: %w", err)
	}

	if streamErr != nil {
		return fmt.Errorf("stream read error: %w", streamErr)
	}

	return nil
}

// parseStreamJSONLine 解析 stream-json 格式的单行输出
// stream-json 格式每行是一个 JSON 对象
// 实际格式示例: {"type":"stream_event","event":{"delta":{"type":"text_delta","text":"Hello!"},"type":"content_block_delta","index":1}}
func parseStreamJSONLine(line string, onChunk func(string)) (string, error) {
	// 定义完整的消息结构
	var msg struct {
		Type    string `json:"type"`
		Event   struct {
			Type  string `json:"type"`
			Delta struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"delta"`
			ContentBlock struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content_block"`
		} `json:"event"`
		// 用于 result 类型
		Result   string `json:"result"`
		SubType  string `json:"subtype"`
	}

	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return "", err
	}

	var text string

	switch msg.Type {
	case "stream_event":
		// 流式事件，检查事件类型
		switch msg.Event.Type {
		case "content_block_delta":
			// 文本增量
			if msg.Event.Delta.Type == "text_delta" {
				text = msg.Event.Delta.Text
			} else if msg.Event.Delta.Type == "thinking_delta" {
				// 思考过程，可以选择性输出或忽略
				// 暂时也输出，方便调试
				text = msg.Event.Delta.Thinking
			}
		case "content_block_start":
			// 内容块开始，通常没有文本
		case "content_block_stop":
			// 内容块结束
		case "message_start", "message_stop", "message_delta":
			// 消息级别事件，通常没有文本
		}
	case "result":
		// 最终结果
		text = msg.Result
	case "assistant":
		// 完整的助手消息，通常包含完整内容
		// 这种情况一般是流式结束后才会出现
	case "system":
		// 系统初始化消息，忽略
	default:
		// 其他类型，尝试提取文本
	}

	// 如果有文本且不是空的，调用回调
	if text != "" && onChunk != nil {
		onChunk(text)
	}

	return text, nil
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