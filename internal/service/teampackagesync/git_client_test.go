package teampackagesync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropic/isdp/pkg/config"
	"go.uber.org/zap"
)

func TestGitClientCloneFromURLTempBaseError(t *testing.T) {
	root := t.TempDir()
	baseFile := filepath.Join(root, "base-file")
	if err := os.WriteFile(baseFile, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("write base file: %v", err)
	}

	client := NewGitClient(config.TeamPackageSyncConfig{}, baseFile, zap.NewNop(), &config.GitURLConversionConfig{})
	_, err := client.CloneFromURL(context.Background(), "git@example.com:owner/repo.git", "main")
	if err == nil || !strings.Contains(err.Error(), "create temp base dir") {
		t.Fatalf("expected temp base error, got %v", err)
	}
}

func TestGitClientCleanupAndCachedFailure(t *testing.T) {
	root := t.TempDir()
	client := NewGitClient(config.TeamPackageSyncConfig{}, root, zap.NewNop(), &config.GitURLConversionConfig{})

	cloneDir := filepath.Join(root, "temp", "clone")
	if err := os.MkdirAll(cloneDir, 0755); err != nil {
		t.Fatalf("mkdir clone dir: %v", err)
	}
	client.Cleanup(cloneDir)
	if _, err := os.Stat(cloneDir); !os.IsNotExist(err) {
		t.Fatalf("cleanup should remove clone dir, err=%v", err)
	}
	client.Cleanup("")

	cache := NewCloneCache()
	_, err := client.CloneWithCache(context.Background(), "git@example.com:owner/repo.git", "main", cache)
	if err == nil {
		t.Fatalf("expected clone failure in test environment")
	}
	_, secondErr, isFirst := cache.GetOrMarkPending("git@example.com:owner/repo.git", "main")
	if isFirst || secondErr == nil || !strings.Contains(secondErr.Error(), "previous clone failed") {
		t.Fatalf("expected cached clone failure, isFirst=%v err=%v", isFirst, secondErr)
	}
}
