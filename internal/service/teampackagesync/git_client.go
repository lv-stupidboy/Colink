package teampackagesync

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anthropic/isdp/pkg/config"
	"go.uber.org/zap"
)

// GitClient handles git operations for remote package repository
type GitClient struct {
	config config.TeamPackageSyncConfig
	logger *zap.Logger
}

// NewGitClient creates a new GitClient instance
func NewGitClient(cfg config.TeamPackageSyncConfig, logger *zap.Logger) *GitClient {
	return &GitClient{
		config: cfg,
		logger: logger,
	}
}

// Clone clones the remote repo to a temp directory with --depth 1 for lightweight fetching
func (g *GitClient) Clone(ctx context.Context) (string, error) {
	// Create temp directory
	tempDir, err := ioutil.TempDir("", "team-package-sync-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	g.logger.Info("cloning remote repository",
		zap.String("url", g.config.RemoteRepoURL),
		zap.String("branch", g.config.Branch),
		zap.String("tempDir", tempDir),
	)

	// Git clone with --depth 1 and specified branch
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", g.config.Branch, g.config.RemoteRepoURL, tempDir)
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

// GetPackageList scans clone directory and builds RemotePackageList
func (g *GitClient) GetPackageList(cloneDir string) (*RemotePackageList, error) {
	list := &RemotePackageList{Categories: []RemotePackageCategory{}}

	// Read root directory entries
	entries, err := ioutil.ReadDir(cloneDir)
	if err != nil {
		return nil, fmt.Errorf("read clone dir: %w", err)
	}

	// Iterate through categories (directories in root)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") {
			continue // Skip hidden dirs and special dirs like .git
		}

		categoryPath := filepath.Join(cloneDir, entry.Name())
		category := RemotePackageCategory{
			Name:     entry.Name(),
			Packages: []RemotePackage{},
		}

		// Read packages in category
		pkgEntries, err := ioutil.ReadDir(categoryPath)
		if err != nil {
			g.logger.Warn("failed to read category dir",
				zap.String("path", categoryPath),
				zap.Error(err),
			)
			continue
		}

		for _, pkgEntry := range pkgEntries {
			if !pkgEntry.IsDir() || strings.HasPrefix(pkgEntry.Name(), ".") {
				continue
			}

			pkgPath := filepath.Join(categoryPath, pkgEntry.Name())
			pkg, err := g.parsePackageJSON(pkgPath)
			if err != nil {
				g.logger.Warn("failed to parse package",
					zap.String("path", pkgPath),
					zap.Error(err),
				)
				continue
			}

			if pkg != nil {
				category.Packages = append(category.Packages, *pkg)
			}
		}

		if len(category.Packages) > 0 {
			list.Categories = append(list.Categories, category)
		}
	}

	g.logger.Info("package list built",
		zap.Int("categories", len(list.Categories)),
	)

	return list, nil
}

// parsePackageJSON reads package.json from a package directory
// Security: limits package.json size to 64KB to prevent memory exhaustion
func (g *GitClient) parsePackageJSON(pkgPath string) (*RemotePackage, error) {
	pkgFile := filepath.Join(pkgPath, "package.json")

	// Check file exists and size limit (64KB)
	info, err := os.Stat(pkgFile)
	if err != nil {
		return nil, err
	}
	if info.Size() > 64*1024 {
		return nil, fmt.Errorf("package.json too large: %d bytes (max 64KB)", info.Size())
	}

	data, err := ioutil.ReadFile(pkgFile)
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}

	var pkg PackageInfo
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}

	return &RemotePackage{
		Name:        pkg.Name,
		Version:     pkg.Version,
		Description: pkg.Description,
		Path:        pkgPath,
	}, nil
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