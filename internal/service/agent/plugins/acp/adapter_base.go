// internal/service/agent/plugins/acp/adapter_base.go
// Base ACP Adapter implementation
package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
}

type acpSession struct {
	id              string
	isdpID          string
	transport       *acpTransport
	cmd             *exec.Cmd
	ctx             context.Context
	cancel          context.CancelFunc
	status          agent.SessionStatus
	output          strings.Builder
	pendingQuestion *agent.Chunk // 待处理的 AskUserQuestion（等待用户响应）
	mu              sync.Mutex
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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			// stderr 内容通常不处理，只消耗
		}
	}()

	// 设置 invocationID 用于 AskUserQuestion 答案发送
	var invocationIDStr string
	if req.InvocationID != uuid.Nil {
		invocationIDStr = req.InvocationID.String()
	}

	session := &acpSession{
		cmd:    cmd,
		ctx:    ctx,
		status: agent.SessionStatusRunning,
		isdpID: invocationIDStr,
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
		a.cleanup(session)
		wg.Wait()
		return nil, fmt.Errorf("ACP: initialize handshake failed: %w", err)
	}

	var initResp acpInitializeResult
	if err := json.Unmarshal(initResult, &initResp); err != nil {
		LogWarn("ACP: initialize response parse warning", zap.Error(err))
	}
	LogInfo("[PERF] ACP initialize handshake", zap.Duration("duration", time.Since(cliStartTime)),
		zap.Int("protocolVersion", initResp.ProtocolVersion))

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
		a.cleanup(session)
		wg.Wait()
		return nil, fmt.Errorf("ACP: session/new failed: %w", err)
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
	promptResult, err := transport.SendRequest("session/prompt", &acpPromptParams{
		SessionID: session.id,
		Prompt:    []acpContentBlock{{Type: "text", Text: prompt}},
	})
	if err != nil {
		a.cleanup(session)
		wg.Wait()
		return nil, fmt.Errorf("ACP: session/prompt failed: %w", err)
	}

	var promptResp acpPromptResult
	if err := json.Unmarshal(promptResult, &promptResp); err != nil {
		LogWarn("ACP: prompt response parse warning", zap.Error(err))
	}

	LogInfo("[PERF] ACP total execution",
		zap.Duration("duration", time.Since(cliStartTime)),
		zap.String("stopReason", promptResp.StopReason),
		zap.String("promptResultRaw", string(promptResult)))

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

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
		}
	}()

	sessionCtx, sessionCancel := context.WithCancel(ctx)
	session := &acpSession{
		id:     sessionID,
		isdpID: sessionID,
		cmd:    cmd,
		ctx:    sessionCtx,
		cancel: sessionCancel,
		status: agent.SessionStatusRunning,
	}

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
		transport.Close()
		cmd.Process.Kill()
		return fmt.Errorf("ACP: initialize handshake failed: %w", err)
	}

	// 根据服务器实际支持的协议版本决定是否传递 MCP Servers
	// ACP v1 不支持 mcpServers 字段，只有 v2025+ 支持
	// 由于 StartSession 不解析 init 响应，默认不传递 mcpServers（保守策略）
	mcpServers := []interface{}{}

	sessionNewResult, err := transport.SendRequest("session/new", &acpNewSessionParams{
		CWD:        req.WorkDir,
		MCPServers: mcpServers,
	})
	if err != nil {
		transport.Close()
		cmd.Process.Kill()
		return fmt.Errorf("ACP: session/new failed: %w", err)
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
	// 记录所有收到的通知（调试用）
	LogInfo("ACP: received notification",
		zap.String("method", method),
		zap.String("paramsPreview", string(params)[:min(500, len(string(params)))]))

	switch method {
	case "session/update":
		if onChunk == nil {
			return
		}
		var updateParams acpSessionUpdateParams
		if err := json.Unmarshal(params, &updateParams); err != nil {
			LogError("ACP: failed to parse session/update params", zap.Error(err))
			return
		}

		// 解析 sessionUpdate 类型
		var header acpSessionUpdateHeader
		if err := json.Unmarshal(updateParams.Update, &header); err == nil {
			LogInfo("ACP: session/update type",
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
		LogInfo("ACP: received user input request",
			zap.String("sessionId", inputRequest.SessionID),
			zap.String("toolCallId", inputRequest.ToolCallID),
			zap.String("toolName", inputRequest.ToolName),
			zap.Any("input", inputRequest.Input))

		// 详细打印 input 结构（用于调试解析问题）
		inputJSON, _ := json.MarshalIndent(inputRequest.Input, "", "  ")
		LogInfo("ACP: user input request - detailed input structure",
			zap.String("inputJSON", string(inputJSON)))

		// 解析问题并创建 question chunk
		chunk := parseACPUserInputRequest(inputRequest)
		LogInfo("ACP: parsed question chunk",
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
			LogInfo("ACP: session saved to sessions map for AskUserQuestion",
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
		LogInfo("ACP: received session/tool_call_update notification", zap.String("params", string(params)))

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
		LogInfo("ACP: unknown notification method",
			zap.String("method", method),
			zap.String("params", string(params)))
	}
}

func (a *BaseACPAdapter) buildPromptFromRequest(req *agent.ExecutionRequest) string {
	var sb strings.Builder

	if req.Context != nil {
		if req.Context.Layer0 != "" {
			sb.WriteString("<system>\n")
			sb.WriteString(req.Context.Layer0)
			sb.WriteString("\n</system>\n\n")
		}

		if req.Context.Layer1 != "" {
			sb.WriteString("<conversation>\n")
			sb.WriteString(req.Context.Layer1)
			sb.WriteString("\n</conversation>\n\n")
		}

		if req.Context.Layer2 != "" {
			sb.WriteString("<artifacts>\n")
			sb.WriteString(req.Context.Layer2)
			sb.WriteString("\n</artifacts>\n\n")
		}

		if req.Context.Layer3 != "" {
			sb.WriteString("<environment>\n")
			sb.WriteString(req.Context.Layer3)
			sb.WriteString("\n</environment>\n\n")
		}
	}

	sb.WriteString("<user>\n")
	sb.WriteString(req.Input)
	sb.WriteString("\n</user>\n")

	return sb.String()
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
// 用于在 session/new 时注入 memory MCP server
func (a *BaseACPAdapter) buildMCPServers(req *agent.ExecutionRequest) []interface{} {
	if req.CallbackToken == "" || req.APIURL == "" || req.InvocationID == uuid.Nil {
		return []interface{}{}
	}

	// 获取 MCP server 可执行文件路径
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
		return []interface{}{}
	}

	LogInfo("ACP: MCP server path", zap.String("path", mcpServerPath))

	// 构建 memory MCP server 配置
	mcpServer := map[string]interface{}{
		"name":    "isdp-memory",
		"type":    "stdio",
		"command": mcpServerPath,
		"args":    []string{},
		"env": map[string]string{
			"ISDP_API_URL":        req.APIURL,
			"ISDP_INVOCATION_ID":  req.InvocationID.String(),
			"ISDP_CALLBACK_TOKEN": req.CallbackToken,
		},
	}

	return []interface{}{mcpServer}
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
	// 先关闭 transport（这会关闭 stdin/stdout）
	if session.transport != nil {
		session.transport.Close()
	}

	// 等待进程结束（stdin 关闭后进程应该退出）
	if session.cmd != nil && session.cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- session.cmd.Wait()
		}()
		select {
		case <-done:
			// 进程正常退出
		case <-time.After(3 * time.Second):
			LogWarn("ACP: process still running, sending interrupt", zap.String("sessionId", session.id))
			session.cmd.Process.Signal(os.Interrupt)
			select {
			case <-done:
				LogInfo("ACP: process exited after interrupt")
			case <-time.After(2 * time.Second):
				session.cmd.Process.Kill()
				LogWarn("ACP: process killed after timeout")
				<-done // 等待 Kill 完成
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

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}