package project

import (
	"context"
	"errors"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Service 项目服务
type Service struct {
	repo         *repo.ProjectRepository
	workflowRepo *repo.WorkflowTemplateRepository // 新增依赖
}

// NewService 创建项目服务
func NewService(repo *repo.ProjectRepository, workflowRepo *repo.WorkflowTemplateRepository) *Service {
	return &Service{
		repo:         repo,
		workflowRepo: workflowRepo,
	}
}

// List 列出项目
func (s *Service) List(ctx context.Context) ([]*model.Project, error) {
	return s.repo.FindAll(ctx, 100, 0)
}

// GetByID 根据ID获取项目
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	return s.repo.FindByID(ctx, id)
}

// Create 创建项目
func (s *Service) Create(ctx context.Context, req *model.CreateProjectRequest) (*model.Project, error) {
	project := &model.Project{
		ID:        uuid.New(),
		Name:      req.Name,
		Type:      req.Type,
		Mode:      req.Mode,
		Status:    model.ProjectStatusDraft,
		GitRepo:   req.ExistingRepoURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, project); err != nil {
		return nil, err
	}

	return project, nil
}

// Update 更新项目
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateProjectRequest) (*model.Project, error) {
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 如果设置了工作流ID，验证工作流是否存在
	if req.WorkflowTemplateID != nil && *req.WorkflowTemplateID != uuid.Nil {
		_, err := s.workflowRepo.FindByID(ctx, *req.WorkflowTemplateID)
		if err != nil {
			return nil, errors.New("指定的工作流模板不存在")
		}
	}

	// 更新字段
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Type != nil {
		project.Type = *req.Type
	}
	if req.Mode != nil {
		project.Mode = *req.Mode
	}
	if req.Status != nil {
		project.Status = *req.Status
	}
	if req.GitRepo != nil {
		project.GitRepo = *req.GitRepo
	}
	project.WorkflowTemplateID = req.WorkflowTemplateID

	if err := s.repo.Update(ctx, project); err != nil {
		return nil, err
	}

	return project, nil
}

// Delete 删除项目
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}