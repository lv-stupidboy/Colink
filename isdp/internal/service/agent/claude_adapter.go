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
	"go.uber.org/zap"
)

// ClaudeAdapter Claude CLI适配器
type ClaudeAdapter struct {
	cliPath     string
	apiURL      string
	apiToken    string
	gitBashPath string
	maxRetries  int
	timeout     time.Duration
	baseAgent   *model.BaseAgent

	// Session management
	sessions map[string]*claudeSession
	mu       sync.RWMutex
}

// claudeSession Claude会话
type claudeSession struct {
	id     string
	cmd    *exec.Cmd
	ctx    context.Context
	cancel context.CancelFunc
	status SessionStatus
}

// NewClaudeAdapter 创建Claude适配器
func NewClaudeAdapter(baseAgent *model.BaseAgent) *ClaudeAdapter {
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
		baseAgent:   baseAgent,
		sessions:    make(map[string]*claudeSession),
	}
}

// Execute 执行单次任务（无会话上下文）
func (a *ClaudeAdapter) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	result, err := a.ExecuteWithStream(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ExecuteWithStream 流式执行
func (a *ClaudeAdapter) ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) (*ExecutionResult, error) {
	prompt := a.buildPromptFromRequest(req)

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--permission-mode", "auto",
		"--continue", // 统一使用 --continue 让 CLI 自动恢复上次会话
	}

	if req.BaseAgent != nil && req.BaseAgent.DefaultModel != "" {
		args = append(args, "--model", req.BaseAgent.DefaultModel)
	}

	logInfo("Claude: Starting with --continue", zap.String("workDir", req.WorkDir))

	cmd := exec.CommandContext(ctx, a.cliPath, args...)
	cmd.Stdin = strings.NewReader(prompt)

	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	env := a.buildEnv()
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	var wg sync.WaitGroup
	var stderrOutput strings.Builder

	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			stderrOutput.WriteString(scanner.Text())
			stderrOutput.WriteString("\n")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			text := a.parseStreamJSONLine(line)
			if text != "" && onChunk != nil {
				onChunk(Chunk{Type: ChunkTypeText, Content: text})
			}
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if stderrOutput.Len() > 0 {
			return nil, fmt.Errorf("CLI error: %s", stderrOutput.String())
		}
		return nil, fmt.Errorf("CLI execution failed: %w", err)
	}

	return &ExecutionResult{}, nil
}

// StartSession 启动交互式会话
func (a *ClaudeAdapter) StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := &claudeSession{
		id:     sessionID,
		status: SessionStatusRunning,
	}

	// 首次启动使用 ExecuteWithStream
	_, err := a.ExecuteWithStream(ctx, req, nil)
	if err != nil {
		session.status = SessionStatusFailed
		return err
	}

	a.sessions[sessionID] = session

	return nil
}

// ResumeSession 恢复会话
func (a *ClaudeAdapter) ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error {
	a.mu.RLock()
	_, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	req := &ExecutionRequest{
		Input:     input,
		BaseAgent: a.baseAgent,
	}

	_, err := a.ExecuteWithStream(ctx, req, onChunk)
	return err
}

// StopSession 停止会话
func (a *ClaudeAdapter) StopSession(sessionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return nil
	}

	session.status = SessionStatusStopped
	if session.cancel != nil {
		session.cancel()
	}
	if session.cmd != nil && session.cmd.Process != nil {
		session.cmd.Process.Kill()
	}

	delete(a.sessions, sessionID)
	return nil
}

// GetSessionStatus 获取会话状态
func (a *ClaudeAdapter) GetSessionStatus(sessionID string) SessionStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return SessionStatusStopped
	}
	return session.status
}

// buildPromptFromRequest 从ExecutionRequest构建提示词
func (a *ClaudeAdapter) buildPromptFromRequest(req *ExecutionRequest) string {
	var sb strings.Builder

	if req.Context != nil {
		// Layer 0: 系统提示
		if req.Context.Layer0 != "" {
			sb.WriteString("<system>\n")
			sb.WriteString(req.Context.Layer0)
			sb.WriteString("\n</system>\n\n")
		}

		// Layer 1: Thread历史
		if req.Context.Layer1 != "" {
			sb.WriteString("<conversation>\n")
			sb.WriteString(req.Context.Layer1)
			sb.WriteString("\n</conversation>\n\n")
		}

		// Layer 2: 工作产物
		if req.Context.Layer2 != "" {
			sb.WriteString("<artifacts>\n")
			sb.WriteString(req.Context.Layer2)
			sb.WriteString("\n</artifacts>\n\n")
		}

		// Layer 3: 环境信息
		if req.Context.Layer3 != "" {
			sb.WriteString("<environment>\n")
			sb.WriteString(req.Context.Layer3)
			sb.WriteString("\n</environment>\n\n")
		}
	}

	// 用户输入
	sb.WriteString("<user>\n")
	sb.WriteString(req.Input)
	sb.WriteString("\n</user>\n")

	return sb.String()
}

// parseStreamJSONLine 解析 stream-json 格式的单行输出
func (a *ClaudeAdapter) parseStreamJSONLine(line string) string {
	var msg struct {
		Type  string `json:"type"`
		Event struct {
			Type  string `json:"type"`
			Delta struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"delta"`
		} `json:"event"`
		Result string `json:"result"`
	}

	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return ""
	}

	switch msg.Type {
	case "stream_event":
		if msg.Event.Type == "content_block_delta" && msg.Event.Delta.Type == "text_delta" {
			return msg.Event.Delta.Text
		}
	case "result":
		return msg.Result
	}

	return ""
}

// buildEnv 构建环境变量
func (a *ClaudeAdapter) buildEnv() []string {
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
	return env
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
