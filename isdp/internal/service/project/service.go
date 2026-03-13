package project

import (
	"context"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Service 项目服务
type Service struct {
	repo *repo.ProjectRepository
}

// NewService 创建项目服务
func NewService(repo *repo.ProjectRepository) *Service {
	return &Service{repo: repo}
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
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.CreateProjectRequest) (*model.Project, error) {
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		project.Name = req.Name
	}
	if req.Type != "" {
		project.Type = req.Type
	}
	if req.Mode != "" {
		project.Mode = req.Mode
	}
	project.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, project); err != nil {
		return nil, err
	}

	return project, nil
}

// Delete 删除项目
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}