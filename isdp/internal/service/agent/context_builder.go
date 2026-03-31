package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	threadRepo     *repo.ThreadRepository
	msgRepo        *repo.MessageRepository
	artifactRepo   *repo.ArtifactRepository
	rosterBuilder  *TeammateRosterBuilder
	templateRepo   *repo.WorkflowTemplateRepository
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(
	threadRepo *repo.ThreadRepository,
	msgRepo *repo.MessageRepository,
	artifactRepo *repo.ArtifactRepository,
	rosterBuilder *TeammateRosterBuilder,
	templateRepo *repo.WorkflowTemplateRepository,
) *ContextBuilder {
	return &ContextBuilder{
		threadRepo:    threadRepo,
		msgRepo:       msgRepo,
		artifactRepo:  artifactRepo,
		rosterBuilder: rosterBuilder,
		templateRepo:  templateRepo,
	}
}

// Build 构建四层上下文
func (b *ContextBuilder) Build(ctx context.Context, threadID uuid.UUID, config *model.AgentConfig) (*ContextLayers, error) {
	layers := &ContextLayers{}

	// Layer 0: 系统提示 (角色定义 + 协作信息)
	l0, err := b.buildLayer0WithCollaboration(ctx, threadID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to build layer 0: %w", err)
	}
	layers.Layer0 = l0

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

	// Layer 3: 环境信息 (项目信息)
	l3, err := b.buildLayer3(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to build layer 3: %w", err)
	}
	layers.Layer3 = l3

	return layers, nil
}

// buildLayer0 构建Layer 0: 系统提示
func (b *ContextBuilder) buildLayer0(config *model.AgentConfig) string {
	var sb strings.Builder

	// 角色定义
	sb.WriteString(fmt.Sprintf("你是 %s (%s)。\n\n", config.Name, config.Description))

	// 系统提示
	sb.WriteString(config.SystemPrompt)
	sb.WriteString("\n\n")

	return sb.String()
}

// buildLayer0WithCollaboration 构建Layer 0: 系统提示 + 协作信息
func (b *ContextBuilder) buildLayer0WithCollaboration(ctx context.Context, threadID uuid.UUID, config *model.AgentConfig) (string, error) {
	var sb strings.Builder

	// 角色定义
	sb.WriteString(fmt.Sprintf("你是 %s (%s)。\n\n", config.Name, config.Description))

	// 系统提示
	sb.WriteString(config.SystemPrompt)
	sb.WriteString("\n\n")

	// 获取 Thread 信息
	thread, err := b.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return "", err
	}
	if thread == nil {
		return sb.String(), nil
	}

	// 注入协作信息
	sb.WriteString(b.buildCollaborationPrompt(ctx, thread, config.ID.String()))

	return sb.String(), nil
}

// buildCollaborationPrompt 构建协作提示
func (b *ContextBuilder) buildCollaborationPrompt(ctx context.Context, thread *model.Thread, currentAgentID string) string {
	var sb strings.Builder

	// 1. 队友名册
	if b.rosterBuilder != nil {
		teammates, err := b.rosterBuilder.Build(ctx, thread.ID, currentAgentID)
		if err == nil && len(teammates) > 0 {
			sb.WriteString("## 协作\n")
			sb.WriteString("你可以 @队友 请求协作。可用队友：\n\n")
			sb.WriteString("| 队友 | 角色 | 擅长 |\n")
			sb.WriteString("|------|------|------|\n")
			for _, t := range teammates {
				skills := strings.Join(t.Skills, ", ")
				if skills == "" {
					skills = "—"
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", t.Name, t.Role, skills))
			}
			sb.WriteString("\n")
		}
	}

	// 2. 协作规则（家规）
	sb.WriteString(b.buildGovernancePrompt())

	// 3. 工作流触发点（如果是工作流模式）
	if thread.Type == model.ThreadTypeWorkflow && thread.WorkflowTemplateID != nil && b.templateRepo != nil {
		sb.WriteString(b.buildWorkflowTriggersPrompt(ctx, *thread.WorkflowTemplateID, currentAgentID))
	}

	return sb.String()
}

// buildGovernancePrompt 构建协作规则提示
func (b *ContextBuilder) buildGovernancePrompt() string {
	return `## 协作规则

### 先搜后问原则
调用 isdp_multi_mention 前，必须先搜索相关资料（代码、文档、知识库）。
提供 searchEvidenceRefs 参数，说明你找到了什么证据。

**例外情况**：如果确实无法搜索（如全新概念），必须提供 overrideReason 说明理由。

### 防止级联
被召唤的 Agent 不得再次发起 isdp_multi_mention，避免无限级联。

### 多 Agent 讨论
使用 isdp_multi_mention 可以并行邀请 1-3 个 Agent 讨论同一问题。

`

}

// buildWorkflowTriggersPrompt 构建工作流触发点提示
func (b *ContextBuilder) buildWorkflowTriggersPrompt(ctx context.Context, templateID uuid.UUID, currentAgentID string) string {
	template, err := b.templateRepo.FindByID(ctx, templateID)
	if err != nil || template == nil {
		return ""
	}

	// 解析 Transitions
	var transitions []model.Transition
	if err := parseJSON(template.Transitions, &transitions); err != nil {
		return ""
	}

	// 找到当前 Agent 作为 fromAgentID 的 transitions
	var triggers []string
	for _, t := range transitions {
		if t.FromAgentID == currentAgentID && t.TriggerHint != "" {
			triggers = append(triggers, t.TriggerHint)
		}
	}

	if len(triggers) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 工作流触发点\n")
	sb.WriteString("在完成当前任务后，根据以下规则触发下一步：\n\n")
	for _, t := range triggers {
		sb.WriteString(fmt.Sprintf("- %s\n", t))
	}
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

// buildLayer3 构建Layer 3: 环境信息
func (b *ContextBuilder) buildLayer3(ctx context.Context, threadID uuid.UUID) (string, error) {
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

// parseJSON 解析 JSON 字节到目标
func parseJSON(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}