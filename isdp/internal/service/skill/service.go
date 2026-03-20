package skill

import (
	"context"
	"errors"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Service Skill业务服务
type Service struct {
	skillRepo  *repo.SkillRepository
	bindingRepo *repo.AgentSkillBindingRepository
	agentRepo  *repo.AgentConfigRepository
}

// NewService 创建Skill Service
func NewService(skillRepo *repo.SkillRepository, bindingRepo *repo.AgentSkillBindingRepository, agentRepo *repo.AgentConfigRepository) *Service {
	return &Service{
		skillRepo:   skillRepo,
		bindingRepo: bindingRepo,
		agentRepo:   agentRepo,
	}
}

// Create 创建Skill
func (s *Service) Create(ctx context.Context, req *model.CreateSkillRequest) (*model.Skill, error) {
	// 检查名称是否重复
	existing, err := s.skillRepo.FindByName(ctx, req.Name)
	if err == nil && existing != nil {
		return nil, errors.New("skill name already exists")
	}

	skill := &model.Skill{
		ID:              uuid.New(),
		Name:            req.Name,
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		Type:            req.Type,
		Category:        req.Category,
		SourceType:      req.SourceType,
		InstallSource:   req.InstallSource,
		SupportedAgents: req.SupportedAgents,
		Version:         req.Version,
		IsPublic:        req.IsPublic,
		Status:          model.SkillStatusActive,
		UseCount:        0,
		StarCount:       0,
		FavoriteCount:   0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.skillRepo.Create(ctx, skill); err != nil {
		return nil, err
	}

	return skill, nil
}

// GetByID 根据ID获取Skill
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.Skill, error) {
	return s.skillRepo.FindByID(ctx, id)
}

// GetByName 根据名称获取Skill
func (s *Service) GetByName(ctx context.Context, name string) (*model.Skill, error) {
	return s.skillRepo.FindByName(ctx, name)
}

// List 列出Skills
func (s *Service) List(ctx context.Context, query *model.SkillListQuery) ([]*model.Skill, int64, error) {
	return s.skillRepo.List(ctx, query)
}

// Update 更新Skill
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateSkillRequest) (*model.Skill, error) {
	skill, err := s.skillRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 更新字段
	if req.DisplayName != "" {
		skill.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		skill.Description = req.Description
	}
	if req.Type != "" {
		skill.Type = req.Type
	}
	if req.Category != "" {
		skill.Category = req.Category
	}
	if req.InstallSource != nil {
		skill.InstallSource = req.InstallSource
	}
	if req.SupportedAgents != nil {
		skill.SupportedAgents = req.SupportedAgents
	}
	if req.Version != "" {
		skill.Version = req.Version
	}
	if req.Status != "" {
		skill.Status = model.SkillStatus(req.Status)
	}
	skill.IsPublic = req.IsPublic
	skill.UpdatedAt = time.Now()

	if err := s.skillRepo.Update(ctx, skill); err != nil {
		return nil, err
	}

	return skill, nil
}

// Delete 删除Skill
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 检查是否有Agent绑定
	agentRoleIDs, err := s.bindingRepo.FindBySkillID(ctx, id)
	if err != nil {
		return err
	}
	if len(agentRoleIDs) > 0 {
		return errors.New("cannot delete skill: skill is bound to agents")
	}

	return s.skillRepo.Delete(ctx, id)
}

// BindSkills 绑定Skills到Agent
func (s *Service) BindSkills(ctx context.Context, agentRoleID uuid.UUID, skillIDs []uuid.UUID) error {
	// 验证Agent是否存在
	_, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return errors.New("agent not found")
	}

	// 验证所有Skill存在
	for _, skillID := range skillIDs {
		_, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			return errors.New("skill not found: " + skillID.String())
		}
	}

	// 创建绑定
	for _, skillID := range skillIDs {
		// 检查是否已存在绑定
		exists, err := s.bindingRepo.ExistsBinding(ctx, agentRoleID, skillID)
		if err != nil {
			return err
		}
		if exists {
			continue // 已存在绑定，跳过
		}

		binding := &model.AgentSkillBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			SkillID:     skillID,
			CreatedAt:   time.Now(),
		}
		if err := s.bindingRepo.Create(ctx, binding); err != nil {
			return err
		}
	}

	return nil
}

// UnbindSkill 解除Skill绑定
func (s *Service) UnbindSkill(ctx context.Context, agentRoleID, skillID uuid.UUID) error {
	return s.bindingRepo.DeleteBinding(ctx, agentRoleID, skillID)
}

// GetBoundSkills 获取Agent绑定的所有Skills
func (s *Service) GetBoundSkills(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Skill, error) {
	skillIDs, err := s.bindingRepo.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		return nil, err
	}

	skills := make([]*model.Skill, 0, len(skillIDs))
	for _, skillID := range skillIDs {
		skill, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			continue // 跳过不存在的skill
		}
		skills = append(skills, skill)
	}

	return skills, nil
}

// GetBoundAgents 获取Skill绑定的所有Agents
func (s *Service) GetBoundAgents(ctx context.Context, skillID uuid.UUID) ([]*model.AgentRoleConfig, error) {
	agentRoleIDs, err := s.bindingRepo.FindBySkillID(ctx, skillID)
	if err != nil {
		return nil, err
	}

	agents := make([]*model.AgentRoleConfig, 0, len(agentRoleIDs))
	for _, agentRoleID := range agentRoleIDs {
		agent, err := s.agentRepo.FindByID(ctx, agentRoleID)
		if err != nil {
			continue // 跳过不存在的agent
		}
		agents = append(agents, agent)
	}

	return agents, nil
}

// IncrementUse 增加使用次数
func (s *Service) IncrementUse(ctx context.Context, id uuid.UUID) error {
	return s.skillRepo.IncrementUseCount(ctx, id)
}