// Package claude_code implements the Claude CLI adapter as a plugin.
package claude_code

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
	"github.com/anthropic/isdp/internal/service/agent"
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

	// CLI 进程管理（用于取消）
	currentCmd   *exec.Cmd
	currentCmdMu sync.RWMutex

	// AskUserQuestion 支持：保存 stdin 管道用于发送答案
	currentStdin   io.WriteCloser
	currentStdinMu sync.RWMutex
}

// claudeSession Claude会话
type claudeSession struct {
	id     string
	cmd    *exec.Cmd
	ctx    context.Context
	cancel context.CancelFunc
	status agent.SessionStatus
}

// NewClaudeAdapter 创建Claude适配器
func NewClaudeAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
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
func (a *ClaudeAdapter) Execute(ctx context.Context, req *agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	result, err := a.ExecuteWithStream(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetCurrentProcess 获取当前执行的进程（用于取消）
func (a *ClaudeAdapter) GetCurrentProcess() *exec.Cmd {
	a.currentCmdMu.RLock()
	defer a.currentCmdMu.RUnlock()
	return a.currentCmd
}

// ExecuteWithStream 流式执行
func (a *ClaudeAdapter) ExecuteWithStream(ctx context.Context, req *agent.ExecutionRequest, onChunk func(agent.Chunk)) (*agent.ExecutionResult, error) {
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

	// 使用 stdin 管道发送 prompt，发送后关闭让 CLI 知道输入结束
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// 发送初始 prompt 后关闭 stdin
	go func() {
		stdinPipe.Write([]byte(prompt))
		stdinPipe.Close()
	}()

	// stdin 已关闭，清除引用
	a.currentStdinMu.Lock()
	a.currentStdin = nil
	a.currentStdinMu.Unlock()

	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	env := a.buildEnv(req)
	cmd.Env = env

	// 构建可复制的命令（PowerShell 格式，方便调试）
	// 提取关键环境变量用于日志，对敏感信息脱敏
	var envVarsForLog []string
	for _, e := range env {
		if strings.HasPrefix(e, "ANTHROPIC_") || strings.HasPrefix(e, "CLAUDE_") {
			idx := strings.Index(e, "=")
			if idx > 0 {
				key := e[:idx]
				value := e[idx+1:]
				if key == "ANTHROPIC_AUTH_TOKEN" {
					value = maskToken(value)
				}
				envVarsForLog = append(envVarsForLog, fmt.Sprintf("$env:%s='%s'", key, value))
			}
		}
	}

	// 构建 PowerShell 格式的完整命令
	cliCmd := a.cliPath + " " + strings.Join(args, " ")
	var cmdForCopy strings.Builder
	if cmd.Dir != "" {
		cmdForCopy.WriteString(fmt.Sprintf("cd '%s'; ", cmd.Dir))
	}
	for _, envLine := range envVarsForLog {
		cmdForCopy.WriteString(envLine)
		cmdForCopy.WriteString("; ")
	}
	cmdForCopy.WriteString(cliCmd)


	// 打印完整命令到 zap 日志
	logInfo("Claude: CLI full command",
		zap.String("workDir", cmd.Dir),
		zap.String("command", cmdForCopy.String()),
	)
	logInfo("Claude: CLI env vars",
		zap.Strings("envVars", envVarsForLog),
	)
	logInfo("Claude: CLI args",
		zap.String("cliPath", a.cliPath),
		zap.Strings("args", args),
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

	// 保存当前进程引用（用于取消）
	a.currentCmdMu.Lock()
	a.currentCmd = cmd
	a.currentCmdMu.Unlock()

	// 确保执行结束后清除进程引用
	defer func() {
		a.currentCmdMu.Lock()
		a.currentCmd = nil
		a.currentCmdMu.Unlock()
	}()

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

	// 在后台调用 cmd.Wait()，这样当进程结束时 stdout 会被关闭
	// 这是打破循环依赖的关键：stdout 关闭 -> scanner 结束 -> lineChan 关闭 -> 主循环退出
	cmdWaitDone := make(chan error, 1)
	go func() {
		cmdWaitDone <- cmd.Wait()
	}()

	// 使用 goroutine + channel 读取 stdout，配合 select 监听 ctx.Done()
	// 这样可以在 context 取消时立即退出，不需要等待 scanner.Scan() 返回
	lineChan := make(chan string, 100)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(lineChan) // 确保 lineChan 在 scanner 结束时关闭，避免主 goroutine 永久阻塞
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			select {
			case lineChan <- line:
			case <-ctx.Done():
				logInfo("ExecuteWithStream: context cancelled during scan")
				return
			}
		}
		if err := scanner.Err(); err != nil {
			logInfo("ExecuteWithStream: scanner error", zap.Error(err))
		}
		logInfo("ExecuteWithStream: scanner goroutine finished, closed lineChan")
	}()

	// 主 goroutine 处理行数据，使用 select 监听多个 channel
	// 添加超时机制防止永久阻塞（如果 CLI 进程结束但 stdout 未正确关闭）
	lineCount := 0
	chunkCount := 0
	firstLineReceived := false
	processLines := true
	cmdWaitErr := error(nil) // 存储 cmd.Wait() 的结果
	cmdWaitReceived := false // 标记是否已收到 cmd.Wait 结果
	lineTimeout := time.NewTimer(30 * time.Second) // 30秒无新输出则检查进程状态
	defer lineTimeout.Stop()

	for processLines {
		select {
		case line, ok := <-lineChan:
			if !ok {
				// lineChan 关闭，scanner 结束
				processLines = false
				break
			}
			// 有输出，重置超时
			lineTimeout.Reset(30 * time.Second)
			if line == "" {
				continue
			}
			lineCount++
			// 记录首行时间
			if !firstLineReceived {
				firstLineReceived = true
				logInfo("[PERF] CLI first line received", zap.Duration("duration", time.Since(cliStartTime)), zap.Int("lineNum", lineCount))
			}
			// 调试：打印每行原始输出（前5行或包含工具相关内容）
			if lineCount <= 5 || strings.Contains(line, "tool_use") || strings.Contains(line, "AskUserQuestion") || strings.Contains(line, "input_json") {
				logInfo("ExecuteWithStream: received line", zap.Int("lineNum", lineCount), zap.String("line", line[:min(500, len(line))]))
			}
			chunks := parseStreamJSONLine(line, onChunk != nil)
			for _, chunk := range chunks {
				if onChunk != nil {
					chunkCount++
					logInfo("ExecuteWithStream: calling onChunk", zap.Int("chunkNum", chunkCount), zap.String("type", string(chunk.Type)))
					onChunk(chunk)
				}
			}
		case <-lineTimeout.C:
			// 超时：检查进程是否已结束
			if cmd.Process == nil {
				logInfo("ExecuteWithStream: timeout, process already nil, exiting loop")
				processLines = false
				break
			}
			// 尝试检查进程状态（非阻塞）
			// 如果进程已退出，强制退出循环
			logInfo("ExecuteWithStream: timeout waiting for output, checking process state")
		case waitErr := <-cmdWaitDone:
			// 进程已结束，stdout 应该被关闭了
			cmdWaitErr = waitErr
			cmdWaitReceived = true
			// 等待 scanner goroutine 处理剩余数据并关闭 lineChan
			logInfo("ExecuteWithStream: process finished, waiting for remaining output", zap.Error(waitErr))
			// 继续处理剩余的输出直到 lineChan 关闭
			// 设置一个短超时等待剩余数据
			remainingTimeout := time.NewTimer(2 * time.Second)
			for {
				select {
				case line, ok := <-lineChan:
					if !ok {
						logInfo("ExecuteWithStream: lineChan closed after process end")
						processLines = false
						break
					}
					if line == "" {
						continue
					}
					lineCount++
					chunks := parseStreamJSONLine(line, onChunk != nil)
					for _, chunk := range chunks {
						if onChunk != nil {
							chunkCount++
							onChunk(chunk)
						}
					}
				case <-remainingTimeout.C:
					logInfo("ExecuteWithStream: timeout waiting for remaining output after process end")
					processLines = false
					break
				}
				if !processLines {
					remainingTimeout.Stop()
					break
				}
			}
		case <-ctx.Done():
			logInfo("ExecuteWithStream: context cancelled, stopping output processing")
			processLines = false
			// 不等待 scanner goroutine，wg.Wait() 会在后面等待
		}
	}
	logInfo("ExecuteWithStream: stdout processing complete", zap.Int("lines", lineCount), zap.Int("chunks", chunkCount))

	// 如果 context 已取消，不等待 scanner goroutine（它可能阻塞在已关闭的 stdout 上）
	// 使用带超时的等待，避免永久阻塞
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		logInfo("ExecuteWithStream: scanner goroutine completed normally")
	case <-waitCtx.Done():
		logInfo("ExecuteWithStream: scanner goroutine timeout, proceeding without waiting")
		// scanner goroutine 可能阻塞在 scanner.Scan() 上，但 stdout 最终会被关闭
		// 不影响后续处理
	}

	// 获取 cmd.Wait 结果（如果主循环中已收到则直接使用，否则等待）
	if !cmdWaitReceived {
		// 主循环通过 lineChan 关闭退出，进程可能还在运行或刚刚结束
		select {
		case cmdWaitErr = <-cmdWaitDone:
			cmdWaitReceived = true
		case <-time.After(1 * time.Second):
			logInfo("ExecuteWithStream: timeout waiting for cmd.Wait result")
		}
	}

	if cmdWaitErr != nil {
		// 获取模型名称
		modelName := ""
		if a.baseAgent != nil {
			modelName = a.baseAgent.DefaultModel
		}
		// 详细记录执行失败信息
		logError("Claude: Execution failed",
			zap.Error(cmdWaitErr),
			zap.String("cliPath", a.cliPath),
			zap.String("workDir", cmd.Dir),
			zap.String("configDir", req.ConfigDir),
			zap.String("stderr", stderrOutput.String()),
			zap.String("model", modelName),
			zap.String("sessionId", sessionID), // 记录 sessionId 用于问题定位
		)
		// 执行失败时也返回 sessionId（用于入库追踪和问题定位）
		if stderrOutput.Len() > 0 {
			return &agent.ExecutionResult{SessionID: sessionID}, fmt.Errorf("CLI error: %s", stderrOutput.String())
		}
		return &agent.ExecutionResult{SessionID: sessionID}, fmt.Errorf("CLI execution failed: %w", cmdWaitErr)
	}

	logInfo("[PERF] CLI total execution", zap.Duration("duration", time.Since(cliStartTime)))
	return &agent.ExecutionResult{SessionID: sessionID}, nil
}

// StartSession 启动交互式会话
func (a *ClaudeAdapter) StartSession(ctx context.Context, sessionID string, req *agent.ExecutionRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := &claudeSession{
		id:     sessionID,
		status: agent.SessionStatusRunning,
	}

	// 首次启动使用 ExecuteWithStream
	_, err := a.ExecuteWithStream(ctx, req, nil)
	if err != nil {
		session.status = agent.SessionStatusFailed
		return err
	}

	a.sessions[sessionID] = session

	return nil
}

// ResumeSession 恢复会话
func (a *ClaudeAdapter) ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(agent.Chunk)) error {
	a.mu.RLock()
	_, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	req := &agent.ExecutionRequest{
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

	session.status = agent.SessionStatusStopped
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
func (a *ClaudeAdapter) GetSessionStatus(sessionID string) agent.SessionStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return agent.SessionStatusStopped
	}
	return session.status
}

// buildPromptFromRequest 从ExecutionRequest构建提示词
func (a *ClaudeAdapter) buildPromptFromRequest(req *agent.ExecutionRequest) string {
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

// buildEnv 构建环境变量
// 使用 map 去重，BaseAgent 配置的值会覆盖系统环境变量
func (a *ClaudeAdapter) buildEnv(req *agent.ExecutionRequest) []string {
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
	// 设置模型环境变量，覆盖用户级的其他精细化设置的模型或系统默认的模型
	if a.baseAgent != nil && a.baseAgent.DefaultModel != "" {
		envMap["ANTHROPIC_MODEL"] = a.baseAgent.DefaultModel
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
	execReq := &agent.ExecutionRequest{
		BaseAgent: a.baseAgent,
	}
	env := a.buildEnv(execReq)
	cmd.Env = env

	// 提取关键环境变量用于日志，对敏感信息脱敏
	var envVarsForLog []string
	for _, e := range env {
		if strings.HasPrefix(e, "ANTHROPIC_") || strings.HasPrefix(e, "CLAUDE_") {
			idx := strings.Index(e, "=")
			if idx > 0 {
				key := e[:idx]
				value := e[idx+1:]
				if key == "ANTHROPIC_AUTH_TOKEN" {
					value = maskToken(value)
				}
				envVarsForLog = append(envVarsForLog, fmt.Sprintf("$env:%s='%s'", key, value))
			}
		}
	}

	// 构建 PowerShell 格式的完整命令
	cliCmd := a.cliPath + " " + strings.Join(args, " ")
	var cmdForCopy strings.Builder
	cmdForCopy.WriteString(fmt.Sprintf("cd '%s'; ", os.TempDir()))
	for _, envLine := range envVarsForLog {
		cmdForCopy.WriteString(envLine)
		cmdForCopy.WriteString("; ")
	}
	cmdForCopy.WriteString(cliCmd)


	// 打印完整命令到 zap 日志
	logInfo("Claude: CheckHealth full command",
		zap.String("workDir", os.TempDir()),
		zap.String("command", cmdForCopy.String()),
	)
	logInfo("Claude: CheckHealth env vars",
		zap.Strings("envVars", envVarsForLog),
	)
	logInfo("Claude: CheckHealth args",
		zap.String("cliPath", a.cliPath),
		zap.Strings("args", args),
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

// minLen returns the minimum of two integers
func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// maskToken 对敏感token进行掩码处理，只显示前4位和后4位
// 例如: "sk-ant-api03-xxxxx" -> "sk-a****xxxx"
func maskToken(token string) string {
	if token == "" {
		return "<empty>"
	}
	if len(token) <= 8 {
		return "****"
	}
	// 显示前4位和后4位，中间用****替代
	return token[:4] + "****" + token[len(token)-4:]
}

// logInfo 记录信息级别日志
func logInfo(msg string, fields ...zap.Field) {
	if logger := zap.L(); logger != nil {
		logger.Info(msg, fields...)
	}
}

// logError 记录错误级别日志
func logError(msg string, fields ...zap.Field) {
	if logger := zap.L(); logger != nil {
		logger.Error(msg, fields...)
	}
}

// logDebug 记录调试级别日志
func logDebug(msg string, fields ...zap.Field) {
	if logger := zap.L(); logger != nil {
		logger.Debug(msg, fields...)
	}
}

// SendToolResult 发送工具结果给 CLI（用于 AskUserQuestion 等需要用户输入的工具）
// CLI 使用 stdin 接收用户答案
func (a *ClaudeAdapter) SendToolResult(invocationID uuid.UUID, toolCallID string, result string) error {
	a.currentStdinMu.RLock()
	stdin := a.currentStdin
	a.currentStdinMu.RUnlock()

	if stdin == nil {
		return fmt.Errorf("ClaudeAdapter: stdin pipe not available for invocation %s", invocationID.String())
	}

	// 构建答案消息格式
	// 使用 json.Marshal 确保 result 正确转义（处理引号、换行等特殊字符）
	contentJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("ClaudeAdapter: failed to marshal result: %w", err)
	}
	answerMsg := fmt.Sprintf("{\"type\":\"tool_result\",\"tool_use_id\":'%s',\"content\":%s}\n", toolCallID, string(contentJSON))

	logInfo("ClaudeAdapter: sending tool result via stdin",
		zap.String("invocationID", invocationID.String()),
		zap.String("toolCallId", toolCallID),
		zap.String("answer", result))

	_, err = stdin.Write([]byte(answerMsg))
	if err != nil {
		logError("ClaudeAdapter: failed to write tool result to stdin", zap.Error(err))
		return fmt.Errorf("ClaudeAdapter: failed to send tool result: %w", err)
	}

	logInfo("ClaudeAdapter: tool result sent successfully", zap.String("toolCallId", toolCallID))
	return nil
}