package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/humantask"
	"github.com/anthropic/isdp/internal/service/memory"
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
)

// AgentInfo 触发者信息（A2A 优化）
type AgentInfo struct {
	ID   uuid.UUID
	Name string
	Role string
}

// A2AContext A2A 上下文，用于追踪深度和去重
// 参考 clowder-ai route-serial previousResponses 累积机制
type A2AContext struct {
	Depth           int                // 当前深度
	InvokedAgents   map[uuid.UUID]bool // 已调用的 Agent ID 集合
	CompletedAgents map[uuid.UUID]bool // 已完成的 Agent ID 集合（用于汇聚判断）
	FromAgent       *AgentInfo         // 触发者信息（谁 @ 的下游 Agent）
	SessionStrategy SessionStrategy    // 会话策略：Resume 或 New

	// 链路追踪（参考 clowder-ai route-serial）
	PreviousResponses []ChainResponse // 前序响应累积（按时间顺序）
	OriginalMessage   string          // 原始用户消息（始终保留）
	ChainIndex        int             // 当前在链路中的位置
	ChainTotal        int             // 链路总长度（预计）

	// clowder-ai 对齐新增字段
	ChainHistory *A2AChainContext // 链路历史上下文（包含 TokenBudget、ActiveParticipants 等）
}

// ThreadContext 预加载的 Thread 上下文，避免重复数据库查询
type ThreadContext struct {
	Thread             *model.Thread
	Project            *model.Project
	WorkflowTemplate   *model.WorkflowTemplate
	WorkflowAgentIDs   []string
	Transitions        []model.Transition
	AllowedAgents      []*model.AgentRoleConfig
	RoutableTeamAgents []*model.AgentRoleConfig // T2T: 可路由团队的 Agent 列表
	LoadedAt           time.Time
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

	// Human 任务服务（用于 A2A 触发 Human 角色）
	humanTaskSvc     *humantask.Service
	humanTaskEnabled bool // 待办任务开关，控制自动创建和关闭

	// 记忆管理器（US-004 集成）
	memoryManager *memory.MemoryManager

	// API URL（用于 MCP server 回调）
	apiURL string

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

	// Token 预算管理器（用于 A2A 深度控制）
	tokenBudgetManager *TokenBudgetManager

	// Session 管理器（用于不同 CLI 类型的 session 策略）
	sessionManager *SessionManager

	// MCP Server 绑定仓库（显式 MCP 资产管理）
	mcpBindingRepo *repo.AgentMCPBindingRepository
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
	humanTaskSvc *humantask.Service,
	humanTaskEnabled bool,
	memoryManager *memory.MemoryManager,
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
		humanTaskSvc:       humanTaskSvc,
		humanTaskEnabled:   humanTaskEnabled,
		memoryManager:      memoryManager,
		contentBlockRepo:   contentBlockRepo,
		contentBlockBuffer: make([]model.InvocationContentBlock, 0, 20),
		lastFlush:          time.Now(),
		runningAgents:      make(map[uuid.UUID]*RunningAgent),
		a2aContexts:        make(map[uuid.UUID]*A2AContext),
		threadContexts:     make(map[uuid.UUID]*ThreadContext),
		cliSessions:        make(map[string]string),
		chunkListeners:     make([]ChunkListener, 0),
		tokenBudgetManager: NewTokenBudgetManager(),
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

// SetMemoryManager 设置记忆管理器（用于依赖注入）
// 在 cmd/server/main.go 中调用此方法完成注入
func (es *ExecutionService) SetMemoryManager(mm *memory.MemoryManager) {
	es.memoryManager = mm
}

// SetAPIURL 设置 API URL（用于 MCP server 回调）
func (es *ExecutionService) SetAPIURL(url string) {
	es.apiURL = url
}

// SetSessionManager 设置 Session 管理器（用于不同 CLI 类型的 session 策略）
func (es *ExecutionService) SetSessionManager(sm *SessionManager) {
	es.sessionManager = sm
}

// SetMCPBindingRepository 设置 MCP Server 绑定仓库。
func (es *ExecutionService) SetMCPBindingRepository(repo *repo.AgentMCPBindingRepository) {
	es.mcpBindingRepo = repo
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

// SyncMemoryTurn 对话后同步记忆（US-004 集成）
// 参考 hermes-agent MemoryManager.SyncTurn：goroutine + 30s timeout
func (es *ExecutionService) SyncMemoryTurn(ctx context.Context, threadID uuid.UUID, userContent, assistantContent string) {
	if es.memoryManager == nil {
		return
	}

	// 异步执行，带超时保护（已在 MemoryManager 内部实现）
	es.memoryManager.SyncTurn(ctx, userContent, assistantContent)
}

// OnThreadEndMemory 线程结束时清理记忆（US-004 集成）
func (es *ExecutionService) OnThreadEndMemory(ctx context.Context, threadID uuid.UUID) error {
	if es.memoryManager == nil {
		return nil
	}
	return es.memoryManager.OnThreadEnd(ctx, threadID.String())
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

	// 生成 callbackToken（用于 MCP server 回调）
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate callback token: %w", err)
	}
	callbackToken := hex.EncodeToString(tokenBytes)

	invocation := &model.AgentInvocation{
		ID:            uuid.New(),
		ThreadID:      req.ThreadID,
		AgentConfigID: config.ID,
		Role:          config.Role,
		AgentName:     config.Name, // 存储 Agent 名称，用于历史显示
		Status:        model.InvocationStatusPending,
		Input:         req.Input,
		CreatedAt:     now,
		StartedAt:     &now,          // 设置开始时间，用于历史显示耗时
		CallbackToken: callbackToken, // MCP 回调认证 Token
		TriggeredBy:   req.TriggeredBy,
	}

	if err := es.invocationRepo.Create(ctx, invocation); err != nil {
		return nil, fmt.Errorf("failed to create invocation: %w", err)
	}
	logInfo("[PERF] createInvocationRecord", zap.Duration("duration", time.Since(invocationCreateStart)))

	logInfo("[PERF] SpawnAgent total", zap.Duration("duration", time.Since(spawnStart)), zap.String("invocationID", invocation.ID.String()))

	// 创建上下文 - 使用独立的context，不受HTTP请求生命周期影响
	agentCtx, cancel := context.WithCancel(context.Background())

	// 记录运行中的Agent
	// 当使用 resume 策略时，继承之前 invocation 的已回答 question blocks
	var inheritedContentBlocks []ContentBlockData
	if req.SessionStrategy == SessionStrategyResume {
		es.mu.Lock()
		// 查找同 Thread 中之前的 RunningAgent（已回答问题但未完成）
		for _, prevAgent := range es.runningAgents {
			if prevAgent.ThreadID == req.ThreadID {
				prevAgent.ContentBlocksMu.Lock()
				// 只继承已回答的 question blocks（status='success'）
				for _, block := range prevAgent.AccumulatedContentBlocks {
					if block.Type == "question" && block.Status == "success" {
						inheritedContentBlocks = append(inheritedContentBlocks, block)
						logInfo("SpawnAgent: 继承已回答的 question block",
							zap.String("blockId", block.ID),
							zap.String("output", block.Output))
					}
				}
				prevAgent.ContentBlocksMu.Unlock()
				break // 只继承一个之前的 Agent
			}
		}
		es.mu.Unlock()
	}

	es.mu.Lock()
	es.runningAgents[invocation.ID] = &RunningAgent{
		InvocationID:             invocation.ID,
		ThreadID:                 req.ThreadID,
		AgentConfig:              config,
		BaseAgent:                baseAgent,
		StartedAt:                time.Now(),
		LastActiveAt:             time.Now(), // 初始化活动时间
		CancelFunc:               cancel,
		ActiveToolCount:          0,                      // 初始化工具计数
		AccumulatedContentBlocks: inheritedContentBlocks, // 继承已回答的 question blocks
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
	mcpServers := es.loadBoundMCPServers(ctx, config, baseAgent)

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
		OnSessionIDAcquired: func(sid string) {
			es.saveSessionIDEarly(ctx, req.ThreadID, config, baseAgent, invocation, sid)
		},
		BaseAgent:       baseAgent,
		Context:         contextLayers,
		Input:           req.Input,
		Images:          req.Images,
		WorkDir:         req.ProjectPath,
		ConfigDir:       config.ConfigPath,
		MCPServers:      mcpServers,
		SessionID:       sessionID,
		SessionStrategy: req.SessionStrategy,
		InvocationID:    invocation.ID,            // 用于 AskUserQuestion 答案发送
		CallbackToken:   invocation.CallbackToken, // 用于 MCP server 回调
		APIURL:          es.apiURL,                // 用于 MCP server 回调
	}
	logInfo("[PERF] buildExecutionRequest", zap.Duration("duration", time.Since(execReqBuildStart)), zap.String("sessionID", sessionID), zap.String("sessionStrategy", string(req.SessionStrategy)))

	// === ACP 原生 session/resume 模式 ===
	var acpSessionID string
	var newACPSessionID string
	if es.sessionManager != nil {
		threadUUID, _ := uuid.Parse(req.ThreadID.String())
		configUUID := config.ID
		sessionHandle, handleErr := es.sessionManager.GetOrCreateSession(ctx, threadUUID, configUUID, baseAgent)
		if handleErr != nil {
			logError("GetOrCreateSession failed", zap.Error(handleErr))
		} else if sessionHandle != nil {
			acpSessionID = sessionHandle.GetACPSessionID()
			logInfo("SessionManager: got session handle",
				zap.String("acpSessionId", acpSessionID),
				zap.String("strategy", string(sessionHandle.GetStrategy())))
		}
	}

	var outputBuilder strings.Builder
	var result *ExecutionResult

	cliStart := time.Now()
	logInfo("[PERF] CLI execution starting", zap.String("invocationID", invocation.ID.String()))

	// 检查 adapter 是否支持 ACP 原生 session/resume
	resumeCapable, ok := adapter.(SessionResumeCapable)
	if ok && acpSessionID != "" {
		// 使用 ACP 原生 session/resume 执行
		logInfo("Using ACP native session/resume",
			zap.String("invocationID", invocation.ID.String()),
			zap.String("acpSessionId", acpSessionID))
		result, newACPSessionID, err = resumeCapable.ExecuteWithResume(ctx, execReq, acpSessionID, func(chunk Chunk) {
			outputBuilder.WriteString(chunk.Content)
			es.broadcastChunk(req.ThreadID, invocation.ID, chunk, config.ID.String(), config.Name)
		})
	} else {
		// 普通执行（新 session）
		result, err = adapter.ExecuteWithStream(ctx, execReq, func(chunk Chunk) {
			outputBuilder.WriteString(chunk.Content)
			es.broadcastChunk(req.ThreadID, invocation.ID, chunk, config.ID.String(), config.Name)
		})
		if result != nil {
			newACPSessionID = result.SessionID
		}
	}

	// 保存 ACP session ID 到数据库
	if newACPSessionID != "" && es.sessionManager != nil {
		saveErr := es.sessionManager.SaveACPSessionID(ctx, req.ThreadID.String(), config.ID.String(), newACPSessionID, baseAgent.Type)
		if saveErr != nil {
			logError("SaveACPSessionID failed", zap.Error(saveErr))
		}
	}

	// 会话恢复失败降级机制
	if err != nil && acpSessionID != "" && isResumeFallbackError(err) {
		logWarn("Session resume failed, falling back to new session",
			zap.String("invocationId", invocation.ID.String()),
			zap.String("acpSessionId", acpSessionID),
			zap.Error(err))

		// 降级：使用新会话重试
		outputBuilder.Reset()
		result, err = adapter.ExecuteWithStream(ctx, execReq, func(chunk Chunk) {
			outputBuilder.WriteString(chunk.Content)
			es.broadcastChunk(req.ThreadID, invocation.ID, chunk, config.ID.String(), config.Name)
		})
		if result != nil {
			newACPSessionID = result.SessionID
		} else {
			newACPSessionID = ""
		}

		if err == nil {
			logInfo("Session fallback succeeded, created new session",
				zap.String("invocationId", invocation.ID.String()),
				zap.String("newAcpSessionId", newACPSessionID))

			// Fallback 成功后保存新的 session ID 到 session_records 表
			if newACPSessionID != "" && es.sessionManager != nil {
				saveErr := es.sessionManager.SaveACPSessionID(ctx, req.ThreadID.String(), config.ID.String(), newACPSessionID, baseAgent.Type)
				if saveErr != nil {
					logError("SaveACPSessionID after fallback failed", zap.Error(saveErr))
				} else {
					logInfo("Saved fallback session ID to session_records",
						zap.String("threadId", req.ThreadID.String()),
						zap.String("agentId", config.ID.String()),
						zap.String("acpSessionId", newACPSessionID))
				}
			}
		}
	}

	cliDuration := time.Since(cliStart)
	logInfo("[PERF] CLI execution completed", zap.Duration("duration", cliDuration), zap.String("invocationID", invocation.ID.String()), zap.Bool("hasResult", result != nil), zap.String("resultSessionId", func() string {
		if result != nil {
			return result.SessionID
		} else {
			return "nil_result"
		}
	}()), zap.Bool("hasError", err != nil))

	if err != nil {
		// 检查是否因 AskUserQuestion 等待用户输入而取消
		es.mu.Lock()
		runningAgent, agentExists := es.runningAgents[invocation.ID]
		isWaitingForInput := agentExists && runningAgent.WaitingForUserInput
		es.mu.Unlock()

		if isWaitingForInput {
			// 因 AskUserQuestion 等待用户输入而取消
			// 关键修复：将 invocation 状态更新为 "interrupted"，以便 shouldAutoResume 可以找到
			// 并触发 --resume 恢复会话（继续处理用户的 AskUserQuestion 答案）
			logInfo("CLI canceled due to AskUserQuestion waiting for user input",
				zap.String("invocationID", invocation.ID.String()),
				zap.String("sessionId", func() string {
					if result != nil {
						return result.SessionID
					} else {
						return ""
					}
				}()))

			// 设置 invocation 为 interrupted 状态（终态）
			// 这样 FindRecentlyCompletedByThread 可以查询到，触发 --resume
			invocation.Status = model.InvocationStatusInterrupted
			invocation.CompletedAt = timePtr(time.Now())

			// 保存 sessionId 用于后续 --resume
			if result != nil && result.SessionID != "" {
				invocation.SessionID = result.SessionID
			}

			// 使用新的 context 保存 invocation 状态（因为原 ctx 已被取消）
			saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer saveCancel()

			if updateErr := es.invocationRepo.Update(saveCtx, invocation); updateErr != nil {
				logError("Failed to save interrupted invocation", zap.Error(updateErr))
			} else {
				logInfo("Invocation saved as interrupted for --resume",
					zap.String("invocationID", invocation.ID.String()),
					zap.String("sessionId", invocation.SessionID))
			}

			// 获取已累积的内容块（需要保存到消息）
			es.mu.Lock()
			runningAgentForSave, _ := es.runningAgents[invocation.ID]
			var contentBlocksForSave []ContentBlockData
			if runningAgentForSave != nil {
				runningAgentForSave.ContentBlocksMu.Lock()
				contentBlocksForSave = runningAgentForSave.AccumulatedContentBlocks
				runningAgentForSave.ContentBlocksMu.Unlock()
			}
			es.mu.Unlock()

			// 保存 Agent 消息到数据库（包含已累积的内容块）
			// 这样刷新页面后对话不会丢失
			if len(contentBlocksForSave) > 0 {
				// 提取文本内容
				var outputBuilderForSave strings.Builder
				for _, block := range contentBlocksForSave {
					if block.Type == "text" {
						outputBuilderForSave.WriteString(block.Content)
					}
				}
				outputForSave := outputBuilderForSave.String()

				msgForSave := es.saveAgentMessageWithReturn(saveCtx, req.ThreadID, invocation.ID, config, baseAgent, outputForSave, contentBlocksForSave)
				if msgForSave != nil {
					es.broadcastAgentMessage(req.ThreadID, invocation.ID, msgForSave, config.Name, string(config.Role))
				}
				logInfo("Interrupted: saved agent message to database",
					zap.String("invocationID", invocation.ID.String()),
					zap.Int("contentBlocksCount", len(contentBlocksForSave)))
			}

			// 广播 interrupted 状态，通知前端 Agent 已中断等待用户输入
			es.broadcastStatus(invocation.ThreadID, invocation.ID, "interrupted",
				invocation.Role, "", invocation.AgentConfigID.String(), "")

			// 不调用 handleAgentError，已正确更新为 interrupted 状态
			// 用户响应后通过 shouldAutoResume 找到该 invocation，使用 --resume 恢复
			return
		}

		logError("Adapter.ExecuteWithStream failed", zap.Error(err))
		// 执行失败时也保存 sessionId 用于问题定位
		if result != nil && result.SessionID != "" {
			// 保存 sessionId 到 invocation（入库用于问题定位）
			// 注意：此时 ctx 可能已被取消，使用新的 context
			// 关键：先检查数据库状态是否已被取消，避免覆盖 cancelled 状态
			checkCtx, checkCancel := context.WithTimeout(context.Background(), 2*time.Second)
			currentInvocation, checkErr := es.invocationRepo.FindByID(checkCtx, invocation.ID)
			checkCancel()
			if checkErr == nil && currentInvocation != nil && currentInvocation.Status == model.InvocationStatusCancelled {
				logInfo("Execution failed but invocation was cancelled, skip sessionId save",
					zap.String("invocationID", invocation.ID.String()))
				// 直接返回，不覆盖 cancelled 状态
				es.handleAgentErrorWithContext(ctx, invocation, fmt.Errorf("adapter execution failed: %w", err), execReq)
				return
			}

			invocation.SessionID = result.SessionID
			saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer saveCancel()
			if updateErr := es.invocationRepo.Update(saveCtx, invocation); updateErr != nil {
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
		logInfo("Session ID assigned to invocation", zap.String("invocationID", invocation.ID.String()), zap.String("sessionId", result.SessionID))
		// 记录成功会话（通过 sessionRecorder）
		if globalSessionRecorder != nil {
			globalSessionRecorder.RecordSuccessfulSession(req.ThreadID.String(), config.ID.String(), result.SessionID)
		}
		logInfo("Session ID saved for future resume and persistence", zap.String("sessionKey", sessionKey), zap.String("sessionId", result.SessionID))
	} else {
		logWarn("Session ID not saved: result nil or empty sessionId", zap.Bool("hasResult", result != nil), zap.String("sessionId", func() string {
			if result != nil {
				return result.SessionID
			} else {
				return "nil"
			}
		}()))
	}

	output := outputBuilder.String()
	logInfo("Execution completed", zap.Int("outputLength", len(output)), zap.String("invocationID", invocation.ID.String()), zap.String("invocationSessionId", invocation.SessionID))

	// 更新调用记录前，检查是否已被取消（使用新的 context，因为 ctx 可能已被取消）
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 2*time.Second)
	currentInvocation, checkErr := es.invocationRepo.FindByID(checkCtx, invocation.ID)
	checkCancel()
	if checkErr == nil && currentInvocation != nil && currentInvocation.Status == model.InvocationStatusCancelled {
		logInfo("Execution completed but invocation was cancelled, skip status update",
			zap.String("invocationID", invocation.ID.String()))
		return
	}

	// 更新调用记录
	invocation.Status = model.InvocationStatusCompleted
	invocation.Output = output
	invocation.CompletedAt = timePtr(time.Now())
	if err := es.invocationRepo.Update(ctx, invocation); err != nil {
		logError("Failed to update invocation", zap.Error(err))
	} else {
		logInfo("Invocation updated successfully", zap.String("invocationID", invocation.ID.String()), zap.String("status", string(invocation.Status)), zap.String("sessionId", invocation.SessionID))
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

	// 注：以前这里有"从上一条 agent message 继承已回答的 question blocks"的逻辑，
	// 目的是想让 question 卡片在刷新页面后仍能展示。但这会导致每条新 assistant
	// message 都"挂"着同一个 question block，前端就在后续每个对话末尾都看到一次。
	// 现在删掉——前端会从原始消息（首次出现 question 的那条）自然渲染，历史里只渲染一次。

	// 保存输出消息到数据库（包含内容块）
	msg := es.saveAgentMessageWithReturn(ctx, req.ThreadID, invocation.ID, config, baseAgent, output, contentBlocks)

	// 广播消息（让前端用真实 ID 更新）
	if msg != nil {
		es.broadcastAgentMessage(req.ThreadID, invocation.ID, msg, config.Name, string(config.Role))
	}

	// 广播完成状态
	es.broadcastStatus(req.ThreadID, invocation.ID, "completed", config.Role, config.Name, config.ID.String(), "")

	// 追加 Agent 输出到 A2AContext.PreviousResponses（链路历史）
	// 优先提取 a2a-handoff，无 handoff 时降级存储截断摘要
	es.a2aMu.Lock()
	if a2aCtx, exists := es.a2aContexts[req.ThreadID]; exists {
		// 提取 handoff 块（含标签，便于下游识别）
		handoffBlock, hasHandoff := ExtractHandoffBlockWithTags(output)
		var storedContent string
		if hasHandoff {
			// 有 handoff：存储完整块（含标签）
			storedContent = handoffBlock
			logInfo("A2A: 提取到 handoff 块（含标签）",
				zap.String("agentId", config.ID.String()),
				zap.String("agentName", config.Name),
				zap.Int("handoffLen", len(handoffBlock)))
		} else {
			// 无 handoff：降级存储截断摘要
			storedContent = output
			if len(output) > 800 {
				storedContent = TruncateHeadTail(output, 800)
			}
			logInfo("A2A: 无 handoff，存储截断摘要",
				zap.String("agentId", config.ID.String()),
				zap.String("agentName", config.Name),
				zap.Int("storedLen", len(storedContent)))
		}
		a2aCtx.PreviousResponses = append(a2aCtx.PreviousResponses, ChainResponse{
			AgentID:   config.ID,
			AgentName: config.Name,
			Content:   storedContent,
			Role:      string(config.Role),
			Timestamp: time.Now().Unix(),
		})
		a2aCtx.ChainIndex++
		logInfo("A2A: Agent 输出追加到 PreviousResponses",
			zap.String("agentId", config.ID.String()),
			zap.String("agentName", config.Name),
			zap.Int("previousResponsesLen", len(a2aCtx.PreviousResponses)),
			zap.Int("chainIndex", a2aCtx.ChainIndex),
			zap.String("threadId", req.ThreadID.String()))
	} else {
		// 如果不存在 A2AContext，创建一个新的（兜底）
		truncatedOutput := output
		if len(output) > 800 {
			truncatedOutput = TruncateHeadTail(output, 800)
		}
		a2aCtx = &A2AContext{
			Depth:           0,
			InvokedAgents:   make(map[uuid.UUID]bool),
			CompletedAgents: make(map[uuid.UUID]bool),
			PreviousResponses: []ChainResponse{
				{
					AgentID:   config.ID,
					AgentName: config.Name,
					Content:   truncatedOutput,
					Role:      string(config.Role),
					Timestamp: time.Now().Unix(),
				},
			},
			ChainIndex: 1,
		}
		es.a2aContexts[req.ThreadID] = a2aCtx
		logInfo("A2A: 创建 A2AContext 并追加 Agent 输出",
			zap.String("agentId", config.ID.String()),
			zap.String("agentName", config.Name),
			zap.String("threadId", req.ThreadID.String()))
	}
	es.a2aMu.Unlock()

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

func (es *ExecutionService) loadBoundMCPServers(ctx context.Context, config *model.AgentRoleConfig, baseAgent *model.BaseAgent) []*model.MCPServer {
	if es.mcpBindingRepo == nil || config == nil {
		return nil
	}
	servers, err := es.mcpBindingRepo.FindServersByAgentRoleID(ctx, config.ID)
	if err != nil {
		logWarn("Failed to load bound MCP servers",
			zap.String("agentConfigID", config.ID.String()),
			zap.Error(err))
		return nil
	}
	baseAgentType := ""
	if baseAgent != nil {
		baseAgentType = string(baseAgent.Type)
	}
	logInfo("Loaded bound MCP servers",
		zap.String("agentConfigID", config.ID.String()),
		zap.String("baseAgentType", baseAgentType),
		zap.Int("count", len(servers)))
	return servers
}

// saveAgentMessage 保存Agent消息
func (es *ExecutionService) saveAgentMessage(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, baseAgent *model.BaseAgent, output string, contentBlocks []ContentBlockData) {
	metadata := map[string]string{
		"agentName": config.Name,
		"agentRole": string(config.Role),
	}
	// 添加基础Agent信息
	if baseAgent != nil {
		metadata["baseAgentType"] = string(baseAgent.Type)
		metadata["baseAgentModel"] = baseAgent.DefaultModel
		// 从插件注册中心获取类型名称
		pluginMeta := GetMeta(baseAgent.Type)
		if pluginMeta != nil {
			metadata["baseAgentTypeName"] = pluginMeta.Name
		}
	}
	metadataJSON, _ := json.Marshal(metadata)

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
		Metadata:      metadataJSON,
		CreatedAt:     time.Now(),
	}
	if err := es.msgRepo.Create(ctx, msg); err != nil {
		logError("Failed to save agent message", zap.Error(err))
	}
}

// saveAgentMessageWithReturn 保存Agent消息并返回消息对象（含真实ID）
// invocationID 用于前端关联临时消息和真实消息
func (es *ExecutionService) saveAgentMessageWithReturn(ctx context.Context, threadID uuid.UUID, invocationID uuid.UUID, config *model.AgentRoleConfig, baseAgent *model.BaseAgent, output string, contentBlocks []ContentBlockData) *model.Message {
	metadata := map[string]string{
		"agentName":    config.Name,
		"agentRole":    string(config.Role),
		"invocationId": invocationID.String(),
	}
	// 添加基础Agent信息
	if baseAgent != nil {
		metadata["baseAgentType"] = string(baseAgent.Type)
		metadata["baseAgentModel"] = baseAgent.DefaultModel
		// 从插件注册中心获取类型名称
		pluginMeta := GetMeta(baseAgent.Type)
		if pluginMeta != nil {
			metadata["baseAgentTypeName"] = pluginMeta.Name
		}
	}
	metadataJSON, _ := json.Marshal(metadata)

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
		Metadata:      metadataJSON,
		CreatedAt:     time.Now(),
	}
	if err := es.msgRepo.Create(ctx, msg); err != nil {
		logError("Failed to save agent message", zap.Error(err))
		return nil
	}
	return msg
}

// broadcastAgentMessage 广播Agent消息（让前端用真实ID更新）
// invocationID 用于前端关联临时消息和真实消息
func (es *ExecutionService) broadcastAgentMessage(threadID uuid.UUID, invocationID uuid.UUID, msg *model.Message, agentName, agentRole string) {
	if es.wsHub != nil {
		// 解析内容块
		var contentBlocks []ContentBlockData
		if len(msg.ContentBlocks) > 0 {
			json.Unmarshal(msg.ContentBlocks, &contentBlocks)
		}

		// 解析 metadata
		var metadata map[string]string
		if len(msg.Metadata) > 0 {
			json.Unmarshal(msg.Metadata, &metadata)
		}

		es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_message",
			ThreadID:  threadID.String(),
			Timestamp: msg.CreatedAt.UnixMilli(),
			Payload: map[string]interface{}{
				"messageId":     msg.ID.String(),
				"invocationId":  invocationID.String(),
				"agentId":       msg.AgentID,
				"content":       msg.Content,
				"contentBlocks": contentBlocks,
				"agentName":     agentName,
				"agentRole":     agentRole,
				"metadata":      metadata,
			},
		})
	}
}

// getAdapter 获取适配器
func (es *ExecutionService) getAdapter(ctx context.Context, config *model.AgentRoleConfig, baseAgent *model.BaseAgent) (AgentAdapter, error) {
	// 如果有 BaseAgent，使用 Registry 获取适配器
	if baseAgent != nil {
		adapter := GetAdapter(baseAgent)
		if adapter == nil {
			return nil, fmt.Errorf("不支持的基础Agent类型: %s", baseAgent.Type)
		}
		return adapter, nil
	}

	// 如果配置了BaseAgentID但没有传入baseAgent，尝试获取
	if config.BaseAgentID != uuid.Nil && es.baseAgentRepo != nil {
		ba, err := es.baseAgentRepo.FindByID(ctx, config.BaseAgentID)
		if err == nil {
			adapter := GetAdapter(ba)
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
	// 检查 invocation 是否已经被取消，避免覆盖 cancelled 状态
	// 使用新的 context，因为传入的 ctx 可能已被取消
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer checkCancel()
	currentInvocation, findErr := es.invocationRepo.FindByID(checkCtx, invocation.ID)
	if findErr == nil && currentInvocation != nil {
		if currentInvocation.Status == model.InvocationStatusCancelled {
			logInfo("handleAgentError: invocation already cancelled, skip error handling",
				zap.String("invocationID", invocation.ID.String()))
			return
		}
	}

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
	// 检查 invocation 是否已经被取消，避免覆盖 cancelled 状态
	// 使用新的 context，因为传入的 ctx 可能已被取消
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 2*time.Second)
	currentInvocation, findErr := es.invocationRepo.FindByID(checkCtx, invocation.ID)
	checkCancel()
	if findErr == nil && currentInvocation != nil {
		if currentInvocation.Status == model.InvocationStatusCancelled {
			logInfo("handleAgentErrorWithContext: invocation already cancelled, skip error handling",
				zap.String("invocationID", invocation.ID.String()))
			return
		}
	}

	// 输出详细诊断日志
	es.logErrorDiagnostics(ctx, invocation, err, execReq)

	// 获取 sessionID 和 isResumeAttempt 用于生成建议
	var sessionID string
	var isResumeAttempt bool
	if execReq != nil {
		sessionID = execReq.SessionID
		isResumeAttempt = sessionID != ""
	}

	// 生成建议
	suggestions := generateErrorSuggestions(err, sessionID, isResumeAttempt)

	// 构建详细错误输出（用于前端展示）
	errorOutput := buildDetailedErrorOutput(err, execReq, suggestions)

	invocation.Status = model.InvocationStatusFailed
	invocation.Output = errorOutput
	invocation.CompletedAt = timePtr(time.Now())
	if updateErr := es.invocationRepo.Update(ctx, invocation); updateErr != nil {
		logError("Failed to update invocation on error", zap.Error(updateErr))
	}

	// 广播失败状态，包含详细错误信息（通过 input 参数传递）
	es.broadcastStatus(invocation.ThreadID, invocation.ID, "failed", invocation.Role, "", invocation.AgentConfigID.String(), errorOutput)
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
		"resource not found",   // ACP session/resume 找不到 session
		"pipe is being closed", // 进程退出导致管道关闭
		"broken pipe",          // 管道断裂
		"process not alive",    // 进程已退出
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

// buildEmptyOutputError 构建空输出错误信息
// 包含 CLI stderr 输出用于诊断
func buildEmptyOutputError(cliStderr string) string {
	var sb strings.Builder

	sb.WriteString("执行返回空内容\n\n")

	// 可能原因
	sb.WriteString("可能原因:\n")
	sb.WriteString("1. CLI 进程异常退出\n")
	sb.WriteString("2. 通知处理超时\n")
	sb.WriteString("3. Agent 无响应\n")
	sb.WriteString("4. 上下文过长导致 CLI 拒绝响应\n\n")

	// 如果有 stderr 输出，展示给用户
	if cliStderr != "" {
		sb.WriteString("CLI 错误输出:\n")
		sb.WriteString("---\n")
		// 限制 stderr 输出长度，避免过长
		stderrDisplay := cliStderr
		if len(stderrDisplay) > 2000 {
			stderrDisplay = stderrDisplay[:2000] + "\n... (输出过长，已截断)"
		}
		sb.WriteString(stderrDisplay)
		sb.WriteString("\n---\n\n")
	}

	// 建议
	sb.WriteString("建议:\n")
	sb.WriteString("- 刷新页面重新发起对话\n")
	sb.WriteString("- 如果问题持续，检查 CLI 配置是否正确\n")
	sb.WriteString("- 查看 server.log 获取详细日志\n")

	return sb.String()
}

// buildDetailedErrorOutput 构建详细的错误输出信息，用于前端展示
// 包含错误类型、原始错误、诊断信息、解决建议
func buildDetailedErrorOutput(err error, execReq *ExecutionRequest, suggestions []string) string {
	var sb strings.Builder

	sb.WriteString("执行失败\n")
	sb.WriteString(fmt.Sprintf("\n错误类型: %s\n", getErrorType(err)))
	sb.WriteString(fmt.Sprintf("\n错误详情: %s\n", err.Error()))

	// 诊断信息
	if execReq != nil {
		sb.WriteString("\n诊断信息:\n")
		if execReq.SessionID != "" {
			sessionIDDisplay := execReq.SessionID
			if len(sessionIDDisplay) > 8 {
				sessionIDDisplay = sessionIDDisplay[:8] + "..."
			}
			sb.WriteString(fmt.Sprintf("- SessionID: %s\n", sessionIDDisplay))
		}
		if execReq.WorkDir != "" {
			sb.WriteString(fmt.Sprintf("- 工作目录: %s\n", execReq.WorkDir))
		}
		if execReq.ConfigDir != "" {
			sb.WriteString(fmt.Sprintf("- 配置目录: %s\n", execReq.ConfigDir))
		}
		if execReq.BaseAgent != nil && execReq.BaseAgent.DefaultModel != "" {
			sb.WriteString(fmt.Sprintf("- 模型: %s\n", execReq.BaseAgent.DefaultModel))
		}
		// 输入长度提示
		if len(execReq.Input) > 0 {
			inputLen := len(execReq.Input)
			sb.WriteString(fmt.Sprintf("- 输入长度: %d 字符\n", inputLen))
			if inputLen > 10000 {
				sb.WriteString("  (输入较长，可能影响处理)\n")
			}
		}
		// 历史长度提示
		if execReq.Context != nil && execReq.Context.Layer1 != "" {
			historyLines := strings.Count(execReq.Context.Layer1, "\n")
			sb.WriteString(fmt.Sprintf("- 对话历史: 约 %d 行\n", historyLines))
			if historyLines > 500 {
				sb.WriteString("  (历史较长，可能接近上下文限制)\n")
			}
		}
	}

	// 建议
	if len(suggestions) > 0 {
		sb.WriteString("\n建议:\n")
		for _, s := range suggestions {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}

	sb.WriteString("\n详细信息请查看 server.log\n")
	return sb.String()
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

	// 3. 确定工作流模板ID：任务绑定团队优先，项目绑定团队仅作为兜底。
	workflowTemplateID := selectThreadTeamWorkflowTemplateID(thread, project)

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
// 根据场景决定注入哪些层，避免 Resume 场景下的重复注入：
// - 单个角色 Resume：仅用户输入（CLI 内部已有完整上下文）
// - 单个角色 New：Layer 0 + Layer 2-3 + MemoryContext + 用户输入
// - A2A Resume：Layer 2-3 + ChainHistory + MemoryContext + 用户输入
// - A2A New：Layer 0 + Layer 2-3 + ChainHistory + MemoryContext + 用户输入
// Layer 1（Thread 历史）在任何场景都不注入：
// - Resume 场景 CLI 内部已有历史
// - New 场景没有历史
// - A2A 场景 ChainHistory 已包含必要的上游信息
func (es *ExecutionService) buildContextLayers(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, req *SpawnRequest) (*ContextLayers, error) {
	layers := &ContextLayers{}

	// 预加载上下文（一次性获取所有数据）
	tc, err := es.getThreadContext(ctx, threadID)
	if err != nil {
		return nil, err
	}

	// 判断场景
	isA2A := req != nil && req.ChainHistory != nil
	isResume := req != nil && req.SessionStrategy == SessionStrategyResume

	// 场景判断日志
	if req != nil {
		logInfo("buildContextLayers: 场景判断",
			zap.Bool("isA2A", isA2A),
			zap.Bool("isResume", isResume),
			zap.String("sessionStrategy", string(req.SessionStrategy)))
	} else {
		logInfo("buildContextLayers: 场景判断",
			zap.Bool("isA2A", isA2A),
			zap.Bool("isResume", isResume),
			zap.String("sessionStrategy", "nil"))
	}

	// Layer 1（Thread 历史）不再注入
	// 原因：Resume 场景 CLI 内部已有；New 场景没有历史；A2A 场景用 ChainHistory
	layers.Layer1 = ""

	// 根据场景注入 Layer 0（角色定义）
	// 规则：始终注入角色 prompt，防止 CLI 内部历史被压缩后角色边界模糊
	// - New / A2A 场景：使用完整版（含治理摘要等）
	// - Resume 场景：使用轻量版（仅角色定义 + SystemPrompt + 下游协作方）
	if !isResume {
		layers.Layer0 = es.buildDynamicSystemPromptFromContext(tc, config)
		logInfo("buildContextLayers: 注入 Layer 0（角色定义，完整版）", zap.Int("length", len(layers.Layer0)))
	} else {
		layers.Layer0 = es.buildResumeRolePromptFromContext(tc, config)
		logInfo("buildContextLayers: 注入 Layer 0（角色定义，Resume 轻量版）", zap.Int("length", len(layers.Layer0)))
	}

	// 根据场景注入 ChainHistory（A2A 链路历史）
	// 规则：仅 A2A 场景注入
	if isA2A {
		layers.ChainHistory = BuildChainHistoryLayer(req.ChainHistory)
		logInfo("buildContextLayers: 注入 ChainHistory（A2A 链路历史）", zap.Int("length", len(layers.ChainHistory)))
	} else {
		layers.ChainHistory = ""
	}

	// 根据场景注入 Layer 2-3（工作产物和环境信息）
	// 规则：除"单个角色 Resume"外都注入（动态环境信息需要最新状态）
	if isA2A || !isResume {
		layers.Layer2 = es.getArtifacts(tc.Thread)
		layers.Layer3 = es.getEnvironmentInfo(tc.Thread)
		logInfo("buildContextLayers: 注入 Layer 2-3",
			zap.Int("layer2Length", len(layers.Layer2)),
			zap.Int("layer3Length", len(layers.Layer3)))
	} else {
		// 单个角色 Resume：CLI 内部已有，不需要注入
		layers.Layer2 = ""
		layers.Layer3 = ""
		logInfo("buildContextLayers: 跳过 Layer 2-3（单个角色 Resume CLI 内部已有）")
	}

	// 根据场景注入 MemoryContext（记忆索引）
	// 规则：除"单个角色 Resume"外都注入（记忆可能在对话中更新）
	if isA2A || !isResume {
		if es.memoryManager != nil {
			threadIDStr := threadID.String()
			agentIDStr := config.ID.String()

			// Team 级记忆绑定 WorkflowTemplate.ID
			teamIDStr := ""
			if tc.WorkflowTemplate != nil {
				teamIDStr = tc.WorkflowTemplate.ID.String()
			}

			// Project 级记忆绑定 Project.ID
			projectIDStr := ""
			workspacePath := ""
			if tc.Project != nil {
				projectIDStr = tc.Project.ID.String()
				workspacePath = tc.Project.LocalPath
			}

			memoryIndex := es.memoryManager.BuildAutoMemoryIndexBlock(ctx, memory.MemoryScopeIdentity{
				TeamID:        teamIDStr,
				ProjectID:     projectIDStr,
				WorkspacePath: workspacePath,
			}, 30)
			if memoryIndex != "" {
				layers.MemoryContext = memoryIndex
				logInfo("buildContextLayers: 注入 MemoryContext",
					zap.String("threadID", threadIDStr),
					zap.String("agentID", agentIDStr),
					zap.Int("memoryLength", len(layers.MemoryContext)))
			}
		}
	} else {
		// 单个角色 Resume：CLI 内部已有记忆索引
		logInfo("buildContextLayers: 跳过 MemoryContext（单个角色 Resume CLI 内部已有）")
	}

	return layers, nil
}

func (es *ExecutionService) resolveWorkspacePath(ctx context.Context, threadID uuid.UUID, req *SpawnRequest) string {
	if req != nil && strings.TrimSpace(req.ProjectPath) != "" {
		return strings.TrimSpace(req.ProjectPath)
	}
	tc, err := es.getThreadContext(ctx, threadID)
	if err == nil && tc != nil && tc.Project != nil {
		return tc.Project.LocalPath
	}
	if es.projectRepo != nil {
		project, err := es.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil {
			return project.LocalPath
		}
	}
	return ""
}

func selectThreadTeamWorkflowTemplateID(thread *model.Thread, project *model.Project) *uuid.UUID {
	if thread != nil && thread.WorkflowTemplateID != nil {
		return thread.WorkflowTemplateID
	}
	if project != nil && project.WorkflowTemplateID != nil {
		return project.WorkflowTemplateID
	}
	return nil
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

	// 1. 角色定义（最核心，优先注入）
	sb.WriteString(fmt.Sprintf("你是 %s (%s)。\n\n", config.Name, config.Description))
	sb.WriteString(config.SystemPrompt)
	sb.WriteString("\n\n")

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

	// 2. 协作治理规则（合并版：治理摘要 + 下游协作方）
	sb.WriteString("---\n\n")
	sb.WriteString(BuildGovernanceDigestWithVersion())
	sb.WriteString("\n")
	sb.WriteString(BuildMemoryToolGovernance())
	sb.WriteString("\n")

	// 动态追加下游协作方（合并到 L0）
	if len(transitions) > 0 {
		sb.WriteString("\n**你的下游协作方**：\n")
		for _, t := range transitions {
			toAgent := agentMap[t.ToAgentID]
			var hint string
			if t.TriggerHint != "" {
				hint = t.TriggerHint
			} else if toAgent != nil {
				hint = generateTriggerHint(toAgent)
			} else {
				hint = fmt.Sprintf("@%s", t.ToAgentID[:8])
			}
			sb.WriteString(fmt.Sprintf("- %s\n", hint))
		}
	} else {
		sb.WriteString("\n**无下游协作方** — 直接结束即可。\n")
	}

	sb.WriteString("\n---\n\n")

	return sb.String()
}

// buildResumeRolePromptFromContext 构建 Resume 场景下注入的角色提示（轻量版）
// 仅注入角色定义 + SystemPrompt + 下游协作方，去掉治理摘要等大体积内容
// 目的：CLI 内部历史会被压缩，每轮注入角色定义抵抗上下文压缩、防止职责边界模糊
func (es *ExecutionService) buildResumeRolePromptFromContext(tc *ThreadContext, config *model.AgentRoleConfig) string {
	var sb strings.Builder

	// 1. 角色定义（最核心）
	sb.WriteString(fmt.Sprintf("你是 %s (%s)。\n\n", config.Name, config.Description))
	sb.WriteString(config.SystemPrompt)
	sb.WriteString("\n")

	// 2. 下游协作方（简短，影响 A2A 路由）
	var transitions []model.Transition
	agentIDStr := config.ID.String()
	for _, t := range tc.Transitions {
		if t.FromAgentID == agentIDStr {
			transitions = append(transitions, t)
		}
	}

	if len(transitions) > 0 {
		agentMap := make(map[string]*model.AgentRoleConfig)
		for _, agent := range tc.AllowedAgents {
			agentMap[agent.ID.String()] = agent
		}
		sb.WriteString("\n**你的下游协作方**：\n")
		for _, t := range transitions {
			toAgent := agentMap[t.ToAgentID]
			var hint string
			if t.TriggerHint != "" {
				hint = t.TriggerHint
			} else if toAgent != nil {
				hint = generateTriggerHint(toAgent)
			} else {
				hint = fmt.Sprintf("@%s", t.ToAgentID[:8])
			}
			sb.WriteString(fmt.Sprintf("- %s\n", hint))
		}
	}

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

// getRoutableTeamAgents 获取可路由团队的 AllowedAgents
// 用于 T2T (Team-to-Team) 跨团队协作支持
// 数据流: Thread → WorkflowTemplate.RoutableTeams → 目标 Team → AgentIDs → AgentConfigs
func (es *ExecutionService) getRoutableTeamAgents(ctx context.Context, threadID uuid.UUID) []*model.AgentRoleConfig {
	tc, err := es.getThreadContext(ctx, threadID)
	if err != nil || tc.WorkflowTemplate == nil {
		return nil
	}

	// 解析 RoutableTeams JSON
	var routableTeamIDs []string
	if len(tc.WorkflowTemplate.RoutableTeams) > 0 {
		if err := json.Unmarshal(tc.WorkflowTemplate.RoutableTeams, &routableTeamIDs); err != nil {
			logError("Failed to parse RoutableTeams", zap.Error(err))
			return nil
		}
	}

	if len(routableTeamIDs) == 0 {
		return nil
	}

	logInfo("T2T: 解析 RoutableTeams",
		zap.String("threadId", threadID.String()),
		zap.Strings("routableTeamIDs", routableTeamIDs),
		zap.Int("count", len(routableTeamIDs)))

	// 收集所有可路由团队的 Agent
	var allAgents []*model.AgentRoleConfig
	for _, teamIDStr := range routableTeamIDs {
		teamID, err := uuid.Parse(teamIDStr)
		if err != nil {
			logError("Invalid team ID in RoutableTeams", zap.String("teamID", teamIDStr), zap.Error(err))
			continue
		}

		// 获取目标团队的工作流模板
		teamTemplate, err := es.workflowRepo.FindByID(ctx, teamID)
		if err != nil {
			logInfo("RoutableTeam not found", zap.String("teamID", teamIDStr), zap.Error(err))
			continue
		}

		// 解析目标团队的 AgentIDs
		var agentIDs []string
		if len(teamTemplate.AgentIDs) > 0 {
			if err := json.Unmarshal(teamTemplate.AgentIDs, &agentIDs); err != nil {
				logError("Failed to parse AgentIDs from RoutableTeam", zap.String("teamID", teamIDStr), zap.Error(err))
				continue
			}
		}

		logInfo("T2T: 获取目标团队 Agent",
			zap.String("teamID", teamIDStr),
			zap.String("teamName", teamTemplate.Name),
			zap.Int("agentCount", len(agentIDs)))

		// 获取每个 Agent 的配置
		for _, idStr := range agentIDs {
			id, err := uuid.Parse(idStr)
			if err != nil {
				continue
			}
			agent, err := es.configSvc.GetByID(ctx, id)
			if err == nil {
				allAgents = append(allAgents, agent)
			}
		}
	}

	logInfo("T2T: 可路由团队 Agent 获取完成",
		zap.String("threadId", threadID.String()),
		zap.Int("totalAgents", len(allAgents)))

	return allAgents
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

	// 获取工作流模板中的 Agent 列表（当前团队）
	currentTeamAgents := es.getAllowedAgentsFromWorkflow(ctx, threadID)
	// 获取可路由团队的 Agent（T2T 支持）
	routableTeamAgents := es.getRoutableTeamAgents(ctx, threadID)

	// 构建 Agent ID -> AgentConfig 映射（合并当前团队和可路由团队）
	agentMap := make(map[string]*model.AgentRoleConfig)
	for _, agent := range currentTeamAgents {
		agentMap[agent.ID.String()] = agent
	}
	for _, agent := range routableTeamAgents {
		agentMap[agent.ID.String()] = agent // 跨团队 Agent 加入映射
	}

	logInfo("T2T 路由: Agent 映射构建完成",
		zap.String("threadId", threadID.String()),
		zap.Int("currentTeamAgents", len(currentTeamAgents)),
		zap.Int("routableTeamAgents", len(routableTeamAgents)),
		zap.Int("totalAgents", len(agentMap)))

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
	totalAgentsToTrigger := len(agentsToTrigger)
	triggeredCount := 0
	for _, targetConfig := range agentsToTrigger {
		triggeredCount++
		// 更新 A2A 上下文
		es.a2aMu.Lock()
		a2aCtx.InvokedAgents[targetConfig.ID] = true
		// 设置触发者信息（A2A 优化）
		a2aCtx.FromAgent = &AgentInfo{
			ID:   currentConfig.ID,
			Name: currentConfig.Name,
			Role: string(currentConfig.Role),
		}
		es.a2aMu.Unlock()

		// 决定会话策略：只有同一个 Agent ID 才能 resume，跨 Agent 调用使用新会话
		var sessionStrategy SessionStrategy

		// 详细日志：打印 ID 对比
		fromAgentID := ""
		if a2aCtx.FromAgent != nil {
			fromAgentID = a2aCtx.FromAgent.ID.String()
		}
		logInfo("A2A ID对比详情",
			zap.String("fromAgentID", fromAgentID),
			zap.String("toAgentID", targetConfig.ID.String()),
			zap.Bool("fromAgentNotNil", a2aCtx.FromAgent != nil),
			zap.Bool("ID相等", a2aCtx.FromAgent != nil && a2aCtx.FromAgent.ID == targetConfig.ID))

		if a2aCtx.FromAgent != nil && a2aCtx.FromAgent.ID == targetConfig.ID {
			// 同一 Agent 再次调用 → 恢复会话
			sessionStrategy = SessionStrategyResume
			logInfo("A2A 会话策略: 同Agent调用，使用 resume",
				zap.String("fromAgent", a2aCtx.FromAgent.Name),
				zap.String("fromAgentID", a2aCtx.FromAgent.ID.String()),
				zap.String("toAgent", targetConfig.Name),
				zap.String("toAgentID", targetConfig.ID.String()))
		} else {
			// 跨 Agent 调用 → 新会话，不传递历史
			// 即使是同角色，不同 Agent 之间也不共享会话上下文
			sessionStrategy = SessionStrategyNew
			logInfo("A2A 会话策略: 跨Agent调用，使用新会话",
				zap.String("fromAgent", func() string {
					if a2aCtx.FromAgent != nil {
						return a2aCtx.FromAgent.Name
					} else {
						return "nil"
					}
				}()),
				zap.String("fromAgentID", fromAgentID),
				zap.String("toAgent", targetConfig.Name),
				zap.String("toAgentID", targetConfig.ID.String()))

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

		// 构建 ChainHistory（与人类触发统一，使用编号+摘要格式）
		remainingAgents := totalAgentsToTrigger - triggeredCount
		chainHistory := BuildA2AChainContext(a2aCtx, sessionStrategy, remainingAgents, es.tokenBudgetManager)

		// 使用构建的 A2A 输入和 ChainHistory（统一：A2A 触发也传递链路历史）
		es.SpawnAgent(ctx, &SpawnRequest{
			ThreadID:        threadID,
			ConfigID:        targetConfig.ID,
			Role:            targetConfig.Role,
			Input:           a2aInput,
			ProjectPath:     projectPath,
			SessionStrategy: sessionStrategy,
			ChainHistory:    chainHistory, // 统一：A2A 触发也传递链路历史
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

// getOrCreateHumanChainHistory 构建人类触发的链路历史
// 使用 a2aContexts[threadID] 统一管理（与 A2A 触发共用同一上下文）
// 确保人类触发与 A2A 触发的上下文继承行为一致
func (es *ExecutionService) getOrCreateHumanChainHistory(ctx context.Context, threadID uuid.UUID, userInput string) *A2AChainContext {
	es.a2aMu.Lock()
	defer es.a2aMu.Unlock()

	// 获取或创建 A2AContext
	a2aCtx, exists := es.a2aContexts[threadID]
	if !exists {
		// Token 预算保护：截断超长输入
		content := userInput
		if len(content) > 500 {
			content = TruncateHeadTail(content, 500)
		}

		a2aCtx = &A2AContext{
			Depth:           0,
			InvokedAgents:   make(map[uuid.UUID]bool),
			CompletedAgents: make(map[uuid.UUID]bool),
			PreviousResponses: []ChainResponse{
				{
					AgentID:   uuid.Nil,
					AgentName: "User",
					Content:   content,
					Role:      "user",
					Timestamp: time.Now().Unix(),
				},
			},
			OriginalMessage: userInput,
			ChainIndex:      1, // 人类是链路起点
			SessionStrategy: SessionStrategyNew,
		}
		es.a2aContexts[threadID] = a2aCtx
		logInfo("getOrCreateHumanChainHistory: 初始化 A2AContext",
			zap.String("threadID", threadID.String()),
			zap.Int("inputLen", len(userInput)))
	} else {
		// 已有 A2AContext，用户后续消息（与 SpawnAgentForUserMessage 一致）

		// Token 预算保护：截断超长输入
		content := userInput
		if len(content) > 500 {
			content = TruncateHeadTail(content, 500)
		}

		// 1. Append 用户消息到 PreviousResponses
		a2aCtx.PreviousResponses = append(a2aCtx.PreviousResponses, ChainResponse{
			AgentID:   uuid.Nil,
			AgentName: "User",
			Content:   content,
			Role:      "user",
			Timestamp: time.Now().Unix(),
		})

		// 2. 覆盖 OriginalMessage（新意图）
		a2aCtx.OriginalMessage = userInput

		// 3. 增加 ChainIndex
		a2aCtx.ChainIndex++

		// 4. 重置 InvokedAgents/CompletedAgents
		a2aCtx.InvokedAgents = make(map[uuid.UUID]bool)
		a2aCtx.CompletedAgents = make(map[uuid.UUID]bool)

		// 5. 重置 Depth
		a2aCtx.Depth = 0

		// 6. Safeguard: 限制 PreviousResponses 长度
		if len(a2aCtx.PreviousResponses) > MaxPreviousResponses {
			logInfo("PreviousResponses 达到上限，删除最早的条目",
				zap.Int("before", len(a2aCtx.PreviousResponses)),
				zap.Int("after", MaxPreviousResponses))
			a2aCtx.PreviousResponses = a2aCtx.PreviousResponses[len(a2aCtx.PreviousResponses)-MaxPreviousResponses:]
		}

		logInfo("getOrCreateHumanChainHistory: 用户后续消息",
			zap.String("threadID", threadID.String()),
			zap.Int("previousResponsesLen", len(a2aCtx.PreviousResponses)),
			zap.Int("chainIndex", a2aCtx.ChainIndex))
	}

	// 使用 BuildA2AChainContext 构建链路历史（与 A2A 触发统一）
	return BuildA2AChainContext(a2aCtx, SessionStrategyNew, 1, es.tokenBudgetManager)
}

// buildHumanChainHistory 构建人类触发的 ChainHistory（已废弃，使用 getOrCreateHumanChainHistory）
// 将用户输入作为 PreviousResponses 的第一个条目
// 包含 Token 预算保护：截断超长输入
func (es *ExecutionService) buildHumanChainHistory(userInput string) *A2AChainContext {
	// Token 预算保护：截断超长输入
	content := userInput
	if len(content) > 500 {
		content = TruncateHeadTail(content, 500)
	}

	return &A2AChainContext{
		ChainIndex: 1, // 人类是链路起点
		ChainTotal: 1, // 预计链路长度
		PreviousResponses: []ChainResponse{
			{
				AgentID:   uuid.Nil, // 人类无 AgentID
				AgentName: "User",   // 标识为用户
				Content:   content,  // 截断后的内容
				Role:      "user",   // 角色标识
				Timestamp: time.Now().Unix(),
			},
		},
		OriginalMessage: userInput, // 保留完整原始消息
		FromAgent:       nil,       // nil 表示人类触发
		SessionStrategy: SessionStrategyNew,
		Depth:           0,    // 人类触发深度为 0
		A2AEnabled:      true, // 允许后续 A2A
	}
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

	// 方案B：Input 只包含当前输入，历史全部由 Layer1 负责
	// 移除：原始请求（已在 Layer1 conversation 中）
	// 移除：前序分析（已在 Layer1 conversation 中）
	// 移除：会话策略（下游不需要关心）
	// 保留：协作规则、触发者信息（A2A 特有）

	// 1. 协作规则（有触发者信息时注入）
	if a2aCtx != nil && a2aCtx.FromAgent != nil {
		sb.WriteString("## 协作规则\n\n")
		sb.WriteString("A2A 出口检查：回复前问\"到我这里结束了吗？\"不是 → 末尾另起一行行首写 @句柄 触发下游。\n\n")
		sb.WriteString("---\n\n")
	}

	// 2. 触发者信息（Direct message 提示）
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

	// 获取项目路径
	var projectPath string
	if es.projectRepo != nil {
		project, err := es.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil {
			projectPath = project.LocalPath
		}
	}

	// 获取工作流模板中的 Agent 列表（当前团队）
	currentTeamAgents := es.getAllowedAgentsFromWorkflow(ctx, threadID)
	// 获取可路由团队的 Agent（T2T 支持）
	routableTeamAgents := es.getRoutableTeamAgents(ctx, threadID)

	// 合并 Agent 列表用于 mention 解析（T2T 支持）
	allAllowedAgents := append(currentTeamAgents, routableTeamAgents...)

	// 收集所有待触发的 Agent（支持并行和博弈）
	agentsToTrigger := make(map[uuid.UUID]*model.AgentRoleConfig) // 使用 map 去重

	// ========== 1. 解析输出中的 @mention（优先触发，支持博弈场景）==========
	// 使用 ParseForAgents 限制在当前工作流 + 可路由团队的 Agent 范围内（T2T 支持）
	var a2aMentions []string
	if es.mentionParser != nil {
		var err error
		a2aMentions, err = es.mentionParser.ParseForAgents(ctx, output, config.ID.String(), allAllowedAgents)
		if err != nil {
			logError("checkSignalRouting: 解析失败", zap.Error(err))
		}
	}
	logInfo("A2A @mention 解析结果（T2T 扩展范围）",
		zap.String("fromAgent", config.Name),
		zap.Strings("agentIDs", a2aMentions),
		zap.Int("count", len(a2aMentions)),
		zap.Int("totalAllowedAgents", len(allAllowedAgents)))

	// 构建 Agent ID -> AgentConfig 映射（合并当前团队和可路由团队）
	agentMap := make(map[string]*model.AgentRoleConfig)
	for _, agent := range currentTeamAgents {
		agentMap[agent.ID.String()] = agent
	}
	for _, agent := range routableTeamAgents {
		agentMap[agent.ID.String()] = agent // 跨团队 Agent 加入映射
	}

	logInfo("T2T 路由: Agent 映射构建完成",
		zap.String("threadId", threadID.String()),
		zap.Int("currentTeamAgents", len(currentTeamAgents)),
		zap.Int("routableTeamAgents", len(routableTeamAgents)),
		zap.Int("totalAgents", len(agentMap)))

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
		// Human 角色不再通过 @mention 触发
		// Human 任务由 ExecutionService 状态检测机制创建（waiting 状态检测）
		if targetConfig.Role.IsHumanRole() {
			continue // 跳过 Human 角色的 Agent 触发流程
		}

		// 更新 A2A 上下文
		es.a2aMu.Lock()
		a2aCtx.InvokedAgents[targetConfig.ID] = true
		// 设置触发者信息（A2A 优化）
		a2aCtx.FromAgent = &AgentInfo{
			ID:   config.ID,
			Name: config.Name,
			Role: string(config.Role),
		}
		es.a2aMu.Unlock()

		// 决定会话策略：只有同一个 Agent ID 才能 resume，跨 Agent 调用使用新会话
		var sessionStrategy SessionStrategy

		// 详细日志：打印 ID 对比
		fromAgentID := ""
		if a2aCtx.FromAgent != nil {
			fromAgentID = a2aCtx.FromAgent.ID.String()
		}
		logInfo("A2A ID对比详情",
			zap.String("fromAgentID", fromAgentID),
			zap.String("toAgentID", targetConfig.ID.String()),
			zap.Bool("fromAgentNotNil", a2aCtx.FromAgent != nil),
			zap.Bool("ID相等", a2aCtx.FromAgent != nil && a2aCtx.FromAgent.ID == targetConfig.ID))

		if a2aCtx.FromAgent != nil && a2aCtx.FromAgent.ID == targetConfig.ID {
			// 同一 Agent 再次调用 → 恢复会话
			sessionStrategy = SessionStrategyResume
			logInfo("A2A 会话策略: 同Agent调用，使用 resume",
				zap.String("fromAgent", a2aCtx.FromAgent.Name),
				zap.String("fromAgentID", a2aCtx.FromAgent.ID.String()),
				zap.String("toAgent", targetConfig.Name),
				zap.String("toAgentID", targetConfig.ID.String()))
		} else {
			// 跨 Agent 调用 → 新会话，不传递历史
			// 即使是同角色，不同 Agent 之间也不共享会话上下文
			sessionStrategy = SessionStrategyNew
			logInfo("A2A 会话策略: 跨Agent调用，使用新会话",
				zap.String("fromAgent", func() string {
					if a2aCtx.FromAgent != nil {
						return a2aCtx.FromAgent.Name
					} else {
						return "nil"
					}
				}()),
				zap.String("fromAgentID", fromAgentID),
				zap.String("toAgent", targetConfig.Name),
				zap.String("toAgentID", targetConfig.ID.String()))

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
	// 先查询 invocation，确保数据库记录存在
	invocation, err := es.invocationRepo.FindByID(ctx, invocationID)
	if err != nil {
		return fmt.Errorf("failed to find invocation: %w", err)
	}
	if invocation == nil {
		return ErrAgentNotFound
	}

	// 检查是否已经是完成状态
	if invocation.Status == model.InvocationStatusCompleted ||
		invocation.Status == model.InvocationStatusFailed ||
		invocation.Status == model.InvocationStatusCancelled ||
		invocation.Status == model.InvocationStatusInterrupted {
		logInfo("CancelAgent: invocation already finished",
			zap.String("invocationID", invocationID.String()),
			zap.String("status", string(invocation.Status)))
		return nil // 已经完成，无需取消
	}

	// 1. 先更新数据库状态为 cancelled（必须在 kill 进程之前）
	// 这样当 CLI 被 kill 报错时，检查 cancelled 状态才能正确跳过
	invocation.Status = model.InvocationStatusCancelled
	invocation.CompletedAt = timePtr(time.Now())

	// 关键：保留已有的 sessionId（cli 缓存或 ACP），以便下一轮对话可以 resume 上下文
	if invocation.SessionID == "" && invocation.AgentConfigID != uuid.Nil {
		sessionKey := fmt.Sprintf("%s:%s", invocation.ThreadID.String(), invocation.AgentConfigID.String())
		es.csMu.RLock()
		if cached, ok := es.cliSessions[sessionKey]; ok && cached != "" {
			invocation.SessionID = cached
		}
		es.csMu.RUnlock()
	}

	if err := es.invocationRepo.Update(ctx, invocation); err != nil {
		logError("Failed to update invocation status", zap.Error(err))
		return fmt.Errorf("failed to update invocation status: %w", err)
	}
	logInfo("CancelAgent: invocation status updated to cancelled (before kill)",
		zap.String("invocationID", invocationID.String()),
		zap.String("sessionId", invocation.SessionID))

	// 2. 广播取消状态（让前端知道）
	es.broadcastStatus(invocation.ThreadID, invocation.ID, "cancelled", invocation.Role, "", invocation.AgentConfigID.String(), "")

	es.mu.Lock()
	agent, exists := es.runningAgents[invocationID]
	if exists {
		logInfo("CancelAgent: stopping agent",
			zap.String("invocationID", invocationID.String()),
			zap.Bool("hasCmd", agent.Cmd != nil),
			zap.Bool("hasAdapter", agent.Adapter != nil))

		// 3. 终止 CLI 进程
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

		// 4. 保存已累积的消息到数据库（刷新页面后不丢失）
		agent.ContentBlocksMu.Lock()
		contentBlocksForSave := agent.AccumulatedContentBlocks
		agent.ContentBlocksMu.Unlock()

		if len(contentBlocksForSave) > 0 {
			// 提取文本内容
			var outputBuilder strings.Builder
			for _, block := range contentBlocksForSave {
				if block.Type == "text" {
					outputBuilder.WriteString(block.Content)
				}
			}
			outputForSave := outputBuilder.String()

			// 获取 config 和 baseAgent
			config := agent.AgentConfig
			baseAgent := agent.BaseAgent

			if config != nil {
				// 使用新的 context 保存消息（因为当前 ctx 可能已被取消）
				saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
				msgForSave := es.saveAgentMessageWithReturn(saveCtx, invocation.ThreadID, invocationID, config, baseAgent, outputForSave, contentBlocksForSave)
				saveCancel()

				if msgForSave != nil {
					es.broadcastAgentMessage(invocation.ThreadID, invocationID, msgForSave, config.Name, string(config.Role))
					logInfo("CancelAgent: saved agent message to database",
						zap.String("invocationID", invocationID.String()),
						zap.Int("contentBlocksCount", len(contentBlocksForSave)))
				}
			}
		}

		// 5. 取消 Go goroutine
		agent.CancelFunc()
		delete(es.runningAgents, invocationID)
	} else {
		logInfo("CancelAgent: agent not in runningAgents map, database already updated",
			zap.String("invocationID", invocationID.String()))
	}
	es.mu.Unlock()

	// 注意：故意不清 cliSessions 缓存，让下一轮对话仍可 resume 之前的上下文
	// （之前会清掉，导致用户取消后再发消息丢失全部上下文）

	return nil
}

// broadcastQuestionReadyEvent 广播 question_ready 事件
// 当 AskUserQuestion 被拒绝时（stdin 已关闭），通知前端可以等待用户输入
// 用户响应后，前端会发送 user_message，isdp 使用 --resume 恢复会话
func (es *ExecutionService) broadcastQuestionReadyEvent(invocationID uuid.UUID, agent *RunningAgent, toolCallID string) {
	if es.wsHub == nil {
		return
	}

	// 获取 threadID
	threadID := agent.ThreadID

	// 构造事件 payload
	payload := map[string]interface{}{
		"invocationId": invocationID.String(),
		"toolCallId":   toolCallID,
		"status":       "question_ready",
		"message":      "AskUserQuestion was rejected due to stdin closed, waiting for user input",
	}

	// 查找对应的 question block 获取问题详情
	for _, block := range agent.AccumulatedContentBlocks {
		if block.Type == "question" && block.ToolID == toolCallID {
			payload["questions"] = block.Questions
			payload["toolName"] = block.ToolName
			break
		}
	}

	logInfo("broadcastQuestionReadyEvent called",
		zap.String("threadId", threadID.String()),
		zap.String("invocationId", invocationID.String()),
		zap.String("toolCallId", toolCallID))

	es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
		Type:      "question_ready",
		ThreadID:  threadID.String(),
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	})
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
		// failed 状态时包含详细错误信息
		if status == "failed" && input != "" {
			payload["errorDetails"] = input
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
	// 高频流式日志降级为 Debug
	logDebug("broadcastChunk called", zap.String("threadId", threadID.String()), zap.String("chunkType", string(chunk.Type)), zap.String("toolName", chunk.ToolName))

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
		shouldCancel := false
		cancelReason := ""
		now := time.Now().UnixMilli()

		// 上层屏蔽差异：在收到非 thinking 的 chunk 时，自动结束之前的 streaming thinking 块
		// 这是为了处理某些适配器（如 OpenCode ACP）不发送 Done 标记的情况
		autoClosedThinking := false
		if chunk.Type != ChunkTypeThinking && len(agent.AccumulatedContentBlocks) > 0 {
			for i := len(agent.AccumulatedContentBlocks) - 1; i >= 0; i-- {
				block := &agent.AccumulatedContentBlocks[i]
				if block.Type == "thinking" && block.Status == "streaming" {
					block.Status = "success"
					block.Done = true
					autoClosedThinking = true
					logInfo("Auto-closed streaming thinking block", zap.Int("blockIndex", i))
					break // 只结束最后一个 streaming thinking 块
				}
			}
		}

		// 如果自动关闭了 thinking 块，需要先广播一个 done 标记让前端感知
		if autoClosedThinking {
			agent.ContentBlocksMu.Unlock()
			es.mu.Unlock()
			// 广播一个空的 thinking chunk 带 done 标记
			doneChunk := Chunk{
				Type:    ChunkTypeThinking,
				Content: "",
				Done:    true,
			}
			es.broadcastChunk(threadID, invocationID, doneChunk, agentID, agentName)
			// 重新获取锁继续处理当前 chunk
			es.mu.Lock()
			agent, exists = es.runningAgents[invocationID]
			if !exists {
				es.mu.Unlock()
				return
			}
			agent.ContentBlocksMu.Lock()
		}

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
		case ChunkTypeInputJSONDelta:
			// 工具参数增量更新 - 累积 PartialJSON 并解析
			// ToolIndex 在每个 message/turn 中会重置（从 0 开始）
			// 因此需要找到 LAST block with matching ToolIndex（最新的那个）
			// 而不是第一个匹配的 block（可能来自之前的 turn）
			lastMatchingIdx := -1
			for i := len(agent.AccumulatedContentBlocks) - 1; i >= 0; i-- {
				b := agent.AccumulatedContentBlocks[i]
				if b.Type == "tool_use" && b.ToolIndex == chunk.ToolIndex {
					lastMatchingIdx = i
					break // 找到最新的匹配块，停止搜索
				}
			}
			if lastMatchingIdx >= 0 {
				b := agent.AccumulatedContentBlocks[lastMatchingIdx]
				// 累积 InputJSON
				accumulatedJSON := b.InputJSON + chunk.PartialJSON
				agent.AccumulatedContentBlocks[lastMatchingIdx].InputJSON = accumulatedJSON

				// 尝试解析累积的 JSON
				// PartialJSON 可能是不完整的，只有完整的 JSON 才能解析
				var input map[string]interface{}
				if err := json.Unmarshal([]byte(accumulatedJSON), &input); err == nil {
					// JSON 完整，更新 Input
					agent.AccumulatedContentBlocks[lastMatchingIdx].Input = input
					logInfo("ChunkTypeInputJSONDelta: JSON parsed successfully",
						zap.Int("toolIndex", chunk.ToolIndex),
						zap.String("toolId", b.ToolID),
						zap.Int("inputFields", len(input)))
				} else {
					// JSON 不完整，等待更多 delta
					logInfo("ChunkTypeInputJSONDelta: JSON incomplete, waiting for more",
						zap.Int("toolIndex", chunk.ToolIndex),
						zap.String("partialJSON", accumulatedJSON[:min(50, len(accumulatedJSON))]))
				}
			} else {
				logInfo("ChunkTypeInputJSONDelta: no matching tool_use block found",
					zap.Int("toolIndex", chunk.ToolIndex),
					zap.Int("blocksCount", len(agent.AccumulatedContentBlocks)))
			}
		case ChunkTypeToolUse:
			// 工具调用开始或 assistant message fallback 更新
			blockID := fmt.Sprintf("tool-%s", chunk.ToolID)
			existingIdx := -1
			for i, b := range agent.AccumulatedContentBlocks {
				if b.ID == blockID {
					existingIdx = i
					break
				}
			}
			if existingIdx >= 0 {
				// 已存在：assistant message fallback，更新 Input
				// input_json_delta 解析失败时，assistant 消息提供完整 Input
				if chunk.ToolInput != nil && len(chunk.ToolInput) > 0 {
					agent.AccumulatedContentBlocks[existingIdx].Input = chunk.ToolInput
					logInfo("ChunkTypeToolUse: updated Input via fallback", zap.String("toolID", chunk.ToolID))
				}
			} else {
				// 不存在：stream_event content_block_start 创建新块
				agent.AccumulatedContentBlocks = append(agent.AccumulatedContentBlocks, ContentBlockData{
					ID:        blockID,
					Type:      "tool_use",
					Timestamp: now,
					Status:    "streaming",
					ToolName:  chunk.ToolName,
					ToolID:    chunk.ToolID,
					ToolIndex: chunk.ToolIndex,
					Input:     chunk.ToolInput,
					StartedAt: now,
				})
			}
		case ChunkTypeToolResult:
			// 工具调用结果：更新对应的工具块
			// 特殊处理：AskUserQuestion 工具的 tool_result 表示"拒绝"
			// 因为 stdin 已关闭，CLI 无法等待用户输入，会返回拒绝响应
			// 正确做法：忽略拒绝错误，让 CLI 正常结束，保存 sessionId 用于 --resume
			isQuestionResult := agent.LastQuestionToolID != "" && chunk.ToolID == agent.LastQuestionToolID
			if isQuestionResult && chunk.IsError {
				// AskUserQuestion 被拒绝（stdin 已关闭）
				// 设置 Agent 状态，阻止模型继续循环调用工具
				agent.WaitingForUserInput = true
				agent.PendingQuestionID = chunk.ToolID

				// 更新 question 块状态并广播事件
				for i := len(agent.AccumulatedContentBlocks) - 1; i >= 0; i-- {
					if agent.AccumulatedContentBlocks[i].Type == "question" && agent.AccumulatedContentBlocks[i].ToolID == chunk.ToolID {
						// 保持 waiting_user_input 状态，用户响应后通过 --resume 恢复会话
						logInfo("AskUserQuestion tool_result 收到（拒绝），设置 WaitingForUserInput=true，广播 question_ready 事件",
							zap.String("invocationID", invocationID.String()),
							zap.String("toolCallID", chunk.ToolID),
							zap.String("content", chunk.Content))

						// 广播 question_ready 事件通知前端
						es.broadcastQuestionReadyEvent(invocationID, agent, chunk.ToolID)
						// 主动取消 CLI 进程，阻止模型继续调用工具
						// CLI 会被取消，ExecutionContext 的 error 处理会检测 WaitingForUserInput 状态
						// 并正确处理为"等待用户输入"而非"执行失败"（等待用户响应后 --resume）
						// 设置标志位，在释放锁后取消 CLI 进程（避免死锁）
						shouldCancel = true
						cancelReason = "AskUserQuestion rejected, waiting for user input"
						break
					}
				}
			} else {
				// 正常的 tool_result：更新对应的工具块
				for i := len(agent.AccumulatedContentBlocks) - 1; i >= 0; i-- {
					if agent.AccumulatedContentBlocks[i].Type == "tool_use" && agent.AccumulatedContentBlocks[i].ToolID == chunk.ToolID {
						agent.AccumulatedContentBlocks[i].Output = chunk.Content
						agent.AccumulatedContentBlocks[i].IsError = chunk.IsError
						agent.AccumulatedContentBlocks[i].Status = "success"
						agent.AccumulatedContentBlocks[i].CompletedAt = now
						break
					}
				}
			}
		case ChunkTypeQuestion:
			// AskUserQuestion 工具调用：需要用户输入
			// 检查是否已存在同 ID 的 question block（parser 可能发送两次）
			blockID := fmt.Sprintf("question-%s", chunk.ToolID)
			existingIdx := -1
			for i, b := range agent.AccumulatedContentBlocks {
				if b.ID == blockID {
					existingIdx = i
					break
				}
			}

			// 获取 Agent ID 和 Name（用于前端 @mention resume）
			agentIDStr := ""
			agentNameStr := ""
			if agent.AgentConfig != nil {
				agentIDStr = agent.AgentConfig.ID.String()
				agentNameStr = agent.AgentConfig.Name
			}

			if existingIdx >= 0 {
				// 已存在：更新它（特别是 Questions 字段，可能在第二次解析时才有）
				// 注意：保留已有的 AgentID 和 AgentName（第一次创建时已设置）
				agent.AccumulatedContentBlocks[existingIdx] = ContentBlockData{
					ID:           blockID,
					Type:         "question",
					Timestamp:    now,
					Status:       "waiting_user_input",
					ToolName:     chunk.ToolName,
					ToolID:       chunk.ToolID,
					Input:        chunk.ToolInput,
					Questions:    chunk.Questions,
					InvocationID: invocationID.String(),
					AgentID:      agent.AccumulatedContentBlocks[existingIdx].AgentID,   // 保留已有值
					AgentName:    agent.AccumulatedContentBlocks[existingIdx].AgentName, // 保留已有值
					StartedAt:    agent.AccumulatedContentBlocks[existingIdx].StartedAt,
				}
				logInfo("更新已存在的 question block",
					zap.String("blockId", blockID),
					zap.Int("questionsCount", len(chunk.Questions)),
					zap.String("agentName", agent.AccumulatedContentBlocks[existingIdx].AgentName))
			} else {
				// 不存在：添加新 block（包含 AgentID 和 AgentName）
				agent.AccumulatedContentBlocks = append(agent.AccumulatedContentBlocks, ContentBlockData{
					ID:           blockID,
					Type:         "question",
					Timestamp:    now,
					Status:       "waiting_user_input",
					ToolName:     chunk.ToolName,
					ToolID:       chunk.ToolID,
					Input:        chunk.ToolInput,
					Questions:    chunk.Questions,
					InvocationID: invocationID.String(),
					AgentID:      agentIDStr,   // 保存 Agent ID
					AgentName:    agentNameStr, // 保存 Agent Name（用于前端 @mention）
					StartedAt:    now,
				})
				logInfo("添加新的 question block",
					zap.String("blockId", blockID),
					zap.Int("questionsCount", len(chunk.Questions)),
					zap.String("agentId", agentIDStr),
					zap.String("agentName", agentNameStr))
			}
			// 标记为等待用户输入状态
			agent.WaitingForUserInput = true
			agent.PendingQuestionID = chunk.ToolID
			// 记录 AskUserQuestion 工具调用，用于后续判断 tool_result 是否是该工具的拒绝响应
			agent.LastQuestionToolID = chunk.ToolID

			// 创建待办任务（Agent 等待用户输入）
			// 仅在开关开启时自动创建
			if es.humanTaskEnabled && es.humanTaskSvc != nil {
				// 提取等待原因：从 Questions 中获取第一个问题的摘要
				waitReason := "Agent 等待您的输入"
				if len(chunk.Questions) > 0 && chunk.Questions[0].Question != "" {
					waitReason = chunk.Questions[0].Question
					if len(waitReason) > 100 {
						waitReason = waitReason[:100] + "..."
					}
				}
				_, err := es.humanTaskSvc.CreateTaskFromWaiting(
					context.Background(),
					agent.ThreadID,
					invocationID,
					agent.AgentConfig.ID,
					agentNameStr,
					waitReason,
				)
				if err != nil {
					logError("创建待办任务失败", zap.Error(err))
				} else {
					logInfo("待办任务已创建",
						zap.String("invocationID", invocationID.String()),
						zap.String("agentName", agentNameStr))
				}
			}
		case ChunkTypeError:
			// 错误提示（如 CLI 限流/重试）：作为独立内容块持久化，
			// 让历史消息也能回看，刷新页面不丢失
			if chunk.Content != "" {
				agent.AccumulatedContentBlocks = append(agent.AccumulatedContentBlocks, ContentBlockData{
					ID:        fmt.Sprintf("error-%d-%d", invocationID.ID(), now),
					Type:      "error",
					Content:   chunk.Content,
					Timestamp: now,
					Status:    "failed",
				})
			}
		}
		agent.ContentBlocksMu.Unlock()
		// 在释放锁后检查是否需要取消 CLI 进程（避免死锁）
		if shouldCancel && agent.CancelFunc != nil {
			logInfo("Canceling CLI process after releasing locks",
				zap.String("invocationID", invocationID.String()),
				zap.String("reason", cancelReason))
			agent.CancelFunc()
		}
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
		// 处理 Usage 类型的 Chunk - 注：前端已移除 Token 统计展示，不再推送 usage_update WebSocket 事件
		if chunk.Type == ChunkTypeUsage && chunk.Usage != nil {
			logInfo("broadcastChunk: usage chunk received",
				zap.String("threadId", threadID.String()),
				zap.Int64("inputTokens", chunk.Usage.InputTokens),
				zap.Int64("outputTokens", chunk.Usage.OutputTokens),
				zap.Int64("cacheReadTokens", chunk.Usage.CacheReadTokens),
				zap.Int64("contextUsed", chunk.Usage.ContextUsed),
				zap.Int64("contextSize", chunk.Usage.ContextSize),
				zap.Float64("costUsd", chunk.Usage.CostUsd))
			// 注：不再广播 usage_update WebSocket 事件，前端已移除 TOKEN 统计显示
			// es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			// 	Type:      "usage_update",
			// 	ThreadID:  threadID.String(),
			// 	Timestamp: time.Now().UnixMilli(),
			// 	Payload: map[string]interface{}{
			// 		"invocationId": invocationID.String(),
			// 		"usage": map[string]interface{}{
			// 			"inputTokens":         chunk.Usage.InputTokens,
			// 			"outputTokens":        chunk.Usage.OutputTokens,
			// 			"cacheReadTokens":     chunk.Usage.CacheReadTokens,
			// 			"cacheCreationTokens": chunk.Usage.CacheCreationTokens,
			// 			"costUsd":             chunk.Usage.CostUsd,
			// 			"durationMs":          chunk.Usage.DurationMs,
			// 			"durationApiMs":       chunk.Usage.DurationApiMs,
			// 			"numTurns":            chunk.Usage.NumTurns,
			// 			"contextUsed":         chunk.Usage.ContextUsed,
			// 			"contextSize":         chunk.Usage.ContextSize,
			// 		},
			// 	},
			// })
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
			payload["toolIndex"] = chunk.ToolIndex
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

		// input_json_delta：工具参数增量更新
		if chunk.Type == ChunkTypeInputJSONDelta {
			payload["toolIndex"] = chunk.ToolIndex
			payload["partialJSON"] = chunk.PartialJSON
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

// AskUserQuestionPendingError 表示 AskUserQuestion 工具需要用户输入
// 这是一个特殊的 error，表示 CLI 应该正常结束（而非失败），等待用户响应后通过 --resume 恢复
var ErrAskUserQuestionPending = fmt.Errorf("ask_user_question_pending: waiting for user input")

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

			// 关闭关联的待办任务（仅在开关开启时）
			if es.humanTaskEnabled && es.humanTaskSvc != nil {
				if err := es.humanTaskSvc.CompleteTaskFromReply(context.Background(), invocationID); err != nil {
					logError("关闭待办任务失败", zap.Error(err))
				} else {
					logInfo("待办任务已关闭", zap.String("invocationID", invocationID.String()))
				}
			}

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
				// 尝试通过 ToolResultSender 接口发送响应（统一接口）
				if sender, ok := agent.Adapter.(ToolResultSender); ok {
					err := sender.SendToolResult(invocationID, toolCallID, answer)
					if err != nil {
						logError("发送工具结果失败", zap.Error(err))
						return err
					}
				} else {
					logWarn("Adapter 不支持 ToolResultSender 接口", zap.String("adapterType", fmt.Sprintf("%T", agent.Adapter)))
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

// updatePreviousQuestionBlock 更新上一个 Agent 消息中已回答的 question block
// 当用户回答 AskUserQuestion 时，需要将上一个消息中的 question block 更新为 success 状态
func (es *ExecutionService) updatePreviousQuestionBlock(ctx context.Context, threadID uuid.UUID, userAnswer string) {
	if es.msgRepo == nil {
		return
	}

	// 查询最近的消息
	messages, err := es.msgRepo.FindByThreadID(ctx, threadID, 10)
	if err != nil || len(messages) == 0 {
		return
	}

	// 找到最后一个 agent 消息（包含 question block）
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == model.MessageRoleAgent && len(msg.ContentBlocks) > 0 {
			var blocks []ContentBlockData
			if json.Unmarshal(msg.ContentBlocks, &blocks) != nil {
				continue
			}

			// 查找 waiting_user_input 状态的 question block
			updated := false
			for j, block := range blocks {
				if block.Type == "question" && block.Status == "waiting_user_input" {
					blocks[j].Status = "success"
					blocks[j].Output = userAnswer
					blocks[j].CompletedAt = time.Now().UnixMilli()
					updated = true
					logInfo("updatePreviousQuestionBlock: 更新 question block",
						zap.String("messageId", msg.ID.String()),
						zap.String("blockId", block.ID),
						zap.String("status", "success"),
						zap.String("output", userAnswer))
				}
			}

			// 如果有更新，保存到数据库
			if updated {
				newBlocksJSON, _ := json.Marshal(blocks)
				msg.ContentBlocks = newBlocksJSON
				if err := es.msgRepo.Update(ctx, msg); err != nil {
					logError("updatePreviousQuestionBlock: 更新消息失败", zap.Error(err))
				} else {
					logInfo("updatePreviousQuestionBlock: 消息更新成功", zap.String("messageId", msg.ID.String()))
				}
			}
			break // 只处理最近的 agent 消息
		}
	}
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
func (es *ExecutionService) SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string, images []model.ImageContent) error {
	logInfo("SpawnAgentForUserMessage 被调用", zap.String("threadID", threadID.String()), zap.String("userMessage", userMessage), zap.Int("imagesCount", len(images)))

	// 获取Thread信息（先获取，用于确定 workflow 范围）
	thread, err := es.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to get thread: %w", err)
	}

	// 获取项目路径和工作流模板（提前获取，用于限定 @mention 解析范围）
	var projectPath string
	var project *model.Project
	if es.projectRepo != nil {
		project, err = es.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil {
			projectPath = project.LocalPath
		}
	}
	workflowTemplateID := selectThreadTeamWorkflowTemplateID(thread, project)

	// 获取 workflow 范围内的 agents，用于限定 @mention 解析
	var workflowAgents []*model.AgentRoleConfig
	if workflowTemplateID != nil && es.workflowRepo != nil {
		workflow, err := es.workflowRepo.FindByID(ctx, *workflowTemplateID)
		if err == nil && workflow != nil && len(workflow.AgentIDs) > 0 {
			var agentIDStrs []string
			if json.Unmarshal(workflow.AgentIDs, &agentIDStrs) == nil {
				for _, idStr := range agentIDStrs {
					agentUUID, err := uuid.Parse(idStr)
					if err != nil {
						continue
					}
					config, err := es.configSvc.GetByID(ctx, agentUUID)
					if err == nil && config != nil {
						workflowAgents = append(workflowAgents, config)
					}
				}
			}
			logInfo("SpawnAgentForUserMessage: 获取到 workflow agents",
				zap.Int("count", len(workflowAgents)),
				zap.Strings("agentNames", func() []string {
					names := make([]string, len(workflowAgents))
					for i, a := range workflowAgents {
						names[i] = a.Name
					}
					return names
				}()))
		}
	}

	// 解析用户消息中的 @mentions（限定在 workflow agents 范围内）
	var mentionedAgentIDs []string
	if es.mentionParser != nil {
		if len(workflowAgents) > 0 {
			// 使用 ParseForAgents 限定范围
			mentionedIDs, err := es.mentionParser.ParseForAgents(ctx, userMessage, "", workflowAgents)
			if err != nil {
				logError("SpawnAgentForUserMessage: 解析 @mentions 失败", zap.Error(err))
			} else {
				mentionedAgentIDs = mentionedIDs
				logInfo("SpawnAgentForUserMessage: 解析到 @mentions（限定 workflow 范围）", zap.Strings("agentIDs", mentionedAgentIDs))
			}
		} else {
			// 没有 workflow 时，使用全局解析（向后兼容）
			mentionedIDs, err := es.mentionParser.Parse(ctx, userMessage, "")
			if err != nil {
				logError("SpawnAgentForUserMessage: 解析 @mentions 失败", zap.Error(err))
			} else {
				mentionedAgentIDs = mentionedIDs
				logInfo("SpawnAgentForUserMessage: 解析到 @mentions（全局范围）", zap.Strings("agentIDs", mentionedAgentIDs))
			}
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

	logInfo("SpawnAgentForUserMessage: 工作流模板查找结果",
		zap.Any("workflowTemplateID", workflowTemplateID),
		zap.Any("threadWorkflowTemplateID", thread.WorkflowTemplateID))

	// 如果用户 @mention 了某个 Agent，优先调用那个 Agent（使用 resume 策略）
	// 这是对 AskUserQuestion 响应的关键支持
	var targetConfig *model.AgentRoleConfig
	var sessionStrategy SessionStrategy
	var sessionIdFromDB string

	if len(mentionedAgentIDs) > 0 {
		logInfo("SpawnAgentForUserMessage: 用户 @mention 了 Agent，优先调用被 @ 的 Agent", zap.Strings("mentionedAgentIDs", mentionedAgentIDs))

		// 找到被 @ 的 Agent 配置
		for _, mentionedID := range mentionedAgentIDs {
			configID, err := uuid.Parse(mentionedID)
			if err != nil {
				logError("SpawnAgentForUserMessage: 无法解析 @mention 的 Agent ID", zap.String("mentionedID", mentionedID), zap.Error(err))
				continue
			}

			config, err := es.configSvc.GetByID(ctx, configID)
			if err != nil {
				logError("SpawnAgentForUserMessage: 无法找到 @mention 的 Agent 配置", zap.String("mentionedID", mentionedID), zap.Error(err))
				continue
			}

			// 找到被 @ 的 Agent，检查是否应该使用 resume 策略
			targetConfig = config
			logInfo("SpawnAgentForUserMessage: 找到被 @ 的 Agent", zap.String("name", config.Name), zap.String("id", config.ID.String()))

			// 检查是否与最后一个完成的 Agent 相同（使用 resume）
			sessionStrategy, sessionIdFromDB = es.shouldUseResumeStrategy(ctx, threadID, config.ID, mentionedAgentIDs)
			if sessionStrategy == SessionStrategyResume {
				logInfo("SpawnAgentForUserMessage: 用户 @ 同一 Agent，使用 resume 会话策略",
					zap.String("agentName", config.Name),
					zap.String("agentID", config.ID.String()),
					zap.String("sessionID", sessionIdFromDB))
			}
			break
		}
	}

	// 如果没有 @mention 或没找到被 @ 的 Agent，使用 workflow 入口 Agent
	if targetConfig == nil {
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
				targetConfig = config
				logInfo("SpawnAgentForUserMessage: 使用入口 Agent", zap.String("name", config.Name), zap.String("id", config.ID.String()))

				// 没有 @mention，检查是否应该自动 resume
				sessionStrategy, sessionIdFromDB = es.shouldAutoResume(ctx, threadID, config.ID)
				if sessionStrategy == SessionStrategyResume {
					logInfo("SpawnAgentForUserMessage: 自动判断使用 resume 会话策略（无@mention）",
						zap.String("agentName", config.Name),
						zap.String("agentID", config.ID.String()))
				}
			}
		}
	}

	// 如果仍然没有找到 Agent，使用回退逻辑
	if targetConfig == nil {
		// 回退逻辑：获取任意一个可用的默认 Agent
		logDebug("No workflow agent found, using fallback selection")
		configs, listErr := es.configSvc.List(ctx)
		if listErr != nil || len(configs) == 0 {
			return fmt.Errorf("no agent config available: %w", listErr)
		}
		// 优先选择 is_default=true 的，否则选第一个
		targetConfig = configs[0]
		for _, c := range configs {
			if c.IsDefault {
				targetConfig = c
				break
			}
		}

		logInfo("SpawnAgentForUserMessage: 使用回退 Agent", zap.String("name", targetConfig.Name), zap.String("id", targetConfig.ID.String()))

		// 没有 @mention，检查是否应该自动 resume
		sessionStrategy, sessionIdFromDB = es.shouldAutoResume(ctx, threadID, targetConfig.ID)
		if sessionStrategy == SessionStrategyResume {
			logInfo("SpawnAgentForUserMessage: 自动判断使用 resume 会话策略（回退，无@mention）",
				zap.String("agentName", targetConfig.Name),
				zap.String("agentID", targetConfig.ID.String()))
		}
	}

	// 如果使用 resume 策略，先更新上一个消息中已回答的 question block
	if sessionStrategy == SessionStrategyResume {
		es.updatePreviousQuestionBlock(ctx, threadID, userMessage)
	}

	// 触发Agent
	_, err = es.SpawnAgent(ctx, &SpawnRequest{
		ThreadID:        threadID,
		Role:            targetConfig.Role,
		ConfigID:        targetConfig.ID,
		Input:           userMessage,
		Images:          images,
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

	// 检查最后一个完成的 Agent 是否与目标 Agent **完全相同**（同一个 Agent ID）
	// 只有同一个 Agent 才能 resume，跨 Agent 调用（即使同角色）应使用新会话
	// 取最近一条该 Agent 的记录（优先有 sessionId 的）
	var lastInvocation *model.AgentInvocation
	for _, inv := range lastCompleted {
		if inv.AgentConfigID != targetConfigID {
			continue
		}
		if lastInvocation == nil {
			lastInvocation = inv
		}
		if inv.SessionID != "" {
			lastInvocation = inv
			break
		}
	}
	if lastInvocation == nil {
		logInfo("shouldUseResumeStrategy: 没有匹配的最近 invocation",
			zap.String("targetConfigID", targetConfigID.String()))
		return "", ""
	}

	// 详细日志：打印两个 ID 的完整值进行比较
	logInfo("shouldUseResumeStrategy: ID对比详情",
		zap.String("targetConfigID", targetConfigID.String()),
		zap.String("lastCompletedConfigID", lastInvocation.AgentConfigID.String()),
		zap.Bool("相等", lastInvocation.AgentConfigID == targetConfigID),
		zap.String("lastCompletedName", lastInvocation.AgentName),
		zap.String("lastSessionId", lastInvocation.SessionID),
		zap.String("lastStatus", string(lastInvocation.Status)))

	// 如果找到了匹配的 invocation 但 session ID 为空，说明上次取消时 session ID 未持久化
	// 此时不能返回 Resume，必须降级为 New 否则 agent 将丢失上下文重新开始
	if lastInvocation.SessionID == "" {
		logWarn("shouldUseResumeStrategy: 找到匹配的 invocation 但 sessionId 为空，降级为 new session",
			zap.String("targetConfigID", targetConfigID.String()),
			zap.String("lastCompletedName", lastInvocation.AgentName),
			zap.String("lastStatus", string(lastInvocation.Status)))
		return "", ""
	}

	logInfo("shouldUseResumeStrategy: 目标 Agent 与最后一个完成的 Agent 相同，使用 resume",
		zap.String("targetConfigID", targetConfigID.String()),
		zap.String("lastCompletedConfigID", lastInvocation.AgentConfigID.String()),
		zap.String("lastCompletedRole", string(lastInvocation.Role)))
	return SessionStrategyResume, lastInvocation.SessionID
}

// saveSessionIDEarly 在 adapter 拿到 session ID 后立即持久化
// 不等进程退出，确保取消/崩溃后仍可 resume
func (es *ExecutionService) saveSessionIDEarly(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, baseAgent *model.BaseAgent, invocation *model.AgentInvocation, sessionID string) {
	if sessionID == "" {
		return
	}

	// 更新 invocation 的 SessionID
	invocation.SessionID = sessionID
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := es.invocationRepo.Update(saveCtx, invocation); err != nil {
		logError("saveSessionIDEarly: failed to update invocation sessionId", zap.Error(err), zap.String("invocationID", invocation.ID.String()), zap.String("sessionId", sessionID))
		return
	}
	logInfo("saveSessionIDEarly: persisted sessionId", zap.String("invocationID", invocation.ID.String()), zap.String("sessionId", sessionID), zap.String("agentName", config.Name))

	// 同时保存到 ACP session_records 表
	if es.sessionManager != nil {
		if err := es.sessionManager.SaveACPSessionID(saveCtx, threadID.String(), config.ID.String(), sessionID, baseAgent.Type); err != nil {
			logError("saveSessionIDEarly: failed to save ACP session ID", zap.Error(err))
		}
	}
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

	// 只有同一个 Agent ID 才能 resume
	// 跨 Agent 调用（即使同角色）应使用新会话
	// 取最近一条该 Agent 的记录（优先有 sessionId 的，避免取消后 sessionId 为空的记录覆盖可 resume 的旧记录）
	var lastInvocation *model.AgentInvocation
	for _, inv := range lastCompleted {
		if inv.AgentConfigID != targetConfigID {
			continue
		}
		if lastInvocation == nil {
			lastInvocation = inv
		}
		if inv.SessionID != "" {
			lastInvocation = inv
			break
		}
	}
	if lastInvocation == nil {
		logInfo("shouldAutoResume: 没有匹配的最近 invocation",
			zap.String("targetConfigID", targetConfigID.String()))
		return "", ""
	}

	// 详细日志：打印两个 ID 的完整值进行比较
	logInfo("shouldAutoResume: ID对比详情",
		zap.String("targetConfigID", targetConfigID.String()),
		zap.String("lastCompletedConfigID", lastInvocation.AgentConfigID.String()),
		zap.Bool("相等", lastInvocation.AgentConfigID == targetConfigID),
		zap.String("lastCompletedName", lastInvocation.AgentName),
		zap.String("lastSessionId", lastInvocation.SessionID),
		zap.String("lastStatus", string(lastInvocation.Status)))

	// 如果找到了匹配的 invocation 但 session ID 为空，说明上次取消时 session ID 未持久化
	// 此时不能返回 Resume，必须降级为 New 否则 agent 将丢失上下文重新开始
	if lastInvocation.SessionID == "" {
		logWarn("shouldAutoResume: 找到匹配的 invocation 但 sessionId 为空，降级为 new session",
			zap.String("targetConfigID", targetConfigID.String()),
			zap.String("lastCompletedName", lastInvocation.AgentName),
			zap.String("lastStatus", string(lastInvocation.Status)))
		return "", ""
	}

	logInfo("shouldAutoResume: 目标 Agent 与最后一个完成的 Agent 相同，自动使用 resume",
		zap.String("targetConfigID", targetConfigID.String()),
		zap.String("lastCompletedConfigID", lastInvocation.AgentConfigID.String()),
		zap.String("lastCompletedRole", string(lastInvocation.Role)))
	return SessionStrategyResume, lastInvocation.SessionID
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
	return BuildPrompt(layers, input)
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
		`file://[^\s]+`,  // file://xxx
		`path:\s*[^\s]+`, // path: xxx
		`\.\/[^\s]+`,     // ./xxx
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
	sb.WriteString("## 会话历史\n\n")

	// 限制处理的消息数量
	if len(messages) > maxMessages {
		messages = messages[:maxMessages]
	}

	// 逐条处理消息
	for _, msg := range messages {
		if msg.Role == model.MessageRoleUser {
			// 用户消息：完整保留
			sb.WriteString("**用户**: ")
			sb.WriteString(msg.Content)
			sb.WriteString("\n\n")
		} else if msg.Role == model.MessageRoleAgent {
			// Agent 消息：简化处理，删除 tool output，保留摘要
			sb.WriteString("**")
			// 尝试从 metadata 获取 agentName
			var agentName string
			if msg.Metadata != nil {
				var metadata map[string]interface{}
				if err := json.Unmarshal(msg.Metadata, &metadata); err == nil {
					if name, ok := metadata["agentName"].(string); ok && name != "" {
						agentName = name
					}
				}
			}
			if agentName == "" {
				agentName = msg.AgentID
			}
			sb.WriteString(agentName)
			sb.WriteString("**: ")

			// 从 ContentBlocks 中提取简化内容
			if len(msg.ContentBlocks) > 0 {
				var blocks []ContentBlockData
				if err := json.Unmarshal(msg.ContentBlocks, &blocks); err == nil {
					sb.WriteString(es.extractSimplifiedAgentBlocks(blocks))
				} else {
					// 解析失败，使用过滤后的原始 content
					sb.WriteString(es.filterAndTruncateContent(msg.Content))
				}
			} else {
				// 没有 ContentBlocks，使用过滤后的原始 content
				sb.WriteString(es.filterAndTruncateContent(msg.Content))
			}
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// extractSimplifiedAgentBlocks 从 Agent 的 ContentBlocks 中提取简化内容
// 删除 tool_result 的 output，简化 text/thinking 内容
func (es *ExecutionService) extractSimplifiedAgentBlocks(blocks []ContentBlockData) string {
	var parts []string
	toolCalls := make(map[string]string) // toolID -> toolName，用于关联 tool_result

	for _, block := range blocks {
		switch block.Type {
		case "text":
			// text 内容：截断过长的内容
			content := es.filterThinkingContent(block.Content)
			if content != "" && content != "(无实质性内容)" {
				// 截断过长的 text 内容，保留前 500 字符
				if len(content) > 500 {
					content = content[:500] + "...(已省略)"
				}
				parts = append(parts, content)
			}
		case "thinking":
			// thinking 内容：聚合展示，标记为思考过程
			content := strings.TrimSpace(block.Content)
			if content != "" {
				// 截断过长的 thinking 内容
				if len(content) > 300 {
					content = content[:300] + "...(思考内容已省略)"
				}
				parts = append(parts, "[思考] "+content)
			}
		case "tool_use":
			// tool_use：只记录工具名，不保留完整 input
			toolCalls[block.ToolID] = block.ToolName
			parts = append(parts, fmt.Sprintf("[调用工具: %s]", block.ToolName))
		case "tool_result":
			// tool_result：删除 output 内容，只记录工具名和状态
			toolName := toolCalls[block.ToolID]
			if toolName == "" {
				toolName = "未知工具"
			}
			status := "完成"
			if block.IsError {
				status = "失败"
			}
			// 不保留 output 内容
			parts = append(parts, fmt.Sprintf("[工具结果: %s - %s]", toolName, status))
		}
	}

	if len(parts) == 0 {
		return "(无可用内容)"
	}

	return strings.Join(parts, "\n")
}

// filterAndTruncateContent 过滤 thinking 内容并截断过长内容
func (es *ExecutionService) filterAndTruncateContent(content string) string {
	filtered := es.filterThinkingContent(content)
	if filtered == "" || filtered == "(无实质性内容)" {
		return "(无实质性内容)"
	}
	// 截断过长的内容
	if len(filtered) > 500 {
		return filtered[:500] + "...(已省略)"
	}
	return filtered
}

// filterThinkingContent 过滤掉 thinking 内容（<thinking>...</thinking> 标签）
func (es *ExecutionService) filterThinkingContent(content string) string {
	// 匹配 <thinking>...</thinking> 标签（可能跨多行）
	thinkingRegex := regexp.MustCompile(`<thinking>[\s\S]*?</thinking>`)
	filtered := thinkingRegex.ReplaceAllString(content, "")

	// 也过滤可能的 markdown 格式的思考块
	// 例如：**思考过程** 或 ## Thinking 等开头的段落
	thinkingPatterns := []string{
		`(?s)\*\*思考过程\*\*.*?\n\n`,
		`(?s)## Thinking.*?\n\n`,
		`(?s)## 思考.*?\n\n`,
	}
	for _, pattern := range thinkingPatterns {
		re := regexp.MustCompile(pattern)
		filtered = re.ReplaceAllString(filtered, "")
	}

	// 清理多余的空白
	filtered = strings.TrimSpace(filtered)
	if filtered == "" {
		return "(无实质性内容)"
	}
	return filtered
}

// GetAllRunningAgents 获取所有运行中的Agent信息
func (es *ExecutionService) GetAllRunningAgents(ctx context.Context) ([]RunningAgentInfo, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	var result []RunningAgentInfo
	for _, agent := range es.runningAgents {
		// 通过ThreadContext缓存获取Project和Thread信息
		threadCtx, exists := es.threadContexts[agent.ThreadID]
		projectName := ""
		threadTitle := ""
		if exists && threadCtx != nil {
			if threadCtx.Project != nil {
				projectName = threadCtx.Project.Name
			}
			if threadCtx.Thread != nil {
				threadTitle = threadCtx.Thread.Name
			}
		}

		result = append(result, RunningAgentInfo{
			InvocationID:           agent.InvocationID,
			AgentName:              agent.AgentConfig.Name,
			ProjectName:            projectName,
			ThreadTitle:            threadTitle,
			StartedAt:              agent.StartedAt,
			RunningDurationSeconds: int(time.Since(agent.StartedAt).Seconds()),
		})
	}
	return result, nil
}
