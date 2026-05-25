package market

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/anthropic/isdp/pkg/errors"
	pkgexec "github.com/anthropic/isdp/pkg/exec"
	"go.uber.org/zap"
)

// GitClient Git客户端，用于克隆市场仓库
type GitClient struct {
	logger *zap.Logger
	cfg    *config.GitURLConversionConfig
}

// NewGitClient 创建GitClient
func NewGitClient(logger *zap.Logger, cfg *config.GitURLConversionConfig) *GitClient {
	return &GitClient{logger: logger, cfg: cfg}
}

// Clone 克隆市场仓库到临时目录
func (g *GitClient) Clone(ctx context.Context, url, branch, tempBase string) (string, error) {
	url = g.cfg.ConvertHTTPToSSH(url)

	if err := os.MkdirAll(tempBase, 0755); err != nil {
		return "", errors.WithDetail(errors.ErrInternal, "create temp base dir: "+err.Error())
	}

	tempDir, err := os.MkdirTemp(tempBase, "market-sync-")
	if err != nil {
		return "", errors.WithDetail(errors.ErrInternal, "create temp dir: "+err.Error())
	}

	g.logger.Info("cloning market repository",
		zap.String("url", url),
		zap.String("branch", branch),
		zap.String("tempDir", tempDir),
	)

	cmd := pkgexec.GitCommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, url, tempDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		g.logger.Error("git clone failed", zap.Error(err), zap.String("output", string(output)))
		g.Cleanup(tempDir)
		return "", errors.WrapGitError(string(output), err)
	}

	g.logger.Info("market repository cloned successfully", zap.String("tempDir", tempDir))
	return tempDir, nil
}

// ParseMarketplaceJSON 解析 marketplace.json
func (g *GitClient) ParseMarketplaceJSON(cloneDir string) (*model.Marketplace, error) {
	marketplaceFile := filepath.Join(cloneDir, "marketplace.json")

	info, err := os.Stat(marketplaceFile)
	if err != nil {
		return nil, errors.NewParseFailed("marketplace.json", err)
	}
	if info.Size() > 64*1024 {
		return nil, errors.WithDetail(errors.ErrParseFailed,
			fmt.Sprintf("marketplace.json too large: %d bytes (max 64KB)", info.Size()))
	}

	data, err := os.ReadFile(marketplaceFile)
	if err != nil {
		return nil, errors.NewParseFailed("marketplace.json", err)
	}

	var marketplace model.Marketplace
	if err := json.Unmarshal(data, &marketplace); err != nil {
		return nil, errors.NewParseFailed("marketplace.json", err)
	}

	return &marketplace, nil
}

// Cleanup 清理临时目录
func (g *GitClient) Cleanup(cloneDir string) {
	if cloneDir == "" {
		return
	}
	if err := os.RemoveAll(cloneDir); err != nil {
		g.logger.Warn("failed to cleanup temp directory",
			zap.String("path", cloneDir),
			zap.Error(err))
	} else {
		g.logger.Info("market temp directory cleaned up", zap.String("path", cloneDir))
	}
}