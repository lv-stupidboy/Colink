package thread

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Service Thread服务
type Service struct {
	repo *repo.ThreadRepository
}

// NewService 创建Thread服务
func NewService(repo *repo.ThreadRepository) *Service {
	return &Service{repo: repo}
}

// Create 创建Thread
func (s *Service) Create(ctx context.Context, projectID uuid.UUID) (*model.Thread, error) {
	thread := &model.Thread{
		ID:           uuid.New(),
		ProjectID:    projectID,
		Status:       model.ThreadStatusIdle,
		CurrentPhase: model.PhaseRequirement,
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