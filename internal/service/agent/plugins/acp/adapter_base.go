// internal/service/agent/plugins/acp/adapter_base.go
// Base ACP Adapter implementation
package acp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AcpAdapterConfig holds configuration for ACP adapter.
// Exported for reuse by other ACP-based plugins (e.g., OpenClaw, OpenCode).
type AcpAdapterConfig struct {
	CliPath           string
	BuildArgs         func(req *agent.ExecutionRequest) []string
	BuildEnv          func(req *agent.ExecutionRequest) []string
	SkipModelConfig   func(req *agent.ExecutionRequest) bool // 如果返回 true，跳过默认模型配置
	LegacyModelConfig bool                                   // 如果 true，使用 session/set_model 而非 configOptions
	// Gateway 配置（用于第三方 API）
	// 如果设置了 GatewayBaseURL，会在 initialize 后发送 authenticate 请求
	GatewayBaseURL string            // 第三方 API 地址（如 https://coding.dashscope.aliyuncs.com/apps/anthropic/v1）
	GatewayHeaders map[string]string // 自定义 headers（如 x-api-key: xxx）
	// 用户级 MCP 配置加载函数（可选）
	// 返回用户级 MCP servers 配置（map 格式），会被转换为 ACP 的 mcpServers 数组格式
	LoadUserMCPConfig func() map[string]interface{}
}

// maxStderrSize 定义 stderr 缓冲的最大大小（64KB）
// 防止 stderr 输出过量导致内存问题
const maxStderrSize = 64 * 1024

type acpSession struct {
	id                string
	isdpID            string
	transport         *acpTransport
	stdinPipe         io.WriteCloser // stdin 管道引用（用于检测断开）
	cmd               *exec.Cmd
	ctx               context.Context
	cancel            context.CancelFunc
	status            agent.SessionStatus
	output            strings.Builder
	stderrOutput      strings.Builder // stderr 输出缓冲（用于错误诊断）
	pendingQuestion   *agent.Chunk    // 待处理的 AskUserQuestion（等待用户响应）
	thoughtChunkCount int             // 流式思考内容计数器（用于采样打印）
	// 诊断字段（info 级别可见，用于捕捉无限循环问题）
	notificationCount    int    // 收到的通知总数
	duplicateUpdateCount int    // 连续重复通知计数
	lastUpdateHash       string // 最后一次 session/update 的内容哈希（前16位）
	// 长连接模式输出同步信号
	lastOutputLen       int        // 上次检查时的输出长度
	outputUpdatedSignal chan struct{} // 输出更新信号（用于等待通知处理完成）
	mu                  sync.Mutex
}

// BaseACPAdapter implements AgentAdapter using ACP (Agent Client Protocol) over stdio.
// ACP lifecycle: initialize -> session/new -> session/prompt -> session/update notifications -> response
type BaseACPAdapter struct {
	Config    AcpAdapterConfig
	baseAgent *model.BaseAgent
	sessions  map[string]*acpSession
	mu        sync.RWMutex
}

// NewBaseACPAdapter creates a new BaseACPAdapter with the given configuration.
func NewBaseACPAdapter(config AcpAdapterConfig, baseAgent *model.BaseAgent) *BaseACPAdapter {
	return &BaseACPAdapter{
		Config:    config,
		baseAgent: baseAgent,
		sessions:  make(map[string]*acpSession),
	}
}

// GetCurrentProcess returns the current running process from active sessions.
// Returns nil if no process is currently running.
func (a *BaseACPAdapter) GetCurrentProcess() *exec.Cmd {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, session := range a.sessions {
		if session.cmd != nil && session.cmd.Process != nil {
			return session.cmd
		}
	}
	return nil
}

func (a *BaseACPAdapter) Execute(ctx context.Context, req *agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	return a.ExecuteWithStream(ctx, req, nil)
}

func (a *BaseACPAdapter) ExecuteWithStream(ctx context.Context, req *agent.ExecutionRequest, onChunk func(agent.Chunk)) (*agent.ExecutionResult, error) {
	cliStartTime := time.Now()

	args := a.Config.BuildArgs(req)
	cmd := exec.CommandContext(ctx, a.Config.CliPath, args...)
	hideCommandLineWindow(cmd) // 隐藏命令行窗口（Windows）

	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	env := a.buildEnv(req)
	cmd.Env = env

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("ACP: failed to create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ACP: failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("ACP: failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ACP: failed to start CLI process: %w", err)
	}

	// 打印 PowerShell 可执行的命令（只打印我们设置的环境变量）
	customEnv := a.Config.BuildEnv(req)
	var psCmd strings.Builder
	if req.WorkDir != "" {
		psCmd.WriteString("cd '")
		psCmd.WriteString(req.WorkDir)
		psCmd.WriteString("'; ")
	}
	for _, e := range customEnv {
		// 环境变量格式: KEY=VALUE，转换为 PowerShell 格式: $env:KEY='VALUE'
		if idx := strings.Index(e, "="); idx > 0 {
			key := e[:idx]
			value := e[idx+1:]
			psCmd.WriteString("$env:")
			psCmd.WriteString(key)
			psCmd.WriteString("='")
			psCmd.WriteString(value)
			psCmd.WriteString("'; ")
		}
	}
	psCmd.WriteString(a.Config.CliPath)
	for _, arg := range args {
		psCmd.WriteString(" '")
		psCmd.WriteString(arg)
		psCmd.WriteString("'")
	}
	LogInfo("ACP: PowerShell command", zap.String("psCommand", psCmd.String()))

	LogInfo("[PERF] ACP cmd.Start", zap.Duration("duration", time.Since(cliStartTime)))

	// 设置 invocationID 用于 AskUserQuestion 答案发送
	var invocationIDStr string
	if req.InvocationID != uuid.Nil {
		invocationIDStr = req.InvocationID.String()
	}

	// 创建 session（先创建，以便 stderr goroutine 可以引用它）
	session := &acpSession{
		cmd:    cmd,
		ctx:    ctx,
		status: agent.SessionStatusRunning,
		isdpID: invocationIDStr,
	}

	// 启动 stderr 消费 goroutine（在 session 创建之后）
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			// 记录到日志（实时可见）
			LogInfo("ACP: stderr output", zap.String("line", line))
			// 缓存到 session（带 64KB 上限）
			session.mu.Lock()
			if session.stderrOutput.Len() < maxStderrSize {
				session.stderrOutput.WriteString(line)
				session.stderrOutput.WriteString("\n")
			}
			session.mu.Unlock()
		}
		if err := scanner.Err(); err != nil {
			LogError("ACP: stderr scanner error", zap.Error(err))
		}
	}()

	// 将 session 保存到 sessions map，以便 GetCurrentProcess 能找到它用于取消执行
	if invocationIDStr != "" {
		a.mu.Lock()
		a.sessions[invocationIDStr] = session
		a.mu.Unlock()
		LogInfo("ACP: session saved to sessions map for execution",
			zap.String("isdpID", invocationIDStr))
	}

	transport := newACPTransport(stdinPipe, stdoutPipe, func(method string, params json.RawMessage) {
		a.handleNotification(session, method, params, onChunk)
	})
	session.transport = transport
	transport.Start()

	initResult, err := transport.SendRequest("initialize", &acpInitializeParams{
		ProtocolVersion:    2025,
		ClientCapabilities: acpClientCapabilities{},
	})
	if err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		a.cleanup(session)
		wg.Wait()
		return nil, fmt.Errorf("ACP: initialize handshake failed: %w\nstderr: %s", err, stderrContent)
	}

	var initResp acpInitializeResult
	if err := json.Unmarshal(initResult, &initResp); err != nil {
		LogWarn("ACP: initialize response parse warning", zap.Error(err))
	}
	LogInfo("[PERF] ACP initialize handshake", zap.Duration("duration", time.Since(cliStartTime)),
		zap.Int("protocolVersion", initResp.ProtocolVersion))

	// 发送 gateway authenticate（如果配置了第三方 API）
	if err := a.sendAuthenticate(transport); err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		a.cleanup(session)
		wg.Wait()
		return nil, fmt.Errorf("ACP: gateway authenticate failed: %w\nstderr: %s", err, stderrContent)
	}

	// 根据服务器实际支持的协议版本决定是否传递 MCP Servers
	// ACP v1 不支持 mcpServers 字段，只有 v2025+ 支持
	mcpServers := []interface{}{}
	if initResp.ProtocolVersion >= 2025 {
		mcpServers = a.buildMCPServers(req)
	}

	sessionNewResult, err := transport.SendRequest("session/new", &acpNewSessionParams{
		CWD:        req.WorkDir,
		MCPServers: mcpServers,
	})
	if err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		a.cleanup(session)
		wg.Wait()
		return nil, fmt.Errorf("ACP: session/new failed: %w\nstderr: %s", err, stderrContent)
	}

	var sessionResp acpNewSessionResult
	if err := json.Unmarshal(sessionNewResult, &sessionResp); err != nil {
		LogWarn("ACP: session/new response parse warning", zap.Error(err))
	}
	if sessionResp.SessionID != "" {
		session.id = sessionResp.SessionID
	} else {
		session.id = uuid.New().String()
	}

	LogInfo("ACP: session created",
		zap.String("sessionId", session.id),
		zap.String("invocationId", invocationIDStr))

	if err := a.configureSession(transport, session, &sessionResp, req); err != nil {
		LogWarn("ACP: session configuration warning", zap.Error(err))
	}

	prompt := a.buildPromptFromRequest(req)
	// 构建内容块列表（文本 + 图片）
	contentBlocks := a.buildContentBlocks(prompt, req.Images)
	LogInfo("ACP: buildContentBlocks", zap.Int("textLen", len(prompt)), zap.Int("imagesCount", len(req.Images)), zap.Int("blocksCount", len(contentBlocks)))
	promptResult, err := transport.SendRequest("session/prompt", &acpPromptParams{
		SessionID: session.id,
		Prompt:    contentBlocks,
	})
	if err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		a.cleanup(session)
		wg.Wait()
		return nil, fmt.Errorf("ACP: session/prompt failed: %w\nstderr: %s", err, stderrContent)
	}

	var promptResp acpPromptResult
	if err := json.Unmarshal(promptResult, &promptResp); err != nil {
		LogWarn("ACP: prompt response parse warning", zap.Error(err))
	}

	LogInfo("[PERF] ACP total execution",
		zap.Duration("duration", time.Since(cliStartTime)),
		zap.String("stopReason", promptResp.StopReason),
		zap.String("promptResultRaw", string(promptResult)))

	// 注意：不调用 session/close，因为它会释放资源并清理 session 数据
	// SDK 会自动持久化 session 到磁盘，不需要显式调用 close
	// 只有需要删除 session 时才调用 session/delete

	// 执行完成后，从 sessions map 中移除（如果有）
	if invocationIDStr != "" {
		a.mu.Lock()
		delete(a.sessions, invocationIDStr)
		a.mu.Unlock()
	}

	a.cleanup(session)
	wg.Wait()

	session.mu.Lock()
	output := session.output.String()
	session.mu.Unlock()

	return &agent.ExecutionResult{
		Output:    output,
		SessionID: session.id,
	}, nil
}

func (a *BaseACPAdapter) StartSession(ctx context.Context, sessionID string, req *agent.ExecutionRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.sessions[sessionID]; exists {
		return fmt.Errorf("ACP: session already exists: %s", sessionID)
	}

	args := a.Config.BuildArgs(req)
	cmd := exec.CommandContext(ctx, a.Config.CliPath, args...)
	hideCommandLineWindow(cmd) // 隐藏命令行窗口（Windows）

	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	env := a.buildEnv(req)
	cmd.Env = env

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("ACP: failed to create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ACP: failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("ACP: failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ACP: failed to start CLI process: %w", err)
	}

	// 打印 PowerShell 可执行的命令（只打印我们设置的环境变量）
	customEnv := a.Config.BuildEnv(req)
	var psCmd strings.Builder
	if req.WorkDir != "" {
		psCmd.WriteString("cd '")
		psCmd.WriteString(req.WorkDir)
		psCmd.WriteString("'; ")
	}
	for _, e := range customEnv {
		// 环境变量格式: KEY=VALUE，转换为 PowerShell 格式: $env:KEY='VALUE'
		if idx := strings.Index(e, "="); idx > 0 {
			key := e[:idx]
			value := e[idx+1:]
			psCmd.WriteString("$env:")
			psCmd.WriteString(key)
			psCmd.WriteString("='")
			psCmd.WriteString(value)
			psCmd.WriteString("'; ")
		}
	}
	psCmd.WriteString(a.Config.CliPath)
	for _, arg := range args {
		psCmd.WriteString(" '")
		psCmd.WriteString(arg)
		psCmd.WriteString("'")
	}
	LogInfo("ACP: PowerShell command (StartSession)",
		zap.String("sessionID", sessionID),
		zap.String("psCommand", psCmd.String()))

	// 创建 session（先创建，以便 stderr goroutine 可以引用它）
	sessionCtx, sessionCancel := context.WithCancel(ctx)
	session := &acpSession{
		id:     sessionID,
		isdpID: sessionID,
		cmd:    cmd,
		ctx:    sessionCtx,
		cancel: sessionCancel,
		status: agent.SessionStatusRunning,
	}

	// 启动 stderr 消费 goroutine（添加 wg 以防止 goroutine leak）
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			// 记录到日志
			LogInfo("ACP: stderr output (StartSession)",
				zap.String("sessionID", sessionID),
				zap.String("line", line))
			// 缓存到 session（带 64KB 上限）
			session.mu.Lock()
			if session.stderrOutput.Len() < maxStderrSize {
				session.stderrOutput.WriteString(line)
				session.stderrOutput.WriteString("\n")
			}
			session.mu.Unlock()
		}
		if err := scanner.Err(); err != nil {
			LogError("ACP: stderr scanner error (StartSession)", zap.Error(err))
		}
	}()

	transport := newACPTransport(stdinPipe, stdoutPipe, func(method string, params json.RawMessage) {
		a.handleNotification(session, method, params, nil)
	})
	session.transport = transport
	transport.Start()

	_, err = transport.SendRequest("initialize", &acpInitializeParams{
		ProtocolVersion:    2025,
		ClientCapabilities: acpClientCapabilities{},
	})
	if err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		wg.Wait()
		transport.Close()
		cmd.Process.Kill()
		return fmt.Errorf("ACP: initialize handshake failed: %w\nstderr: %s", err, stderrContent)
	}

	// 发送 gateway authenticate（如果配置了第三方 API）
	if err := a.sendAuthenticate(transport); err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		wg.Wait()
		transport.Close()
		cmd.Process.Kill()
		return fmt.Errorf("ACP: gateway authenticate failed: %w\nstderr: %s", err, stderrContent)
	}

	// 根据服务器实际支持的协议版本决定是否传递 MCP Servers
	// ACP v1 不支持 mcpServers 字段，只有 v2025+ 支持
	mcpServers := a.buildMCPServers(req)

	sessionNewResult, err := transport.SendRequest("session/new", &acpNewSessionParams{
		CWD:        req.WorkDir,
		MCPServers: mcpServers,
	})
	if err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		wg.Wait()
		transport.Close()
		cmd.Process.Kill()
		return fmt.Errorf("ACP: session/new failed: %w\nstderr: %s", err, stderrContent)
	}

	var sessionResp acpNewSessionResult
	if err := json.Unmarshal(sessionNewResult, &sessionResp); err == nil && sessionResp.SessionID != "" {
		session.id = sessionResp.SessionID
	}

	if err := a.configureSession(transport, session, &sessionResp, req); err != nil {
		LogWarn("ACP: session configuration warning", zap.Error(err))
	}

	a.sessions[sessionID] = session
	LogInfo("ACP: session started", zap.String("sessionId", sessionID), zap.String("acpSessionId", session.id))

	return nil
}

func (a *BaseACPAdapter) ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(agent.Chunk)) error {
	a.mu.RLock()
	session, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return fmt.Errorf("ACP: session not found: %s", sessionID)
	}

	session.mu.Lock()
	if session.transport == nil {
		session.mu.Unlock()
		return fmt.Errorf("ACP: session transport not available: %s", sessionID)
	}
	session.mu.Unlock()

	// Resume 时只传递文本（不传递图片）
	prompt := []acpContentBlock{{Type: "text", Text: input}}
	_, err := session.transport.SendRequest("session/prompt", &acpPromptParams{
		SessionID: session.id,
		Prompt:    prompt,
	})
	if err != nil {
		return fmt.Errorf("ACP: session/prompt failed: %w", err)
	}

	return nil
}

func (a *BaseACPAdapter) StopSession(sessionID string) error {
	a.mu.Lock()
	session, exists := a.sessions[sessionID]
	if !exists {
		a.mu.Unlock()
		return nil
	}
	delete(a.sessions, sessionID)
	a.mu.Unlock()

	session.mu.Lock()
	session.status = agent.SessionStatusStopped
	if session.cancel != nil {
		session.cancel()
	}
	session.mu.Unlock()

	a.cleanup(session)

	LogInfo("ACP: session stopped", zap.String("sessionId", sessionID))
	return nil
}

func (a *BaseACPAdapter) GetSessionStatus(sessionID string) agent.SessionStatus {
	a.mu.RLock()
	session, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return agent.SessionStatusIdle
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	return session.status
}

func (a *BaseACPAdapter) CheckHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := a.Config.BuildArgs(&agent.ExecutionRequest{BaseAgent: a.baseAgent})
	cmd := exec.CommandContext(ctx, a.Config.CliPath, args...)
	hideCommandLineWindow(cmd) // 隐藏命令行窗口（Windows）
	cmd.Dir = os.TempDir()
	cmd.Env = a.buildEnv(&agent.ExecutionRequest{BaseAgent: a.baseAgent})

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("ACP: health check stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ACP: health check stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ACP: health check start failed: %w", err)
	}

	transport := newACPTransport(stdinPipe, stdoutPipe, nil)
	transport.Start()
	defer transport.Close()

	_, err = transport.SendRequest("initialize", &acpInitializeParams{
		ProtocolVersion:    2025,
		ClientCapabilities: acpClientCapabilities{},
	})
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return fmt.Errorf("ACP: health check initialize failed: %w", err)
	}

	transport.Close()
	cmd.Wait()

	return nil
}

func (a *BaseACPAdapter) handleNotification(session *acpSession, method string, params json.RawMessage, onChunk func(agent.Chunk)) {
	// 只对重要通知打印 Info 日志，高频通知降级为 Debug
	switch method {
	case "usage_update", "session/new", "session/prompt", "session/end":
		LogInfo("ACP: received notification",
			zap.String("method", method),
			zap.String("sessionId", session.id))
	default:
		// 高频通知（session/update 等）使用 Debug 级别
		LogDebug("ACP: received notification",
			zap.String("method", method),
			zap.String("sessionId", session.id))
	}

	switch method {
	case "session/update":
		if onChunk == nil {
			return
		}

		// 诊断：检测重复通知（info 级别，用于捕捉无限循环）
		session.mu.Lock()
		session.notificationCount++
		// 每 20 个通知打印一次摘要（避免日志爆炸，但能捕捉异常）
		if session.notificationCount%20 == 0 {
			LogInfo("ACP: notification progress",
				zap.String("sessionId", session.id),
				zap.Int("count", session.notificationCount),
				zap.Int("duplicateCount", session.duplicateUpdateCount))
		}
		// 检测内容重复：计算 params 哈希并比较
		contentHash := fmt.Sprintf("%x", sha256.Sum256(params))[:16]
		if session.lastUpdateHash == contentHash {
			session.duplicateUpdateCount++
			// 重复 >3 次立即警告（info 级别）
			if session.duplicateUpdateCount > 3 {
				LogWarn("ACP: duplicate session/update detected",
					zap.String("sessionId", session.id),
					zap.String("hash", contentHash),
					zap.Int("duplicateCount", session.duplicateUpdateCount),
					zap.Int("totalCount", session.notificationCount))
			}
		} else {
			session.duplicateUpdateCount = 0
			session.lastUpdateHash = contentHash
		}
		session.mu.Unlock()

		var updateParams acpSessionUpdateParams
		if err := json.Unmarshal(params, &updateParams); err != nil {
			LogError("ACP: failed to parse session/update params", zap.Error(err))
			return
		}

		// 解析 sessionUpdate 类型（高频日志降级为 Debug）
		var header acpSessionUpdateHeader
		if err := json.Unmarshal(updateParams.Update, &header); err == nil {
			LogDebug("ACP: session/update type",
				zap.String("sessionId", updateParams.SessionID),
				zap.String("sessionUpdate", header.SessionUpdate))
		}

		chunks, err := parseACPSessionUpdate(updateParams.Update, session)
		if err != nil {
			LogError("ACP: failed to parse session update", zap.Error(err))
			return
		}

		session.mu.Lock()
		defer session.mu.Unlock()
		for _, chunk := range chunks {
			if chunk.Type == agent.ChunkTypeText {
				session.output.WriteString(chunk.Content)
			}
			// 对于 question 类型，存储到 session 以便等待用户响应
			if chunk.Type == agent.ChunkTypeQuestion {
				session.pendingQuestion = &chunk
			}
			// 流式思考内容采样打印：每50个 thinking chunk 打印一条摘要日志
			if chunk.Type == agent.ChunkTypeThinking {
				session.thoughtChunkCount++
				if session.thoughtChunkCount%50 == 0 {
					LogInfo("ACP: thinking progress",
						zap.String("sessionId", session.id),
						zap.Int("chunkCount", session.thoughtChunkCount),
						zap.Int("contentLen", len(chunk.Content)))
				}
			}
			onChunk(chunk)
		}

	case "session/request_permission":
		if session.transport != nil {
			session.transport.SendNotification("session/resolve_permission", &acpPermissionResponse{
				Allow: "allow_always",
			})
			LogDebug("ACP: permission auto-approved", zap.String("sessionId", session.id))
		}

	case "session/request_user_input":
		// 处理 AskUserQuestion 工具的用户输入请求
		if onChunk == nil {
			return
		}
		var inputRequest acpUserInputRequest
		if err := json.Unmarshal(params, &inputRequest); err != nil {
			LogError("ACP: failed to parse session/request_user_input params", zap.Error(err))
			return
		}
		// 用户输入请求日志降级为 Debug（调试时可用）
		LogDebug("ACP: received user input request",
			zap.String("sessionId", inputRequest.SessionID),
			zap.String("toolCallId", inputRequest.ToolCallID),
			zap.String("toolName", inputRequest.ToolName),
			zap.Any("input", inputRequest.Input))

		// 详细打印 input 结构（用于调试解析问题，高频降级为 Debug）
		inputJSON, _ := json.MarshalIndent(inputRequest.Input, "", "  ")
		LogDebug("ACP: user input request - detailed input structure",
			zap.String("inputJSON", string(inputJSON)))

		// 解析问题并创建 question chunk（调试日志降级为 Debug）
		chunk := parseACPUserInputRequest(inputRequest)
		LogDebug("ACP: parsed question chunk",
			zap.String("toolName", chunk.ToolName),
			zap.Int("questionsCount", len(chunk.Questions)),
			zap.Any("questions", chunk.Questions))
		session.mu.Lock()
		session.pendingQuestion = &chunk
		session.mu.Unlock()

		// 将 session 保存到 sessions map，以便 SendToolResult 能找到它
		// 使用 isdpID（即 invocationID）作为 key
		if session.isdpID != "" {
			a.mu.Lock()
			a.sessions[session.isdpID] = session
			a.mu.Unlock()
			LogDebug("ACP: session saved to sessions map for AskUserQuestion",
				zap.String("isdpID", session.isdpID),
				zap.String("acpSessionId", session.id),
				zap.String("toolCallId", inputRequest.ToolCallID))
		}

		onChunk(chunk)

	case "session/tool_call_update":
		// OpenCode 通过单独的 session/tool_call_update 通知发送工具更新
		if onChunk == nil {
			return
		}
		// 工具调用更新日志降级为 Debug（高频）
		LogDebug("ACP: received session/tool_call_update notification", zap.String("params", string(params)))

		// 尝试解析通知参数
		var updateParams struct {
			SessionID string          `json:"sessionId"`
			Update    json.RawMessage `json:"update"`
		}
		if err := json.Unmarshal(params, &updateParams); err != nil {
			// 可能直接是 tool_call_update 结构（不带 sessionId）
			LogWarn("ACP: failed to parse session/tool_call_update params, trying direct parse", zap.Error(err))
			chunks, err := parseACPToolCallUpdate(params)
			if err != nil {
				LogError("ACP: failed to parse tool_call_update directly", zap.Error(err))
				return
			}
			for _, chunk := range chunks {
				onChunk(chunk)
			}
			return
		}

		// 有 update 字段的情况
		if len(updateParams.Update) > 0 {
			chunks, err := parseACPToolCallUpdate(updateParams.Update)
			if err != nil {
				LogError("ACP: failed to parse tool_call_update from update field", zap.Error(err))
				return
			}
			for _, chunk := range chunks {
				onChunk(chunk)
			}
		}

	default:
		// 未知通知方法降级为 Debug
		LogDebug("ACP: unknown notification method",
			zap.String("method", method),
			zap.String("params", string(params)))
	}
}

func (a *BaseACPAdapter) buildPromptFromRequest(req *agent.ExecutionRequest) string {
	return agent.BuildPromptFromRequest(req)
}

// buildContentBlocks 构建内容块列表（文本 + 图片）
func (a *BaseACPAdapter) buildContentBlocks(text string, images []model.ImageContent) []acpContentBlock {
	blocks := []acpContentBlock{{Type: "text", Text: text}}

	// 添加图片内容块（使用 ACP source 格式）
	for _, img := range images {
		blocks = append(blocks, acpContentBlock{
			Type: "image",
			Source: &acpImageSource{
				Type:      "base64",
				MediaType: img.MimeType,
				Data:      img.Data,
			},
		})
	}

	return blocks
}

func (a *BaseACPAdapter) buildEnv(req *agent.ExecutionRequest) []string {
	envMap := make(map[string]string)

	for _, e := range os.Environ() {
		if idx := strings.Index(e, "="); idx > 0 {
			envMap[e[:idx]] = e[idx+1:]
		}
	}

	if extraEnv := a.Config.BuildEnv(req); len(extraEnv) > 0 {
		for _, e := range extraEnv {
			if idx := strings.Index(e, "="); idx > 0 {
				envMap[e[:idx]] = e[idx+1:]
			}
		}
	}

	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

// buildMCPServers 构建 MCP server 配置数组
// 用于在 session/new 时注入 memory MCP server 和用户级 MCP 配置
func (a *BaseACPAdapter) buildMCPServers(req *agent.ExecutionRequest) []interface{} {
	// 如果没有必要的参数，不注入任何 MCP
	if req.CallbackToken == "" || req.APIURL == "" || req.InvocationID == uuid.Nil {
		// 但仍然尝试加载用户级 MCP
		if a.Config.LoadUserMCPConfig != nil {
			userMCP := a.Config.LoadUserMCPConfig()
			return convertUserMCPToACPFormat(userMCP)
		}
		return []interface{}{}
	}

	// 结果数组
	mcpServers := []interface{}{}

	// 1. 加载用户级 MCP 配置（如果有）
	if a.Config.LoadUserMCPConfig != nil {
		userMCP := a.Config.LoadUserMCPConfig()
		if userMCP != nil && len(userMCP) > 0 {
			userServers := convertUserMCPToACPFormat(userMCP)
			mcpServers = append(mcpServers, userServers...)
			serverNames := make([]string, 0, len(userMCP))
			for name := range userMCP {
				serverNames = append(serverNames, name)
			}
			LogInfo("ACP: Loaded user MCP servers", zap.Strings("servers", serverNames))
		}
	}

	// 2. 获取平台 MCP server 可执行文件路径
	// 服务启动时已设置 ISDP_MCP_SERVER_PATH 环境变量（支持开发模式和安装模式）
	mcpServerPath := os.Getenv("ISDP_MCP_SERVER_PATH")
	if mcpServerPath == "" {
		// 回退：如果环境变量未设置，尝试开发模式路径
		workDir, err := os.Getwd()
		if err == nil {
			mcpServerPath = filepath.Join(workDir, "bin", "mcp-server.exe")
		}
	}

	if mcpServerPath == "" {
		LogInfo("ACP: WARNING - MCP server path not configured")
		if len(mcpServers) > 0 {
			return mcpServers // 至少返回用户级 MCP
		}
		return []interface{}{}
	}

	LogInfo("ACP: MCP server path", zap.String("path", mcpServerPath))

	// 3. 构建 memory MCP server 配置
	// ACP 协议：stdio 类型不需要 type 字段，env 是数组格式 [{name, value}]
	mcpServer := map[string]interface{}{
		"name":    "isdp-memory",
		"command": mcpServerPath,
		"args":    []string{},
		"env": []map[string]string{
			{"name": "ISDP_API_URL", "value": req.APIURL},
			{"name": "ISDP_INVOCATION_ID", "value": req.InvocationID.String()},
			{"name": "ISDP_CALLBACK_TOKEN", "value": req.CallbackToken},
		},
	}

	mcpServers = append(mcpServers, mcpServer)
	return mcpServers
}

// convertUserMCPToACPFormat 将用户级 MCP 配置（map 格式）转换为 ACP 的数组格式
// 用户级配置格式：{"serverName": {"type": "http/stdio/sse", "command/url": "...", ...}}
// ACP 格式：
// - stdio: {"name": "serverName", "command": "...", "args": [...], "env": [{name, value}]}
// - http: {"type": "http", "name": "serverName", "url": "...", "headers": [{name, value}]}
// - sse: {"type": "sse", "name": "serverName", "url": "...", "headers": [{name, value}]}
func convertUserMCPToACPFormat(userMCP map[string]interface{}) []interface{} {
	if userMCP == nil {
		return []interface{}{}
	}

	result := []interface{}{}
	for name, config := range userMCP {
		configMap, ok := config.(map[string]interface{})
		if !ok {
			continue
		}

		// 检查 transport 类型
		transportType := ""
		if t, ok := configMap["type"].(string); ok {
			transportType = t
		}

		// 构建 ACP 格式的 MCP server 配置
		acpServer := map[string]interface{}{
			"name": name,
		}

		switch transportType {
		case "http", "sse":
			// HTTP/SSE transport: 需要 type 字段
			acpServer["type"] = transportType
			if url, ok := configMap["url"].(string); ok {
				acpServer["url"] = url
			}
			// headers 必须转换为数组格式 [{name, value}]
			headersArray := []map[string]string{}
			if headers, ok := configMap["headers"].(map[string]interface{}); ok {
				for k, v := range headers {
					if vs, ok := v.(string); ok {
						headersArray = append(headersArray, map[string]string{
							"name":  k,
							"value": vs,
						})
					}
				}
			} else if headers, ok := configMap["headers"].(map[string]string); ok {
				for k, v := range headers {
					headersArray = append(headersArray, map[string]string{
						"name":  k,
						"value": v,
					})
				}
			}
			acpServer["headers"] = headersArray

		default:
			// Stdio transport（默认）: 不需要 type 字段
			// 复制各个字段
			if cmd, ok := configMap["command"].(string); ok {
				acpServer["command"] = cmd
			}
			if args, ok := configMap["args"].([]interface{}); ok {
				acpServer["args"] = args
			} else if args, ok := configMap["args"].([]string); ok {
				acpServer["args"] = args
			}
			// env 必须转换为数组格式 [{name, value}]
			envArray := []map[string]string{}
			if env, ok := configMap["env"].(map[string]interface{}); ok {
				for k, v := range env {
					if vs, ok := v.(string); ok {
						envArray = append(envArray, map[string]string{
							"name":  k,
							"value": vs,
						})
					}
				}
			} else if env, ok := configMap["env"].(map[string]string); ok {
				for k, v := range env {
					envArray = append(envArray, map[string]string{
						"name":  k,
						"value": v,
					})
				}
			}
			acpServer["env"] = envArray
		}

		result = append(result, acpServer)
	}

	return result
}

func (a *BaseACPAdapter) configureSession(transport *acpTransport, session *acpSession, sessionResp *acpNewSessionResult, req *agent.ExecutionRequest) error {
	// 检查是否跳过模型配置
	if a.Config.SkipModelConfig != nil && a.Config.SkipModelConfig(req) {
		LogInfo("ACP: skipping model config (plugin requested)")
		return nil
	}

	desiredModel := a.baseAgent.DefaultModel

	// 如果插件指定使用 legacy API
	if a.Config.LegacyModelConfig {
		return a.configureViaLegacyAPI(transport, session, desiredModel)
	}

	// 默认：优先使用 configOptions，如果没有则使用 legacy API
	if len(sessionResp.ConfigOptions) > 0 {
		return a.configureViaConfigOptions(transport, session, sessionResp, desiredModel)
	}

	return a.configureViaLegacyAPI(transport, session, desiredModel)
}

func (a *BaseACPAdapter) configureViaLegacyAPI(transport *acpTransport, session *acpSession, desiredModel string) error {
	if desiredModel != "" {
		if _, err := transport.SendRequest("session/set_model", &acpSetModelParams{
			SessionID: session.id,
			ModelID:   desiredModel,
		}); err != nil {
			return fmt.Errorf("set_model %s: %w", desiredModel, err)
		}
		LogInfo("ACP: model set via legacy API", zap.String("model", desiredModel))
	}
	return nil
}

func (a *BaseACPAdapter) configureViaConfigOptions(transport *acpTransport, session *acpSession, sessionResp *acpNewSessionResult, desiredModel string) error {
	for _, opt := range sessionResp.ConfigOptions {
		if opt.ConfigID == "model" && desiredModel != "" {
			if _, err := transport.SendRequest("session/set_config_option", &acpSetConfigOptionParams{
				SessionID: session.id,
				ConfigID:  "model",
				Value:     desiredModel,
			}); err != nil {
				return fmt.Errorf("set_config_option model=%s: %w", desiredModel, err)
			}
			LogInfo("ACP: model set via configOptions", zap.String("model", desiredModel))
		}
	}
	return nil
}

func (a *BaseACPAdapter) cleanup(session *acpSession) {
	// 诊断：cleanup 调用时打印关键指标（info 级别）
	session.mu.Lock()
	LogInfo("ACP: cleanup called",
		zap.String("sessionId", session.id),
		zap.Int("notificationCount", session.notificationCount),
		zap.Int("duplicateCount", session.duplicateUpdateCount),
		zap.Int("outputLen", session.output.Len()),
		zap.Int("stderrLen", session.stderrOutput.Len()))
	session.mu.Unlock()

	if session.transport != nil {
		session.transport.Close()
	}

	if session.cmd != nil && session.cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- session.cmd.Wait()
		}()
		select {
		case <-done:
			// 进程正常退出
			LogInfo("ACP: process exited normally", zap.String("sessionId", session.id))
		case <-time.After(3 * time.Second):
			LogWarn("ACP: process still running, terminating process tree", zap.String("sessionId", session.id), zap.Int("pid", session.cmd.Process.Pid))
			// 使用 killProcessTree 终止整个进程树（包括子进程如 bun）
			if err := killProcessTree(session.cmd.Process); err != nil {
				LogError("ACP: failed to terminate process tree", zap.Error(err), zap.Int("pid", session.cmd.Process.Pid))
			}
			// 等待进程完全退出
			select {
			case <-done:
				LogInfo("ACP: process tree terminated")
			case <-time.After(2 * time.Second):
				LogWarn("ACP: process still not exiting after killProcessTree")
				<-done // 最终等待
			}
		}
	}
}

// SendToolResult 发送工具结果给 CLI（用于 AskUserQuestion 等需要用户输入的工具）
// ACP 协议说明：
// - 使用 session/resolve_tool_call 请求方法（而非通知）
// - 参数包含 toolCallId 和 response
func (a *BaseACPAdapter) SendToolResult(invocationID uuid.UUID, toolCallID string, result string) error {
	a.mu.RLock()
	session, exists := a.sessions[invocationID.String()]
	a.mu.RUnlock()

	if !exists {
		return fmt.Errorf("ACP: session not found for invocation %s", invocationID.String())
	}

	if session.transport == nil {
		return fmt.Errorf("ACP: session transport not available")
	}

	// 使用请求方法 session/resolve_tool_call（而非通知）
	// 这确保 CLI 正确处理响应并继续执行
	resolveParams := map[string]interface{}{
		"toolCallId": toolCallID,
		"response":   result,
	}

	LogInfo("ACP: sending tool result via session/resolve_tool_call",
		zap.String("invocationID", invocationID.String()),
		zap.String("toolCallId", toolCallID),
		zap.String("response", result))

	// 发送请求而非通知，等待 CLI 确认
	_, err := session.transport.SendRequest("session/resolve_tool_call", resolveParams)
	if err != nil {
		// 如果请求方法失败，尝试使用通知方法作为备选
		LogWarn("ACP: session/resolve_tool_call request failed, trying notification",
			zap.Error(err))

		err = session.transport.SendNotification("session/resolve_user_input", &acpUserInputResponse{
			ToolCallID: toolCallID,
			Response:   result,
		})
		if err != nil {
			return fmt.Errorf("ACP: failed to send user input response: %w", err)
		}
	}

	LogInfo("ACP: tool result sent successfully",
		zap.String("toolCallId", toolCallID))
	return nil
}


// ========== ACP 原生 Session 管理 API ==========

// SessionList 获取历史会话列表
// 实现 agent.SessionResumeCapable 接口
func (a *BaseACPAdapter) SessionList(ctx context.Context, cwd string) ([]agent.SessionInfo, error) {
	// 启动临时进程来获取 session list
	args := a.Config.BuildArgs(&agent.ExecutionRequest{WorkDir: cwd})
	cmd := exec.CommandContext(ctx, a.Config.CliPath, args...)
	hideCommandLineWindow(cmd)

	if cwd != "" {
		cmd.Dir = cwd
	}

	env := a.buildEnv(&agent.ExecutionRequest{WorkDir: cwd})
	cmd.Env = env

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("ACP: failed to create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ACP: failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("ACP: failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ACP: failed to start CLI process: %w", err)
	}

	// 创建临时 session 用于 transport
	session := &acpSession{
		cmd:    cmd,
		ctx:    ctx,
		status: agent.SessionStatusRunning,
	}

	// 启动 stderr 消费
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			LogInfo("ACP: stderr output (session/list)", zap.String("line", line))
		}
	}()

	transport := newACPTransport(stdinPipe, stdoutPipe, func(method string, params json.RawMessage) {
		a.handleNotification(session, method, params, nil)
	})
	transport.Start()

	// Initialize
	initResult, err := transport.SendRequest("initialize", &acpInitializeParams{
		ProtocolVersion:    2025,
		ClientCapabilities: acpClientCapabilities{},
	})
	if err != nil {
		transport.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("ACP: initialize handshake failed: %w", err)
	}

	var initResp acpInitializeResult
	if err := json.Unmarshal(initResult, &initResp); err != nil {
		LogWarn("ACP: initialize response parse warning", zap.Error(err))
	}

	// 发送 gateway authenticate（如果配置了第三方 API）
	if err := a.sendAuthenticate(transport); err != nil {
		transport.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("ACP: gateway authenticate failed: %w", err)
	}

	// SessionList
	listResult, err := transport.SendRequest("session/list", &acpSessionListParams{
		CWD: cwd,
	})
	if err != nil {
		transport.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("ACP: session/list failed: %w", err)
	}

	var listResp acpSessionListResult
	if err := json.Unmarshal(listResult, &listResp); err != nil {
		transport.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("ACP: session/list response parse error: %w", err)
	}

	// 关闭进程
	transport.Close()
	cmd.Process.Kill()
	cmd.Wait()

	// 转换为 agent.SessionInfo
	sessions := make([]agent.SessionInfo, len(listResp.Sessions))
	for i, s := range listResp.Sessions {
		updatedAt, _ := time.Parse(time.RFC3339, s.UpdatedAt)
		sessions[i] = agent.SessionInfo{
			SessionID: s.SessionID,
			CWD:       s.CWD,
			Title:     s.Title,
			UpdatedAt: updatedAt,
		}
	}

	LogInfo("ACP: session/list completed",
		zap.String("cwd", cwd),
		zap.Int("count", len(sessions)))

	return sessions, nil
}

// SessionResume 恢复已有会话（不回放历史）
// 实现 agent.SessionResumeCapable 接口
// 返回新的进程内 session ID（用于后续 prompt）
func (a *BaseACPAdapter) SessionResume(ctx context.Context, acpSessionID string, cwd string, mcpServers []interface{}) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 启动新进程
	args := a.Config.BuildArgs(&agent.ExecutionRequest{WorkDir: cwd})
	cmd := exec.CommandContext(ctx, a.Config.CliPath, args...)
	hideCommandLineWindow(cmd)

	if cwd != "" {
		cmd.Dir = cwd
	}

	env := a.buildEnv(&agent.ExecutionRequest{WorkDir: cwd})
	cmd.Env = env

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("ACP: failed to create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("ACP: failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("ACP: failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("ACP: failed to start CLI process: %w", err)
	}

	// 生成内部 session ID
	internalSessionID := uuid.New().String()

	// 创建 session context
	sessionCtx, sessionCancel := context.WithCancel(context.Background())
	session := &acpSession{
		id:                 internalSessionID,
		isdpID:             internalSessionID,
		cmd:                cmd,
		ctx:                sessionCtx,
		cancel:             sessionCancel,
		status:             agent.SessionStatusRunning,
	 stdinPipe:          stdinPipe,
		outputUpdatedSignal: make(chan struct{}, 1),
	}

	// 启动 stderr 消费
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			LogInfo("ACP: stderr output (session/resume)",
				zap.String("sessionId", internalSessionID),
				zap.String("line", line))
			session.mu.Lock()
			if session.stderrOutput.Len() < maxStderrSize {
				session.stderrOutput.WriteString(line)
				session.stderrOutput.WriteString("\n")
			}
			session.mu.Unlock()
		}
	}()

	transport := newACPTransport(stdinPipe, stdoutPipe, func(method string, params json.RawMessage) {
		a.handleNotification(session, method, params, nil)
	})
	session.transport = transport
	transport.Start()

	// Initialize
	initResult, err := transport.SendRequest("initialize", &acpInitializeParams{
		ProtocolVersion:    2025,
		ClientCapabilities: acpClientCapabilities{},
	})
	if err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		transport.Close()
		cmd.Process.Kill()
		return "", fmt.Errorf("ACP: initialize handshake failed: %w\nstderr: %s", err, stderrContent)
	}

	var initResp acpInitializeResult
	if err := json.Unmarshal(initResult, &initResp); err != nil {
		LogWarn("ACP: initialize response parse warning", zap.Error(err))
	}

	// 发送 gateway authenticate（如果配置了第三方 API）
	if err := a.sendAuthenticate(transport); err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		transport.Close()
		cmd.Process.Kill()
		return "", fmt.Errorf("ACP: gateway authenticate failed: %w\nstderr: %s", err, stderrContent)
	}

	// SessionResume
	resumeResult, err := transport.SendRequest("session/resume", &acpSessionResumeParams{
		SessionID:  acpSessionID,
		CWD:        cwd,
		MCPServers: mcpServers,
	})
	if err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		transport.Close()
		cmd.Process.Kill()
		return "", fmt.Errorf("ACP: session/resume failed: %w\nstderr: %s", err, stderrContent)
	}

	var resumeResp acpSessionResumeResult
	if err := json.Unmarshal(resumeResult, &resumeResp); err != nil {
		LogWarn("ACP: session/resume response parse warning", zap.Error(err))
	}

	// 保存 session 到 sessions map
	a.sessions[internalSessionID] = session

	LogInfo("ACP: session/resume completed",
		zap.String("internalSessionId", internalSessionID),
		zap.String("acpSessionId", acpSessionID),
		zap.String("cwd", cwd))

	return internalSessionID, nil
}

// SessionLoad 加载已有会话（回放完整历史）
// 实现 agent.SessionResumeCapable 接口
// 注意：会通过 session/update 通知回放所有历史消息
func (a *BaseACPAdapter) SessionLoad(ctx context.Context, acpSessionID string, cwd string, mcpServers []interface{}) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 启动新进程
	args := a.Config.BuildArgs(&agent.ExecutionRequest{WorkDir: cwd})
	cmd := exec.CommandContext(ctx, a.Config.CliPath, args...)
	hideCommandLineWindow(cmd)

	if cwd != "" {
		cmd.Dir = cwd
	}

	env := a.buildEnv(&agent.ExecutionRequest{WorkDir: cwd})
	cmd.Env = env

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("ACP: failed to create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("ACP: failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("ACP: failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("ACP: failed to start CLI process: %w", err)
	}

	// 生成内部 session ID
	internalSessionID := uuid.New().String()

	// 创建 session context
	sessionCtx, sessionCancel := context.WithCancel(context.Background())
	session := &acpSession{
		id:                 internalSessionID,
		isdpID:             internalSessionID,
		cmd:                cmd,
		ctx:                sessionCtx,
		cancel:             sessionCancel,
		status:             agent.SessionStatusRunning,
		stdinPipe:          stdinPipe,
		outputUpdatedSignal: make(chan struct{}, 1),
	}

	// 启动 stderr 消费
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			LogInfo("ACP: stderr output (session/load)",
				zap.String("sessionId", internalSessionID),
				zap.String("line", line))
			session.mu.Lock()
			if session.stderrOutput.Len() < maxStderrSize {
				session.stderrOutput.WriteString(line)
				session.stderrOutput.WriteString("\n")
			}
			session.mu.Unlock()
		}
	}()

	transport := newACPTransport(stdinPipe, stdoutPipe, func(method string, params json.RawMessage) {
		a.handleNotification(session, method, params, nil)
	})
	session.transport = transport
	transport.Start()

	// Initialize
	initResult, err := transport.SendRequest("initialize", &acpInitializeParams{
		ProtocolVersion:    2025,
		ClientCapabilities: acpClientCapabilities{},
	})
	if err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		transport.Close()
		cmd.Process.Kill()
		return "", fmt.Errorf("ACP: initialize handshake failed: %w\nstderr: %s", err, stderrContent)
	}

	var initResp acpInitializeResult
	if err := json.Unmarshal(initResult, &initResp); err != nil {
		LogWarn("ACP: initialize response parse warning", zap.Error(err))
	}

	// 发送 gateway authenticate（如果配置了第三方 API）
	if err := a.sendAuthenticate(transport); err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		transport.Close()
		cmd.Process.Kill()
		return "", fmt.Errorf("ACP: gateway authenticate failed: %w\nstderr: %s", err, stderrContent)
	}

	// SessionLoad - 使用 session/load 而非 session/resume
	loadResult, err := transport.SendRequest("session/load", &acpSessionLoadParams{
		SessionID:  acpSessionID,
		CWD:        cwd,
		MCPServers: mcpServers,
	})
	if err != nil {
		session.mu.Lock()
		stderrContent := session.stderrOutput.String()
		session.mu.Unlock()
		transport.Close()
		cmd.Process.Kill()
		return "", fmt.Errorf("ACP: session/load failed: %w\nstderr: %s", err, stderrContent)
	}

	var loadResp acpSessionResumeResult // response structure is same
	if err := json.Unmarshal(loadResult, &loadResp); err != nil {
		LogWarn("ACP: session/load response parse warning", zap.Error(err))
	}

	// 保存 session 到 sessions map
	a.sessions[internalSessionID] = session

	LogInfo("ACP: session/load completed",
		zap.String("internalSessionId", internalSessionID),
		zap.String("acpSessionId", acpSessionID),
		zap.String("cwd", cwd))

	return internalSessionID, nil
}

// SessionClose 关闭会话
// 实现 agent.SessionResumeCapable 接口
func (a *BaseACPAdapter) SessionClose(ctx context.Context, acpSessionID string) error {
	// 注意：这里的 acpSessionID 是 ACP 协议的 session ID
	// 不是我们内部的 session ID
	// 需要启动临时进程来发送 session/close

	args := a.Config.BuildArgs(&agent.ExecutionRequest{})
	cmd := exec.CommandContext(ctx, a.Config.CliPath, args...)
	hideCommandLineWindow(cmd)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("ACP: failed to create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ACP: failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ACP: failed to start CLI process: %w", err)
	}

	transport := newACPTransport(stdinPipe, stdoutPipe, nil)
	transport.Start()

	// Initialize
	_, err = transport.SendRequest("initialize", &acpInitializeParams{
		ProtocolVersion:    2025,
		ClientCapabilities: acpClientCapabilities{},
	})
	if err != nil {
		transport.Close()
		cmd.Process.Kill()
		return fmt.Errorf("ACP: initialize handshake failed: %w", err)
	}

	// SessionClose
	_, err = transport.SendRequest("session/close", &acpSessionCloseParams{
		SessionID: acpSessionID,
	})
	if err != nil {
		transport.Close()
		cmd.Process.Kill()
		return fmt.Errorf("ACP: session/close failed: %w", err)
	}

	transport.Close()
	cmd.Process.Kill()
	cmd.Wait()

	LogInfo("ACP: session/close completed",
		zap.String("acpSessionId", acpSessionID))

	return nil
}

// ExecuteWithResume 使用 session/resume 执行
// 实现 agent.SessionResumeCapable 接口
// 如果 acpSessionID 不为空，先 resume 再发送 prompt
// 否则创建新 session
func (a *BaseACPAdapter) ExecuteWithResume(ctx context.Context, req *agent.ExecutionRequest, acpSessionID string, onChunk func(agent.Chunk)) (result *agent.ExecutionResult, newSessionID string, err error) {
	var acpSessID string

	if acpSessionID != "" {
		// Resume existing session - start process directly with correct handler
		// 关键修复：不调用 SessionResume，而是直接启动进程并发送完整流程
		// 这样可以确保 handler 从一开始就正确设置，不会丢失历史回放通知

		args := a.Config.BuildArgs(req)
		cmd := exec.CommandContext(ctx, a.Config.CliPath, args...)
		cmd.Dir = req.WorkDir

		// 设置环境变量，继承系统环境变量（特别是 PATH）
		// 并添加自定义环境变量（如 CLAUDE_CONFIG_DIR）
		cmd.Env = append(os.Environ(), a.Config.BuildEnv(req)...)

		stdinPipe, err := cmd.StdinPipe()
		if err != nil {
			return nil, "", fmt.Errorf("ACP: failed to create stdin pipe: %w", err)
		}
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return nil, "", fmt.Errorf("ACP: failed to create stdout pipe: %w", err)
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			return nil, "", fmt.Errorf("ACP: failed to create stderr pipe: %w", err)
		}

		if err := cmd.Start(); err != nil {
			return nil, "", fmt.Errorf("ACP: failed to start CLI process: %w", err)
		}

		// 生成内部 session ID
		internalSessionID := uuid.New().String()

		// 创建 session context
		sessionCtx, sessionCancel := context.WithCancel(context.Background())
		session := &acpSession{
			id:                  internalSessionID,
			isdpID:              internalSessionID,
			cmd:                 cmd,
			ctx:                 sessionCtx,
			cancel:              sessionCancel,
			status:              agent.SessionStatusRunning,
			stdinPipe:           stdinPipe,
			outputUpdatedSignal: make(chan struct{}, 1),
		}

		// 启动 stderr 消费
		go func() {
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				line := scanner.Text()
				LogInfo("ACP: stderr output (ExecuteWithResume)",
					zap.String("sessionId", internalSessionID),
					zap.String("line", line))
				session.mu.Lock()
				if session.stderrOutput.Len() < maxStderrSize {
					session.stderrOutput.WriteString(line)
					session.stderrOutput.WriteString("\n")
				}
				session.mu.Unlock()
			}
		}()

		// 关键：创建 transport 时使用正确的 handler（带 onChunk）
		// 这样 session/resume 的历史回放通知会被正确处理
		transport := newACPTransport(stdinPipe, stdoutPipe, func(method string, params json.RawMessage) {
			a.handleNotification(session, method, params, onChunk)
			// Signal output update
			session.mu.Lock()
			if session.outputUpdatedSignal != nil {
				select {
				case session.outputUpdatedSignal <- struct{}{}:
				default:
				}
			}
			session.mu.Unlock()
		})
		session.transport = transport
		transport.Start()

		// 保存 session 到 sessions map
		a.mu.Lock()
		a.sessions[internalSessionID] = session
		a.mu.Unlock()

		// Initialize
		initResult, err := transport.SendRequest("initialize", &acpInitializeParams{
			ProtocolVersion:    2025,
			ClientCapabilities: acpClientCapabilities{},
		})
		if err != nil {
			session.mu.Lock()
			stderrContent := session.stderrOutput.String()
			session.mu.Unlock()
			transport.Close()
			cmd.Process.Kill()
			cmd.Wait()
			a.mu.Lock()
			delete(a.sessions, internalSessionID)
			a.mu.Unlock()
			return nil, "", fmt.Errorf("ACP: initialize handshake failed: %w\nstderr: %s", err, stderrContent)
		}

		var initResp acpInitializeResult
		if err := json.Unmarshal(initResult, &initResp); err != nil {
			LogWarn("ACP: initialize response parse warning", zap.Error(err))
		}

		// 发送 gateway authenticate（如果配置了第三方 API）
		if err := a.sendAuthenticate(transport); err != nil {
			session.mu.Lock()
			stderrContent := session.stderrOutput.String()
			session.mu.Unlock()
			transport.Close()
			cmd.Process.Kill()
			cmd.Wait()
			a.mu.Lock()
			delete(a.sessions, internalSessionID)
			a.mu.Unlock()
			return nil, "", fmt.Errorf("ACP: gateway authenticate failed: %w\nstderr: %s", err, stderrContent)
		}

		// SessionResume - 使用 session/resume（不回放历史）
		// session/resume 不回放历史消息，只继承上下文
		// session/load 会回放完整历史消息给客户端
		mcpServers := a.buildMCPServers(req)
		mcpServersJSON, _ := json.Marshal(mcpServers)
		LogInfo("ACP: session/resume mcpServers", zap.String("mcpServers", string(mcpServersJSON)))
		resumeResult, err := transport.SendRequest("session/resume", &acpSessionResumeParams{
			SessionID:  acpSessionID,
			CWD:        req.WorkDir,
			MCPServers: mcpServers,
		})
		if err != nil {
			session.mu.Lock()
			stderrContent := session.stderrOutput.String()
			session.mu.Unlock()
			transport.Close()
			cmd.Process.Kill()
			cmd.Wait()
			a.mu.Lock()
			delete(a.sessions, internalSessionID)
			a.mu.Unlock()
			return nil, "", fmt.Errorf("ACP: session/resume failed: %w\nstderr: %s", err, stderrContent)
		}

		var resumeResp acpSessionResumeResult // response structure is same
		if err := json.Unmarshal(resumeResult, &resumeResp); err != nil {
			LogWarn("ACP: session/resume response parse warning", zap.Error(err))
		}

		LogInfo("ACP: ExecuteWithResume session/resume completed",
			zap.String("internalSessionId", internalSessionID),
			zap.String("acpSessionId", acpSessionID),
			zap.String("cwd", req.WorkDir))

		acpSessID = acpSessionID

		// Send prompt to resumed session
		prompt := a.buildPromptFromRequest(req)
		_, promptErr := transport.SendRequest("session/prompt", &acpPromptParams{
			SessionID: acpSessionID,
			Prompt:    []acpContentBlock{{Type: "text", Text: prompt}},
		})
		if promptErr != nil {
			return nil, acpSessID, fmt.Errorf("ACP: session/prompt failed: %w", promptErr)
		}

		// Wait for notifications to be processed
		session.mu.Lock()
		signal := session.outputUpdatedSignal
		currentOutputLen := session.output.Len()
		session.mu.Unlock()

		if signal != nil && currentOutputLen == 0 {
			select {
			case <-signal:
				// Wait for more updates with backoff
				noUpdateCount := 0
				for noUpdateCount < 3 {
					session.mu.Lock()
					newLen := session.output.Len()
					session.mu.Unlock()
					if newLen > currentOutputLen {
						currentOutputLen = newLen
						noUpdateCount = 0
					} else {
						noUpdateCount++
					}
					select {
					case <-signal:
					case <-time.After(100 * time.Millisecond):
						break
					}
				}
			case <-time.After(500 * time.Millisecond):
				LogWarn("ACP: timeout waiting for output updates (ExecuteWithResume)",
					zap.String("acpSessionId", acpSessionID))
			}
		} else if signal != nil {
			select {
			case <-signal:
			case <-time.After(300 * time.Millisecond):
			}
		}

		// Clean up signal
		session.mu.Lock()
		session.outputUpdatedSignal = nil
		output := session.output.String()
		session.mu.Unlock()

		result = &agent.ExecutionResult{
			Output:    output,
			SessionID: acpSessID,
		}

		return result, acpSessID, nil
	}

	// Create new session - use regular ExecuteWithStream
	result, err = a.ExecuteWithStream(ctx, req, onChunk)
	if err != nil {
		return nil, "", fmt.Errorf("ACP: execute failed: %w", err)
	}
	acpSessID = result.SessionID

	LogInfo("ACP: ExecuteWithResume using new session",
		zap.String("acpSessionId", acpSessID))

	return result, acpSessID, nil
}

// sendAuthenticate 发送 gateway authenticate 请求
// 用于配置第三方 API（如阿里云百炼）
// 必须在 initialize 成功后调用
func (a *BaseACPAdapter) sendAuthenticate(transport *acpTransport) error {
	// 如果没有配置 gateway，跳过
	if a.Config.GatewayBaseURL == "" {
		return nil
	}

	LogInfo("ACP: sending gateway authenticate",
		zap.String("baseUrl", a.Config.GatewayBaseURL))

	// 构建 headers map
	headers := a.Config.GatewayHeaders
	if headers == nil {
		headers = make(map[string]string)
	}

	// 发送 authenticate 请求
	_, err := transport.SendRequest("authenticate", &acpAuthenticateParams{
		MethodId: "gateway",
		Meta: &acpGatewayMeta{
			Gateway: acpGatewayConfig{
				BaseURL: a.Config.GatewayBaseURL,
				Headers: headers,
			},
		},
	})

	if err != nil {
		LogError("ACP: gateway authenticate failed", zap.Error(err))
		return fmt.Errorf("ACP: gateway authenticate failed: %w", err)
	}

	LogInfo("ACP: gateway authenticate success")
	return nil
}
