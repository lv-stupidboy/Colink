package repo

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// ReviewRepository 评审数据访问（使用Artifact表）
type ReviewRepository struct {
	artifactRepo *ArtifactRepository
}

// NewReviewRepository 创建Review Repository
func NewReviewRepository(artifactRepo *ArtifactRepository) *ReviewRepository {
	return &ReviewRepository{artifactRepo: artifactRepo}
}

// FindByThreadID 根据ThreadID查找评审
func (r *ReviewRepository) FindByThreadID(ctx context.Context, threadID uuid.UUID) ([]*model.Artifact, error) {
	artifacts, err := r.artifactRepo.FindByThreadID(ctx, threadID)
	if err != nil {
		return nil, err
	}

	var reviews = make([]*model.Artifact, 0) // 初始化为空数组，避免 JSON null
	for _, a := range artifacts {
		if a.Type == model.ArtifactTypeReview {
			reviews = append(reviews, a)
		}
	}
	return reviews, nil
}