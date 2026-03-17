package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var sessionLogger *zap.Logger

// SetSessionLogger 设置会话日志记录器
func SetSessionLogger(logger *zap.Logger) {
	sessionLogger = logger
}

// logDebug 调试日志
func logDebug(msg string, fields ...zap.Field) {
	if sessionLogger != nil {
		sessionLogger.Debug(msg, fields...)
	} else {
		fmt.Printf("[DEBUG] %s %v\n", msg, fields)
	}
}

// logInfo 信息日志
func logInfo(msg string, fields ...zap.Field) {
	if sessionLogger != nil {
		sessionLogger.Info(msg, fields...)
	} else {
		fmt.Printf("[INFO] %s %v\n", msg, fields)
	}
}

// logError 错误日志
func logError(msg string, fields ...zap.Field) {
	if sessionLogger != nil {
		sessionLogger.Error(msg, fields...)
	} else {
		fmt.Printf("[ERROR] %s %v\n", msg, fields)
	}
}

// InteractiveSession 交互式会话
type InteractiveSession struct {
	ID          uuid.UUID
	ThreadID    uuid.UUID
	AgentConfig *model.AgentRoleConfig
	BaseAgent   *model.BaseAgent
	WorkDir     string
	SessionID   string // Claude CLI 的 session ID

	cmd    *exec.Cmd
	stdout io.Reader
	ctx    context.Context
	cancel context.CancelFunc
	wsHub  *ws.Hub

	mu      sync.Mutex
	running bool
}

// InteractiveSessionManager 交互式会话管理器
type InteractiveSessionManager struct {
	sessions map[uuid.UUID]*InteractiveSession
	mu       sync.RWMutex
	wsHub    *ws.Hub
}

// NewInteractiveSessionManager 创建会话管理器
func NewInteractiveSessionManager(wsHub *ws.Hub) *InteractiveSessionManager {
	return &InteractiveSessionManager{
		sessions: make(map[uuid.UUID]*InteractiveSession),
		wsHub:    wsHub,
	}
}

// StartSession 启动交互式会话
func (m *InteractiveSessionManager) StartSession(
	ctx context.Context,
	threadID uuid.UUID,
	config *model.AgentRoleConfig,
	baseAgent *model.BaseAgent,
	workDir string,
	initialInput string,
) (*InteractiveSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	logInfo("StartSession called",
		zap.String("threadId", threadID.String()),
		zap.String("configName", config.Name),
		zap.String("workDir", workDir),
		zap.String("initialInput", func(a, b int) string {
			if a < b {
				return initialInput[:a]
			}
			return initialInput[:b]
		}(100, len(initialInput))))

	session := &InteractiveSession{
		ID:          uuid.New(),
		ThreadID:    threadID,
		AgentConfig: config,
		BaseAgent:   baseAgent,
		WorkDir:     workDir,
		SessionID:   "", // 首次启动时由 start() 方法生成，用于区分新会话和恢复会话
		wsHub:       m.wsHub,
	}

	logInfo("Starting new session", zap.String("sessionId", session.ID.String()))

	if err := session.start(ctx, initialInput); err != nil {
		logError("Failed to start session", zap.Error(err))
		return nil, err
	}

	m.sessions[threadID] = session
	logInfo("Session started successfully", zap.String("sessionId", session.ID.String()))
	return session, nil
}

// GetSession 获取会话
func (m *InteractiveSessionManager) GetSession(threadID uuid.UUID) *InteractiveSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[threadID]
}

// SendMessageToSession 向会话发送消息
func (m *InteractiveSessionManager) SendMessageToSession(threadID uuid.UUID, message string) error {
	m.mu.RLock()
	session, exists := m.sessions[threadID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", threadID)
	}

	return session.SendMessage(message)
}

// StopSession 停止会话
func (m *InteractiveSessionManager) StopSession(threadID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[threadID]
	if !exists {
		return nil
	}

	session.stop()
	delete(m.sessions, threadID)
	return nil
}

// start 启动 Claude CLI 进程
func (s *InteractiveSession) start(ctx context.Context, initialInput string) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	cliPath := "claude"
	if s.BaseAgent != nil && s.BaseAgent.CliPath != "" {
		cliPath = s.BaseAgent.CliPath
	}

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--permission-mode", "auto",
	}

	if s.AgentConfig.ModelName != "" {
		args = append(args, "--model", s.AgentConfig.ModelName)
	}

	// 会话恢复逻辑：
	// - 首次启动：使用 --session-id 指定会话ID，CLI 会创建并保存会话
	// - 后续消息：使用 --resume 恢复之前的会话，保持上下文
	if s.SessionID == "" {
		// 首次启动，生成新的 session ID
		s.SessionID = uuid.New().String()
		args = append(args, "--session-id", s.SessionID)
		logInfo("Starting new session with session-id",
			zap.String("sessionId", s.SessionID))
	} else {
		// 后续消息，恢复已有会话
		args = append(args, "--resume", s.SessionID)
		logInfo("Resuming existing session",
			zap.String("sessionId", s.SessionID))
	}

	prompt := s.buildPrompt(initialInput)

	// 记录完整命令行信息
	logInfo("Claude CLI Command",
		zap.String("cliPath", cliPath),
		zap.Strings("args", args),
		zap.String("workDir", s.WorkDir))

	if s.BaseAgent != nil {
		if s.BaseAgent.GitBashPath != "" {
			logDebug("GitBash path", zap.String("path", s.BaseAgent.GitBashPath))
		}
		if s.BaseAgent.ApiToken != "" {
			masked := s.BaseAgent.ApiToken
			if len(masked) > 10 {
				masked = masked[:10] + "..."
			}
			logDebug("API Token (masked)", zap.String("token", masked))
		}
	}

	logInfo("Starting Claude CLI process",
		zap.String("cliPath", cliPath),
		zap.Strings("args", args),
		zap.String("workDir", s.WorkDir))

	s.cmd = exec.CommandContext(s.ctx, cliPath, args...)
	s.cmd.Stdin = strings.NewReader(prompt)

	if s.WorkDir != "" {
		s.cmd.Dir = s.WorkDir
	}

	// 设置环境变量
	env := os.Environ()
	env = append(env, "CLAUDE_NO_INTERACTIVE=1")
	if s.BaseAgent != nil {
		if s.BaseAgent.ApiURL != "" {
			env = append(env, fmt.Sprintf("ANTHROPIC_API_URL=%s", s.BaseAgent.ApiURL))
		}
		if s.BaseAgent.ApiToken != "" {
			env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", s.BaseAgent.ApiToken))
		}
		if s.BaseAgent.GitBashPath != "" {
			env = append(env, fmt.Sprintf("CLAUDE_CODE_GIT_BASH_PATH=%s", s.BaseAgent.GitBashPath))
		}
	}
	s.cmd.Env = env

	// 获取输出管道
	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		logError("Failed to create stdout pipe", zap.Error(err))
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	s.stdout = stdout

	// 启动进程
	if err := s.cmd.Start(); err != nil {
		logError("Failed to start Claude CLI", zap.Error(err))
		return fmt.Errorf("failed to start claude: %w", err)
	}

	logInfo("Claude CLI process started", zap.Int("pid", s.cmd.Process.Pid))
	s.running = true

	// 启动输出读取协程
	go s.readOutput()

	return nil
}

// SendMessage 发送消息到会话
func (s *InteractiveSession) SendMessage(message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 停止当前进程（如果正在运行）
	if s.running && s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		logInfo("Killed running Claude process before sending new message",
			zap.String("threadId", s.ThreadID.String()),
			zap.String("sessionId", s.SessionID))
	}

	logInfo("SendMessage: resuming session with --resume",
		zap.String("threadId", s.ThreadID.String()),
		zap.String("sessionId", s.SessionID))

	// 启动新进程（start 方法内部会创建新的 context）
	if err := s.start(s.ctx, message); err != nil {
		return err
	}
	s.running = true
	return nil
}

// readOutput 读取输出
func (s *InteractiveSession) readOutput() {
	scanner := bufio.NewScanner(s.stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	logInfo("---------- Claude CLI Output Start ----------",
		zap.String("threadId", s.ThreadID.String()))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 记录原始输出到日志
		func() {
			endIdx := 500
			if endIdx > len(line) {
				endIdx = len(line)
			}
			logDebug("Claude CLI raw output",
				zap.String("threadId", s.ThreadID.String()),
				zap.String("raw", line[:endIdx]))
		}()

		// 解析并提取文本内容
		text := s.extractTextFromJSON(line)
		if text != "" {
			func() {
				endIdx := 200
				if endIdx > len(text) {
					endIdx = len(text)
				}
				logDebug("Claude CLI text extracted",
					zap.String("threadId", s.ThreadID.String()),
					zap.String("text", text[:endIdx]))
			}()
			s.broadcastChunk(text)
		}
	}

	if err := scanner.Err(); err != nil {
		logError("Scanner error", zap.Error(err))
	}

	// 进程结束
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	s.broadcastStatus("completed")
}

// extractTextFromJSON 从 JSON 输出中提取文本内容
func (s *InteractiveSession) extractTextFromJSON(line string) string {
	var base struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal([]byte(line), &base); err != nil {
		return line // 非 JSON，直接返回
	}

	switch base.Type {
	case "assistant":
		// assistant 类型: {"type":"assistant","message":{"content":[{"type":"text","text":"..."}]}}
		var assistant struct {
			Message struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &assistant); err == nil {
			var texts []string
			for _, c := range assistant.Message.Content {
				if c.Type == "text" && c.Text != "" {
					texts = append(texts, c.Text)
				}
			}
			return strings.Join(texts, "\n")
		}

	case "result":
		// result 类型: {"type":"result","result":"..."}
		var result struct {
			Result string `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &result); err == nil && result.Result != "" {
			return result.Result
		}

	case "error":
		// error 类型
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &errResp); err == nil {
			return fmt.Sprintf("ERROR: %s", errResp.Error)
		}
	}

	// 其他类型，尝试提取常见的文本字段
	var generic struct {
		Text    string `json:"text"`
		Content string `json:"content"`
		Message string `json:"message"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal([]byte(line), &generic); err == nil {
		if generic.Text != "" {
			return generic.Text
		}
		if generic.Content != "" {
			return generic.Content
		}
		if generic.Message != "" {
			return generic.Message
		}
		if generic.Result != "" {
			return generic.Result
		}
	}

	return "" // 无法提取，忽略
}


// broadcastChunk 广播输出块
func (s *InteractiveSession) broadcastChunk(chunk string) {
	logDebug("Broadcasting chunk",
		zap.String("threadId", s.ThreadID.String()),
		zap.Int("chunkLen", len(chunk)))
	if s.wsHub != nil {
		s.wsHub.BroadcastToThread(s.ThreadID.String(), ws.WSMessage{
			Type:      "agent_output_chunk",
			ThreadID:  s.ThreadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"sessionId": s.ID.String(),
				"chunk":     chunk,
			},
		})
	} else {
		logError("wsHub is nil, cannot broadcast", zap.String("threadId", s.ThreadID.String()))
	}
}

// broadcastStatus 广播状态
func (s *InteractiveSession) broadcastStatus(status string) {
	logInfo("Broadcasting status",
		zap.String("threadId", s.ThreadID.String()),
		zap.String("status", status))
	if s.wsHub != nil {
		s.wsHub.BroadcastToThread(s.ThreadID.String(), ws.WSMessage{
			Type:      "agent_status",
			ThreadID:  s.ThreadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"sessionId": s.ID.String(),
				"status":    status,
			},
		})
	}
}

// stop 停止会话
func (s *InteractiveSession) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
	if s.cancel != nil {
		s.cancel()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
}

// buildPrompt 构建提示词
func (s *InteractiveSession) buildPrompt(input string) string {
	var sb strings.Builder

	if s.AgentConfig.SystemPrompt != "" {
		sb.WriteString(s.AgentConfig.SystemPrompt)
		sb.WriteString("\n\n")
	}

	sb.WriteString(input)

	return sb.String()
}