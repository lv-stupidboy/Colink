// 文件路径: isdp/internal/service/rule/service.go
package rule

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrRuleNameExists 规约名称已存在错误
var ErrRuleNameExists = fmt.Errorf("rule name already exists")

// Service Rule业务服务
type Service struct {
	repo            *repo.RuleRepository
	agentBindingRepo *repo.AgentRuleBindingRepository
	agentRepo       *repo.AgentConfigRepository
	storagePath     string
	logger          *zap.Logger
}

// NewService 创建Rule Service
func NewService(
	ruleRepo *repo.RuleRepository,
	agentBindingRepo *repo.AgentRuleBindingRepository,
	agentRepo *repo.AgentConfigRepository,
	storagePath string,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:            ruleRepo,
		agentBindingRepo: agentBindingRepo,
		agentRepo:       agentRepo,
		storagePath:     storagePath,
		logger:          logger,
	}
}

// Create 创建Rule
func (s *Service) Create(ctx context.Context, req *model.CreateRuleRequest) (*model.Rule, error) {
	// 检查名称格式
	if !isValidName(req.Name) {
		return nil, errors.New("名称只能包含小写字母、数字和中划线，且必须以字母开头")
	}

	// 检查名称是否重复
	existing, err := s.repo.FindByName(ctx, req.Name)
	if err == nil && existing != nil {
		return nil, ErrRuleNameExists
	}

	// 验证Scope
	if req.Scope != model.RuleScopePublic && req.Scope != model.RuleScopeInstance {
		return nil, errors.New("scope 必须是 public 或 instance")
	}

	rule := &model.Rule{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Scope:       req.Scope,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("创建规约失败: %w", err)
	}

	s.logger.Info("创建规约成功",
		zap.String("id", rule.ID.String()),
		zap.String("name", rule.Name),
		zap.String("scope", string(rule.Scope)),
	)

	return rule, nil
}

// Get 根据ID获取Rule
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Rule, error) {
	return s.repo.FindByID(ctx, id)
}

// GetByName 根据名称获取Rule
func (s *Service) GetByName(ctx context.Context, name string) (*model.Rule, error) {
	return s.repo.FindByName(ctx, name)
}

// List 列出Rules
func (s *Service) List(ctx context.Context, query *model.RuleListQuery) ([]*model.Rule, int64, error) {
	return s.repo.List(ctx, query)
}

// Update 更新Rule
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRuleRequest) (*model.Rule, error) {
	rule, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("规约不存在: %w", err)
	}

	if req.Description != "" {
		rule.Description = req.Description
	}
	if req.Scope != "" {
		if req.Scope != model.RuleScopePublic && req.Scope != model.RuleScopeInstance {
			return nil, errors.New("scope 必须是 public 或 instance")
		}
		rule.Scope = req.Scope
	}
	rule.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, fmt.Errorf("更新规约失败: %w", err)
	}

	s.logger.Info("更新规约成功",
		zap.String("id", rule.ID.String()),
		zap.String("name", rule.Name),
	)

	return rule, nil
}

// Delete 删除Rule
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 检查是否有Agent绑定
	agentRoleIDs, err := s.agentBindingRepo.FindByRuleID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if len(agentRoleIDs) > 0 {
		return fmt.Errorf("无法删除规约：该规约已被 %d 个Agent绑定", len(agentRoleIDs))
	}

	// 删除文件
	rule, err := s.repo.FindByID(ctx, id)
	if err == nil && rule != nil {
		filePath := fmt.Sprintf("%s/%s.md", s.storagePath, rule.Name)
		os.Remove(filePath) // 忽略错误
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除规约失败: %w", err)
	}

	s.logger.Info("删除规约成功",
		zap.String("id", id.String()),
	)

	return nil
}

// BindRulesToAgent 绑定Rules到Agent
func (s *Service) BindRulesToAgent(ctx context.Context, agentRoleID uuid.UUID, ruleIDs []uuid.UUID) error {
	// 空切片检查
	if len(ruleIDs) == 0 {
		return errors.New("规约ID列表不能为空")
	}

	// 验证Agent是否存在
	_, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return fmt.Errorf("Agent角色不存在: %w", err)
	}

	// 验证所有Rule存在
	for _, ruleID := range ruleIDs {
		_, err := s.repo.FindByID(ctx, ruleID)
		if err != nil {
			return fmt.Errorf("规约 %s 不存在: %w", ruleID.String(), err)
		}
	}

	// 创建绑定
	for _, ruleID := range ruleIDs {
		// 检查是否已存在绑定
		exists, err := s.agentBindingRepo.ExistsBinding(ctx, agentRoleID, ruleID)
		if err != nil {
			return err
		}
		if exists {
			continue // 已存在绑定，跳过
		}

		binding := &model.AgentRuleBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			RuleID:      ruleID,
			CreatedAt:   time.Now(),
		}
		if err := s.agentBindingRepo.Create(ctx, binding); err != nil {
			return err
		}
	}

	s.logger.Info("绑定规约到Agent成功",
		zap.String("agent_role_id", agentRoleID.String()),
		zap.Int("rule_count", len(ruleIDs)),
	)

	return nil
}

// BindPublicRulesToAgent 自动绑定所有公共规约到Agent
func (s *Service) BindPublicRulesToAgent(ctx context.Context, agentRoleID uuid.UUID) error {
	// 验证Agent是否存在
	_, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return fmt.Errorf("Agent角色不存在: %w", err)
	}

	// 获取所有公共规约
	publicRules, err := s.repo.FindByScope(ctx, model.RuleScopePublic)
	if err != nil {
		return fmt.Errorf("获取公共规约失败: %w", err)
	}

	if len(publicRules) == 0 {
		s.logger.Info("没有公共规约需要绑定",
			zap.String("agent_role_id", agentRoleID.String()),
		)
		return nil
	}

	// 绑定所有公共规约
	boundCount := 0
	for _, rule := range publicRules {
		// 检查是否已存在绑定
		exists, err := s.agentBindingRepo.ExistsBinding(ctx, agentRoleID, rule.ID)
		if err != nil {
			s.logger.Warn("检查绑定关系失败",
				zap.String("agent_role_id", agentRoleID.String()),
				zap.String("rule_id", rule.ID.String()),
				zap.Error(err),
			)
			continue
		}
		if exists {
			continue // 已存在绑定，跳过
		}

		binding := &model.AgentRuleBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			RuleID:      rule.ID,
			CreatedAt:   time.Now(),
		}
		if err := s.agentBindingRepo.Create(ctx, binding); err != nil {
			s.logger.Warn("创建绑定失败",
				zap.String("agent_role_id", agentRoleID.String()),
				zap.String("rule_id", rule.ID.String()),
				zap.Error(err),
			)
			continue
		}
		boundCount++
	}

	s.logger.Info("自动绑定公共规约到Agent成功",
		zap.String("agent_role_id", agentRoleID.String()),
		zap.Int("bound_count", boundCount),
		zap.Int("total_public_rules", len(publicRules)),
	)

	return nil
}

// GetAgentRules 获取Agent绑定的所有Rules
func (s *Service) GetAgentRules(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Rule, error) {
	return s.agentBindingRepo.FindRulesByAgentRoleID(ctx, agentRoleID)
}

// UnbindRuleFromAgent 解除Rule绑定
func (s *Service) UnbindRuleFromAgent(ctx context.Context, agentRoleID, ruleID uuid.UUID) error {
	// 检查绑定是否存在
	exists, err := s.agentBindingRepo.ExistsBinding(ctx, agentRoleID, ruleID)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("绑定关系不存在")
	}

	if err := s.agentBindingRepo.DeleteBinding(ctx, agentRoleID, ruleID); err != nil {
		return fmt.Errorf("解除绑定失败: %w", err)
	}

	s.logger.Info("解除规约绑定成功",
		zap.String("agent_role_id", agentRoleID.String()),
		zap.String("rule_id", ruleID.String()),
	)

	return nil
}

// GetPublicRules 获取所有公共规约
func (s *Service) GetPublicRules(ctx context.Context) ([]*model.Rule, error) {
	return s.repo.FindByScope(ctx, model.RuleScopePublic)
}

// GetInstanceRules 获取所有实例规约
func (s *Service) GetInstanceRules(ctx context.Context) ([]*model.Rule, error) {
	return s.repo.FindByScope(ctx, model.RuleScopeInstance)
}

// isValidName 校验名称格式
func isValidName(name string) bool {
	if len(name) == 0 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9-]*$`, name)
	return matched
}

// GetStoragePath 获取存储路径
func (s *Service) GetStoragePath() string {
	return s.storagePath
}