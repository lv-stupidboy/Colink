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
	"github.com/anthropic/isdp/internal/service/mention"
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

	// Mention 解析器（支持动态 patterns）
	mentionParser *mention.Parser

	// Multi-Mention 编排器（可选，后续注入）
	multiMentionOrchestrator interface {
		IsActiveTarget(threadID uuid.UUID, agentID string) bool
	}

	runningAgents  map[uuid.UUID]*RunningAgent
	mu             sync.RWMutex

	// A2A 上下文追踪
	a2aContexts    map[uuid.UUID]*A2AContext // threadID -> A2AContext
	a2aMu          sync.RWMutex
}

// SetMultiMentionOrchestrator 设置 Multi-Mention 编排器
func (es *ExecutionService) SetMultiMentionOrchestrator(orch interface {
	IsActiveTarget(threadID uuid.UUID, agentID string) bool
}) {
	es.multiMentionOrchestrator = orch
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
		mentionParser:    mentionParser,
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
		ConfigDir: config.ConfigPath, // 使用生成的配置目录
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

// buildDynamicSystemPrompt 构建动态系统提示，注入工作流协作关系
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

	// 构建 A2A 输入（原始用户消息 + 前序响应上下文）
	a2aInput := es.buildA2AInput(ctx, threadID, currentConfig, output)

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
		es.a2aMu.Unlock()

		logInfo("A2A 路由触发",
			zap.String("fromAgent", currentConfig.Name),
			zap.String("toAgent", targetConfig.Name),
			zap.Int("depth", a2aCtx.Depth),
			zap.String("threadId", threadID.String()))

		// 使用构建的 A2A 输入（原始用户消息 + 前序响应）
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
func (es *ExecutionService) buildA2AInput(ctx context.Context, threadID uuid.UUID, fromAgent *model.AgentRoleConfig, output string) string {
	// 1. 获取原始用户消息
	originalMessage := es.getLastUserMessage(ctx, threadID)

	// 2. 移除纯 @mention 行
	strippedOutput := es.stripPureMentionLines(output)

	// 3. 构建格式化输入
	var sb strings.Builder

	if originalMessage != "" {
		sb.WriteString(originalMessage)
		sb.WriteString("\n\n---\n\n")
	}

	sb.WriteString(fmt.Sprintf("[%s 已经分析了这个问题：]\n\n", fromAgent.Name))
	sb.WriteString(strippedOutput)

	return sb.String()
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
		logInfo("SpawnAgentForUserMessage: 已有Agent运行，跳过")
		return nil
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