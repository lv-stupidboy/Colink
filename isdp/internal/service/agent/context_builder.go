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

	// 能力说明
	sb.WriteString("你的能力:\n")
	sb.WriteString(b.getCapabilities(config.Role))

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

// getCapabilities 获取角色能力
func (b *ContextBuilder) getCapabilities(role model.AgentRole) string {
	capabilities := map[model.AgentRole]string{
		model.AgentRoleRequirement: `- 分析用户需求
- 编写需求文档
- 识别边界条件
- 与相关方沟通确认`,
		model.AgentRoleArchitect: `- 设计系统架构
- 选择技术栈
- 定义模块边界
- 评审技术方案`,
		model.AgentRoleDeveloper: `- 编写代码
- 实现功能
- 修复Bug
- 重构优化`,
		model.AgentRoleReviewer: `- 代码评审
- 安全审计
- 性能分析
- 提出改进建议`,
		model.AgentRoleTestEngineer: `- 编写测试用例
- 执行测试
- 报告问题
- 验证修复`,
		model.AgentRoleDevOps: `- 部署应用
- 配置环境
- 监控告警
- 自动化运维`,
	}
	return capabilities[role]
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

// BuildForPhase 为特定阶段构建上下文
func (b *ContextBuilder) BuildForPhase(ctx context.Context, threadID uuid.UUID, phase model.Phase) (*ContextLayers, error) {
	// 获取该阶段对应的默认配置
	role := getPhaseAgent(phase)
	defaultConfig := &model.AgentConfig{
		Name:   string(role),
		Role:   role,
		SystemPrompt: b.getDefaultSystemPrompt(role),
	}

	return b.Build(ctx, threadID, defaultConfig)
}

// getDefaultSystemPrompt 获取默认系统提示
func (b *ContextBuilder) getDefaultSystemPrompt(role model.AgentRole) string {
	prompts := map[model.AgentRole]string{
		model.AgentRoleRequirement: `你是一个专业的需求分析师。你的任务是:
1. 理解用户的需求
2. 分析需求的可行性和完整性
3. 编写详细的需求文档
4. 识别潜在的边界条件和异常情况

请使用@architect来将需求转交给架构师。`,
		model.AgentRoleArchitect: `你是一个经验丰富的架构师。你的任务是:
1. 分析需求文档
2. 设计系统架构
3. 选择合适的技术栈
4. 定义模块边界和接口

请使用@developer来将设计转交给开发者。`,
		model.AgentRoleDeveloper: `你是一个专业的开发者。你的任务是:
1. 根据设计文档实现功能
2. 编写高质量的代码
3. 确保代码可测试
4. 遵循编码规范

请使用@reviewer来请求代码评审。`,
		model.AgentRoleReviewer: `你是一个严格的代码评审者。你的任务是:
1. 评审代码质量
2. 检查安全问题
3. 分析性能影响
4. 提出改进建议

评审结果分为 P1/P2/P3 三个等级。P1必须修复，P2建议修复，P3可选修复。`,
		model.AgentRoleTestEngineer: `你是一个专业的测试工程师。你的任务是:
1. 设计测试用例
2. 执行功能测试
3. 验证Bug修复
4. 编写测试报告

请确保测试覆盖主要功能路径和边界条件。`,
		model.AgentRoleDevOps: `你是一个专业的DevOps工程师。你的任务是:
1. 配置部署环境
2. 执行部署操作
3. 配置监控告警
4. 处理线上问题

确保部署过程可回滚。`,
	}
	return prompts[role]
}

func getPhaseAgent(phase model.Phase) model.AgentRole {
	switch phase {
	case model.PhaseRequirement:
		return model.AgentRoleRequirement
	case model.PhaseDesign:
		return model.AgentRoleArchitect
	case model.PhaseDevelopment:
		return model.AgentRoleDeveloper
	case model.PhaseReview:
		return model.AgentRoleReviewer
	case model.PhaseTest:
		return model.AgentRoleTestEngineer
	case model.PhaseMerge:
		return model.AgentRoleDevOps
	default:
		return model.AgentRoleRequirement
	}
}