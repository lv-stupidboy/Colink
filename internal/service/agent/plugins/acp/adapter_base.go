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
	"net/http"
	"net/url"
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
	// ModelRef 返回设置模型时使用的引用字符串。某些 CLI（如 OpenCode）要求 "provider/model"
	// 格式，与配置中注册的 provider 前缀一致。为 nil 时直接使用 baseAgent.DefaultModel。
	ModelRef func() string
	// ModelRefForSetModel 返回 session/set_model 调用时使用的模型引用（provider/model 格式）。
	// 1.3.3 等旧版 OpenCode 只有 session/set_model 没有 session/set_config_option，
	// 而 set_model 的 parseModelSelection 需要 provider/model 格式才能正确解析。
	// 为 nil 时回退到 ModelRef → baseAgent.DefaultModel。
	ModelRefForSetModel func() string
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
	id              string
	isdpID          string
	transport       *acpTransport
	stdinPipe       io.WriteCloser // stdin 管道引用（用于检测断开）
	cmd             *exec.Cmd
	ctx             context.Context
	cancel          context.CancelFunc
	status          agent.SessionStatus
	cwd             string // 工作目录（用于 OpenCode HTTP API 调用等）
	output          strings.Builder
	stderrOutput    strings.Builder // stderr 输出缓冲（用于错误诊断）
	pendingQuestion *agent.Chunk    // 待处理的 AskUserQuestion（等待用户响应）
	// pendingElicitationID 是 ACP unstable elicitation/create 反向请求的 RPC id，
	// 收到请求后我们暂存它，等前端回答后再用 SendResponse 把 elicitation 结果回给 CLI。
	pendingElicitationID        interface{}
	pendingElicitationQuestions []agent.QuestionItem // 与请求中 question_<n> 一一对应，按下标回填
	toolCallNames               map[string]string    // 工具调用ID到名称的映射（用于tool_call_update时查找）
	thoughtChunkCount           int                  // 流式思考内容计数器（用于采样打印）
	// 诊断字段（info 级别可见，用于捕捉无限循环问题）
	notificationCount    int    // 收到的通知总数
	duplicateUpdateCount int    // 连续重复通知计数
	lastUpdateHash       string // 最后一次 session/update 的内容哈希（前16位）
	// replayPhase 标记当前是否处在 session/resume 之后、session/prompt 之前的"历史回放"阶段。
	// OpenCode 的 ACP 协议在 resume 时会用普通 session/update 通知把整段历史 chunk 重推一遍
	// （没有 isHistory 标志），唯一可靠的区分手段就是阶段：resume 完成后的所有通知都视为
	// 历史回放，发送 session/prompt 时切换为 false，之后的通知才是真正的新输出。
	replayPhase bool
	mu          sync.Mutex
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
		cwd:    req.WorkDir,
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
			// 推送用户可感知的 stderr（限流、重试等）到前端，做瞬时状态展示
			if onChunk != nil && agent.ShouldNotifyStderr(line) {
				onChunk(agent.Chunk{Type: agent.ChunkTypeError, Content: line})
			}
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
	transport.SetServerRequestHandler(func(id interface{}, method string, params json.RawMessage) {
		a.handleServerRequest(session, id, method, params, onChunk)
	})
	session.transport = transport
	transport.Start()

	initResult, err := transport.SendRequest("initialize", &acpInitializeParams{
		ProtocolVersion: 2025,
		ClientCapabilities: acpClientCapabilities{
			Elicitation: &acpElicitationCapabilities{Form: &struct{}{}},
		},
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

	// 构建 MCP Servers 配置
	mcpServers := a.buildMCPServers(req)
	mcpServersJSON, _ := json.Marshal(mcpServers)
	LogInfo("ACP: session/new mcpServers", zap.Int("protocolVersion", initResp.ProtocolVersion), zap.String("mcpServers", string(mcpServersJSON)))

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
		cwd:    req.WorkDir,
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
	transport.SetServerRequestHandler(func(id interface{}, method string, params json.RawMessage) {
		a.handleServerRequest(session, id, method, params, nil)
	})
	session.transport = transport
	transport.Start()

	_, err = transport.SendRequest("initialize", &acpInitializeParams{
		ProtocolVersion: 2025,
		ClientCapabilities: acpClientCapabilities{
			Elicitation: &acpElicitationCapabilities{Form: &struct{}{}},
		},
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
	mcpServersJSON, _ := json.Marshal(mcpServers)
	LogInfo("ACP: StartSession session/new mcpServers", zap.String("sessionID", sessionID), zap.String("mcpServers", string(mcpServersJSON)))

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
		ProtocolVersion: 2025,
		ClientCapabilities: acpClientCapabilities{
			Elicitation: &acpElicitationCapabilities{Form: &struct{}{}},
		},
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
			zap.String("sessionId", session.id),
			zap.String("paramsPreview", string(params)[:min(200, len(params))]))
	}

	switch method {
	case "session/update":
		if onChunk == nil {
			return
		}

		// 历史回放过滤：session/resume 触发的回放在协议中没有任何标志位，
		// 但它们必然出现在 session/prompt 发出之前。replayPhase 由 ExecuteWithResume
		// 在 resume 完成后置 true、prompt 发出前置 false 来标记这段窗口。
		// 这段窗口内的 chunk 是 CLI 内部上下文（用于让模型 KV 缓存恢复），
		// 既不能写到我们的 output buffer（会污染本轮回答），
		// 也不能 onChunk 广播（前端会重复显示历史）。
		session.mu.Lock()
		isReplay := session.replayPhase
		session.mu.Unlock()
		if isReplay {
			LogDebug("ACP: skip replay-phase session/update",
				zap.String("sessionId", session.id))
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
		// 同 session/update：回放阶段的工具调用通知吞掉，避免历史工具调用重复展示
		session.mu.Lock()
		isReplay := session.replayPhase
		session.mu.Unlock()
		if isReplay {
			LogDebug("ACP: skip replay-phase session/tool_call_update",
				zap.String("sessionId", session.id))
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
			chunks, err := parseACPToolCallUpdate(params, session)
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
			chunks, err := parseACPToolCallUpdate(updateParams.Update, session)
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

// handleServerRequest 处理服务端发起的 request（如 session/request_permission、
// elicitation/create）。这是 ACP 协议中服务端向客户端发送的 request，客户端必须回 response。
//
// onChunk 用于把 elicitation 转换成 question chunk 推给前端；nil 时跳过 elicitation 处理。
func (a *BaseACPAdapter) handleServerRequest(session *acpSession, id interface{}, method string, params json.RawMessage, onChunk func(agent.Chunk)) {
	LogInfo("ACP: received server request",
		zap.Any("id", id),
		zap.String("method", method),
		zap.String("sessionId", session.id),
		zap.String("params", string(params)))

	switch method {
	case "session/request_permission":
		// ACP 协议：params 中带 options 数组，每项形如
		//   {"optionId": "...", "name": "...", "kind": "allow_always|allow_once|reject_once|reject_always"}
		// optionId 的字面值由 CLI 自定（OpenCode 用连字符 "allow-always"，
		// Claude CLI 可能不一样），所以必须从 params 里拿真实 optionId，
		// 不能再硬编码字符串。这里按 kind 优先级 allow_always > allow_once 选取。
		if session.transport == nil {
			return
		}

		var perm struct {
			Options []struct {
				OptionID string `json:"optionId"`
				Kind     string `json:"kind"`
				Name     string `json:"name"`
			} `json:"options"`
		}
		_ = json.Unmarshal(params, &perm)

		var chosenID string
		for _, opt := range perm.Options {
			if opt.Kind == "allow_always" {
				chosenID = opt.OptionID
				break
			}
		}
		if chosenID == "" {
			for _, opt := range perm.Options {
				if opt.Kind == "allow_once" {
					chosenID = opt.OptionID
					break
				}
			}
		}
		if chosenID == "" {
			chosenID = "allow_always"
			LogWarn("ACP: request_permission has no allow option, falling back",
				zap.Int("optionsCount", len(perm.Options)))
		}

		response := map[string]interface{}{
			"outcome": map[string]interface{}{
				"outcome":  "selected",
				"optionId": chosenID,
			},
		}
		if err := session.transport.SendResponse(id, response); err != nil {
			LogError("ACP: failed to send permission response", zap.Error(err))
		} else {
			LogInfo("ACP: permission auto-approved via response",
				zap.Any("requestId", id),
				zap.String("chosenOptionId", chosenID))
		}

	case "elicitation/create":
		// ACP unstable elicitation 协议（form 模式）。claude-agent-acp 把 AskUserQuestion
		// 工具翻译成此请求；params 形如：
		//   {
		//     "mode": "form",
		//     "sessionId": "...",
		//     "toolCallId": "...",        // optional
		//     "message": "...",
		//     "requestedSchema": { "type":"object","properties": { "question_<n>": {...} } }
		//   }
		//
		// **设计：异步等待用户答案**（贴合 ACP 协议本意）
		//
		// 1. 推 question chunk 给前端展示选择框；
		// 2. 缓存 RPC id 与 questions 到 session，**不立即 SendResponse**；
		// 3. invocation goroutine 继续阻塞在 SendRequest("session/prompt") 上；
		// 4. 用户答完 → SendToolResult 用 {action:"accept", content:{...}} 把答案
		//    SendResponse 给 CLI，CLI 端 AskUserQuestion 工具收到 updatedInput 后继续
		//    在同一 prompt turn 内推后续 chunk。
		//
		// 期间 invocation 状态保持 running、isStreaming=true；前端按钮可点性由
		// question block 自身 status='waiting_user_input' 决定（已去掉 !agentRunning 限制）。
		if session.transport == nil || onChunk == nil {
			if session.transport != nil {
				_ = session.transport.SendResponse(id, map[string]interface{}{"action": "cancel"})
			}
			return
		}

		var elicit struct {
			Mode            string `json:"mode"`
			Message         string `json:"message"`
			ToolCallID      string `json:"toolCallId"`
			RequestedSchema struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"requestedSchema"`
		}
		if err := json.Unmarshal(params, &elicit); err != nil {
			LogError("ACP: failed to parse elicitation/create params", zap.Error(err))
			_ = session.transport.SendResponse(id, map[string]interface{}{"action": "cancel"})
			return
		}
		if elicit.Mode != "form" {
			LogWarn("ACP: elicitation mode not supported, declining",
				zap.String("mode", elicit.Mode))
			_ = session.transport.SendResponse(id, map[string]interface{}{"action": "decline"})
			return
		}

		questions := parseElicitationQuestions(elicit.RequestedSchema.Properties, elicit.Message)
		if len(questions) == 0 {
			LogWarn("ACP: elicitation has no questions, declining")
			_ = session.transport.SendResponse(id, map[string]interface{}{"action": "decline"})
			return
		}

		toolCallID := elicit.ToolCallID
		if toolCallID == "" {
			toolCallID = fmt.Sprintf("elicit-%v", id)
		}

		session.mu.Lock()
		session.pendingElicitationID = id
		session.pendingElicitationQuestions = questions
		session.mu.Unlock()
		// 注册到 sessions map 让 SubmitQuestionAnswer → SendToolResult 找得到
		if session.isdpID != "" {
			a.mu.Lock()
			a.sessions[session.isdpID] = session
			a.mu.Unlock()
		}

		chunk := agent.Chunk{
			Type:      agent.ChunkTypeQuestion,
			ToolName:  "AskUserQuestion",
			ToolID:    toolCallID,
			Questions: questions,
		}
		session.mu.Lock()
		session.pendingQuestion = &chunk
		session.mu.Unlock()
		onChunk(chunk)

		LogInfo("ACP: elicitation/create dispatched, awaiting async answer",
			zap.Any("requestId", id),
			zap.String("toolCallId", toolCallID),
			zap.Int("questionsCount", len(questions)))

	default:
		LogWarn("ACP: unknown server request method",
			zap.String("method", method),
			zap.Any("id", id))
	}
}

// elicitationCustomOptionLabel 是我们注入到每个问题 options 末尾的"自定义答案"占位项。
//
// 前端 QuestionBlock.tsx 的 `optionNeedsCustomInput` 检测 label 是否含 "其他"/"自定义"，
// 命中就额外渲染一个文本框让用户填自定义答案。claude-agent-acp 的 AskUserQuestion 不会把
// "Other" 当 enum 选项放进 oneOf——它单独存到 question_<n>_custom 字段——所以我们必须在
// 前端可见的 options 里手动追加这个占位选项。
//
// SendToolResult 收到答案时再用此常量识别"用户选了占位项还填了文本框"，把答案塞进
// question_<n>_custom（claude-agent-acp 的 elicitation.applyAskElicitationResponse 优先
// 读 _custom 字段）。
const elicitationCustomOptionLabel = "其他（请填写自定义答案）"

// parseElicitationQuestions 把 elicitation/create 的 requestedSchema.properties 还原为
// agent.QuestionItem[]。仅识别形如 question_<n> 的字段（同时跳过 question_<n>_custom）。
// 每个问题的 options 末尾会被追加一个 elicitationCustomOptionLabel 占位项。
func parseElicitationQuestions(props map[string]json.RawMessage, fallbackMessage string) []agent.QuestionItem {
	type enumOption struct {
		Const string                 `json:"const"`
		Title string                 `json:"title"`
		Meta  map[string]interface{} `json:"_meta"`
	}
	type fieldSchema struct {
		Type        string       `json:"type"`        // "string" 单选 / "array" 多选
		Title       string       `json:"title"`       // QuestionItem.Header
		Description string       `json:"description"` // QuestionItem.Question（多问题时）
		OneOf       []enumOption `json:"oneOf"`
		Items       *struct {
			AnyOf []enumOption `json:"anyOf"`
		} `json:"items"`
	}

	indices := make([]int, 0, len(props))
	for k := range props {
		var idx int
		if _, err := fmt.Sscanf(k, "question_%d", &idx); err != nil {
			continue
		}
		// 排除 question_<n>_custom 等带后缀的字段
		if k != fmt.Sprintf("question_%d", idx) {
			continue
		}
		indices = append(indices, idx)
	}
	for i := 1; i < len(indices); i++ {
		for j := i; j > 0 && indices[j-1] > indices[j]; j-- {
			indices[j-1], indices[j] = indices[j], indices[j-1]
		}
	}

	out := make([]agent.QuestionItem, 0, len(indices))
	for _, idx := range indices {
		raw := props[fmt.Sprintf("question_%d", idx)]
		var f fieldSchema
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}

		var enumOpts []enumOption
		multi := false
		switch f.Type {
		case "array":
			multi = true
			if f.Items != nil {
				enumOpts = f.Items.AnyOf
			}
		default:
			enumOpts = f.OneOf
		}

		options := make([]agent.QuestionOption, 0, len(enumOpts))
		for _, eo := range enumOpts {
			opt := agent.QuestionOption{
				Label: eo.Const,
			}
			// _meta 里 claude-agent-acp 用 _claude/askUserQuestionOption key 携带结构化 description / preview
			if detailRaw, ok := eo.Meta["_claude/askUserQuestionOption"]; ok {
				if detail, ok := detailRaw.(map[string]interface{}); ok {
					if d, _ := detail["description"].(string); d != "" {
						opt.Description = d
					}
					if p, _ := detail["preview"].(string); p != "" {
						opt.Preview = p
					}
				}
			}
			// fallback：从 title "label — description" 拆出 description
			if opt.Description == "" && eo.Title != "" && eo.Title != opt.Label {
				if sep := " — "; len(eo.Title) > len(opt.Label)+len(sep) && eo.Title[:len(opt.Label)] == opt.Label {
					opt.Description = eo.Title[len(opt.Label)+len(sep):]
				}
			}
			options = append(options, opt)
		}

		// 追加"自定义答案"占位选项 —— 前端识别 label 含"其他"后会自动渲染输入框，
		// 让用户填一段自定义文本。SendToolResult 端会把这段文本写到 question_<n>_custom。
		options = append(options, agent.QuestionOption{
			Label:       elicitationCustomOptionLabel,
			Description: "上面选项都不合适？请填写你自己的答案。",
		})

		question := f.Description
		if question == "" {
			// 单问题场景下 description 为空，问题文本由 elicitation.message 承载
			question = fallbackMessage
		}
		out = append(out, agent.QuestionItem{
			Header:      f.Title,
			Question:    question,
			MultiSelect: multi,
			Options:     options,
		})
	}
	return out
}

func (a *BaseACPAdapter) buildPromptFromRequest(req *agent.ExecutionRequest) string {
	return agent.BuildPromptFromRequest(req)
}

// buildContentBlocks 构建内容块列表（文本 + 图片）
func (a *BaseACPAdapter) buildContentBlocks(text string, images []model.ImageContent) []acpContentBlock {
	blocks := []acpContentBlock{{Type: "text", Text: text}}

	// 添加图片内容块：ACP ImageContent 要求顶层 mimeType + data（base64），
	// 不是 Anthropic Messages API 的嵌套 source 格式
	for _, img := range images {
		blocks = append(blocks, acpContentBlock{
			Type:     "image",
			MimeType: img.MimeType,
			Data:     img.Data,
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

// buildMCPServers 构建 MCP server 配置数组。
// 用于在 session/new 时注入显式绑定的 MCP servers、用户级 MCP 配置和 Colink memory MCP server。
func (a *BaseACPAdapter) buildMCPServers(req *agent.ExecutionRequest) []interface{} {
	if req == nil {
		return []interface{}{}
	}
	mcpServers := convertManagedMCPToACPFormat(req.MCPServers)

	if a.Config.LoadUserMCPConfig != nil {
		userMCP := a.Config.LoadUserMCPConfig()
		if len(userMCP) > 0 {
			mcpServers = append(mcpServers, convertUserMCPToACPFormat(userMCP)...)
			serverNames := make([]string, 0, len(userMCP))
			for name := range userMCP {
				serverNames = append(serverNames, name)
			}
			LogInfo("ACP: Loaded user MCP servers", zap.Strings("servers", serverNames))
		}
	}

	// 如果没有必要的参数，不注入任何 MCP
	if req.CallbackToken == "" || req.APIURL == "" || req.InvocationID == uuid.Nil {
		return mcpServers
	}

	// 获取平台 MCP server 可执行文件路径
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
			return mcpServers
		}
		return []interface{}{}
	}

	LogInfo("ACP: MCP server path", zap.String("path", mcpServerPath))

	// 构建 memory MCP server 配置
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

func convertManagedMCPToACPFormat(servers []*model.MCPServer) []interface{} {
	if len(servers) == 0 {
		return []interface{}{}
	}
	result := make([]interface{}, 0, len(servers))
	for _, server := range servers {
		if server == nil || server.Status == model.MCPStatusDisabled {
			continue
		}
		acpServer := map[string]interface{}{
			"name": server.Name,
		}
		switch server.Transport {
		case model.MCPTransportHTTP, model.MCPTransportSSE:
			acpServer["type"] = string(server.Transport)
			acpServer["url"] = server.URL
			acpServer["headers"] = mapToACPNameValueArray(server.Headers)
		default:
			acpServer["command"] = server.Command
			acpServer["args"] = server.Args
			acpServer["env"] = mapToACPNameValueArray(server.Env)
		}
		result = append(result, acpServer)
	}
	return result
}

func mapToACPNameValueArray(values map[string]string) []map[string]string {
	if len(values) == 0 {
		return []map[string]string{}
	}
	result := make([]map[string]string, 0, len(values))
	for name, value := range values {
		result = append(result, map[string]string{
			"name":  name,
			"value": value,
		})
	}
	return result
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
	// 某些 CLI 要求带 provider 前缀的模型引用（如 OpenCode 的 "colink/qwen3.7-plus"），
	// 否则会把裸模型名当成 provider 解析，导致 ProviderModelNotFoundError
	if a.Config.ModelRef != nil {
		if ref := a.Config.ModelRef(); ref != "" {
			desiredModel = ref
		}
	}

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

// SendToolResult 把用户答案回传给 CLI。两条路径自动切换：
//
//  1. **Elicitation 路径**（claude-agent-acp 等用 elicitation/create 反向请求实现 AskUserQuestion）：
//     session 上有 pendingElicitationID 时，按 ACP elicitation 协议构造
//     {action:"accept", content:{question_<n>: label}} 并 SendResponse 给那个待响应的 RPC id；
//     CLI 端 SDK 会把 content 当作 AskUserQuestion 工具的 updatedInput 继续推进 prompt turn。
//  2. **Resolve tool call 路径**（OpenCode 等通过 session/request_user_input 通知触发的工具回调）：
//     调用 session/resolve_tool_call 把答案带 toolCallId 发回去；失败再降级为
//     session/resolve_user_input 通知。
//
// `result` 一般是单个 label 字符串；多题场景前端可传 JSON 对象（`{"question_0":"a","question_1":"b"}`），
// 内部会原样作为 elicitation content 透传。
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

	// === 路径 1：elicitation/create 待响应 ===
	session.mu.Lock()
	pendingID := session.pendingElicitationID
	pendingQs := session.pendingElicitationQuestions
	session.mu.Unlock()

	if pendingID != nil {
		content := buildElicitationContent(result, pendingQs)
		response := map[string]interface{}{
			"action":  "accept",
			"content": content,
		}

		LogInfo("ACP: sending elicitation response",
			zap.String("invocationID", invocationID.String()),
			zap.Any("requestId", pendingID),
			zap.Any("content", content))

		if err := session.transport.SendResponse(pendingID, response); err != nil {
			return fmt.Errorf("ACP: failed to send elicitation response: %w", err)
		}

		// 清除待响应状态，避免下一次答案误触
		session.mu.Lock()
		session.pendingElicitationID = nil
		session.pendingElicitationQuestions = nil
		session.pendingQuestion = nil
		session.mu.Unlock()

		LogInfo("ACP: elicitation response sent successfully",
			zap.Any("requestId", pendingID))
		return nil
	}

	// === 路径 2：session/resolve_tool_call 兼容 OpenCode 等 ===
	//
	// 如果是 JSON 格式（前端 ACP 路径的 question 编码），尝试先通过 OpenCode HTTP
	// API 直接 reply question 工具。OpenCode ACP 模式下 question.asked 事件没有
	// 桥接到 ACP——question.ask() 创建 Deferred 后永远等不到 reply()，session/resolve_tool_call
	// 发给 ACP 也是被静默丢弃。而 OpenCode 的 HTTP API 端口 26307 完整暴露了
	// /question REPL API（启动时 --port 固定）。
	//
	// 如果 HTTP API 不可用（非 OpenCode 或端口不通），自动回退到原有 session/resolve_tool_call
	// 兼容路径。
	if strings.HasPrefix(result, "{") && session.cwd != "" {
		if err := openCodeQuestionReply(session.cwd, toolCallID, result); err != nil {
			LogWarn("ACP: OpenCode question reply via HTTP failed, falling back to resolve_tool_call",
				zap.Error(err),
				zap.String("cwd", session.cwd),
				zap.String("toolCallId", toolCallID))
		} else {
			LogInfo("ACP: OpenCode question reply via HTTP succeeded",
				zap.String("toolCallId", toolCallID))
			return nil
		}
	}

	plainResponse := flattenJSONAnswerForLegacy(result)
	resolveParams := map[string]interface{}{
		"toolCallId": toolCallID,
		"response":   plainResponse,
	}

	LogInfo("ACP: sending tool result via session/resolve_tool_call",
		zap.String("invocationID", invocationID.String()),
		zap.String("toolCallId", toolCallID),
		zap.String("response", plainResponse))

	_, err := session.transport.SendRequest("session/resolve_tool_call", resolveParams)
	if err != nil {
		LogWarn("ACP: session/resolve_tool_call request failed, trying notification",
			zap.Error(err))

		err = session.transport.SendNotification("session/resolve_user_input", &acpUserInputResponse{
			ToolCallID: toolCallID,
			Response:   plainResponse,
		})
		if err != nil {
			return fmt.Errorf("ACP: failed to send user input response: %w", err)
		}
	}

	LogInfo("ACP: tool result sent successfully",
		zap.String("toolCallId", toolCallID))
	return nil
}

// flattenJSONAnswerForLegacy 把前端为 elicitation/create 路径准备的
// `{"question_<n>": ...}` JSON 字符串展平为换行分隔的纯文本，供 OpenCode 等
// session/resolve_tool_call 路径使用——那条协议把 response 字段当作"用户答案
// 纯文本"直接塞给模型，不做结构解析。
//
// 规则：
//   - 不是 JSON 对象 → 原样返回（兼容真正的纯文本答案）
//   - JSON 对象 → 按 question_0、question_1 ... 顺序展平
//   - 字符串值直接拼；数组（多选）用顿号拼接；其它类型 fmt.Sprint
//   - 多题用换行连接
func flattenJSONAnswerForLegacy(answer string) string {
	trimmed := strings.TrimSpace(answer)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return answer
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil || parsed == nil {
		return answer
	}

	// 收集 question_<n> 索引；忽略 question_<n>_custom 等带后缀的字段（虽然
	// 现在前端不再发 _custom，保留容错）和其它非约定 key。
	indices := make([]int, 0, len(parsed))
	for k := range parsed {
		var idx int
		if _, err := fmt.Sscanf(k, "question_%d", &idx); err != nil {
			continue
		}
		if k != fmt.Sprintf("question_%d", idx) {
			continue
		}
		indices = append(indices, idx)
	}
	if len(indices) == 0 {
		return answer
	}
	// 插入排序（题数一般 ≤ 4）
	for i := 1; i < len(indices); i++ {
		for j := i; j > 0 && indices[j-1] > indices[j]; j-- {
			indices[j-1], indices[j] = indices[j], indices[j-1]
		}
	}

	parts := make([]string, 0, len(indices))
	for _, idx := range indices {
		v := parsed[fmt.Sprintf("question_%d", idx)]
		switch x := v.(type) {
		case string:
			if s := strings.TrimSpace(x); s != "" {
				parts = append(parts, s)
			}
		case []interface{}:
			ss := make([]string, 0, len(x))
			for _, e := range x {
				if s, ok := e.(string); ok {
					if t := strings.TrimSpace(s); t != "" {
						ss = append(ss, t)
					}
				} else if e != nil {
					ss = append(ss, fmt.Sprint(e))
				}
			}
			if len(ss) > 0 {
				parts = append(parts, strings.Join(ss, "、"))
			}
		case nil:
			// 该题用户未作答，skip
		default:
			parts = append(parts, fmt.Sprint(v))
		}
	}
	if len(parts) == 0 {
		return answer
	}
	return strings.Join(parts, "\n")
}

// buildElicitationContent 把前端提交的答案字符串还原为 elicitation 的 content map。
//
// 支持两种输入：
//   - JSON 对象（多题场景）：直接当 content（{"question_0":"a","question_1":"b"}）
//   - 普通字符串（单题场景）：塞到 question_0
//
// 注意 claude-agent-acp 的 applyAskElicitationResponse 只读 question_<n>（用
// String(value) 转文字写到 answers，不校验 enum），所以无论用户选的是 enum 选项还是
// 填了自定义文本，都直接塞 question_<n>。SDK 端的 form-level "customAnswer" 字段会
// 写到工具的 response 字段而非某题的 answer，无法表达"对第 N 题的自定义答案"的语义。
func buildElicitationContent(answer string, questions []agent.QuestionItem) map[string]interface{} {
	trimmed := strings.TrimSpace(answer)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil && parsed != nil {
			return parsed
		}
	}
	return map[string]interface{}{
		"question_0": answer,
	}
}

// openCodeQuestionReply 通过 OpenCode HTTP API (端口 26307) 回复 question 工具的用户答案。
//
// OpenCode 在 ACP 模式下，question.asked 事件没有被桥接——question.ask() 内部创建一个
// Deferred 阻塞等待 reply()，但 ACP event handler 不监听 question.asked。唯一的解锁途径是
// 调 OpenCode HTTP API 的 POST /question/{requestID}/reply。
//
// 流程：
//  1. GET  /question?directory={cwd} → 列出所有 pending question，找到 tool.callID 匹配项
//  2. 将 JSON {"question_0":"a","question_1":["b","c"]} 转为 OpenCode reply 格式的
//     {"answers":[["a"],["b","c"]]}
//  3. POST /question/{requestID}/reply?directory={cwd}
func openCodeQuestionReply(cwd, toolCallID, jsonAnswer string) error {
	baseURL := "http://127.0.0.1:26307"
	dirParam := url.QueryEscape(cwd)

	// 1. GET /question?directory={cwd} — 列出 pending questions
	listURL := fmt.Sprintf("%s/question?directory=%s", baseURL, dirParam)
	resp, err := http.Get(listURL)
	if err != nil {
		return fmt.Errorf("GET /question: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GET /question returned %d: %s", resp.StatusCode, string(body))
	}

	var pendingQuestions []struct {
		ID        string `json:"id"`
		SessionID string `json:"sessionID"`
		Tool      *struct {
			CallID string `json:"callID"`
		} `json:"tool"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pendingQuestions); err != nil {
		return fmt.Errorf("parse /question response: %w", err)
	}

	var requestID string
	for _, q := range pendingQuestions {
		if q.Tool != nil && q.Tool.CallID == toolCallID {
			requestID = q.ID
			break
		}
	}
	if requestID == "" {
		return fmt.Errorf("no pending question found with tool.callID=%s (total %d pending)", toolCallID, len(pendingQuestions))
	}

	// 2. JSON → OpenCode reply answers 格式: [[...], [...], ...]
	answers, err := buildOpenCodeReplyAnswers(jsonAnswer)
	if err != nil {
		return fmt.Errorf("build reply answers: %w", err)
	}
	replyBody, _ := json.Marshal(map[string]interface{}{
		"answers": answers,
	})

	// 3. POST /question/{requestID}/reply?directory={cwd}
	replyURL := fmt.Sprintf("%s/question/%s/reply?directory=%s", baseURL, requestID, dirParam)
	replyResp, err := http.Post(replyURL, "application/json", strings.NewReader(string(replyBody)))
	if err != nil {
		return fmt.Errorf("POST /question/%s/reply: %w", requestID, err)
	}
	defer replyResp.Body.Close()

	if replyResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(replyResp.Body, 512))
		return fmt.Errorf("POST /question/%s/reply returned %d: %s", requestID, replyResp.StatusCode, string(body))
	}

	LogInfo("ACP: OpenCode question replied via HTTP",
		zap.String("requestID", requestID),
		zap.String("toolCallID", toolCallID))

	return nil
}

// buildOpenCodeReplyAnswers 将 {"question_0":"a","question_1":["b","c"]} 转为
// OpenCode HTTP reply 所需的 [["a"],["b","c"]] (Array<Array<string>>)。
func buildOpenCodeReplyAnswers(jsonAnswer string) ([]interface{}, error) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonAnswer), &parsed); err != nil {
		return nil, err
	}

	// 按 question_<n> 升序排列
	indices := make([]int, 0, len(parsed))
	for k := range parsed {
		var idx int
		if _, err := fmt.Sscanf(k, "question_%d", &idx); err != nil {
			continue
		}
		if k != fmt.Sprintf("question_%d", idx) {
			continue
		}
		indices = append(indices, idx)
	}
	for i := 1; i < len(indices); i++ {
		for j := i; j > 0 && indices[j-1] > indices[j]; j-- {
			indices[j-1], indices[j] = indices[j], indices[j-1]
		}
	}

	out := make([]interface{}, len(indices))
	for i, idx := range indices {
		v := parsed[fmt.Sprintf("question_%d", idx)]
		switch x := v.(type) {
		case string:
			out[i] = []interface{}{x}
		case []interface{}:
			out[i] = x
		case nil:
			out[i] = []interface{}{}
		default:
			out[i] = []interface{}{fmt.Sprint(x)}
		}
	}
	return out, nil
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
		cwd:    cwd,
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
		ProtocolVersion: 2025,
		ClientCapabilities: acpClientCapabilities{
			Elicitation: &acpElicitationCapabilities{Form: &struct{}{}},
		},
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
		id:        internalSessionID,
		isdpID:    internalSessionID,
		cmd:       cmd,
		ctx:       sessionCtx,
		cancel:    sessionCancel,
		status:    agent.SessionStatusRunning,
		stdinPipe: stdinPipe,
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
	// 注册服务端 request handler，确保 CLI 反向请求（如 session/request_permission）
	// 不会被静默丢弃，避免工具调用挂死。
	transport.SetServerRequestHandler(func(id interface{}, method string, params json.RawMessage) {
		a.handleServerRequest(session, id, method, params, nil)
	})
	session.transport = transport
	transport.Start()

	// Initialize
	initResult, err := transport.SendRequest("initialize", &acpInitializeParams{
		ProtocolVersion: 2025,
		ClientCapabilities: acpClientCapabilities{
			Elicitation: &acpElicitationCapabilities{Form: &struct{}{}},
		},
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
		id:        internalSessionID,
		isdpID:    internalSessionID,
		cmd:       cmd,
		ctx:       sessionCtx,
		cancel:    sessionCancel,
		status:    agent.SessionStatusRunning,
		stdinPipe: stdinPipe,
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
	// 注册服务端 request handler，确保 CLI 反向请求（如 session/request_permission）
	// 不会被静默丢弃，避免工具调用挂死。
	transport.SetServerRequestHandler(func(id interface{}, method string, params json.RawMessage) {
		a.handleServerRequest(session, id, method, params, nil)
	})
	session.transport = transport
	transport.Start()

	// Initialize
	initResult, err := transport.SendRequest("initialize", &acpInitializeParams{
		ProtocolVersion: 2025,
		ClientCapabilities: acpClientCapabilities{
			Elicitation: &acpElicitationCapabilities{Form: &struct{}{}},
		},
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
		ProtocolVersion: 2025,
		ClientCapabilities: acpClientCapabilities{
			Elicitation: &acpElicitationCapabilities{Form: &struct{}{}},
		},
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

		// isdpID 用 invocation ID（与 ExecuteWithStream 一致），SubmitQuestionAnswer →
		// SendToolResult 才能用 invocationID 在 a.sessions map 里找回这个 session 来回 elicitation
		// 响应。fallback 到 internalSessionID 仅为防止 InvocationID 偶发为空时崩溃。
		isdpIDStr := internalSessionID
		if req != nil && req.InvocationID != uuid.Nil {
			isdpIDStr = req.InvocationID.String()
		}

		// 创建 session context
		sessionCtx, sessionCancel := context.WithCancel(context.Background())
		session := &acpSession{
			id:        internalSessionID,
			isdpID:    isdpIDStr,
			cmd:       cmd,
			ctx:       sessionCtx,
			cancel:    sessionCancel,
			status:    agent.SessionStatusRunning,
			stdinPipe: stdinPipe,
			cwd:       req.WorkDir,
			// 进入 ExecuteWithResume 即处于"回放阶段"：从 session/resume 发送之后到
			// session/prompt 发送之前，CLI 推回来的所有 session/update 都属于历史
			// 重放（用于让模型 KV 缓存恢复上下文），不应当作本轮新输出。
			replayPhase: true,
		}

		// 启动 stderr 消费（用 wg 保证 ExecuteWithResume 返回前 goroutine 能正常退出，
		// 避免 stderr goroutine 泄漏 + 让 cleanup 可靠）
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
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
				// 推送用户可感知的 stderr（限流、重试等）到前端
				if onChunk != nil && agent.ShouldNotifyStderr(line) {
					onChunk(agent.Chunk{Type: agent.ChunkTypeError, Content: line})
				}
			}
		}()

		// 关键：创建 transport 时使用正确的 handler（带 onChunk）
		// 这样 session/resume 的历史回放通知会被正确处理
		transport := newACPTransport(stdinPipe, stdoutPipe, func(method string, params json.RawMessage) {
			a.handleNotification(session, method, params, onChunk)
		})
		// 关键：必须注册服务端 request handler，否则 CLI 在 resume 路径下发起的反向 request
		// （如 session/request_permission 让客户端确认读取 workspace 外文件的权限、
		// elicitation/create 让客户端展示 AskUserQuestion 选择框）会被静默丢弃，
		// CLI 永远等不到 response，工具调用就一直停在 in_progress，整轮对话挂死。
		// 这里传 onChunk 让 elicitation 能转换为 question chunk 推给前端。
		transport.SetServerRequestHandler(func(id interface{}, method string, params json.RawMessage) {
			a.handleServerRequest(session, id, method, params, onChunk)
		})
		session.transport = transport
		transport.Start()

		// 保存 session 到 sessions map
		a.mu.Lock()
		a.sessions[isdpIDStr] = session
		a.mu.Unlock()

		// Initialize
		initResult, err := transport.SendRequest("initialize", &acpInitializeParams{
			ProtocolVersion: 2025,
			ClientCapabilities: acpClientCapabilities{
				Elicitation: &acpElicitationCapabilities{Form: &struct{}{}},
			},
		})
		if err != nil {
			session.mu.Lock()
			stderrContent := session.stderrOutput.String()
			session.mu.Unlock()
			transport.Close()
			cmd.Process.Kill()
			cmd.Wait()
			a.mu.Lock()
			delete(a.sessions, isdpIDStr)
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
			delete(a.sessions, isdpIDStr)
			a.mu.Unlock()
			return nil, "", fmt.Errorf("ACP: gateway authenticate failed: %w\nstderr: %s", err, stderrContent)
		}

		// 统一走 session/load：session/resume 在 OpenCode 1.17.x 上会因为新版严格的
		// 内存态 session 校验返回 "session not found"，而 session/load 从磁盘加载持久化
		// session 并完整回放历史。两个版本（1.3.3 / 1.17.x）行为一致，且 replayPhase=true
		// 期间的历史 chunk 在 handleNotification 中被早返回过滤，不污染本轮 output 也
		// 不广播到前端——回放只用于让模型 KV 缓存恢复上下文。
		mcpServers := a.buildMCPServers(req)
		mcpServersJSON, _ := json.Marshal(mcpServers)
		LogInfo("ACP: session/load mcpServers", zap.Int("protocolVersion", initResp.ProtocolVersion), zap.String("mcpServers", string(mcpServersJSON)))
		loadResult, err := transport.SendRequest("session/load", &acpSessionLoadParams{
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
			delete(a.sessions, isdpIDStr)
			a.mu.Unlock()
			return nil, "", fmt.Errorf("ACP: session/load failed: %w\nstderr: %s", err, stderrContent)
		}

		var loadResp acpSessionResumeResult // response structure is same
		if err := json.Unmarshal(loadResult, &loadResp); err != nil {
			LogWarn("ACP: session/load response parse warning", zap.Error(err))
		}

		LogInfo("ACP: ExecuteWithResume session/load completed",
			zap.String("internalSessionId", internalSessionID),
			zap.String("acpSessionId", acpSessionID),
			zap.String("cwd", req.WorkDir),
			zap.Bool("hasConfigOptions", len(loadResp.ConfigOptions) > 0))

		acpSessID = acpSessionID

		// 把 session.id 同步成真正的 ACP sessionId，后续 configureSession / session/prompt
		// 等所有 RPC 才能用正确的 sessionId（否则 CLI 会报 "session not found"）。
		// 与 ExecuteWithStream 在 session/new 后覆盖 session.id 的行为对齐。
		session.mu.Lock()
		session.id = acpSessionID
		session.mu.Unlock()

		// session/load 在 1.3.3 通过历史回放恢复 model/agent，不需要 configureSession。
		// 1.17.x 若返回 ConfigOptions 则补一次 configureSession（与 set_config_option 对齐）。
		if len(loadResp.ConfigOptions) > 0 {
			newSessionResp := &acpNewSessionResult{
				SessionID:     loadResp.SessionID,
				ConfigOptions: loadResp.ConfigOptions,
			}
			if err := a.configureSession(transport, session, newSessionResp, req); err != nil {
				LogWarn("ACP: session configure after load failed", zap.Error(err))
			}
		}

		// 回放窗口结束：session/resume 同步响应已到，后续 session/update 都是真正的本轮输出
		session.mu.Lock()
		session.replayPhase = false
		session.mu.Unlock()

		// Send prompt to resumed session
		// transport.SendRequest 是同步 JSON-RPC 请求：jsonrpc.go: readLoop 单线程顺序解析
		// stdout，每个 notification 同步派发到 handleNotification 处理完才能轮到下一行；
		// CLI 端必然先 emit 所有 session/update notification 再返回 prompt 的 response。
		// 所以 SendRequest 解锁时 session.output 里所有本轮 chunk 都已落入，**不需要**任何
		// "等 signal" 的兜底循环。
		prompt := a.buildPromptFromRequest(req)
		promptResult, promptErr := transport.SendRequest("session/prompt", &acpPromptParams{
			SessionID: acpSessionID,
			Prompt:    []acpContentBlock{{Type: "text", Text: prompt}},
		})

		// 与 ExecuteWithStream 对齐：从 sessions map 移除、cleanup CLI 进程 + transport、
		// wg.Wait 等 stderr goroutine 退出。之前缺这套清理，导致每次 ExecuteWithResume 都
		// 泄漏一个 CLI 子进程 + stderr goroutine + transport 资源。
		// 二次/三次对话时新进程通过 session/resume 复用同一 ACP sessionId，会与上一轮
		// 残留的进程并发访问磁盘上同一份 OpenCode session storage，读到不完整 session
		// → 模型上下文残缺 → 秒返空内容（症状："首次正常、第二次起一直空"）。
		if isdpIDStr != "" {
			a.mu.Lock()
			delete(a.sessions, isdpIDStr)
			a.mu.Unlock()
		}

		if promptErr != nil {
			session.mu.Lock()
			stderrContent := session.stderrOutput.String()
			session.mu.Unlock()
			a.cleanup(session)
			wg.Wait()
			return nil, acpSessID, fmt.Errorf("ACP: session/prompt failed: %w\nstderr: %s", promptErr, stderrContent)
		}

		var promptResp acpPromptResult
		if err := json.Unmarshal(promptResult, &promptResp); err != nil {
			LogWarn("ACP: prompt response parse warning", zap.Error(err))
		}
		LogInfo("[PERF] ACP ExecuteWithResume execution",
			zap.String("acpSessionId", acpSessionID),
			zap.String("stopReason", promptResp.StopReason))

		a.cleanup(session)
		wg.Wait()

		session.mu.Lock()
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
