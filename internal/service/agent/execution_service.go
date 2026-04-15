package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/mention"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// Agent 执行超时时间（10分钟）
	agentExecutionTimeout = 10 * time.Minute
	// 工具执行心跳间隔（30秒更新一次 LastActiveAt）
	toolHeartbeatInterval = 30 * time.Second
	// 进程终止优雅期限（3秒后 SIGKILL）
	killGracePeriod = 3 * time.Second
)

// MaxA2ADepth A2A 最大深度限制
const MaxA2ADepth = 15

// killChild 终止 CLI 进程，先 SIGTERM，3秒后 SIGKILL
func killChild(cmd *exec.Cmd, cmdMu *sync.Mutex) {
	cmdMu.Lock()
	defer cmdMu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	logInfo("killChild: terminating process", zap.Int("pid", cmd.Process.Pid))

	// Windows 不支持 SIGTERM，直接用 Kill
	if runtime.GOOS == "windows" {
		cmd.Process.Kill()
		return
	}

	// Unix: 先 SIGTERM
	cmd.Process.Signal(syscall.SIGTERM)

	// 3秒后升级到 SIGKILL
	go func(pid int, cmd *exec.Cmd, cmdMu *sync.Mutex) {
		time.Sleep(killGracePeriod)
		cmdMu.Lock()
		defer cmdMu.Unlock()
		if cmd.Process != nil && cmd.Process.Pid == pid {
			logInfo("killChild: escalating to SIGKILL", zap.Int("pid", pid))
			cmd.Process.Kill()
		}
	}(cmd.Process.Pid, cmd, cmdMu)
}

// AgentInfo 触发者信息（A2A 优化）
type AgentInfo struct {
	ID   uuid.UUID
	Name string
	Role string
}

// A2AContext A2A 上下文，用于追踪深度和去重
type A2AContext struct {
	Depth           int                // 当前深度
	InvokedAgents   map[uuid.UUID]bool // 已调用的 Agent ID 集合
	CompletedAgents map[uuid.UUID]bool // 已完成的 Agent ID 集合（用于汇聚判断）
	FromAgent       *AgentInfo         // 触发者信息（谁 @ 的下游 Agent）
}

// ThreadContext 预加载的 Thread 上下文，避免重复数据库查询
type ThreadContext struct {
	Thread           *model.Thread
	Project          *model.Project
	WorkflowTemplate *model.WorkflowTemplate
	WorkflowAgentIDs []string
	Transitions      []model.Transition
	AllowedAgents    []*model.AgentRoleConfig
	LoadedAt         time.Time
}

// SessionRecorder 会话记录器接口（用于解耦 a2a 包，避免循环导入）
type SessionRecorder interface {
	RecordFailedSession(threadID, configID, sessionID string)
	RecordSuccessfulSession(threadID, configID, sessionID string)
}

// 全局 SessionRecorder 实例（通过 SetSessionRecorder 设置）
var globalSessionRecorder SessionRecorder

// SetSessionRecorder 设置全局 SessionRecorder（在程序启动时由 a2a 包调用）
func SetSessionRecorder(recorder SessionRecorder) {
	globalSessionRecorder = recorder
}

// ExecutionService 统一执行服务，整合Orchestrator和InteractiveSession的功能
type ExecutionService struct {
	invocationRepo *repo.AgentInvocationRepository
	threadRepo     *repo.ThreadRepository
	msgRepo        *repo.MessageRepository
	configSvc      *ConfigService
	baseAgentSvc   *BaseAgentService
	baseAgentRepo  *repo.BaseAgentRepository // 直接访问repo获取完整BaseAgent（含ApiToken）
	tracker        *InvocationTracker
	workflow       *WorkflowEngine
	workflowRepo   *repo.WorkflowTemplateRepository
	projectRepo    *repo.ProjectRepository
	wsHub          *ws.Hub
	defaultAdapter AgentAdapter

	// Mention 解析器（支持动态 patterns）
	mentionParser *mention.Parser

	// 后台执行支持：内容块持久化
	contentBlockRepo    *repo.ContentBlockRepository
	contentBlockBuffer  []model.InvocationContentBlock // 缓冲区
	lastFlush           time.Time                      // 上次刷新时间
	contentBlockFlushMu sync.Mutex                     // 保护缓冲区

	runningAgents map[uuid.UUID]*RunningAgent
	mu            sync.RWMutex

	// A2A 上下文追踪
	a2aContexts map[uuid.UUID]*A2AContext // threadID -> A2AContext
	a2aMu       sync.RWMutex

	// Thread 上下文缓存（避免重复查询）
	threadContexts map[uuid.UUID]*ThreadContext
	tcMu           sync.RWMutex

	// CLI 会话ID缓存（用于 --resume 复用会话，避免冷启动延迟）
	// key: "threadID:agentID" -> value: sessionID
	cliSessions map[string]string
	csMu        sync.RWMutex

	// ChunkListeners 外部 chunk 监听器（如飞书 IM 转发）
	chunkListeners   []ChunkListener
	chunkListenersMu sync.RWMutex
}

// NewExecutionService 创建统一执行服务
func NewExecutionService(
	invocationRepo *repo.AgentInvocationRepository,
	threadRepo *repo.ThreadRepository,
	msgRepo *repo.MessageRepository,
	configSvc *ConfigService,
	baseAgentSvc *BaseAgentService,
	baseAgentRepo *repo.BaseAgentRepository,
	tracker *InvocationTracker,
	workflow *WorkflowEngine,
	workflowRepo *repo.WorkflowTemplateRepository,
	projectRepo *repo.ProjectRepository,
	wsHub *ws.Hub,
	defaultAdapter AgentAdapter,
	mentionParser *mention.Parser,
	contentBlockRepo *repo.ContentBlockRepository,
) *ExecutionService {
	es := &ExecutionService{
		invocationRepo:     invocationRepo,
		threadRepo:         threadRepo,
		msgRepo:            msgRepo,
		configSvc:          configSvc,
		baseAgentSvc:       baseAgentSvc,
		baseAgentRepo:      baseAgentRepo,
		tracker:            tracker,
		workflow:           workflow,
		workflowRepo:       workflowRepo,
		projectRepo:        projectRepo,
		wsHub:              wsHub,
		defaultAdapter:     defaultAdapter,
		mentionParser:      mentionParser,
		contentBlockRepo:   contentBlockRepo,
		contentBlockBuffer: make([]model.InvocationContentBlock, 0, 20),
		lastFlush:          time.Now(),
		runningAgents:      make(map[uuid.UUID]*RunningAgent),
		a2aContexts:        make(map[uuid.UUID]*A2AContext),
		threadContexts:     make(map[uuid.UUID]*ThreadContext),
		cliSessions:        make(map[string]string),
		chunkListeners:     make([]ChunkListener, 0),
	}

	// 启动后台清理 goroutine，定期清理超时的 Agent
	go es.cleanupStaleAgents()

	return es
}

// AddChunkListener 注册外部 chunk 监听器
func (es *ExecutionService) AddChunkListener(listener ChunkListener) {
	es.chunkListenersMu.Lock()
	defer es.chunkListenersMu.Unlock()
	es.chunkListeners = append(es.chunkListeners, listener)
}

// NotifyChunkListeners 通知所有外部 chunk 监听器
func (es *ExecutionService) NotifyChunkListeners(threadID, invocationID uuid.UUID, chunk Chunk, agentID, agentName string) {
	es.chunkListenersMu.RLock()
	listeners := make([]ChunkListener, len(es.chunkListeners))
	copy(listeners, es.chunkListeners)
	es.chunkListenersMu.RUnlock()
	for _, listener := range listeners {
		go func(l ChunkListener) {
			defer func() {
				if r := recover(); r != nil {
					logError("chunk listener panic recovered", zap.Any("panic", r))
				}
			}()
			l(threadID, invocationID, chunk, agentID, agentName)
		}(listener)
	}
}

// cleanupStaleAgents 定期清理超时的 Agent entry
// 防止 goroutine 卡住导致 runningAgents 残留
// 超时判断考虑工具执行状态：有活跃工具调用时延长超时
func (es *ExecutionService) cleanupStaleAgents() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		es.mu.Lock()
		now := time.Now()
		for id, agent := range es.runningAgents {
			// 获取工具执行状态
			agent.HeartbeatMu.Lock()
			hasActiveTool := agent.ActiveToolCount > 0
			agent.HeartbeatMu.Unlock()

			// 超时判断：有活跃工具时使用更长超时（20分钟），否则使用默认超时（10分钟）
			timeout := agentExecutionTimeout
			if hasActiveTool {
				timeout = 2 * agentExecutionTimeout // 工具执行中，延长超时
			}

			inactiveDuration := now.Sub(agent.LastActiveAt)
			if inactiveDuration > timeout {
				logInfo("清理无活动 Agent",
					zap.String("invocationID", id.String()),
					zap.String("threadID", agent.ThreadID.String()),
					zap.Duration("inactiveTime", inactiveDuration),
					zap.Duration("totalRunningTime", now.Sub(agent.StartedAt)),
					zap.Bool("hadActiveTools", hasActiveTool))

				// 停止心跳 goroutine
				agent.HeartbeatMu.Lock()
				if agent.HeartbeatCancel != nil {
					agent.HeartbeatCancel()
				}
				agent.HeartbeatMu.Unlock()

				// 取消 goroutine
				agent.CancelFunc()
				delete(es.runningAgents, id)

				// 更新数据库状态为失败
				go es.markInvocationFailed(id, "agent inactive for timeout, no output activity")
			}
		}
		es.mu.Unlock()
	}
}

// markInvocationFailed 标记 invocation 为失败状态
func (es *ExecutionService) markInvocationFailed(invocationID uuid.UUID, reason string) {
	ctx := context.Background()
	invocation, err := es.invocationRepo.FindByID(ctx, invocationID)
	if err != nil {
		logError("Failed to get invocation for timeout cleanup", zap.Error(err))
		return
	}

	invocation.Status = model.InvocationStatusFailed
	invocation.Output = reason
	invocation.CompletedAt = timePtr(time.Now())
	if err := es.invocationRepo.Update(ctx, invocation); err != nil {
		logError("Failed to update invocation status on timeout", zap.Error(err))
	}

	es.broadcastStatus(invocation.ThreadID, invocation.ID, "failed", invocation.Role, "", invocation.AgentConfigID.String(), "")
}

// SpawnAgent 启动Agent（统一执行入口）
func (es *ExecutionService) SpawnAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
	spawnStart := time.Now()

	// 解析配置和BaseAgent
	resolveStart := time.Now()
	config, baseAgent, err := es.resolveConfigAndBaseAgent(ctx, req)
	if err != nil {
		return nil, err
	}
	logInfo("[PERF] resolveConfigAndBaseAgent", zap.Duration("duration", time.Since(resolveStart)))

	// 如果 SessionStrategy 未设置，自动判断是否应该使用 resume
	// 场景：用户通过前端直接 Spawn（如 @mention），需要自动判断会话策略
	if req.SessionStrategy == "" {
		req.SessionStrategy, req.SessionID = es.shouldAutoResume(ctx, req.ThreadID, config.ID)
		if req.SessionStrategy == SessionStrategyResume {
			logInfo("SpawnAgent: 自动判断使用 resume 会话策略",
				zap.String("agentName", config.Name),
				zap.String("agentID", config.ID.String()),
				zap.String("threadID", req.ThreadID.String()))
		}
	}

	// 创建调用记录
	invocationCreateStart := time.Now()
	now := time.Now()
	invocation := &model.AgentInvocation{
		ID:            uuid.New(),
		ThreadID:      req.ThreadID,
		AgentConfigID: config.ID,
		Role:          config.Role,
		AgentName:     config.Name, // 存储 Agent 名称，用于历史显示
		Status:        model.InvocationStatusPending,
		Input:         req.Input,
		CreatedAt:     now,
		StartedAt:     &now, // 设置开始时间，用于历史显示耗时
	}

	if err := es.invocationRepo.Create(ctx, invocation); err != nil {
		return nil, fmt.Errorf("failed to create invocation: %w", err)
	}
	logInfo("[PERF] createInvocationRecord", zap.Duration("duration", time.Since(invocationCreateStart)))

	logInfo("[PERF] SpawnAgent total", zap.Duration("duration", time.Since(spawnStart)), zap.String("invocationID", invocation.ID.String()))

	// 创建上下文 - 使用独立的context，不受HTTP请求生命周期影响
	agentCtx, cancel := context.WithCancel(context.Background())

	// 记录运行中的Agent
	es.mu.Lock()
	es.runningAgents[invocation.ID] = &RunningAgent{
		InvocationID:    invocation.ID,
		ThreadID:        req.ThreadID,
		AgentConfig:     config,
		BaseAgent:       baseAgent,
		StartedAt:       time.Now(),
		LastActiveAt:    time.Now(), // 初始化活动时间
		CancelFunc:      cancel,
		ActiveToolCount: 0, // 初始化工具计数
	}
	es.mu.Unlock()

	// 广播状态更新
	es.broadcastStatus(req.ThreadID, invocation.ID, "started", config.Role, config.Name, config.ID.String(), req.Input)

	// 异步执行Agent
	go es.executeAgent(agentCtx, invocation, config, baseAgent, req)

	return invocation, nil
}

// executeAgent 执行Agent（统一执行路径）
func (es *ExecutionService) executeAgent(ctx context.Context, invocation *model.AgentInvocation, config *model.AgentRoleConfig, baseAgent *model.BaseAgent, req *SpawnRequest) {
	startTime := time.Now()
	defer func() {
		// 确保刷新内容块缓冲区（后台执行支持）
		es.flushContentBlocks(invocation.ID)

		// 恢复可能的panic
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in executeAgent: %v", r)
			es.handleAgentError(ctx, invocation, err)
		}
		es.mu.Lock()
		delete(es.runningAgents, invocation.ID)
		es.mu.Unlock()
		logInfo("executeAgent completed",
			zap.String("invocationID", invocation.ID.String()),
			zap.Duration("totalDuration", time.Since(startTime)))
	}()

	logInfo("executeAgent started", zap.String("invocationID", invocation.ID.String()))

	// 立即更新状态为 running，确保状态同步
	invocation.Status = model.InvocationStatusRunning
	if err := es.invocationRepo.Update(ctx, invocation); err != nil {
		logError("Failed to update invocation status to running", zap.Error(err))
	}
	es.broadcastStatus(req.ThreadID, invocation.ID, "running", config.Role, config.Name, config.ID.String(), "")

	// 构建上下文
	contextStart := time.Now()
	contextLayers, err := es.buildContextLayers(ctx, req.ThreadID, config, req)
	if err != nil {
		logError("buildContextLayers failed", zap.Error(err))
		es.handleAgentError(ctx, invocation, fmt.Errorf("failed to build context layers: %w", err))
		return
	}
	logInfo("[PERF] buildContextLayers", zap.Duration("duration", time.Since(contextStart)))

	// 构建完整 prompt 并存储（用于调用日志显示）
	fullPrompt := es.formatFullPrompt(contextLayers, req.Input)
	invocation.FullPrompt = fullPrompt
	if err := es.invocationRepo.Update(ctx, invocation); err != nil {
		logError("Failed to update invocation with full prompt", zap.Error(err))
	}

	// 广播完整 prompt 更新（用于前端调用日志显示）
	es.broadcastFullPrompt(req.ThreadID, invocation.ID, fullPrompt)

	// 获取适配器
	adapterStart := time.Now()
	adapter, err := es.getAdapter(ctx, config, baseAgent)
	if err != nil {
		logError("getAdapter failed", zap.Error(err))
		es.handleAgentError(ctx, invocation, fmt.Errorf("failed to get adapter: %w", err))
		return
	}
	logInfo("[PERF] getAdapter", zap.Duration("duration", time.Since(adapterStart)))

	// 保存 adapter 引用到 RunningAgent（用于取消时获取 Cmd）
	es.mu.Lock()
	if agent, ok := es.runningAgents[invocation.ID]; ok {
		agent.Adapter = adapter
	}
	es.mu.Unlock()

	// 构建ExecutionRequest
	execReqBuildStart := time.Now()

	// 根据会话策略决定是否使用 --resume
	// 跨角色调用（SessionStrategyNew）：不传递历史，使用新会话
	// 同角色调用（SessionStrategyResume）：传递历史，尝试恢复会话
	sessionKey := fmt.Sprintf("%s:%s", req.ThreadID.String(), config.ID.String())
	var sessionID string
	if req.SessionStrategy == SessionStrategyResume {
		// 同角色调用：尝试恢复会话
		es.csMu.RLock()
		sessionID = es.cliSessions[sessionKey]
		es.csMu.RUnlock()
		if sessionID == "" && req.SessionID != "" {
			sessionID = req.SessionID
			logInfo("使用请求中的 SessionID（从数据库获取）",
				zap.String("sessionKey", sessionKey),
				zap.String("sessionId", sessionID))
		}
		logInfo("A2A 会话策略: resume，尝试复用会话",
			zap.String("sessionKey", sessionKey),
			zap.String("sessionId", sessionID),
			zap.Bool("hasSession", sessionID != ""))
	} else {
		// 跨角色调用或默认：不使用会话缓存，确保新会话
		logInfo("A2A 会话策略: new，使用新会话（不传递历史）",
			zap.String("sessionKey", sessionKey))
	}

	execReq := &ExecutionRequest{
		Config:          config,
		BaseAgent:       baseAgent,
		Context:         contextLayers,
		Input:           req.Input,
		WorkDir:         req.ProjectPath,
		ConfigDir:       config.ConfigPath,
		SessionID:       sessionID,
		SessionStrategy: req.SessionStrategy,
		InvocationID:    invocation.ID, // 用于 AskUserQuestion 答案发送
	}
	logInfo("[PERF] buildExecutionRequest", zap.Duration("duration", time.Since(execReqBuildStart)), zap.String("sessionID", sessionID), zap.String("sessionStrategy", string(req.SessionStrategy)))

	// CLI 执行阶段（这是主要耗时点）
	cliStart := time.Now()
	logInfo("[PERF] CLI execution starting", zap.String("invocationID", invocation.ID.String()))

	// 使用流式执行，实时广播输出
	var outputBuilder strings.Builder
	result, err := adapter.ExecuteWithStream(ctx, execReq, func(chunk Chunk) {
		outputBuilder.WriteString(chunk.Content)
		// 实时广播输出块
		es.broadcastChunk(req.ThreadID, invocation.ID, chunk, config.ID.String(), config.Name)
	})

	// 会话恢复失败降级机制
	if err != nil && sessionID != "" && isResumeFallbackError(err) {
		logWarn("Session resume failed, falling back to new session",
			zap.String("invocationId", invocation.ID.String()),
			zap.String("sessionId", sessionID),
			zap.Error(err))

		// 清除缓存的 sessionID
		es.csMu.Lock()
		delete(es.cliSessions, sessionKey)
		es.csMu.Unlock()

		// 降级：使用新会话重试
		execReq.SessionID = ""
		outputBuilder.Reset()
		result, err = adapter.ExecuteWithStream(ctx, execReq, func(chunk Chunk) {
			outputBuilder.WriteString(chunk.Content)
			es.broadcastChunk(req.ThreadID, invocation.ID, chunk, config.ID.String(), config.Name)
		})

		if err == nil {
			logInfo("Session fallback succeeded, created new session",
				zap.String("invocationId", invocation.ID.String()))
		}
	}

	cliDuration := time.Since(cliStart)
	logInfo("[PERF] CLI execution completed", zap.Duration("duration", cliDuration), zap.String("invocationID", invocation.ID.String()))

	if err != nil {
		logError("Adapter.ExecuteWithStream failed", zap.Error(err))
		// 执行失败时也保存 sessionId 用于问题定位
		if result != nil && result.SessionID != "" {
			// 保存 sessionId 到 invocation（入库用于问题定位）
			invocation.SessionID = result.SessionID
			if updateErr := es.invocationRepo.Update(ctx, invocation); updateErr != nil {
				logError("Failed to save sessionId on failure", zap.Error(updateErr))
			}
			// 记录失败会话（通过 sessionRecorder）
			if globalSessionRecorder != nil {
				globalSessionRecorder.RecordFailedSession(req.ThreadID.String(), config.ID.String(), result.SessionID)
			}
		}
		es.handleAgentErrorWithContext(ctx, invocation, fmt.Errorf("adapter execution failed: %w", err), execReq)
		return
	}

	// 保存会话ID供后续复用（避免冷启动延迟）
	if result != nil && result.SessionID != "" {
		es.csMu.Lock()
		es.cliSessions[sessionKey] = result.SessionID
		es.csMu.Unlock()
		// 同时保存到 invocation 对象（持久化到数据库用于问题定位）
		invocation.SessionID = result.SessionID
		// 记录成功会话（通过 sessionRecorder）
		if globalSessionRecorder != nil {
			globalSessionRecorder.RecordSuccessfulSession(req.ThreadID.String(), config.ID.String(), result.SessionID)
		}
		logInfo("Session ID saved for future resume and persistence", zap.String("sessionKey", sessionKey), zap.String("sessionId", result.SessionID))
	}

	output := outputBuilder.String()
	logInfo("Execution completed", zap.Int("outputLength", len(output)))

	// 更新调用记录
	invocation.Status = model.InvocationStatusCompleted
	invocation.Output = output
	invocation.CompletedAt = timePtr(time.Now())
	if err := es.invocationRepo.Update(ctx, invocation); err != nil {
		logError("Failed to update invocation", zap.Error(err))
	}

	// 获取累积的内容块
	var contentBlocks []ContentBlockData
	es.mu.Lock()
	if agent, ok := es.runningAgents[invocation.ID]; ok {
		agent.ContentBlocksMu.Lock()
		contentBlocks = make([]ContentBlockData, len(agent.AccumulatedContentBlocks))
		copy(contentBlocks, agent.AccumulatedContentBlocks)
		agent.ContentBlocksMu.Unlock()
	}
	es.mu.Unlock()

	// 保存输出消息到数据库（包含内容块）
	msg := es.saveAgentMessageWithReturn(ctx, req.ThreadID, config, output, contentBlocks)

	// 广播消息（让前端用真实 ID 更新）
	if msg != nil {
		es.broadcastAgentMessage(req.ThreadID, msg, config.Name, string(config.Role))
	}

	// 广播完成状态
	es.broadcastStatus(req.ThreadID, invocation.ID, "completed", config.Role, config.Name, config.ID.String(), "")

	// 检查是否需要路由到下一个Agent
	es.checkRouting(ctx, req.ThreadID, config, output)

	// Agent 完成后，从 InvokedAgents 中移除，允许再次被调用（支持反馈循环）
	es.a2aMu.Lock()
	if a2aCtx, exists := es.a2aContexts[req.ThreadID]; exists {
		delete(a2aCtx.InvokedAgents, config.ID)
		logInfo("A2A: Agent 完成后从 InvokedAgents 移除，允许再次被调用",
			zap.String("agentId", config.ID.String()),
			zap.String("agentName", config.Name),
			zap.String("threadId", req.ThreadID.String()))
	}
	es.a2aMu.Unlock()
}

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
	if config.BaseAgentID != uuid.Nil && es.baseAgentRepo != nil {
		// 直接使用repo获取完整BaseAgent（包含ApiToken）
		baseAgent, err = es.baseAgentRepo.FindByID(ctx, config.BaseAgentID)
		if err != nil {
			baseAgent = nil // 获取失败，尝试使用默认
		}
	}

	// 如果角色未指定基础Agent或获取失败，使用默认基础Agent
	// 注意：这里直接使用 repo.FindDefault 获取完整信息（含 ApiToken）
	// 不能使用 baseAgentSvc.GetDefault，因为它会 sanitize 清除 ApiToken
	if baseAgent == nil && es.baseAgentRepo != nil {
		baseAgent, err = es.baseAgentRepo.FindDefault(ctx)
		if err != nil {
			logInfo("No default base agent found", zap.Error(err))
			baseAgent = nil
		} else if baseAgent != nil {
			logInfo("Using default base agent",
				zap.String("id", baseAgent.ID.String()),
				zap.String("name", baseAgent.Name),
				zap.Bool("hasApiToken", baseAgent.ApiToken != ""))
		}
	}

	return config, baseAgent, nil
}

// saveAgentMessage 保存Agent消息
func (es *ExecutionService) saveAgentMessage(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string, contentBlocks []ContentBlockData) {
	metadata, _ := json.Marshal(map[string]string{
		"agentName": config.Name,
		"agentRole": string(config.Role),
	})

	// 序列化内容块
	var contentBlocksJSON []byte
	if len(contentBlocks) > 0 {
		contentBlocksJSON, _ = json.Marshal(contentBlocks)
	}

	msg := &model.Message{
		ThreadID:      threadID,
		Role:          model.MessageRoleAgent,
		AgentID:       config.ID.String(),
		Content:       output,
		ContentBlocks: contentBlocksJSON,
		MessageType:   model.MessageTypeText,
		Metadata:      metadata,
		CreatedAt:     time.Now(),
	}
	if err := es.msgRepo.Create(ctx, msg); err != nil {
		logError("Failed to save agent message", zap.Error(err))
	}
}

// saveAgentMessageWithReturn 保存Agent消息并返回消息对象（含真实ID）
func (es *ExecutionService) saveAgentMessageWithReturn(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string, contentBlocks []ContentBlockData) *model.Message {
	metadata, _ := json.Marshal(map[string]string{
		"agentName": config.Name,
		"agentRole": string(config.Role),
	})

	// 序列化内容块
	var contentBlocksJSON []byte
	if len(contentBlocks) > 0 {
		contentBlocksJSON, _ = json.Marshal(contentBlocks)
	}

	msg := &model.Message{
		ThreadID:      threadID,
		Role:          model.MessageRoleAgent,
		AgentID:       config.ID.String(),
		Content:       output,
		ContentBlocks: contentBlocksJSON,
		MessageType:   model.MessageTypeText,
		Metadata:      metadata,
		CreatedAt:     time.Now(),
	}
	if err := es.msgRepo.Create(ctx, msg); err != nil {
		logError("Failed to save agent message", zap.Error(err))
		return nil
	}
	return msg
}

// broadcastAgentMessage 广播Agent消息（让前端用真实ID更新）
func (es *ExecutionService) broadcastAgentMessage(threadID uuid.UUID, msg *model.Message, agentName, agentRole string) {
	if es.wsHub != nil {
		// 解析内容块
		var contentBlocks []ContentBlockData
		if len(msg.ContentBlocks) > 0 {
			json.Unmarshal(msg.ContentBlocks, &contentBlocks)
		}

		es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_message",
			ThreadID:  threadID.String(),
			Timestamp: msg.CreatedAt.UnixMilli(),
			Payload: map[string]interface{}{
				"messageId":     msg.ID.String(),
				"agentId":       msg.AgentID,
				"content":       msg.Content,
				"contentBlocks": contentBlocks,
				"agentName":     agentName,
				"agentRole":     agentRole,
			},
		})
	}
}

// getAdapter 获取适配器
func (es *ExecutionService) getAdapter(ctx context.Context, config *model.AgentRoleConfig, baseAgent *model.BaseAgent) (AgentAdapter, error) {
	// 如果有 BaseAgent，使用它创建适配器
	if baseAgent != nil {
		adapter := NewAdapter(baseAgent)
		if adapter == nil {
			return nil, fmt.Errorf("不支持的基础Agent类型: %s", baseAgent.Type)
		}
		return adapter, nil
	}

	// 如果配置了BaseAgentID但没有传入baseAgent，尝试获取
	if config.BaseAgentID != uuid.Nil && es.baseAgentRepo != nil {
		ba, err := es.baseAgentRepo.FindByID(ctx, config.BaseAgentID)
		if err == nil {
			adapter := NewAdapter(ba)
			if adapter != nil {
				return adapter, nil
			}
		}
		// 如果获取失败，继续尝试使用默认适配器
	}

	// 向后兼容：使用默认适配器
	if es.defaultAdapter != nil {
		return es.defaultAdapter, nil
	}

	return nil, errors.New("未找到可用的基础Agent，请先设置一个默认的基础Agent")
}

// handleAgentError 处理Agent错误
func (es *ExecutionService) handleAgentError(ctx context.Context, invocation *model.AgentInvocation, err error) {
	invocation.Status = model.InvocationStatusFailed
	invocation.Output = err.Error()
	invocation.CompletedAt = timePtr(time.Now())
	if updateErr := es.invocationRepo.Update(ctx, invocation); updateErr != nil {
		logError("Failed to update invocation on error", zap.Error(updateErr))
	}

	es.broadcastStatus(invocation.ThreadID, invocation.ID, "failed", invocation.Role, "", invocation.AgentConfigID.String(), "")
}

// handleAgentErrorWithContext 处理 Agent 错误（带执行请求上下文，用于详细诊断）
func (es *ExecutionService) handleAgentErrorWithContext(ctx context.Context, invocation *model.AgentInvocation, err error, execReq *ExecutionRequest) {
	// 输出详细诊断日志
	es.logErrorDiagnostics(ctx, invocation, err, execReq)

	invocation.Status = model.InvocationStatusFailed
	invocation.Output = fmt.Sprintf("执行失败: %s\n\n详细信息请查看 server.log", err.Error())
	invocation.CompletedAt = timePtr(time.Now())
	if updateErr := es.invocationRepo.Update(ctx, invocation); updateErr != nil {
		logError("Failed to update invocation on error", zap.Error(updateErr))
	}

	es.broadcastStatus(invocation.ThreadID, invocation.ID, "failed", invocation.Role, "", invocation.AgentConfigID.String(), "")
}

// isResumeFallbackError 判断是否为可降级的 resume 错误
func isResumeFallbackError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	fallbackPatterns := []string{
		"session not found",
		"session expired",
		"context too large",
		"invalid session",
		"no such session",
		"session corrupt",
	}
	for _, pattern := range fallbackPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// logErrorDiagnostics 输出详细的错误诊断日志
func (es *ExecutionService) logErrorDiagnostics(ctx context.Context, invocation *model.AgentInvocation, err error, execReq *ExecutionRequest) {
	// 获取运行时信息
	var historyLength int
	var inputLength int
	var workDir, configDir, model string
	var sessionID string
	var isResumeAttempt bool

	if execReq != nil {
		inputLength = len(execReq.Input)
		if execReq.Context != nil && execReq.Context.Layer1 != "" {
			historyLength = strings.Count(execReq.Context.Layer1, "\n")
		}
		workDir = execReq.WorkDir
		configDir = execReq.ConfigDir
		sessionID = execReq.SessionID
		isResumeAttempt = sessionID != ""
		if execReq.BaseAgent != nil {
			model = execReq.BaseAgent.DefaultModel
		}
	}

	// 生成建议
	suggestions := generateErrorSuggestions(err, sessionID, isResumeAttempt)

	// 构建诊断日志
	logError("Agent 执行失败 - 诊断报告",
		// 基本信息
		zap.String("invocationId", invocation.ID.String()),
		zap.String("threadId", invocation.ThreadID.String()),
		zap.String("agentId", invocation.AgentConfigID.String()),
		zap.String("agentName", invocation.AgentName),
		zap.String("agentRole", string(invocation.Role)),

		// 错误信息
		zap.String("errorType", getErrorType(err)),
		zap.String("errorMessage", err.Error()),

		// 上下文诊断
		zap.Int("inputLength", inputLength),
		zap.Int("historyLength", historyLength),
		zap.String("sessionId", sessionID),
		zap.Bool("isResumeAttempt", isResumeAttempt),

		// CLI 诊断
		zap.String("workDir", workDir),
		zap.String("configDir", configDir),
		zap.String("model", model),
		zap.Duration("duration", time.Since(invocation.CreatedAt)),

		// 建议
		zap.Strings("suggestions", suggestions))
}

// getErrorType 从错误中提取类型
func getErrorType(err error) string {
	if err == nil {
		return "unknown"
	}
	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "timeout"):
		return "cli_timeout"
	case strings.Contains(errStr, "session"):
		return "session_error"
	case strings.Contains(errStr, "api"):
		return "api_error"
	case strings.Contains(errStr, "context") || strings.Contains(errStr, "token"):
		return "context_overflow"
	case strings.Contains(errStr, "permission"):
		return "permission_error"
	default:
		return "unknown"
	}
}

// generateErrorSuggestions 根据错误类型生成建议
func generateErrorSuggestions(err error, sessionID string, isResumeAttempt bool) []string {
	var suggestions []string
	errType := getErrorType(err)

	switch errType {
	case "session_error":
		suggestions = append(suggestions, "会话恢复失败，已尝试降级到新会话")
		suggestions = append(suggestions, "如问题持续，建议清理旧会话缓存")
	case "cli_timeout":
		suggestions = append(suggestions, "执行超时，可能是任务过于复杂")
		suggestions = append(suggestions, "建议简化任务或增加超时时间")
	case "api_error":
		suggestions = append(suggestions, "API 调用失败，请检查 API 配置和网络连接")
	case "context_overflow":
		suggestions = append(suggestions, "上下文过长，建议开启新会话")
		suggestions = append(suggestions, "或减少历史消息长度")
	case "permission_error":
		suggestions = append(suggestions, "权限不足，请检查工作目录访问权限")
	default:
		suggestions = append(suggestions, "请检查日志获取详细信息")
	}

	if isResumeAttempt && sessionID != "" {
		suggestions = append(suggestions, fmt.Sprintf("会话恢复尝试失败 (sessionID: %s)", sessionID[:8]+"..."))
	}

	return suggestions
}

// loadThreadContext 预加载 Thread 上下文，一次性获取所有需要的数据
// 避免在后续流程中重复查询数据库
func (es *ExecutionService) loadThreadContext(ctx context.Context, threadID uuid.UUID) (*ThreadContext, error) {
	tc := &ThreadContext{
		LoadedAt: time.Now(),
	}

	// 1. 获取 Thread
	thread, err := es.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to find thread: %w", err)
	}
	tc.Thread = thread

	// 2. 获取 Project（并行）
	var project *model.Project
	if es.projectRepo != nil {
		project, _ = es.projectRepo.GetByThreadID(ctx, threadID)
	}
	tc.Project = project

	// 3. 确定工作流模板ID
	var workflowTemplateID *uuid.UUID
	if project != nil && project.WorkflowTemplateID != nil {
		workflowTemplateID = project.WorkflowTemplateID
	} else if thread.WorkflowTemplateID != nil {
		workflowTemplateID = thread.WorkflowTemplateID
	}

	// 4. 获取工作流模板和 Agent 列表
	if workflowTemplateID != nil && es.workflowRepo != nil {
		workflow, err := es.workflowRepo.FindByID(ctx, *workflowTemplateID)
		if err == nil && workflow != nil {
			tc.WorkflowTemplate = workflow

			// 解析 AgentIDs
			if len(workflow.AgentIDs) > 0 {
				var agentIDs []string
				if err := json.Unmarshal(workflow.AgentIDs, &agentIDs); err == nil {
					tc.WorkflowAgentIDs = agentIDs
				}
			}

			// 解析 Transitions
			if len(workflow.Transitions) > 0 {
				var transitions []model.Transition
				if err := json.Unmarshal(workflow.Transitions, &transitions); err == nil {
					tc.Transitions = transitions
				}
			}
		}
	}

	// 5. 获取所有 Agent 配置（一次性查询）
	if len(tc.WorkflowAgentIDs) > 0 {
		var agents []*model.AgentRoleConfig
		for _, idStr := range tc.WorkflowAgentIDs {
			id, err := uuid.Parse(idStr)
			if err != nil {
				continue
			}
			agent, err := es.configSvc.GetByID(ctx, id)
			if err == nil {
				agents = append(agents, agent)
			}
		}
		tc.AllowedAgents = agents
	}

	// 6. 缓存上下文
	es.tcMu.Lock()
	es.threadContexts[threadID] = tc
	es.tcMu.Unlock()

	logInfo("loadThreadContext: loaded",
		zap.String("threadID", threadID.String()),
		zap.Int("agentCount", len(tc.AllowedAgents)),
		zap.Int("transitionCount", len(tc.Transitions)))

	return tc, nil
}

// getThreadContext 获取 Thread 上下文（优先使用缓存）
func (es *ExecutionService) getThreadContext(ctx context.Context, threadID uuid.UUID) (*ThreadContext, error) {
	// 检查缓存
	es.tcMu.RLock()
	tc, exists := es.threadContexts[threadID]
	es.tcMu.RUnlock()

	if exists && time.Since(tc.LoadedAt) < 5*time.Minute {
		return tc, nil
	}

	// 缓存不存在或过期，重新加载
	return es.loadThreadContext(ctx, threadID)
}

// ClearThreadContext 清除 Thread 上下文缓存（Thread 状态变化时调用）
func (es *ExecutionService) ClearThreadContext(threadID uuid.UUID) {
	es.tcMu.Lock()
	delete(es.threadContexts, threadID)
	es.tcMu.Unlock()
}

// buildContextLayers 构建上下文层
func (es *ExecutionService) buildContextLayers(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, req *SpawnRequest) (*ContextLayers, error) {
	layers := &ContextLayers{}

	// 预加载上下文（一次性获取所有数据）
	tc, err := es.getThreadContext(ctx, threadID)
	if err != nil {
		return nil, err
	}

	// Layer 0: 系统提示（使用缓存的上下文）
	layers.Layer0 = es.buildDynamicSystemPromptFromContext(tc, config)

	// Layer 1: Thread历史 - 根据会话策略决定是否传递
	// A2A 机制下，无论是跨角色还是同角色，都不需要传递历史消息：
	// - 跨角色（SessionStrategyNew）：CLI 使用新会话，历史不相关
	// - 同角色（SessionStrategyResume）：CLI --resume 自动恢复历史，无需重复传递
	if req != nil && (req.SessionStrategy == SessionStrategyNew || req.SessionStrategy == SessionStrategyResume) {
		layers.Layer1 = "" // A2A 调用不传递历史
	} else {
		// 非 A2A 调用（用户直接触发）：使用结构化提取而非完整历史
		messages, err := es.msgRepo.FindByThreadID(ctx, threadID, 100)
		if err != nil {
			return nil, err
		}
		layers.Layer1 = es.extractStructuredHistory(messages, 50)
	}

	// Layer 2: 工作产物（使用缓存的 Thread）
	layers.Layer2 = es.getArtifacts(tc.Thread)

	// Layer 3: 环境信息（使用缓存的 Thread）
	layers.Layer3 = es.getEnvironmentInfo(tc.Thread)

	return layers, nil
}

// roleTriggerHints 根据角色自动生成的触发提示
// key 对应数据库中 agent_configs.role 字段的值
var roleTriggerHints = map[string]string{
	"requirement_analyst": "@需求分析师 当需要需求分析时",
	"architect":           "@架构师 当需要架构设计时",
	"frontend_developer":  "@前端开发工程师 当需要前端实现时",
	"backend_developer":   "@后端开发工程师 当需要后端实现时",
	"code_reviewer":       "@代码审查工程师 当需要代码审查时",
	"test_engineer":       "@测试工程师 当需要测试时",
	"sre_engineer":        "@运维工程师 当需要部署运维时",
	"project_manager":     "@项目经理 当需要项目协调时",
	"ui_designer":         "@UI设计师 当需要界面设计时",
	"database_designer":   "@数据库设计师 当需要数据库设计时",
	"security_engineer":   "@安全工程师 当需要安全审计时",
	"tech_writer":         "@技术文档工程师 当需要文档编写时",
}

// generateTriggerHint 根据目标 Agent 生成触发提示
func generateTriggerHint(toAgent *model.AgentRoleConfig) string {
	// 优先使用角色预设
	if hint, ok := roleTriggerHints[string(toAgent.Role)]; ok {
		return hint
	}
	// 兜底：使用 Agent 名称
	return fmt.Sprintf("@%s", toAgent.Name)
}

// buildDynamicSystemPromptFromContext 使用预加载的上下文构建动态系统提示
func (es *ExecutionService) buildDynamicSystemPromptFromContext(tc *ThreadContext, config *model.AgentRoleConfig) string {
	var sb strings.Builder

	// 原始系统提示
	sb.WriteString(config.SystemPrompt)

	// 从缓存中过滤当前 Agent 的转换规则
	var transitions []model.Transition
	agentIDStr := config.ID.String()
	for _, t := range tc.Transitions {
		if t.FromAgentID == agentIDStr {
			transitions = append(transitions, t)
		}
	}

	// 构建 Agent ID -> AgentConfig 映射（使用缓存）
	agentMap := make(map[string]*model.AgentRoleConfig)
	for _, agent := range tc.AllowedAgents {
		agentMap[agent.ID.String()] = agent
	}

	// 注入协作提示（使用 trigger_hint 或智能生成）
	if len(transitions) > 0 {
		sb.WriteString("\n\n## 下游协作方（需要时 @ 触发）\n")
		sb.WriteString("**重要格式规则**：@mention 必须单独成行，不能嵌入句子中。\n")
		sb.WriteString("正确示例：\n```\n@后端开发工程师 请实现用户登录 API\n```\n")
		sb.WriteString("错误示例：\n```\n确认后我将 @后端开发工程师 进行实现  ← 无效，不会触发\n```\n\n")
		sb.WriteString("可用的下游协作方：\n")
		for _, t := range transitions {
			toAgent := agentMap[t.ToAgentID]
			var hint string

			// 优先使用用户填写的 trigger_hint
			if t.TriggerHint != "" {
				hint = t.TriggerHint
			} else if toAgent != nil {
				// 智能生成
				hint = generateTriggerHint(toAgent)
			} else {
				hint = fmt.Sprintf("@%s", t.ToAgentID[:8])
			}

			sb.WriteString(fmt.Sprintf("- %s\n", hint))
		}
		sb.WriteString("\n**角色边界**：你的职责是输出分析结果，不要直接进行代码实现。实现工作由下游协作方负责。\n")
	}

	// 注入出口检查提示
	sb.WriteString("\n\n## 发送消息前的出口检查\n")
	sb.WriteString("回复前问\"到我这里结束了吗？\"\n")
	sb.WriteString("- 如果不是，想想谁需要接下来处理 → @ 对方\n")
	sb.WriteString("- @ 前三问自检（短路规则）：\n")
	sb.WriteString("  1. 需要对方采取行动？= 是 → 直接 @（跳过后续问题）\n")
	sb.WriteString("  2. 对方需要知道这个信息？\n")
	sb.WriteString("  3. 会影响对方的工作？\n")
	sb.WriteString("  - 三个都否 → 不 @\n")

	return sb.String()
}

// buildDynamicSystemPrompt 构建动态系统提示，注入工作流协作关系
// 注意：此方法会进行数据库查询，建议使用 buildDynamicSystemPromptFromContext
func (es *ExecutionService) buildDynamicSystemPrompt(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig) string {
	var sb strings.Builder

	// 原始系统提示
	sb.WriteString(config.SystemPrompt)

	// 获取当前 Agent 的转换规则
	transitions := es.getTransitionsForAgent(ctx, threadID, config.ID)

	// DEBUG: 打印 transitions 数量
	fmt.Printf("[DEBUG] buildDynamicSystemPrompt: agent=%s, transitions=%d\n", config.Name, len(transitions))

	// 获取工作流中的所有 Agent，用于生成 trigger_hint
	allowedAgents := es.getAllowedAgentsFromWorkflow(ctx, threadID)
	agentMap := make(map[string]*model.AgentRoleConfig)
	for _, agent := range allowedAgents {
		agentMap[agent.ID.String()] = agent
	}

	// 注入协作提示（使用 trigger_hint 或智能生成）
	if len(transitions) > 0 {
		sb.WriteString("\n\n## 下游协作方（需要时 @ 触发）\n")
		sb.WriteString("**重要格式规则**：@mention 必须单独成行，不能嵌入句子中。\n")
		sb.WriteString("正确示例：\n```\n@后端开发工程师 请实现用户登录 API\n```\n")
		sb.WriteString("错误示例：\n```\n确认后我将 @后端开发工程师 进行实现  ← 无效，不会触发\n```\n\n")
		sb.WriteString("可用的下游协作方：\n")
		for _, t := range transitions {
			toAgent := agentMap[t.ToAgentID]
			var hint string

			// 优先使用用户填写的 trigger_hint
			if t.TriggerHint != "" {
				hint = t.TriggerHint
			} else if toAgent != nil {
				// 智能生成
				hint = generateTriggerHint(toAgent)
			} else {
				hint = fmt.Sprintf("@%s", t.ToAgentID[:8])
			}

			fmt.Printf("[DEBUG] buildDynamicSystemPrompt: 注入 hint=%s\n", hint)
			sb.WriteString(fmt.Sprintf("- %s\n", hint))
		}
		sb.WriteString("\n**角色边界**：你的职责是输出分析结果，不要直接进行代码实现。实现工作由下游协作方负责。\n")
	}

	// 注入出口检查提示
	sb.WriteString("\n\n## 发送消息前的出口检查\n")
	sb.WriteString("回复前问\"到我这里结束了吗？\"\n")
	sb.WriteString("- 如果不是，想想谁需要接下来处理 → @ 对方\n")
	sb.WriteString("- @ 前三问自检（短路规则）：\n")
	sb.WriteString("  1. 需要对方采取行动？= 是 → 直接 @（跳过后续问题）\n")
	sb.WriteString("  2. 对方需要知道这个信息？\n")
	sb.WriteString("  3. 会影响对方的工作？\n")
	sb.WriteString("  - 三个都否 → 不 @\n")

	return sb.String()
}

// getTransitionsForAgent 获取当前 Agent 的转换规则
func (es *ExecutionService) getTransitionsForAgent(ctx context.Context, threadID uuid.UUID, agentConfigID uuid.UUID) []model.Transition {
	// 优先使用缓存
	es.tcMu.RLock()
	tc, exists := es.threadContexts[threadID]
	es.tcMu.RUnlock()

	if !exists || len(tc.Transitions) == 0 {
		// 缓存不存在，尝试加载
		var err error
		tc, err = es.loadThreadContext(ctx, threadID)
		if err != nil {
			return nil
		}
	}

	// 过滤出当前 Agent 作为源头的转换规则
	var result []model.Transition
	agentIDStr := agentConfigID.String()
	for _, t := range tc.Transitions {
		if t.FromAgentID == agentIDStr {
			result = append(result, t)
		}
	}

	return result
}

// ClearA2AContext 清理 A2A 上下文（Thread 完成或取消时调用）
func (es *ExecutionService) ClearA2AContext(threadID uuid.UUID) {
	es.a2aMu.Lock()
	delete(es.a2aContexts, threadID)
	es.a2aMu.Unlock()
}

// checkRouting 检查路由
// 支持博弈场景：一个 mention 可能匹配多个 Agent
// mentionIDs 是解析出的 Agent ID 列表
func (es *ExecutionService) checkRouting(ctx context.Context, threadID uuid.UUID, currentConfig *model.AgentRoleConfig, output string) {
	mentionIDs := es.parseMentions(output)

	if len(mentionIDs) == 0 {
		// 检查信号路由
		es.checkSignalRouting(ctx, threadID, currentConfig, output)
		return
	}

	// 获取或创建 A2A 上下文
	es.a2aMu.Lock()
	a2aCtx, exists := es.a2aContexts[threadID]
	if !exists {
		a2aCtx = &A2AContext{
			Depth:           0,
			InvokedAgents:   make(map[uuid.UUID]bool),
			CompletedAgents: make(map[uuid.UUID]bool),
		}
		es.a2aContexts[threadID] = a2aCtx
	}
	es.a2aMu.Unlock()

	// 深度检查
	if a2aCtx.Depth >= MaxA2ADepth {
		logInfo("A2A 深度达到上限，停止路由",
			zap.String("threadId", threadID.String()),
			zap.Int("depth", a2aCtx.Depth))
		return
	}

	// 获取工作流模板中的 Agent 列表
	allowedAgents := es.getAllowedAgentsFromWorkflow(ctx, threadID)

	// 构建 Agent ID -> AgentConfig 映射（限制在当前团队内）
	agentMap := make(map[string]*model.AgentRoleConfig)
	for _, agent := range allowedAgents {
		agentMap[agent.ID.String()] = agent
	}

	// 获取项目路径
	var projectPath string
	if es.projectRepo != nil {
		project, err := es.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil {
			projectPath = project.LocalPath
		}
	}

	// 收集所有待触发的 Agent（支持博弈场景）
	agentsToTrigger := make(map[uuid.UUID]*model.AgentRoleConfig)

	for _, agentID := range mentionIDs {
		targetConfig, exists := agentMap[agentID]
		if !exists {
			logInfo("路由被拒绝：目标不在工作流团队中",
				zap.String("agentID", agentID),
				zap.String("threadId", threadID.String()))
			continue
		}

		// 去重检查：同一 Agent 不重复调用
		if a2aCtx.InvokedAgents[targetConfig.ID] {
			logInfo("A2A 去重：Agent 已被调用过",
				zap.String("agentId", targetConfig.ID.String()),
				zap.String("threadId", threadID.String()))
			continue
		}

		agentsToTrigger[targetConfig.ID] = targetConfig
	}

	// 批量触发 Agent
	for _, targetConfig := range agentsToTrigger {
		// 更新 A2A 上下文
		es.a2aMu.Lock()
		if a2aCtx.Depth >= MaxA2ADepth {
			es.a2aMu.Unlock()
			break
		}
		a2aCtx.Depth++
		a2aCtx.InvokedAgents[targetConfig.ID] = true
		// 设置触发者信息（A2A 优化）
		a2aCtx.FromAgent = &AgentInfo{
			ID:   currentConfig.ID,
			Name: currentConfig.Name,
			Role: string(currentConfig.Role),
		}
		es.a2aMu.Unlock()

		// 决定会话策略：跨角色使用新会话，同角色使用 resume
		var sessionStrategy SessionStrategy
		if a2aCtx.FromAgent != nil && a2aCtx.FromAgent.ID == targetConfig.ID {
			// 同一 Agent 再次调用 → 恢复会话
			sessionStrategy = SessionStrategyResume
			logInfo("A2A 会话策略: 同Agent调用，使用 resume",
				zap.String("fromAgent", a2aCtx.FromAgent.Name),
				zap.String("toAgent", targetConfig.Name))
		} else if a2aCtx.FromAgent != nil && a2aCtx.FromAgent.Role == string(targetConfig.Role) {
			// 同角色不同实例 → 恢复会话
			sessionStrategy = SessionStrategyResume
			logInfo("A2A 会话策略: 同角色调用，使用 resume",
				zap.String("fromRole", a2aCtx.FromAgent.Role),
				zap.String("toAgent", targetConfig.Name))
		} else {
			// 跨角色 → 新会话，不传递历史
			sessionStrategy = SessionStrategyNew
			logInfo("A2A 会话策略: 跨角色调用，使用新会话",
				zap.String("fromAgent", a2aCtx.FromAgent.Name),
				zap.String("fromRole", a2aCtx.FromAgent.Role),
				zap.String("toAgent", targetConfig.Name),
				zap.String("toRole", string(targetConfig.Role)))

			// 清除该 Agent 的会话缓存，确保不传递历史
			sessionKey := fmt.Sprintf("%s:%s", threadID.String(), targetConfig.ID.String())
			es.csMu.Lock()
			delete(es.cliSessions, sessionKey)
			es.csMu.Unlock()
		}

		logInfo("A2A 路由触发",
			zap.String("fromAgent", currentConfig.Name),
			zap.String("toAgent", targetConfig.Name),
			zap.Int("depth", a2aCtx.Depth),
			zap.String("threadId", threadID.String()),
			zap.String("sessionStrategy", string(sessionStrategy)))

		// 构建 A2A 输入（原始用户消息 + 前序响应上下文 + 触发者信息）
		// 简化调用 - 不传递 contentBlocks（前序输出已包含工具调用结果）
		a2aInput := es.buildA2AInput(ctx, threadID, currentConfig, a2aCtx, output, nil, sessionStrategy)

		// 使用构建的 A2A 输入
		es.SpawnAgent(ctx, &SpawnRequest{
			ThreadID:        threadID,
			ConfigID:        targetConfig.ID,
			Role:            targetConfig.Role,
			Input:           a2aInput,
			ProjectPath:     projectPath,
			SessionStrategy: sessionStrategy,
		})
	}

}

// parseMentions 解析@mention（仅匹配行首）
// 返回匹配的 Agent ID 列表
// 只在行首（可带空白缩进）的 @mention 才会触发 A2A 路由
func (es *ExecutionService) parseMentions(content string) []string {
	seen := make(map[string]bool) // 去重

	// 使用动态 MentionParser
	var mentionIDs []string
	if es.mentionParser != nil {
		var err error
		mentionIDs, err = es.mentionParser.Parse(context.Background(), content, "")
		if err != nil {
			logError("parseMentions: 解析失败", zap.Error(err))
			return nil
		}
	}

	// 去重
	result := make([]string, 0)
	for _, id := range mentionIDs {
		if id != "" && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}

	logInfo("parseMentions: 解析结果（仅行首）", zap.Int("count", len(result)), zap.Strings("agentIDs", result))

	return result
}

// buildA2AInput 构建 A2A 输入
// 参考 clowder-ai 的做法：传递原始用户消息 + 格式化的前序响应上下文
// 而不是传递包含 @mention 的原始输出
// 优化：新增协作规则、触发者信息、Agent 元信息
func (es *ExecutionService) buildA2AInput(
	ctx context.Context,
	threadID uuid.UUID,
	fromAgent *model.AgentRoleConfig,
	a2aCtx *A2AContext,
	output string,
	contentBlocks []ContentBlockData,
	sessionStrategy SessionStrategy,
) string {
	var sb strings.Builder

	// 1. 协作规则（有触发者信息时注入）
	if a2aCtx != nil && a2aCtx.FromAgent != nil {
		sb.WriteString("## 协作规则\n\n")
		sb.WriteString("A2A 出口检查：回复前问\"到我这里结束了吗？\"不是 → 谁需要动 → 末尾另起一行行首写 @句柄。\n\n")
		sb.WriteString("---\n\n")
	}

	// 2. 会话策略信息（新增）
	sb.WriteString("## 会话策略\n\n")
	if sessionStrategy == SessionStrategyResume {
		sb.WriteString("**类型**: Resume（恢复会话）\n")
		sb.WriteString("**说明**: CLI 将使用 --resume 恢复之前的会话上下文\n\n")
	} else {
		sb.WriteString("**类型**: New（新会话）\n")
		sb.WriteString("**说明**: CLI 将使用全新会话，不继承历史上下文\n\n")
	}
	sb.WriteString("---\n\n")

	// 3. 原始请求
	originalMessage := es.getLastUserMessage(ctx, threadID)
	if originalMessage != "" {
		sb.WriteString("## 原始请求\n\n")
		sb.WriteString(originalMessage)
		sb.WriteString("\n\n---\n\n")
	}

	// 4. 前序分析（使用结构化过滤）
	if fromAgent != nil {
		sb.WriteString("## 前序分析（结构化摘要）\n\n")
		sb.WriteString(fmt.Sprintf("**来自**: %s\n", fromAgent.Name))
		if fromAgent.Role != "" {
			sb.WriteString(fmt.Sprintf("**角色**: %s\n", es.getRoleDescription(fromAgent.Role)))
		}
		if fromAgent.Description != "" {
			sb.WriteString(fmt.Sprintf("**擅长**: %s\n", fromAgent.Description))
		}
		sb.WriteString("\n")

		// 结构化过滤后的输出
		filteredOutput := es.filterStructuredOutput(output, contentBlocks)
		sb.WriteString(filteredOutput)
		sb.WriteString("\n\n---\n\n")
	}

	// 5. 触发者信息
	if a2aCtx != nil && a2aCtx.FromAgent != nil {
		sb.WriteString(fmt.Sprintf("**Direct message from %s; reply to %s**\n",
			a2aCtx.FromAgent.Name,
			a2aCtx.FromAgent.Name))
	}

	return sb.String()
}

// getRoleDescription 获取角色的中文描述
func (es *ExecutionService) getRoleDescription(role model.AgentRole) string {
	descriptions := map[model.AgentRole]string{
		model.AgentRoleRequirement:       "需求分析专家",
		model.AgentRoleArchitect:         "架构设计专家",
		model.AgentRoleDeveloper:         "后端开发专家",
		model.AgentRoleReviewer:          "代码审查专家",
		model.AgentRoleTestEngineer:      "测试工程专家",
		model.AgentRoleDevOps:            "运维部署专家",
		model.AgentRoleFullstackEngineer: "全栈开发专家",
		model.AgentRoleCustom:            "自定义角色",
	}
	if desc, ok := descriptions[role]; ok {
		return desc
	}
	return string(role)
}

// getLastUserMessage 获取 Thread 中最近的用户消息
func (es *ExecutionService) getLastUserMessage(ctx context.Context, threadID uuid.UUID) string {
	if es.msgRepo == nil {
		return ""
	}

	messages, err := es.msgRepo.GetRecent(ctx, threadID, 10)
	if err != nil || len(messages) == 0 {
		return ""
	}

	// 从后往前找最后一条用户消息
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == model.MessageRoleUser {
			return messages[i].Content
		}
	}

	return ""
}

// stripPureMentionLines 移除输出中的纯 @mention 行
// 只移除"行首 @mention 后面没有实质内容"的行
// 避免移除"@后端 请帮忙"这种有效指令
func (es *ExecutionService) stripPureMentionLines(output string) string {
	lines := strings.Split(output, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")

		// 检查是否是纯 @mention 行
		if strings.HasPrefix(trimmed, "@") {
			// 找到 mention 结束位置
			mentionEnd := len(trimmed)
			for i, r := range trimmed[1:] {
				if r == ' ' || r == '\t' {
					mentionEnd = i + 1
					break
				}
			}

			// 如果 @mention 后面只有空白，则跳过这一行
			afterMention := strings.TrimSpace(trimmed[mentionEnd:])
			if afterMention == "" {
				continue
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// getAllowedAgentsFromWorkflow 从工作流模板获取允许路由的 Agent 列表
// 数据流: Thread → WorkflowTemplate → AgentIDs → AgentConfigs
func (es *ExecutionService) getAllowedAgentsFromWorkflow(ctx context.Context, threadID uuid.UUID) []*model.AgentRoleConfig {
	// 优先使用缓存
	es.tcMu.RLock()
	tc, exists := es.threadContexts[threadID]
	es.tcMu.RUnlock()

	if exists && len(tc.AllowedAgents) > 0 {
		return tc.AllowedAgents
	}

	// 缓存不存在，加载上下文
	tc, err := es.loadThreadContext(ctx, threadID)
	if err != nil {
		return nil
	}
	return tc.AllowedAgents
}

// findAgentByRole 在 Agent 列表中按角色查找
func (es *ExecutionService) findAgentByRole(agents []*model.AgentRoleConfig, role model.AgentRole) *model.AgentRoleConfig {
	for _, agent := range agents {
		if agent.Role == role {
			return agent
		}
	}
	return nil
}

// findAgentByName 在 Agent 列表中按名称查找
func (es *ExecutionService) findAgentByName(agents []*model.AgentRoleConfig, name string) *model.AgentRoleConfig {
	for _, agent := range agents {
		if agent.Name == name {
			return agent
		}
	}
	return nil
}

// checkSignalRouting 检查信号路由（混合模式：@mention + workflow配置）
// 支持三种路由类型：
// 1. sequence - 顺序执行：触发单个下游 Agent
// 2. parallel - 并行执行：同时触发多个下游 Agent（分支工作流）
// 3. merge - 汇聚执行：等待多个上游 Agent 完成后再执行
//
// 混合模式说明：
// - @mention 触发（优先）：解析输出中的 @mention 动态触发 Agent
// - workflow condition 触发：当 Transition.Condition 不为空且匹配时自动触发
// - workflow 建议流转：Condition 为空时仅作为建议（已在 systemPrompt 中注入）
//
// 博弈场景：一个 mention pattern 可能匹配多个 Agent，全部触发
func (es *ExecutionService) checkSignalRouting(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string) {
	// 获取或创建 A2A 上下文
	es.a2aMu.Lock()
	a2aCtx, exists := es.a2aContexts[threadID]
	if !exists {
		a2aCtx = &A2AContext{
			Depth:           0,
			InvokedAgents:   make(map[uuid.UUID]bool),
			CompletedAgents: make(map[uuid.UUID]bool),
		}
		es.a2aContexts[threadID] = a2aCtx
	}
	// 记录当前 Agent 已完成
	a2aCtx.CompletedAgents[config.ID] = true
	es.a2aMu.Unlock()

	// 深度检查
	if a2aCtx.Depth >= MaxA2ADepth {
		logInfo("A2A 深度达到上限，停止自动路由",
			zap.String("threadId", threadID.String()),
			zap.Int("depth", a2aCtx.Depth))
		return
	}

	// 获取项目路径
	var projectPath string
	if es.projectRepo != nil {
		project, err := es.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil {
			projectPath = project.LocalPath
		}
	}

	// 获取工作流模板中的 Agent 列表
	allowedAgents := es.getAllowedAgentsFromWorkflow(ctx, threadID)

	// 收集所有待触发的 Agent（支持并行和博弈）
	agentsToTrigger := make(map[uuid.UUID]*model.AgentRoleConfig) // 使用 map 去重

	// ========== 1. 解析输出中的 @mention（优先触发，支持博弈场景）==========
	// 使用 ParseForAgents 限制在当前工作流的 Agent 范围内
	var a2aMentions []string
	if es.mentionParser != nil {
		var err error
		a2aMentions, err = es.mentionParser.ParseForAgents(ctx, output, config.ID.String(), allowedAgents)
		if err != nil {
			logError("checkSignalRouting: 解析失败", zap.Error(err))
		}
	}
	logInfo("A2A @mention 解析结果（限制在工作流范围内）",
		zap.String("fromAgent", config.Name),
		zap.Strings("agentIDs", a2aMentions),
		zap.Int("count", len(a2aMentions)))

	// 构建 Agent ID -> AgentConfig 映射（限制在当前团队内）
	agentMap := make(map[string]*model.AgentRoleConfig)
	for _, agent := range allowedAgents {
		agentMap[agent.ID.String()] = agent
	}

	// 博弈场景：一个 mention 可能匹配多个 Agent
	for _, agentID := range a2aMentions {
		agent, exists := agentMap[agentID]
		if !exists {
			continue
		}

		// 去重检查
		if a2aCtx.InvokedAgents[agent.ID] {
			logInfo("A2A 去重：Agent 已被调用过",
				zap.String("agentId", agent.ID.String()),
				zap.String("source", "@mention"))
			continue
		}

		agentsToTrigger[agent.ID] = agent
		logInfo("A2A @mention 触发添加（博弈场景支持）",
			zap.String("fromAgent", config.Name),
			zap.String("toAgent", agent.Name),
			zap.String("agentID", agentID))
	}

	// ========== 2. 批量触发 Agent ==========
	for _, targetConfig := range agentsToTrigger {
		// 更新 A2A 上下文
		es.a2aMu.Lock()
		if a2aCtx.Depth >= MaxA2ADepth {
			es.a2aMu.Unlock()
			break
		}
		a2aCtx.Depth++
		a2aCtx.InvokedAgents[targetConfig.ID] = true
		// 设置触发者信息（A2A 优化）
		a2aCtx.FromAgent = &AgentInfo{
			ID:   config.ID,
			Name: config.Name,
			Role: string(config.Role),
		}
		es.a2aMu.Unlock()

		// 决定会话策略：跨角色使用新会话，同角色使用 resume
		var sessionStrategy SessionStrategy
		if a2aCtx.FromAgent != nil && a2aCtx.FromAgent.ID == targetConfig.ID {
			// 同一 Agent 再次调用 → 恢复会话
			sessionStrategy = SessionStrategyResume
			logInfo("A2A 会话策略: 同Agent调用，使用 resume",
				zap.String("fromAgent", a2aCtx.FromAgent.Name),
				zap.String("toAgent", targetConfig.Name))
		} else if a2aCtx.FromAgent != nil && a2aCtx.FromAgent.Role == string(targetConfig.Role) {
			// 同角色不同实例 → 恢复会话
			sessionStrategy = SessionStrategyResume
			logInfo("A2A 会话策略: 同角色调用，使用 resume",
				zap.String("fromRole", a2aCtx.FromAgent.Role),
				zap.String("toAgent", targetConfig.Name))
		} else {
			// 跨角色 → 新会话，不传递历史
			sessionStrategy = SessionStrategyNew
			logInfo("A2A 会话策略: 跨角色调用，使用新会话",
				zap.String("fromAgent", a2aCtx.FromAgent.Name),
				zap.String("fromRole", a2aCtx.FromAgent.Role),
				zap.String("toAgent", targetConfig.Name),
				zap.String("toRole", string(targetConfig.Role)))

			// 清除该 Agent 的会话缓存，确保不传递历史
			sessionKey := fmt.Sprintf("%s:%s", threadID.String(), targetConfig.ID.String())
			es.csMu.Lock()
			delete(es.cliSessions, sessionKey)
			es.csMu.Unlock()
		}

		logInfo("A2A 触发执行",
			zap.String("fromAgent", config.Name),
			zap.String("toAgent", targetConfig.Name),
			zap.Int("depth", a2aCtx.Depth),
			zap.String("threadId", threadID.String()),
			zap.String("sessionStrategy", string(sessionStrategy)))

		// 构建 A2A 输入（结构化摘要 + 会话策略）
		// 简化调用 - 不传递 contentBlocks（前序输出已包含工具调用结果）
		a2aInput := es.buildA2AInput(ctx, threadID, config, a2aCtx, output, nil, sessionStrategy)

		// 触发下一个 Agent
		es.SpawnAgent(ctx, &SpawnRequest{
			ThreadID:        threadID,
			ConfigID:        targetConfig.ID,
			Role:            targetConfig.Role,
			Input:           a2aInput,
			ProjectPath:     projectPath,
			SessionStrategy: sessionStrategy,
		})
	}
}

// formatMessages 格式化消息
func (es *ExecutionService) formatMessages(messages []*model.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		role := "用户"
		if msg.Role == model.MessageRoleAgent {
			role = msg.AgentID
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", role, msg.Content))
	}
	return sb.String()
}

// getArtifacts 获取工作产物
func (es *ExecutionService) getArtifacts(thread *model.Thread) string {
	// TODO: 实现获取工作产物
	return ""
}

// getEnvironmentInfo 获取环境信息
func (es *ExecutionService) getEnvironmentInfo(thread *model.Thread) string {
	return fmt.Sprintf("Thread ID: %s\n当前阶段: %s\n状态: %s",
		thread.ID, thread.CurrentPhase, thread.Status)
}

// CancelAgent 取消Agent
func (es *ExecutionService) CancelAgent(ctx context.Context, invocationID uuid.UUID) error {
	es.mu.Lock()
	agent, exists := es.runningAgents[invocationID]
	if exists {
		logInfo("CancelAgent: stopping agent",
			zap.String("invocationID", invocationID.String()),
			zap.Bool("hasCmd", agent.Cmd != nil),
			zap.Bool("hasAdapter", agent.Adapter != nil))

		// 1. 终止 CLI 进程
		// 优先使用保存的 Cmd，如果为空则从 adapter 获取
		cmd := agent.Cmd
		if cmd == nil && agent.Adapter != nil {
			cmd = agent.Adapter.GetCurrentProcess()
			logInfo("CancelAgent: got cmd from adapter",
				zap.Bool("cmdIsNil", cmd == nil))
		}
		if cmd != nil {
			logInfo("CancelAgent: calling killChild",
				zap.Int("pid", cmd.Process.Pid))
			killChild(cmd, &agent.cmdMu)
		} else {
			logWarn("CancelAgent: cmd is nil, cannot kill process")
		}

		// 2. 取消 Go goroutine
		agent.CancelFunc()
		delete(es.runningAgents, invocationID)
	}
	es.mu.Unlock()

	if !exists {
		return ErrAgentNotFound
	}

	// 3. 更新状态
	invocation, err := es.invocationRepo.FindByID(ctx, invocationID)
	if err != nil {
		return err
	}

	// 4. 清除 CLI session 缓存（避免下次复用残留状态的 session）
	if invocation.AgentConfigID != uuid.Nil {
		sessionKey := fmt.Sprintf("%s:%s", invocation.ThreadID.String(), invocation.AgentConfigID.String())
		es.csMu.Lock()
		delete(es.cliSessions, sessionKey)
		es.csMu.Unlock()
		logInfo("CancelAgent: cleared CLI session cache", zap.String("sessionKey", sessionKey))
	}

	invocation.Status = model.InvocationStatusCancelled
	invocation.CompletedAt = timePtr(time.Now())
	if err := es.invocationRepo.Update(ctx, invocation); err != nil {
		logError("Failed to update invocation status", zap.Error(err))
	}

	// 5. 广播取消状态
	es.broadcastStatus(invocation.ThreadID, invocation.ID, "cancelled", invocation.Role, "", invocation.AgentConfigID.String(), "")

	return nil
}

// broadcastStatus 广播状态
func (es *ExecutionService) broadcastStatus(threadID, invocationID uuid.UUID, status string, role model.AgentRole, agentName string, agentID string, input string) {
	logInfo("broadcastStatus called", zap.String("threadId", threadID.String()), zap.String("invocationId", invocationID.String()), zap.String("status", status))
	if es.wsHub != nil {
		payload := map[string]interface{}{
			"invocationId": invocationID.String(),
			"status":       status,
			"role":         string(role),
			"agentName":    agentName,
			"agentId":      agentID,
		}
		// 仅在 started 状态时包含 input
		if status == "started" && input != "" {
			payload["input"] = input
		}
		es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_status",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload:   payload,
		})
	}
}

// broadcastChunk 广播输出块（实时流式输出）
func (es *ExecutionService) broadcastChunk(threadID, invocationID uuid.UUID, chunk Chunk, agentID, agentName string) {
	logInfo("broadcastChunk called", zap.String("threadId", threadID.String()), zap.String("chunkType", string(chunk.Type)), zap.String("toolName", chunk.ToolName))

	// 更新 Agent 的最后活动时间，并处理工具执行状态
	es.mu.Lock()
	agent, exists := es.runningAgents[invocationID]
	if exists {
		agent.LastActiveAt = time.Now()

		// 累积文本输出（用于 WebSocket 重连恢复）
		if chunk.Type == ChunkTypeText && chunk.Content != "" {
			agent.OutputMu.Lock()
			agent.AccumulatedOutput += chunk.Content
			agent.OutputMu.Unlock()
		}

		// 累积结构化内容块（用于持久化）
		agent.ContentBlocksMu.Lock()
		now := time.Now().UnixMilli()
		switch chunk.Type {
		case ChunkTypeThinking:
			// 思考块：智能累积或追加
			if len(agent.AccumulatedContentBlocks) > 0 {
				lastBlock := &agent.AccumulatedContentBlocks[len(agent.AccumulatedContentBlocks)-1]
				if lastBlock.Type == "thinking" && lastBlock.Status == "streaming" {
					// 追加到最后一个思考块
					lastBlock.Content += chunk.Content
					if chunk.Done {
						lastBlock.Status = "success"
						lastBlock.Done = true
					}
					agent.ContentBlocksMu.Unlock()
					es.mu.Unlock()
					goto broadcast
				}
			}
			// 只有在有内容或不是 Done 标记时才创建新块
			if chunk.Content != "" || !chunk.Done {
				status := "streaming"
				if chunk.Done {
					status = "success"
				}
				agent.AccumulatedContentBlocks = append(agent.AccumulatedContentBlocks, ContentBlockData{
					ID:        fmt.Sprintf("thinking-%d-%d", invocationID.ID(), now),
					Type:      "thinking",
					Content:   chunk.Content,
					Timestamp: now,
					Status:    status,
					Done:      chunk.Done,
				})
			}
		case ChunkTypeText:
			// 文本块：智能累积
			if len(agent.AccumulatedContentBlocks) > 0 {
				lastBlock := &agent.AccumulatedContentBlocks[len(agent.AccumulatedContentBlocks)-1]
				if lastBlock.Type == "text" {
					lastBlock.Content += chunk.Content
					agent.ContentBlocksMu.Unlock()
					es.mu.Unlock()
					goto broadcast
				}
			}
			// 创建新的文本块
			agent.AccumulatedContentBlocks = append(agent.AccumulatedContentBlocks, ContentBlockData{
				ID:        fmt.Sprintf("text-%d-%d", invocationID.ID(), now),
				Type:      "text",
				Content:   chunk.Content,
				Timestamp: now,
			})
		case ChunkTypeToolUse:
			// 工具调用开始
			agent.AccumulatedContentBlocks = append(agent.AccumulatedContentBlocks, ContentBlockData{
				ID:        fmt.Sprintf("tool-%s", chunk.ToolID),
				Type:      "tool_use",
				Timestamp: now,
				Status:    "streaming",
				ToolName:  chunk.ToolName,
				ToolID:    chunk.ToolID,
				Input:     chunk.ToolInput,
				StartedAt: now,
			})
		case ChunkTypeToolResult:
			// 工具调用结果：更新对应的工具块
			for i := len(agent.AccumulatedContentBlocks) - 1; i >= 0; i-- {
				if agent.AccumulatedContentBlocks[i].Type == "tool_use" && agent.AccumulatedContentBlocks[i].ToolID == chunk.ToolID {
					agent.AccumulatedContentBlocks[i].Output = chunk.Content
					agent.AccumulatedContentBlocks[i].IsError = chunk.IsError
					agent.AccumulatedContentBlocks[i].Status = "success"
					agent.AccumulatedContentBlocks[i].CompletedAt = now
					break
				}
			}
		case ChunkTypeQuestion:
			// AskUserQuestion 工具调用：需要用户输入
			// 添加到内容块，等待用户响应
			agent.AccumulatedContentBlocks = append(agent.AccumulatedContentBlocks, ContentBlockData{
				ID:           fmt.Sprintf("question-%s", chunk.ToolID),
				Type:         "question",
				Timestamp:    now,
				Status:       "waiting_user_input",
				ToolName:     chunk.ToolName,
				ToolID:       chunk.ToolID,
				Input:        chunk.ToolInput,
				Questions:    chunk.Questions,
				InvocationID: invocationID.String(),
				StartedAt:    now,
			})
			// 标记为等待用户输入状态
			agent.WaitingForUserInput = true
			agent.PendingQuestionID = chunk.ToolID
		}
		agent.ContentBlocksMu.Unlock()

		// 增量持久化：将内容块写入数据库（后台执行支持）
		if es.contentBlockRepo != nil && len(agent.AccumulatedContentBlocks) > 0 {
			lastBlock := agent.AccumulatedContentBlocks[len(agent.AccumulatedContentBlocks)-1]
			// 转换为持久化模型
			persistBlock := model.InvocationContentBlock{
				ID:           lastBlock.ID,
				InvocationID: invocationID.String(),
				Type:         lastBlock.Type,
				Content:      lastBlock.Content,
				Timestamp:    lastBlock.Timestamp,
				Status:       lastBlock.Status,
				ToolName:     lastBlock.ToolName,
				ToolID:       lastBlock.ToolID,
				Input:        lastBlock.Input,
				Output:       lastBlock.Output,
				IsError:      lastBlock.IsError,
				StartedAt:    lastBlock.StartedAt,
				CompletedAt:  lastBlock.CompletedAt,
			}
			es.addToContentBlockBuffer(persistBlock, invocationID)
		}

		// 工具调用开始：增加计数并启动心跳
		if chunk.Type == ChunkTypeToolUse {
			agent.HeartbeatMu.Lock()
			agent.ActiveToolCount++
			logInfo("工具调用开始", zap.String("toolName", chunk.ToolName), zap.Int("activeToolCount", agent.ActiveToolCount))

			// 如果是第一个工具调用，启动心跳 goroutine
			if agent.ActiveToolCount == 1 && agent.HeartbeatCancel == nil {
				heartbeatCtx, heartbeatCancel := context.WithCancel(context.Background())
				agent.HeartbeatCancel = heartbeatCancel
				go es.toolHeartbeat(heartbeatCtx, invocationID, chunk.ToolName)
			}
			agent.HeartbeatMu.Unlock()
		}

		// 工具调用结果：减少计数并可能停止心跳
		if chunk.Type == ChunkTypeToolResult {
			agent.HeartbeatMu.Lock()
			if agent.ActiveToolCount > 0 {
				agent.ActiveToolCount--
				logInfo("工具调用完成", zap.String("toolName", chunk.ToolName), zap.Int("activeToolCount", agent.ActiveToolCount))
			}
			// 如果没有活跃的工具调用，停止心跳
			if agent.ActiveToolCount == 0 && agent.HeartbeatCancel != nil {
				agent.HeartbeatCancel()
				agent.HeartbeatCancel = nil
				logInfo("所有工具调用完成，停止心跳")
			}
			agent.HeartbeatMu.Unlock()
		}
	}
	es.mu.Unlock()

broadcast:

	if es.wsHub != nil {
		// 处理 Usage 类型的 Chunk
		if chunk.Type == ChunkTypeUsage && chunk.Usage != nil {
			es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
				Type:      "usage_update",
				ThreadID:  threadID.String(),
				Timestamp: time.Now().UnixMilli(),
				Payload: map[string]interface{}{
					"invocationId": invocationID.String(),
					"usage": map[string]interface{}{
						"inputTokens":         chunk.Usage.InputTokens,
						"outputTokens":        chunk.Usage.OutputTokens,
						"cacheReadTokens":     chunk.Usage.CacheReadTokens,
						"cacheCreationTokens": chunk.Usage.CacheCreationTokens,
						"costUsd":             chunk.Usage.CostUsd,
						"durationMs":          chunk.Usage.DurationMs,
						"durationApiMs":       chunk.Usage.DurationApiMs,
						"numTurns":            chunk.Usage.NumTurns,
					},
				},
			})
			// 通知外部 chunk 监听器（包括 usage chunks）
			es.NotifyChunkListeners(threadID, invocationID, chunk, agentID, agentName)
			return
		}

		payload := map[string]interface{}{
			"invocationId": invocationID.String(),
			"chunk":        chunk.Content,
			"chunkType":    string(chunk.Type),
			"agentId":      agentID,
			"agentName":    agentName,
		}

		// thinking 完成：发送 Done 标记
		if chunk.Type == ChunkTypeThinking && chunk.Done {
			payload["done"] = true
		}

		// 添加工具相关信息
		if chunk.Type == ChunkTypeToolUse {
			payload["toolName"] = chunk.ToolName
			payload["toolId"] = chunk.ToolID
			if chunk.ToolInput != nil {
				payload["toolInput"] = chunk.ToolInput
			}
		}

		// 工具结果：包含工具执行输出
		if chunk.Type == ChunkTypeToolResult {
			payload["toolId"] = chunk.ToolID
			payload["toolOutput"] = chunk.Content
			payload["isError"] = chunk.IsError
		}

		// AskUserQuestion 工具调用：包含问题列表
		if chunk.Type == ChunkTypeQuestion {
			payload["toolName"] = chunk.ToolName
			payload["toolId"] = chunk.ToolID
			payload["questions"] = chunk.Questions
			if chunk.ToolInput != nil {
				payload["toolInput"] = chunk.ToolInput
			}
		}

		es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_output_chunk",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload:   payload,
		})

		// 通知外部 chunk 监听器
		es.NotifyChunkListeners(threadID, invocationID, chunk, agentID, agentName)
	} else {
		logInfo("broadcastChunk: wsHub is nil!")
	}
}

// toolHeartbeat 工具执行心跳，定期更新 LastActiveAt 防止误判超时
func (es *ExecutionService) toolHeartbeat(ctx context.Context, invocationID uuid.UUID, initialToolName string) {
	ticker := time.NewTicker(toolHeartbeatInterval)
	defer ticker.Stop()

	logInfo("工具心跳启动", zap.String("invocationID", invocationID.String()), zap.String("initialToolName", initialToolName))

	for {
		select {
		case <-ctx.Done():
			logInfo("工具心跳停止", zap.String("invocationID", invocationID.String()))
			return
		case <-ticker.C:
			es.mu.Lock()
			if agent, exists := es.runningAgents[invocationID]; exists {
				agent.LastActiveAt = time.Now()
				agent.HeartbeatMu.Lock()
				count := agent.ActiveToolCount
				agent.HeartbeatMu.Unlock()
				logInfo("工具心跳更新", zap.String("invocationID", invocationID.String()), zap.Int("activeToolCount", count))
			} else {
				es.mu.Unlock()
				return // Agent 已不存在，停止心跳
			}
			es.mu.Unlock()
		}
	}
}

// SubmitQuestionAnswer 提交 AskUserQuestion 的用户答案
// 找到运行中的 Agent，并通过 stdin 发送答案给 CLI
func (es *ExecutionService) SubmitQuestionAnswer(threadID uuid.UUID, toolCallID string, answer string) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	// 查找该 thread 中等待用户输入的 Agent
	for invocationID, agent := range es.runningAgents {
		if agent.ThreadID == threadID && agent.WaitingForUserInput && agent.PendingQuestionID == toolCallID {
			logInfo("收到 AskUserQuestion 答案",
				zap.String("threadID", threadID.String()),
				zap.String("invocationID", invocationID.String()),
				zap.String("toolCallID", toolCallID),
				zap.String("answer", answer))

			// 清除等待状态
			agent.WaitingForUserInput = false
			agent.PendingQuestionID = ""

			// 更新内容块状态
			agent.ContentBlocksMu.Lock()
			for i := len(agent.AccumulatedContentBlocks) - 1; i >= 0; i-- {
				if agent.AccumulatedContentBlocks[i].Type == "question" && agent.AccumulatedContentBlocks[i].ToolID == toolCallID {
					agent.AccumulatedContentBlocks[i].Status = "success"
					agent.AccumulatedContentBlocks[i].Output = answer
					agent.AccumulatedContentBlocks[i].CompletedAt = time.Now().UnixMilli()
					break
				}
			}
			agent.ContentBlocksMu.Unlock()

			// 通过 Adapter 发送答案给 CLI
			if agent.Adapter != nil {
				// 尝试通过 ClaudeAdapter 发送响应
				if claudeAdapter, ok := agent.Adapter.(*ClaudeAdapter); ok {
					err := claudeAdapter.SendToolResult(invocationID, toolCallID, answer)
					if err != nil {
						logError("发送工具结果失败", zap.Error(err))
						return err
					}
				} else if acpAdapter, ok := agent.Adapter.(*BaseACPAdapter); ok {
					// 尝试通过 ACP adapter 发送响应
					err := acpAdapter.SendToolResult(invocationID, toolCallID, answer)
					if err != nil {
						logError("发送工具结果失败", zap.Error(err))
						return err
					}
				}
			}

			// 广播更新
			if es.wsHub != nil {
				es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
					Type:      "question_answered",
					ThreadID:  threadID.String(),
					Timestamp: time.Now().UnixMilli(),
					Payload: map[string]interface{}{
						"invocationId": invocationID.String(),
						"toolId":       toolCallID,
						"answer":       answer,
					},
				})
			}

			return nil
		}
	}

	return fmt.Errorf("未找到等待输入的 Agent 或 toolCallID 不匹配")
}

// GetInvocationsByThread 获取 Thread 的所有 Agent 调用
func (es *ExecutionService) GetInvocationsByThread(ctx context.Context, threadID uuid.UUID) ([]model.AgentInvocation, error) {
	invocations, err := es.invocationRepo.FindByThreadID(ctx, threadID)
	if err != nil {
		return nil, err
	}

	// 转换为 slice 返回
	result := make([]model.AgentInvocation, 0, len(invocations))
	for _, inv := range invocations {
		result = append(result, *inv)
	}
	return result, nil
}

// RunningAgentState 运行中 Agent 的状态（用于 WebSocket 恢复）
type RunningAgentState struct {
	InvocationID      string `json:"invocationId"`
	AgentID           string `json:"agentId"`
	AgentName         string `json:"agentName"`
	AccumulatedOutput string `json:"accumulatedOutput"`
	Status            string `json:"status"`
}

// GetRunningAgentsForThread 获取 Thread 中运行中的 Agent 状态（用于 WebSocket 重连恢复）
func (es *ExecutionService) GetRunningAgentsForThread(threadID uuid.UUID) []RunningAgentState {
	es.mu.RLock()
	defer es.mu.RUnlock()

	var states []RunningAgentState
	for _, agent := range es.runningAgents {
		if agent.ThreadID == threadID {
			agent.OutputMu.Lock()
			output := agent.AccumulatedOutput
			agent.OutputMu.Unlock()

			states = append(states, RunningAgentState{
				InvocationID:      agent.InvocationID.String(),
				AgentID:           agent.AgentConfig.ID.String(),
				AgentName:         agent.AgentConfig.Name,
				AccumulatedOutput: output,
				Status:            "running",
			})
		}
	}
	return states
}

// GetRunningInvocationsWithContentBlocks 获取运行中的 invocation 及其内容块（后台执行支持）
func (es *ExecutionService) GetRunningInvocationsWithContentBlocks(ctx context.Context, threadID uuid.UUID) []ws.InvocationRecoveryData {
	es.mu.RLock()
	defer es.mu.RUnlock()

	var result []ws.InvocationRecoveryData

	for _, agent := range es.runningAgents {
		if agent.ThreadID == threadID {
			// 获取累积的内容块
			agent.ContentBlocksMu.Lock()
			contentBlocks := make([]ContentBlockData, len(agent.AccumulatedContentBlocks))
			copy(contentBlocks, agent.AccumulatedContentBlocks)
			agent.ContentBlocksMu.Unlock()

			// 如果没有内存中的内容块，尝试从数据库恢复
			if len(contentBlocks) == 0 && es.contentBlockRepo != nil {
				blocks, err := es.contentBlockRepo.FindByInvocation(ctx, agent.InvocationID)
				if err == nil && len(blocks) > 0 {
					// 转换为 ContentBlockData
					for _, b := range blocks {
						contentBlocks = append(contentBlocks, ContentBlockData{
							ID:          b.ID,
							Type:        b.Type,
							Content:     b.Content,
							Timestamp:   b.Timestamp,
							Status:      b.Status,
							ToolName:    b.ToolName,
							ToolID:      b.ToolID,
							Input:       b.Input,
							Output:      b.Output,
							IsError:     b.IsError,
							StartedAt:   b.StartedAt,
							CompletedAt: b.CompletedAt,
						})
					}
				}
			}

			result = append(result, ws.InvocationRecoveryData{
				InvocationID:  agent.InvocationID.String(),
				AgentID:       agent.AgentConfig.ID.String(),
				AgentName:     agent.AgentConfig.Name,
				Status:        "running",
				ContentBlocks: contentBlocks,
			})
		}
	}

	return result
}

// GetRecentlyCompletedInvocations 获取最近完成的 invocation（用于 WebSocket 重连状态同步）
func (es *ExecutionService) GetRecentlyCompletedInvocations(ctx context.Context, threadID uuid.UUID, sinceMinutes int) []ws.InvocationRecoveryData {
	if es.invocationRepo == nil {
		logInfo("GetRecentlyCompletedInvocations: invocationRepo is nil")
		return nil
	}

	// 查询最近完成的 invocation
	invocations, err := es.invocationRepo.FindRecentlyCompletedByThread(ctx, threadID, sinceMinutes)
	if err != nil {
		logError("GetRecentlyCompletedInvocations: failed to query", zap.Error(err))
		return nil
	}

	logInfo("GetRecentlyCompletedInvocations: found invocations",
		zap.String("threadID", threadID.String()),
		zap.Int("count", len(invocations)))

	var result []ws.InvocationRecoveryData
	for _, inv := range invocations {
		result = append(result, ws.InvocationRecoveryData{
			InvocationID:  inv.ID.String(),
			AgentID:       inv.AgentConfigID.String(),
			AgentName:     string(inv.Role),
			Status:        string(inv.Status),
			ContentBlocks: nil, // 已完成的 invocation 不需要内容块
		})
	}

	return result
}

// GetInvocationStatus 获取单个调用的状态
func (es *ExecutionService) GetInvocationStatus(ctx context.Context, invocationID uuid.UUID) (*model.AgentInvocation, error) {
	return es.invocationRepo.FindByID(ctx, invocationID)
}

// SpawnAgentForUserMessage 为用户消息触发Agent响应
// 实现message.AgentSpawner接口
// 使用工作流模板中指定的Agent，而不是根据Phase硬编码选择
// 如果用户@的是同一个Agent，使用 resume 方式触发对话
func (es *ExecutionService) SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string) error {
	logInfo("SpawnAgentForUserMessage 被调用", zap.String("threadID", threadID.String()), zap.String("userMessage", userMessage))

	// 解析用户消息中的 @mentions
	var mentionedAgentIDs []string
	if es.mentionParser != nil {
		mentionedIDs, err := es.mentionParser.Parse(ctx, userMessage, "")
		if err != nil {
			logError("SpawnAgentForUserMessage: 解析 @mentions 失败", zap.Error(err))
		} else {
			mentionedAgentIDs = mentionedIDs
			logInfo("SpawnAgentForUserMessage: 解析到 @mentions", zap.Strings("agentIDs", mentionedAgentIDs))
		}
	}

	// 检查是否已有Agent在运行
	es.mu.RLock()
	runningCount := len(es.runningAgents)
	var runningInThread bool
	for _, agent := range es.runningAgents {
		if agent.ThreadID == threadID {
			runningInThread = true
			break
		}
	}
	es.mu.RUnlock()

	logInfo("SpawnAgentForUserMessage: runningAgents 状态",
		zap.Int("totalRunning", runningCount),
		zap.Bool("runningInThread", runningInThread))

	if runningInThread {
		// 检查运行中的 Agent 是否已超时
		// 超时判断考虑工具执行状态：有活跃工具调用时延长超时
		es.mu.Lock()
		now := time.Now()
		var timedOutAgent *RunningAgent
		for id, agent := range es.runningAgents {
			// 获取工具执行状态
			agent.HeartbeatMu.Lock()
			hasActiveTool := agent.ActiveToolCount > 0
			agent.HeartbeatMu.Unlock()

			// 超时判断：有活跃工具时使用更长超时（20分钟），否则使用默认超时（10分钟）
			timeout := agentExecutionTimeout
			if hasActiveTool {
				timeout = 2 * agentExecutionTimeout
			}

			inactiveDuration := now.Sub(agent.LastActiveAt)
			if agent.ThreadID == threadID && inactiveDuration > timeout {
				timedOutAgent = agent
				// 停止心跳
				agent.HeartbeatMu.Lock()
				if agent.HeartbeatCancel != nil {
					agent.HeartbeatCancel()
				}
				agent.HeartbeatMu.Unlock()
				delete(es.runningAgents, id)
				logInfo("SpawnAgentForUserMessage: 检测到无活动 Agent，自动清理",
					zap.String("invocationID", id.String()),
					zap.Duration("inactiveTime", inactiveDuration),
					zap.Duration("totalRunningTime", now.Sub(agent.StartedAt)),
					zap.Bool("hadActiveTools", hasActiveTool))

				// 异步标记为失败
				go es.markInvocationFailed(id, "agent inactive for timeout, no output activity")
				break
			}
		}
		es.mu.Unlock()

		// 如果没有超时的 Agent，才跳过
		if timedOutAgent == nil {
			logInfo("SpawnAgentForUserMessage: 已有 Agent 运行中且有活动输出，跳过")
			return nil
		}
		// 有超时 Agent 已清理，继续执行
		logInfo("SpawnAgentForUserMessage: 无活动 Agent 已清理，继续执行")
	}

	// 获取Thread信息
	thread, err := es.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to get thread: %w", err)
	}

	// 获取项目路径和工作流模板
	var projectPath string
	var workflowTemplateID *uuid.UUID
	if es.projectRepo != nil {
		project, err := es.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil {
			projectPath = project.LocalPath
			// 优先使用 Project 的工作流模板
			if project.WorkflowTemplateID != nil {
				workflowTemplateID = project.WorkflowTemplateID
			}
		}
	}
	// 如果 Project 没有工作流模板，使用 Thread 的
	if workflowTemplateID == nil && thread.WorkflowTemplateID != nil {
		workflowTemplateID = thread.WorkflowTemplateID
	}

	logInfo("SpawnAgentForUserMessage: 工作流模板查找结果",
		zap.Any("workflowTemplateID", workflowTemplateID),
		zap.Any("threadWorkflowTemplateID", thread.WorkflowTemplateID))

	// 获取工作流模板中的Agent列表和Transitions
	var agentIDs []string
	var transitions []model.Transition
	if workflowTemplateID != nil && es.workflowRepo != nil {
		workflow, err := es.workflowRepo.FindByID(ctx, *workflowTemplateID)
		if err != nil {
			logError("SpawnAgentForUserMessage: 查找工作流模板失败", zap.Error(err))
		} else if workflow == nil {
			logInfo("SpawnAgentForUserMessage: 工作流模板为 nil")
		} else {
			logInfo("SpawnAgentForUserMessage: 找到工作流模板",
				zap.String("name", workflow.Name),
				zap.Int("agentIDsLen", len(workflow.AgentIDs)))
			// 解析 agent_ids JSON
			if len(workflow.AgentIDs) > 0 {
				if err := json.Unmarshal(workflow.AgentIDs, &agentIDs); err != nil {
					logError("Failed to parse agent_ids", zap.Error(err))
				} else {
					logInfo("SpawnAgentForUserMessage: 解析的 Agent IDs", zap.Strings("agentIDs", agentIDs))
				}
			}
			// 解析 transitions JSON
			if len(workflow.Transitions) > 0 {
				if err := json.Unmarshal(workflow.Transitions, &transitions); err != nil {
					logError("Failed to parse transitions", zap.Error(err))
				} else {
					logInfo("SpawnAgentForUserMessage: 解析的 Transitions", zap.Int("count", len(transitions)))
				}
			}
		}
	} else if workflowTemplateID == nil {
		logInfo("SpawnAgentForUserMessage: workflowTemplateID 为 nil，无法获取 Agent 列表")
	}

	// 根据Transitions找到入口Agent（没有被其他Agent指向的Agent）
	entryAgentID := ""
	if len(agentIDs) > 0 {
		if len(transitions) > 0 {
			// 收集所有 to_agent_id（被指向的Agent）
			targetAgents := make(map[string]bool)
			for _, t := range transitions {
				targetAgents[t.ToAgentID] = true
			}

			// 找到不在 targetAgents 中的 Agent（入口 Agent）
			for _, id := range agentIDs {
				if !targetAgents[id] {
					entryAgentID = id
					logInfo("SpawnAgentForUserMessage: 找到入口 Agent", zap.String("entryAgentID", entryAgentID))
					break
				}
			}

			// 如果没找到入口 Agent，使用 agent_ids[0]
			if entryAgentID == "" {
				logInfo("SpawnAgentForUserMessage: 未找到入口 Agent，使用 agent_ids[0]")
				entryAgentID = agentIDs[0]
			}
		} else {
			// 没有 Transitions，使用 agent_ids[0]
			entryAgentID = agentIDs[0]
		}

		configID, err := uuid.Parse(entryAgentID)
		if err != nil {
			return fmt.Errorf("invalid agent id in workflow template: %w", err)
		}

		// 验证Agent配置存在
		config, err := es.configSvc.GetByID(ctx, configID)
		if err != nil {
			logError("Agent config not found, falling back to default", zap.Error(err))
			// 继续使用回退逻辑
		} else {
			logInfo("SpawnAgentForUserMessage: 使用入口 Agent", zap.String("name", config.Name), zap.String("id", config.ID.String()))

			// 判断会话策略：如果用户@的是同一个Agent，使用 resume
			var sessionStrategy SessionStrategy
			var sessionIdFromDB string
			if len(mentionedAgentIDs) > 0 {
				sessionStrategy, sessionIdFromDB = es.shouldUseResumeStrategy(ctx, threadID, config.ID, mentionedAgentIDs)
				if sessionStrategy == SessionStrategyResume {
					logInfo("SpawnAgentForUserMessage: 用户@同一Agent，使用 resume 会话策略",
						zap.String("agentName", config.Name),
						zap.String("agentID", config.ID.String()))
				}
			} else {
				// 没有 @mention，检查是否应该自动 resume
				sessionStrategy, sessionIdFromDB = es.shouldAutoResume(ctx, threadID, config.ID)
				if sessionStrategy == SessionStrategyResume {
					logInfo("SpawnAgentForUserMessage: 自动判断使用 resume 会话策略（无@mention）",
						zap.String("agentName", config.Name),
						zap.String("agentID", config.ID.String()))
				}
			}

			// 使用工作流模板中指定的Agent
			_, err = es.SpawnAgent(ctx, &SpawnRequest{
				ThreadID:        threadID,
				Role:            config.Role,
				ConfigID:        config.ID,
				Input:           userMessage,
				ProjectPath:     projectPath,
				SessionStrategy: sessionStrategy,
				SessionID:       sessionIdFromDB,
			})
			return err
		}
	}

	// 回退逻辑：获取任意一个可用的默认 Agent
	logDebug("No workflow agent found, using fallback selection")
	configs, listErr := es.configSvc.List(ctx)
	if listErr != nil || len(configs) == 0 {
		return fmt.Errorf("no agent config available: %w", listErr)
	}
	// 优先选择 is_default=true 的，否则选第一个
	config := configs[0]
	for _, c := range configs {
		if c.IsDefault {
			config = c
			break
		}
	}

	// 判断会话策略：如果用户@的是同一个Agent，使用 resume
	var sessionStrategy SessionStrategy
	var sessionIdFromDB string
	if len(mentionedAgentIDs) > 0 {
		sessionStrategy, sessionIdFromDB = es.shouldUseResumeStrategy(ctx, threadID, config.ID, mentionedAgentIDs)
		if sessionStrategy == SessionStrategyResume {
			logInfo("SpawnAgentForUserMessage: 用户@同一Agent（回退），使用 resume 会话策略",
				zap.String("agentName", config.Name),
				zap.String("agentID", config.ID.String()))
		}
	} else {
		// 没有 @mention，检查是否应该自动 resume
		sessionStrategy, sessionIdFromDB = es.shouldAutoResume(ctx, threadID, config.ID)
		if sessionStrategy == SessionStrategyResume {
			logInfo("SpawnAgentForUserMessage: 自动判断使用 resume 会话策略（回退，无@mention）",
				zap.String("agentName", config.Name),
				zap.String("agentID", config.ID.String()))
		}
	}

	// 触发Agent
	_, err = es.SpawnAgent(ctx, &SpawnRequest{
		ThreadID:        threadID,
		Role:            config.Role,
		ConfigID:        config.ID,
		Input:           userMessage,
		ProjectPath:     projectPath,
		SessionStrategy: sessionStrategy,
		SessionID:       sessionIdFromDB,
	})
	return err
}

// timePtr 返回时间的指针
func timePtr(t time.Time) *time.Time {
	return &t
}

// shouldUseResumeStrategy 判断是否应该使用 resume 会话策略
// 当用户@的是同一个Agent（与最后一个完成的Agent相同）时使用 resume
func (es *ExecutionService) shouldUseResumeStrategy(ctx context.Context, threadID uuid.UUID, targetConfigID uuid.UUID, mentionedAgentIDs []string) (SessionStrategy, string) {
	// 如果没有 @mentions，不使用 resume
	if len(mentionedAgentIDs) == 0 {
		return "", ""
	}

	// 检查目标 Agent 是否在被 @ 的列表中
	targetIDStr := targetConfigID.String()
	isMentioned := false
	for _, mentionedID := range mentionedAgentIDs {
		if mentionedID == targetIDStr {
			isMentioned = true
			break
		}
	}
	if !isMentioned {
		return "", ""
	}

	// 获取该线程最后一个完成的 invocation
	lastCompleted, err := es.invocationRepo.FindRecentlyCompletedByThread(ctx, threadID, 5) // 最近5分钟
	if err != nil || len(lastCompleted) == 0 {
		logInfo("shouldUseResumeStrategy: 没有找到最近完成的 invocation", zap.Error(err))
		return "", ""
	}

	// 检查最后一个完成的 Agent 是否与目标 Agent 相同
	lastInvocation := lastCompleted[0] // 第一个是最近的
	if lastInvocation.AgentConfigID == targetConfigID {
		logInfo("shouldUseResumeStrategy: 目标 Agent 与最后一个完成的 Agent 相同，使用 resume",
			zap.String("targetConfigID", targetConfigID.String()),
			zap.String("lastCompletedConfigID", lastInvocation.AgentConfigID.String()),
			zap.String("lastCompletedRole", string(lastInvocation.Role)))
		return SessionStrategyResume, lastInvocation.SessionID
	}

	// 检查是否同角色（不同实例）
	config, err := es.configSvc.GetByID(ctx, targetConfigID)
	if err != nil || config == nil {
		return "", ""
	}
	if string(config.Role) == string(lastInvocation.Role) {
		logInfo("shouldUseResumeStrategy: 目标 Agent 与最后一个完成的 Agent 同角色，使用 resume",
			zap.String("targetRole", string(config.Role)),
			zap.String("lastCompletedRole", string(lastInvocation.Role)))
		return SessionStrategyResume, lastInvocation.SessionID
	}

	return "", ""
}

// shouldAutoResume 判断是否应该自动使用 resume 会话策略
// 当目标 Agent 与最后一个完成的 Agent 相同时自动使用 resume（无需显式 @mention）
func (es *ExecutionService) shouldAutoResume(ctx context.Context, threadID uuid.UUID, targetConfigID uuid.UUID) (SessionStrategy, string) {
	// 获取该线程最后一个完成的 invocation
	lastCompleted, err := es.invocationRepo.FindRecentlyCompletedByThread(ctx, threadID, 5) // 最近5分钟
	if err != nil || len(lastCompleted) == 0 {
		logInfo("shouldAutoResume: 没有找到最近完成的 invocation", zap.Error(err))
		return "", ""
	}

	// 检查最后一个完成的 Agent 是否与目标 Agent 相同
	lastInvocation := lastCompleted[0] // 第一个是最近的
	if lastInvocation.AgentConfigID == targetConfigID {
		logInfo("shouldAutoResume: 目标 Agent 与最后一个完成的 Agent 相同，自动使用 resume",
			zap.String("targetConfigID", targetConfigID.String()),
			zap.String("lastCompletedConfigID", lastInvocation.AgentConfigID.String()),
			zap.String("lastCompletedRole", string(lastInvocation.Role)))
		return SessionStrategyResume, lastInvocation.SessionID
	}

	// 检查是否同角色（不同实例）
	config, err := es.configSvc.GetByID(ctx, targetConfigID)
	if err != nil || config == nil {
		return "", ""
	}
	if string(config.Role) == string(lastInvocation.Role) {
		logInfo("shouldAutoResume: 目标 Agent 与最后一个完成的 Agent 同角色，自动使用 resume",
			zap.String("targetRole", string(config.Role)),
			zap.String("lastCompletedRole", string(lastInvocation.Role)))
		return SessionStrategyResume, lastInvocation.SessionID
	}

	return "", ""
}

// matchCondition 匹配条件表达式
// 支持简单的关键词匹配和正则表达式
// 条件格式：
//   - "contains:关键词" - 输出包含指定关键词
//   - "regex:正则表达式" - 正则表达式匹配
//   - "关键词" - 默认使用 contains 匹配
func (es *ExecutionService) matchCondition(output, condition string) bool {
	if condition == "" {
		return true
	}

	// 解析条件类型
	if strings.HasPrefix(condition, "contains:") {
		keyword := strings.TrimPrefix(condition, "contains:")
		return strings.Contains(output, keyword)
	}

	if strings.HasPrefix(condition, "regex:") {
		pattern := strings.TrimPrefix(condition, "regex:")
		matched, err := regexp.MatchString(pattern, output)
		if err != nil {
			logError("Invalid regex condition", zap.Error(err), zap.String("pattern", pattern))
			return false
		}
		return matched
	}

	// 默认使用 contains 匹配
	return strings.Contains(output, condition)
}

// checkMergeCondition 检查汇聚条件是否满足
// 所有指定的上游 Agent 都完成后才返回 true
func (es *ExecutionService) checkMergeCondition(threadID uuid.UUID, waitFor []string) bool {
	es.a2aMu.RLock()
	defer es.a2aMu.RUnlock()

	a2aCtx, exists := es.a2aContexts[threadID]
	if !exists {
		return false
	}

	for _, agentIDStr := range waitFor {
		agentID, err := uuid.Parse(agentIDStr)
		if err != nil {
			logError("Invalid agent ID in wait_for", zap.Error(err), zap.String("agentId", agentIDStr))
			continue
		}

		if !a2aCtx.CompletedAgents[agentID] {
			return false
		}
	}

	return true
}

// flushContentBlocks 刷新内容块缓冲区到数据库（增量持久化）
// 节流策略：每 10 个块或每 500ms 刷新一次
func (es *ExecutionService) flushContentBlocks(invocationID uuid.UUID) {
	es.contentBlockFlushMu.Lock()
	defer es.contentBlockFlushMu.Unlock()

	if len(es.contentBlockBuffer) == 0 {
		return
	}

	// 取出缓冲区内容
	blocks := es.contentBlockBuffer
	es.contentBlockBuffer = make([]model.InvocationContentBlock, 0, 20)
	es.lastFlush = time.Now()

	// 异步写入数据库（不阻塞主流程）
	go func() {
		if es.contentBlockRepo == nil {
			return
		}
		if err := es.contentBlockRepo.BatchUpsert(context.Background(), blocks); err != nil {
			logError("flushContentBlocks: failed to persist content blocks",
				zap.Error(err),
				zap.String("invocationID", invocationID.String()),
				zap.Int("blockCount", len(blocks)))
		} else {
			logInfo("flushContentBlocks: persisted content blocks",
				zap.String("invocationID", invocationID.String()),
				zap.Int("blockCount", len(blocks)))
		}
	}()
}

// addToContentBlockBuffer 添加内容块到缓冲区（带节流）
func (es *ExecutionService) addToContentBlockBuffer(block model.InvocationContentBlock, invocationID uuid.UUID) {
	es.contentBlockFlushMu.Lock()
	es.contentBlockBuffer = append(es.contentBlockBuffer, block)
	bufferLen := len(es.contentBlockBuffer)
	timeSinceFlush := time.Since(es.lastFlush)
	es.contentBlockFlushMu.Unlock()

	// 节流策略：每 10 个块或每 500ms 刷新一次
	if bufferLen >= 10 || timeSinceFlush >= 500*time.Millisecond {
		es.flushContentBlocks(invocationID)
	}
}

// formatFullPrompt 格式化完整提示词（用于调用日志显示）
// 格式与 ClaudeAdapter.buildPromptFromRequest 保持一致
func (es *ExecutionService) formatFullPrompt(layers *ContextLayers, input string) string {
	var sb strings.Builder

	if layers != nil {
		// Layer 0: 系统提示
		if layers.Layer0 != "" {
			sb.WriteString("<system>\n")
			sb.WriteString(layers.Layer0)
			sb.WriteString("\n</system>\n\n")
		}

		// Layer 1: Thread历史
		if layers.Layer1 != "" {
			sb.WriteString("<conversation>\n")
			sb.WriteString(layers.Layer1)
			sb.WriteString("\n</conversation>\n\n")
		}

		// Layer 2: 工作产物
		if layers.Layer2 != "" {
			sb.WriteString("<artifacts>\n")
			sb.WriteString(layers.Layer2)
			sb.WriteString("\n</artifacts>\n\n")
		}

		// Layer 3: 环境信息
		if layers.Layer3 != "" {
			sb.WriteString("<environment>\n")
			sb.WriteString(layers.Layer3)
			sb.WriteString("\n</environment>\n\n")
		}
	}

	// 用户输入部分：区分 A2A 输入和普通输入
	// A2A 输入包含 "## 会话策略" 或 "## 前序分析" 特征
	if strings.Contains(input, "## 会话策略") || strings.Contains(input, "## 前序分析") {
		sb.WriteString("<a2a_input>\n")
		sb.WriteString(input)
		sb.WriteString("\n</a2a_input>\n")
	} else {
		sb.WriteString("<user>\n")
		sb.WriteString(input)
		sb.WriteString("\n</user>\n")
	}

	return sb.String()
}

// broadcastFullPrompt 广播完整 prompt 更新（用于前端调用日志显示）
func (es *ExecutionService) broadcastFullPrompt(threadID, invocationID uuid.UUID, fullPrompt string) {
	if es.wsHub != nil {
		es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "invocation_full_prompt",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"invocationId": invocationID.String(),
				"fullPrompt":   fullPrompt,
			},
		})
	}
}

// filterStructuredOutput 从前序 Agent 输出中提取结构化关键信息
// 保留：文件路径引用、关键结论标记、工具调用结果
func (es *ExecutionService) filterStructuredOutput(output string, contentBlocks []ContentBlockData) string {
	var result []string

	// 1. 提取文件路径引用
	filePatterns := []string{
		`file://[^\s]+`,           // file://xxx
		`path:\s*[^\s]+`,          // path: xxx
		`\.\/[^\s]+`,              // ./xxx
	}
	for _, pattern := range filePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(output, -1)
		result = append(result, matches...)
	}

	// 2. 提取代码文件引用（排除干扰词）
	// 匹配 xxx.go, xxx.ts, xxx.py 等文件名
	codeFilePattern := `[a-zA-Z0-9_\-]+\.(go|ts|tsx|js|jsx|py|java|kt|rs|c|cpp|h|sql|yaml|yml|json|md)`
	re := regexp.MustCompile(codeFilePattern)
	matches := re.FindAllString(output, -1)
	// 过滤掉常见干扰词
	excludeWords := map[string]bool{
		"true.md": true, "false.md": true, "null.json": true,
	}
	for _, m := range matches {
		if !excludeWords[m] {
			result = append(result, m)
		}
	}

	// 3. 提取关键结论标记后的内容
	conclusionPatterns := []string{
		`结论[:：]\s*[^\n]+`,
		`结果[:：]\s*[^\n]+`,
		`关键点[:：]\s*[^\n]+`,
		`总结[:：]\s*[^\n]+`,
		`建议[:：]\s*[^\n]+`,
		`要点[:：]\s*[^\n]+`,
		`分析结果[:：]\s*[^\n]+`,
	}
	for _, pattern := range conclusionPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(output, -1)
		result = append(result, matches...)
	}

	// 4. 提取工具调用结果（如果有）
	for _, block := range contentBlocks {
		if block.Type == "tool_use" && block.Output != "" {
			// 截取工具输出前 200 字符（避免过长）
			outputStr := block.Output
			if len(outputStr) > 200 {
				outputStr = outputStr[:200] + "..."
			}
			result = append(result, fmt.Sprintf("[%s] %s", block.ToolName, outputStr))
		}
	}

	// 去重并返回
	seen := make(map[string]bool)
	var unique []string
	for _, item := range result {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] && len(item) > 3 { // 过滤过短内容
			seen[item] = true
			unique = append(unique, item)
		}
	}

	if len(unique) == 0 {
		return "(无关键结构化信息)"
	}

	return strings.Join(unique, "\n")
}

// extractStructuredHistory 从历史消息中提取结构化关键信息
// 用于优化 Layer1 内容，避免完整历史导致输入过长
func (es *ExecutionService) extractStructuredHistory(messages []*model.Message, maxMessages int) string {
	var sb strings.Builder
	sb.WriteString("## 会话历史摘要\n\n")

	// 限制处理的消息数量
	if len(messages) > maxMessages {
		messages = messages[:maxMessages]
	}

	// 1. 提取用户核心请求（第一条用户消息）
	for _, msg := range messages {
		if msg.Role == model.MessageRoleUser {
			sb.WriteString("**用户请求**: ")
			content := msg.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			sb.WriteString(content)
			sb.WriteString("\n\n")
			break
		}
	}

	// 2. 提取关键决策和结论
	sb.WriteString("**关键结论**:\n")
	conclusionPatterns := []string{
		`结论[:：]\s*[^\n]+`,
		`结果[:：]\s*[^\n]+`,
		`关键点[:：]\s*[^\n]+`,
		`总结[:：]\s*[^\n]+`,
		`建议[:：]\s*[^\n]+`,
		`要点[:：]\s*[^\n]+`,
		`决定[:：]\s*[^\n]+`,
		`完成[:：]\s*[^\n]+`,
		`分析[:：]\s*[^\n]+`,
	}

	conclusionsFound := false
	for _, msg := range messages {
		if msg.Role == model.MessageRoleAgent {
			for _, pattern := range conclusionPatterns {
				re := regexp.MustCompile(pattern)
				matches := re.FindAllString(msg.Content, -1)
				for _, m := range matches {
					sb.WriteString("- ")
					sb.WriteString(m)
					sb.WriteString("\n")
					conclusionsFound = true
				}
			}
		}
	}
	if !conclusionsFound {
		sb.WriteString("- (无明确结论)\n")
	}
	sb.WriteString("\n")

	// 3. 提取文件路径引用
	sb.WriteString("**涉及文件**:\n")
	filePatterns := []string{
		`file://[^\s]+`,
		`path:\s*[^\s]+`,
		`\./[^\s]+`,
		`[a-zA-Z0-9_\-]+\.(go|ts|tsx|js|jsx|py|java|kt|rs|c|cpp|h|sql|yaml|yml|json|md|html|css)`,
	}
	excludeWords := map[string]bool{
		"true.md":    true,
		"false.md":   true,
		"null.json":  true,
		"true.json":  true,
		"false.json": true,
	}

	filesFound := false
	seenFiles := make(map[string]bool)
	for _, msg := range messages {
		for _, pattern := range filePatterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindAllString(msg.Content, -1)
			for _, m := range matches {
				m = strings.TrimSpace(m)
				if !excludeWords[m] && !seenFiles[m] && len(m) > 5 {
					sb.WriteString("- ")
					sb.WriteString(m)
					sb.WriteString("\n")
					seenFiles[m] = true
					filesFound = true
				}
			}
		}
	}
	if !filesFound {
		sb.WriteString("- (无文件引用)\n")
	}
	sb.WriteString("\n")

	// 4. 提取工具调用摘要（从 ContentBlocks 解析）
	sb.WriteString("**工具调用摘要**:\n")
	toolsFound := false
	for _, msg := range messages {
		if msg.Role == model.MessageRoleAgent && len(msg.ContentBlocks) > 0 {
			var blocks []ContentBlockData
			if err := json.Unmarshal(msg.ContentBlocks, &blocks); err == nil {
				for _, block := range blocks {
					if block.Type == "tool_use" {
						sb.WriteString("- [")
						sb.WriteString(block.ToolName)
						sb.WriteString("] ")
						// 输入摘要（前 100 字符）
						if block.Input != nil {
							inputStr := fmt.Sprintf("%v", block.Input)
							if len(inputStr) > 100 {
								inputStr = inputStr[:100] + "..."
							}
							sb.WriteString(inputStr)
						}
						// 输出摘要（前 100 字符）
						if block.Output != "" && !block.IsError {
							outputStr := block.Output
							if len(outputStr) > 100 {
								outputStr = outputStr[:100] + "..."
							}
							sb.WriteString(" -> ")
							sb.WriteString(outputStr)
						}
						sb.WriteString("\n")
						toolsFound = true
					}
				}
			}
		}
	}
	if !toolsFound {
		sb.WriteString("- (无工具调用)\n")
	}
	sb.WriteString("\n")

	// 5. Agent 角色标识（最近的消息来源）
	sb.WriteString("**对话参与者**:\n")
	seenAgents := make(map[string]bool)
	recentCount := 0
	for i := len(messages) - 1; i >= 0 && recentCount < 5; i-- {
		msg := messages[i]
		if msg.Role == model.MessageRoleAgent && msg.AgentID != "" {
			if !seenAgents[msg.AgentID] {
				sb.WriteString("- ")
				sb.WriteString(msg.AgentID)
				sb.WriteString("\n")
				seenAgents[msg.AgentID] = true
				recentCount++
			}
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

