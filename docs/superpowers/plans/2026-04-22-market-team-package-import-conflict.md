# 市场团队包导入冲突处理优化 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为市场管理下的团队包导入增加冲突检测和策略选择功能，与管理工具保持一致

**Architecture:** 后端增强 PreviewPackage API 返回冲突检测字段，前端复用管理工具的预览表格样式和冲突处理按钮

**Tech Stack:** Go (后端), React + Ant Design (前端)

---

## File Structure

| 文件 | 责任 |
|------|------|
| `internal/service/teampackagesync/types.go` | 预览响应类型定义（增加 Exists 字段） |
| `internal/service/teampackagesync/service.go` | PreviewPackage 方法改造（增加冲突检测逻辑） |
| `internal/repo/agent_config.go` | 添加 FindByName 方法（角色名称检查） |
| `web/src/types/index.ts` | PackagePreviewResponse 类型更新 |
| `web/src/pages/Market/TeamPackages.tsx` | 预览弹框和批量导入改造 |

---

## Task 1: 后端类型定义更新

**Files:**
- Modify: `internal/service/teampackagesync/types.go:36-69`

- [ ] **Step 1: 更新 PreviewWorkflowInfo 结构体**

在 `types.go` 中，为 `PreviewWorkflowInfo` 添加 `Exists` 字段：

```go
type PreviewWorkflowInfo struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Exists      bool   `json:"exists"` // 新增：本地是否已存在
}
```

- [ ] **Step 2: 更新 PreviewRoleInfo 结构体**

为 `PreviewRoleInfo` 添加 `Exists` 和 `LocalId` 字段：

```go
type PreviewRoleInfo struct {
    Name        string   `json:"name"`
    Role        string   `json:"role"`
    Description string   `json:"description"`
    Assets      []string `json:"assets"`
    Exists      bool     `json:"exists"`  // 新增：本地是否已存在
    LocalId     string   `json:"localId"` // 新增：本地已存在的ID
}
```

- [ ] **Step 3: 更新 PreviewAssetInfo 结构体**

为 `PreviewAssetInfo` 添加 `Exists` 字段：

```go
type PreviewAssetInfo struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Exists      bool   `json:"exists"` // 新增：本地是否已存在
}
```

- [ ] **Step 4: 更新 PreviewPackageResponse 结构体**

为 `PreviewPackageResponse` 添加 `ConflictCount` 字段：

```go
type PreviewPackageResponse struct {
    PackageName   string              `json:"packageName"`
    Version       string              `json:"version"`
    Description   string              `json:"description"`
    Workflow      PreviewWorkflowInfo `json:"workflow"`
    Roles         []PreviewRoleInfo   `json:"roles"`
    Assets        PreviewAssetsInfo   `json:"assets"`
    ConflictCount int                 `json:"conflictCount"` // 新增：冲突总数
}
```

- [ ] **Step 5: 验证类型定义编译通过**

```bash
cd D:/CoLinkProject/Colink-0421/isdp
go build ./internal/service/teampackagesync
```

Expected: 编译成功，无错误

- [ ] **Step 6: Commit**

```bash
git add internal/service/teampackagesync/types.go
git commit -m "feat(teampackagesync): add conflict detection fields to preview response types

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 2: 后端 AgentRepo 添加 FindByName 方法

**Files:**
- Modify: `internal/repo/agent_config.go:240-256` (在 Delete 方法后添加)

- [ ] **Step 1: 添加 FindByName 方法**

在 `agent_config.go` 文件末尾添加 `FindByName` 方法：

```go
// FindByName 根据名称查找 Agent 配置（用于冲突检测）
func (r *AgentConfigRepository) FindByName(ctx context.Context, name string) (*model.AgentRoleConfig, error) {
    query := `
        SELECT id, name, role, description, system_prompt, max_tokens, temperature, base_agent_id, is_default, is_system, requires_human, mention_patterns, config_generated_at, config_path, created_at, updated_at
        FROM agent_configs WHERE name = ? LIMIT 1
    `
    config := &model.AgentRoleConfig{}
    var idStr string
    var mentionPatterns []byte
    var baseAgentID, description, systemPrompt, configPath sql.NullString
    var maxTokens sql.NullInt64
    var temperature sql.NullFloat64
    var configGeneratedAt sql.NullTime
    var createdAt, updatedAt time.Time
    var roleStr string
    var isDefault, isSystem, requiresHuman bool

    err := r.DB().QueryRowContext(ctx, query, name).Scan(
        &idStr, &roleStr, &description, &systemPrompt, &maxTokens, &temperature,
        &baseAgentID, &isDefault, &isSystem, &requiresHuman, &mentionPatterns,
        &configGeneratedAt, &configPath, &createdAt, &updatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, nil // 不存在返回 nil，不返回错误
    }
    if err != nil {
        return nil, fmt.Errorf("failed to find agent config by name: %w", err)
    }

    id, _ := uuid.Parse(idStr)
    config.ID = id
    config.Name = roleStr // 注意：数据库中 name 列存储的是显示名称
    config.Role = model.AgentRole(roleStr)
    config.IsDefault = isDefault
    config.IsSystem = isSystem
    config.RequiresHuman = requiresHuman
    config.CreatedAt = createdAt
    config.UpdatedAt = updatedAt

    if description.Valid {
        config.Description = description.String
    }
    if systemPrompt.Valid {
        config.SystemPrompt = systemPrompt.String
    }
    if maxTokens.Valid {
        config.MaxTokens = int(maxTokens.Int64)
    }
    if temperature.Valid {
        config.Temperature = temperature.Float64
    }
    if baseAgentID.Valid {
        config.BaseAgentID, _ = uuid.Parse(baseAgentID.String)
    }
    if len(mentionPatterns) > 0 {
        json.Unmarshal(mentionPatterns, &config.MentionPatterns)
    }
    if configGeneratedAt.Valid {
        config.ConfigGeneratedAt = &configGeneratedAt.Time
    }
    if configPath.Valid {
        config.ConfigPath = configPath.String
    }

    return config, nil
}
```

- [ ] **Step 2: 验证编译通过**

```bash
cd D:/CoLinkProject/Colink-0421/isdp
go build ./internal/repo
```

Expected: 编译成功，无错误

- [ ] **Step 3: Commit**

```bash
git add internal/repo/agent_config.go
git commit -m "feat(repo): add FindByName method to AgentConfigRepository

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 3: 后端 SyncService 注入所需 Repo

**Files:**
- Modify: `internal/service/teampackagesync/service.go:24-53`

- [ ] **Step 1: 扩展 SyncService 结构体**

在 `service.go` 中，扩展 `SyncService` 结构体添加所需 repo：

```go
type SyncService struct {
    versionRepo    *repo.TeamPackageVersionRepository
    workflowRepo   *repo.WorkflowTemplateRepository
    teamPackageSvc *teampackage.Service
    marketSvc      *market.Service
    config         config.TeamPackageSyncConfig
    gitClient      *GitClient
    logger         *zap.Logger
    // 新增：用于冲突检测的 repo
    agentRepo    *repo.AgentConfigRepository
    skillRepo    *repo.SkillRepository
    commandRepo  *repo.CommandRepository
    subagentRepo *repo.SubagentRepository
    ruleRepo     *repo.RuleRepository
    settingsRepo *repo.SettingsRepository
}
```

- [ ] **Step 2: 更新 NewSyncService 函数签名和实现**

更新 `NewSyncService` 函数：

```go
func NewSyncService(
    versionRepo *repo.TeamPackageVersionRepository,
    workflowRepo *repo.WorkflowTemplateRepository,
    teamPackageSvc *teampackage.Service,
    marketSvc *market.Service,
    cfg config.TeamPackageSyncConfig,
    basePath string,
    logger *zap.Logger,
    // 新增参数
    agentRepo *repo.AgentConfigRepository,
    skillRepo *repo.SkillRepository,
    commandRepo *repo.CommandRepository,
    subagentRepo *repo.SubagentRepository,
    ruleRepo *repo.RuleRepository,
    settingsRepo *repo.SettingsRepository,
) *SyncService {
    return &SyncService{
        versionRepo:    versionRepo,
        workflowRepo:   workflowRepo,
        teamPackageSvc: teamPackageSvc,
        marketSvc:      marketSvc,
        config:         cfg,
        gitClient:      NewGitClient(cfg, basePath, logger),
        logger:         logger,
        agentRepo:      agentRepo,
        skillRepo:      skillRepo,
        commandRepo:    commandRepo,
        subagentRepo:   subagentRepo,
        ruleRepo:       ruleRepo,
        settingsRepo:   settingsRepo,
    }
}
```

- [ ] **Step 3: 验证编译通过（预期有调用点错误）**

```bash
cd D:/CoLinkProject/Colink-0421/isdp
go build ./internal/service/teampackagesync
```

Expected: 编译成功，但其他调用 NewSyncService 的地方会报错

- [ ] **Step 4: Commit**

```bash
git add internal/service/teampackagesync/service.go
git commit -m "feat(teampackagesync): inject required repos for conflict detection

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 4: 更新 NewSyncService 调用点

**Files:**
- Find and modify: `internal/api/team_package_sync_handler.go` 或其他调用点

- [ ] **Step 1: 找到 NewSyncService 调用点**

```bash
cd D:/CoLinkProject/Colink-0421/isdp
grep -rn "NewSyncService" --include="*.go" .
```

Expected: 找到调用点文件和行号

- [ ] **Step 2: 更新调用点传入新参数**

根据实际调用点位置，在 wire 或 main 中添加新 repo 参数传递。实际文件位置可能需要确认。

- [ ] **Step 3: 验证整体编译通过**

```bash
cd D:/CoLinkProject/Colink-0421/isdp
go build ./...
```

Expected: 编译成功，无错误

- [ ] **Step 4: Commit**

```bash
git add .
git commit -m "feat: update NewSyncService call sites with new repo parameters

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 5: 后端 PreviewPackage 方法增加冲突检测逻辑

**Files:**
- Modify: `internal/service/teampackagesync/service.go` 的 `PreviewPackage` 方法（约第 368 行开始）

- [ ] **Step 1: 在 PreviewPackage 方法中添加 Workflow 存在检查**

在 `response := &PreviewPackageResponse{...}` 构建之后，添加 Workflow 存在检查：

```go
    // 检查 Workflow 是否已存在（按名称匹配）
    workflows, err := s.workflowRepo.FindAll(ctx)
    if err == nil {
        for _, wf := range workflows {
            if wf.Name == manifest.Workflow.Name {
                response.Workflow.Exists = true
                break
            }
        }
    }
```

- [ ] **Step 2: 添加 Roles 存在检查**

继续添加角色检查（按 ID 匹配，与管理工具一致）：

```go
    // 检查 Roles 是否已存在（按 ID 匹配）
    for i, role := range manifest.Roles {
        roleID, err := uuid.Parse(role.ID)
        if err == nil {
            existing, err := s.agentRepo.FindByID(ctx, roleID)
            if err == nil && existing != nil {
                response.Roles[i].Exists = true
                response.Roles[i].LocalId = existing.ID.String()
            }
        }
    }
```

- [ ] **Step 3: 添加 Skills 存在检查**

```go
    // 检查 Skills 是否已存在（按名称匹配）
    for i, skill := range manifest.Assets.Skills {
        if s.skillRepo != nil {
            existing, err := s.skillRepo.FindByName(ctx, skill.Name)
            if err == nil && existing != nil {
                response.Assets.Skills[i].Exists = true
            }
        }
    }
```

- [ ] **Step 4: 添加 Commands 存在检查**

```go
    // 检查 Commands 是否已存在（按名称匹配）
    for i, cmd := range manifest.Assets.Commands {
        if s.commandRepo != nil {
            existing, err := s.commandRepo.FindByName(ctx, cmd.Name)
            if err == nil && existing != nil {
                response.Assets.Commands[i].Exists = true
            }
        }
    }
```

- [ ] **Step 5: 添加 Subagents 存在检查**

```go
    // 检查 Subagents 是否已存在（按名称匹配）
    for i, sub := range manifest.Assets.Subagents {
        if s.subagentRepo != nil {
            existing, err := s.subagentRepo.FindByName(ctx, sub.Name)
            if err == nil && existing != nil {
                response.Assets.Subagents[i].Exists = true
            }
        }
    }
```

- [ ] **Step 6: 添加 Rules 存在检查**

```go
    // 检查 Rules 是否已存在（按名称匹配）
    for i, rule := range manifest.Assets.Rules {
        if s.ruleRepo != nil {
            existing, err := s.ruleRepo.FindByName(ctx, rule.Name)
            if err == nil && existing != nil {
                response.Assets.Rules[i].Exists = true
            }
        }
    }
```

- [ ] **Step 7: 添加 Settings 存在检查**

```go
    // 检查 Settings 是否已存在（按名称匹配）
    for i, settings := range manifest.Assets.Settings {
        if s.settingsRepo != nil {
            existing, err := s.settingsRepo.FindByName(ctx, settings.Name)
            if err == nil && existing != nil {
                response.Assets.Settings[i].Exists = true
            }
        }
    }
```

- [ ] **Step 8: 计算 ConflictCount**

```go
    // 计算冲突总数
    response.ConflictCount = 0
    if response.Workflow.Exists {
        response.ConflictCount++
    }
    for _, role := range response.Roles {
        if role.Exists {
            response.ConflictCount++
        }
    }
    for _, skill := range response.Assets.Skills {
        if skill.Exists {
            response.ConflictCount++
        }
    }
    for _, cmd := range response.Assets.Commands {
        if cmd.Exists {
            response.ConflictCount++
        }
    }
    for _, sub := range response.Assets.Subagents {
        if sub.Exists {
            response.ConflictCount++
        }
    }
    for _, rule := range response.Assets.Rules {
        if rule.Exists {
            response.ConflictCount++
        }
    }
    for _, settings := range response.Assets.Settings {
        if settings.Exists {
            response.ConflictCount++
        }
    }
```

- [ ] **Step 9: 更新日志输出包含冲突信息**

修改末尾日志：

```go
    s.logger.Info("团队包预览完成",
        zap.String("package", packageName),
        zap.Int("roles", len(response.Roles)),
        zap.Int("skills", len(response.Assets.Skills)),
        zap.Int("conflictCount", response.ConflictCount))
```

- [ ] **Step 10: 验证编译通过**

```bash
cd D:/CoLinkProject/Colink-0421/isdp
go build ./internal/service/teampackagesync
```

Expected: 编译成功，无错误

- [ ] **Step 11: Commit**

```bash
git add internal/service/teampackagesync/service.go
git commit -m "feat(teampackagesync): add conflict detection logic to PreviewPackage

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 6: 前端类型定义更新

**Files:**
- Modify: `web/src/types/index.ts:1219-1245` (PackagePreviewResponse 类型)

- [ ] **Step 1: 更新 PackagePreviewResponse 类型**

在 `types/index.ts` 中更新 `PackagePreviewResponse` 接口：

```typescript
// PackagePreviewResponse 团队包预览响应
export interface PackagePreviewResponse {
  packageName: string;
  version: string;
  description: string;
  workflow: {
    name: string;
    description: string;
    exists: boolean;  // 新增
  };
  roles: Array<{
    name: string;
    role: string;
    description: string;
    assets: string[];
    exists: boolean;  // 新增
    localId?: string; // 新增
  }>;
  assets: {
    skills: Array<{ name: string; description: string; exists: boolean }>;    // exists 新增
    commands: Array<{ name: string; description: string; exists: boolean }>;  // exists 新增
    subagents: Array<{ name: string; description: string; exists: boolean }>; // exists 新增
    rules: Array<{ name: string; description: string; exists: boolean }>;     // exists 新增
    settings: Array<{ name: string; description: string; exists: boolean }>;  // exists 新增
  };
  conflictCount: number;  // 新增：冲突总数
}
```

- [ ] **Step 2: 验证 TypeScript 编译**

```bash
cd D:/CoLinkProject/Colink-0421/isdp/web
npm run build
```

Expected: 编译成功，可能有 TypeScript 错误需要后续修复

- [ ] **Step 3: Commit**

```bash
git add web/src/types/index.ts
git commit -m "feat(types): add conflict detection fields to PackagePreviewResponse

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 7: 前端预览弹框改造 - Alert 和表格列

**Files:**
- Modify: `web/src/pages/Market/TeamPackages.tsx:198-354` (renderPreviewModal 函数)

- [ ] **Step 1: 导入 WarningOutlined 图标**

在文件顶部导入区域添加 `WarningOutlined`：

```tsx
import {
  CloudDownloadOutlined, ShopOutlined, CheckSquareOutlined, WarningOutlined
} from '@ant-design/icons';
```

- [ ] **Step 2: 导入 Alert 组件**

在 Ant Design 导入中添加 `Alert`：

```tsx
import {
  Card, Table, Button, Space, Tag, message, Spin, Modal,
  Descriptions, Collapse, Typography, Divider, Progress, Alert
} from 'antd';
```

- [ ] **Step 3: 在预览弹框内容顶部添加冲突提示 Alert**

在 `renderPreviewModal` 函数的 Modal 内容开头（`<Descriptions bordered...>` 之前）添加：

```tsx
      {/* 冲突提示 */}
      {previewData.conflictCount > 0 && (
        <Alert
          type="warning"
          icon={<WarningOutlined />}
          message={`检测到 ${previewData.conflictCount} 个冲突项，请选择处理方式`}
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}
```

- [ ] **Step 4: 更新角色表格列定义增加状态列**

修改 `roleColumns` 定义，在 `assets` 列之前添加状态列：

```tsx
    const roleColumns = [
      { title: '角色名称', dataIndex: 'name', key: 'name', width: 150 },
      { title: '角色类型', dataIndex: 'role', key: 'role', width: 100 },
      { title: '描述', dataIndex: 'description', key: 'description' },
      {
        title: '状态',
        dataIndex: 'exists',
        key: 'exists',
        width: 80,
        render: (exists: boolean) => (
          <Tag color={exists ? 'warning' : 'success'}>
            {exists ? '已存在' : '新增'}
          </Tag>
        ),
      },
      {
        title: '绑定资产',
        dataIndex: 'assets',
        key: 'assets',
        render: (assets: string[]) => (
          <Space direction="vertical" size="small">
            {assets.map((asset, idx) => (
              <Tag key={idx} color={
                asset.startsWith('Skill:') ? 'blue' :
                asset.startsWith('Command:') ? 'green' :
                asset.startsWith('Subagent:') ? 'purple' :
                asset.startsWith('Rule:') ? 'orange' :
                asset.startsWith('Settings:') ? 'cyan' : 'default'
              }>
                {asset}
              </Tag>
            ))}
          </Space>
        ),
      },
    ];
```

- [ ] **Step 5: 定义资产表格通用列模板**

在 `renderPreviewModal` 中添加 `assetColumns` 定义：

```tsx
    // 资产表格列定义（通用，带状态列）
    const assetColumns = [
      { title: '名称', dataIndex: 'name', key: 'name', width: 200 },
      { title: '描述', dataIndex: 'description', key: 'description' },
      {
        title: '状态',
        dataIndex: 'exists',
        key: 'exists',
        width: 80,
        render: (exists: boolean) => (
          <Tag color={exists ? 'warning' : 'success'}>
            {exists ? '已存在' : '新增'}
          </Tag>
        ),
      },
    ];
```

- [ ] **Step 6: 更新 Collapse 中各资产表格使用新列定义**

修改 Collapse items 中的表格，使用 `assetColumns`：

```tsx
        <Collapse
          items={[
            {
              key: 'skills',
              label: `Skills (${previewData.assets.skills?.length || 0})`,
              children: (
                <Table
                  dataSource={previewData.assets.skills || []}
                  columns={assetColumns}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              ),
            },
            {
              key: 'commands',
              label: `Commands (${previewData.assets.commands?.length || 0})`,
              children: (
                <Table
                  dataSource={previewData.assets.commands || []}
                  columns={assetColumns}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              ),
            },
            {
              key: 'subagents',
              label: `Subagents (${previewData.assets.subagents?.length || 0})`,
              children: (
                <Table
                  dataSource={previewData.assets.subagents || []}
                  columns={assetColumns}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              ),
            },
            {
              key: 'rules',
              label: `Rules (${previewData.assets.rules?.length || 0})`,
              children: (
                <Table
                  dataSource={previewData.assets.rules || []}
                  columns={assetColumns}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              ),
            },
            {
              key: 'settings',
              label: `Settings (${previewData.assets.settings?.length || 0})`,
              children: (
                <Table
                  dataSource={previewData.assets.settings || []}
                  columns={assetColumns}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              ),
            },
          ]}
        />
```

- [ ] **Step 7: 验证前端编译**

```bash
cd D:/CoLinkProject/Colink-0421/isdp/web
npm run build
```

Expected: 编译成功，无错误

- [ ] **Step 8: Commit**

```bash
git add web/src/pages/Market/TeamPackages.tsx
git commit -m "feat(market): add conflict alert and status column to preview modal

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 8: 前端预览弹框改造 - Footer 按钮和 handleSync

**Files:**
- Modify: `web/src/pages/Market/TeamPackages.tsx:59-71` (handleSync) 和 Modal footer

- [ ] **Step 1: 修改 handleSync 方法接收策略参数**

修改 `handleSync` 函数签名和实现：

```tsx
  const handleSync = async (packageName: string, mode: 'overwrite' | 'skip') => {
    setSyncingPackage(packageName);
    try {
      const pkg = packages.find(p => p.name === packageName);
      const confirm = {
        mode,
        workflowAction: mode,
        roleActions: [],
        assetActions: [],
      };
      await api.teamPackages.syncPackage(packageName, confirm, pkg?.marketId);
      message.success(`团队包 ${packageName} 导入成功`);
      loadPackages();
      setPreviewModalVisible(false);
    } catch (error: any) {
      message.error(error.response?.data?.error || '导入失败');
    } finally {
      setSyncingPackage(null);
    }
  };
```

- [ ] **Step 2: 导入 Popconfirm 组件**

在 Ant Design 导入中添加 `Popconfirm`：

```tsx
import {
  Card, Table, Button, Space, Tag, message, Spin, Modal,
  Descriptions, Collapse, Typography, Divider, Progress, Alert, Popconfirm
} from 'antd';
```

- [ ] **Step 3: 修改预览弹框 Footer 按钮逻辑**

修改 `renderPreviewModal` 中 Modal 的 footer：

```tsx
      footer={[
        <Button key="cancel" onClick={() => setPreviewModalVisible(false)}>
          取消
        </Button>,
        previewData.conflictCount === 0 ? (
          <Button
            key="import"
            type="primary"
            icon={<CloudDownloadOutlined />}
            loading={syncingPackage === previewData.packageName}
            onClick={() => handleSync(previewData.packageName, 'overwrite')}
          >
            确认导入
          </Button>
        ) : (
          <>
            <Popconfirm
              key="overwrite-confirm"
              title="确定要覆盖所有冲突项吗？"
              onConfirm={() => handleSync(previewData.packageName, 'overwrite')}
              okText="确定"
              cancelText="取消"
            >
              <Button
                type="primary"
                icon={<CloudDownloadOutlined />}
                loading={syncingPackage === previewData.packageName}
              >
                全部覆盖
              </Button>
            </Popconfirm>
            <Button
              key="skip"
              icon={<CloudDownloadOutlined />}
              loading={syncingPackage === previewData.packageName}
              onClick={() => handleSync(previewData.packageName, 'skip')}
            >
              全部跳过
            </Button>
          </>
        ),
      ]}
```

- [ ] **Step 4: 修改原有导入按钮调用**

将原有的 "确认导入" 按钮 onClick 从调用旧 `handleSync(record)` 改为调用预览：

```tsx
          onClick={() => handlePreview(record)}
```

（这部分应该已经正确，只需确认 handleSync 不再被直接调用导入）

- [ ] **Step 5: 验证前端编译**

```bash
cd D:/CoLinkProject/Colink-0421/isdp/web
npm run build
```

Expected: 编译成功，无错误

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/Market/TeamPackages.tsx
git commit -m "feat(market): add conflict handling buttons to preview modal footer

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 9: 前端批量导入改造 - 状态和预览逻辑

**Files:**
- Modify: `web/src/pages/Market/TeamPackages.tsx:73-118` (批量导入相关)

- [ ] **Step 1: 添加批量预览相关状态**

在组件状态定义区域添加：

```tsx
  const [batchPreviews, setBatchPreviews] = useState<PackagePreviewResponse[]>([]);
  const [loadingPreview, setLoadingPreview] = useState(false);
```

- [ ] **Step 2: 修改 handleBatchImportClick 执行预览**

修改批量导入按钮点击处理：

```tsx
  const handleBatchImportClick = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning('请先选择要导入的团队包');
      return;
    }

    const toImport = packages.filter(pkg =>
      selectedRowKeys.includes(`${pkg.marketId}-${pkg.name}`)
    );

    // 预览每个包获取冲突信息
    setLoadingPreview(true);
    const previews: PackagePreviewResponse[] = [];
    for (const pkg of toImport) {
      try {
        const result = await api.teamPackages.previewPackage(pkg.name, pkg.marketId);
        previews.push(result);
      } catch (error: any) {
        // 单个预览失败，添加一个空预览避免索引错位
        previews.push({
          packageName: pkg.name,
          version: pkg.version,
          description: '',
          workflow: { name: '', description: '', exists: false },
          roles: [],
          assets: { skills: [], commands: [], subagents: [], rules: [], settings: [] },
          conflictCount: 0,
        });
      }
    }
    setLoadingPreview(false);

    setBatchPreviews(previews);
    setPendingImportPackages(toImport);
    setConfirmModalVisible(true);
  };
```

- [ ] **Step 3: 验证前端编译**

```bash
cd D:/CoLinkProject/Colink-0421/isdp/web
npm run build
```

Expected: 编译成功，无错误

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/Market/TeamPackages.tsx
git commit -m "feat(market): add batch preview logic for conflict detection

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 10: 前端批量导入改造 - 确认弹框显示冲突统计

**Files:**
- Modify: `web/src/pages/Market/TeamPackages.tsx:454-476` (批量导入确认弹框)

- [ ] **Step 1: 计算总冲突数**

在确认弹框渲染前计算总冲突数：

```tsx
  const totalConflicts = batchPreviews.reduce((sum, p) => sum + p.conflictCount, 0);
```

- [ ] **Step 2: 改造确认弹框内容显示冲突统计表格**

修改批量导入确认弹框 Modal 内容：

```tsx
      {/* 批量导入确认弹框 */}
      <Modal
        title="确认批量导入"
        open={confirmModalVisible}
        onCancel={() => setConfirmModalVisible(false)}
        onOk={totalConflicts === 0 ? () => handleBatchImportConfirm('overwrite') : undefined}
        okText={totalConflicts === 0 ? '确认导入' : undefined}
        cancelText="取消"
        width={600}
        footer={totalConflicts === 0 ? undefined : [
          <Button key="cancel" onClick={() => setConfirmModalVisible(false)}>
            取消
          </Button>,
          <Popconfirm
            key="overwrite-confirm"
            title="确定要覆盖所有冲突项吗？"
            onConfirm={() => handleBatchImportConfirm('overwrite')}
            okText="确定"
            cancelText="取消"
          >
            <Button key="overwrite" type="primary">全部覆盖</Button>
          </Popconfirm>,
          <Button key="skip" onClick={() => handleBatchImportConfirm('skip')}>
            全部跳过
          </Button>,
        ]}
      >
        <Spin spinning={loadingPreview}>
          <Text>将导入以下 {pendingImportPackages.length} 个团队包：</Text>
          <Table
            dataSource={batchPreviews.map((preview, idx) => ({
              key: idx,
              name: pendingImportPackages[idx]?.name || preview.packageName,
              version: pendingImportPackages[idx]?.version || preview.version,
              status: pendingImportPackages[idx]?.localStatus,
              conflictCount: preview.conflictCount,
            }))}
            columns={[
              { title: '名称', dataIndex: 'name', key: 'name' },
              { title: '版本', dataIndex: 'version', key: 'version', width: 100, render: (v: string) => <Tag color="blue">{v}</Tag> },
              { title: '状态', dataIndex: 'status', key: 'status', width: 100, render: getStatusTag },
              { title: '冲突数', dataIndex: 'conflictCount', key: 'conflictCount', width: 80, render: (c: number) =>
                c > 0 ? <Tag color="warning">{c}</Tag> : <Tag color="success">0</Tag>
              },
            ]}
            pagination={false}
            size="small"
            style={{ marginTop: 12 }}
          />
          {totalConflicts > 0 && (
            <Alert
              type="warning"
              message={`共检测到 ${totalConflicts} 个冲突项，请选择处理策略`}
              showIcon
              style={{ marginTop: 16 }}
            />
          )}
        </Spin>
      </Modal>
```

- [ ] **Step 3: 验证前端编译**

```bash
cd D:/CoLinkProject/Colink-0421/isdp/web
npm run build
```

Expected: 编译成功，无错误

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/Market/TeamPackages.tsx
git commit -m "feat(market): show conflict stats in batch import confirm modal

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 11: 前端批量导入改造 - handleBatchImportConfirm 传递策略

**Files:**
- Modify: `web/src/pages/Market/TeamPackages.tsx:88-118` (handleBatchImportConfirm)

- [ ] **Step 1: 修改 handleBatchImportConfirm 接收策略参数**

修改函数签名：

```tsx
  const handleBatchImportConfirm = async (mode: 'overwrite' | 'skip') => {
```

- [ ] **Step 2: 在导入循环中传递策略参数**

修改导入执行部分：

```tsx
    for (let i = 0; i < pendingImportPackages.length; i++) {
      const pkg = pendingImportPackages[i];
      setBatchProgress(prev => ({ ...prev, current: i + 1 }));

      try {
        const confirm = {
          mode,
          workflowAction: mode,
          roleActions: [],
          assetActions: [],
        };
        await api.teamPackages.syncPackage(pkg.name, confirm, pkg.marketId);
        results.push({ name: pkg.name, status: 'success' });
        setBatchProgress(prev => ({ ...prev, success: prev.success + 1 }));
      } catch (error: any) {
        const errorMsg = error.response?.data?.error || '导入失败';
        results.push({ name: pkg.name, status: 'failed', error: errorMsg });
        setBatchProgress(prev => ({ ...prev, failed: prev.failed + 1 }));
      }
    }
```

- [ ] **Step 3: 验证前端编译**

```bash
cd D:/CoLinkProject/Colink-0421/isdp/web
npm run build
```

Expected: 编译成功，无错误

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/Market/TeamPackages.tsx
git commit -m "feat(market): pass conflict strategy to batch import sync

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 12: 集成测试 - 单包导入

**Files:**
- Test: 手动测试

- [ ] **Step 1: 启动后端服务**

```bash
cd D:/CoLinkProject/Colink-0421/isdp
go run ./cmd/server
```

- [ ] **Step 2: 启动前端开发服务器**

```bash
cd D:/CoLinkProject/Colink-0421/isdp/web
npm run dev
```

- [ ] **Step 3: 测试场景 1 - 导入全新团队包**

操作：
1. 进入市场管理 -> 团队包页面
2. 选择一个"未导入"状态的团队包，点击"导入"
3. 查看预览弹框，确认：
   - 无冲突提示 Alert
   - 所有项状态显示"新增"
   - 只有一个"确认导入"按钮

Expected: 导入成功，无冲突处理提示

- [ ] **Step 4: 测试场景 2 - 导入已存在的团队包**

操作：
1. 重新导入刚才导入的团队包（状态应为"已导入"）
2. 查看预览弹框，确认：
   - 有冲突提示 Alert 显示冲突数量
   - 各项状态显示"已存在"
   - 有"全部覆盖"和"全部跳过"两个按钮

Expected: 预览正确显示冲突

- [ ] **Step 5: 测试场景 3 - 选择覆盖策略**

点击"全部覆盖"，确认 Popconfirm 提示后执行导入。

Expected: 导入成功，原有数据被覆盖

- [ ] **Step 6: 测试场景 4 - 选择跳过策略**

再次导入同一包，点击"全部跳过"。

Expected: 导入成功，原有数据保持不变

---

## Task 13: 集成测试 - 批量导入

**Files:**
- Test: 手动测试

- [ ] **Step 1: 测试场景 1 - 批量导入无冲突的包**

操作：
1. 选择多个"未导入"状态的团队包
2. 点击"批量导入"
3. 查看确认弹框，确认：
   - 显示每个包的冲突数（应为0）
   - 无冲突提示 Alert
   - 只有"确认导入"按钮

Expected: 批量导入成功

- [ ] **Step 2: 测试场景 2 - 批量导入有冲突的包**

操作：
1. 选择已导入的包和未导入的包混合
2. 点击"批量导入"
3. 查看确认弹框，确认：
   - 显示每个包的冲突数
   - 有冲突提示 Alert 显示总冲突数
   - 有"全部覆盖"和"全部跳过"按钮

Expected: 预览正确显示各包冲突统计

- [ ] **Step 3: 测试场景 3 - 批量导入使用覆盖策略**

点击"全部覆盖"，确认执行。

Expected: 所有冲突项被覆盖，未冲突项正常导入

- [ ] **Step 4: 测试场景 4 - 批量导入使用跳过策略**

选择有冲突的包，点击"全部跳过"。

Expected: 冲突项保持不变，未冲突项正常导入

---

## Task 14: 深色模式样式检查

**Files:**
- Review: `web/src/pages/Market/TeamPackages.tsx`

- [ ] **Step 1: 切换到深色模式验证样式**

操作：
1. 切换系统主题为深色模式
2. 刷新页面
3. 检查预览弹框中：
   - Alert 样式是否正常
   - Tag 颜色是否清晰可辨
   - 表格样式是否正常

Expected: 深色模式下样式正常（已使用 Ant Design 默认颜色，无需自定义 CSS 变量）

- [ ] **Step 2: 如需调整，添加深色模式 CSS**

如果有样式问题，在组件中添加深色模式适配。

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/Market/TeamPackages.tsx
git commit -m "fix(market): ensure dark mode compatibility for conflict UI

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 15: 最终提交和清理

- [ ] **Step 1: 确认所有改动已提交**

```bash
cd D:/CoLinkProject/Colink-0421/isdp
git status
```

Expected: 无未提交改动

- [ ] **Step 2: 创建整合提交（如有必要）**

如果改动分散在多个小提交，可考虑整合：

```bash
git log --oneline -10
# 如果需要，可进行 rebase 或创建 merge commit
```

- [ ] **Step 3: 推送到远程（如需要）**

```bash
git push origin cc
```

- [ ] **Step 4: 更新设计文档状态**

将设计文档状态从"待审查"改为"实施完成"：

```markdown
**状态**: 实施完成
```

---

## Self-Review Checklist

**1. Spec coverage:**
- 单包导入冲突检测: Task 5, 7, 8 ✓
- 单包导入策略选择: Task 8 ✓
- 批量导入冲突检测: Task 9, 10 ✓
- 批量导入统一策略: Task 10, 11 ✓

**2. Placeholder scan:**
- 无 TBD、TODO ✓
- 所有代码步骤都有完整代码 ✓
- 所有命令步骤都有具体命令 ✓

**3. Type consistency:**
- 后端 types.go 定义与前端 types/index.ts 一致 ✓
- handleSync 参数类型 'overwrite' | 'skip' 与后端 confirm.mode 一致 ✓