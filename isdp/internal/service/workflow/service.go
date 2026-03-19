package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Service 工作流模板服务
type Service struct {
	repo *repo.WorkflowTemplateRepository
}

// NewService 创建工作流模板服务
func NewService(repo *repo.WorkflowTemplateRepository) *Service {
	return &Service{repo: repo}
}

// List 获取所有工作流模板
func (s *Service) List(ctx context.Context) ([]*model.WorkflowTemplate, error) {
	return s.repo.FindAll(ctx)
}

// GetByID 根据ID获取工作流模板
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowTemplate, error) {
	return s.repo.FindByID(ctx, id)
}

// Create 创建工作流模板
func (s *Service) Create(ctx context.Context, req *model.CreateWorkflowTemplateRequest) (*model.WorkflowTemplate, error) {
	agentIDs, _ := json.Marshal(req.AgentIDs)
	checkpoints, _ := json.Marshal(req.Checkpoints)
	transitions, _ := json.Marshal(req.Transitions)

	template := &model.WorkflowTemplate{
		ID:            uuid.New(),
		Name:          req.Name,
		Description:   req.Description,
		AgentIDs:      agentIDs,
		Transitions:   transitions,
		Checkpoints:   checkpoints,
		EstimatedTime: req.EstimatedTime,
		IsSystem:      false,
	}

	if err := s.repo.Create(ctx, template); err != nil {
		return nil, err
	}

	return template, nil
}

// Update 更新工作流模板
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateWorkflowTemplateRequest) (*model.WorkflowTemplate, error) {
	template, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		template.Name = req.Name
	}
	if req.Description != "" {
		template.Description = req.Description
	}
	if req.AgentIDs != nil {
		agentIDs, _ := json.Marshal(req.AgentIDs)
		template.AgentIDs = agentIDs
	}
	if req.Transitions != nil {
		transitions, _ := json.Marshal(req.Transitions)
		template.Transitions = transitions
	}
	if req.Checkpoints != nil {
		checkpoints, _ := json.Marshal(req.Checkpoints)
		template.Checkpoints = checkpoints
	}
	if req.EstimatedTime != "" {
		template.EstimatedTime = req.EstimatedTime
	}

	if err := s.repo.Update(ctx, template); err != nil {
		return nil, err
	}

	return template, nil
}

// Delete 删除工作流模板
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 检查是否为默认工作流
	template, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if template.IsDefault {
		return fmt.Errorf("该工作流是系统默认工作流，请先设置其他工作流为默认")
	}

	// 检查是否被项目引用
	count, err := s.repo.CountProjectReferences(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该工作流已被 %d 个项目绑定，无法删除", count)
	}

	return s.repo.Delete(ctx, id)
}

// SetDefault 设置默认工作流模板
func (s *Service) SetDefault(ctx context.Context, id uuid.UUID) (*model.WorkflowTemplate, error) {
	if err := s.repo.SetDefault(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// GetDefault 获取默认工作流模板
func (s *Service) GetDefault(ctx context.Context) (*model.WorkflowTemplate, error) {
	return s.repo.GetDefault(ctx)
}

// InitSystemTemplates 初始化系统预设模板
func (s *Service) InitSystemTemplates(ctx context.Context) error {
	// 先检查是否已有系统模板
	existingTemplates, err := s.repo.FindAll(ctx)
	if err != nil {
		return err
	}

	// 如果已有系统模板，跳过初始化
	for _, t := range existingTemplates {
		if t.IsSystem {
			return nil
		}
	}

	templates := []struct {
		Name          string
		Description   string
		AgentIDs      []string
		Checkpoints   []string
		EstimatedTime string
	}{
		{
			Name:          "标准开发流程",
			Description:   "完整的软件开发流程，从需求到部署",
			AgentIDs:      []string{},
			Checkpoints:   []string{"需求确认", "方案确认", "代码合入", "部署确认"},
			EstimatedTime: "2-4小时",
		},
		{
			Name:          "快速原型流程",
			Description:   "快速构建原型，验证想法",
			AgentIDs:      []string{},
			Checkpoints:   []string{"需求确认"},
			EstimatedTime: "30分钟-1小时",
		},
		{
			Name:          "代码重构流程",
			Description:   "优化现有代码结构和质量",
			AgentIDs:      []string{},
			Checkpoints:   []string{"方案确认", "代码合入"},
			EstimatedTime: "1-3小时",
		},
		{
			Name:          "问题修复流程",
			Description:   "快速定位和修复问题",
			AgentIDs:      []string{},
			Checkpoints:   []string{"修复确认"},
			EstimatedTime: "30分钟-2小时",
		},
	}

	for _, t := range templates {
		agentIDs, _ := json.Marshal(t.AgentIDs)
		checkpoints, _ := json.Marshal(t.Checkpoints)

		template := &model.WorkflowTemplate{
			ID:            uuid.New(),
			Name:          t.Name,
			Description:   t.Description,
			AgentIDs:      agentIDs,
			Checkpoints:   checkpoints,
			EstimatedTime: t.EstimatedTime,
			IsSystem:      true,
		}

		if err := s.repo.Create(ctx, template); err != nil {
			// 记录错误但继续尝试创建其他模板
			continue
		}
	}

	return nil
}