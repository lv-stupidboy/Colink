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

// SessionManager 会话管理器，统一管理工作流场景和调试场景的会话
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	wsHub    *ws.Hub
	service  *ExecutionService // 引用ExecutionService以访问其方法
}

// Session 会话实体
type Session struct {
	ID          string                    // 会话唯一标识
	SessionType ExecutionContext          // 会话类型（工作流/调试）
	Config      *model.AgentRoleConfig    // Agent配置
	BaseAgent   *model.BaseAgent          // 基础Agent配置
	WorkDir     string                    // 工作目录
	SessionKey  string                    // Claude CLI 的 session key
	State       SessionStatus             // 当前状态
	Adapter     SessionExecutor           // 对应的适配器
	Cmd         *exec.Cmd                 // 当前运行的命令
	Ctx         context.Context           // 上下文
	Cancel      context.CancelFunc        // 取消函数
	Stdout      io.Reader                 // 标准输出
	mu          sync.Mutex                // 保护会话内部状态
}

// NewSessionManager 创建会话管理器
func NewSessionManager(wsHub *ws.Hub, service *ExecutionService) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
		wsHub:    wsHub,
		service:  service,
	}
}

// StartSession 启动会话
func (sm *SessionManager) StartSession(ctx context.Context, sessionID string, config *model.AgentRoleConfig, baseAgent *model.BaseAgent, workDir string, initialInput string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 获取对应的适配器
	adapter, err := sm.service.getAdapter(ctx, config, baseAgent)
	if err != nil {
		return fmt.Errorf("failed to get adapter: %w", err)
	}

	// 创建新的会话
	session := &Session{
		ID:          sessionID,
		SessionType: sm.determineSessionType(ctx, sessionID),
		Config:      config,
		BaseAgent:   baseAgent,
		WorkDir:     workDir,
		SessionKey:  "", // 首次启动时生成
		State:       SessionStatusRunning,
		Adapter:     adapter.(SessionExecutor), // 确保实现了SessionExecutor接口
	}

	// 如果是Claude适配器，我们需要特殊的启动逻辑
	if claudeAdapter, ok := adapter.(*ClaudeAdapter); ok {
		// 使用ClaudeAdapter启动交互式会话
		err = sm.startClaudeSession(ctx, session, initialInput, claudeAdapter)
		if err != nil {
			session.State = SessionStatusFailed
			return err
		}
	} else if openCodeAdapter, ok := adapter.(*OpenCodeAdapter); ok {
		// 使用OpenCodeAdapter启动交互式会话
		err = sm.startOpenCodeSession(ctx, session, initialInput, openCodeAdapter)
		if err != nil {
			session.State = SessionStatusFailed
			return err
		}
	} else {
		// 其他类型的适配器，目前主要针对Claude和OpenCode
		return fmt.Errorf("unsupported adapter type for session management: %T", adapter)
	}

	sm.sessions[sessionID] = session
	return nil
}

// determineSessionType 根据上下文确定会话类型
func (sm *SessionManager) determineSessionType(ctx context.Context, sessionID string) ExecutionContext {
	// 这里可以根据实际需要进行更复杂的判断逻辑
	// 简单起见，我们假设通过context中的值或者sessionID的特征来判断
	return ExecutionContextInteractive
}

// startClaudeSession 启动Claude会话
func (sm *SessionManager) startClaudeSession(ctx context.Context, session *Session, initialInput string, adapter *ClaudeAdapter) error {
	session.Ctx, session.Cancel = context.WithCancel(context.Background())

	cliPath := adapter.cliPath
	if session.BaseAgent != nil && session.BaseAgent.CliPath != "" {
		cliPath = session.BaseAgent.CliPath
	}

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--permission-mode", "auto",
	}

	if session.Config.ModelName != "" {
		args = append(args, "--model", session.Config.ModelName)
	}

	// 会话恢复逻辑：
	// - 首次启动：使用 --session-id 指定会话ID，CLI 会创建并保存会话
	// - 后续消息：使用 --resume 恢复之前的会话，保持上下文
	if session.SessionKey == "" {
		// 首次启动，生成新的 session key
		session.SessionKey = uuid.New().String()
		args = append(args, "--session-id", session.SessionKey)
		logInfo("Starting new Claude session with session-id",
			zap.String("sessionId", session.SessionKey))
	} else {
		// 后续消息，恢复已有会话
		args = append(args, "--resume", session.SessionKey)
		logInfo("Resuming existing Claude session",
			zap.String("sessionId", session.SessionKey))
	}

	prompt := sm.buildPrompt(session.Config, initialInput)

	// 记录完整命令行信息
	logInfo("Claude CLI Command",
		zap.String("cliPath", cliPath),
		zap.Strings("args", args),
		zap.String("workDir", session.WorkDir))

	if session.BaseAgent != nil {
		if session.BaseAgent.GitBashPath != "" {
			logDebug("GitBash path", zap.String("path", session.BaseAgent.GitBashPath))
		}
		if session.BaseAgent.ApiToken != "" {
			masked := session.BaseAgent.ApiToken
			if len(masked) > 10 {
				masked = masked[:10] + "..."
			}
			logDebug("API Token (masked)", zap.String("token", masked))
		}
	}

	session.Cmd = exec.CommandContext(session.Ctx, cliPath, args...)
	session.Cmd.Stdin = strings.NewReader(prompt)

	if session.WorkDir != "" {
		session.Cmd.Dir = session.WorkDir
	}

	// 设置环境变量
	env := os.Environ()
	env = append(env, "CLAUDE_NO_INTERACTIVE=1")
	if session.BaseAgent != nil {
		if session.BaseAgent.ApiURL != "" {
			env = append(env, fmt.Sprintf("ANTHROPIC_API_URL=%s", session.BaseAgent.ApiURL))
		}
		if session.BaseAgent.ApiToken != "" {
			env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", session.BaseAgent.ApiToken))
		}
		if session.BaseAgent.GitBashPath != "" {
			env = append(env, fmt.Sprintf("CLAUDE_CODE_GIT_BASH_PATH=%s", session.BaseAgent.GitBashPath))
		}
	}
	session.Cmd.Env = env

	// 获取输出管道
	stdout, err := session.Cmd.StdoutPipe()
	if err != nil {
		logError("Failed to create stdout pipe", zap.Error(err))
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	session.Stdout = stdout

	// 启动进程
	if err := session.Cmd.Start(); err != nil {
		logError("Failed to start Claude CLI", zap.Error(err))
		return fmt.Errorf("failed to start claude: %w", err)
	}

	logInfo("Claude CLI process started", zap.Int("pid", session.Cmd.Process.Pid))

	// 启动输出读取协程
	go sm.readClaudeOutput(session)

	return nil
}

// startOpenCodeSession 启动OpenCode会话
func (sm *SessionManager) startOpenCodeSession(ctx context.Context, session *Session, initialInput string, adapter *OpenCodeAdapter) error {
	session.Ctx, session.Cancel = context.WithCancel(context.Background())

	cliPath := adapter.cliPath
	if session.BaseAgent != nil && session.BaseAgent.CliPath != "" {
		cliPath = session.BaseAgent.CliPath
	}

	// OpenCode使用不同的命令参数
	args := []string{
		"run",
		"--model", session.Config.ModelName,
		"--stream",
		"--non-interactive",
	}

	if session.Config.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", session.Config.MaxTokens))
	}

	prompt := sm.buildPrompt(session.Config, initialInput)

	session.Cmd = exec.CommandContext(session.Ctx, cliPath, args...)
	session.Cmd.Stdin = strings.NewReader(prompt)

	if session.WorkDir != "" {
		session.Cmd.Dir = session.WorkDir
	}

	// 设置环境变量
	env := os.Environ()
	if session.BaseAgent != nil {
		if session.BaseAgent.ApiURL != "" {
			env = append(env, fmt.Sprintf("OPENCODE_API_URL=%s", session.BaseAgent.ApiURL))
		}
		if session.BaseAgent.ApiToken != "" {
			env = append(env, fmt.Sprintf("OPENCODE_API_KEY=%s", session.BaseAgent.ApiToken))
		}
	}
	session.Cmd.Env = env

	// 获取输出管道
	stdout, err := session.Cmd.StdoutPipe()
	if err != nil {
		logError("Failed to create stdout pipe for OpenCode", zap.Error(err))
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	session.Stdout = stdout

	// 启动进程
	if err := session.Cmd.Start(); err != nil {
		logError("Failed to start OpenCode CLI", zap.Error(err))
		return fmt.Errorf("failed to start opencode: %w", err)
	}

	logInfo("OpenCode CLI process started", zap.Int("pid", session.Cmd.Process.Pid))

	// 启动输出读取协程
	go sm.readOpenCodeOutput(session)

	return nil
}

// ResumeSession 恢复会话
func (sm *SessionManager) ResumeSession(ctx context.Context, sessionID string, input string) error {
	sm.mu.Lock()
	session, exists := sm.sessions[sessionID]
	sm.mu.Unlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// 停止当前进程（如果正在运行）
	if session.Cmd != nil && session.Cmd.Process != nil {
		session.Cmd.Process.Kill()
		logInfo("Killed running process before resuming session",
			zap.String("sessionID", session.ID))
	}

	// 根据适配器类型重新启动会话
	if claudeAdapter, ok := session.Adapter.(*ClaudeAdapter); ok {
		return sm.startClaudeSession(ctx, session, input, claudeAdapter)
	} else if openCodeAdapter, ok := session.Adapter.(*OpenCodeAdapter); ok {
		return sm.startOpenCodeSession(ctx, session, input, openCodeAdapter)
	} else {
		return fmt.Errorf("unsupported adapter type for resuming session: %T", session.Adapter)
	}
}

// StopSession 停止会话
func (sm *SessionManager) StopSession(ctx context.Context, sessionID string) error {
	sm.mu.Lock()
	session, exists := sm.sessions[sessionID]
	sm.mu.Unlock()

	if !exists {
		return nil // 会话不存在，视为成功停止
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.State = SessionStatusStopped

	if session.Cancel != nil {
		session.Cancel()
	}

	if session.Cmd != nil && session.Cmd.Process != nil {
		session.Cmd.Process.Kill()
	}

	// 从管理器中移除会话
	sm.mu.Lock()
	delete(sm.sessions, sessionID)
	sm.mu.Unlock()

	// 广播会话停止状态
	threadID, _ := uuid.Parse(sessionID) // 注意：这里简化处理，实际情况可能需要更复杂的ID映射
	sm.broadcastStatus(threadID, sessionID, "stopped")

	return nil
}

// GetSessionStatus 获取会话状态
func (sm *SessionManager) GetSessionStatus(sessionID string) SessionStatus {
	sm.mu.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !exists {
		return SessionStatusStopped
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	return session.State
}

// readClaudeOutput 读取Claude输出
func (sm *SessionManager) readClaudeOutput(session *Session) {
	scanner := bufio.NewScanner(session.Stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	threadID, _ := uuid.Parse(session.ID) // 简化处理
	logInfo("---------- Claude CLI Output Start ----------",
		zap.String("threadId", threadID.String()))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 记录原始输出到日志
		logDebug("Claude CLI raw output",
			zap.String("threadId", threadID.String()),
			zap.String("raw", line[:min(500, len(line))]))

		// 解析并提取文本内容
		text := sm.extractClaudeTextFromJSON(line)
		if text != "" {
			logDebug("Claude CLI text extracted",
				zap.String("threadId", threadID.String()),
				zap.String("text", text[:min(200, len(text))]))

			// 广播输出块
			sm.broadcastChunk(threadID, session.ID, text)
		}
	}

	if err := scanner.Err(); err != nil {
		logError("Scanner error", zap.Error(err))
	}

	// 进程结束
	session.mu.Lock()
	session.State = SessionStatusCompleted
	session.mu.Unlock()

	sm.broadcastStatus(threadID, session.ID, "completed")
}

// extractClaudeTextFromJSON 从 Claude JSON 输出中提取文本内容
func (sm *SessionManager) extractClaudeTextFromJSON(line string) string {
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

// readOpenCodeOutput 读取OpenCode输出
func (sm *SessionManager) readOpenCodeOutput(session *Session) {
	scanner := bufio.NewScanner(session.Stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	threadID, _ := uuid.Parse(session.ID) // 简化处理
	logInfo("---------- OpenCode CLI Output Start ----------",
		zap.String("threadId", threadID.String()))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 尝试解析JSON格式的输出
		var chunk OpenCodeStreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			// 如果不是JSON，直接作为文本处理
			sm.broadcastChunk(threadID, session.ID, line+"\n")
			continue
		}

		if chunk.Content != "" {
			sm.broadcastChunk(threadID, session.ID, chunk.Content)
		}
	}

	if err := scanner.Err(); err != nil {
		logError("OpenCode Scanner error", zap.Error(err))
	}

	// 进程结束
	session.mu.Lock()
	session.State = SessionStatusCompleted
	session.mu.Unlock()

	sm.broadcastStatus(threadID, session.ID, "completed")
}

// broadcastChunk 广播输出块
func (sm *SessionManager) broadcastChunk(threadID uuid.UUID, sessionID string, chunk string) {
	logDebug("Broadcasting chunk",
		zap.String("threadId", threadID.String()),
		zap.Int("chunkLen", len(chunk)))
	if sm.wsHub != nil {
		sm.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_output_chunk",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"sessionId": sessionID,
				"chunk":     chunk,
			},
		})
	} else {
		logError("wsHub is nil, cannot broadcast", zap.String("threadId", threadID.String()))
	}
}

// broadcastStatus 广播状态
func (sm *SessionManager) broadcastStatus(threadID uuid.UUID, sessionID string, status string) {
	logInfo("Broadcasting session status",
		zap.String("threadId", threadID.String()),
		zap.String("sessionId", sessionID),
		zap.String("status", status))
	if sm.wsHub != nil {
		sm.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_status",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"sessionId": sessionID,
				"status":    status,
			},
		})
	}
}

// buildPrompt 构建提示词
func (sm *SessionManager) buildPrompt(config *model.AgentRoleConfig, input string) string {
	var sb strings.Builder

	if config.SystemPrompt != "" {
		sb.WriteString(config.SystemPrompt)
		sb.WriteString("\n\n")
	}

	sb.WriteString(input)

	return sb.String()
}