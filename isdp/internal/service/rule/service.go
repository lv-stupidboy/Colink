package rule

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

	// 验证Visibility
	if req.Visibility != model.RuleVisibilityPublic && req.Visibility != model.RuleVisibilityPrivate {
		return nil, errors.New("visibility 必须是 public 或 private")
	}

	rule := &model.Rule{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Visibility:  req.Visibility,
		Version:     req.Version,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 如果有内容，写入文件
	if s.storagePath != "" && req.Content != "" {
		if err := os.MkdirAll(s.storagePath, 0755); err != nil {
			return nil, fmt.Errorf("创建存储目录失败: %w", err)
		}
		filePath := filepath.Join(s.storagePath, req.Name+".md")
		if err := os.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
			return nil, fmt.Errorf("写入规约文件失败: %w", err)
		}
		rule.Content = req.Content
	}

	// 设置默认版本
	if rule.Version == "" {
		rule.Version = "1.0.0"
	}

	if err := s.repo.Create(ctx, rule); err != nil {
		// 回滚：删除已创建的文件
		if s.storagePath != "" && req.Content != "" {
			filePath := filepath.Join(s.storagePath, req.Name+".md")
			os.Remove(filePath)
		}
		return nil, fmt.Errorf("创建规约失败: %w", err)
	}

	s.logger.Info("创建规约成功",
		zap.String("id", rule.ID.String()),
		zap.String("name", rule.Name),
		zap.String("visibility", string(rule.Visibility)),
	)

	return rule, nil
}

// Get 根据ID获取Rule
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Rule, error) {
	rule, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 从文件读取内容
	s.populateContent(rule)
	return rule, nil
}

// GetByName 根据名称获取Rule
func (s *Service) GetByName(ctx context.Context, name string) (*model.Rule, error) {
	rule, err := s.repo.FindByName(ctx, name)
	if err != nil {
		return nil, err
	}
	// 从文件读取内容
	s.populateContent(rule)
	return rule, nil
}

// List 列出Rules
func (s *Service) List(ctx context.Context, query *model.RuleListQuery) ([]*model.Rule, int64, error) {
	rules, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	// 从文件读取内容
	for _, rule := range rules {
		s.populateContent(rule)
	}
	return rules, total, nil
}

// populateContent 从文件填充内容
func (s *Service) populateContent(rule *model.Rule) {
	if s.storagePath == "" || rule == nil {
		return
	}
	filePath := filepath.Join(s.storagePath, rule.Name+".md")
	content, err := os.ReadFile(filePath)
	if err == nil {
		rule.Content = string(content)
	}
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
	if req.Visibility != "" {
		if req.Visibility != model.RuleVisibilityPublic && req.Visibility != model.RuleVisibilityPrivate {
			return nil, errors.New("visibility 必须是 public 或 private")
		}
		rule.Visibility = req.Visibility
	}
	if req.Version != "" {
		rule.Version = req.Version
	}
	// 更新内容文件
	if s.storagePath != "" && req.Content != "" {
		filePath := filepath.Join(s.storagePath, rule.Name+".md")
		if err := os.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
			return nil, fmt.Errorf("更新规约文件失败: %w", err)
		}
		rule.Content = req.Content
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
	// 先获取规约信息（用于删除文件）
	rule, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("规约不存在: %w", err)
	}

	// 检查是否有Agent绑定，获取绑定的Agent名称
	agentRoleIDs, err := s.agentBindingRepo.FindByRuleID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if len(agentRoleIDs) > 0 {
		// 获取Agent名称列表
		agentNames := make([]string, 0, len(agentRoleIDs))
		for _, agentID := range agentRoleIDs {
			agent, err := s.agentRepo.FindByID(ctx, agentID)
			if err == nil {
				agentNames = append(agentNames, agent.Name)
			}
		}
		return fmt.Errorf("无法删除规约：该规约已被以下Agent绑定：%s", strings.Join(agentNames, "、"))
	}

	// 删除数据库记录
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除规约失败: %w", err)
	}

	// 删除对应的文件
	if s.storagePath != "" && rule != nil {
		filePath := filepath.Join(s.storagePath, rule.Name+".md")
		if _, err := os.Stat(filePath); err == nil {
			if err := os.Remove(filePath); err != nil {
				s.logger.Warn("删除规约文件失败", zap.String("path", filePath), zap.Error(err))
			} else {
				s.logger.Info("删除规约文件成功", zap.String("path", filePath))
			}
		}
	}

	s.logger.Info("删除规约成功", zap.String("id", id.String()), zap.String("name", rule.Name))
	return nil
}

// BindRulesToAgent 绑定Rules到Agent（全量替换）
func (s *Service) BindRulesToAgent(ctx context.Context, agentRoleID uuid.UUID, ruleIDs []uuid.UUID) error {
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

	// 先删除所有现有绑定
	if err := s.agentBindingRepo.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return fmt.Errorf("清理旧绑定失败: %w", err)
	}

	// 创建新的绑定
	for _, ruleID := range ruleIDs {
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

// BindPublicRulesToAgent 自动绑定所有公开规约到Agent
func (s *Service) BindPublicRulesToAgent(ctx context.Context, agentRoleID uuid.UUID) error {
	// 验证Agent是否存在
	_, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return fmt.Errorf("Agent角色不存在: %w", err)
	}

	// 获取所有公开规约
	publicRules, err := s.repo.FindByVisibility(ctx, model.RuleVisibilityPublic)
	if err != nil {
		return fmt.Errorf("获取公开规约失败: %w", err)
	}

	if len(publicRules) == 0 {
		s.logger.Info("没有公开规约需要绑定",
			zap.String("agent_role_id", agentRoleID.String()),
		)
		return nil
	}

	// 绑定所有公开规约
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

	s.logger.Info("自动绑定公开规约到Agent成功",
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

// GetPublicRules 获取所有公开规约
func (s *Service) GetPublicRules(ctx context.Context) ([]*model.Rule, error) {
	return s.repo.FindByVisibility(ctx, model.RuleVisibilityPublic)
}

// GetPrivateRules 获取所有私有规约
func (s *Service) GetPrivateRules(ctx context.Context) ([]*model.Rule, error) {
	return s.repo.FindByVisibility(ctx, model.RuleVisibilityPrivate)
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