package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MaxA2ADepth A2A 最大深度限制
const MaxA2ADepth = 15

// A2AContext A2A 上下文，用于追踪深度和去重
type A2AContext struct {
	Depth           int                // 当前深度
	InvokedAgents   map[uuid.UUID]bool // 已调用的 Agent ID 集合
	CompletedAgents map[uuid.UUID]bool // 已完成的 Agent ID 集合（用于汇聚判断）
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

	runningAgents  map[uuid.UUID]*RunningAgent
	mu             sync.RWMutex

	// A2A 上下文追踪
	a2aContexts    map[uuid.UUID]*A2AContext // threadID -> A2AContext
	a2aMu          sync.RWMutex
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
) *ExecutionService {
	es := &ExecutionService{
		invocationRepo:   invocationRepo,
		threadRepo:       threadRepo,
		msgRepo:          msgRepo,
		configSvc:        configSvc,
		baseAgentSvc:     baseAgentSvc,
		baseAgentRepo:    baseAgentRepo,
		tracker:          tracker,
		workflow:         workflow,
		workflowRepo:     workflowRepo,
		projectRepo:      projectRepo,
		wsHub:            wsHub,
		defaultAdapter:   defaultAdapter,
		runningAgents:    make(map[uuid.UUID]*RunningAgent),
		a2aContexts:      make(map[uuid.UUID]*A2AContext),
	}

	return es
}

// SpawnAgent 启动Agent（统一执行入口）
func (es *ExecutionService) SpawnAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
	// 解析配置和BaseAgent
	config, baseAgent, err := es.resolveConfigAndBaseAgent(ctx, req)
	if err != nil {
		return nil, err
	}

	// 创建调用记录
	invocation := &model.AgentInvocation{
		ID:            uuid.New(),
		ThreadID:      req.ThreadID,
		AgentConfigID: config.ID,
		Role:          config.Role,
		Status:        model.InvocationStatusPending,
		Input:         req.Input,
		CreatedAt:     time.Now(),
	}

	if err := es.invocationRepo.Create(ctx, invocation); err != nil {
		return nil, fmt.Errorf("failed to create invocation: %w", err)
	}

	// 创建上下文 - 使用独立的context，不受HTTP请求生命周期影响
	agentCtx, cancel := context.WithCancel(context.Background())

	// 记录运行中的Agent
	es.mu.Lock()
	es.runningAgents[invocation.ID] = &RunningAgent{
		InvocationID: invocation.ID,
		ThreadID:     req.ThreadID,
		AgentConfig:  config,
		BaseAgent:    baseAgent,
		StartedAt:    time.Now(),
		CancelFunc:   cancel,
	}
	es.mu.Unlock()

	// 广播状态更新
	es.broadcastStatus(req.ThreadID, invocation.ID, "started", config.Role)

	// 异步执行Agent
	go es.executeAgent(agentCtx, invocation, config, baseAgent, req)

	return invocation, nil
}

// executeAgent 执行Agent（统一执行路径）
func (es *ExecutionService) executeAgent(ctx context.Context, invocation *model.AgentInvocation, config *model.AgentRoleConfig, baseAgent *model.BaseAgent, req *SpawnRequest) {
	defer func() {
		// 恢复可能的panic
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in executeAgent: %v", r)
			es.handleAgentError(ctx, invocation, err)
		}
		es.mu.Lock()
		delete(es.runningAgents, invocation.ID)
		es.mu.Unlock()
	}()

	logInfo("executeAgent started", zap.String("invocationID", invocation.ID.String()))

	// 构建上下文
	contextLayers, err := es.buildContextLayers(ctx, req.ThreadID, config)
	if err != nil {
		logError("buildContextLayers failed", zap.Error(err))
		es.handleAgentError(ctx, invocation, fmt.Errorf("failed to build context layers: %w", err))
		return
	}
	logDebug("Context layers built")

	// 获取适配器
	adapter, err := es.getAdapter(ctx, config, baseAgent)
	if err != nil {
		logError("getAdapter failed", zap.Error(err))
		es.handleAgentError(ctx, invocation, fmt.Errorf("failed to get adapter: %w", err))
		return
	}
	logDebug("Adapter obtained", zap.String("adapterType", fmt.Sprintf("%T", adapter)))

	// 构建ExecutionRequest
	execReq := &ExecutionRequest{
		Config:    config,
		BaseAgent: baseAgent,
		Context:   contextLayers,
		Input:     req.Input,
		WorkDir:   req.ProjectPath,
	}

	// 使用流式执行，实时广播输出
	var outputBuilder strings.Builder
	_, err = adapter.ExecuteWithStream(ctx, execReq, func(chunk Chunk) {
		outputBuilder.WriteString(chunk.Content)
		// 实时广播输出块
		es.broadcastChunk(req.ThreadID, invocation.ID, chunk, config.ID.String(), config.Name)
	})

	if err != nil {
		logError("Adapter.ExecuteWithStream failed", zap.Error(err))
		es.handleAgentError(ctx, invocation, fmt.Errorf("adapter execution failed: %w", err))
		return
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

	// 保存输出消息到数据库
	es.saveAgentMessage(ctx, req.ThreadID, config, output)

	// 广播完成状态
	es.broadcastStatus(req.ThreadID, invocation.ID, "completed", config.Role)

	// 检查是否需要路由到下一个Agent
	es.checkRouting(ctx, req.ThreadID, config, output)
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

// getAdapter 获取适配器
func (es *ExecutionService) getAdapter(ctx context.Context, config *model.AgentRoleConfig, baseAgent *model.BaseAgent) (AgentAdapter, error) {
	// 如果有 BaseAgent，使用它创建适配器
	if baseAgent != nil {
		adapter := NewAdapter(baseAgent)
		if adapter == nil {
			return nil, fmt.Errorf("unsupported base agent type: %s", baseAgent.Type)
		}
		return adapter, nil
	}

	// 如果配置了BaseAgentID但没有传入baseAgent，尝试获取
	if config.BaseAgentID != uuid.Nil && es.baseAgentSvc != nil {
		ba, err := es.baseAgentSvc.GetByID(ctx, config.BaseAgentID)
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

	return nil, errors.New("no adapter available")
}

// handleAgentError 处理Agent错误
func (es *ExecutionService) handleAgentError(ctx context.Context, invocation *model.AgentInvocation, err error) {
	invocation.Status = model.InvocationStatusFailed
	invocation.Output = err.Error()
	invocation.CompletedAt = timePtr(time.Now())
	if updateErr := es.invocationRepo.Update(ctx, invocation); updateErr != nil {
		logError("Failed to update invocation on error", zap.Error(updateErr))
	}

	es.broadcastStatus(invocation.ThreadID, invocation.ID, "failed", invocation.Role)
}

// buildContextLayers 构建上下文层
func (es *ExecutionService) buildContextLayers(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig) (*ContextLayers, error) {
	layers := &ContextLayers{}

	// Layer 0: 系统提示（动态注入工作流触发点和出口检查）
	layers.Layer0 = es.buildDynamicSystemPrompt(ctx, threadID, config)

	// Layer 1: Thread历史
	messages, err := es.msgRepo.FindByThreadID(ctx, threadID, 100)
	if err != nil {
		return nil, err
	}
	layers.Layer1 = es.formatMessages(messages)

	// Layer 2: 工作产物
	thread, err := es.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return nil, err
	}
	layers.Layer2 = es.getArtifacts(thread)

	// Layer 3: 环境信息
	layers.Layer3 = es.getEnvironmentInfo(thread)

	return layers, nil
}

// buildDynamicSystemPrompt 构建动态系统提示，注入工作流触发点和出口检查
func (es *ExecutionService) buildDynamicSystemPrompt(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig) string {
	var sb strings.Builder

	// 原始系统提示
	sb.WriteString(config.SystemPrompt)

	// 获取当前 Agent 的转换规则
	transitions := es.getTransitionsForAgent(ctx, threadID, config.ID)

	// 获取工作流中的所有 Agent，用于将 ID 转换为名称
	allowedAgents := es.getAllowedAgentsFromWorkflow(ctx, threadID)
	agentNameMap := make(map[string]string)
	for _, agent := range allowedAgents {
		agentNameMap[agent.ID.String()] = agent.Name
	}

	// 注入工作流触发点提示
	if len(transitions) > 0 {
		sb.WriteString("\n\n## 工作流（主动 @ 触发点）\n")
		for _, t := range transitions {
			// 将 Agent ID 转换为名称
			agentName := t.ToAgentID
			if name, ok := agentNameMap[t.ToAgentID]; ok {
				agentName = name
			}
			sb.WriteString(fmt.Sprintf("- %s → @%s\n", t.Trigger, agentName))
		}
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
	logInfo("getTransitionsForAgent: starting", zap.String("threadID", threadID.String()), zap.String("agentConfigID", agentConfigID.String()))

	// 获取 Thread
	thread, err := es.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		logError("getTransitionsForAgent: failed to find thread", zap.Error(err))
		return nil
	}

	// 优先使用 Project 的工作流模板，如果没有则使用 Thread 的
	var workflowTemplateID *uuid.UUID
	if es.projectRepo != nil {
		project, err := es.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil && project.WorkflowTemplateID != nil {
			workflowTemplateID = project.WorkflowTemplateID
			logInfo("getTransitionsForAgent: using project's workflow template", zap.String("workflowTemplateID", workflowTemplateID.String()))
		}
	}
	if workflowTemplateID == nil && thread.WorkflowTemplateID != nil {
		workflowTemplateID = thread.WorkflowTemplateID
		logInfo("getTransitionsForAgent: using thread's workflow template", zap.String("workflowTemplateID", workflowTemplateID.String()))
	}

	if workflowTemplateID == nil {
		logInfo("getTransitionsForAgent: no workflow template found", zap.String("threadID", threadID.String()))
		return nil
	}

	// 获取工作流模板
	workflow, err := es.workflowRepo.FindByID(ctx, *workflowTemplateID)
	if err != nil {
		logError("getTransitionsForAgent: failed to find workflow template", zap.Error(err))
		return nil
	}
	if workflow == nil {
		logInfo("getTransitionsForAgent: workflow is nil")
		return nil
	}

	// 解析 Transitions JSON
	if len(workflow.Transitions) == 0 {
		logInfo("getTransitionsForAgent: workflow has no Transitions JSON")
		return nil
	}
	logInfo("getTransitionsForAgent: raw Transitions JSON", zap.String("transitions", string(workflow.Transitions)))

	var transitions []model.Transition
	if err := json.Unmarshal(workflow.Transitions, &transitions); err != nil {
		logError("getTransitionsForAgent: failed to parse transitions JSON", zap.Error(err))
		return nil
	}
	logInfo("getTransitionsForAgent: parsed transitions", zap.Int("count", len(transitions)))

	// 过滤出当前 Agent 作为源头的转换规则
	var result []model.Transition
	agentIDStr := agentConfigID.String()
	for _, t := range transitions {
		logInfo("getTransitionsForAgent: checking transition", zap.String("fromAgentID", t.FromAgentID), zap.String("currentAgentID", agentIDStr))
		if t.FromAgentID == agentIDStr {
			result = append(result, t)
		}
	}

	logInfo("getTransitionsForAgent: result", zap.Int("matchedCount", len(result)))
	return result
}

// ClearA2AContext 清理 A2A 上下文（Thread 完成或取消时调用）
func (es *ExecutionService) ClearA2AContext(threadID uuid.UUID) {
	es.a2aMu.Lock()
	delete(es.a2aContexts, threadID)
	es.a2aMu.Unlock()
}

// checkRouting 检查路由
func (es *ExecutionService) checkRouting(ctx context.Context, threadID uuid.UUID, currentConfig *model.AgentRoleConfig, output string) {
	mentions := es.parseMentions(output)

	if len(mentions) == 0 {
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

	// 获取项目路径
	var projectPath string
	if es.projectRepo != nil {
		project, err := es.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil {
			projectPath = project.LocalPath
		}
	}

	for _, mention := range mentions {
		var targetConfig *model.AgentRoleConfig

		if mention.Role != "" {
			// 按 role 查找
			targetConfig = es.findAgentByRole(allowedAgents, mention.Role)
		} else {
			// 按 name 查找
			targetConfig = es.findAgentByName(allowedAgents, mention.AgentName)
		}

		if targetConfig == nil {
			logInfo("路由被拒绝：目标不在工作流模板中",
				zap.String("mention", mention.Raw),
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

		// 更新 A2A 上下文
		es.a2aMu.Lock()
		a2aCtx.Depth++
		a2aCtx.InvokedAgents[targetConfig.ID] = true
		es.a2aMu.Unlock()

		logInfo("A2A 路由触发",
			zap.String("fromAgent", currentConfig.Name),
			zap.String("toAgent", targetConfig.Name),
			zap.Int("depth", a2aCtx.Depth),
			zap.String("threadId", threadID.String()))

		// 使用工作流模板中指定的 Agent 实例
		es.SpawnAgent(ctx, &SpawnRequest{
			ThreadID:    threadID,
			ConfigID:    targetConfig.ID,
			Role:        targetConfig.Role,
			Input:       output,
			ProjectPath: projectPath,
		})
	}
}

// parseMentions 解析@mention
// 支持: @developer (角色) 或 @前端开发 (实例名称)
func (es *ExecutionService) parseMentions(content string) []ParsedMention {
	var mentions []ParsedMention
	lines := strings.Split(content, "\n")
	count := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if count >= 2 {
			break
		}
		if strings.HasPrefix(line, "@") {
			mention := strings.Fields(line[1:])[0]
			if mention != "" {
				// 尝试解析为角色
				role := parseAgentRole(mention)

				mentions = append(mentions, ParsedMention{
					Role:      role,
					AgentName: mention,
					Raw:       mention,
				})
				count++
			}
		}
	}
	return mentions
}

// getAllowedAgentsFromWorkflow 从工作流模板获取允许路由的 Agent 列表
// 数据流: Thread → WorkflowTemplate → AgentIDs → AgentConfigs
func (es *ExecutionService) getAllowedAgentsFromWorkflow(ctx context.Context, threadID uuid.UUID) []*model.AgentRoleConfig {
	// 1. 获取 Thread
	thread, err := es.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return nil
	}

	// 优先使用 Project 的工作流模板，如果没有则使用 Thread 的
	var workflowTemplateID *uuid.UUID
	if es.projectRepo != nil {
		project, err := es.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil && project.WorkflowTemplateID != nil {
			workflowTemplateID = project.WorkflowTemplateID
		}
	}
	if workflowTemplateID == nil && thread.WorkflowTemplateID != nil {
		workflowTemplateID = thread.WorkflowTemplateID
	}

	if workflowTemplateID == nil {
		return nil
	}

	// 2. 获取工作流模板
	workflow, err := es.workflowRepo.FindByID(ctx, *workflowTemplateID)
	if err != nil || workflow == nil {
		return nil
	}

	// 3. 解析 AgentIDs JSON
	var agentIDs []string
	if len(workflow.AgentIDs) == 0 {
		return nil
	}
	if err := json.Unmarshal(workflow.AgentIDs, &agentIDs); err != nil {
		return nil
	}

	// 4. 查询每个 Agent 的配置
	var agents []*model.AgentRoleConfig
	for _, idStr := range agentIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		agent, err := es.configSvc.GetByID(ctx, id)
		if err == nil {
			agents = append(agents, agent)
		}
	}

	return agents
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

// checkSignalRouting 检查信号路由（基于Transitions自动路由）
// 支持三种路由类型：
// 1. sequence - 顺序执行：触发单个下游 Agent
// 2. parallel - 并行执行：同时触发多个下游 Agent（分支工作流）
// 3. merge - 汇聚执行：等待多个上游 Agent 完成后再执行
func (es *ExecutionService) checkSignalRouting(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string) {
	// 获取当前 Agent 的 Transitions
	transitions := es.getTransitionsForAgent(ctx, threadID, config.ID)
	if len(transitions) == 0 {
		logInfo("checkSignalRouting: no transitions found for agent", zap.String("agentId", config.ID.String()))
		return
	}

	// 获取或创建 A2A 上下文
	es.a2aMu.Lock()
	a2aCtx, exists := es.a2aContexts[threadID]
	if !exists {
		a2aCtx = &A2AContext{
			Depth:         0,
			InvokedAgents: make(map[uuid.UUID]bool),
			CompletedAgents: make(map[uuid.UUID]bool), // 初始化完成记录
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

	// 收集所有待触发的 Agent（支持并行）
	var agentsToTrigger []*model.AgentRoleConfig
	var agentsToMerge []*model.AgentRoleConfig

	// 根据 Transitions 自动路由到下一个 Agent
	for _, t := range transitions {
		// 检查条件路由
		if t.Condition != "" && !es.matchCondition(output, t.Condition) {
			logInfo("A2A 条件路由：条件不匹配，跳过",
				zap.String("condition", t.Condition),
				zap.String("fromAgent", config.Name))
			continue
		}

		targetID, err := uuid.Parse(t.ToAgentID)
		if err != nil {
			logError("Invalid target agent ID in transition", zap.Error(err), zap.String("toAgentId", t.ToAgentID))
			continue
		}

		// 查找目标 Agent 配置
		var targetConfig *model.AgentRoleConfig
		for _, agent := range allowedAgents {
			if agent.ID == targetID {
				targetConfig = agent
				break
			}
		}

		if targetConfig == nil {
			logInfo("自动路由被拒绝：目标不在工作流模板中",
				zap.String("toAgentId", t.ToAgentID),
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

		// 汇聚类型：检查是否所有上游 Agent 都已完成
		if t.Type == model.TransitionTypeMerge && len(t.WaitFor) > 0 {
			allCompleted := es.checkMergeCondition(threadID, t.WaitFor)
			if !allCompleted {
				logInfo("A2A 汇聚：等待上游 Agent 完成",
					zap.String("toAgent", targetConfig.Name),
					zap.Strings("waitingFor", t.WaitFor),
					zap.String("threadId", threadID.String()))
				continue
			}
			agentsToMerge = append(agentsToMerge, targetConfig)
			continue
		}

		// 顺序或并行类型：添加到待触发列表
		agentsToTrigger = append(agentsToTrigger, targetConfig)
	}

	// 合并需要触发的 Agent（汇聚类型的 Agent 也需要触发）
	agentsToTrigger = append(agentsToTrigger, agentsToMerge...)

	// 批量触发 Agent（支持并行）
	for _, targetConfig := range agentsToTrigger {
		// 更新 A2A 上下文
		es.a2aMu.Lock()
		a2aCtx.Depth++
		a2aCtx.InvokedAgents[targetConfig.ID] = true
		es.a2aMu.Unlock()

		// 查找对应的 Transition 获取类型
		var transitionType model.TransitionType
		for _, t := range transitions {
			if t.ToAgentID == targetConfig.ID.String() {
				transitionType = t.Type
				break
			}
		}
		if transitionType == "" {
			transitionType = model.TransitionTypeSequence // 默认顺序
		}

		logInfo("A2A 自动路由触发（基于Transitions）",
			zap.String("fromAgent", config.Name),
			zap.String("toAgent", targetConfig.Name),
			zap.String("trigger", "auto"),
			zap.String("type", string(transitionType)),
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
		agent.CancelFunc()
		delete(es.runningAgents, invocationID)
	}
	es.mu.Unlock()

	if !exists {
		return ErrAgentNotFound
	}

	// 更新状态
	invocation, err := es.invocationRepo.FindByID(ctx, invocationID)
	if err != nil {
		return err
	}

	invocation.Status = model.InvocationStatusCancelled
	invocation.CompletedAt = timePtr(time.Now())
	return es.invocationRepo.Update(ctx, invocation)
}

// broadcastStatus 广播状态
func (es *ExecutionService) broadcastStatus(threadID, invocationID uuid.UUID, status string, role model.AgentRole) {
	logInfo("broadcastStatus called", zap.String("threadId", threadID.String()), zap.String("invocationId", invocationID.String()), zap.String("status", status))
	if es.wsHub != nil {
		es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_status",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"invocationId": invocationID.String(),
				"status":       status,
				"role":         string(role),
			},
		})
	}
}

// broadcastChunk 广播输出块（实时流式输出）
func (es *ExecutionService) broadcastChunk(threadID, invocationID uuid.UUID, chunk Chunk, agentID, agentName string) {
	logInfo("broadcastChunk called", zap.String("threadId", threadID.String()), zap.String("chunkType", string(chunk.Type)), zap.String("toolName", chunk.ToolName))
	if es.wsHub != nil {
		payload := map[string]interface{}{
			"invocationId": invocationID.String(),
			"chunk":        chunk.Content,
			"chunkType":    string(chunk.Type),
			"agentId":      agentID,
			"agentName":    agentName,
		}

		// 添加工具相关信息
		if chunk.Type == ChunkTypeToolUse {
			payload["toolName"] = chunk.ToolName
			payload["toolId"] = chunk.ToolID
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
	} else {
		logInfo("broadcastChunk: wsHub is nil!")
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

// GetInvocationStatus 获取单个调用的状态
func (es *ExecutionService) GetInvocationStatus(ctx context.Context, invocationID uuid.UUID) (*model.AgentInvocation, error) {
	return es.invocationRepo.FindByID(ctx, invocationID)
}

// SpawnAgentForUserMessage 为用户消息触发Agent响应
// 实现message.AgentSpawner接口
// 使用工作流模板中指定的Agent，而不是根据Phase硬编码选择
func (es *ExecutionService) SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string) error {
	// 检查是否已有Agent在运行
	es.mu.RLock()
	for _, agent := range es.runningAgents {
		if agent.ThreadID == threadID {
			es.mu.RUnlock()
			// 已有Agent运行，不需要再触发
			return nil
		}
	}
	es.mu.RUnlock()

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

	// 获取工作流模板中的Agent列表
	var agentIDs []string
	if workflowTemplateID != nil && es.workflowRepo != nil {
		workflow, err := es.workflowRepo.FindByID(ctx, *workflowTemplateID)
		if err == nil && workflow != nil {
			// 解析 agent_ids JSON
			if len(workflow.AgentIDs) > 0 {
				if err := json.Unmarshal(workflow.AgentIDs, &agentIDs); err != nil {
					logError("Failed to parse agent_ids", zap.Error(err))
				}
			}
		}
	}

	// 如果工作流模板中有Agent，使用第一个Agent
	if len(agentIDs) > 0 {
		configID, err := uuid.Parse(agentIDs[0])
		if err != nil {
			return fmt.Errorf("invalid agent id in workflow template: %w", err)
		}

		// 验证Agent配置存在
		config, err := es.configSvc.GetByID(ctx, configID)
		if err != nil {
			logError("Agent config not found, falling back to default", zap.Error(err))
			// 继续使用回退逻辑
		} else {
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

	// 回退逻辑：根据当前阶段决定触发哪个Agent
	logDebug("No workflow agent found, using phase-based selection")
	role := es.workflow.GetPhaseAgent(thread.CurrentPhase)
	if role == "" {
		role = model.AgentRoleRequirement
	}

	// 检查是否有该角色的默认配置
	config, err := es.configSvc.GetDefaultByRole(ctx, role)
	if err != nil {
		// 如果没有找到配置，尝试获取任意一个可用的配置
		configs, listErr := es.configSvc.List(ctx)
		if listErr != nil || len(configs) == 0 {
			return fmt.Errorf("no agent config available: %w", err)
		}
		config = configs[0]
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