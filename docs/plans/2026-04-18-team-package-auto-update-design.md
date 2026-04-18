# 团队包自动更新功能设计文档

## 概述

为 ISDP 平台增加团队包自动更新能力，支持从远程 Git 仓库获取团队包，通过版本对比判断是否需要更新，在通用设置中提供开关和版本展示。

## 需求总结

- **远程仓库**：从配置的 Git 仓库（默认 https://gitee.com/colink_1/isdp.git）获取团队包
- **目录结构**：根目录为团队分类文件夹，每个分类下有多个团队包文件夹
- **版本追踪**：按团队包名称匹配，记录本地版本号与远程对比
- **触发时机**：启动时自动检查 + 定时自动检查（可配置间隔）
- **首次体验**：首次检测到更新时弹窗询问用户
- **用户控制**：通用设置中提供开关、版本展示、手动检查按钮
- **导入复用**：复用现有 zip 导入逻辑，将文件夹打包成 zip 后调用现有服务

---

## 一、数据库设计

### 新增表 `team_package_versions`

**迁移文件路径：** `sql-change/v1.2.2/sqlite/00003_team_package_versions.sql`

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE team_package_versions (
    id UUID PRIMARY KEY DEFAULT (lower(hex(random_blob(16)))),
    workflow_id UUID NOT NULL REFERENCES workflow_templates(id),
    name VARCHAR(255) NOT NULL,           -- 团队包名称（用于匹配远程）
    category VARCHAR(255),                -- 团队分类（如 dev-team）
    version VARCHAR(50) NOT NULL,         -- 当前版本号（如 1.0.0）
    description TEXT,                     -- 团队包描述
    last_synced_at DATETIME,              -- 最后同步时间
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id),                  -- 每个 workflow 只有一条版本记录
    UNIQUE(name)                          -- 名称唯一，用于匹配
);

CREATE INDEX idx_tpv_workflow ON team_package_versions(workflow_id);
CREATE INDEX idx_tpv_name ON team_package_versions(name);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS team_package_versions;
-- +goose StatementEnd
```

---

## 二、配置设计

### config.yaml 新增配置项

```yaml
# 团队包自动更新配置
team_package_sync:
  # 远程仓库 URL
  remote_repo_url: "https://gitee.com/colink_1/isdp.git"
  # 是否启用自动更新
  auto_update_enabled: true
  # 检查间隔（格式: "24h", "12h", "30m"）
  check_interval: "24h"
  # 克隆时使用的分支（默认 main）
  branch: "main"
```

---

## 三、后端设计

### 3.1 文件结构规划（最小侵入原则）

```
internal/
├── model/
│   └── team_package_version.go          # 新增：版本模型
│
├── repo/
│   └── team_package_version_repo.go     # 新增：版本 Repository
│
├── service/
│   └── teampackage/
│   │   └── service.go                   # 保持不变，复用
│   │
│   └── teampackagesync/                 # 新增目录
│       ├── service.go                   # 同步服务主逻辑
│       ├── git_client.go                # Git 克隆封装
│       ├── parser.go                    # package.json 解析
│       ├── checker.go                   # 定时检查器
│       └── types.go                     # 类型定义
│
├── api/
│   ├── team_package_handler.go          # 保持不变
│   └── team_package_sync_handler.go     # 新增：同步 API
│
├── config/
│   └── config.go                        # 追加一个字段
```

### 3.2 数据模型

```go
// TeamPackageVersion 团队包版本记录
type TeamPackageVersion struct {
    ID           uuid.UUID  `json:"id"`
    WorkflowID   uuid.UUID  `json:"workflowId"`
    Name         string     `json:"name"`
    Category     string     `json:"category"`
    Version      string     `json:"version"`
    Description  string     `json:"description"`
    LastSyncedAt *time.Time `json:"lastSyncedAt,omitempty"`
    CreatedAt    time.Time  `json:"createdAt"`
    UpdatedAt    time.Time  `json:"updatedAt"`
}

func (t *TeamPackageVersion) TableName() string {
    return "team_package_versions"
}
```

### 3.3 Repository 接口

```go
type TeamPackageVersionRepository struct {
    db *sql.DB
}

func NewTeamPackageVersionRepository(db *sql.DB) *TeamPackageVersionRepository

// CRUD 方法
func (r *TeamPackageVersionRepository) Create(ctx context.Context, version *model.TeamPackageVersion) error
func (r *TeamPackageVersionRepository) FindByWorkflowID(ctx context.Context, workflowID uuid.UUID) (*model.TeamPackageVersion, error)
func (r *TeamPackageVersionRepository) FindByName(ctx context.Context, name string) (*model.TeamPackageVersion, error)
func (r *TeamPackageVersionRepository) ListAll(ctx context.Context) ([]model.TeamPackageVersion, error)
func (r *TeamPackageVersionRepository) Update(ctx context.Context, version *model.TeamPackageVersion) error
func (r *TeamPackageVersionRepository) Delete(ctx context.Context, id uuid.UUID) error
```

### 3.4 同步服务类型定义

```go
// types.go
type RemotePackageList struct {
    Categories []RemotePackageCategory `json:"categories"`
}

type RemotePackageCategory struct {
    Name     string          `json:"name"`
    Packages []RemotePackage `json:"packages"`
}

type RemotePackage struct {
    Name        string `json:"name"`
    Version     string `json:"version"`
    Description string `json:"description"`
    Path        string `json:"path"`
}

type UpdateCheckResult struct {
    NeedUpdate  []PackageUpdateInfo `json:"needUpdate"`
    NewPackages []RemotePackage     `json:"newPackages"`
    Removed     []string            `json:"removed"`
}

type PackageUpdateInfo struct {
    Local  TeamPackageVersion `json:"local"`
    Remote RemotePackage      `json:"remote"`
}
```

### 3.5 同步服务核心方法

```go
type SyncService struct {
    versionRepo    *repo.TeamPackageVersionRepository
    workflowRepo   *repo.WorkflowTemplateRepository
    teamPackageSvc *teampackage.Service  // 复用现有导入服务
    config         TeamPackageSyncConfig
    gitClient      *GitClient
    logger         *zap.Logger
}

func NewSyncService(...) *SyncService

// GetRemotePackages 获取远程团队包列表（按分类组织）
func (s *SyncService) GetRemotePackages(ctx context.Context) (*RemotePackageList, error)

// CheckUpdates 检查需要更新的团队包
func (s *SyncService) CheckUpdates(ctx context.Context) (*UpdateCheckResult, error)

// SyncPackage 同步指定团队包（导入/更新）
func (s *SyncService) SyncPackage(ctx context.Context, packageName string, confirm *model.TeamPackageImportConfirm) (*model.ImportResult, error)
```

### 3.6 Git 克隆流程

```
1. 克隆远程仓库到临时目录 (git clone --depth 1 --branch <branch>)
2. 遍历根目录，识别分类文件夹
3. 遍历分类文件夹，识别团队包文件夹（包含 package.json）
4. 解析每个 package.json，提取 name/version/description
5. 清理临时目录
6. 返回结构化列表
```

**安全考虑：**
- 使用 `--depth 1` 只克隆最新提交，减少下载量
- 临时目录自动清理
- 路径安全检查（防止路径穿越）
- 限制克隆 URL 来源（只允许配置中的 URL）

### 3.7 版本对比逻辑

```
1. 从 team_package_versions 表查询本地所有团队包
2. 遍历远程团队包列表
3. 按 name 字段匹配
4. 对比 version 字段（语义化版本比较）
5. 远程版本 > 本地版本 → 标记为需要更新
6. 远程存在但本地无 → 标记为可导入
7. 本地存在但远程无 → 标记为已移除（仅提示，不自动删除）
```

### 3.8 复用导入逻辑

`SyncPackage` 方法将团队包文件夹打包成 zip，调用现有 `teampackage.Service.ImportConfirm`：

```go
func (s *SyncService) SyncPackage(ctx context.Context, packageName string, confirm *model.TeamPackageImportConfirm) (*model.ImportResult, error) {
    // 1. 从 git 仓库获取团队包文件夹路径
    packagePath := s.getPackagePath(packageName)
    
    // 2. 将文件夹打包成 zip（临时）
    zipData, err := s.createZipFromDir(packagePath)
    
    // 3. 调用现有导入服务（零侵入复用）
    result, err := s.teamPackageSvc.ImportConfirm(ctx, zipData, confirm)
    
    // 4. 更新版本记录
    if err == nil {
        s.updateVersionRecord(ctx, packageName, remoteVersion)
    }
    
    return result, err
}
```

### 3.9 API Handler

新增路由：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/team-packages/remote` | 获取远程团队包列表 |
| GET | `/team-packages/check-update` | 检查更新 |
| POST | `/team-packages/sync` | 同步指定团队包 |

请求体：
```json
{
  "packageName": "team-1",
  "confirm": {
    "mode": "overwrite",
    "workflowAction": "overwrite",
    ...
  }
}
```

---

## 四、前端设计

### 4.1 通用设置页面扩展

新增卡片（追加在现有卡片下方）：

```tsx
<Card title="团队包自动更新">
  {/* 自动更新开关 */}
  <Form.Item label="启用自动更新">
    <Switch checked={autoUpdateEnabled} onChange={handleAutoUpdateChange} />
  </Form.Item>
  
  {/* 检查间隔 */}
  <Form.Item label="检查间隔">
    <Select options={['12小时', '24小时', '每周']} />
  </Form.Item>
  
  {/* 远程仓库地址（只读展示） */}
  <Form.Item label="远程仓库">
    <Text copyable>{remoteRepoUrl}</Text>
  </Form.Item>
  
  {/* 当前导入的团队包版本列表 */}
  <Form.Item label="已导入团队包">
    <Table columns={[名称, 分类, 版本号, 最后同步时间]} dataSource={localPackages} />
  </Form.Item>
  
  {/* 手动检查更新按钮 */}
  <Button type="primary" onClick={handleCheckUpdate}>检查更新</Button>
</Card>
```

### 4.2 团队包管理页面扩展

新增远程团队包区域：

```tsx
<Card title="远程团队包">
  {/* 分类筛选 */}
  <Select placeholder="选择分类" onChange={handleCategoryChange} />
  
  {/* 远程团队包列表 */}
  <Table
    columns={[
      { title: '名称', dataIndex: 'name' },
      { title: '版本', dataIndex: 'version' },
      { title: '分类', dataIndex: 'category' },
      { title: '本地状态', render: () => ... }, // 已导入/待更新/未导入
      { title: '操作', render: () => ... },     // 导入/更新按钮
    ]}
    dataSource={remotePackages}
  />
  
  {/* 刷新按钮 */}
  <Button icon={<ReloadOutlined />} onClick={refreshRemotePackages}>刷新远程列表</Button>
</Card>
```

### 4.3 API Client 扩展

```typescript
teamPackages: {
  // 现有方法保持不变
  import: (file) => ...,
  importConfirm: (file, confirm) => ...,
  export: (workflowId) => ...,
  
  // 新增方法
  getRemotePackages: (): Promise<RemotePackageList> => 
    request.get('/team-packages/remote'),
  
  checkUpdates: (): Promise<UpdateCheckResult> => 
    request.get('/team-packages/check-update'),
  
  syncPackage: (packageName: string, confirm: ImportConfirm): Promise<ImportResult> =>
    request.post('/team-packages/sync', { packageName, confirm }),
}
```

### 4.4 类型定义

```typescript
interface RemotePackageList {
  categories: RemotePackageCategory[];
}

interface RemotePackageCategory {
  name: string;
  packages: RemotePackage[];
}

interface RemotePackage {
  name: string;
  version: string;
  description: string;
  path: string;
}

interface UpdateCheckResult {
  needUpdate: PackageUpdateInfo[];
  newPackages: RemotePackage[];
  removed: string[];
}

interface PackageUpdateInfo {
  local: TeamPackageVersion;
  remote: RemotePackage;
}

interface TeamPackageVersion {
  id: string;
  workflowId: string;
  name: string;
  category: string;
  version: string;
  description: string;
  lastSyncedAt?: string;
}
```

### 4.5 更新提示弹窗

```tsx
<Modal title="团队包更新提醒" visible={showUpdateModal}>
  <Text>检测到 {needUpdate.length} 个团队包有新版本</Text>
  <Table columns={[名称, 当前版本, 新版本]} dataSource={needUpdate} />
  <Checkbox>自动更新所有团队包</Checkbox>
</Modal>
```

### 4.6 localStorage 存储

```typescript
const STORAGE_KEYS = {
  CHECKED_BEFORE: 'isdp_team_package_checked',           // 是否已首次检查
  UPDATE_SKIPPED: 'isdp_team_package_update_skipped',    // 用户曾跳过更新
  AUTO_UPDATE_ENABLED: 'isdp_team_package_auto_update', // 自动更新开关
};
```

---

## 五、自动检查流程

### 5.1 后端启动检查

在 `cmd/server/main.go` 启动后端时，启动后台同步检查器：

```go
if cfg.TeamPackageSync.AutoUpdateEnabled {
    syncChecker := teampackagesync.NewSyncChecker(syncSvc, cfg.TeamPackageSync.CheckInterval)
    syncChecker.Start()  // 后台 goroutine
    defer syncChecker.Stop()
}
```

### 5.2 定时检查器

```go
type SyncChecker struct {
    syncSvc    *SyncService
    interval   time.Duration
    stopChan   chan struct{}
    logger     *zap.Logger
}

func (c *SyncChecker) Start() {
    c.check()  // 启动时立即检查一次
    go c.runLoop()  // 启动定时检查 goroutine
}

func (c *SyncChecker) runLoop() {
    ticker := time.NewTicker(c.interval)
    for {
        select {
        case <-ticker.C: c.check()
        case <-c.stopChan: return
        }
    }
}
```

### 5.3 前端启动检查

```tsx
useEffect(() => {
  checkTeamPackageUpdateOnStartup();
}, []);

const checkTeamPackageUpdateOnStartup = async () => {
  const hasCheckedBefore = localStorage.getItem('isdp_team_package_checked');
  if (!hasCheckedBefore) {
    const result = await api.teamPackages.checkUpdates();
    localStorage.setItem('isdp_team_package_checked', 'true');
    
    if (result.needUpdate.length > 0) {
      showUpdateNotification(result);
    }
  }
};
```

### 5.4 首次弹窗询问

检测到更新且用户首次检查时，弹窗询问是否更新，提供：
- 确认更新按钮
- 跳过更新按钮（记住到 localStorage）
- 自动更新 checkbox

---

## 六、测试策略

### 6.1 单元测试

| 模块 | 测试点 |
|------|--------|
| SyncService | GetRemotePackages、CheckUpdates、SyncPackage |
| GitClient | Clone（正常/无效URL/超时） |
| Parser | package.json 解析、版本比较 |
| Checker | 定时触发、启动检查 |
| Repository | CRUD 操作 |

### 6.2 版本比较测试

```go
func TestCompareVersions(t *testing.T) {
    assert.Equal(t, -1, compareVersions("1.0.0", "1.0.1"))
    assert.Equal(t, 0, compareVersions("1.0.0", "1.0.0"))
    assert.Equal(t, 1, compareVersions("2.0.0", "1.9.9"))
    assert.Equal(t, -1, compareVersions("1.0", "1.0.1"))
}
```

### 6.3 集成测试场景

```
1. 初始化测试数据库
2. 导入 team-package-1（版本 1.0.0）
3. 修改远程仓库版本为 1.0.1
4. 执行 CheckUpdates → 返回 needUpdate
5. 执行 SyncPackage → 更新成功
6. 检查版本记录 → 1.0.1
```

### 6.4 前端测试清单

```
□ 通用设置：开关切换、检查间隔、版本列表、手动检查
□ 团队包管理：远程列表、分类筛选、导入/更新操作
□ 更新弹窗：确认、跳过、自动更新记忆
□ 错误场景：网络断开、仓库不可达、服务异常
```

---

## 七、安全考虑

### 7.1 Git 克隆安全

- 限制克隆 URL 来源（只允许配置中的 URL）
- 使用临时目录，自动清理
- 路径安全检查（防止路径穿越）

### 7.2 文件解析安全

- 限制 package.json 文件大小（64KB）
- 限制 ZIP 解压大小和文件数量

---

## 八、性能考虑

### 8.1 Git 克隆优化

- 使用 `--depth 1` 只克隆最新提交
- 设置克隆超时（60秒）

### 8.2 缓存策略（可选）

- 缓存远程团队包列表，减少重复克隆
- 缓存有效期（如 1 小时）

---

## 九、侵入性总结

| 文件 | 改动类型 |
|------|---------|
| `team_package_handler.go` | 无改动 |
| `teampackage/service.go` | 无改动 |
| `GeneralSettings.tsx` | 追加一个 Card |
| `TeamPackage/index.tsx` | 追加一个 Card |
| `client.ts` | 追加 API 方法 |
| `types/index.ts` | 追加类型定义 |
| `config.go` | 追加一个配置字段 |
| 其他 | 全部新增文件 |

---

## 十、后续迭代方向

1. **同步历史记录**：记录每次同步操作详情
2. **多仓库支持**：支持从多个远程仓库同步
3. **选择性更新**：支持只更新特定资产（如只更新 Skills）
4. **回滚能力**：支持回退到旧版本团队包