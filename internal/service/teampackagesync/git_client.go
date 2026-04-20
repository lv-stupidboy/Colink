package teampackagesync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/anthropic/isdp/pkg/config"
	"go.uber.org/zap"
)

// GitClient handles git operations for remote package repository
type GitClient struct {
	basePath string // 数据根目录
	logger   *zap.Logger
}

// NewGitClient creates a new GitClient instance
func NewGitClient(cfg config.TeamPackageSyncConfig, basePath string, logger *zap.Logger) *GitClient {
	return &GitClient{
		basePath: basePath,
		logger:   logger,
	}
}

// CloneFromURL clones a specific URL to a temp directory
func (g *GitClient) CloneFromURL(ctx context.Context, url string, branch string) (string, error) {
	// 使用项目数据目录下的临时目录
	tempBase := filepath.Join(g.basePath, "temp")

	// 确保临时目录存在
	if err := os.MkdirAll(tempBase, 0755); err != nil {
		return "", fmt.Errorf("create temp base dir: %w", err)
	}

	// 在临时目录下创建本次同步的子目录
	tempDir, err := os.MkdirTemp(tempBase, "team-package-sync-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	g.logger.Info("cloning remote repository",
		zap.String("url", url),
		zap.String("branch", branch),
		zap.String("tempDir", tempDir),
	)

	// Git clone with --depth 1 and specified branch
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, url, tempDir)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		g.logger.Error("git clone failed",
			zap.Error(err),
			zap.String("output", string(output)),
		)
		g.Cleanup(tempDir)
		return "", fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}

	g.logger.Info("repository cloned successfully", zap.String("tempDir", tempDir))
	return tempDir, nil
}

// Cleanup removes the temp directory
func (g *GitClient) Cleanup(cloneDir string) {
	if cloneDir == "" {
		return
	}

	err := os.RemoveAll(cloneDir)
	if err != nil {
		g.logger.Warn("failed to cleanup temp dir",
			zap.String("path", cloneDir),
			zap.Error(err),
		)
	} else {
		g.logger.Info("temp directory cleaned up", zap.String("path", cloneDir))
	}
}