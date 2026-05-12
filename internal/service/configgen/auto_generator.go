package configgen

import (
	"context"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AutoGenerator 自动配置生成器
// 在角色创建/更新、绑定变更、资产更新时自动触发配置生成
type AutoGenerator struct {
	configGenSvc    *Service
	agentRepo       *repo.AgentConfigRepository
	baseAgentRepo   *repo.BaseAgentRepository
	bindingRepos    *BindingRepositories
	logger          *zap.Logger
}

// BindingRepositories 绑定关系仓库集合
type BindingRepositories struct {
	SkillBindingRepo    *repo.AgentSkillBindingRepository
	CommandBindingRepo  *repo.AgentCommandBindingRepository
	SubagentBindingRepo *repo.AgentSubagentBindingRepository
	RuleBindingRepo     *repo.AgentRuleBindingRepository
	SettingsBindingRepo *repo.AgentSettingsBindingRepository
}

// NewAutoGenerator 创建自动配置生成器
func NewAutoGenerator(
	configGenSvc *Service,
	agentRepo *repo.AgentConfigRepository,
	baseAgentRepo *repo.BaseAgentRepository,
	bindingRepos *BindingRepositories,
	logger *zap.Logger,
) *AutoGenerator {
	return &AutoGenerator{
		configGenSvc:    configGenSvc,
		agentRepo:       agentRepo,
		baseAgentRepo:   baseAgentRepo,
		bindingRepos:    bindingRepos,
		logger:          logger,
	}
}

// GenerateSync 同步生成单个角色配置
// 阻塞等待生成完成，用于角色创建/更新后的自动生成
func (a *AutoGenerator) GenerateSync(ctx context.Context, agentRoleID uuid.UUID) error {
	// 1. 获取角色配置
	agentRole, err := a.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return fmt.Errorf("获取角色配置失败: %w", err)
	}

	// 2. 获取基础Agent类型
	baseAgentType, err := a.getBaseAgentType(ctx, agentRole)
	if err != nil {
		return fmt.Errorf("获取基础Agent类型失败: %w", err)
	}

	a.logger.Info("自动生成角色配置",
		zap.String("agent_id", agentRoleID.String()),
		zap.String("agent_name", agentRole.Name),
		zap.String("base_agent_type", baseAgentType))

	// 3. 调用现有配置生成服务
	req := &GenerateAgentConfigRequest{
		AgentRoleID:   agentRoleID,
		BaseAgentType: baseAgentType,
		CleanExisting: true,
	}

	_, err = a.configGenSvc.GenerateAgentConfig(ctx, req)
	if err != nil {
		a.logger.Error("配置生成失败",
			zap.String("agent_id", agentRoleID.String()),
			zap.Error(err))
		return fmt.Errorf("配置生成失败: %w", err)
	}

	a.logger.Info("配置生成完成",
		zap.String("agent_id", agentRoleID.String()))

	return nil
}

// GenerateMultiple 批量生成多个角色配置
// 用于资产更新后触发所有关联角色的配置重新生成
func (a *AutoGenerator) GenerateMultiple(ctx context.Context, agentIDs []uuid.UUID) []error {
	errors := make([]error, 0)
	for _, id := range agentIDs {
		if err := a.GenerateSync(ctx, id); err != nil {
			errors = append(errors, fmt.Errorf("角色 %s: %w", id.String(), err))
		}
	}
	return errors
}

// getBaseAgentType 获取角色的基础Agent类型
func (a *AutoGenerator) getBaseAgentType(ctx context.Context, agentRole *model.AgentRoleConfig) (string, error) {
	// 如果角色指定了 BaseAgentID，获取其类型
	if agentRole.BaseAgentID != uuid.Nil {
		baseAgent, err := a.baseAgentRepo.FindByID(ctx, agentRole.BaseAgentID)
		if err != nil {
			return "", fmt.Errorf("获取基础Agent失败: %w", err)
		}
		return string(baseAgent.Type), nil
	}

	// 如果未指定 BaseAgentID，使用默认基础Agent
	defaultBaseAgent, err := a.baseAgentRepo.FindDefault(ctx)
	if err != nil {
		return "", fmt.Errorf("获取默认基础Agent失败: %w", err)
	}
	if defaultBaseAgent == nil {
		// 如果没有默认基础Agent，返回空（后续会在GenerateAgentConfig中报错）
		return "", nil
	}

	return string(defaultBaseAgent.Type), nil
}

// GetAffectedAgentsBySkill 获取受Skill变更影响的Agent列表
func (a *AutoGenerator) GetAffectedAgentsBySkill(ctx context.Context, skillID uuid.UUID) ([]uuid.UUID, error) {
	if a.bindingRepos.SkillBindingRepo == nil {
		return nil, nil
	}
	return a.bindingRepos.SkillBindingRepo.FindBySkillID(ctx, skillID)
}

// GetAffectedAgentsByCommand 获取受Command变更影响的Agent列表
func (a *AutoGenerator) GetAffectedAgentsByCommand(ctx context.Context, commandID uuid.UUID) ([]uuid.UUID, error) {
	if a.bindingRepos.CommandBindingRepo == nil {
		return nil, nil
	}
	return a.bindingRepos.CommandBindingRepo.FindByCommandID(ctx, commandID)
}

// GetAffectedAgentsBySubagent 获取受Subagent变更影响的Agent列表
func (a *AutoGenerator) GetAffectedAgentsBySubagent(ctx context.Context, subagentID uuid.UUID) ([]uuid.UUID, error) {
	if a.bindingRepos.SubagentBindingRepo == nil {
		return nil, nil
	}
	return a.bindingRepos.SubagentBindingRepo.FindBySubagentID(ctx, subagentID)
}

// GetAffectedAgentsByRule 获取受Rule变更影响的Agent列表
func (a *AutoGenerator) GetAffectedAgentsByRule(ctx context.Context, ruleID uuid.UUID) ([]uuid.UUID, error) {
	if a.bindingRepos.RuleBindingRepo == nil {
		return nil, nil
	}
	return a.bindingRepos.RuleBindingRepo.FindByRuleID(ctx, ruleID)
}

// GetAffectedAgentsBySettings 获取受Settings变更影响的Agent列表
func (a *AutoGenerator) GetAffectedAgentsBySettings(ctx context.Context, settingsID uuid.UUID) ([]uuid.UUID, error) {
	if a.bindingRepos.SettingsBindingRepo == nil {
		return nil, nil
	}
	return a.bindingRepos.SettingsBindingRepo.FindBySettingsID(ctx, settingsID)
}