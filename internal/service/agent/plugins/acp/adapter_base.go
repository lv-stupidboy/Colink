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
}

// maxStderrSize 定义 stderr 缓冲的最大大小（64KB）
// 防止 stderr 输出过量导致内存问题
const maxStderrSize = 64 * 1024

// 默认上下文配置（当未通过 SetContextConfig 设置时使用）
const defaultWarningThreshold = 0.80
const defaultCompactThreshold = 0.95
const defaultContextLimit = 200000

// 上下文配置（可通过 SetContextConfig 设置）
// 用于智能压缩功能，从配置文件读取
var globalContextConfig *ContextConfig

// ContextConfig 上下文管理配置
type ContextConfig struct {
	WarningThreshold float64           // 预警阈值（百分比）
	CompactThreshold float64           // 压缩阈值（百分比）
	ModelLimits       map[string]int64  // 各模型的上下文限制（tokens）
	DefaultLimit      int64             // 默认上下文限制
}

// SetContextConfig 设置全局上下文配置（由 main.go 在启动时调用）
func SetContextConfig(cfg *ContextConfig) {
	globalContextConfig = cfg
	LogInfo("ACP: context config set",
		zap.Float64("warningThreshold", cfg.WarningThreshold),
		zap.Float64("compactThreshold", cfg.CompactThreshold),
		zap.Int64("defaultLimit", cfg.DefaultLimit),
		zap.Int("modelLimitsCount", len(cfg.ModelLimits)))
}

// getContextConfig 获取上下文配置（返回配置值或默认值）
func getContextConfig() *ContextConfig {
	if globalContextConfig != nil {
		return globalContextConfig
	}
	// 返回默认配置
	return &ContextConfig{
		WarningThreshold: defaultWarningThreshold,
		CompactThreshold: defaultCompactThreshold,
		DefaultLimit:     defaultContextLimit,
		ModelLimits:      getDefaultModelLimits(),
	}
}

// getDefaultModelLimits 返回默认的模型上下文限制
func getDefaultModelLimits() map[string]int64 {
	return map[string]int64{
		// Claude 模型
		"claude-opus-4-7":   200000,
		"claude-opus-4-6":   200000,
		"claude-sonnet-4-6": 200000,
		"claude-3-5-sonnet": 200000,
		// OpenAI 模型
		"gpt-4o":      128000,
		"gpt-4-turbo": 128000,
		// Gemini 模型
		"gemini-1.5-pro":   1000000,
		"gemini-2.0-flash": 1000000,
	}
}

// getModelContextLimit 根据模型名称获取上下文限制
func getModelContextLimit(model string) int64 {
	cfg := getContextConfig()
	// 1. 精确匹配
	if limit, ok := cfg.ModelLimits[model]; ok {
		return limit
	}
	// 2. 提取模型后缀（处理 provider/model 格式）
	// 例如：bailian-coding-plan/glm-5 → glm-5
	modelSuffix := model
	if idx := strings.LastIndex(model, "/"); idx >= 0 {
		modelSuffix = model[idx+1:]
	}
	if limit, ok := cfg.ModelLimits[modelSuffix]; ok {
		return limit
	}
	// 3. 下划线替代点匹配（配置文件中用下划线替代点，如 gemini-1.5-pro → gemini-1_5-pro）
	// 这是因为 YAML 的 mapstructure 无法正确处理带点的键名
	modelWithUnderscore := strings.ReplaceAll(model, ".", "_")
	if limit, ok := cfg.ModelLimits[modelWithUnderscore]; ok {
		return limit
	}
	suffixWithUnderscore := strings.ReplaceAll(modelSuffix, ".", "_")
	if limit, ok := cfg.ModelLimits[suffixWithUnderscore]; ok {
		return limit
	}
	// 4. 前缀匹配（优先匹配最长前缀，更精确的配置优先）
	// 例如：glm-5.1-xxx 应匹配 glm-5_1 而非 glm-5
	var bestPrefix string
	var bestLimit int64
	for prefix, limit := range cfg.ModelLimits {
		// 检查完整模型名或后缀是否能匹配该前缀
		// 同时支持下划线替代点的匹配
		if strings.HasPrefix(model, prefix) ||
			strings.HasPrefix(modelSuffix, prefix) ||
			strings.HasPrefix(modelWithUnderscore, prefix) ||
			strings.HasPrefix(suffixWithUnderscore, prefix) {
			// 选择最长前缀（更精确）
			if len(prefix) > len(bestPrefix) {
				bestPrefix = prefix
				bestLimit = limit
			}
		}
	}
	if bestPrefix != "" {
		return bestLimit
	}
	// 5. 默认值
	return cfg.DefaultLimit
}

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
	// 上下文使用量追踪（用于智能压缩）
	cumulativeInputTokens  int64  // 累计输入 tokens（从 usage_update 通知获取）
	cumulativeOutputTokens int64  // 累计输出 tokens
	lastUsageInputTokens   int64  // 最近一次 usage_update 的输入 tokens
	lastUsageOutputTokens  int64  // 最近一次 usage_update 的输出 tokens
	contextLimit           int64  // 上下文限制（根据模型确定）
	mu                     sync.Mutex
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
	promptResult, err := transport.SendRequest("session/prompt", &acpPromptParams{
		SessionID: session.id,
		Prompt:    []acpContentBlock{{Type: "text", Text: prompt}},
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

	// 根据服务器实际支持的协议版本决定是否传递 MCP Servers
	// ACP v1 不支持 mcpServers 字段，只有 v2025+ 支持
	// 由于 StartSession 不解析 init 响应，默认不传递 mcpServers（保守策略）
	mcpServers := []interface{}{}

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
	// 记录所有收到的通知类型（Info 级别，用于排查 usage_update 是否收到）
	LogInfo("ACP: received notification",
		zap.String("method", method),
		zap.String("sessionId", session.id))

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
			// 追踪 token 使用量（用于上下文监控）
			if chunk.Type == agent.ChunkTypeUsage && chunk.Usage != nil {
				session.lastUsageInputTokens = chunk.Usage.InputTokens
				session.lastUsageOutputTokens = chunk.Usage.OutputTokens
				// 累加（usage_update 通常包含当前请求的完整统计）
				session.cumulativeInputTokens = chunk.Usage.InputTokens
				session.cumulativeOutputTokens += chunk.Usage.OutputTokens
				LogInfo("ACP: usage update",
					zap.String("sessionId", session.id),
					zap.Int64("inputTokens", chunk.Usage.InputTokens),
					zap.Int64("outputTokens", chunk.Usage.OutputTokens),
					zap.Int64("cumulativeInput", session.cumulativeInputTokens),
					zap.Int64("cumulativeOutput", session.cumulativeOutputTokens),
					zap.Float64("contextUsage", float64(session.cumulativeInputTokens)/float64(session.contextLimit)))
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

// ========== 长连接 Session 支持 ==========

// StartLongRunningSession 启动长连接 session
// 用于 OpenCode/CodeAgent 等不支持原生 resume 的 CLI
// 保持进程存活，避免每次都重新启动
// 返回 ACP session ID 用于后续 SendPromptToSession
func (a *BaseACPAdapter) StartLongRunningSession(ctx context.Context, req *agent.ExecutionRequest) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 生成唯一的 session ID
	sessionID := uuid.New().String()

	args := a.Config.BuildArgs(req)
	cmd := exec.CommandContext(ctx, a.Config.CliPath, args...)
	hideCommandLineWindow(cmd)

	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	env := a.buildEnv(req)
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

	LogInfo("ACP: long running session started",
		zap.String("sessionId", sessionID),
		zap.Int("pid", cmd.Process.Pid),
		zap.String("workDir", req.WorkDir))

	// 创建 session context（独立于请求 context）
	sessionCtx, sessionCancel := context.WithCancel(context.Background())
	session := &acpSession{
		id:                 sessionID,
		isdpID:             sessionID,
		cmd:                cmd,
		ctx:                sessionCtx,
		cancel:             sessionCancel,
		status:             agent.SessionStatusRunning,
		stdinPipe:          stdinPipe, // 保存 stdin 引用用于断开检测
		outputUpdatedSignal: make(chan struct{}, 1), // 初始化输出更新信号
	}

	// 启动 stdin 断开监控 goroutine
	// 当后端进程退出时，stdin 管道会被关闭，CLI 应该检测到并退出
	go a.monitorStdinConnection(session, stdinPipe)

	// 启动 stderr 消费 goroutine
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			LogInfo("ACP: stderr output (long running)",
				zap.String("sessionId", sessionID),
				zap.String("line", line))
			session.mu.Lock()
			if session.stderrOutput.Len() < maxStderrSize {
				session.stderrOutput.WriteString(line)
				session.stderrOutput.WriteString("\n")
			}
			session.mu.Unlock()
		}
	}()

	// 创建 transport
	transport := newACPTransport(stdinPipe, stdoutPipe, func(method string, params json.RawMessage) {
		// 长连接模式下，notification handler 可能不设置 onChunk
		// 但仍需要处理 notification
		a.handleNotification(session, method, params, nil)
	})
	session.transport = transport
	transport.Start()

	// 执行 initialize 和 session/new
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

	// 根据 protocol version 决定是否传递 MCP Servers
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
		transport.Close()
		cmd.Process.Kill()
		return "", fmt.Errorf("ACP: session/new failed: %w\nstderr: %s", err, stderrContent)
	}

	var sessionResp acpNewSessionResult
	if err := json.Unmarshal(sessionNewResult, &sessionResp); err != nil {
		LogWarn("ACP: session/new response parse warning", zap.Error(err))
	}

	// 更新 ACP session ID
	if sessionResp.SessionID != "" {
		session.id = sessionResp.SessionID
	}

	// 配置 session（设置模型）
	if err := a.configureSession(transport, session, &sessionResp, req); err != nil {
		LogWarn("ACP: session configuration warning", zap.Error(err))
	}

	// 保存到 sessions map
	a.sessions[sessionID] = session

	LogInfo("ACP: long running session ready",
		zap.String("sessionId", sessionID),
		zap.String("acpSessionId", session.id))

	return sessionID, nil
}

// SendPromptToSession 向已有 session 发送新的 prompt
// 用于长连接模式，复用已有进程
// 添加等待机制确保所有 session/update 通知处理完毕
func (a *BaseACPAdapter) SendPromptToSession(ctx context.Context, sessionID string, prompt string, onChunk func(agent.Chunk)) error {
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
	transport := session.transport
	// 初始化输出更新信号（用于等待通知处理完成）
	session.outputUpdatedSignal = make(chan struct{}, 1)
	// 重置输出缓冲（确保新一轮的输出是干净的）
	session.output.Reset()
	session.mu.Unlock()

	// 更新 notification handler 以处理 chunks
	if onChunk != nil {
		transport.SetNotificationHandler(func(method string, params json.RawMessage) {
			a.handleNotification(session, method, params, onChunk)
			// 每次通知处理后，发送更新信号
			session.mu.Lock()
			if session.outputUpdatedSignal != nil {
				select {
				case session.outputUpdatedSignal <- struct{}{}:
				default: // 信号已存在，跳过
				}
			}
			session.mu.Unlock()
		})
	}

	// 发送 prompt
	promptResult, err := transport.SendRequest("session/prompt", &acpPromptParams{
		SessionID: session.id,
		Prompt:    []acpContentBlock{{Type: "text", Text: prompt}},
	})
	if err != nil {
		return fmt.Errorf("ACP: session/prompt failed: %w", err)
	}

	var promptResp acpPromptResult
	if err := json.Unmarshal(promptResult, &promptResp); err != nil {
		LogWarn("ACP: prompt response parse warning", zap.Error(err))
	}

	LogInfo("ACP: prompt sent to long running session",
		zap.String("sessionId", sessionID),
		zap.String("stopReason", promptResp.StopReason))

	// 等待所有通知处理完成（最多等待 500ms）
	// 当 session/prompt 响应返回时，通知应该都已经发送了
	// 但 readLoop 可能还在异步处理，需要等待一小段时间
	session.mu.Lock()
	signal := session.outputUpdatedSignal
	currentOutputLen := session.output.Len()
	session.mu.Unlock()

	if signal != nil && currentOutputLen == 0 {
		// 如果当前没有输出，等待更新信号（最多 500ms）
		select {
		case <-signal:
			// 收到更新信号，继续等待可能的更多更新
			// 使用退避策略：连续 3 次无新更新则认为完成
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
			LogWarn("ACP: timeout waiting for output updates",
				zap.String("sessionId", sessionID),
				zap.String("stopReason", promptResp.StopReason))
		}
	} else if signal != nil {
		// 已有输出，等待可能的更多更新（最多 300ms）
		select {
		case <-signal:
		case <-time.After(300 * time.Millisecond):
		}
	}

	// 清理信号
	session.mu.Lock()
	session.outputUpdatedSignal = nil
	session.mu.Unlock()

	return nil
}

// IsSessionAlive 检查 session 是否存活
func (a *BaseACPAdapter) IsSessionAlive(sessionID string) bool {
	a.mu.RLock()
	session, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return false
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// 检查进程是否存活
	if session.cmd == nil || session.cmd.Process == nil {
		return false
	}

	// 检查进程是否已退出（关键修复：防止写入已关闭的管道）
	if session.cmd.ProcessState != nil && session.cmd.ProcessState.Exited() {
		LogInfo("ACP: IsSessionAlive detected process exited",
			zap.String("sessionId", session.id),
			zap.Int("pid", session.cmd.Process.Pid))
		return false
	}

	// 棅查状态
	if session.status != agent.SessionStatusRunning {
		return false
	}

	return true
}

// StopLongRunningSession 停止长连接 session
func (a *BaseACPAdapter) StopLongRunningSession(sessionID string) error {
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

	LogInfo("ACP: long running session stopped",
		zap.String("sessionId", sessionID))

	return nil
}

// GetSessionOutput 获取 session 累积的输出
func (a *BaseACPAdapter) GetSessionOutput(sessionID string) string {
	a.mu.RLock()
	session, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return ""
	}

	session.mu.Lock()
	output := session.output.String()
	session.mu.Unlock()

	return output
}

// GetSessionStderr 获取 session 累积的 stderr 输出（用于错误诊断）
func (a *BaseACPAdapter) GetSessionStderr(sessionID string) string {
	a.mu.RLock()
	session, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return ""
	}

	session.mu.Lock()
	stderr := session.stderrOutput.String()
	session.mu.Unlock()

	return stderr
}

// GetContextUsage 获取 session 的上下文使用情况
// 返回值：usagePercent（使用百分比），inputTokens（累计输入），contextLimit（上下文限制）
func (a *BaseACPAdapter) GetContextUsage(sessionID string) (usagePercent float64, inputTokens int64, contextLimit int64) {
	a.mu.RLock()
	session, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return 0, 0, 0
	}

	session.mu.Lock()
	inputTokens = session.cumulativeInputTokens
	contextLimit = session.contextLimit
	session.mu.Unlock()

	if contextLimit > 0 {
		usagePercent = float64(inputTokens) / float64(contextLimit)
	}

	return usagePercent, inputTokens, contextLimit
}

// ShouldCompact 检查是否需要进行上下文压缩
// 返回值：needsCompact（是否需要压缩），needsWarning（是否需要预警）
func (a *BaseACPAdapter) ShouldCompact(sessionID string) (needsCompact bool, needsWarning bool, usagePercent float64) {
	usagePercent, _, _ = a.GetContextUsage(sessionID)
	cfg := getContextConfig()

	if usagePercent >= cfg.CompactThreshold {
		return true, true, usagePercent
	}
	if usagePercent >= cfg.WarningThreshold {
		return false, true, usagePercent
	}
	return false, false, usagePercent
}

// SetContextLimit 设置 session 的上下文限制（根据模型）
func (a *BaseACPAdapter) SetContextLimit(sessionID string, model string) {
	a.mu.RLock()
	session, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return
	}

	contextLimit := getModelContextLimit(model)
	session.mu.Lock()
	session.contextLimit = contextLimit
	usedTokens := session.cumulativeInputTokens
	session.mu.Unlock()

	usagePercent := float64(usedTokens) / float64(contextLimit)
	LogInfo("ACP: context limit set",
		zap.String("sessionId", sessionID),
		zap.String("model", model),
		zap.Int64("contextLimit", contextLimit),
		zap.Int64("usedTokens", usedTokens),
		zap.Float64("usagePercent", usagePercent))
}

// TriggerCompact 触发上下文压缩
// 发送一个特殊的 prompt 让 CLI 执行类似 /compact 的操作
func (a *BaseACPAdapter) TriggerCompact(ctx context.Context, sessionID string) error {
	a.mu.RLock()
	session, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return fmt.Errorf("ACP: session not found: %s", sessionID)
	}

	// 构建压缩 prompt
	// 类似 Claude CLI 的 /compact 功能，压缩对话历史保留关键信息
	compactPrompt := `[SYSTEM] 请执行以下上下文压缩操作：

1. 总结之前的对话要点，保留关键信息：
   - 用户的核心需求
   - 已完成的工作和结果
   - 待解决的问题
   - 重要的技术决策和约束

2. 删除冗余的历史消息，只保留：
   - 最近 3-5 轮对话的完整内容
   - 之前的对话摘要

3. 保持对话连贯性，确保：
   - 你仍然能理解用户的后续请求
   - 关键上下文信息不丢失

请开始压缩并回复"上下文已压缩"确认完成。`

	LogInfo("ACP: triggering context compact",
		zap.String("sessionId", sessionID),
		zap.Int64("currentInputTokens", session.cumulativeInputTokens),
		zap.Float64("usagePercent", float64(session.cumulativeInputTokens)/float64(session.contextLimit)))

	// 发送压缩指令
	session.mu.Lock()
	transport := session.transport
	session.mu.Unlock()

	if transport == nil {
		return fmt.Errorf("ACP: session transport not available")
	}

	_, err := transport.SendRequest("session/prompt", &acpPromptParams{
		SessionID: session.id,
		Prompt:    []acpContentBlock{{Type: "text", Text: compactPrompt}},
	})
	if err != nil {
		return fmt.Errorf("ACP: compact prompt failed: %w", err)
	}

	// 重置 token 计数（压缩后应该会大幅减少）
	session.mu.Lock()
	session.cumulativeInputTokens = 0
	session.cumulativeOutputTokens = 0
	session.mu.Unlock()

	LogInfo("ACP: context compact completed",
		zap.String("sessionId", sessionID))

	return nil
}

// ResetSessionOutput 重置 session 输出缓冲
func (a *BaseACPAdapter) ResetSessionOutput(sessionID string) {
	a.mu.RLock()
	session, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return
	}

	session.mu.Lock()
	session.output.Reset()
	session.mu.Unlock()
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// monitorStdinConnection monitors stdin pipe connection state.
// When stdin is disconnected (backend process exits), it closes session and kills CLI process.
// This is core of Plan C: ensuring CLI process lifecycle is bound to backend process.
func (a *BaseACPAdapter) monitorStdinConnection(session *acpSession, stdinPipe io.WriteCloser) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-session.ctx.Done():
			LogInfo("ACP: stdin monitor stopped (session done)",
				zap.String("sessionId", session.id))
			return
		case <-ticker.C:
			session.mu.Lock()
			cmd := session.cmd
			session.mu.Unlock()

			if cmd == nil || cmd.Process == nil {
				LogInfo("ACP: stdin monitor detected process gone",
					zap.String("sessionId", session.id))
				session.cancel()
				return
			}

			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				LogInfo("ACP: stdin monitor detected process exited",
					zap.String("sessionId", session.id),
					zap.Int("pid", cmd.Process.Pid))
				session.cancel()
				return
			}
		}
	}
}

// CloseAllSessions closes all active sessions (for graceful shutdown).
// Called when backend process exits to ensure all CLI processes are properly terminated.
func (a *BaseACPAdapter) CloseAllSessions() {
	a.mu.Lock()
	sessions := make([]*acpSession, 0, len(a.sessions))
	for _, s := range a.sessions {
		sessions = append(sessions, s)
	}
	a.mu.Unlock()

	for _, session := range sessions {
		session.mu.Lock()
		cmd := session.cmd
		transport := session.transport
		session.mu.Unlock()

		if transport != nil {
			transport.Close()
		}

		if cmd != nil && cmd.Process != nil {
			done := make(chan struct{})
			go func() {
				cmd.Wait()
				close(done)
			}()

			select {
			case <-done:
				LogInfo("ACP: session closed gracefully",
					zap.String("sessionId", session.id))
			case <-time.After(3 * time.Second):
				cmd.Process.Kill()
				LogInfo("ACP: session killed (timeout)",
					zap.String("sessionId", session.id))
			}
		}

		session.cancel()
	}

	LogInfo("ACP: all sessions closed", zap.Int("count", len(sessions)))
}
