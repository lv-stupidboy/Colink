# 团队包页面性能优化设计文档

**日期**: 2026-04-22
**作者**: Colink需求分析师
**状态**: 待实施

---

## 1. 问题分析

### 1.1 当前性能瓶颈

| 场景 | 问题描述 | 根因分析 |
|------|----------|----------|
| 页面加载 | 打开团队包列表需要很长时间 | `GetTeamPackages` 每次克隆所有市场 Git 仓库，无缓存 |
| 批量预览 | 选择多个包导入时预览慢 | 前端串行调用 `previewPackage`，每个包都要克隆两个仓库 |
| 批量导入 | 执行导入耗时长 | 前端串行调用 `syncPackage`，无并行处理 |
| 单个导入 | 单包导入也较慢 | Git 克隆操作本身耗时 |

### 1.2 用户需求确认

- **问题范围**: 以上全部场景都需要优化
- **数据新鲜度**: 可接受几分钟的缓存延迟，需要手动刷新能力

---

## 2. 方案选择

### 2.1 考虑的方案

| 方案 | 描述 | 优点 | 缺点 |
|------|------|------|------|
| 方案一 | 内存缓存 + 并行处理 | 实现简单 | 重启丢失缓存 |
| 方案二 | 数据库缓存 + 后台同步 | 持久化 | 需新增表结构 |
| 方案三 | 混合方案（内存缓存 + 前端缓存 + 并行处理） | 平衡性能和实时性 | 前后端协同改动 |

### 2.2 选定方案

**方案三：混合方案**

理由：
1. 符合用户"可接受几分钟缓存"的需求
2. 改动适中，不影响现有架构
3. 并行处理能显著提升批量操作速度
4. 用户可通过刷新按钮获取最新数据

---

## 3. 架构设计

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        前端 (React)                              │
│  ┌──────────────────┐  ┌──────────────────┐                    │
│  │ TeamPackages.tsx │  │ localStorage缓存 │                    │
│  │ (包列表页面)     │  │ (包列表快照)     │                    │
│  └──────────────────┘  └──────────────────┘                    │
│  新增: 刷新按钮、批量API调用                                      │
└─────────────────────────────────────────────────────────────────┘
                              │ REST API
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        后端 (Go)                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                MarketService (市场服务)                   │  │
│  │  新增: 内存缓存模块                                        │  │
│  │  - 5分钟过期                                              │  │
│  │  - 手动刷新API                                            │  │
│  │  - 降级策略（使用过期缓存）                                │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              SyncService (同步服务)                       │  │
│  │  新增: 批量操作并行化                                      │  │
│  │  - PreviewPackagesBatch (并发5)                           │  │
│  │  - SyncPackagesBatch (并发3)                              │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 4. 后端详细设计

### 4.1 MarketService 内存缓存

**新增文件**: `internal/service/market/cache.go`

```go
// MarketCache 市场数据缓存
type MarketCache struct {
    data      map[uuid.UUID]*CacheEntry
    ttl       time.Duration    // 5分钟
    mutex     sync.RWMutex
}

// CacheEntry 缓存条目
type CacheEntry struct {
    Marketplace *model.Marketplace
    ExpiredAt   time.Time
}

// 方法:
// - Get(marketId) -> *Marketplace, bool (是否有效)
// - GetExpired(marketId) -> *Marketplace (允许使用过期缓存作为降级)
// - Set(marketId, marketplace)
// - Invalidate(marketId)
// - InvalidateAll()
```

**修改 GetTeamPackages**:

```go
func (s *Service) GetTeamPackages(ctx context.Context, forceRefresh bool) ([]model.MarketPackage, error) {
    // 1. 尝试从缓存读取（forceRefresh=false 时）
    if !forceRefresh {
        cached := s.cache.Get(market.ID)
        if cached != nil {
            return s.buildPackageList(cached, localVersions)
        }
    }
    
    // 2. 缓存失效或强制刷新，克隆仓库
    marketplace, err := s.RefreshMarket(ctx, market.ID)
    if err != nil {
        // 降级：尝试使用过期缓存
        cached := s.cache.GetExpired(market.ID)
        if cached != nil {
            s.logger.Warn("using expired cache", zap.Error(err))
            return s.buildPackageList(cached, localVersions)
        }
        return nil, err
    }
    
    // 3. 更新缓存
    s.cache.Set(market.ID, marketplace)
    return s.buildPackageList(marketplace, localVersions)
}
```

### 4.2 SyncService 批量操作并行化

**新增文件**: `internal/service/teampackagesync/batch.go`

```go
// PreviewPackagesBatch 批量预览（并发5）
func (s *SyncService) PreviewPackagesBatch(ctx context.Context,
    packages []PreviewRequest) (*BatchPreviewResult, error) {
    
    sem := make(chan struct{}, 5)  // 最大并发5
    results := make(chan PreviewResult, len(packages))
    
    for _, pkg := range packages {
        sem <- struct{}{}
        go func(name, marketId string) {
            defer func() { <-sem }()
            result, err := s.PreviewPackage(ctx, name, marketId)
            results <- PreviewResult{Name: name, Data: result, Error: err}
        }(pkg.Name, pkg.MarketId)
    }
    
    // 收集结果...
}

// SyncPackagesBatch 批量同步（并发3）
func (s *SyncService) SyncPackagesBatch(ctx context.Context,
    packages []SyncRequest) (*BatchSyncResult, error) {
    // 类似预览，并发限制为3
}
```

### 4.3 API 接口变更

| 接口 | 变更类型 | 说明 |
|------|----------|------|
| `GET /markets/packages` | 修改 | 新增 `forceRefresh=true` 参数 |
| `POST /markets/packages/refresh` | 新增 | 手动刷新所有市场缓存 |
| `POST /team-package-sync/preview-batch` | 新增 | 批量预览接口 |
| `POST /team-package-sync/sync-batch` | 新增 | 批量同步接口 |

---

## 5. 前端详细设计

### 5.1 localStorage 缓存

**新增文件**: `web/src/utils/teamPackageCache.ts`

```typescript
interface TeamPackageCache {
  data: MarketPackage[];
  timestamp: number;
  ttl: number;  // 300000ms (5分钟)
}

export function getCachedPackages(): MarketPackage[] | null;
export function setCachedPackages(packages: MarketPackage[]): void;
export function clearCache(): void;
```

### 5.2 刷新按钮

**位置**: TeamPackages 页面 Card extra 区域

**功能**:
- 点击刷新时清除 localStorage 缓存
- 调用 API 时带 `forceRefresh=true` 参数
- 显示 loading 状态

### 5.3 API 客户端更新

**文件**: `web/src/api/client.ts`

新增方法:
- `getTeamPackages(forceRefresh?: boolean)`
- `refreshPackages()`
- `previewPackagesBatch(packages)`
- `syncPackagesBatch(packages)`

### 5.4 批量导入流程优化

**当前流程**（串行）:
```
前端逐个调用 previewPackage → 串行克隆仓库 → 显示预览
```

**优化后流程**（并行）:
```
前端调用 previewPackagesBatch → 后端并行克隆5个仓库 → 显示预览
```

---

## 6. 预期效果

| 场景 | 当前耗时 | 优化后预期 | 提升比例 |
|------|----------|------------|----------|
| 页面加载（首次） | 10-30秒 | 10-30秒（无缓存时） | - |
| 页面加载（有缓存） | 10-30秒 | <1秒 | ~90% |
| 批量预览（5个包） | 25-50秒 | 5-10秒 | ~80% |
| 批量导入（5个包） | 25-50秒 | 10-15秒 | ~60% |

---

## 7. 风险与降级

| 风险 | 降级策略 |
|------|----------|
| Git 克隆失败 | 使用过期缓存数据，提示用户 |
| 并发过多导致资源耗尽 | 限制并发数（预览5，同步3） |
| 服务重启缓存丢失 | 首次加载慢，后续正常 |

---

## 8. 文件改动清单

### 后端新增文件
- `internal/service/market/cache.go` - 缓存模块
- `internal/service/teampackagesync/batch.go` - 批量操作

### 后端修改文件
- `internal/service/market/service.go` - 引入缓存
- `internal/api/market_handler.go` - 新增 forceRefresh 参数
- `internal/api/team_package_sync_handler.go` - 新增批量接口

### 前端新增文件
- `web/src/utils/teamPackageCache.ts` - 缓存工具

### 前端修改文件
- `web/src/api/client.ts` - 新增批量API
- `web/src/pages/Market/TeamPackages.tsx` - 刷新按钮、批量调用
- `web/src/types/index.ts` - 新增类型定义

---

## 9. 后续步骤

1. 调用 `writing-plans` 技能创建详细实施计划
2. 由后端开发工程师实现后端改动
3. 由前端开发工程师实现前端改动
4. 测试验证性能提升效果

---

**文档版本**: v1.0
**最后更新**: 2026-04-22 21:28