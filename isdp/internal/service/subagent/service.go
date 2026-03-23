package subagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrSubagentNameExists 子代理名称已存在错误
var ErrSubagentNameExists = fmt.Errorf("subagent name already exists")

// Service Subagent业务服务
type Service struct {
	repo            *repo.SubagentRepository
	bindingRepo     *repo.AgentSubagentBindingRepository
	skillBindingRepo *repo.SubagentSkillBindingRepository
	agentRepo       *repo.AgentConfigRepository
	skillRepo       *repo.SkillRepository
	logger          *zap.Logger
}

// NewService 创建Subagent Service
func NewService(
	subagentRepo *repo.SubagentRepository,
	bindingRepo *repo.AgentSubagentBindingRepository,
	skillBindingRepo *repo.SubagentSkillBindingRepository,
	agentRepo *repo.AgentConfigRepository,
	skillRepo *repo.SkillRepository,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:            subagentRepo,
		bindingRepo:     bindingRepo,
		skillBindingRepo: skillBindingRepo,
		agentRepo:       agentRepo,
		skillRepo:       skillRepo,
		logger:          logger,
	}
}

// Create 创建Subagent
func (s *Service) Create(ctx context.Context, req *model.CreateSubagentRequest) (*model.Subagent, error) {
	// 检查名称是否重复
	existing, err := s.repo.FindByName(ctx, req.Name)
	if err != nil {
		// 如果不是"未找到"错误，返回实际错误
		if !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("检查子代理名称失败: %w", err)
		}
		// 名称不存在，可以创建
	} else if existing != nil {
		return nil, ErrSubagentNameExists
	}

	subagent := &model.Subagent{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		SkillID:     req.SkillID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, subagent); err != nil {
		return nil, fmt.Errorf("创建子代理失败: %w", err)
	}

	s.logger.Info("创建子代理成功",
		zap.String("id", subagent.ID.String()),
		zap.String("name", subagent.Name),
	)

	return subagent, nil
}

// Get 根据ID获取Subagent
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Subagent, error) {
	return s.repo.FindByID(ctx, id)
}

// List 列出Subagents
func (s *Service) List(ctx context.Context, query *model.SubagentListQuery) ([]*model.Subagent, int64, error) {
	return s.repo.List(ctx, query)
}

// Update 更新Subagent
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateSubagentRequest) (*model.Subagent, error) {
	subagent, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("子代理不存在: %w", err)
	}

	// 更新字段
	if req.Description != "" {
		subagent.Description = req.Description
	}
	if req.Content != "" {
		subagent.Content = req.Content
	}
	subagent.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, subagent); err != nil {
		return nil, fmt.Errorf("更新子代理失败: %w", err)
	}

	s.logger.Info("更新子代理成功",
		zap.String("id", subagent.ID.String()),
		zap.String("name", subagent.Name),
	)

	return subagent, nil
}

// Delete 删除Subagent
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 检查是否有Agent绑定
	agentRoleIDs, err := s.bindingRepo.FindBySubagentID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if len(agentRoleIDs) > 0 {
		return fmt.Errorf("无法删除子代理：该子代理已被 %d 个Agent绑定", len(agentRoleIDs))
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除子代理失败: %w", err)
	}

	s.logger.Info("删除子代理成功",
		zap.String("id", id.String()),
	)

	return nil
}

// BindSubagents 绑定Subagents到Agent
func (s *Service) BindSubagents(ctx context.Context, agentRoleID uuid.UUID, subagentIDs []uuid.UUID) error {
	// 空切片检查
	if len(subagentIDs) == 0 {
		return errors.New("子代理ID列表不能为空")
	}

	// 验证Agent是否存在
	_, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return fmt.Errorf("Agent角色不存在: %w", err)
	}

	// 验证所有Subagent存在
	for _, subagentID := range subagentIDs {
		_, err := s.repo.FindByID(ctx, subagentID)
		if err != nil {
			return fmt.Errorf("子代理 %s 不存在: %w", subagentID.String(), err)
		}
	}

	// 创建绑定
	for _, subagentID := range subagentIDs {
		// 检查是否已存在绑定
		exists, err := s.bindingRepo.ExistsBinding(ctx, agentRoleID, subagentID)
		if err != nil {
			return err
		}
		if exists {
			continue // 已存在绑定，跳过
		}

		binding := &model.AgentSubagentBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			SubagentID:  subagentID,
			CreatedAt:   time.Now(),
		}
		if err := s.bindingRepo.Create(ctx, binding); err != nil {
			return err
		}
	}

	s.logger.Info("绑定子代理到Agent成功",
		zap.String("agent_role_id", agentRoleID.String()),
		zap.Int("subagent_count", len(subagentIDs)),
	)

	return nil
}

// GetAgentSubagents 获取Agent绑定的所有Subagents
func (s *Service) GetAgentSubagents(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Subagent, error) {
	return s.bindingRepo.FindSubagentsByAgentRoleID(ctx, agentRoleID)
}

// UnbindSubagent 解除Subagent绑定
func (s *Service) UnbindSubagent(ctx context.Context, agentRoleID, subagentID uuid.UUID) error {
	// 检查绑定是否存在
	exists, err := s.bindingRepo.ExistsBinding(ctx, agentRoleID, subagentID)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("绑定关系不存在")
	}

	if err := s.bindingRepo.DeleteBinding(ctx, agentRoleID, subagentID); err != nil {
		return fmt.Errorf("解除绑定失败: %w", err)
	}

	s.logger.Info("解除子代理绑定成功",
		zap.String("agent_role_id", agentRoleID.String()),
		zap.String("subagent_id", subagentID.String()),
	)

	return nil
}

// GetSkills 获取Subagent绑定的所有Skills
func (s *Service) GetSkills(ctx context.Context, subagentID uuid.UUID) ([]*model.Skill, error) {
	// 验证Subagent是否存在
	_, err := s.repo.FindByID(ctx, subagentID)
	if err != nil {
		return nil, fmt.Errorf("子代理不存在: %w", err)
	}

	return s.skillBindingRepo.FindSkillsBySubagentID(ctx, subagentID)
}

// BindSkills 绑定Skills到Subagent
func (s *Service) BindSkills(ctx context.Context, subagentID uuid.UUID, skillIDs []uuid.UUID) error {
	// 验证Subagent是否存在
	_, err := s.repo.FindByID(ctx, subagentID)
	if err != nil {
		return fmt.Errorf("子代理不存在: %w", err)
	}

	// 验证所有Skill存在
	for _, skillID := range skillIDs {
		_, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			return fmt.Errorf("技能 %s 不存在: %w", skillID.String(), err)
		}
	}

	// 先删除现有绑定
	if err := s.skillBindingRepo.DeleteBySubagentID(ctx, subagentID); err != nil {
		return fmt.Errorf("清除旧绑定失败: %w", err)
	}

	// 创建新绑定
	for _, skillID := range skillIDs {
		binding := &model.SubagentSkillBinding{
			ID:         uuid.New(),
			SubagentID: subagentID,
			SkillID:    skillID,
			CreatedAt:  time.Now(),
		}
		if err := s.skillBindingRepo.Create(ctx, binding); err != nil {
			return fmt.Errorf("创建绑定失败: %w", err)
		}
	}

	s.logger.Info("绑定技能到子代理成功",
		zap.String("subagent_id", subagentID.String()),
		zap.Int("skill_count", len(skillIDs)),
	)

	return nil
}

// UnbindSkill 解除Subagent的Skill绑定
func (s *Service) UnbindSkill(ctx context.Context, subagentID, skillID uuid.UUID) error {
	// 检查绑定是否存在
	exists, err := s.skillBindingRepo.ExistsBinding(ctx, subagentID, skillID)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("绑定关系不存在")
	}

	if err := s.skillBindingRepo.DeleteBinding(ctx, subagentID, skillID); err != nil {
		return fmt.Errorf("解除绑定失败: %w", err)
	}

	s.logger.Info("解除子代理技能绑定成功",
		zap.String("subagent_id", subagentID.String()),
		zap.String("skill_id", skillID.String()),
	)

	return nil
}