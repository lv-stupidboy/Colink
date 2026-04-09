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
	Thread             *model.Thread
	Project            *model.Project
	WorkflowTemplate   *model.WorkflowTemplate
	WorkflowAgentIDs   []string
	Transitions        []model.Transition
	AllowedAgents      []*model.AgentRoleConfig
	LoadedAt           time.Time
}

// ExecutionService 统一执行服务，整合Orchestrator和InteractiveSession的功能
type ExecutionService struct {
	invocationRepo   *repo.AgentInvocationRepository
	threadRepo       *repo.ThreadRepository
	msgRepo          *repo.MessageRepository
	configSvc        *ConfigService
	baseAgentSvc     *BaseAgentService
	baseAgentRepo    *repo.BaseAgentRepository // 直接访问repo获取完整BaseAgent（含ApiToken）
	tracker          *InvocationTracker
	workflow         *WorkflowEngine
	workflowRepo     *repo.WorkflowTemplateRepository
	projectRepo      *repo.ProjectRepository
	wsHub            *ws.Hub
	defaultAdapter   AgentAdapter

	// Mention 解析器（支持动态 patterns）
	mentionParser *mention.Parser

	// 后台执行支持：内容块持久化
	contentBlockRepo    *repo.ContentBlockRepository
	contentBlockBuffer  []model.InvocationContentBlock // 缓冲区
	lastFlush           time.Time                      // 上次刷新时间
	contentBlockFlushMu sync.Mutex                     // 保护缓冲区

	runningAgents  map[uuid.UUID]*RunningAgent
	mu             sync.RWMutex

	// A2A 上下文追踪
	a2aContexts    map[uuid.UUID]*A2AContext // threadID -> A2AContext
	a2aMu          sync.RWMutex

	// Thread 上下文缓存（避免重复查询）
	threadContexts map[uuid.UUID]*ThreadContext
	tcMu           sync.RWMutex

	// CLI 会话ID缓存（用于 --resume 复用会话，避免冷启动延迟）
	// key: "threadID:agentID" -> value: sessionID
	cliSessions map[string]string
	csMu        sync.RWMutex
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
	}

	// 启动后台清理 goroutine，定期清理超时的 Agent
	go es.cleanupStaleAgents()

	return es
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

	es.broadcastStatus(invocation.ThreadID, invocation.ID, "failed", invocation.Role, "", "")
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

	// 创建调用记录
	invocationCreateStart := time.Now()
	invocation := &model.AgentInvocation{
		ID:            uuid.New(),
		ThreadID:      req.ThreadID,
		AgentConfigID: config.ID,
		Role:          config.Role,
		AgentName:     config.Name, // 存储 Agent 名称，用于历史显示
		Status:        model.InvocationStatusPending,
		Input:         req.Input,
		CreatedAt:     time.Now(),
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
		ActiveToolCount: 0,          // 初始化工具计数
	}
	es.mu.Unlock()

	// 广播状态更新
	es.broadcastStatus(req.ThreadID, invocation.ID, "started", config.Role, config.Name, req.Input)

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
	es.broadcastStatus(req.ThreadID, invocation.ID, "running", config.Role, config.Name, "")

	// 构建上下文
	contextStart := time.Now()
	contextLayers, err := es.buildContextLayers(ctx, req.ThreadID, config)
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

	// 获取已有的会话ID（用于 --resume 复用会话）
	sessionKey := fmt.Sprintf("%s:%s", req.ThreadID.String(), config.ID.String())
	es.csMu.RLock()
	sessionID := es.cliSessions[sessionKey]
	es.csMu.RUnlock()

	execReq := &ExecutionRequest{
		Config:    config,
		BaseAgent: baseAgent,
		Context:   contextLayers,
		Input:     req.Input,
		WorkDir:   req.ProjectPath,
		ConfigDir: config.ConfigPath, // 使用生成的配置目录
		SessionID: sessionID,          // 传递会话ID以支持 --resume
	}
	logInfo("[PERF] buildExecutionRequest", zap.Duration("duration", time.Since(execReqBuildStart)), zap.String("sessionID", sessionID), zap.Bool("isResume", sessionID != ""))

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

	cliDuration := time.Since(cliStart)
	logInfo("[PERF] CLI execution completed", zap.Duration("duration", cliDuration), zap.String("invocationID", invocation.ID.String()))

	if err != nil {
		logError("Adapter.ExecuteWithStream failed", zap.Error(err))
		es.handleAgentError(ctx, invocation, fmt.Errorf("adapter execution failed: %w", err))
		return
	}

	// 保存会话ID供后续复用（避免冷启动延迟）
	if result != nil && result.SessionID != "" {
		es.csMu.Lock()
		es.cliSessions[sessionKey] = result.SessionID
		es.csMu.Unlock()
		logInfo("Session ID saved for future resume", zap.String("sessionKey", sessionKey), zap.String("sessionId", result.SessionID))
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
	es.broadcastStatus(req.ThreadID, invocation.ID, "completed", config.Role, config.Name, "")

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
		ThreadID:     threadID,
		Role:         model.MessageRoleAgent,
		AgentID:      config.ID.String(),
		Content:      output,
		ContentBlocks: contentBlocksJSON,
		MessageType:  model.MessageTypeText,
		Metadata:     metadata,
		CreatedAt:    time.Now(),
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
		ThreadID:     threadID,
		Role:         model.MessageRoleAgent,
		AgentID:      config.ID.String(),
		Content:      output,
		ContentBlocks: contentBlocksJSON,
		MessageType:  model.MessageTypeText,
		Metadata:     metadata,
		CreatedAt:    time.Now(),
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

	es.broadcastStatus(invocation.ThreadID, invocation.ID, "failed", invocation.Role, "", "")
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
func (es *ExecutionService) buildContextLayers(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig) (*ContextLayers, error) {
	layers := &ContextLayers{}

	// 预加载上下文（一次性获取所有数据）
	tc, err := es.getThreadContext(ctx, threadID)
	if err != nil {
		return nil, err
	}

	// Layer 0: 系统提示（使用缓存的上下文）
	layers.Layer0 = es.buildDynamicSystemPromptFromContext(tc, config)

	// Layer 1: Thread历史
	messages, err := es.msgRepo.FindByThreadID(ctx, threadID, 100)
	if err != nil {
		return nil, err
	}
	layers.Layer1 = es.formatMessages(messages)

	// Layer 2: 工作产物（使用缓存的 Thread）
	layers.Layer2 = es.getArtifacts(tc.Thread)

	// Layer 3: 环境信息（使用缓存的 Thread）
	layers.Layer3 = es.getEnvironmentInfo(tc.Thread)

	return layers, nil
}

// roleTriggerHints 根据角色自动生成的触发提示
// key 对应数据库中 agent_configs.role 字段的值
var roleTriggerHints = map[string]string{
	"requirement_analyst":  "@需求分析师 当需要需求分析时",
	"architect":            "@架构师 当需要架构设计时",
	"frontend_developer":   "@前端开发工程师 当需要前端实现时",
	"backend_developer":    "@后端开发工程师 当需要后端实现时",
	"code_reviewer":        "@代码审查工程师 当需要代码审查时",
	"test_engineer":        "@测试工程师 当需要测试时",
	"sre_engineer":         "@运维工程师 当需要部署运维时",
	"project_manager":      "@项目经理 当需要项目协调时",
	"ui_designer":          "@UI设计师 当需要界面设计时",
	"database_designer":    "@数据库设计师 当需要数据库设计时",
	"security_engineer":    "@安全工程师 当需要安全审计时",
	"tech_writer":          "@技术文档工程师 当需要文档编写时",
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
			ID:        currentConfig.ID,
			Name:      currentConfig.Name,
			Role:      string(currentConfig.Role),
		}
		es.a2aMu.Unlock()

		logInfo("A2A 路由触发",
			zap.String("fromAgent", currentConfig.Name),
			zap.String("toAgent", targetConfig.Name),
			zap.Int("depth", a2aCtx.Depth),
			zap.String("threadId", threadID.String()))

		// 构建 A2A 输入（原始用户消息 + 前序响应上下文 + 触发者信息）
		a2aInput := es.buildA2AInput(ctx, threadID, currentConfig, a2aCtx, output)

		// 使用构建的 A2A 输入
		es.SpawnAgent(ctx, &SpawnRequest{
			ThreadID:    threadID,
			ConfigID:    targetConfig.ID,
			Role:        targetConfig.Role,
			Input:       a2aInput,
			ProjectPath: projectPath,
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
func (es *ExecutionService) buildA2AInput(ctx context.Context, threadID uuid.UUID, fromAgent *model.AgentRoleConfig, a2aCtx *A2AContext, output string) string {
	var sb strings.Builder

	// 1. 协作规则（有触发者信息时注入）
	if a2aCtx != nil && a2aCtx.FromAgent != nil {
		sb.WriteString("## 协作规则\n\n")
		sb.WriteString("A2A 出口检查：回复前问\"到我这里结束了吗？\"不是 → 谁需要动 → 末尾另起一行行首写 @句柄。\n\n")
		sb.WriteString("---\n\n")
	}

	// 2. 原始请求
	originalMessage := es.getLastUserMessage(ctx, threadID)
	if originalMessage != "" {
		sb.WriteString("## 原始请求\n\n")
		sb.WriteString(originalMessage)
		sb.WriteString("\n\n---\n\n")
	}

	// 3. 前序分析（包含元信息）
	if fromAgent != nil {
		sb.WriteString("## 前序分析\n\n")
		sb.WriteString(fmt.Sprintf("**来自**: %s\n", fromAgent.Name))
		if fromAgent.Role != "" {
			sb.WriteString(fmt.Sprintf("**角色**: %s\n", es.getRoleDescription(fromAgent.Role)))
		}
		// 擅长领域（使用 Description）
		strengths := fromAgent.Description
		if strengths != "" {
			sb.WriteString(fmt.Sprintf("**擅长**: %s\n", strengths))
		}
		sb.WriteString("\n")

		// 前序响应内容（移除纯 @mention 行）
		strippedOutput := es.stripPureMentionLines(output)
		sb.WriteString(strippedOutput)
		sb.WriteString("\n\n---\n\n")
	}

	// 4. 触发者信息
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
			ID:        config.ID,
			Name:      config.Name,
			Role:      string(config.Role),
			}
		es.a2aMu.Unlock()

		logInfo("A2A 触发执行",
			zap.String("fromAgent", config.Name),
			zap.String("toAgent", targetConfig.Name),
			zap.Int("depth", a2aCtx.Depth),
			zap.String("threadId", threadID.String()))

		// 触发下一个 Agent
		es.SpawnAgent(ctx, &SpawnRequest{
			ThreadID:    threadID,
			ConfigID:    targetConfig.ID,
			Role:        targetConfig.Role,
			Input:       output,
			ProjectPath: projectPath,
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
	es.broadcastStatus(invocation.ThreadID, invocation.ID, "cancelled", invocation.Role, "", "")

	return nil
}

// broadcastStatus 广播状态
func (es *ExecutionService) broadcastStatus(threadID, invocationID uuid.UUID, status string, role model.AgentRole, agentName string, input string) {
	logInfo("broadcastStatus called", zap.String("threadId", threadID.String()), zap.String("invocationId", invocationID.String()), zap.String("status", status))
	if es.wsHub != nil {
		payload := map[string]interface{}{
			"invocationId": invocationID.String(),
			"status":       status,
			"role":         string(role),
			"agentName":    agentName,
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
		}
		agent.ContentBlocksMu.Unlock()

		// 增量持久化：将内容块写入数据库（后台执行支持）
		if es.contentBlockRepo != nil && len(agent.AccumulatedContentBlocks) > 0 {
			lastBlock := agent.AccumulatedContentBlocks[len(agent.AccumulatedContentBlocks)-1]
			// 转换为持久化模型
			persistBlock := model.InvocationContentBlock{
				ID:          lastBlock.ID,
				InvocationID: invocationID.String(),
				Type:        lastBlock.Type,
				Content:     lastBlock.Content,
				Timestamp:   lastBlock.Timestamp,
				Status:      lastBlock.Status,
				ToolName:    lastBlock.ToolName,
				ToolID:      lastBlock.ToolID,
				Input:       lastBlock.Input,
				Output:      lastBlock.Output,
				IsError:     lastBlock.IsError,
				StartedAt:   lastBlock.StartedAt,
				CompletedAt: lastBlock.CompletedAt,
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

		es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_output_chunk",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload:   payload,
		})
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
func (es *ExecutionService) SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string) error {
	logInfo("SpawnAgentForUserMessage 被调用", zap.String("threadID", threadID.String()), zap.String("userMessage", userMessage))

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
			// 使用工作流模板中指定的Agent
			_, err = es.SpawnAgent(ctx, &SpawnRequest{
				ThreadID:    threadID,
				Role:        config.Role,
				ConfigID:    config.ID,
				Input:       userMessage,
				ProjectPath: projectPath,
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

	// 触发Agent
	_, err = es.SpawnAgent(ctx, &SpawnRequest{
		ThreadID:    threadID,
		Role:        config.Role,
		ConfigID:    config.ID,
		Input:       userMessage,
		ProjectPath: projectPath,
	})
	return err
}

// timePtr 返回时间的指针
func timePtr(t time.Time) *time.Time {
	return &t
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

	// 用户输入
	sb.WriteString("<user>\n")
	sb.WriteString(input)
	sb.WriteString("\n</user>\n")

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