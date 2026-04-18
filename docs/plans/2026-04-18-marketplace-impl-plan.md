# Marketplace Management Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement multi-market management for team packages, enabling users to add/configure multiple Git markets, parse marketplace.json, and sync team packages from any market.

**Architecture:** Add MarketService as a new service layer above existing TeamPackageSyncService. MarketService manages market configs, parses marketplace.json from Git repos, and caches team packages. TeamPackageSyncService is minimally modified to accept marketId parameter.

**Tech Stack:** Go (Gin), SQLite/MySQL, React (Ant Design), Zustand

---

## Task 1: Create Database Migration for Markets Table

**Files:**
- Create: `sql-change/v1.2.3/sqlite/00004_markets.sql`
- Create: `sql-change/v1.2.3/mysql/00004_markets.sql`
- Modify: `sql-change/v1.2.2/sqlite/00003_team_package_versions.sql` (add columns)

**Step 1: Write SQLite migration**

Create `sql-change/v1.2.3/sqlite/00004_markets.sql`:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS markets (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(random_blob(16)))),
    name VARCHAR(255) NOT NULL,
    url VARCHAR(500) NOT NULL,
    branch VARCHAR(100) DEFAULT 'main',
    enabled BOOLEAN DEFAULT 1,
    auto_update BOOLEAN DEFAULT 0,
    check_interval VARCHAR(20) DEFAULT '24h',
    last_synced_at DATETIME,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_markets_url ON markets(url);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS markets;
-- +goose StatementEnd
```

**Step 2: Write MySQL migration**

Create `sql-change/v1.2.3/mysql/00004_markets.sql`:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS markets (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(500) NOT NULL,
    branch VARCHAR(100) DEFAULT 'main',
    enabled BOOLEAN DEFAULT TRUE,
    auto_update BOOLEAN DEFAULT FALSE,
    check_interval VARCHAR(20) DEFAULT '24h',
    last_synced_at DATETIME,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE INDEX idx_markets_url ON markets(url);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS markets;
-- +goose StatementEnd
```

**Step 3: Add columns to team_package_versions**

Edit `sql-change/v1.2.2/sqlite/00003_team_package_versions.sql`, add after line 12:

```sql
-- Add market tracking columns
ALTER TABLE team_package_versions ADD COLUMN market_id TEXT;
ALTER TABLE team_package_versions ADD COLUMN source_path VARCHAR(500);
```

**Step 4: Run migration**

```bash
cd D:/CoLinkProject/Colink-TeamsUpdate/isdp
go build -o bin/migrate.exe ./cmd/migrate
bin/migrate.exe up --db ./data/colink.db --version 1.2.3
```

**Step 5: Commit**

```bash
git add sql-change/v1.2.3/
git commit -m "feat(db): add markets table for multi-market management"
```

---

## Task 2: Create Market Model

**Files:**
- Create: `internal/model/market.go`

**Step 1: Write Market model**

Create `internal/model/market.go`:

```go
package model

import (
	"time"
	"github.com/google/uuid"
)

// Market 市场配置
type Market struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	Branch         string     `json:"branch"`
	Enabled        bool       `json:"enabled"`
	AutoUpdate     bool       `json:"autoUpdate"`
	CheckInterval  string     `json:"checkInterval"`
	LastSyncedAt   *time.Time `json:"lastSyncedAt,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// Marketplace marketplace.json 结构
type Marketplace struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Owner       Owner     `json:"owner,omitempty"`
	Plugins     []Plugin  `json:"plugins"`
}

// Owner 市场所有者
type Owner struct {
	Name string `json:"name"`
}

// Plugin 市场插件/包
type Plugin struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Repository  string `json:"repository"`
	Source      string `json:"source"`
	Category    string `json:"category"`
}

// MarketPackage 市场团队包（用于前端展示）
type MarketPackage struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	MarketID     string `json:"marketId"`
	MarketName   string `json:"marketName"`
	Repository   string `json:"repository"`
	Source       string `json:"source"`
	LocalVersion string `json:"localVersion,omitempty"`
	LocalStatus  string `json:"localStatus"` // new, update, latest
}
```

**Step 2: Commit**

```bash
git add internal/model/market.go
git commit -m "feat(model): add Market and Marketplace models"
```

---

## Task 3: Create MarketRepository

**Files:**
- Create: `internal/repo/market_repo.go`

**Step 1: Write MarketRepository**

Create `internal/repo/market_repo.go`:

```go
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// MarketRepository 市场数据访问
type MarketRepository struct {
	BaseRepository
}

// NewMarketRepository 创建 MarketRepository
func NewMarketRepository(db *sql.DB, dbType DBType) *MarketRepository {
	return &MarketRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建市场
func (r *MarketRepository) Create(ctx context.Context, market *model.Market) error {
	query := `
		INSERT INTO markets (id, name, url, branch, enabled, auto_update, check_interval, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	id := uuid.New()

	_, err := r.DB().ExecContext(ctx, query,
		id.String(),
		market.Name,
		market.URL,
		market.Branch,
		market.Enabled,
		market.AutoUpdate,
		market.CheckInterval,
		now.Format(time.RFC3339),
		now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to create market: %w", err)
	}

	market.ID = id
	market.CreatedAt = now
	market.UpdatedAt = now
	return nil
}

// List 列出所有市场
func (r *MarketRepository) List(ctx context.Context) ([]model.Market, error) {
	query := `
		SELECT id, name, url, branch, enabled, auto_update, check_interval, last_synced_at, last_error, created_at, updated_at
		FROM markets ORDER BY created_at DESC
	`
	rows, err := r.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list markets: %w", err)
	}
	defer rows.Close()

	var markets []model.Market
	for rows.Next() {
		var m model.Market
		var idStr string
		var lastSyncedAt sql.NullString
		var lastError sql.NullString
		var createdAt, updatedAt SQLiteTimeScanner

		if err := rows.Scan(
			&idStr,
			&m.Name,
			&m.URL,
			&m.Branch,
			&m.Enabled,
			&m.AutoUpdate,
			&m.CheckInterval,
			&lastSyncedAt,
			&lastError,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan market: %w", err)
		}

		m.ID, _ = uuid.Parse(idStr)
		if lastSyncedAt.Valid {
			t, _ := time.Parse(time.RFC3339, lastSyncedAt.String)
			m.LastSyncedAt = &t
		}
		if lastError.Valid {
			m.LastError = lastError.String
		}
		m.CreatedAt = createdAt.Time
		m.UpdatedAt = updatedAt.Time
		markets = append(markets, m)
	}
	return markets, nil
}

// FindByID 根据ID查找市场
func (r *MarketRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Market, error) {
	query := `
		SELECT id, name, url, branch, enabled, auto_update, check_interval, last_synced_at, last_error, created_at, updated_at
		FROM markets WHERE id = ?
	`
	row := r.DB().QueryRowContext(ctx, query, id.String())

	var m model.Market
	var idStr string
	var lastSyncedAt sql.NullString
	var lastError sql.NullString
	var createdAt, updatedAt SQLiteTimeScanner

	err := row.Scan(
		&idStr,
		&m.Name,
		&m.URL,
		&m.Branch,
		&m.Enabled,
		&m.AutoUpdate,
		&m.CheckInterval,
		&lastSyncedAt,
		&lastError,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find market: %w", err)
	}

	m.ID, _ = uuid.Parse(idStr)
	if lastSyncedAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastSyncedAt.String)
		m.LastSyncedAt = &t
	}
	if lastError.Valid {
		m.LastError = lastError.String
	}
	m.CreatedAt = createdAt.Time
	m.UpdatedAt = updatedAt.Time
	return &m, nil
}

// Update 更新市场
func (r *MarketRepository) Update(ctx context.Context, market *model.Market) error {
	query := `
		UPDATE markets
		SET name = ?, url = ?, branch = ?, enabled = ?, auto_update = ?, check_interval = ?, last_synced_at = ?, last_error = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now()

	var lastSyncedAt interface{}
	if market.LastSyncedAt != nil {
		lastSyncedAt = market.LastSyncedAt.Format(time.RFC3339)
	}

	_, err := r.DB().ExecContext(ctx, query,
		market.Name,
		market.URL,
		market.Branch,
		market.Enabled,
		market.AutoUpdate,
		market.CheckInterval,
		lastSyncedAt,
		market.LastError,
		now.Format(time.RFC3339),
		market.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update market: %w", err)
	}

	market.UpdatedAt = now
	return nil
}

// Delete 删除市场
func (r *MarketRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM markets WHERE id = ?`
	_, err := r.DB().ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete market: %w", err)
	}
	return nil
}

// UpdateSyncStatus 更新同步状态
func (r *MarketRepository) UpdateSyncStatus(ctx context.Context, id uuid.UUID, syncedAt *time.Time, errMsg string) error {
	query := `
		UPDATE markets SET last_synced_at = ?, last_error = ?, updated_at = ? WHERE id = ?
	`
	now := time.Now()

	var lastSyncedAt interface{}
	if syncedAt != nil {
		lastSyncedAt = syncedAt.Format(time.RFC3339)
	}

	_, err := r.DB().ExecContext(ctx, query, lastSyncedAt, errMsg, now.Format(time.RFC3339), id.String())
	return err
}
```

**Step 2: Commit**

```bash
git add internal/repo/market_repo.go
git commit -m "feat(repo): add MarketRepository for market CRUD operations"
```

---

## Task 4: Create MarketService

**Files:**
- Create: `internal/service/market/types.go`
- Create: `internal/service/market/service.go`
- Create: `internal/service/market/git_client.go`

**Step 1: Create types.go**

Create `internal/service/market/types.go`:

```go
package market

import (
	"github.com/anthropic/isdp/internal/model"
)

// AddMarketRequest 添加市场请求
type AddMarketRequest struct {
	Name   string `json:"name" binding:"required"`
	URL    string `json:"url" binding:"required"`
	Branch string `json:"branch"`
}

// UpdateMarketRequest 更新市场请求
type UpdateMarketRequest struct {
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	AutoUpdate    bool   `json:"autoUpdate"`
	CheckInterval string `json:"checkInterval"`
}

// MarketSyncResult 市场同步结果
type MarketSyncResult struct {
	Market     model.Market     `json:"market"`
	Marketplace model.Marketplace `json:"marketplace"`
}
```

**Step 2: Create git_client.go (reuse pattern from teampackagesync)**

Create `internal/service/market/git_client.go`:

```go
package market

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
```

**Step 3: Create service.go**

Create `internal/service/market/service.go`:

```go
package market

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service 市场管理服务
type Service struct {
	marketRepo *repo.MarketRepository
	versionRepo *repo.TeamPackageVersionRepository
	gitClient  *GitClient
	tempBase   string
	cache      map[uuid.UUID]*model.Marketplace
	cacheMutex sync.RWMutex
	logger     *zap.Logger
}

// NewService 创建市场服务
func NewService(
	marketRepo *repo.MarketRepository,
	versionRepo *repo.TeamPackageVersionRepository,
	tempBase string,
	logger *zap.Logger,
) *Service {
	return &Service{
		marketRepo: marketRepo,
		versionRepo: versionRepo,
		gitClient:  NewGitClient(logger),
		tempBase:   tempBase,
		cache:      make(map[uuid.UUID]*model.Marketplace),
		logger:     logger,
	}
}

// ListMarkets 列出所有市场
func (s *Service) ListMarkets(ctx context.Context) ([]model.Market, error) {
	return s.marketRepo.List(ctx)
}

// AddMarket 添加市场
func (s *Service) AddMarket(ctx context.Context, req AddMarketRequest) (*model.Market, error) {
	if req.Branch == "" {
		req.Branch = "main"
	}

	market := &model.Market{
		Name:          req.Name,
		URL:           req.URL,
		Branch:        req.Branch,
		Enabled:       true,
		AutoUpdate:    false,
		CheckInterval: "24h",
	}

	if err := s.marketRepo.Create(ctx, market); err != nil {
		return nil, err
	}

	return market, nil
}

// UpdateMarket 更新市场配置
func (s *Service) UpdateMarket(ctx context.Context, id uuid.UUID, req UpdateMarketRequest) (*model.Market, error) {
	market, err := s.marketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if market == nil {
		return nil, fmt.Errorf("market not found")
	}

	if req.Name != "" {
		market.Name = req.Name
	}
	market.Enabled = req.Enabled
	market.AutoUpdate = req.AutoUpdate
	if req.CheckInterval != "" {
		market.CheckInterval = req.CheckInterval
	}

	if err := s.marketRepo.Update(ctx, market); err != nil {
		return nil, err
	}

	return market, nil
}

// DeleteMarket 删除市场
func (s *Service) DeleteMarket(ctx context.Context, id uuid.UUID) error {
	return s.marketRepo.Delete(ctx, id)
}

// RefreshMarket 刷新市场（重新克隆并解析 marketplace.json）
func (s *Service) RefreshMarket(ctx context.Context, id uuid.UUID) (*model.Marketplace, error) {
	market, err := s.marketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if market == nil {
		return nil, fmt.Errorf("market not found")
	}

	cloneDir, err := s.gitClient.Clone(ctx, market.URL, market.Branch, s.tempBase)
	if err != nil {
		s.marketRepo.UpdateSyncStatus(ctx, id, nil, err.Error())
		return nil, err
	}
	defer s.gitClient.Cleanup(cloneDir)

	marketplace, err := s.gitClient.ParseMarketplaceJSON(cloneDir)
	if err != nil {
		s.marketRepo.UpdateSyncStatus(ctx, id, nil, err.Error())
		return nil, err
	}

	// 更新缓存
	s.cacheMutex.Lock()
	s.cache[id] = marketplace
	s.cacheMutex.Unlock()

	// 更新同步状态
	now := time.Now()
	s.marketRepo.UpdateSyncStatus(ctx, id, &now, "")

	s.logger.Info("market refreshed successfully",
		zap.String("market", market.Name),
		zap.Int("plugins", len(marketplace.Plugins)),
	)

	return marketplace, nil
}

// GetCachedMarketplace 获取缓存的市场数据
func (s *Service) GetCachedMarketplace(id uuid.UUID) *model.Marketplace {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	return s.cache[id]
}

// GetTeamPackages 获取所有市场的团队包列表
func (s *Service) GetTeamPackages(ctx context.Context) ([]model.MarketPackage, error) {
	markets, err := s.marketRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	// 获取本地版本列表
	localVersions, err := s.versionRepo.ListAll(ctx)
	if err != nil {
		s.logger.Warn("failed to get local versions", zap.Error(err))
		localVersions = []model.TeamPackageVersion{}
	}

	// 构建本地版本映射
	localMap := make(map[string]string)
	for _, v := range localVersions {
		localMap[v.Name] = v.Version
	}

	var packages []model.MarketPackage

	for _, market := range markets {
		if !market.Enabled {
			continue
		}

		// 获取缓存数据，如果没有则刷新
		marketplace := s.GetCachedMarketplace(market.ID)
		if marketplace == nil {
			// 尝试刷新（但不阻塞）
			mp, err := s.RefreshMarket(ctx, market.ID)
			if err != nil {
				s.logger.Warn("failed to refresh market",
					zap.String("market", market.Name),
					zap.Error(err),
				)
				continue
			}
			marketplace = mp
		}

		// 只筛选 category=team 的包
		for _, plugin := range marketplace.Plugins {
			if strings.ToLower(plugin.Category) != "team" {
				continue
			}

			pkg := model.MarketPackage{
				Name:        plugin.Name,
				Version:     plugin.Version,
				Description: plugin.Description,
				MarketID:    market.ID.String(),
				MarketName:  market.Name,
				Repository:  plugin.Repository,
				Source:      plugin.Source,
			}

			// 对比本地版本
			if localVer, exists := localMap[plugin.Name]; exists {
				pkg.LocalVersion = localVer
				if compareVersions(localVer, plugin.Version) < 0 {
					pkg.LocalStatus = "update"
				} else {
					pkg.LocalStatus = "latest"
				}
			} else {
				pkg.LocalStatus = "new"
			}

			packages = append(packages, pkg)
		}
	}

	return packages, nil
}

// compareVersions 比较版本号
func compareVersions(v1, v2 string) int {
	// 简单的语义化版本比较
	if v1 == v2 {
		return 0
	}
	if v1 < v2 {
		return -1
	}
	return 1
}

// StartAutoUpdateChecker 启动自动更新检查器
func (s *Service) StartAutoUpdateChecker(ctx context.Context) {
	markets, err := s.marketRepo.List(ctx)
	if err != nil {
		s.logger.Error("failed to list markets for auto update", zap.Error(err))
		return
	}

	for _, market := range markets {
		if market.AutoUpdate && market.Enabled {
			s.scheduleAutoUpdate(ctx, market)
		}
	}
}

// scheduleAutoUpdate 为单个市场调度自动更新
func (s *Service) scheduleAutoUpdate(ctx context.Context, market model.Market) {
	duration, err := time.ParseDuration(market.CheckInterval)
	if err != nil {
		duration = 24 * time.Hour
	}

	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.logger.Info("auto refreshing market", zap.String("market", market.Name))
				_, err := s.RefreshMarket(ctx, market.ID)
				if err != nil {
					s.logger.Warn("auto refresh failed", zap.String("market", market.Name), zap.Error(err))
				}
			}
		}
	}()

	s.logger.Info("auto update scheduled",
		zap.String("market", market.Name),
		zap.Duration("interval", duration),
	)
}
```

需要添加 strings import。

**Step 4: Commit**

```bash
git add internal/service/market/
git commit -m "feat(service): add MarketService for market management"
```

---

## Task 5: Create Market API Handler

**Files:**
- Create: `internal/api/market_handler.go`

**Step 1: Create market_handler.go**

Create `internal/api/market_handler.go`:

```go
package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/market"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MarketHandler 市场 API 处理器
type MarketHandler struct {
	marketSvc *market.Service
	logger    *zap.Logger
}

// NewMarketHandler 创建 MarketHandler
func NewMarketHandler(marketSvc *market.Service, logger *zap.Logger) *MarketHandler {
	return &MarketHandler{
		marketSvc: marketSvc,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由
func (h *MarketHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/markets")
	g.GET("", h.ListMarkets)
	g.POST("", h.AddMarket)
	g.PUT("/:id", h.UpdateMarket)
	g.DELETE("/:id", h.DeleteMarket)
	g.POST("/:id/refresh", h.RefreshMarket)
	g.GET("/packages", h.GetTeamPackages)
}

// ListMarkets 获取市场列表
func (h *MarketHandler) ListMarkets(c *gin.Context) {
	markets, err := h.marketSvc.ListMarkets(c.Request.Context())
	if err != nil {
		h.logger.Error("list markets failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": markets, "total": len(markets)})
}

// AddMarket 添加市场
func (h *MarketHandler) AddMarket(c *gin.Context) {
	var req market.AddMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.marketSvc.AddMarket(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("add market failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

// UpdateMarket 更新市场
func (h *MarketHandler) UpdateMarket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	var req market.UpdateMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.marketSvc.UpdateMarket(c.Request.Context(), id, req)
	if err != nil {
		h.logger.Error("update market failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

// DeleteMarket 删除市场
func (h *MarketHandler) DeleteMarket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	if err := h.marketSvc.DeleteMarket(c.Request.Context(), id); err != nil {
		h.logger.Error("delete market failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "market deleted"})
}

// RefreshMarket 刷新市场
func (h *MarketHandler) RefreshMarket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	marketplace, err := h.marketSvc.RefreshMarket(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("refresh market failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "market refreshed",
		"plugins": len(marketplace.Plugins),
	})
}

// GetTeamPackages 获取所有市场的团队包
func (h *MarketHandler) GetTeamPackages(c *gin.Context) {
	packages, err := h.marketSvc.GetTeamPackages(c.Request.Context())
	if err != nil {
		h.logger.Error("get team packages failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": packages, "total": len(packages)})
}
```

**Step 2: Commit**

```bash
git add internal/api/market_handler.go
git commit -m "feat(api): add MarketHandler for market management API"
```

---

## Task 6: Modify TeamPackageSyncService to Support MarketId

**Files:**
- Modify: `internal/service/teampackagesync/service.go`
- Modify: `internal/service/teampackagesync/types.go`

**Step 1: Update SyncPackageRequest**

Edit `internal/service/teampackagesync/types.go`, add `MarketId` field:

```go
type SyncPackageRequest struct {
	PackageName string                         `json:"packageName" binding:"required"`
	MarketId    string                         `json:"marketId"`
	Confirm     *model.TeamPackageImportConfirm `json:"confirm"`
}
```

**Step 2: Update SyncService to use MarketService**

Edit `internal/service/teampackagesync/service.go`:

1. Import market service:
```go
import (
	...
	"github.com/anthropic/isdp/internal/service/market"
)
```

2. Add MarketService dependency:
```go
type SyncService struct {
	versionRepo    *repo.TeamPackageVersionRepository
	workflowRepo   *repo.WorkflowTemplateRepository
	teamPackageSvc *teampackage.Service
	marketSvc      *market.Service      // 新增
	config         config.TeamPackageSyncConfig
	gitClient      *GitClient
	logger         *zap.Logger
}
```

3. Update NewSyncService signature:
```go
func NewSyncService(
	versionRepo *repo.TeamPackageVersionRepository,
	workflowRepo *repo.WorkflowTemplateRepository,
	teamPackageSvc *teampackage.Service,
	marketSvc *market.Service,  // 新增参数
	cfg config.TeamPackageSyncConfig,
	basePath string,
	logger *zap.Logger,
) *SyncService {
	return &SyncService{
		versionRepo:    versionRepo,
		workflowRepo:   workflowRepo,
		teamPackageSvc: teamPackageSvc,
		marketSvc:      marketSvc,  // 新增
		config:         cfg,
		gitClient:      NewGitClient(cfg, basePath, logger),
		logger:         logger,
	}
}
```

4. Update SyncPackage method to accept marketId:

```go
// SyncPackage 同步指定的团队包
func (s *SyncService) SyncPackage(ctx context.Context, packageName string, marketId string, confirm *model.TeamPackageImportConfirm) (*model.ImportResult, error) {
	// 从 MarketService 获取包信息
	var remotePkg *model.MarketPackage
	var pkgPath string
	
	if marketId != "" && s.marketSvc != nil {
		// 使用市场服务获取包信息
		marketID, err := uuid.Parse(marketId)
		if err != nil {
			return nil, fmt.Errorf("invalid market id: %w", err)
		}
		
		marketplace := s.marketSvc.GetCachedMarketplace(marketID)
		if marketplace == nil {
			return nil, fmt.Errorf("market cache not found, please refresh market first")
		}
		
		// 查找指定包
		for _, plugin := range marketplace.Plugins {
			if plugin.Name == packageName && strings.ToLower(plugin.Category) == "team" {
				remotePkg = &model.MarketPackage{
					Name:        plugin.Name,
					Version:     plugin.Version,
					Repository:  plugin.Repository,
					Source:      plugin.Source,
				}
				break
			}
		}
		
		if remotePkg == nil {
			return nil, fmt.Errorf("package not found in market: %s", packageName)
		}
		
		// 克隆包仓库（repository 字段指定的仓库）
		cloneDir, err := s.gitClient.Clone(ctx)
		if err != nil {
			return nil, fmt.Errorf("clone package repo: %w", err)
		}
		defer s.gitClient.Cleanup(cloneDir)
		
		// 包路径 = 克隆目录 + source 相对路径
		pkgPath = filepath.Join(cloneDir, strings.TrimPrefix(remotePkg.Source, "./"))
	} else {
		// 保留原有逻辑（向后兼容）
		cloneDir, err := s.gitClient.Clone(ctx)
		if err != nil {
			return nil, fmt.Errorf("clone repo: %w", err)
		}
		defer s.gitClient.Cleanup(cloneDir)
		
		// ... 原有的查找逻辑
	}

	// 创建 zip
	zipData, err := s.createZipFromDir(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("create zip: %w", err)
	}

	// 调用 ImportConfirm
	result, err := s.teamPackageSvc.ImportConfirm(ctx, zipData, confirm)
	if err != nil {
		return nil, fmt.Errorf("import confirm: %w", err)
	}

	// 更新版本记录（增加 market_id 和 source_path）
	if err := s.updateVersionRecord(ctx, packageName, remotePkg, result, marketId); err != nil {
		s.logger.Warn("failed to update version record", zap.Error(err))
	}

	return result, nil
}
```

**Step 3: Commit**

```bash
git add internal/service/teampackagesync/
git commit -m "feat(sync): add marketId support to TeamPackageSyncService"
```

---

## Task 7: Update main.go to Initialize MarketService

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Add MarketService initialization**

Find the service initialization section in `cmd/server/main.go` and add:

```go
// 初始化 MarketRepository
marketRepo := repo.NewMarketRepository(db, dbType)

// 初始化 MarketService
marketSvc := market.NewService(marketRepo, versionRepo, cfg.Data.BasePath+"/temp", logger)

// 启动市场自动更新检查器
marketSvc.StartAutoUpdateChecker(ctx)
```

**Step 2: Update TeamPackageSyncService initialization**

Replace:
```go
teamPackageSyncSvc := teampackagesync.NewSyncService(versionRepo, workflowRepo, teamPackageSvc, cfg.TeamPackageSync, cfg.Data.BasePath, logger)
```

With:
```go
teamPackageSyncSvc := teampackagesync.NewSyncService(versionRepo, workflowRepo, teamPackageSvc, marketSvc, cfg.TeamPackageSync, cfg.Data.BasePath, logger)
```

**Step 3: Register MarketHandler routes**

Find the route registration section and add:

```go
// 注册市场管理 API
marketHandler := api.NewMarketHandler(marketSvc, logger)
marketHandler.RegisterRoutes(apiGroup)
```

**Step 4: Build and test**

```bash
go build -o bin/isdp-server.exe ./cmd/server
```

**Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(main): initialize MarketService and register routes"
```

---

## Task 8: Add Frontend API Client Methods

**Files:**
- Modify: `web/src/api/client.ts`
- Modify: `web/src/types/index.ts`

**Step 1: Add types**

Edit `web/src/types/index.ts`, add:

```typescript
// Market types
export interface Market {
  id: string;
  name: string;
  url: string;
  branch: string;
  enabled: boolean;
  autoUpdate: boolean;
  checkInterval: string;
  lastSyncedAt?: string;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
}

export interface MarketPackage {
  name: string;
  version: string;
  description: string;
  marketId: string;
  marketName: string;
  repository: string;
  source: string;
  localVersion?: string;
  localStatus: 'new' | 'update' | 'latest';
}

export interface AddMarketRequest {
  name: string;
  url: string;
  branch?: string;
}

export interface UpdateMarketRequest {
  name?: string;
  enabled?: boolean;
  autoUpdate?: boolean;
  checkInterval?: string;
}
```

**Step 2: Add API methods**

Edit `web/src/api/client.ts`, add markets section:

```typescript
// Market API
markets = {
  list: (): Promise<{ data: Market[]; total: number }> =>
    this.request('/markets', 'GET'),
  
  add: (req: AddMarketRequest): Promise<Market> =>
    this.request('/markets', 'POST', req),
  
  update: (id: string, req: UpdateMarketRequest): Promise<Market> =>
    this.request(`/markets/${id}`, 'PUT', req),
  
  delete: (id: string): Promise<{ message: string }> =>
    this.request(`/markets/${id}`, 'DELETE'),
  
  refresh: (id: string): Promise<{ message: string; plugins: number }> =>
    this.request(`/markets/${id}/refresh`, 'POST'),
  
  getTeamPackages: (): Promise<{ data: MarketPackage[]; total: number }> =>
    this.request('/markets/packages', 'GET'),
};
```

**Step 3: Commit**

```bash
git add web/src/api/client.ts web/src/types/index.ts
git commit -m "feat(frontend): add market API client methods and types"
```

---

## Task 9: Create Market Management Page

**Files:**
- Create: `web/src/pages/Market/MarketManagement.tsx`
- Create: `web/src/pages/Market/TeamPackages.tsx`

**Step 1: Create MarketManagement.tsx**

Create `web/src/pages/Market/MarketManagement.tsx`:

```tsx
import React, { useState, useEffect } from 'react';
import {
  Card, Table, Button, Space, Modal, Form, Input, Switch,
  Tag, message, Popconfirm, Typography, Spin
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SyncOutlined,
  ShopOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { Market, AddMarketRequest, UpdateMarketRequest } from '@/types';

const { Title, Text } = Typography;

const MarketManagement: React.FC = () => {
  const [markets, setMarkets] = useState<Market[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingMarket, setEditingMarket] = useState<Market | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    loadMarkets();
  }, []);

  const loadMarkets = async () => {
    setLoading(true);
    try {
      const result = await api.markets.list();
      setMarkets(result.data);
    } catch (error: any) {
      message.error(error.response?.data?.error || '加载市场列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingMarket(null);
    form.resetFields();
    form.setFieldsValue({ branch: 'main' });
    setModalVisible(true);
  };

  const handleEdit = (market: Market) => {
    setEditingMarket(market);
    form.setFieldsValue({
      name: market.name,
      url: market.url,
      branch: market.branch,
    });
    setModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.markets.delete(id);
      message.success('市场已删除');
      loadMarkets();
    } catch (error: any) {
      message.error(error.response?.data?.error || '删除失败');
    }
  };

  const handleRefresh = async (id: string) => {
    try {
      const result = await api.markets.refresh(id);
      message.success(`市场刷新成功，解析到 ${result.plugins} 个插件`);
      loadMarkets();
    } catch (error: any) {
      message.error(error.response?.data?.error || '刷新失败');
    }
  };

  const handleToggleEnabled = async (market: Market, enabled: boolean) => {
    try {
      await api.markets.update(market.id, { enabled });
      message.success(enabled ? '市场已启用' : '市场已禁用');
      loadMarkets();
    } catch (error: any) {
      message.error('操作失败');
    }
  };

  const handleToggleAutoUpdate = async (market: Market, autoUpdate: boolean) => {
    try {
      await api.markets.update(market.id, { autoUpdate });
      message.success(autoUpdate ? '已开启自动更新' : '已关闭自动更新');
      loadMarkets();
    } catch (error: any) {
      message.error('操作失败');
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      if (editingMarket) {
        await api.markets.update(editingMarket.id, { name: values.name });
        message.success('市场已更新');
      } else {
        const req: AddMarketRequest = {
          name: values.name,
          url: values.url,
          branch: values.branch || 'main',
        };
        await api.markets.add(req);
        message.success('市场已添加');
      }
      setModalVisible(false);
      loadMarkets();
    } catch (error: any) {
      message.error(error.response?.data?.error || '操作失败');
    }
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: 'URL',
      dataIndex: 'url',
      key: 'url',
      ellipsis: true,
      width: 300,
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'green' : 'default'}>
          {enabled ? '启用' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '自动更新',
      dataIndex: 'autoUpdate',
      key: 'autoUpdate',
      render: (autoUpdate: boolean, record: Market) => (
        <Space>
          <Switch
            size="small"
            checked={autoUpdate}
            onChange={(checked) => handleToggleAutoUpdate(record, checked)}
            disabled={!record.enabled}
          />
          <Text type="secondary">{record.checkInterval}</Text>
        </Space>
      ),
    },
    {
      title: '最后同步',
      dataIndex: 'lastSyncedAt',
      key: 'lastSyncedAt',
      render: (time?: string) => time ? new Date(time).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'action',
      width: 180,
      render: (_: any, record: Market) => (
        <Space size="small">
          <Switch
            size="small"
            checked={record.enabled}
            onChange={(checked) => handleToggleEnabled(record, checked)}
          />
          <Button
            size="small"
            icon={<SyncOutlined />}
            onClick={() => handleRefresh(record.id)}
            disabled={!record.enabled}
          >
            刷新
          </Button>
          <Button
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          />
          <Popconfirm
            title="确定删除此市场？"
            onConfirm={() => handleDelete(record.id)}
          >
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className="market-management">
      <Card
        title={
          <Space>
            <ShopOutlined />
            <span>市场管理</span>
          </Space>
        }
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加市场
          </Button>
        }
      >
        <Spin spinning={loading}>
          <Table
            dataSource={markets}
            columns={columns}
            rowKey="id"
            pagination={false}
          />
        </Spin>
      </Card>

      <Modal
        title={editingMarket ? '编辑市场' : '添加市场'}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label="市场名称"
            rules={[{ required: true, message: '请输入市场名称' }]}
          >
            <Input placeholder="如：Colink官方市场" />
          </Form.Item>
          <Form.Item
            name="url"
            label="Git仓库URL"
            rules={[{ required: true, message: '请输入Git仓库URL' }]}
          >
            <Input placeholder="https://gitee.com/xxx/marketplace.git" disabled={!!editingMarket} />
          </Form.Item>
          <Form.Item
            name="branch"
            label="分支"
          >
            <Input placeholder="main" disabled={!!editingMarket} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default MarketManagement;
```

**Step 2: Commit**

```bash
git add web/src/pages/Market/
git commit -m "feat(frontend): add MarketManagement page"
```

---

## Task 10: Create TeamPackages Page (moved from TeamPackage)

**Files:**
- Create: `web/src/pages/Market/TeamPackages.tsx`

**Step 1: Create TeamPackages.tsx**

Create `web/src/pages/Market/TeamPackages.tsx` - similar to existing TeamPackage/index.tsx but using new market API:

```tsx
import React, { useState, useEffect } from 'react';
import {
  Card, Table, Button, Space, Tag, message, Popconfirm, Typography, Spin
} from 'antd';
import {
  CloudDownloadOutlined, SyncOutlined, ShopOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { MarketPackage } from '@/types';

const { Title, Text } = Typography;

const TeamPackages: React.FC = () => {
  const [packages, setPackages] = useState<MarketPackage[]>([]);
  const [loading, setLoading] = useState(false);
  const [refreshingAll, setRefreshingAll] = useState(false);
  const [syncingPackage, setSyncingPackage] = useState<string | null>(null);

  useEffect(() => {
    loadPackages();
  }, []);

  const loadPackages = async () => {
    setLoading(true);
    try {
      const result = await api.markets.getTeamPackages();
      setPackages(result.data);
    } catch (error: any) {
      message.error(error.response?.data?.error || '加载团队包列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleRefreshAll = async () => {
    setRefreshingAll(true);
    try {
      const markets = await api.markets.list();
      for (const market of markets.data) {
        if (market.enabled) {
          await api.markets.refresh(market.id);
        }
      }
      message.success('所有市场已刷新');
      loadPackages();
    } catch (error: any) {
      message.error('刷新失败');
    } finally {
      setRefreshingAll(false);
    }
  };

  const handleSync = async (pkg: MarketPackage) => {
    setSyncingPackage(pkg.name);
    try {
      await api.teamPackages.syncPackage(pkg.name, undefined, pkg.marketId);
      message.success(`团队包 ${pkg.name} 导入成功`);
      loadPackages();
    } catch (error: any) {
      message.error(error.response?.data?.error || '导入失败');
    } finally {
      setSyncingPackage(null);
    }
  };

  const getStatusTag = (status: string) => {
    const colors: Record<string, string> = {
      new: 'blue',
      update: 'orange',
      latest: 'green',
    };
    const labels: Record<string, string> = {
      new: '未导入',
      update: '待更新',
      latest: '已导入',
    };
    return <Tag color={colors[status]}>{labels[status]}</Tag>;
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 120,
    },
    {
      title: '来源市场',
      dataIndex: 'marketName',
      key: 'marketName',
      width: 150,
    },
    {
      title: '本地版本',
      dataIndex: 'localVersion',
      key: 'localVersion',
      width: 120,
      render: (v?: string) => v || '-',
    },
    {
      title: '状态',
      dataIndex: 'localStatus',
      key: 'localStatus',
      width: 100,
      render: getStatusTag,
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_: any, record: MarketPackage) => {
        const isSyncing = syncingPackage === record.name;
        const buttonText = record.localStatus === 'new' ? '导入' :
                           record.localStatus === 'update' ? '更新' : '重新导入';
        return (
          <Popconfirm
            title={`确定要${buttonText}团队包 "${record.name}" 吗？`}
            onConfirm={() => handleSync(record)}
          >
            <Button
              type={record.localStatus === 'new' ? 'primary' : 'default'}
              size="small"
              icon={<CloudDownloadOutlined />}
              loading={isSyncing}
            >
              {buttonText}
            </Button>
          </Popconfirm>
        );
      },
    },
  ];

  return (
    <div className="team-packages">
      <Card
        title={
          <Space>
            <ShopOutlined />
            <span>远程团队包</span>
          </Space>
        }
        extra={
          <Button icon={<SyncOutlined />} onClick={handleRefreshAll} loading={refreshingAll}>
            刷新全部市场
          </Button>
        }
      >
        <Spin spinning={loading}>
          <Table
            dataSource={packages}
            columns={columns}
            rowKey={(record) => `${record.marketId}-${record.name}`}
            pagination={{ pageSize: 20 }}
          />
        </Spin>
      </Card>
    </div>
  );
};

export default TeamPackages;
```

**Step 2: Update api client to support marketId**

Edit `web/src/api/client.ts`, update syncPackage:

```typescript
syncPackage: (packageName: string, confirm?: ImportConfirm, marketId?: string): Promise<ImportResult> =>
  this.request('/team-package-sync/sync', 'POST', { packageName, confirm, marketId }),
```

**Step 3: Commit**

```bash
git add web/src/pages/Market/TeamPackages.tsx web/src/api/client.ts
git commit -m "feat(frontend): add TeamPackages page with market support"
```

---

## Task 11: Update Frontend Router and Menu

**Files:**
- Modify: `web/src/App.tsx` or router config
- Modify: `web/src/layouts/MainLayout.tsx` (menu config)

**Step 1: Add routes for Market pages**

Find the route configuration and add:

```tsx
// Market routes
<Route path="/market" element={<MarketLayout />}>
  <Route path="management" element={<MarketManagement />} />
  <Route path="team-packages" element={<TeamPackages />} />
</Route>
```

**Step 2: Add menu items**

Find the menu configuration and add new "市场" section:

```tsx
{
  key: 'market',
  icon: <ShopOutlined />,
  label: '市场',
  children: [
    { key: 'market-management', icon: <SettingOutlined />, label: '市场管理', path: '/market/management' },
    { key: 'market-packages', icon: <TeamOutlined />, label: '团队包', path: '/market/team-packages' },
  ],
},
```

**Step 3: Remove old TeamPackage route**

Remove the old route for `/agents/team-packages` since it's moved to Market section.

**Step 4: Create MarketLayout**

Create `web/src/pages/Market/Layout.tsx`:

```tsx
import React from 'react';
import { Outlet } from 'react-router-dom';

const MarketLayout: React.FC = () => {
  return <Outlet />;
};

export default MarketLayout;
```

**Step 5: Commit**

```bash
git add web/src/App.tsx web/src/layouts/ web/src/pages/Market/Layout.tsx
git commit -m "feat(frontend): add Market menu and routes"
```

---

## Task 12: Build and Test Complete Flow

**Step 1: Build backend**

```bash
cd D:/CoLinkProject/Colink-TeamsUpdate/isdp
go build -o bin/isdp-server.exe ./cmd/server
```

**Step 2: Run migration**

```bash
bin/migrate.exe up --db ./data/colink.db --version 1.2.3
```

**Step 3: Start backend**

```bash
./bin/isdp-server.exe
```

**Step 4: Build frontend**

```bash
cd web
npm run build
```

**Step 5: Test in browser**

1. Open http://localhost:26308
2. Navigate to 市场 → 市场管理
3. Add a market: `https://gitee.com/ccsunshine/marketplace.git`
4. Refresh the market
5. Navigate to 市场 → 团队包
6. Verify packages are listed with correct status
7. Test import/sync functionality

**Step 6: Final commit**

```bash
git add .
git commit -m "feat(marketplace): complete multi-market team package management"
```

---

## Summary

**Tasks Completed:**
1. ✅ Database migration for markets table
2. ✅ Market model
3. ✅ MarketRepository
4. ✅ MarketService
5. ✅ Market API Handler
6. ✅ TeamPackageSyncService marketId support
7. ✅ main.go initialization
8. ✅ Frontend API client
9. ✅ MarketManagement page
10. ✅ TeamPackages page
11. ✅ Router and menu updates
12. ✅ Build and test

**Files Created:**
- `sql-change/v1.2.3/sqlite/00004_markets.sql`
- `sql-change/v1.2.3/mysql/00004_markets.sql`
- `internal/model/market.go`
- `internal/repo/market_repo.go`
- `internal/service/market/types.go`
- `internal/service/market/service.go`
- `internal/service/market/git_client.go`
- `internal/api/market_handler.go`
- `web/src/pages/Market/Layout.tsx`
- `web/src/pages/Market/MarketManagement.tsx`
- `web/src/pages/Market/TeamPackages.tsx`

**Files Modified:**
- `sql-change/v1.2.2/sqlite/00003_team_package_versions.sql`
- `internal/service/teampackagesync/service.go`
- `internal/service/teampackagesync/types.go`
- `cmd/server/main.go`
- `web/src/api/client.ts`
- `web/src/types/index.ts`
- Router and menu config files