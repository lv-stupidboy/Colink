package teampackagesync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/isdp/pkg/config"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestSyncServiceBatchPreviewAndSyncErrorAggregation(t *testing.T) {
	service := &SyncService{
		gitClient: NewGitClient(config.TeamPackageSyncConfig{}, t.TempDir(), zap.NewNop(), &config.GitURLConversionConfig{}),
		logger:    zap.NewNop(),
	}

	preview, err := service.PreviewPackagesBatch(context.Background(), []PreviewRequestItem{
		{Name: "missing-market"},
		{Name: "no-market-service", MarketId: uuid.NewString()},
	})
	if err != nil {
		t.Fatalf("PreviewPackagesBatch returned error: %v", err)
	}
	if preview.SuccessCount != 0 || preview.FailedCount != 2 || len(preview.Previews) != 2 {
		t.Fatalf("unexpected preview batch result: %+v", preview)
	}
	if preview.Previews[0].Error == nil || preview.Previews[1].Error == nil {
		t.Fatalf("expected preview errors: %+v", preview.Previews)
	}

	syncResult, err := service.SyncPackagesBatch(context.Background(), []SyncRequestItem{
		{Name: "missing-market"},
		{Name: "no-market-service", MarketId: uuid.NewString()},
	})
	if err != nil {
		t.Fatalf("SyncPackagesBatch returned error: %v", err)
	}
	if syncResult.SuccessCount != 0 || syncResult.FailedCount != 2 || len(syncResult.Results) != 2 {
		t.Fatalf("unexpected sync batch result: %+v", syncResult)
	}
	if syncResult.Results[0].Error == nil || syncResult.Results[1].Error == nil {
		t.Fatalf("expected sync errors: %+v", syncResult.Results)
	}
}

func TestSyncServiceCleanupCacheRemovesCloneDirs(t *testing.T) {
	root := t.TempDir()
	cloneDir := filepath.Join(root, "clone")
	if err := os.MkdirAll(cloneDir, 0755); err != nil {
		t.Fatalf("mkdir clone dir: %v", err)
	}

	cache := NewCloneCache()
	cache.Set("git@example.com:owner/repo.git", "main", cloneDir)

	service := &SyncService{
		gitClient: NewGitClient(config.TeamPackageSyncConfig{}, root, zap.NewNop(), &config.GitURLConversionConfig{}),
		logger:    zap.NewNop(),
	}
	service.cleanupCache(cache)

	if _, err := os.Stat(cloneDir); !os.IsNotExist(err) {
		t.Fatalf("clone dir should be removed, err=%v", err)
	}
	if dirs := cache.GetAllDirs(); len(dirs) != 0 {
		t.Fatalf("cache should be cleared, got %v", dirs)
	}
	service.cleanupCache(nil)
}
