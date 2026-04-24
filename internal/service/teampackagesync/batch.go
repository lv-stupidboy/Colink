package teampackagesync

import (
	"context"
	"sync"

	"github.com/anthropic/isdp/internal/model"
)

// BatchPreviewRequest 批量预览请求
type BatchPreviewRequest struct {
	Packages []PreviewRequestItem `json:"packages"`
}

// PreviewRequestItem 单个预览请求项
type PreviewRequestItem struct {
	Name     string `json:"name"`
	MarketId string `json:"marketId"`
}

// PreviewResult 单个预览结果
type PreviewResult struct {
	Name  string                `json:"name"`
	Data  *PreviewPackageResponse `json:"data"`
	Error error                 `json:"error"`
}

// BatchPreviewResult 批量预览结果
type BatchPreviewResult struct {
	Previews       []PreviewResult `json:"previews"`
	TotalConflicts int             `json:"totalConflicts"`
	SuccessCount   int             `json:"successCount"`
	FailedCount    int             `json:"failedCount"`
}

// BatchSyncRequest 批量同步请求
type BatchSyncRequest struct {
	Packages []SyncRequestItem `json:"packages"`
}

// SyncRequestItem 单个同步请求项
type SyncRequestItem struct {
	Name     string                      `json:"name"`
	MarketId string                      `json:"marketId"`
	Confirm  *model.TeamPackageImportConfirm `json:"confirm"`
}

// SyncResult 单个同步结果
type SyncResult struct {
	Name  string             `json:"name"`
	Data  *model.ImportResult `json:"data"`
	Error error              `json:"error"`
}

// BatchSyncResult 批量同步结果
type BatchSyncResult struct {
	Results      []SyncResult `json:"results"`
	SuccessCount int          `json:"successCount"`
	FailedCount  int          `json:"failedCount"`
}

// PreviewPackagesBatch 批量预览团队包（并行，使用缓存避免重复克隆）
func (s *SyncService) PreviewPackagesBatch(ctx context.Context,
	requests []PreviewRequestItem) (*BatchPreviewResult, error) {

	// 创建请求级缓存
	cache := NewCloneCache()

	maxConcurrency := 5
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	results := make([]PreviewResult, len(requests))
	totalConflicts := 0
	successCount := 0
	failedCount := 0
	var mu sync.Mutex

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, name, marketId string) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := s.PreviewPackageWithCache(ctx, name, marketId, cache)

			mu.Lock()
			results[idx] = PreviewResult{
				Name:  name,
				Data:  data,
				Error: err,
			}
			if err != nil {
				failedCount++
			} else {
				successCount++
				totalConflicts += data.ConflictCount
			}
			mu.Unlock()
		}(i, req.Name, req.MarketId)
	}

	wg.Wait()

	// 批量操作完成后，统一清理所有克隆目录
	s.cleanupCache(cache)

	return &BatchPreviewResult{
		Previews:       results,
		TotalConflicts: totalConflicts,
		SuccessCount:   successCount,
		FailedCount:    failedCount,
	}, nil
}

// SyncPackagesBatch 批量同步团队包（并行，使用缓存避免重复克隆）
func (s *SyncService) SyncPackagesBatch(ctx context.Context,
	requests []SyncRequestItem) (*BatchSyncResult, error) {

	// 创建请求级缓存
	cache := NewCloneCache()

	maxConcurrency := 3
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	results := make([]SyncResult, len(requests))
	successCount := 0
	failedCount := 0
	var mu sync.Mutex

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, name, marketId string, confirm *model.TeamPackageImportConfirm) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := s.SyncPackageWithCache(ctx, name, marketId, confirm, cache)

			mu.Lock()
			results[idx] = SyncResult{
				Name:  name,
				Data:  data,
				Error: err,
			}
			if err != nil {
				failedCount++
			} else {
				successCount++
			}
			mu.Unlock()
		}(i, req.Name, req.MarketId, req.Confirm)
	}

	wg.Wait()

	// 批量操作完成后，统一清理所有克隆目录
	s.cleanupCache(cache)

	return &BatchSyncResult{
		Results:      results,
		SuccessCount: successCount,
		FailedCount:  failedCount,
	}, nil
}