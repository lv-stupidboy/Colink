package agent

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// TeammateRosterBuilder 队友名册构建器
// 用于构建当前 Thread 可用的队友列表
type TeammateRosterBuilder struct {
	threadRepo   *repo.ThreadRepository
	templateRepo *repo.WorkflowTemplateRepository
	agentRepo    *repo.AgentConfigRepository
}

// NewTeammateRosterBuilder 创建队友名册构建器
func NewTeammateRosterBuilder(
	threadRepo *repo.ThreadRepository,
	templateRepo *repo.WorkflowTemplateRepository,
	agentRepo *repo.AgentConfigRepository,
) *TeammateRosterBuilder {
	return &TeammateRosterBuilder{
		threadRepo:   threadRepo,
		templateRepo: templateRepo,
		agentRepo:    agentRepo,
	}
}

// TeammateInfo 队友信息
type TeammateInfo struct {
	ID     string   `json:"id"`     // Agent ID
	Name   string   `json:"name"`   // Agent 名称
	Role   string   `json:"role"`   // Agent 角色
	Skills []string `json:"skills"` // 技能/能力列表
}

// Build 构建队友名册
// 根据会话类型返回不同的 Agent 列表：
// - 自由模式：返回 Thread.AvailableAgents
// - 工作流模式：返回 WorkflowTemplate.AgentIDs
func (b *TeammateRosterBuilder) Build(ctx context.Context, threadID uuid.UUID, excludeAgentID string) ([]TeammateInfo, error) {
	// 1. 获取 Thread
	thread, err := b.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return nil, err
	}
	if thread == nil {
		return nil, nil
	}

	// 2. 获取 Agent ID 列表
	var agentIDs []string
	if thread.Type == model.ThreadTypeFreeDiscussion && len(thread.AvailableAgents) > 0 {
		// 自由模式：使用 Thread.AvailableAgents
		agentIDs = thread.AvailableAgents
	} else if thread.WorkflowTemplateID != nil {
		// 工作流模式：从 WorkflowTemplate 获取
		template, err := b.templateRepo.FindByID(ctx, *thread.WorkflowTemplateID)
		if err != nil {
			return nil, err
		}
		if template != nil {
			// 解析 AgentIDs JSON
			var ids []string
			if err := parseJSON(template.AgentIDs, &ids); err == nil {
				agentIDs = ids
			}
		}
	}

	// 3. 过滤掉排除的 Agent（通常是当前 Agent 自己）
	if excludeAgentID != "" {
		filtered := make([]string, 0, len(agentIDs))
		for _, id := range agentIDs {
			if id != excludeAgentID {
				filtered = append(filtered, id)
			}
		}
		agentIDs = filtered
	}

	// 4. 获取每个 Agent 的详细信息
	teammates := make([]TeammateInfo, 0, len(agentIDs))
	for _, agentIDStr := range agentIDs {
		agentID, err := uuid.Parse(agentIDStr)
		if err != nil {
			continue
		}

		config, err := b.agentRepo.FindByID(ctx, agentID)
		if err != nil || config == nil {
			continue
		}

		teammate := TeammateInfo{
			ID:     agentIDStr,
			Name:   config.Name,
			Role:   string(config.Role),
			Skills: config.Capabilities, // 使用 Capabilities 作为技能
		}
		teammates = append(teammates, teammate)
	}

	return teammates, nil
}

// BuildByAvailableAgents 根据指定的 Agent ID 列表构建名册
func (b *TeammateRosterBuilder) BuildByAvailableAgents(ctx context.Context, agentIDs []string, excludeAgentID string) ([]TeammateInfo, error) {
	// 过滤掉排除的 Agent
	if excludeAgentID != "" {
		filtered := make([]string, 0, len(agentIDs))
		for _, id := range agentIDs {
			if id != excludeAgentID {
				filtered = append(filtered, id)
			}
		}
		agentIDs = filtered
	}

	// 获取每个 Agent 的详细信息
	teammates := make([]TeammateInfo, 0, len(agentIDs))
	for _, agentIDStr := range agentIDs {
		agentID, err := uuid.Parse(agentIDStr)
		if err != nil {
			continue
		}

		config, err := b.agentRepo.FindByID(ctx, agentID)
		if err != nil || config == nil {
			continue
		}

		teammate := TeammateInfo{
			ID:     agentIDStr,
			Name:   config.Name,
			Role:   string(config.Role),
			Skills: config.Capabilities,
		}
		teammates = append(teammates, teammate)
	}

	return teammates, nil
}