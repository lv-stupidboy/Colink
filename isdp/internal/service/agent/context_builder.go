package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	threadRepo  *repo.ThreadRepository
	msgRepo     *repo.MessageRepository
	artifactRepo *repo.ArtifactRepository
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(threadRepo *repo.ThreadRepository, msgRepo *repo.MessageRepository, artifactRepo *repo.ArtifactRepository) *ContextBuilder {
	return &ContextBuilder{
		threadRepo:   threadRepo,
		msgRepo:      msgRepo,
		artifactRepo: artifactRepo,
	}
}

// Build 构建四层上下文
func (b *ContextBuilder) Build(ctx context.Context, threadID uuid.UUID, config *model.AgentConfig) (*ContextLayers, error) {
	layers := &ContextLayers{}

	// Layer 0: 系统提示 (角色定义)
	layers.Layer0 = b.buildLayer0(config)

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