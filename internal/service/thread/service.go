package thread

import (
	"context"
	"errors"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Service Thread服务
type Service struct {
	repo         *repo.ThreadRepository
	projectRepo  *repo.ProjectRepository         // 新增依赖
	workflowRepo *repo.WorkflowTemplateRepository // 新增依赖
}

// NewService 创建Thread服务
func NewService(repo *repo.ThreadRepository, projectRepo *repo.ProjectRepository, workflowRepo *repo.WorkflowTemplateRepository) *Service {
	return &Service{
		repo:         repo,
		projectRepo:  projectRepo,
		workflowRepo: workflowRepo,
	}
}

// Create 创建Thread
// overrideWorkflowID: 可选，前端指定的团队ID
// 优先级：指定团队 > 项目绑定团队 > 系统默认团队
func (s *Service) Create(ctx context.Context, projectID uuid.UUID, name string, overrideWorkflowID *uuid.UUID) (*model.Thread, error) {
	// 1. 获取项目信息
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var workflowID *uuid.UUID

	// 2. 确定使用的团队（优先级：指定 > 项目绑定 > 默认）
	if overrideWorkflowID != nil {
		// 前端指定了团队，验证是否存在
		_, err := s.workflowRepo.FindByID(ctx, *overrideWorkflowID)
		if err != nil {
			return nil, errors.New("指定的工作流团队不存在")
		}
		workflowID = overrideWorkflowID
	} else if project.WorkflowTemplateID != nil {
		// 使用项目绑定的团队
		_, err := s.workflowRepo.FindByID(ctx, *project.WorkflowTemplateID)
		if err != nil {
			return nil, errors.New("项目绑定的工作流不存在，请重新配置")
		}
		workflowID = project.WorkflowTemplateID
	} else {
		// 使用默认团队
		defaultWorkflow, err := s.workflowRepo.GetDefault(ctx)
		if err != nil {
			return nil, errors.New("请先在项目设置中绑定工作流，或设置系统默认工作流")
		}
		workflowID = &defaultWorkflow.ID
	}

	// 3. 创建 Thread 并关联工作流
	thread := &model.Thread{
		ID:                 uuid.New(),
		ProjectID:          projectID,
		Name:               name,
		Status:             model.ThreadStatusIdle,
		CurrentPhase:       model.PhaseRequirement,
		WorkflowTemplateID: workflowID,
	}

	if err := s.repo.Create(ctx, thread); err != nil {
		return nil, err
	}
	return thread, nil
}

// GetByID 根据ID获取Thread
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.Thread, error) {
	return s.repo.FindByID(ctx, id)
}

// GetByProjectID 根据项目ID获取Thread列表
func (s *Service) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*model.Thread, error) {
	return s.repo.FindByProjectID(ctx, projectID)
}

// UpdateStatus 更新Thread状态
func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status model.ThreadStatus) error {
	thread, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	thread.Status = status
	return s.repo.Update(ctx, thread)
}

// SetPhase 设置当前阶段
func (s *Service) SetPhase(ctx context.Context, id uuid.UUID, phase model.Phase, agent string) error {
	thread, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	thread.CurrentPhase = phase
	thread.CurrentAgent = agent
	return s.repo.Update(ctx, thread)
}

// Update 更新 Thread（支持更新 workflowTemplateId）
func (s *Service) Update(ctx context.Context, id uuid.UUID, workflowTemplateID *uuid.UUID) (*model.Thread, error) {
	thread, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 如果指定了新的团队，验证是否存在
	if workflowTemplateID != nil {
		_, err := s.workflowRepo.FindByID(ctx, *workflowTemplateID)
		if err != nil {
			return nil, errors.New("指定的工作流团队不存在")
		}
	}

	thread.WorkflowTemplateID = workflowTemplateID
	if err := s.repo.Update(ctx, thread); err != nil {
		return nil, err
	}
	return thread, nil
}

// Delete 删除Thread
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}