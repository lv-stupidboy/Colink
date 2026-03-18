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

// OpenCodeAdapter OpenCode CLI适配器
type OpenCodeAdapter struct {
	cliPath     string
	apiURL      string
	apiToken    string
	gitBashPath string // Windows下git-bash路径
	maxRetries  int
	timeout     time.Duration
	baseAgent   *model.BaseAgent

	// Session management
	sessions map[string]*openCodeSession
	mu       sync.RWMutex
}

// openCodeSession OpenCode会话
type openCodeSession struct {
	id         string
	sessionKey string // 从CLI输出提取的sessionID
	cmd        *exec.Cmd        // Reserved for future process management
	ctx        context.Context  // Reserved for future process management
	cancel     context.CancelFunc // Reserved for future process management
	status     SessionStatus
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
		cliPath:     cliPath,
		apiURL:      baseAgent.ApiURL,
		apiToken:    baseAgent.ApiToken,
		gitBashPath: baseAgent.GitBashPath,
		maxRetries:  3,
		timeout:     timeout,
		baseAgent:   baseAgent,
		sessions:    make(map[string]*openCodeSession),
	}
}

// Execute 执行单次任务（无会话上下文）
func (a *OpenCodeAdapter) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	result, err := a.ExecuteWithStream(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ExecuteWithStream 流式执行
func (a *OpenCodeAdapter) ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) (*ExecutionResult, error) {
	prompt := a.buildPromptFromRequest(req)

	// 从 BaseAgent 获取模型名称 - 必须指定
	modelName := ""
	if req.BaseAgent != nil && req.BaseAgent.DefaultModel != "" {
		modelName = req.BaseAgent.DefaultModel
	}
	if modelName == "" {
		return nil, fmt.Errorf("model name is required for OpenCode adapter, please configure it in BaseAgent")
	}

	// OpenCode CLI 参数:
	// - 使用 --format json 获取结构化输出
	// - 使用 --session 恢复之前的会话，保持上下文
	// - 消息作为位置参数传递
	args := []string{
		"run",
		"--model", modelName,
		"--format", "json",
	}

	// 会话恢复逻辑：
	// - 首次启动：不传 --session，让 OpenCode 自己创建会话，然后从输出中提取 sessionID
	// - 后续消息：只使用 --session 恢复之前的会话，保持对话上下文
	sessionKey := req.SessionKey
	if sessionKey != "" {
		args = append(args, "--session", sessionKey)
		logInfo("OpenCode: Resuming session with --session", zap.String("sessionKey", sessionKey))
	} else {
		logInfo("OpenCode: Starting new session - will extract sessionKey from output")
	}

	// 构建完整命令字符串（用于日志）
	fullCommand := fmt.Sprintf("%s %s", a.cliPath, strings.Join(args, " "))
	if req.WorkDir != "" {
		fullCommand = fmt.Sprintf("cd %s && %s", req.WorkDir, fullCommand)
	}

	// 记录完整命令到日志文件
	logInfo("OpenCode CLI Full Command",
		zap.String("command", fullCommand),
		zap.String("cliPath", a.cliPath),
		zap.String("workDir", req.WorkDir),
		zap.String("sessionKey", sessionKey))

	cmd := exec.CommandContext(ctx, a.cliPath, args...)

	// 通过 Stdin 传递 prompt，避免命令行参数丢失换行符的问题
	cmd.Stdin = strings.NewReader(prompt)

	// 设置工作目录
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	// 设置环境变量
	env := a.buildEnv()
	cmd.Env = env

	// 获取 stdout 和 stderr 管道
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

	logInfo("OpenCode process started", zap.Int("pid", cmd.Process.Pid))

	var wg sync.WaitGroup
	var stderrOutput strings.Builder

	// 读取 stderr - 使用 WaitGroup 确保goroutine完成
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrOutput.WriteString(line)
			stderrOutput.WriteString("\n")
			logDebug("OpenCode stderr", zap.String("line", line))
		}
	}()

	// 用于保存提取的 sessionKey
	extractedSessionKey := sessionKey
	var totalLines int // 用于统计总行数

	// 读取 stdout 并解析 JSON 格式 - 使用 WaitGroup 确保goroutine完成
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
		lineCount := 0
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			lineCount++

			// 记录原始输出
			linePreview := line
			if len(linePreview) > 500 {
				linePreview = linePreview[:500] + "..."
			}
			logDebug("OpenCode stdout line", zap.Int("lineNum", lineCount), zap.String("line", linePreview))

			// 提取 sessionKey（首次启动时）
			if extractedSessionKey == "" {
				extractedSessionKey = a.extractSessionIDFromJSON(line)
				if extractedSessionKey != "" {
					logInfo("OpenCode: Extracted sessionKey from output", zap.String("sessionKey", extractedSessionKey))
				}
			}

			// 解析 JSON 格式输出
			text := a.extractTextFromJSON(line)
			if text != "" {
				textPreview := text
				if len(textPreview) > 200 {
					textPreview = textPreview[:200] + "..."
				}
				logDebug("OpenCode extracted text", zap.String("text", textPreview))
				if onChunk != nil {
					onChunk(Chunk{Type: ChunkTypeText, Content: text})
				}
			} else {
				logDebug("OpenCode: no text extracted from line")
			}
		}

		if err := scanner.Err(); err != nil {
			logError("OpenCode stdout scanner error", zap.Error(err))
		}
		totalLines = lineCount
	}()

	// 等待所有 goroutine 完成
	wg.Wait()

	// 在 wg.Wait() 之后调用 cmd.Wait() 清理进程资源
	if err := cmd.Wait(); err != nil {
		if stderrOutput.Len() > 0 {
			logError("OpenCode CLI error", zap.String("stderr", stderrOutput.String()))
			return nil, fmt.Errorf("CLI error: %s", stderrOutput.String())
		}
		return nil, fmt.Errorf("CLI execution failed: %w", err)
	}

	logInfo("OpenCode process completed",
		zap.Int("totalLines", totalLines),
		zap.String("sessionKey", extractedSessionKey))

	return &ExecutionResult{SessionKey: extractedSessionKey}, nil
}

// StartSession 启动交互式会话
func (a *OpenCodeAdapter) StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := &openCodeSession{
		id:     sessionID,
		status: SessionStatusRunning,
	}

	// 首次启动使用 ExecuteWithStream
	result, err := a.ExecuteWithStream(ctx, req, nil)
	if err != nil {
		session.status = SessionStatusFailed
		return err
	}

	session.sessionKey = result.SessionKey
	a.sessions[sessionID] = session

	return nil
}

// ResumeSession 恢复会话
func (a *OpenCodeAdapter) ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error {
	a.mu.RLock()
	session, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	req := &ExecutionRequest{
		SessionKey: session.sessionKey,
		Input:      input,
		BaseAgent:  a.baseAgent,
	}

	_, err := a.ExecuteWithStream(ctx, req, onChunk)
	return err
}

// StopSession 停止会话
func (a *OpenCodeAdapter) StopSession(sessionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return nil
	}

	session.status = SessionStatusStopped

	// 进程管理 - 与 ClaudeAdapter 保持一致
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
func (a *OpenCodeAdapter) GetSessionStatus(sessionID string) SessionStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return SessionStatusStopped
	}
	return session.status
}

// buildPromptFromRequest 从ExecutionRequest构建提示词
// OpenCode 使用 --session 时会自己管理会话上下文
// 首次调用：只传系统提示 + 用户输入
// 后续调用：只传用户输入
func (a *OpenCodeAdapter) buildPromptFromRequest(req *ExecutionRequest) string {
	// 如果是恢复会话，只返回用户输入
	if req.SessionKey != "" {
		return req.Input
	}

	// 首次调用：系统提示 + 用户输入
	var sb strings.Builder

	// Layer 0: 系统提示
	if req.Context != nil && req.Context.Layer0 != "" {
		sb.WriteString(req.Context.Layer0)
		sb.WriteString("\n\n")
	}

	// 用户输入
	sb.WriteString(req.Input)

	return sb.String()
}

// buildEnv 构建环境变量
func (a *OpenCodeAdapter) buildEnv() []string {
	env := os.Environ()
	if a.apiURL != "" {
		env = append(env, fmt.Sprintf("OPENCODE_API_URL=%s", a.apiURL))
	}
	if a.apiToken != "" {
		env = append(env, fmt.Sprintf("OPENCODE_API_KEY=%s", a.apiToken))
	}
	if a.gitBashPath != "" {
		env = append(env, fmt.Sprintf("OPENCODE_GIT_BASH_PATH=%s", a.gitBashPath))
	}
	return env
}

// OpenCodeJSONChunk OpenCode JSON 响应块
type OpenCodeJSONChunk struct {
	Type    string        `json:"type"`
	Content string        `json:"content"`
	Delta   string        `json:"delta"`
	Text    string        `json:"text"`
	Done    bool          `json:"done"`
	Error   string        `json:"error"`
	Part    *OpenCodePart `json:"part"`
}

// OpenCodePart OpenCode part 结构
type OpenCodePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// extractTextFromJSON 从 OpenCode JSON 输出中提取文本内容
func (a *OpenCodeAdapter) extractTextFromJSON(line string) string {
	var chunk OpenCodeJSONChunk
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		// 非 JSON，直接返回原文
		return line
	}

	// 处理错误
	if chunk.Error != "" {
		return fmt.Sprintf("ERROR: %s", chunk.Error)
	}

	// OpenCode 实际格式：文本在 part.text 中
	if chunk.Part != nil && chunk.Part.Text != "" {
		return chunk.Part.Text
	}

	// 优先返回 delta（增量文本）
	if chunk.Delta != "" {
		return chunk.Delta
	}

	// 其次返回 content
	if chunk.Content != "" {
		return chunk.Content
	}

	// 最后返回 text
	if chunk.Text != "" {
		return chunk.Text
	}

	return ""
}

// extractSessionIDFromJSON 从 OpenCode JSON 输出中提取 sessionID
func (a *OpenCodeAdapter) extractSessionIDFromJSON(line string) string {
	var data struct {
		SessionID string `json:"sessionID"`
	}

	err := json.Unmarshal([]byte(line), &data)
	result := ""

	linePreview := func() string {
		if len(line) > 200 {
			return line[:200]
		}
		return line
	}()

	if err != nil {
		logDebug("extractSessionIDFromJSON: JSON parse error",
			zap.Error(err),
			zap.String("linePreview", linePreview))
	} else if data.SessionID != "" {
		result = data.SessionID
		logInfo("extractSessionIDFromJSON: SUCCESS - extracted sessionID",
			zap.String("sessionID", result))
	} else {
		logDebug("extractSessionIDFromJSON: JSON parsed but sessionID field is empty",
			zap.String("linePreview", linePreview))
	}

	return result
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
		return nil, fmt.Errorf("failed to get available models: %w", err)
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