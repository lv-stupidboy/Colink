package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// BuildOptions 构建选项（渐进式增强）
type BuildOptions struct {
	IncludeGitContext       bool           // 是否包含 Git 上下文
	IncludeInstructionFiles bool           // 是否包含指令文件
	MaxInstructionChars     int            // 最大指令文件字符数（默认 12000）
	ProjectContext          *ProjectContext // 可选：预加载的项目上下文
}

// ContextBuilder 上下文构建器
// T2(H4): 上下文构建逻辑从 ExecutionService 中拆分出来，提高可测试性
type ContextBuilder struct {
	threadRepo           *repo.ThreadRepository
	msgRepo              *repo.MessageRepository
	artifactRepo         *repo.ArtifactRepository
	tokenBudgetManager   *TokenBudgetManager
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(threadRepo *repo.ThreadRepository, msgRepo *repo.MessageRepository, artifactRepo *repo.ArtifactRepository) *ContextBuilder {
	return &ContextBuilder{
		threadRepo:   threadRepo,
		msgRepo:      msgRepo,
		artifactRepo: artifactRepo,
	}
}

// NewContextBuilderWithBudget 创建带 Token 预算管理的上下文构建器
func NewContextBuilderWithBudget(threadRepo *repo.ThreadRepository, msgRepo *repo.MessageRepository, artifactRepo *repo.ArtifactRepository, tbm *TokenBudgetManager) *ContextBuilder {
	cb := NewContextBuilder(threadRepo, msgRepo, artifactRepo)
	cb.tokenBudgetManager = tbm
	return cb
}

// Build 构建四层上下文（向后兼容）
func (b *ContextBuilder) Build(ctx context.Context, threadID uuid.UUID, config *model.AgentConfig) (*ContextLayers, error) {
	return b.BuildWithOptions(ctx, threadID, config, nil)
}

// BuildWithOptions 增强版构建方法
func (b *ContextBuilder) BuildWithOptions(ctx context.Context, threadID uuid.UUID, config *model.AgentConfig, opts *BuildOptions) (*ContextLayers, error) {
	layers := &ContextLayers{}

	// Layer 0: 系统提示 (角色定义)
	layers.Layer0 = BuildStaticLayer0(config)

	// Layer 1: Thread历史 (对话上下文)
	l1, err := b.buildLayer1(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to build layer 1: %w", err)
	}
	layers.Layer1 = l1

	// Layer 2: 工作产物 (Artifacts)
	l2, err := b.buildLayer2(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to build layer 2: %w", err)
	}
	layers.Layer2 = l2

	// Layer 3: 环境信息 (项目信息) - 增强版
	l3, err := b.buildLayer3Enhanced(ctx, threadID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to build layer 3: %w", err)
	}
	layers.Layer3 = l3

	return layers, nil
}

// BuildStaticLayer0 构建 Layer 0: 系统提示（静态部分）
// T3(M1): 角色定义 + 系统提示 + 治理摘要（不变内容）
// T9(L0): 治理规则从 shared-rules.md 编译，嵌入 GOVERNANCE_L0_DIGEST
// 参考 clowder-ai SystemPromptBuilder: 静态部分抵抗上下文压缩
func BuildStaticLayer0(config *model.AgentConfig) string {
	var sb strings.Builder

	// 角色定义
	sb.WriteString(fmt.Sprintf("你是 %s (%s)。\n\n", config.Name, config.Description))

	// 系统提示（来自 AgentConfig）
	sb.WriteString(config.SystemPrompt)
	sb.WriteString("\n\n")

	// 治理摘要（GOVERNANCE_L0_DIGEST）
	// 从 shared-rules.md 编译，约 150 tokens，抵抗上下文压缩
	sb.WriteString("---\n\n")
	sb.WriteString(BuildGovernanceDigestWithVersion())
	sb.WriteString("\n---\n\n")

	return sb.String()
}

// BuildStaticLayer0Minimal 构建 Layer 0 最小版本（不含治理摘要）
// 用于 Token 预算紧张的场景
func BuildStaticLayer0Minimal(config *model.AgentConfig) string {
	var sb strings.Builder

	// 角色定义
	sb.WriteString(fmt.Sprintf("你是 %s (%s)。\n\n", config.Name, config.Description))

	// 系统提示
	sb.WriteString(config.SystemPrompt)
	sb.WriteString("\n")

	return sb.String()
}

// buildLayer1 构建Layer 1: Thread历史
func (b *ContextBuilder) buildLayer1(ctx context.Context, threadID uuid.UUID) (string, error) {
	messages, err := b.msgRepo.FindByThreadID(ctx, threadID, 50)
	if err != nil {
		return "", err
	}

	if len(messages) == 0 {
		return "", nil
	}

	var sb strings.Builder
	for _, msg := range messages {
		role := "用户"
		if msg.Role == model.MessageRoleAgent {
			role = msg.AgentID
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", role, msg.Content))
	}

	return sb.String(), nil
}

// buildLayer2 构建Layer 2: 工作产物
func (b *ContextBuilder) buildLayer2(ctx context.Context, threadID uuid.UUID) (string, error) {
	// 获取Artifacts (需要实现artifactRepo)
	// artifacts, err := b.artifactRepo.FindByThreadID(ctx, threadID)
	// 暂时返回空，后续实现
	return "", nil
}

// buildLayer3 构建Layer 3: 环境信息（原有方法）
func (b *ContextBuilder) buildLayer3(ctx context.Context, threadID uuid.UUID) (string, error) {
	return b.buildLayer3Enhanced(ctx, threadID, nil)
}

// buildLayer3Enhanced 构建 Layer 3: 环境信息（增强版）
func (b *ContextBuilder) buildLayer3Enhanced(ctx context.Context, threadID uuid.UUID, opts *BuildOptions) (string, error) {
	thread, err := b.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("当前环境信息:\n")
	sb.WriteString(fmt.Sprintf("- Thread ID: %s\n", threadID))
	sb.WriteString(fmt.Sprintf("- 当前阶段: %s\n", b.getPhaseName(thread.CurrentPhase)))
	sb.WriteString(fmt.Sprintf("- 当前Agent: %s\n", thread.CurrentAgent))
	sb.WriteString(fmt.Sprintf("- 状态: %s\n", b.getStatusName(thread.Status)))

	// 新增：Git 上下文
	if opts != nil && opts.IncludeGitContext && opts.ProjectContext != nil {
		pc := opts.ProjectContext
		if pc.GitStatus != "" {
			sb.WriteString("\n## Git Status\n")
			sb.WriteString(pc.GitStatus)
			sb.WriteString("\n")
		}
		if len(pc.RecentCommits) > 0 {
			sb.WriteString("\n## Recent Commits\n")
			for _, c := range pc.RecentCommits {
				sb.WriteString(fmt.Sprintf("  %s %s\n", c.Hash, c.Subject))
			}
		}
	}

	// 新增：指令文件
	if opts != nil && opts.IncludeInstructionFiles && opts.ProjectContext != nil {
		pc := opts.ProjectContext
		if len(pc.InstructionFiles) > 0 {
			sb.WriteString("\n## Claude Instructions\n")
			for _, f := range pc.InstructionFiles {
				sb.WriteString(fmt.Sprintf("### %s (scope: %s)\n", f.Path, f.Scope))
				sb.WriteString(f.Content)
				sb.WriteString("\n\n")
			}
		}
	}

	return sb.String(), nil
}

// getPhaseName 获取阶段名称
func (b *ContextBuilder) getPhaseName(phase model.Phase) string {
	names := map[model.Phase]string{
		model.PhaseRequirement:  "需求分析",
		model.PhaseDesign:       "架构设计",
		model.PhaseDevelopment:  "开发实现",
		model.PhaseReview:       "代码评审",
		model.PhaseTest:         "测试验证",
		model.PhaseMerge:        "合并部署",
		model.PhaseComplete:     "完成",
	}
	return names[phase]
}

// getStatusName 获取状态名称
func (b *ContextBuilder) getStatusName(status model.ThreadStatus) string {
	names := map[model.ThreadStatus]string{
		model.ThreadStatusIdle:    "空闲",
		model.ThreadStatusRunning: "运行中",
		model.ThreadStatusPaused:  "暂停",
		model.ThreadStatusComplete: "完成",
		model.ThreadStatusFailed:  "失败",
	}
	return names[status]
}

// ========== T2(H4): 从 ExecutionService 提取的上下文构建函数 ==========

// BuildChainHistoryLayer 构建链路历史层
// 提取自 ExecutionService.buildChainHistoryLayer
// BuildChainHistoryLayer 构建 A2A 链路历史信息块
// 格式：[对话历史]\n1. 用户/Agent名：内容\n...\n[/对话历史]
// 优先展示 a2a-handoff 结构化内容，无 handoff 时才展示摘要
func BuildChainHistoryLayer(chainHistory *A2AChainContext) string {
	if chainHistory == nil {
		return ""
	}

	// 如果没有前序响应，返回空
	if len(chainHistory.PreviousResponses) == 0 {
		return ""
	}

	var sb strings.Builder

	// 计算当前调用编号（ChainIndex + 预计后续调用数）
	totalCalls := chainHistory.ChainTotal

	sb.WriteString(fmt.Sprintf("[对话历史 - 共 %d 次调用]\n", totalCalls))

	// 按顺序输出每条记录：编号. Agent名称/用户：内容
	for i, resp := range chainHistory.PreviousResponses {
		// 编号从 1 开始
		callNum := i + 1

		// 获取发送者名称
		senderName := resp.AgentName
		if senderName == "" || senderName == "user" || resp.Role == "user" {
			senderName = "用户"
		}

		// 添加时间信息（如果有）
		timeInfo := ""
		if resp.Timestamp > 0 {
			t := time.Unix(resp.Timestamp, 0)
			timeInfo = t.Format("15:04")
		}

		// 优先提取 a2a-handoff 结构化内容
		handoff, hasHandoff := ExtractHandoffBlock(resp.Content)

		var content string
		var truncated bool

		if hasHandoff {
			// 有 handoff：直接使用结构化内容（不截断）
			content = FormatHandoffForA2A(handoff, senderName)
			truncated = false
		} else {
			// 无 handoff：截断摘要展示
			content = resp.Content
			if len(content) > 800 {
				content = TruncateHeadTail(content, 800)
				truncated = true
			}
		}

		// 格式：编号. [时间] 角色：内容（截断提示）
		if timeInfo != "" {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s：\n%s", callNum, timeInfo, senderName, content))
		} else {
			sb.WriteString(fmt.Sprintf("%d. %s：\n%s", callNum, senderName, content))
		}
		if truncated {
			sb.WriteString("\n...[内容已截断，保留首尾各400字符]...")
		}
		sb.WriteString("\n\n")
	}

	sb.WriteString("[/对话历史]\n")

	// 添加当前位置提示（简洁版）
	sb.WriteString(fmt.Sprintf("\n**当前**: 第 %d/%d 位\n", chainHistory.ChainIndex, totalCalls))

	return sb.String()
}

// BuildDynamicSystemPromptFromContext 构建动态系统提示（使用缓存的上下文）
// T3(M1): 只注入动态部分（下游协作方），静态部分由 Layer0 负责
// T7(L1): 包含治理规则注入
// T8(L2): 包含动态队友名册
// 提取自 ExecutionService.buildDynamicSystemPromptFromContext
func BuildDynamicSystemPromptFromContext(tc *ThreadContext, config *model.AgentRoleConfig) string {
	var sb strings.Builder

	// 从缓存中过滤当前 Agent 的转换规则
	var transitions []model.Transition
	agentIDStr := config.ID.String()
	for _, t := range tc.Transitions {
		if t.FromAgentID == agentIDStr {
			transitions = append(transitions, t)
		}
	}

	// 构建 Agent ID -> AgentConfig 映射
	agentMap := make(map[string]*model.AgentRoleConfig)
	for _, agent := range tc.AllowedAgents {
		agentMap[agent.ID.String()] = agent
	}

	// 注入协作提示
	if len(transitions) > 0 {
		sb.WriteString("\n\n## 下游协作方（需要时 @ 触发）\n")
		sb.WriteString("**⚠️ 触发规则（必须精确遵守，否则下游不会被拉起）**：\n")
		sb.WriteString("@mention 必须是**整行的第一个字符**（允许前导空白，不允许任何其他字符）。\n\n")
		sb.WriteString("✅ 触发格式（下游会被拉起）：\n```\n@全栈开发工程师 请实现登录 API\n```\n\n")
		sb.WriteString("❌ 以下写法**全部无效**，下游不会被拉起：\n")
		sb.WriteString("- Markdown 加粗前缀：`**接收方：** @全栈开发工程师`\n")
		sb.WriteString("- 列表项前缀：`- @全栈开发工程师 请实现`\n")
		sb.WriteString("- 表格单元格：`| 接收方 | @全栈开发工程师 |`\n")
		sb.WriteString("- 引用块：`> @全栈开发工程师`\n")
		sb.WriteString("- 标题内：`## @全栈开发工程师`\n")
		sb.WriteString("- 嵌入句子：`确认后我 @全栈开发工程师 实现`\n\n")
		sb.WriteString("**自检**：写完后逐行看 —— 触发 @mention 的那一行，去掉前导空格后，**第一个字符必须是 @**，否则改写。\n\n")
		sb.WriteString("可用的下游协作方：\n")
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

	// 跨团队协作方
	if len(tc.RoutableTeamAgents) > 0 {
		sb.WriteString("\n\n## 跨团队协作方（可路由团队）\n")
		sb.WriteString("你可以通过 @mention 触发以下跨团队的协作方：\n")
		for _, agent := range tc.RoutableTeamAgents {
			hint := generateTriggerHint(agent)
			sb.WriteString(fmt.Sprintf("- %s（来自其他团队）\n", hint))
		}
		sb.WriteString("\n**注意**：跨团队协作时，请明确说明任务上下文，帮助对方理解需求。\n")
	}

	// T8(L2): 动态队友名册
	if len(tc.AllowedAgents) > 0 {
		sb.WriteString("\n\n## 队友名册\n\n")
		for _, agent := range tc.AllowedAgents {
			if agent.ID.String() == agentIDStr {
				continue
			}
			sb.WriteString(fmt.Sprintf("- **%s**", agent.Name))
			if agent.Description != "" {
				sb.WriteString(fmt.Sprintf(" | 擅长: %s", agent.Description))
			}
			sb.WriteString(fmt.Sprintf(" (角色: %s)", agent.Role))
			if len(agent.MentionPatterns) > 0 {
				sb.WriteString(fmt.Sprintf(" | @: %s", strings.Join(agent.MentionPatterns, ", ")))
			}
			sb.WriteString("\n")
		}
	}

	// T7(L1): 治理规则已通过 GOVERNANCE_L0_DIGEST 嵌入 Layer0
	// 此处不再重复注入，避免 Token 浪费

	return sb.String()
}

// ApplyTokenBudgetConstraint 根据模型窗口大小裁剪上下文
// 提取自 ExecutionService.applyTokenBudgetConstraint
func ApplyTokenBudgetConstraint(layers *ContextLayers, modelName string, tbm *TokenBudgetManager) {
	if tbm == nil {
		return
	}

	windowSize := tbm.GetContextWindowSize(modelName)
	if windowSize <= 0 {
		return
	}

	systemTokens := EstimateTokens(layers.Layer0)
	inputTokens := EstimateTokens(layers.Layer1)
	artifactTokens := EstimateTokens(layers.Layer2)
	envTokens := EstimateTokens(layers.Layer3)
	totalTokens := systemTokens + inputTokens + artifactTokens + envTokens

	if int64(totalTokens) <= windowSize {
		return
	}

	safetyBuffer := int64(windowSize) * 5 / 100
	availableBudget := int64(windowSize) - int64(systemTokens) - safetyBuffer
	if availableBudget <= 0 {
		layers.Layer1 = ""
		layers.Layer2 = ""
		layers.Layer3 = ""
		return
	}

	remaining := availableBudget

	if int64(envTokens) > remaining {
		truncated := TruncateHeadTail(layers.Layer3, int(remaining*4))
		layers.Layer3 = truncated
		remaining = 0
	} else {
		remaining -= int64(envTokens)
	}

	if remaining > 0 && int64(artifactTokens) > remaining {
		truncated := TruncateHeadTail(layers.Layer2, int(remaining*4))
		layers.Layer2 = truncated
		remaining = 0
	} else if remaining > 0 {
		remaining -= int64(artifactTokens)
	}

	if remaining > 0 && int64(inputTokens) > remaining {
		truncated := TruncateHeadTail(layers.Layer1, int(remaining*4))
		layers.Layer1 = truncated
	}
}

// BuildA2AChainContext 构建 A2A 链路历史上下文
// 提取自 ExecutionService.buildA2AChainContext
func BuildA2AChainContext(a2aCtx *A2AContext, sessionStrategy SessionStrategy, remainingAgents int, tbm *TokenBudgetManager) *A2AChainContext {
	chainHistory := &A2AChainContext{
		ChainIndex:        a2aCtx.ChainIndex,
		ChainTotal:        a2aCtx.ChainIndex + remainingAgents,
		PreviousResponses: a2aCtx.PreviousResponses,
		OriginalMessage:   a2aCtx.OriginalMessage,
		FromAgent:         a2aCtx.FromAgent,
		SessionStrategy:   sessionStrategy,
		Depth:             a2aCtx.Depth,
	}

	// 传播 Token 预算信息
	if tbm != nil && len(a2aCtx.PreviousResponses) > 0 {
		var totalUsedTokens int
		for _, resp := range a2aCtx.PreviousResponses {
			totalUsedTokens += EstimateTokens(resp.Content)
		}
		systemEstimate := EstimateTokens(a2aCtx.OriginalMessage)
		totalUsedTokens += systemEstimate

		chainHistory.TokenBudget = CreateTokenBudgetInfo(
			DefaultMaxTotalTokens,
			totalUsedTokens,
		)
	}

	// 传播活跃参与者信息
	if a2aCtx.FromAgent != nil {
		chainHistory.ActiveParticipants = append(chainHistory.ActiveParticipants, ActiveParticipant{
			AgentID:      a2aCtx.FromAgent.ID.String(),
			LastActiveAt: 0, // caller should set
			MessageCount: len(a2aCtx.PreviousResponses),
		})
		for _, resp := range a2aCtx.PreviousResponses {
			if resp.AgentID != a2aCtx.FromAgent.ID {
				found := false
				for _, p := range chainHistory.ActiveParticipants {
					if p.AgentID == resp.AgentID.String() {
						found = true
						break
					}
				}
				if !found {
					chainHistory.ActiveParticipants = append(chainHistory.ActiveParticipants, ActiveParticipant{
						AgentID:      resp.AgentID.String(),
						LastActiveAt: resp.Timestamp,
						MessageCount: 1,
					})
				}
			}
		}
	}

	return chainHistory
}

// StripA2AMentions 过滤行首 @mention
// 提取自 ExecutionService.stripA2AMentions
func StripA2AMentions(input string) string {
	lines := strings.Split(input, "\n")
	var result []string
	mentionRe := regexp.MustCompile(`^[\s]*@[\w-]+`)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if mentionRe.MatchString(trimmed) {
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// CreatePromptDigest 生成 Prompt 摘要
// 提取自 ExecutionService.createPromptDigest
func CreatePromptDigest(fullPrompt string) (digest string, length int) {
	length = len(fullPrompt)
	hash := sha256.Sum256([]byte(fullPrompt))
	digest = fmt.Sprintf("%d:%x", length, hash[:8])
	return
}

// ExtractStructuredHistoryWithBudget 基于 Token 预算的历史消息裁剪
// 提取自 ExecutionService.extractStructuredHistoryWithBudget
func ExtractStructuredHistoryWithBudget(messages []*model.Message, modelName string, tbm *TokenBudgetManager) string {
	var windowSize int64 = DefaultContextWindow
	if tbm != nil {
		windowSize = tbm.GetContextWindowSize(modelName)
	}

	maxTokens := int(windowSize * 5 / 100)
	if maxTokens < 2000 {
		maxTokens = 2000
	}
	if maxTokens > 10000 {
		maxTokens = 10000
	}

	return ExtractStructuredHistoryWithBudgetLimit(messages, maxTokens)
}

// ExtractStructuredHistoryWithBudgetLimit 使用 token 预算裁剪历史消息
func ExtractStructuredHistoryWithBudgetLimit(messages []*model.Message, maxTokens int) string {
	// 过滤空内容和系统消息（仅保留已投递的用户/Agent 消息）
	delivered := make([]*model.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Content == "" {
			continue
		}
		// L3: 过滤系统消息（内部调试、中间状态等非投递消息）
		if msg.MessageType == model.MessageTypeSystem {
			continue
		}
		delivered = append(delivered, msg)
	}
	messages = delivered

	if len(messages) == 0 {
		return ""
	}

	// 参考 clowder-ai assembleContext: 反向遍历（从最近消息开始），保留最新上下文
	var sb strings.Builder
	sb.WriteString("## 会话历史摘要\n\n")

	usedTokens := 0
	headerTokens := EstimateTokens("## 会话历史摘要\n\n**用户请求**: \n\n**关键结论**:\n\n**涉及文件**:\n\n**工具调用摘要**:\n\n**对话参与者**:\n\n")
	usedTokens += headerTokens

	// 1. 提取用户核心请求（优先最近的用户消息）
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == model.MessageRoleUser {
			if usedTokens >= maxTokens {
				break
			}
			sb.WriteString("**用户请求**: ")
			content := msg.Content
			contentTokens := EstimateTokens(content)
			if contentTokens > 200 {
				content = TruncateHeadTail(content, 800)
				contentTokens = 200
			}
			remainingBudget := maxTokens - usedTokens
			if contentTokens > remainingBudget {
				content = TruncateHeadTail(content, remainingBudget*4)
			}
			sb.WriteString(content)
			sb.WriteString("\n\n")
			usedTokens += EstimateTokens(content)
			break // 只取最近一条用户消息
		}
	}

	if usedTokens >= maxTokens {
		return sb.String()
	}

	// 2. 提取关键决策和结论（反向遍历，优先最近）
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
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
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

	// 3. 提取文件路径引用（反向遍历，优先最近）
	sb.WriteString("**涉及文件**:\n")
	filePatterns := []string{
		`file://[^\s]+`,
		`path:\s*[^\s]+`,
		`\./[^\s]+`,
		`[a-zA-Z0-9_\-]+\.(go|ts|tsx|js|jsx|py|java|kt|rs|c|cpp|h|sql|yaml|yml|json|md|html|css)`,
	}
	excludeWords := map[string]bool{
		"true.md": true, "false.md": true, "null.json": true,
		"true.json": true, "false.json": true,
	}

	filesFound := false
	seenFiles := make(map[string]bool)
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
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

	// 4. 提取工具调用摘要（反向遍历，优先最近）
	sb.WriteString("**工具调用摘要**:\n")
	toolsFound := false
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == model.MessageRoleAgent && len(msg.ContentBlocks) > 0 {
			var blocks []ContentBlockData
			if err := json.Unmarshal(msg.ContentBlocks, &blocks); err == nil {
				for _, block := range blocks {
					if block.Type == "tool_use" {
						sb.WriteString("- [")
						sb.WriteString(block.ToolName)
						sb.WriteString("] ")
						if block.Input != nil {
							inputStr := fmt.Sprintf("%v", block.Input)
							if len(inputStr) > 100 {
								inputStr = inputStr[:100] + "..."
							}
							sb.WriteString(inputStr)
						}
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

	// 5. Agent 角色标识（已经是反向遍历）
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

// ExtractStructuredHistory 从历史消息中提取结构化关键信息
// 向后兼容版本
func ExtractStructuredHistory(messages []*model.Message, maxMessages int) string {
	if len(messages) > maxMessages {
		messages = messages[:maxMessages]
	}
	return ExtractStructuredHistoryWithBudgetLimit(messages, DefaultMaxTotalTokens)
}

// ExtractHandoffBlock extracts the <a2a-handoff> block from agent output.
// Returns the raw handoff content (without tags) and a boolean indicating whether a valid block was found.
func ExtractHandoffBlock(output string) (handoff string, found bool) {
	re := regexp.MustCompile(`(?s)<a2a-handoff>(.*?)</a2a-handoff>`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return "", false
	}
	content := strings.TrimSpace(matches[1])
	if content == "" {
		return "", false
	}
	return content, true
}

// ExtractHandoffBlockWithTags extracts the <a2a-handoff> block including tags.
// Returns the complete block with tags for downstream identification.
func ExtractHandoffBlockWithTags(output string) (handoffBlock string, found bool) {
	re := regexp.MustCompile(`(?s)<a2a-handoff>.*?</a2a-handoff>`)
	match := re.FindString(output)
	if match == "" {
		return "", false
	}
	return strings.TrimSpace(match), true
}

// FormatHandoffForA2A formats a handoff block for downstream consumption,
// adding the upstream agent identity header.
func FormatHandoffForA2A(handoff string, agentName string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[上游 Agent: %s 的输出]\n\n", agentName))
	sb.WriteString("## A2A 交接信息\n\n")
	sb.WriteString(handoff)
	sb.WriteString("\n")
	return sb.String()
}