package market

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/anthropic/isdp/internal/model"
	"go.uber.org/zap"
)

// GitClient Git客户端，用于克隆市场仓库
type GitClient struct {
	logger *zap.Logger
}

// NewGitClient 创建GitClient
func NewGitClient(logger *zap.Logger) *GitClient {
	return &GitClient{logger: logger}
}

// Clone 克隆市场仓库到临时目录
func (g *GitClient) Clone(ctx context.Context, url, branch, tempBase string) (string, error) {
	if err := os.MkdirAll(tempBase, 0755); err != nil {
		return "", fmt.Errorf("create temp base dir: %w", err)
	}

	tempDir, err := os.MkdirTemp(tempBase, "market-sync-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	g.logger.Info("cloning market repository",
		zap.String("url", url),
		zap.String("branch", branch),
		zap.String("tempDir", tempDir),
	)

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, url, tempDir)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		g.logger.Error("git clone failed", zap.Error(err), zap.String("output", string(output)))
		g.Cleanup(tempDir)
		return "", fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}

	g.logger.Info("market repository cloned successfully", zap.String("tempDir", tempDir))
	return tempDir, nil
}

// ParseMarketplaceJSON 解析 marketplace.json
func (g *GitClient) ParseMarketplaceJSON(cloneDir string) (*model.Marketplace, error) {
	marketplaceFile := filepath.Join(cloneDir, "marketplace.json")

	info, err := os.Stat(marketplaceFile)
	if err != nil {
		return nil, fmt.Errorf("marketplace.json not found: %w", err)
	}
	if info.Size() > 64*1024 {
		return nil, fmt.Errorf("marketplace.json too large: %d bytes (max 64KB)", info.Size())
	}

	data, err := os.ReadFile(marketplaceFile)
	if err != nil {
		return nil, fmt.Errorf("read marketplace.json: %w", err)
	}

	var marketplace model.Marketplace
	if err := json.Unmarshal(data, &marketplace); err != nil {
		return nil, fmt.Errorf("parse marketplace.json: %w", err)
	}

	return &marketplace, nil
}

// Cleanup 清理临时目录
func (g *GitClient) Cleanup(cloneDir string) {
	if cloneDir == "" {
		return
	}
	os.RemoveAll(cloneDir)
	g.logger.Info("market temp directory cleaned up", zap.String("path", cloneDir))
}