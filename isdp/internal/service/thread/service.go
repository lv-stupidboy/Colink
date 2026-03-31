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
func (s *Service) Create(ctx context.Context, projectID uuid.UUID, name string) (*model.Thread, error) {
	return s.CreateWithType(ctx, projectID, name, model.ThreadTypeWorkflow, nil)
}

// CreateWithType 创建Thread（支持指定类型）
func (s *Service) CreateWithType(ctx context.Context, projectID uuid.UUID, name string, threadType model.ThreadType, availableAgents []string) (*model.Thread, error) {
	// 1. 获取项目信息
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var workflowID *uuid.UUID

	// 2. 检查项目是否绑定了工作流
	if project.WorkflowTemplateID != nil {
		// 验证工作流是否存在
		_, err := s.workflowRepo.FindByID(ctx, *project.WorkflowTemplateID)
		if err != nil {
			return nil, errors.New("项目绑定的工作流不存在，请重新配置")
		}
		workflowID = project.WorkflowTemplateID
	} else if threadType == model.ThreadTypeWorkflow {
		// 3. 工作流模式：查询默认工作流
		defaultWorkflow, err := s.workflowRepo.GetDefault(ctx)
		if err != nil {
			return nil, errors.New("请先在项目设置中绑定工作流，或设置系统默认工作流")
		}
		workflowID = &defaultWorkflow.ID
	}

	// 4. 创建 Thread 并关联工作流
	thread := &model.Thread{
		ID:                 uuid.New(),
		ProjectID:          projectID,
		Name:               name,
		Status:             model.ThreadStatusIdle,
		CurrentPhase:       model.PhaseRequirement,
		WorkflowTemplateID: workflowID,
		Type:               threadType,
		AvailableAgents:    availableAgents,
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