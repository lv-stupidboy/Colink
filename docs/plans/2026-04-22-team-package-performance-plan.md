# Team Package Performance Optimization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Optimize team package page loading and import performance through caching and parallel processing.

**Architecture:** Backend adds memory cache for marketplace data (5-minute TTL) and parallel batch operations; frontend adds localStorage cache and refresh button.

**Tech Stack:** Go (Gin), React (Ant Design), TypeScript

---

## Task 1: Backend - Create Market Cache Module

**Files:**
- Create: `internal/service/market/cache.go`

**Step 1: Create the cache module file**

Create file `internal/service/market/cache.go`:

```go
package market

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/anthropic/isdp/internal/model"
)

// MarketCache 市场数据缓存
type MarketCache struct {
	data    map[uuid.UUID]*CacheEntry
	ttl     time.Duration
	mutex   sync.RWMutex
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Marketplace *model.Marketplace
	ExpiredAt   time.Time
}

// NewMarketCache 创建缓存实例
func NewMarketCache(ttl time.Duration) *MarketCache {
	return &MarketCache{
		data: make(map[uuid.UUID]*CacheEntry),
		ttl:  ttl,
	}
}

// Get 获取有效缓存
func (c *MarketCache) Get(marketId uuid.UUID) *model.Marketplace {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	entry, exists := c.data[marketId]
	if !exists || time.Now().After(entry.ExpiredAt) {
		return nil
	}
	return entry.Marketplace
}

// GetExpired 获取过期缓存（用于降级）
func (c *MarketCache) GetExpired(marketId uuid.UUID) *model.Marketplace {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	entry, exists := c.data[marketId]
	if !exists {
		return nil
	}
	return entry.Marketplace
}

// Set 设置缓存
func (c *MarketCache) Set(marketId uuid.UUID, marketplace *model.Marketplace) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.data[marketId] = &CacheEntry{
		Marketplace: marketplace,
		ExpiredAt:   time.Now().Add(c.ttl),
	}
}

// Invalidate 清除指定市场缓存
func (c *MarketCache) Invalidate(marketId uuid.UUID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	delete(c.data, marketId)
}

// InvalidateAll 清除所有缓存
func (c *MarketCache) InvalidateAll() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.data = make(map[uuid.UUID]*CacheEntry)
}
```

**Step 2: Verify file compiles**

Run: `go build ./internal/service/market/`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/service/market/cache.go
git commit -m "feat: add market cache module for performance optimization"
```

---

## Task 2: Backend - Integrate Cache into MarketService

**Files:**
- Modify: `internal/service/market/service.go`

**Step 1: Add cache field to Service struct**

Open `internal/service/market/service.go`, add cache field:

```go
// Service 市场管理服务
type Service struct {
	marketRepo  *repo.MarketRepository
	versionRepo *repo.TeamPackageVersionRepository
	gitClient   *GitClient
	tempBase    string
	logger      *zap.Logger
	cache       *MarketCache  // 新增：缓存模块
}
```

**Step 2: Initialize cache in NewService**

Modify NewService:

```go
// NewService 创建市场服务
func NewService(
	marketRepo *repo.MarketRepository,
	versionRepo *repo.TeamPackageVersionRepository,
	tempBase string,
	logger *zap.Logger,
) *Service {
	return &Service{
		marketRepo:  marketRepo,
		versionRepo: versionRepo,
		gitClient:   NewGitClient(logger),
		tempBase:    tempBase,
		logger:      logger,
		cache:       NewMarketCache(5 * time.Minute),  // 5分钟缓存
	}
}
```

**Step 3: Add time import**

Ensure `time` is imported in the file.

**Step 4: Verify file compiles**

Run: `go build ./internal/service/market/`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/service/market/service.go
git commit -m "feat: integrate cache into MarketService"
```

---

## Task 3: Backend - Modify GetTeamPackages with Cache Logic

**Files:**
- Modify: `internal/service/market/service.go` (GetTeamPackages method)

**Step 1: Add forceRefresh parameter and cache logic**

Replace `GetTeamPackages` method:

```go
// GetTeamPackages 获取所有市场的团队包列表
func (s *Service) GetTeamPackages(ctx context.Context, forceRefresh bool) ([]model.MarketPackage, error) {
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
	localMap := make(map[string]model.TeamPackageVersion)
	for _, v := range localVersions {
		localMap[v.Name] = v
	}

	packages := []model.MarketPackage{}

	for _, market := range markets {
		if !market.Enabled {
			continue
		}

		var marketplace *model.Marketplace

		// 尝试从缓存读取（非强制刷新时）
		if !forceRefresh {
			cached := s.cache.Get(market.ID)
			if cached != nil {
				marketplace = cached
			}
		}

		// 缓存失效或强制刷新，克隆仓库
		if marketplace == nil {
			marketplace, err = s.RefreshMarket(ctx, market.ID)
			if err != nil {
				// 降级：尝试使用过期缓存
				cached := s.cache.GetExpired(market.ID)
				if cached != nil {
					s.logger.Warn("using expired cache due to refresh failure",
						zap.String("market", market.Name),
						zap.Error(err))
					marketplace = cached
				} else {
					s.logger.Warn("failed to refresh market",
						zap.String("market", market.Name),
						zap.Error(err))
					continue
				}
			}
			// 更新缓存
			s.cache.Set(market.ID, marketplace)
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
				pkg.LocalVersion = localVer.Version
				pkg.LastImportedAt = localVer.LastSyncedAt
				if compareVersions(localVer.Version, plugin.Version) < 0 {
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
```

**Step 2: Verify file compiles**

Run: `go build ./internal/service/market/`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/service/market/service.go
git commit -m "feat: add cache logic to GetTeamPackages with forceRefresh param"
```

---

## Task 4: Backend - Add RefreshPackages Method

**Files:**
- Modify: `internal/service/market/service.go`

**Step 1: Add RefreshPackages method**

Add new method to Service:

```go
// RefreshPackages 手动刷新所有市场缓存
func (s *Service) RefreshPackages(ctx context.Context) error {
	s.cache.InvalidateAll()
	
	markets, err := s.marketRepo.List(ctx)
	if err != nil {
		return err
	}

	for _, market := range markets {
		if !market.Enabled {
			continue
		}
		marketplace, err := s.RefreshMarket(ctx, market.ID)
		if err != nil {
			s.logger.Warn("failed to refresh market",
				zap.String("market", market.Name),
				zap.Error(err))
			continue
		}
		s.cache.Set(market.ID, marketplace)
	}

	return nil
}
```

**Step 2: Verify file compiles**

Run: `go build ./internal/service/market/`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/service/market/service.go
git commit -m "feat: add RefreshPackages method for manual cache refresh"
```

---

## Task 5: Backend - Update Market Handler

**Files:**
- Modify: `internal/api/market_handler.go`

**Step 1: Add forceRefresh parameter to GetTeamPackages handler**

Modify `GetTeamPackages` handler:

```go
// GetTeamPackages 获取所有市场的团队包
func (h *MarketHandler) GetTeamPackages(c *gin.Context) {
	// 解析 forceRefresh 参数
	forceRefresh := c.Query("forceRefresh") == "true"
	
	packages, err := h.marketSvc.GetTeamPackages(c.Request.Context(), forceRefresh)
	if err != nil {
		h.logger.Error("get team packages failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": packages, "total": len(packages)})
}
```

**Step 2: Add RefreshPackages handler**

Add new handler:

```go
// RefreshPackages 手动刷新所有市场缓存
func (h *MarketHandler) RefreshPackages(c *gin.Context) {
	if err := h.marketSvc.RefreshPackages(c.Request.Context()); err != nil {
		h.logger.Error("refresh packages failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "packages refreshed"})
}
```

**Step 3: Register new route**

Add route in `RegisterRoutes`:

```go
func (h *MarketHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/markets")
	g.GET("", h.ListMarkets)
	g.GET("/default", h.GetDefaultMarketConfig)
	g.POST("", h.AddMarket)
	g.POST("/default", h.AddDefaultMarket)
	g.PUT("/:id", h.UpdateMarket)
	g.DELETE("/:id", h.DeleteMarket)
	g.POST("/:id/refresh", h.RefreshMarket)
	g.GET("/packages", h.GetTeamPackages)
	g.POST("/packages/refresh", h.RefreshPackages)  // 新增
}
```

**Step 4: Verify file compiles**

Run: `go build ./internal/api/`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/api/market_handler.go
git commit -m "feat: add forceRefresh param and RefreshPackages endpoint"
```

---

## Task 6: Backend - Create Batch Preview Module

**Files:**
- Create: `internal/service/teampackagesync/batch.go`

**Step 1: Create batch operations file**

Create file `internal/service/teampackagesync/batch.go`:

```go
package teampackagesync

import (
	"context"
	"sync"

	"github.com/anthropic/isdp/internal/model"
)

// PreviewRequest 批量预览请求
type PreviewRequest struct {
	Name     string
	MarketId string
}

// PreviewResult 单个预览结果
type PreviewResult struct {
	Name  string
	Data  *PreviewPackageResponse
	Error error
}

// BatchPreviewResult 批量预览结果
type BatchPreviewResult struct {
	Previews       []PreviewResult
	TotalConflicts int
	SuccessCount   int
	FailedCount    int
}

// PreviewPackagesBatch 批量预览团队包（并行）
func (s *SyncService) PreviewPackagesBatch(ctx context.Context,
	requests []PreviewRequest) (*BatchPreviewResult, error) {

	maxConcurrency := 5
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	results := make([]PreviewResult, len(requests))
	totalConflicts := 0
	successCount := 0
	failedCount := 0

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, name, marketId string) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := s.PreviewPackage(ctx, name, marketId)
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
		}(i, req.Name, req.MarketId)
	}

	wg.Wait()

	return &BatchPreviewResult{
		Previews:       results,
		TotalConflicts: totalConflicts,
		SuccessCount:   successCount,
		FailedCount:    failedCount,
	}, nil
}

// SyncRequest 批量同步请求
type SyncRequest struct {
	Name     string
	MarketId string
	Confirm  *model.TeamPackageImportConfirm
}

// SyncResult 单个同步结果
type SyncResult struct {
	Name  string
	Data  *model.ImportResult
	Error error
}

// BatchSyncResult 批量同步结果
type BatchSyncResult struct {
	Results     []SyncResult
	SuccessCount int
	FailedCount  int
}

// SyncPackagesBatch 批量同步团队包（并行）
func (s *SyncService) SyncPackagesBatch(ctx context.Context,
	requests []SyncRequest) (*BatchSyncResult, error) {

	maxConcurrency := 3
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	results := make([]SyncResult, len(requests))
	successCount := 0
	failedCount := 0

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, name, marketId string, confirm *model.TeamPackageImportConfirm) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := s.SyncPackage(ctx, name, marketId, confirm)
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
		}(i, req.Name, req.MarketId, req.Confirm)
	}

	wg.Wait()

	return &BatchSyncResult{
		Results:      results,
		SuccessCount: successCount,
		FailedCount:  failedCount,
	}, nil
}
```

**Step 2: Verify file compiles**

Run: `go build ./internal/service/teampackagesync/`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/service/teampackagesync/batch.go
git commit -m "feat: add batch preview and sync operations with parallel processing"
```

---

## Task 7: Backend - Add Batch API Handlers

**Files:**
- Modify: `internal/api/team_package_sync_handler.go`

**Step 1: Add batch preview handler**

Add handlers to TeamPackageSyncHandler:

```go
// PreviewPackagesBatch 批量预览团队包
func (h *TeamPackageSyncHandler) PreviewPackagesBatch(c *gin.Context) {
	var req struct {
		Packages []struct {
			Name     string `json:"name"`
			MarketId string `json:"marketId"`
		} `json:"packages"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 构建请求
	requests := make([]teampackagesync.PreviewRequest, len(req.Packages))
	for i, p := range req.Packages {
		requests[i] = teampackagesync.PreviewRequest{
			Name:     p.Name,
			MarketId: p.MarketId,
		}
	}

	result, err := h.syncSvc.PreviewPackagesBatch(c.Request.Context(), requests)
	if err != nil {
		h.logger.Error("batch preview failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"previews":       result.Previews,
		"totalConflicts": result.TotalConflicts,
		"successCount":   result.SuccessCount,
		"failedCount":    result.FailedCount,
	})
}

// SyncPackagesBatch 批量同步团队包
func (h *TeamPackageSyncHandler) SyncPackagesBatch(c *gin.Context) {
	var req struct {
		Packages []struct {
			Name     string                   `json:"name"`
			MarketId string                   `json:"marketId"`
			Confirm  *model.TeamPackageImportConfirm `json:"confirm"`
		} `json:"packages"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 构建请求
	requests := make([]teampackagesync.SyncRequest, len(req.Packages))
	for i, p := range req.Packages {
		requests[i] = teampackagesync.SyncRequest{
			Name:     p.Name,
			MarketId: p.MarketId,
			Confirm:  p.Confirm,
		}
	}

	result, err := h.syncSvc.SyncPackagesBatch(c.Request.Context(), requests)
	if err != nil {
		h.logger.Error("batch sync failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results":      result.Results,
		"successCount": result.SuccessCount,
		"failedCount":  result.FailedCount,
	})
}
```

**Step 2: Register new routes**

Add routes in RegisterRoutes:

```go
func (h *TeamPackageSyncHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/team-package-sync")
	g.GET("/remote", h.GetRemotePackages)
	g.GET("/check-update", h.CheckUpdates)
	g.POST("/preview", h.PreviewPackage)
	g.POST("/preview-batch", h.PreviewPackagesBatch)  // 新增
	g.POST("/sync", h.SyncPackage)
	g.POST("/sync-batch", h.SyncPackagesBatch)        // 新增
	g.GET("/local-versions", h.GetLocalVersions)
}
```

**Step 3: Add required imports**

Ensure imports include:
- `teampackagesync "github.com/anthropic/isdp/internal/service/teampackagesync"`
- `"github.com/anthropic/isdp/internal/model"`

**Step 4: Verify file compiles**

Run: `go build ./internal/api/`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/api/team_package_sync_handler.go
git commit -m "feat: add batch preview and sync API endpoints"
```

---

## Task 8: Backend - Full Build Verification

**Step 1: Run full backend build**

Run: `make build`
Expected: `bin/isdp-server.exe` created successfully

**Step 2: Commit (if any fixes needed)**

If fixes were needed:
```bash
git add -A
git commit -m "fix: resolve build issues for performance optimization"
```

---

## Task 9: Frontend - Create TeamPackage Cache Utility

**Files:**
- Create: `web/src/utils/teamPackageCache.ts`

**Step 1: Create cache utility file**

Create file `web/src/utils/teamPackageCache.ts`:

```typescript
import type { MarketPackage } from '@/types';

const CACHE_KEY = 'team-packages-cache';
const CACHE_TTL = 5 * 60 * 1000; // 5分钟

interface TeamPackageCache {
  data: MarketPackage[];
  timestamp: number;
}

/**
 * 获取缓存数据（过期返回null）
 */
export function getCachedPackages(): MarketPackage[] | null {
  try {
    const cached = localStorage.getItem(CACHE_KEY);
    if (!cached) return null;

    const cacheData: TeamPackageCache = JSON.parse(cached);
    const now = Date.now();

    if (now - cacheData.timestamp > CACHE_TTL) {
      return null;
    }

    return cacheData.data;
  } catch {
    return null;
  }
}

/**
 * 设置缓存数据
 */
export function setCachedPackages(packages: MarketPackage[]): void {
  try {
    const cacheData: TeamPackageCache = {
      data: packages,
      timestamp: Date.now(),
    };
    localStorage.setItem(CACHE_KEY, JSON.stringify(cacheData));
  } catch {
    // localStorage 可能已满或不可用
  }
}

/**
 * 清除缓存
 */
export function clearCache(): void {
  localStorage.removeItem(CACHE_KEY);
}
```

**Step 2: Verify TypeScript compiles**

Run: `cd web && npm run build`
Expected: No TypeScript errors

**Step 3: Commit**

```bash
git add web/src/utils/teamPackageCache.ts
git commit -m "feat: add localStorage cache utility for team packages"
```

---

## Task 10: Frontend - Update API Client

**Files:**
- Modify: `web/src/api/client.ts`

**Step 1: Add forceRefresh parameter to getTeamPackages**

Modify `markets.getTeamPackages`:

```typescript
markets = {
  // ...existing methods...
  
  getTeamPackages: (forceRefresh?: boolean): Promise<{ data: MarketPackage[]; total: number }> => {
    const url = forceRefresh 
      ? '/markets/packages?forceRefresh=true' 
      : '/markets/packages';
    return this.request(url, 'GET');
  },
  
  // 新增：手动刷新
  refreshPackages: (): Promise<{ message: string }> =>
    this.request('/markets/packages/refresh', 'POST'),
  
  // 新增：批量预览
  previewPackagesBatch: (packages: Array<{ name: string; marketId: string }>): Promise<{
    previews: Array<{ name: string; data: PackagePreviewResponse; error?: string }>;
    totalConflicts: number;
    successCount: number;
    failedCount: number;
  }> =>
    this.request('/team-package-sync/preview-batch', 'POST', { packages }),
  
  // 新增：批量同步
  syncPackagesBatch: (packages: Array<{
    name: string;
    marketId: string;
    confirm?: ImportConfirm;
  }>): Promise<{
    results: Array<{ name: string; data: ImportResult; error?: string }>;
    successCount: number;
    failedCount: number;
  }> =>
    this.request('/team-package-sync/sync-batch', 'POST', { packages }),
};
```

**Step 2: Add PackagePreviewResponse import if needed**

Ensure type is imported.

**Step 3: Verify TypeScript compiles**

Run: `cd web && npm run build`
Expected: No TypeScript errors

**Step 4: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat: add batch API methods and forceRefresh param"
```

---

## Task 11: Frontend - Update TeamPackages Page

**Files:**
- Modify: `web/src/pages/Market/TeamPackages.tsx`

**Step 1: Import cache utility**

Add import:
```typescript
import { getCachedPackages, setCachedPackages, clearCache } from '@/utils/teamPackageCache';
import { ReloadOutlined } from '@ant-design/icons';
```

**Step 2: Add refresh state and button**

Add state and modify loadPackages:

```typescript
// 新增刷新状态
const [refreshing, setRefreshing] = useState(false);

// 修改 loadPackages 使用缓存
const loadPackages = async (forceRefresh = false) => {
  // 非强制刷新时，先尝试读取缓存
  if (!forceRefresh) {
    const cached = getCachedPackages();
    if (cached && cached.length > 0) {
      setPackages(cached);
      setLoading(false);
      return;
    }
  }
  
  setLoading(true);
  try {
    const result = await api.markets.getTeamPackages(forceRefresh);
    setPackages(result.data);
    setCachedPackages(result.data);  // 写入缓存
    setSelectedRowKeys([]);
  } catch (error: any) {
    message.error(error.response?.data?.error || '加载团队包列表失败');
  } finally {
    setLoading(false);
  }
};

// 新增刷新处理函数
const handleRefresh = async () => {
  setRefreshing(true);
  clearCache();
  await loadPackages(true);
  setRefreshing(false);
};
```

**Step 3: Add refresh button to Card extra**

Modify the Card's extra prop:

```typescript
extra={
  <Space>
    <Button
      icon={<ReloadOutlined />}
      loading={refreshing}
      onClick={handleRefresh}
    >
      刷新
    </Button>
    <Text type="secondary">
      已选 {selectedRowKeys.length} 项
    </Text>
    <Button
      type="primary"
      icon={<CheckSquareOutlined />}
      onClick={handleBatchImportClick}
      disabled={selectedRowKeys.length === 0 || batchImporting}
      loading={batchImporting}
    >
      批量导入
    </Button>
  </Space>
}
```

**Step 4: Modify handleBatchImportClick to use batch API**

Replace the for-loop preview logic:

```typescript
const handleBatchImportClick = async () => {
  if (selectedRowKeys.length === 0) {
    message.warning('请先选择要导入的团队包');
    return;
  }

  const toImport = packages.filter(pkg =>
    selectedRowKeys.includes(`${pkg.marketId}-${pkg.name}`)
  );
  setPendingImportPackages(toImport);
  setLoadingBatchPreview(true);

  try {
    // 使用批量预览API（并行处理）
    const result = await api.markets.previewPackagesBatch(
      toImport.map(pkg => ({ name: pkg.name, marketId: pkg.marketId }))
    );

    // 构建预览Map
    const previewMap = new Map<string, PackagePreviewResponse>();
    result.previews.forEach(p => {
      if (p.data) {
        previewMap.set(p.name, p.data);
      }
    });

    setBatchPreviewData(previewMap);
    setBatchConflictTotal(result.totalConflicts);
  } catch (error: any) {
    message.error(error.response?.data?.error || '批量预览失败');
  } finally {
    setLoadingBatchPreview(false);
    setConfirmModalVisible(true);
  }
};
```

**Step 5: Verify TypeScript compiles**

Run: `cd web && npm run build`
Expected: No TypeScript errors

**Step 6: Commit**

```bash
git add web/src/pages/Market/TeamPackages.tsx
git commit -m "feat: add refresh button and use batch preview API"
```

---

## Task 12: Frontend - Full Build Verification

**Step 1: Run frontend build**

Run: `cd web && npm run build`
Expected: Build succeeds without errors

**Step 2: Commit (if any fixes needed)**

If fixes were needed:
```bash
git add -A
git commit -m "fix: resolve frontend build issues"
```

---

## Task 13: Integration Testing

**Step 1: Start backend server**

Run: `go run ./cmd/server`
Expected: Server starts on port 26305

**Step 2: Start frontend dev server**

Run: `cd web && npm run dev`
Expected: Dev server starts on port 26306

**Step 3: Test scenarios**

1. **Page load with cache**: Open team packages page, verify instant load from localStorage
2. **Refresh button**: Click refresh, verify it clears cache and fetches fresh data
3. **Batch preview**: Select 3+ packages, click batch import, verify parallel preview (should be faster)
4. **Force refresh API**: Call `/markets/packages?forceRefresh=true`, verify it bypasses cache

**Step 4: Document results**

If issues found, create follow-up tasks.

**Step 5: Final commit**

```bash
git add -A
git commit -m "feat: team package performance optimization complete"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Create cache module | `internal/service/market/cache.go` |
| 2 | Integrate cache into Service | `internal/service/market/service.go` |
| 3 | Modify GetTeamPackages | `internal/service/market/service.go` |
| 4 | Add RefreshPackages method | `internal/service/market/service.go` |
| 5 | Update market handler | `internal/api/market_handler.go` |
| 6 | Create batch operations | `internal/service/teampackagesync/batch.go` |
| 7 | Add batch API handlers | `internal/api/team_package_sync_handler.go` |
| 8 | Backend build verification | Full build |
| 9 | Create frontend cache utility | `web/src/utils/teamPackageCache.ts` |
| 10 | Update API client | `web/src/api/client.ts` |
| 11 | Update TeamPackages page | `web/src/pages/Market/TeamPackages.tsx` |
| 12 | Frontend build verification | Full build |
| 13 | Integration testing | Manual testing |

---

**Plan complete and saved to `docs/plans/2026-04-22-team-package-performance-plan.md`.**

**Two execution options:**

1. **Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

2. **Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

**Which approach?**