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
	"github.com/google/uuid"
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

	// 确定会话ID：复用已有或创建新的
	var sessionID string
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",     // 启用真正的流式输出（增量 chunks）
		"--dangerously-skip-permissions", // 跳过权限检查，允许 Agent 完全访问项目目录
	}

	// 会话复用：如果提供了 SessionID，使用 --resume 复用已有会话
	// 这可以避免每次调用的冷启动延迟（约 2-3 秒）
	if req.SessionID != "" {
		sessionID = req.SessionID
		args = append(args, "--resume", sessionID)
		logInfo("Claude: Using session resume", zap.String("sessionId", sessionID))
	} else {
		// 新会话：使用 --session-id 指定会话ID，以便后续复用
		// 注意：不再使用 --no-session-persistence，让 CLI 持久化会话
		sessionID = uuid.New().String()
		args = append(args, "--session-id", sessionID)
		logInfo("Claude: Creating new session", zap.String("sessionId", sessionID))
	}

	// 添加模型参数
	if a.baseAgent != nil && a.baseAgent.DefaultModel != "" {
		args = append(args, "--model", a.baseAgent.DefaultModel)
		logDebug("Claude: using model from baseAgent", zap.String("model", a.baseAgent.DefaultModel))
	} else {
		logInfo("Claude: WARNING - no model specified", zap.Bool("hasBaseAgent", a.baseAgent != nil), zap.String("defaultModel", a.baseAgent.DefaultModel))
	}

	logDebug("Claude: Starting execution", zap.String("workDir", req.WorkDir), zap.String("configDir", req.ConfigDir))

	cmd := exec.CommandContext(ctx, a.cliPath, args...)
	cmd.Stdin = strings.NewReader(prompt)

	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	env := a.buildEnv(req)
	cmd.Env = env

	// 构建可复制的命令（方便调试）
	// 提取关键环境变量用于日志，对敏感信息脱敏
	var envVarsForLog []string
	for _, e := range env {
		if strings.HasPrefix(e, "ANTHROPIC_") || strings.HasPrefix(e, "CLAUDE_") {
			if strings.HasPrefix(e, "ANTHROPIC_AUTH_TOKEN=") {
				token := strings.TrimPrefix(e, "ANTHROPIC_AUTH_TOKEN=")
				maskedToken := maskToken(token)
				envVarsForLog = append(envVarsForLog, fmt.Sprintf("ANTHROPIC_AUTH_TOKEN=\"%s\"", maskedToken))
			} else {
				envVarsForLog = append(envVarsForLog, e)
			}
		}
	}

	// 构建可复制的完整命令
	cliCmd := a.cliPath + " " + strings.Join(args, " ")
	var cmdForCopy strings.Builder
	if cmd.Dir != "" {
		cmdForCopy.WriteString(fmt.Sprintf("cd \"%s\" && ", cmd.Dir))
	}
	for i, envLine := range envVarsForLog {
		cmdForCopy.WriteString(envLine)
		if i < len(envVarsForLog)-1 {
			cmdForCopy.WriteString(" ")
		}
	}
	if len(envVarsForLog) > 0 {
		cmdForCopy.WriteString(" ")
	}
	cmdForCopy.WriteString(cliCmd)

	logInfo("Claude: CLI command (copy to test)",
		zap.String("workDir", cmd.Dir),
		zap.Strings("envVars", envVarsForLog),
		zap.String("cliPath", a.cliPath),
		zap.Strings("args", args),
		zap.String("fullCommand", cmdForCopy.String()),
	)

	cliStartTime := time.Now()

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
	logInfo("[PERF] CLI cmd.Start", zap.Duration("duration", time.Since(cliStartTime)))

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
		firstLineReceived := false
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			lineCount++
			// 记录首行时间
			if !firstLineReceived {
				firstLineReceived = true
				logInfo("[PERF] CLI first line received", zap.Duration("duration", time.Since(cliStartTime)), zap.Int("lineNum", lineCount))
			}
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
		// 获取模型名称
		modelName := ""
		if a.baseAgent != nil {
			modelName = a.baseAgent.DefaultModel
		}
		// 详细记录执行失败信息
		logError("Claude: Execution failed",
			zap.Error(err),
			zap.String("cliPath", a.cliPath),
			zap.String("workDir", cmd.Dir),
			zap.String("configDir", req.ConfigDir),
			zap.String("stderr", stderrOutput.String()),
			zap.String("model", modelName),
		)
		if stderrOutput.Len() > 0 {
			return nil, fmt.Errorf("CLI error: %s", stderrOutput.String())
		}
		return nil, fmt.Errorf("CLI execution failed: %w", err)
	}

	logInfo("[PERF] CLI total execution", zap.Duration("duration", time.Since(cliStartTime)))
	return &ExecutionResult{SessionID: sessionID}, nil
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
			Type  string `json:"type"`
			Index int    `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				PartialJSON string `json:"partial_json"`
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
				Type       string                 `json:"type"`
				Text       string                 `json:"text"`
				Name       string                 `json:"name"`
				ID         string                 `json:"id"`
				Input      map[string]interface{} `json:"input"`
				ToolUseID  string                 `json:"tool_use_id"` // tool_result 的关联ID
				ContentStr string                 `json:"content"`     // tool_result 的内容（可能是 string 或 array）
				IsError    bool                   `json:"is_error"`    // tool_result 是否出错
			} `json:"content"`
			Usage *struct {
				InputTokens              int64 `json:"input_tokens"`
				OutputTokens             int64 `json:"output_tokens"`
				CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
				CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			} `json:"usage"`
		} `json:"message"`
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
		Delta struct {
			Usage struct {
				OutputTokens int64 `json:"output_tokens"`
			} `json:"usage"`
		} `json:"delta"`
		Result        string  `json:"result"`
		CostUsd       float64 `json:"cost_usd"`
		DurationMs    int64   `json:"duration_ms"`
		DurationApiMs int64   `json:"duration_api_ms"`
		NumTurns      int     `json:"num_turns"`
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
				// 思考块开始，发送空内容让前端初始化
				chunks = append(chunks, Chunk{
					Type:    ChunkTypeThinking,
					Content: "",
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
				// 思考过程增量 - 发送实际的思考内容
				if msg.Event.Delta.Thinking != "" {
					chunks = append(chunks, Chunk{
						Type:    ChunkTypeThinking,
						Content: msg.Event.Delta.Thinking,
					})
				}
			}
		case "content_block_stop":
			// 内容块结束 - 用于标记 thinking 完成
			// 发送一个带 Done 标记的空 thinking 块
			chunks = append(chunks, Chunk{
				Type:    ChunkTypeThinking,
				Content: "",
				Done:    true,
			})
		}
	case "message_start":
		// 解析 message.usage 字段（input tokens）
		if msg.Message.Usage != nil {
			chunks = append(chunks, Chunk{
				Type: ChunkTypeUsage,
				Usage: &TokenUsage{
					InputTokens:         msg.Message.Usage.InputTokens,
					CacheReadTokens:     msg.Message.Usage.CacheReadInputTokens,
					CacheCreationTokens: msg.Message.Usage.CacheCreationInputTokens,
				},
			})
		}
	case "message_delta":
		// 解析 usage 字段（output tokens 通常在这里）
		if msg.Delta.Usage.OutputTokens > 0 {
			chunks = append(chunks, Chunk{
				Type: ChunkTypeUsage,
				Usage: &TokenUsage{
					OutputTokens: msg.Delta.Usage.OutputTokens,
				},
			})
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
		// 用户消息（包含工具执行结果）
		// 解析 tool_result 内容块
		for _, content := range msg.Message.Content {
			if content.Type == "tool_result" {
				// 提取结果内容
				resultContent := content.ContentStr
				if resultContent == "" && content.Text != "" {
					resultContent = content.Text
				}

				chunks = append(chunks, Chunk{
					Type:    ChunkTypeToolResult,
					ToolID:  content.ToolUseID,
					Content: resultContent,
					IsError: content.IsError,
				})
			}
		}
	case "result":
		// 最终结果（非增量模式下使用）
		// 在增量模式下忽略，避免重复
		if !isStreaming && msg.Result != "" {
			chunks = append(chunks, Chunk{
				Type:    ChunkTypeText,
				Content: msg.Result,
			})
		}
		// 解析完整 usage: input_tokens, output_tokens, cache_read_input_tokens
		// 解析 total_cost_usd, duration_ms, duration_api_ms, num_turns
		if msg.Usage.InputTokens > 0 || msg.Usage.OutputTokens > 0 || msg.CostUsd > 0 {
			chunks = append(chunks, Chunk{
				Type: ChunkTypeUsage,
				Usage: &TokenUsage{
					InputTokens:         msg.Usage.InputTokens,
					OutputTokens:        msg.Usage.OutputTokens,
					CacheReadTokens:     msg.Usage.CacheReadInputTokens,
					CacheCreationTokens: msg.Usage.CacheCreationInputTokens,
					CostUsd:             msg.CostUsd,
					DurationMs:          msg.DurationMs,
					DurationApiMs:       msg.DurationApiMs,
					NumTurns:            msg.NumTurns,
				},
			})
		}
	}

	return chunks
}

// buildEnv 构建环境变量
// 使用 map 去重，BaseAgent 配置的值会覆盖系统环境变量
func (a *ClaudeAdapter) buildEnv(req *ExecutionRequest) []string {
	// 用 map 存储环境变量，后面的值会覆盖前面的
	envMap := make(map[string]string)

	// 先复制系统环境变量
	for _, e := range os.Environ() {
		if idx := strings.Index(e, "="); idx > 0 {
			envMap[e[:idx]] = e[idx+1:]
		}
	}

	// 设置 BaseAgent 配置的环境变量（会覆盖系统环境变量）
	envMap["CLAUDE_NO_INTERACTIVE"] = "1"
	if a.apiURL != "" {
		envMap["ANTHROPIC_BASE_URL"] = a.apiURL
	}
	if a.apiToken != "" {
		envMap["ANTHROPIC_AUTH_TOKEN"] = a.apiToken
	}
	if a.gitBashPath != "" {
		envMap["CLAUDE_CODE_GIT_BASH_PATH"] = a.gitBashPath
	}
	if req.ConfigDir != "" {
		envMap["CLAUDE_CONFIG_DIR"] = req.ConfigDir
	}

	// 转换为 slice
	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
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

// CheckHealth 检查CLI健康状态，执行简单prompt验证API连接
// 使用与正常执行相同的参数和环境变量构建逻辑，确保一致性
func (a *ClaudeAdapter) CheckHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 使用与 ExecuteWithStream 相同的基础参数
	args := []string{
		"--print",
		"--dangerously-skip-permissions",
	}

	// 添加模型参数（与 ExecuteWithStream 保持一致）
	if a.baseAgent != nil && a.baseAgent.DefaultModel != "" {
		args = append(args, "--model", a.baseAgent.DefaultModel)
	}

	cmd := exec.CommandContext(ctx, a.cliPath, args...)

	// 构建与正常执行相同的环境变量（使用空的 ExecutionRequest，但包含基本配置）
	execReq := &ExecutionRequest{
		BaseAgent: a.baseAgent,
	}
	env := a.buildEnv(execReq)
	cmd.Env = env

	// 提取关键环境变量用于日志，对敏感信息脱敏
	var envVarsForLog []string
	for _, e := range env {
		if strings.HasPrefix(e, "ANTHROPIC_") || strings.HasPrefix(e, "CLAUDE_") {
			if strings.HasPrefix(e, "ANTHROPIC_AUTH_TOKEN=") {
				token := strings.TrimPrefix(e, "ANTHROPIC_AUTH_TOKEN=")
				maskedToken := maskToken(token)
				envVarsForLog = append(envVarsForLog, fmt.Sprintf("ANTHROPIC_AUTH_TOKEN=\"%s\"", maskedToken))
			} else {
				envVarsForLog = append(envVarsForLog, e)
			}
		}
	}

	// 构建可复制的完整命令
	cliCmd := a.cliPath + " " + strings.Join(args, " ")
	var cmdForCopy strings.Builder
	cmdForCopy.WriteString(fmt.Sprintf("cd \"%s\" && ", os.TempDir()))
	for i, envLine := range envVarsForLog {
		cmdForCopy.WriteString(envLine)
		if i < len(envVarsForLog)-1 {
			cmdForCopy.WriteString(" ")
		}
	}
	if len(envVarsForLog) > 0 {
		cmdForCopy.WriteString(" ")
	}
	cmdForCopy.WriteString(cliCmd)

	logInfo("Claude: CheckHealth command (copy to test)",
		zap.String("workDir", os.TempDir()),
		zap.Strings("envVars", envVarsForLog),
		zap.String("cliPath", a.cliPath),
		zap.Strings("args", args),
		zap.String("fullCommand", cmdForCopy.String()),
	)

	// 通过stdin传递prompt
	cmd.Stdin = strings.NewReader("reply with ok only")

	// 使用临时目录作为工作目录
	cmd.Dir = os.TempDir()

	output, err := cmd.CombinedOutput()
	if err != nil {
		modelName := ""
		if a.baseAgent != nil {
			modelName = a.baseAgent.DefaultModel
		}
		logError("Claude: Health check failed",
			zap.Error(err),
			zap.String("cliPath", a.cliPath),
			zap.String("model", modelName),
			zap.String("workDir", cmd.Dir),
			zap.String("output", string(output)),
		)
		return fmt.Errorf("claude CLI test failed: %w, output: %s", err, string(output))
	}

	// 检查输出是否包含有效响应
	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return fmt.Errorf("claude CLI returned empty response")
	}

	return nil
}
