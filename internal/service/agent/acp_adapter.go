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

type acpAdapterConfig struct {
	cliPath   string
	buildArgs func(req *ExecutionRequest) []string
	buildEnv  func(req *ExecutionRequest) []string
}

type acpSession struct {
	id        string
	isdpID    string
	transport *acpTransport
	cmd       *exec.Cmd
	ctx       context.Context
	cancel    context.CancelFunc
	status    SessionStatus
	output    strings.Builder
	mu        sync.Mutex
}

// BaseACPAdapter implements AgentAdapter using ACP (Agent Client Protocol) over stdio.
// ACP lifecycle: initialize → session/new → session/prompt → session/update notifications → response
type BaseACPAdapter struct {
	config    acpAdapterConfig
	baseAgent *model.BaseAgent
	sessions  map[string]*acpSession
	mu        sync.RWMutex
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

func (a *BaseACPAdapter) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	return a.ExecuteWithStream(ctx, req, nil)
}

func (a *BaseACPAdapter) ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) (*ExecutionResult, error) {
	cliStartTime := time.Now()

	args := a.config.buildArgs(req)
	cmd := exec.CommandContext(ctx, a.config.cliPath, args...)

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
	logInfo("[PERF] ACP cmd.Start", zap.Duration("duration", time.Since(cliStartTime)))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
		}
	}()

	session := &acpSession{
		cmd:    cmd,
		ctx:    ctx,
		status: SessionStatusRunning,
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
		logWarn("ACP: initialize response parse warning", zap.Error(err))
	}
	logInfo("[PERF] ACP initialize handshake", zap.Duration("duration", time.Since(cliStartTime)),
		zap.Int("protocolVersion", initResp.ProtocolVersion))

	sessionID := uuid.New().String()
	sessionNewResult, err := transport.SendRequest("session/new", &acpNewSessionParams{
		SessionID: sessionID,
		CWD:       req.WorkDir,
		Model:     a.baseAgent.DefaultModel,
	})
	if err != nil {
		a.cleanup(session)
		wg.Wait()
		return nil, fmt.Errorf("ACP: session/new failed (model=%s): %w", a.baseAgent.DefaultModel, err)
	}

	var sessionResp acpNewSessionResult
	if err := json.Unmarshal(sessionNewResult, &sessionResp); err == nil && sessionResp.SessionID != "" {
		session.id = sessionResp.SessionID
	} else {
		session.id = sessionID
	}

	logInfo("ACP: session created",
		zap.String("sessionId", session.id),
		zap.String("model", a.baseAgent.DefaultModel))

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
		logWarn("ACP: prompt response parse warning", zap.Error(err))
	}

	logInfo("[PERF] ACP total execution",
		zap.Duration("duration", time.Since(cliStartTime)),
		zap.String("stopReason", promptResp.StopReason))

	a.cleanup(session)
	wg.Wait()

	session.mu.Lock()
	output := session.output.String()
	session.mu.Unlock()

	return &ExecutionResult{
		Output:    output,
		SessionID: session.id,
	}, nil
}

func (a *BaseACPAdapter) StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.sessions[sessionID]; exists {
		return fmt.Errorf("ACP: session already exists: %s", sessionID)
	}

	args := a.config.buildArgs(req)
	cmd := exec.CommandContext(ctx, a.config.cliPath, args...)

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
		status: SessionStatusRunning,
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

	sessionNewResult, err := transport.SendRequest("session/new", &acpNewSessionParams{
		SessionID: sessionID,
		CWD:       req.WorkDir,
		Model:     a.baseAgent.DefaultModel,
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

	a.sessions[sessionID] = session
	logInfo("ACP: session started", zap.String("sessionId", sessionID), zap.String("acpSessionId", session.id))

	return nil
}

func (a *BaseACPAdapter) ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error {
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
	session.status = SessionStatusStopped
	if session.cancel != nil {
		session.cancel()
	}
	session.mu.Unlock()

	a.cleanup(session)

	logInfo("ACP: session stopped", zap.String("sessionId", sessionID))
	return nil
}

func (a *BaseACPAdapter) GetSessionStatus(sessionID string) SessionStatus {
	a.mu.RLock()
	session, exists := a.sessions[sessionID]
	a.mu.RUnlock()

	if !exists {
		return SessionStatusIdle
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	return session.status
}

func (a *BaseACPAdapter) CheckHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := a.config.buildArgs(&ExecutionRequest{BaseAgent: a.baseAgent})
	cmd := exec.CommandContext(ctx, a.config.cliPath, args...)
	cmd.Dir = os.TempDir()
	cmd.Env = a.buildEnv(&ExecutionRequest{BaseAgent: a.baseAgent})

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

func (a *BaseACPAdapter) handleNotification(session *acpSession, method string, params json.RawMessage, onChunk func(Chunk)) {
	switch method {
	case "session/update":
		if onChunk == nil {
			return
		}
		var updateParams acpSessionUpdateParams
		if err := json.Unmarshal(params, &updateParams); err != nil {
			logError("ACP: failed to parse session/update params", zap.Error(err))
			return
		}

		chunks, err := parseACPSessionUpdate(updateParams.Update)
		if err != nil {
			logError("ACP: failed to parse session update", zap.Error(err))
			return
		}

		session.mu.Lock()
		defer session.mu.Unlock()
		for _, chunk := range chunks {
			if chunk.Type == ChunkTypeText {
				session.output.WriteString(chunk.Content)
			}
			onChunk(chunk)
		}

	case "session/request_permission":
		if session.transport != nil {
			session.transport.SendNotification("session/resolve_permission", &acpPermissionResponse{
				Allow: "allow_always",
			})
			logDebug("ACP: permission auto-approved", zap.String("sessionId", session.id))
		}

	default:
		logDebug("ACP: unknown notification method", zap.String("method", method))
	}
}

func (a *BaseACPAdapter) buildPromptFromRequest(req *ExecutionRequest) string {
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

func (a *BaseACPAdapter) buildEnv(req *ExecutionRequest) []string {
	envMap := make(map[string]string)

	for _, e := range os.Environ() {
		if idx := strings.Index(e, "="); idx > 0 {
			envMap[e[:idx]] = e[idx+1:]
		}
	}

	if extraEnv := a.config.buildEnv(req); len(extraEnv) > 0 {
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

func (a *BaseACPAdapter) cleanup(session *acpSession) {
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
		case <-time.After(5 * time.Second):
			session.cmd.Process.Kill()
			logWarn("ACP: process killed after 5s timeout")
		}
	}
}
