# Agent Adapter Unification Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify Agent execution entry points by encapsulating session management in the Adapter layer, making workflow and debug scenarios use the same execution path.

**Architecture:** All session management logic moves into individual adapters (ClaudeAdapter, OpenCodeAdapter). ExecutionService becomes the single entry point. The difference between workflow and debug scenarios is naturally determined by whether a workflow template exists.

**Tech Stack:** Go 1.21+, Gin, WebSocket, exec.Command

---

## File Structure

| File | Action | Description |
|------|--------|-------------|
| `internal/service/agent/types.go` | Create | New types: ExecutionRequest, ExecutionResult, Chunk, ChunkType |
| `internal/service/agent/adapter.go` | Modify | Redefine AgentAdapter interface |
| `internal/service/agent/claude_adapter.go` | Modify | Implement session management |
| `internal/service/agent/open_code_adapter.go` | Modify | Implement session management |
| `internal/service/agent/execution_service.go` | Modify | Simplify to single execution path |
| `internal/service/agent/orchestrator.go` | Modify | Remove interactiveManager, simplify |
| `internal/api/agent_handler.go` | Modify | Use unified SpawnAgent for debug |
| `internal/service/agent/interactive_session.go` | Delete | Session logic moved to adapters |
| `internal/service/agent/session_manager.go` | Delete | Session logic moved to adapters |
| `internal/service/agent/execution_context.go` | Delete | No longer needed |

---

## Task 1: Create New Types

**Files:**
- Create: `isdp/internal/service/agent/types.go`

- [ ] **Step 1: Create types.go with new type definitions**

```go
package agent

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
)

// ChunkType 输出块类型
type ChunkType string

const (
	ChunkTypeText   ChunkType = "text"
	ChunkTypeError  ChunkType = "error"
	ChunkTypeStatus ChunkType = "status"
)

// Chunk 流式输出块
type Chunk struct {
	Type    ChunkType
	Content string
}

// ExecutionRequest 统一的执行请求
type ExecutionRequest struct {
	Config     *model.AgentRoleConfig
	BaseAgent  *model.BaseAgent
	Context    *ContextLayers
	Input      string
	WorkDir    string
	SessionKey string // 用于会话恢复（空表示新会话）
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Output     string
	SessionKey string // 返回的会话标识（用于后续恢复）
}

// SessionExecutor 会话执行器接口，扩展了AgentAdapter的会话管理能力
type SessionExecutor interface {
	StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error
	ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error
	StopSession(sessionID string) error
	GetSessionStatus(sessionID string) SessionStatus
}
```

- [ ] **Step 2: Verify file compiles**

Run: `cd D:/00-codes/isdp/isdp && go build ./internal/service/agent/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add isdp/internal/service/agent/types.go
git commit -m "feat: add ExecutionRequest, ExecutionResult, Chunk types for unified adapter interface"
```

---

## Task 2: Redefine AgentAdapter Interface

**Files:**
- Modify: `isdp/internal/service/agent/adapter.go`

- [ ] **Step 1: Update AgentAdapter interface**

Replace the existing interface with the new unified interface:

```go
package agent

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
)

// AgentAdapter Agent适配器接口 - 统一的执行和会话管理接口
type AgentAdapter interface {
	// Execute 执行单次任务（无会话上下文）
	Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error)

	// ExecuteWithStream 流式执行，实时回调输出
	ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) (*ExecutionResult, error)

	// StartSession 启动交互式会话
	StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error

	// ResumeSession 恢复会话，发送新消息
	ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error

	// StopSession 停止会话
	StopSession(sessionID string) error

	// GetSessionStatus 获取会话状态
	GetSessionStatus(sessionID string) SessionStatus

	// CheckHealth 检查CLI健康状态
	CheckHealth(ctx context.Context) error
}

// NewAdapter 根据基础Agent类型创建适配器
func NewAdapter(baseAgent *model.BaseAgent) AgentAdapter {
	if baseAgent == nil {
		return nil
	}

	switch baseAgent.Type {
	case model.BaseAgentTypeClaudeCode:
		return NewClaudeAdapter(baseAgent)
	case model.BaseAgentTypeOpenCode:
		return NewOpenCodeAdapter(baseAgent)
	default:
		return nil
	}
}
```

- [ ] **Step 2: Remove duplicate helper functions at the end**

Keep only `min` helper, remove `timePtr` if it exists in adapter.go (it should be in other files).

- [ ] **Step 3: Verify file compiles**

Run: `cd D:/00-codes/isdp/isdp && go build ./internal/service/agent/`
Expected: Compilation errors expected - adapters need to implement new interface

- [ ] **Step 4: Commit**

```bash
git add isdp/internal/service/agent/adapter.go
git commit -m "refactor: redefine AgentAdapter interface with unified Execute methods"
```

---

## Task 3: Rewrite ClaudeAdapter with Session Management

**Files:**
- Modify: `isdp/internal/service/agent/claude_adapter.go`

- [ ] **Step 1: Add session management fields to ClaudeAdapter struct**

Add after the existing struct definition:

```go
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
	id         string
	sessionKey string // CLI的session-id
	cmd        *exec.Cmd
	ctx        context.Context
	cancel     context.CancelFunc
	status     SessionStatus
}
```

- [ ] **Step 2: Update NewClaudeAdapter constructor**

```go
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
```

- [ ] **Step 3: Implement Execute method**

```go
// Execute 执行单次任务（无会话上下文）
func (a *ClaudeAdapter) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	result, err := a.ExecuteWithStream(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return result, nil
}
```

- [ ] **Step 4: Rewrite ExecuteWithStream to use ExecutionRequest**

Replace the existing ExecuteWithStream signature with:

```go
// ExecuteWithStream 流式执行
func (a *ClaudeAdapter) ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) (*ExecutionResult, error) {
	prompt := a.buildPromptFromRequest(req)

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--permission-mode", "auto",
	}

	if req.BaseAgent != nil && req.BaseAgent.DefaultModel != "" {
		args = append(args, "--model", req.BaseAgent.DefaultModel)
	}

	// 会话恢复逻辑
	sessionKey := req.SessionKey
	if sessionKey != "" {
		args = append(args, "--resume", sessionKey)
		logInfo("Claude: Resuming session", zap.String("sessionKey", sessionKey))
	} else {
		// 新会话，生成 sessionKey
		sessionKey = uuid.New().String()
		args = append(args, "--session-id", sessionKey)
		logInfo("Claude: Starting new session", zap.String("sessionKey", sessionKey))
	}

	cmd := exec.CommandContext(ctx, a.cliPath, args...)
	cmd.Stdin = strings.NewReader(prompt)

	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	env := a.buildEnv()
	cmd.Env = env

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
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			text := a.parseStreamJSONLine(line)
			if text != "" && onChunk != nil {
				onChunk(Chunk{Type: ChunkTypeText, Content: text})
			}
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if stderrOutput.Len() > 0 {
			return nil, fmt.Errorf("CLI error: %s", stderrOutput.String())
		}
		return nil, fmt.Errorf("CLI execution failed: %w", err)
	}

	return &ExecutionResult{SessionKey: sessionKey}, nil
}
```

- [ ] **Step 5: Implement StartSession method**

```go
// StartSession 启动交互式会话
func (a *ClaudeAdapter) StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := &claudeSession{
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
```

- [ ] **Step 6: Implement ResumeSession method**

```go
// ResumeSession 恢复会话
func (a *ClaudeAdapter) ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error {
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
```

- [ ] **Step 7: Implement StopSession method**

```go
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
```

- [ ] **Step 8: Implement GetSessionStatus method**

```go
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
```

- [ ] **Step 9: Add helper methods**

```go
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

// parseStreamJSONLine 解析 stream-json 格式的单行输出
func (a *ClaudeAdapter) parseStreamJSONLine(line string) string {
	var msg struct {
		Type   string `json:"type"`
		Event  struct {
			Type  string `json:"type"`
			Delta struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"delta"`
		} `json:"event"`
		Result string `json:"result"`
	}

	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return ""
	}

	switch msg.Type {
	case "stream_event":
		if msg.Event.Type == "content_block_delta" && msg.Event.Delta.Type == "text_delta" {
			return msg.Event.Delta.Text
		}
	case "result":
		return msg.Result
	}

	return ""
}

// buildEnv 构建环境变量
func (a *ClaudeAdapter) buildEnv() []string {
	env := os.Environ()
	env = append(env, "CLAUDE_NO_INTERACTIVE=1")
	if a.apiURL != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_API_URL=%s", a.apiURL))
	}
	if a.apiToken != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", a.apiToken))
	}
	if a.gitBashPath != "" {
		env = append(env, fmt.Sprintf("CLAUDE_CODE_GIT_BASH_PATH=%s", a.gitBashPath))
	}
	return env
}
```

- [ ] **Step 10: Remove old Execute and buildPrompt methods**

Remove the old `Execute`, `buildPrompt`, `runCLI` methods that use old signatures.

- [ ] **Step 11: Verify file compiles**

Run: `cd D:/00-codes/isdp/isdp && go build ./internal/service/agent/`
Expected: No errors

- [ ] **Step 12: Commit**

```bash
git add isdp/internal/service/agent/claude_adapter.go
git commit -m "refactor: rewrite ClaudeAdapter with internal session management"
```

---

## Task 4: Rewrite OpenCodeAdapter with Session Management

**Files:**
- Modify: `isdp/internal/service/agent/open_code_adapter.go`

- [ ] **Step 1: Add session management fields to OpenCodeAdapter struct**

```go
// OpenCodeAdapter OpenCode CLI适配器
type OpenCodeAdapter struct {
	cliPath     string
	apiURL      string
	apiToken    string
	gitBashPath string
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
	status     SessionStatus
}
```

- [ ] **Step 2: Update NewOpenCodeAdapter constructor**

```go
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
```

- [ ] **Step 3: Implement Execute method**

```go
// Execute 执行单次任务
func (a *OpenCodeAdapter) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	result, err := a.ExecuteWithStream(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return result, nil
}
```

- [ ] **Step 4: Rewrite ExecuteWithStream**

```go
// ExecuteWithStream 流式执行
func (a *OpenCodeAdapter) ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) (*ExecutionResult, error) {
	if req.BaseAgent == nil || req.BaseAgent.DefaultModel == "" {
		return nil, fmt.Errorf("model name is required for OpenCode adapter")
	}

	prompt := a.buildPromptFromRequest(req)

	args := []string{
		"run",
		"--format", "json",
		"--model", req.BaseAgent.DefaultModel,
	}

	sessionKey := req.SessionKey
	if sessionKey != "" {
		args = append(args, "--session", sessionKey)
		logInfo("OpenCode: Resuming session", zap.String("sessionKey", sessionKey))
	} else {
		logInfo("OpenCode: Starting new session - will extract from output")
	}

	cmd := exec.CommandContext(ctx, a.cliPath, args...)
	cmd.Stdin = strings.NewReader(prompt)

	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	cmd.Env = a.buildEnv()

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

	var wg sync.WaitGroup
	var stderrOutput strings.Builder
	extractedSessionKey := sessionKey

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
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			// 提取 sessionKey
			if extractedSessionKey == "" {
				if sk := a.extractSessionIDFromJSON(line); sk != "" {
					extractedSessionKey = sk
					logInfo("OpenCode: Extracted sessionKey", zap.String("sessionKey", sk))
				}
			}

			// 解析文本
			text := a.extractTextFromJSON(line)
			if text != "" && onChunk != nil {
				onChunk(Chunk{Type: ChunkTypeText, Content: text})
			}
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if stderrOutput.Len() > 0 {
			return nil, fmt.Errorf("CLI error: %s", stderrOutput.String())
		}
		return nil, fmt.Errorf("CLI execution failed: %w", err)
	}

	return &ExecutionResult{SessionKey: extractedSessionKey}, nil
}
```

- [ ] **Step 5: Implement session management methods**

```go
// StartSession 启动交互式会话
func (a *OpenCodeAdapter) StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	result, err := a.ExecuteWithStream(ctx, req, nil)
	if err != nil {
		return err
	}

	a.sessions[sessionID] = &openCodeSession{
		id:         sessionID,
		sessionKey: result.SessionKey,
		status:     SessionStatusRunning,
	}

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

	if session, exists := a.sessions[sessionID]; exists {
		session.status = SessionStatusStopped
		delete(a.sessions, sessionID)
	}
	return nil
}

// GetSessionStatus 获取会话状态
func (a *OpenCodeAdapter) GetSessionStatus(sessionID string) SessionStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if session, exists := a.sessions[sessionID]; exists {
		return session.status
	}
	return SessionStatusStopped
}
```

- [ ] **Step 6: Add helper methods**

```go
// buildPromptFromRequest 从ExecutionRequest构建提示词
func (a *OpenCodeAdapter) buildPromptFromRequest(req *ExecutionRequest) string {
	// 如果是恢复会话，只返回用户输入
	if req.SessionKey != "" {
		return req.Input
	}

	// 首次调用：系统提示 + 用户输入
	var sb strings.Builder

	if req.Context != nil && req.Context.Layer0 != "" {
		sb.WriteString(req.Context.Layer0)
		sb.WriteString("\n\n")
	}

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
```

- [ ] **Step 7: Remove old methods with old signatures**

Remove the old `Execute`, `buildPrompt`, `runCLI` methods.

- [ ] **Step 8: Verify file compiles**

Run: `cd D:/00-codes/isdp/isdp && go build ./internal/service/agent/`
Expected: No errors

- [ ] **Step 9: Commit**

```bash
git add isdp/internal/service/agent/open_code_adapter.go
git commit -m "refactor: rewrite OpenCodeAdapter with internal session management"
```

---

## Task 5: Simplify ExecutionService

**Files:**
- Modify: `isdp/internal/service/agent/execution_service.go`

- [ ] **Step 1: Remove sessionIDs map and ExecutionContext**

Remove the following from the struct:
- `sessionIDs map[uuid.UUID]string`
- `executionContext ExecutionContext`

Remove from constructor:
- `sessionIDs: make(map[uuid.UUID]string)`
- `executionContext` parameter

- [ ] **Step 2: Remove sessionManager field**

Remove:
- `sessionManager *SessionManager`

And related code in constructor.

- [ ] **Step 3: Remove spawnInteractiveAgent method**

Delete the entire `spawnInteractiveAgent` method.

- [ ] **Step 4: Simplify SpawnAgent method**

Replace the switch statement with direct call to unified execution:

```go
// SpawnAgent 启动Agent - 统一入口
func (es *ExecutionService) SpawnAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
	// 获取Agent配置
	config, baseAgent, err := es.resolveConfigAndBaseAgent(ctx, req)
	if err != nil {
		return nil, err
	}

	// 创建调用记录
	invocation := es.createInvocation(ctx, req, config)

	// 记录运行中的Agent
	es.mu.Lock()
	es.runningAgents[invocation.ID] = &RunningAgent{
		InvocationID: invocation.ID,
		ThreadID:     req.ThreadID,
		AgentConfig:  config,
		BaseAgent:    baseAgent,
		StartedAt:    time.Now(),
		CancelFunc:   nil, // Will be set in executeAgent
	}
	es.mu.Unlock()

	// 广播状态更新
	es.broadcastStatus(req.ThreadID, invocation.ID, "started", config.Role)

	// 异步执行Agent（统一执行路径）
	go es.executeAgent(ctx, config, baseAgent, req, invocation)

	return invocation, nil
}
```

- [ ] **Step 5: Rewrite executeAgent method**

```go
// executeAgent 执行Agent
func (es *ExecutionService) executeAgent(ctx context.Context, config *model.AgentRoleConfig, baseAgent *model.BaseAgent, req *SpawnRequest, invocation *model.AgentInvocation) {
	defer func() {
		if r := recover(); r != nil {
			es.handleAgentError(ctx, invocation, fmt.Errorf("panic: %v", r))
		}
		es.mu.Lock()
		delete(es.runningAgents, invocation.ID)
		es.mu.Unlock()
	}()

	// 获取适配器
	adapter, err := es.getAdapter(ctx, config, baseAgent)
	if err != nil {
		es.handleAgentError(ctx, invocation, err)
		return
	}

	// 构建上下文
	contextLayers, err := es.buildContextLayers(ctx, req.ThreadID, config)
	if err != nil {
		es.handleAgentError(ctx, invocation, err)
		return
	}

	// 构建执行请求
	execReq := &ExecutionRequest{
		Config:    config,
		BaseAgent: baseAgent,
		Context:   contextLayers,
		Input:     req.Input,
		WorkDir:   req.ProjectPath,
	}

	// 执行并流式输出
	var outputBuilder strings.Builder
	_, err = adapter.ExecuteWithStream(ctx, execReq, func(chunk Chunk) {
		if chunk.Type == ChunkTypeText {
			outputBuilder.WriteString(chunk.Content)
		}
		es.broadcastChunk(req.ThreadID, invocation.ID, chunk, config.ID.String(), config.Name)
	})

	if err != nil {
		es.handleAgentError(ctx, invocation, err)
		return
	}

	output := outputBuilder.String()

	// 更新调用记录
	invocation.Status = model.InvocationStatusCompleted
	invocation.Output = output
	invocation.CompletedAt = timePtr(time.Now())
	es.invocationRepo.Update(ctx, invocation)

	// 保存消息
	es.saveAgentMessage(ctx, req.ThreadID, config, output)

	// 广播完成状态
	es.broadcastStatus(req.ThreadID, invocation.ID, "completed", config.Role)

	// 尝试路由到下一个Agent（如果没有工作流模板则跳过）
	es.tryRouteToNextAgent(ctx, req.ThreadID, config, output)
}
```

- [ ] **Step 6: Update broadcastChunk to accept Chunk**

```go
// broadcastChunk 广播输出块
func (es *ExecutionService) broadcastChunk(threadID, invocationID uuid.UUID, chunk Chunk, agentID, agentName string) {
	if es.wsHub != nil {
		es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_output_chunk",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"invocationId": invocationID.String(),
				"chunk":        chunk.Content,
				"chunkType":    string(chunk.Type),
				"agentId":      agentID,
				"agentName":    agentName,
			},
		})
	}
}
```

- [ ] **Step 7: Add helper methods**

```go
// resolveConfigAndBaseAgent 解析配置和BaseAgent
func (es *ExecutionService) resolveConfigAndBaseAgent(ctx context.Context, req *SpawnRequest) (*model.AgentRoleConfig, *model.BaseAgent, error) {
	var config *model.AgentRoleConfig
	var err error

	if req.ConfigID != uuid.Nil {
		config, err = es.configSvc.GetByID(ctx, req.ConfigID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get agent config: %w", err)
		}
	} else {
		config, err = es.configSvc.GetDefaultByRole(ctx, req.Role)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get default agent config: %w", err)
		}
	}

	var baseAgent *model.BaseAgent
	if config.BaseAgentID != uuid.Nil && es.baseAgentSvc != nil {
		baseAgent, err = es.baseAgentSvc.GetByID(ctx, config.BaseAgentID)
		if err != nil {
			baseAgent = nil // 不阻止执行
		}
	}

	return config, baseAgent, nil
}

// saveAgentMessage 保存Agent消息
func (es *ExecutionService) saveAgentMessage(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string) {
	metadata, _ := json.Marshal(map[string]string{
		"agentName": config.Name,
		"agentRole": string(config.Role),
	})
	msg := &model.Message{
		ThreadID:    threadID,
		Role:        model.MessageRoleAgent,
		AgentID:     config.ID.String(),
		Content:     output,
		MessageType: model.MessageTypeText,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
	}
	if err := es.msgRepo.Create(ctx, msg); err != nil {
		logError("Failed to save agent message", zap.Error(err))
	}
}
```

- [ ] **Step 8: Remove obsolete methods**

Remove:
- `StartSession`, `ResumeSession`, `StopSession`, `GetSessionStatus` methods (these are now in adapters)
- `mergeConfig` method (inline in resolveConfigAndBaseAgent)

- [ ] **Step 9: Verify file compiles**

Run: `cd D:/00-codes/isdp/isdp && go build ./internal/service/agent/`
Expected: No errors

- [ ] **Step 10: Commit**

```bash
git add isdp/internal/service/agent/execution_service.go
git commit -m "refactor: simplify ExecutionService to use unified execution path"
```

---

## Task 6: Simplify Orchestrator

**Files:**
- Modify: `isdp/internal/service/agent/orchestrator.go`

- [ ] **Step 1: Remove interactiveManager field**

Remove from struct:
```go
interactiveManager *InteractiveSessionManager
```

Remove from constructor initialization.

- [ ] **Step 2: Remove InteractiveSession related methods**

Delete:
- `StartInteractiveSession`
- `SendMessageToSession`
- `StopInteractiveSession`
- `GetInteractiveSession`

- [ ] **Step 3: Verify file compiles**

Run: `cd D:/00-codes/isdp/isdp && go build ./internal/service/agent/`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add isdp/internal/service/agent/orchestrator.go
git commit -m "refactor: remove interactiveManager from Orchestrator"
```

---

## Task 7: Update AgentHandler Debug Endpoint

**Files:**
- Modify: `isdp/internal/api/agent_handler.go`

- [ ] **Step 1: Update Debug method to use SpawnAgent**

Replace the Debug method implementation:

```go
// Debug 调试Agent - 使用统一的SpawnAgent入口
func (h *AgentHandler) Debug(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req DebugRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取Agent配置
	config, err := h.configSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
		return
	}

	// 确定threadId
	var debugThreadID uuid.UUID
	if req.ThreadID != "" {
		debugThreadID, err = uuid.Parse(req.ThreadID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid threadId"})
			return
		}
	} else {
		debugThreadID = uuid.New()
		if h.threadRepo != nil {
			debugProjectID := uuid.MustParse("6b7bc6f8-bbc8-42bc-ae9e-5bab22a98931")
			debugThread := &model.Thread{
				ID:        debugThreadID,
				ProjectID: debugProjectID,
				Status:    "debug",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := h.threadRepo.Create(c.Request.Context(), debugThread); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create debug thread"})
				return
			}
		}
	}

	// 使用统一的SpawnAgent入口
	invocation, err := h.orchestrator.SpawnAgent(c.Request.Context(), &agent.SpawnRequest{
		ThreadID:    debugThreadID,
		ConfigID:    config.ID,
		Role:        config.Role,
		Input:       req.Input,
		ProjectPath: req.ProjectPath,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := &DebugResponse{
		InvocationID: invocation.ID.String(),
		ThreadID:     debugThreadID.String(),
	}

	c.JSON(http.StatusOK, response)
}
```

- [ ] **Step 2: Remove ContinueDebug method**

Delete the `ContinueDebug` method since session management is now handled internally by adapters.

- [ ] **Step 3: Update route registration**

Remove the continue debug route:
```go
// Remove: agents.POST("/debug/:threadId/continue", h.ContinueDebug)
```

- [ ] **Step 4: Verify file compiles**

Run: `cd D:/00-codes/isdp/isdp && go build ./internal/api/`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add isdp/internal/api/agent_handler.go
git commit -m "refactor: update Debug endpoint to use unified SpawnAgent"
```

---

## Task 8: Delete Obsolete Files

**Files:**
- Delete: `isdp/internal/service/agent/interactive_session.go`
- Delete: `isdp/internal/service/agent/session_manager.go`
- Delete: `isdp/internal/service/agent/execution_context.go`

- [ ] **Step 1: Delete interactive_session.go**

```bash
rm isdp/internal/service/agent/interactive_session.go
```

- [ ] **Step 2: Delete session_manager.go**

```bash
rm isdp/internal/service/agent/session_manager.go
```

- [ ] **Step 3: Delete execution_context.go**

```bash
rm isdp/internal/service/agent/execution_context.go
```

- [ ] **Step 4: Verify project compiles**

Run: `cd D:/00-codes/isdp/isdp && go build ./...`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: remove obsolete session management files"
```

---

## Task 9: Final Verification

- [ ] **Step 1: Run all tests**

Run: `cd D:/00-codes/isdp/isdp && go test ./...`
Expected: All tests pass

- [ ] **Step 2: Run the application**

Run: `cd D:/00-codes/isdp/isdp && go run ./cmd/server/`
Expected: Server starts without errors

- [ ] **Step 3: Final commit if needed**

```bash
git status
# If there are uncommitted changes:
git add -A
git commit -m "fix: final cleanup for agent adapter unification"
```

---

## Summary

This plan transforms the Agent execution architecture from:

```
工作流: ExecutionService → adapter.ExecuteWithStream()
调试: Orchestrator → InteractiveSessionManager → CLI进程
```

To:

```
统一: ExecutionService.SpawnAgent() → adapter.ExecuteWithStream()
```

Key benefits:
1. Single entry point for both workflow and debug scenarios
2. Session management encapsulated in Adapter layer
3. Easy to add new Agent types (just implement AgentAdapter interface)
4. No execution context distinction needed - routing naturally depends on workflow template existence