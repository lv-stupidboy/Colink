# 团队包市场管理设计文档

**日期**: 2026-04-18
**作者**: Claude Code
**状态**: 已批准

---

## 1. 概述

### 1.1 背景

当前团队包同步服务仅支持单个远程仓库，用户无法灵活管理多个市场来源。需要优化为支持多市场管理，每个市场可独立配置自动更新策略。

### 1.2 目标

- 支持多市场（Git URL）管理
- 每个市场独立配置自动更新开关和检查间隔
- 解析 marketplace.json，筛选 category=team 的团队包
- 同名包展示所有市场版本供用户选择
- 缓存市场数据，支持手动刷新

### 1.3 设计原则

- **最小侵入原则**：尽量复用现有代码，减少对现有服务的改动
- **职责分离**：新增独立 MarketService 层，TeamPackageSyncService 依赖它

---

## 2. 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                    Frontend (React)                          │
│  ┌─────────────────┐  ┌─────────────────────────────────────┐│
│  │ 市场管理页面      │  │ 团队包页面（整合市场数据）           ││
│  │ - 添加/删除市场  │  │ - 显示所有市场的团队包              ││
│  │ - 配置自动更新  │  │ - 版本对比 + 用户选择市场更新       ││
│  │ - 手动刷新     │  │ - 手动刷新市场缓存                 ││
│  └─────────────────┘  └─────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼ REST API
┌─────────────────────────────────────────────────────────────┐
│                    Backend (Go + Gin)                        │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              MarketService (新增)                    │    │
│  │  - 管理 Market 配置（持久化到数据库）                 │    │
│  │  - 克隆 Git 仓库，解析 marketplace.json              │    │
│  │  - 缓存市场数据（内存 + 定时刷新）                   │    │
│  │  - 按市场独立配置自动更新检查间隔                    │    │
│  └─────────────────────────────────────────────────────┘    │
│                              │                               │
│                              ▼                               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │         TeamPackageSyncService (改造)               │    │
│  │  - 从 MarketService 获取市场数据                     │    │
│  │  - 与本地 team_package_versions 对比版本            │    │
│  │  - 执行同步导入（复用现有 ImportConfirm）            │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │         MarketRepository (新增)                      │    │
│  │  - markets 表：存储市场配置                          │    │
│  │  - market_packages 表：缓存市场包元数据              │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. 数据模型

### 3.1 新增数据库表

```sql
-- markets 表：市场配置
CREATE TABLE markets (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,           -- 市场名称（用户自定义）
    url VARCHAR(500) NOT NULL,            -- Git 仓库 URL
    branch VARCHAR(100) DEFAULT 'main',   -- 分支
    enabled BOOLEAN DEFAULT true,         -- 是否启用
    auto_update BOOLEAN DEFAULT false,    -- 是否自动更新
    check_interval VARCHAR(20) DEFAULT '24h', -- 检查间隔
    last_synced_at DATETIME,              -- 最后同步时间
    cache_data TEXT,                      -- 缓存的 marketplace.json（压缩存储）
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 扩展现有 team_package_versions 表
ALTER TABLE team_package_versions ADD COLUMN market_id UUID;  -- 标记来源市场
ALTER TABLE team_package_versions ADD COLUMN source_path VARCHAR(500); -- 原始路径
```

### 3.2 marketplace.json 解析模型

```json
{
    "name": "colink-marketplace",
    "version": "1.0.0",
    "description": "colink marketplace",
    "owner": { "name": "colink" },
    "plugins": [
        {
            "name": "CoreDev全流程开发团队",
            "description": "...",
            "version": "1.0.0",
            "repository": "https://gitee.com/ccsunshine/only-my-teams",
            "source": "./dev/CoreDev全流程开发团队",
            "category": "team"
        }
    ]
}
```

```go
type Marketplace struct {
    Name        string    `json:"name"`
    Version     string    `json:"version"`
    Description string    `json:"description"`
    Plugins     []Plugin  `json:"plugins"`
}

type Plugin struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Version     string `json:"version"`
    Repository  string `json:"repository"`  -- 包所在仓库
    Source      string `json:"source"`      -- 相对路径
    Category    string `json:"category"`    -- team/skill/command 等
}
```

### 3.3 前端展示模型

```typescript
interface MarketInfo {
  id: string;
  name: string;
  url: string;
  branch: string;
  enabled: boolean;
  autoUpdate: boolean;
  checkInterval: string;
  lastSyncedAt?: string;
  status?: 'syncing' | 'success' | 'failed';  // 同步状态
}

interface MarketPackage {
  name: string;
  version: string;
  description: string;
  marketId: string;
  marketName: string;
  repository: string;
  source: string;
  localVersion?: string;      -- 本地版本（用于对比）
  localStatus?: 'new' | 'update' | 'latest';  -- 状态
}
```

---

## 4. 服务层设计

### 4.1 MarketService（新增）

```go
type MarketService struct {
    marketRepo    *MarketRepository
    gitClient     *GitClient           -- 复用现有 GitClient
    cache         map[string]*MarketCache  -- 内存缓存
    logger        *zap.Logger
}

// 核心方法
func (s *MarketService) ListMarkets() ([]Market, error)
func (s *MarketService) AddMarket(url, name, branch string) (*Market, error)
func (s *MarketService) UpdateMarket(id string, config MarketConfig) error
func (s *MarketService) DeleteMarket(id string) error
func (s *MarketService) RefreshMarket(id string) (*Marketplace, error)  -- 手动刷新
func (s *MarketService) GetTeamPackages() ([]MarketPackage, error)     -- 获取所有市场的 team 包
func (s *MarketService) StartAutoUpdateChecker()                       -- 启动定时检查
```

### 4.2 TeamPackageSyncService（最小改动）

```go
type SyncService struct {
    versionRepo    *repo.TeamPackageVersionRepository
    workflowRepo   *repo.WorkflowTemplateRepository
    teamPackageSvc *teampackage.Service
    marketSvc      *MarketService       -- 新增依赖
    logger         *zap.Logger
}

// 改动：SyncPackage 增加 marketId 参数
func (s *SyncService) SyncPackage(ctx context.Context, packageName string, marketId string, confirm *ImportConfirm) (*ImportResult, error)
```

### 4.3 复用组件

- `GitClient` - 直接复用，克隆逻辑不变
- `teampackage.Service.ImportConfirm` - 完全复用，零侵入
- 解析逻辑：从 marketplace.json 的 source 字段读取包路径，调用现有打包逻辑

### 4.4 自动更新调度器

```go
func (s *MarketService) scheduleAutoUpdate(market *Market) {
    go func() {
        for {
            if market.AutoUpdate && market.Enabled {
                s.RefreshMarket(market.ID)
                s.notifyTeamPackageSync()  -- 触发 SyncService 检查更新
            }
            time.Sleep(market.GetCheckInterval())
        }
    }()
}
```

---

## 5. API 设计

### 5.1 新增市场管理 API

```
GET    /api/v1/markets                    -- 获取市场列表
POST   /api/v1/markets                    -- 添加市场
PUT    /api/v1/markets/:id                -- 更新市场配置
DELETE /api/v1/markets/:id                -- 删除市场
POST   /api/v1/markets/:id/refresh        -- 手动刷新市场缓存
GET    /api/v1/markets/packages           -- 获取所有市场的团队包列表
```

### 5.2 改造现有 API

```
POST   /api/v1/team-package-sync/sync     -- 参数增加 marketId
GET    /api/v1/team-package-sync/check-update    -- 改为从所有市场检查更新
GET    /api/v1/team-package-sync/local-versions  -- 保持不变
```

### 5.3 请求/响应示例

```typescript
// 添加市场
POST /api/v1/markets
{
  "name": "Colink官方市场",
  "url": "https://gitee.com/ccsunshine/marketplace.git",
  "branch": "main"
}

// 团队包列表响应
GET /api/v1/markets/packages
{
  "data": [
    {
      "name": "CoreDev全流程开发团队",
      "version": "1.0.0",
      "description": "...",
      "marketId": "uuid-1",
      "marketName": "Colink官方市场",
      "repository": "https://gitee.com/ccsunshine/only-my-teams",
      "source": "./dev/CoreDev全流程开发团队",
      "localVersion": null,
      "localStatus": "new"
    },
    {
      "name": "CoreDev全流程开发团队",
      "version": "2.0.0",
      "marketId": "uuid-2",
      "marketName": "企业私有市场",
      "localVersion": "1.0.0",
      "localStatus": "update"
    }
  ],
  "total": 5
}

// 同步请求
POST /api/v1/team-package-sync/sync
{
  "packageName": "CoreDev全流程开发团队",
  "marketId": "uuid-2",
  "confirm": { "mode": "overwrite", ... }
}
```

---

## 6. 前端 UI 设计

### 6.1 菜单结构

```
设置
├── 通用设置
├── 基础Agent设置
├── 联邦技能源

市场（新增一级菜单）
├── 市场管理（管理市场 URL、自动更新配置）
├── 团队包（展示所有市场的团队包，对比版本，执行导入/更新）
```

### 6.2 市场管理页面

```
┌─────────────────────────────────────────────────────────┐
│ 市场管理                                                 │
├─────────────────────────────────────────────────────────┤
│ [+ 添加市场]                                             │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ 名称          │ URL             │ 状态 │ 自动更新 │ 操作 │ │
│ │ Colink官方    │ gitee.com/...   │ 启用 │ ✅ 24h  │ 编辑 删除 刷新 │ │
│ │ 企业私有      │ github.com/...  │ 禁用 │ ❌      │ 编辑 删除 刷新 │ │
│ └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### 6.3 团队包页面

```
┌─────────────────────────────────────────────────────────┐
│ 远程团队包                               [刷新全部市场]  │
├─────────────────────────────────────────────────────────┤
│ 名称            │ 版本  │ 来源市场    │ 本地状态 │ 操作 │
│ CoreDev全流程   │ 2.0.0 │ 企业私有    │ 待更新   │ [更新]│
│                 │ 1.0.0 │ Colink官方  │          │ [导入]│
│ SuperPowers团队 │ 1.0.0 │ Colink官方  │ 未导入   │ [导入]│
│ 团队组建工作室  │ 1.0.0 │ Colink官方  │ 已导入   │ [重新导入]│
└─────────────────────────────────────────────────────────┘
```

### 6.4 交互细节

- 同名包从不同市场各展示一行，用户自主选择
- 版本对比基于语义化版本号
- 点击"刷新全部市场"调用每个启用市场的 refresh API
- 导入/更新按钮复用现有 Popconfirm 确认流程

---

## 7. 错误处理

| 场景 | 处理方式 |
|------|---------|
| Git 克隆失败 | 返回错误信息，标记市场状态为"同步失败" |
| marketplace.json 不存在 | 返回错误，提示用户 |
| marketplace.json 格式错误 | 返回解析错误详情 |
| plugins 字段为空 | 正常返回空列表 |
| category=team 的包为空 | 正常返回空列表 |
| 市场被禁用 | 不参与自动更新，不展示包 |
| 同步包时 marketId 无效 | 返回错误"市场不存在或已禁用" |
| 并发刷新同一市场 | 加锁防止重复克隆 |
| 删除市场时本地有包 | 提示用户不影响已导入包 |

---

## 8. 实施步骤

1. **后端：数据层** - 创建 markets 表迁移脚本
2. **后端：MarketRepository** - 实现市场 CRUD 操作
3. **后端：MarketService** - 实现核心服务方法
4. **后端：API Handler** - 新增市场管理 API
5. **后端：改造 TeamPackageSyncService** - 添加 marketId 参数支持
6. **前端：路由与菜单** - 新增"市场"一级菜单
7. **前端：市场管理页面** - 实现市场 CRUD UI
8. **前端：改造团队包页面** - 改用新 API
9. **后端：自动更新调度器** - 实现按市场独立定时检查
10. **测试验证** - 完整流程测试