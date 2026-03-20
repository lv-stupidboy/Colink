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
		"--include-partial-messages", // 启用真正的流式输出（增量 chunks）
		"--dangerously-skip-permissions", // 跳过权限检查，允许 Agent 完全访问项目目录
		"--no-session-persistence", // 禁用 CLI 会话持久化，由 ISDP 管理记忆（避免多 Agent 间记忆干扰）
	}

	if req.BaseAgent != nil && req.BaseAgent.DefaultModel != "" {
		args = append(args, "--model", req.BaseAgent.DefaultModel)
	}

	logInfo("Claude: Starting execution", zap.String("workDir", req.WorkDir), zap.String("model", req.BaseAgent.DefaultModel), zap.Strings("args", args))

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
		lineCount := 0
		chunkCount := 0
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			lineCount++
			// 调试：打印每行原始输出
			if lineCount <= 5 {
				logInfo("ExecuteWithStream: received line", zap.Int("lineNum", lineCount), zap.String("line", line[:min(200, len(line))]))
			}
			chunks := a.parseStreamJSONLine(line, onChunk != nil)
			for _, chunk := range chunks {
				if onChunk != nil {
					chunkCount++
					logInfo("ExecuteWithStream: calling onChunk", zap.Int("chunkNum", chunkCount), zap.String("type", string(chunk.Type)))
					onChunk(chunk)
				}
			}
		}
		logInfo("ExecuteWithStream: stdout scan complete", zap.Int("lines", lineCount), zap.Int("chunks", chunkCount))
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

// parseStreamJSONLine 解析 stream-json 格式的单行输出，返回 Chunk 数组
// isStreaming: 是否为增量模式，增量模式下忽略完整消息避免重复
func (a *ClaudeAdapter) parseStreamJSONLine(line string, isStreaming bool) []Chunk {
	var chunks []Chunk

	var msg struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Event   struct {
			Type         string `json:"type"`
			Index        int    `json:"index"`
			Delta        struct {
				Type        string                 `json:"type"`
				Text        string                 `json:"text"`
				Thinking    string                 `json:"thinking"`
				PartialJSON string                 `json:"partial_json"`
			} `json:"delta"`
			ContentBlock struct {
				Type  string                 `json:"type"`
				Name  string                 `json:"name"`
				ID    string                 `json:"id"`
				Input map[string]interface{} `json:"input"`
			} `json:"content_block"`
		} `json:"event"`
		Message struct {
			Content []struct {
				Type   string                 `json:"type"`
				Text   string                 `json:"text"`
				Name   string                 `json:"name"`
				ID     string                 `json:"id"`
				Input  map[string]interface{} `json:"input"`
			} `json:"content"`
		} `json:"message"`
		Result string `json:"result"`
	}

	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		logInfo("parseStreamJSONLine: JSON parse error", zap.Error(err), zap.String("line", line[:min(100, len(line))]))
		return nil
	}

	switch msg.Type {
	case "stream_event":
		switch msg.Event.Type {
		case "content_block_start":
			// 内容块开始
			switch msg.Event.ContentBlock.Type {
			case "thinking":
				chunks = append(chunks, Chunk{
					Type:    ChunkTypeThinking,
					Content: "思考中...",
				})
			case "tool_use":
				chunks = append(chunks, Chunk{
					Type:      ChunkTypeToolUse,
					ToolName:  msg.Event.ContentBlock.Name,
					ToolID:    msg.Event.ContentBlock.ID,
					ToolInput: msg.Event.ContentBlock.Input,
				})
			}
		case "content_block_delta":
			switch msg.Event.Delta.Type {
			case "text_delta":
				if msg.Event.Delta.Text != "" {
					chunks = append(chunks, Chunk{
						Type:    ChunkTypeText,
						Content: msg.Event.Delta.Text,
					})
				}
			case "thinking_delta":
				// 思考过程增量（可选：可以累积并显示）
				// 暂不返回，避免干扰主要输出
			}
		}
	case "assistant":
		// 完整消息（非增量模式下的输出）
		// 在增量模式下忽略，避免重复（内容已通过 stream_event.content_block_delta 发送）
		if !isStreaming {
			for _, content := range msg.Message.Content {
				if content.Type == "text" && content.Text != "" {
					chunks = append(chunks, Chunk{
						Type:    ChunkTypeText,
						Content: content.Text,
					})
				} else if content.Type == "tool_use" {
					chunks = append(chunks, Chunk{
						Type:      ChunkTypeToolUse,
						ToolName:  content.Name,
						ToolID:    content.ID,
						ToolInput: content.Input,
					})
				}
			}
		}
	case "user":
		// 用户消息（工具结果）
		// 这里包含工具执行结果，可以用来更新进度
	case "result":
		// 最终结果（非增量模式下使用）
		// 在增量模式下忽略，避免重复
		if !isStreaming && msg.Result != "" {
			chunks = append(chunks, Chunk{
				Type:    ChunkTypeText,
				Content: msg.Result,
			})
		}
	}

	return chunks
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
