# 市场管理团队包导入冲突处理设计

## 需求背景

优化市场管理下团队包导入和批量导入能力，导入前确认冲突，用户可选择跳过或覆盖。参考管理工具下的团队包导入能力。

## 当前状态分析

| 模块 | 文件位置 | 冲突检测 | 冲突处理 |
|------|----------|----------|----------|
| 管理工具导入 | `web/src/pages/TeamPackage/index.tsx` | ✅ 支持 | ✅ 支持覆盖/跳过 |
| 市场单包导入 | `web/src/pages/Market/TeamPackages.tsx` | ❌ 不支持 | ❌ 直接导入 |
| 市场批量导入 | `web/src/pages/Market/TeamPackages.tsx` | ❌ 不支持 | ❌ 直接导入 |

### 管理工具实现分析

管理工具团队包导入流程：
1. 上传 ZIP 文件，调用 `teamPackages.import()` 获取预览
2. 后端返回 `TeamPackagePreview`，包含每个项目的 `exists` 状态
3. 前端显示预览表格，状态列显示"已存在/新增"
4. 有冲突时显示 Alert 提示，提供"全部覆盖"/"全部跳过"按钮
5. 用户选择后调用 `teamPackages.importConfirm()` 传递策略

### 市场导入现状

市场导入流程：
1. 点击导入按钮，调用 `teamPackages.previewPackage()` 获取预览
2. 后端返回 `PackagePreviewResponse`，仅包含包内容信息，**无冲突状态**
3. 用户点击"确认导入"，直接调用 `syncPackage()` 无策略参数
4. 批量导入时，遍历调用 `syncPackage()`，无冲突处理

## 设计方案

**核心思路**：修改后端 `PreviewPackage` API 返回冲突检测结果，前端复用管理工具的预览表格样式和交互逻辑。

### 用户选择

| 决策点 | 用户选择 |
|--------|----------|
| 批量导入策略 | 统一策略模式（所有包使用相同策略） |
| 单包冲突处理 | 预览弹框集成冲突处理 |

## 实现细节

### 1. 后端改造

#### 1.1 类型定义修改

**文件**：`internal/service/teampackagesync/types.go`

```go
// PreviewPackageResponse 包预览响应（增强冲突检测）
type PreviewPackageResponse struct {
    PackageName   string              `json:"packageName"`
    Version       string              `json:"version"`
    Description   string              `json:"description"`
    Workflow      PreviewWorkflowInfo `json:"workflow"`
    Roles         []PreviewRoleInfo   `json:"roles"`
    Assets        PreviewAssetsInfo   `json:"assets"`
    ConflictCount int                 `json:"conflictCount"` // 新增：冲突总数
}

type PreviewWorkflowInfo struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Exists      bool   `json:"exists"` // 新增
}

type PreviewRoleInfo struct {
    Name        string   `json:"name"`
    Role        string   `json:"role"`
    Description string   `json:"description"`
    Assets      []string `json:"assets"`
    Exists      bool     `json:"exists"` // 新增
    LocalId     string   `json:"localId,omitempty"` // 新增：本地已存在的ID
}

type PreviewAssetsInfo struct {
    Skills    []PreviewAssetInfo `json:"skills"`
    Commands  []PreviewAssetInfo `json:"commands"`
    Subagents []PreviewAssetInfo `json:"subagents"`
    Rules     []PreviewAssetInfo `json:"rules"`
    Settings  []PreviewAssetInfo `json:"settings"`
}

type PreviewAssetInfo struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Exists      bool   `json:"exists"` // 新增
}
```

#### 1.2 服务层改造

**文件**：`internal/service/teampackagesync/service.go`

在 `PreviewPackage` 方法中，解析完 manifest 后，增加冲突检测逻辑：

```go
// 检查工作流是否已存在
workflows, err := s.workflowRepo.FindAll(ctx)
for _, wf := range workflows {
    if wf.Name == manifest.Workflow.Name {
        response.Workflow.Exists = true
        break
    }
}

// 检查角色是否已存在（按名称匹配）
agents, _ := s.agentRepo.List(ctx)
for _, role := range manifest.Roles {
    for _, agent := range agents {
        if agent.Name == role.Name {
            roleInfo.Exists = true
            roleInfo.LocalId = agent.ID.String()
            break
        }
    }
}

// 检查各类资产是否已存在
// Skills: skillRepo.FindByName
// Commands: commandRepo.FindByName
// Subagents: subagentRepo.FindByName
// Rules: ruleRepo.FindByName
// Settings: settingsRepo.FindByName

// 统计冲突总数
response.ConflictCount = countConflicts(response)
```

需要注入的 Repository：
- `workflowRepo`
- `agentRepo`
- `skillRepo`
- `commandRepo`
- `subagentRepo`
- `ruleRepo`
- `settingsRepo`

### 2. 前端改造

#### 2.1 类型定义更新

**文件**：`web/src/types/index.ts`

```typescript
// PackagePreviewResponse 团队包预览响应（增强）
export interface PackagePreviewResponse {
  packageName: string;
  version: string;
  description: string;
  workflow: {
    name: string;
    description: string;
    exists: boolean; // 新增
  };
  roles: Array<{
    name: string;
    role: string;
    description: string;
    assets: string[];
    exists: boolean; // 新增
    localId?: string; // 新增
  }>;
  assets: {
    skills: Array<{ name: string; description: string; exists: boolean }>;
    commands: Array<{ name: string; description: string; exists: boolean }>;
    subagents: Array<{ name: string; description: string; exists: boolean }>;
    rules: Array<{ name: string; description: string; exists: boolean }>;
    settings: Array<{ name: string; description: string; exists: boolean }>;
  };
  conflictCount: number; // 新增
}
```

#### 2.2 单包导入改造

**文件**：`web/src/pages/Market/TeamPackages.tsx`

修改 `renderPreviewModal()` 方法：

1. **增加冲突提示 Alert**
   - 当 `conflictCount > 0` 时显示警告 Alert
   - 提示内容：`检测到 ${conflictCount} 个冲突项，请选择处理方式`

2. **增加预览表格状态列**
   - 复用管理工具的表格样式
   - 状态列显示 Tag："已存在"(warning) 或 "新增"(success)
   - 资产类型标签颜色：Team(magenta), Role(geekblue), Skill(blue), Command(green), Subagent(purple), Rule(orange), Settings(cyan)

3. **修改底部按钮**
   - 无冲突时：显示"确认导入"按钮
   - 有冲突时：显示"全部覆盖"按钮 + "全部跳过"按钮
   - "全部覆盖"使用 Popconfirm 确认

4. **调用 syncPackage 时传递策略**
   ```typescript
   await api.teamPackages.syncPackage(pkg.name, { mode: 'overwrite' }, pkg.marketId);
   // 或
   await api.teamPackages.syncPackage(pkg.name, { mode: 'skip' }, pkg.marketId);
   ```

#### 2.3 批量导入改造

**文件**：`web/src/pages/Market/TeamPackages.tsx`

修改批量导入流程：

1. **批量预览**（可选优化）
   - 当前批量导入直接执行，改为先预览每个包
   - 或简化方案：在确认弹框中显示每个包的冲突统计（需要先调用预览API）

2. **修改确认弹框**
   - 当前只显示包列表
   - 改为显示包列表 + 每个包的冲突数量统计
   - 增加 Alert 提示："部分团队包存在冲突，请选择统一处理策略"

3. **策略选择按钮**
   - "全部覆盖"按钮（Popconfirm 确认）
   - "全部跳过"按钮

4. **批量执行时传递策略**
   ```typescript
   for (const pkg of pendingImportPackages) {
     await api.teamPackages.syncPackage(pkg.name, confirm, pkg.marketId);
   }
   ```

### 3. API 参数调整

**文件**：`web/src/api/client.ts`

`syncPackage` 方法参数调整：

```typescript
syncPackage: (
  packageName: string,
  confirm?: { mode: 'overwrite' | 'skip' },
  marketId?: string
): Promise<ImportResult> =>
  this.request('/team-package-sync/sync', 'POST', {
    packageName,
    confirm,
    marketId
  }),
```

后端 `SyncPackageRequest` 已支持 `confirm` 参数，无需修改。

## 改动文件清单

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/service/teampackagesync/types.go` | 修改 | 增加 Exists 字段 |
| `internal/service/teampackagesync/service.go` | 修改 | PreviewPackage 增加冲突检测逻辑 |
| `web/src/types/index.ts` | 修改 | PackagePreviewResponse 增加字段 |
| `web/src/pages/Market/TeamPackages.tsx` | 修改 | 单包/批量导入增加冲突处理 UI |
| `web/src/api/client.ts` | 修改 | syncPackage 参数类型调整 |

## 测试要点

1. **单包导入**
   - 新包导入：无冲突提示，直接导入
   - 已存在包导入：显示冲突，覆盖/跳过都能正确处理

2. **批量导入**
   - 全新包批量导入：无冲突提示，直接批量导入
   - 混合包批量导入：显示冲突统计，统一策略正确应用
   - 全已存在包批量导入：显示全部冲突，覆盖/跳过都能正确处理

3. **边界情况**
   - 包含部分冲突资产：仅冲突项按策略处理，新增项正常导入
   - 角色按名称匹配而非ID匹配（与管理工具按ID匹配不同，市场包角色ID可能无效）

## 风险与约束

1. **API 兼容性**：PreviewPackageResponse 增加字段是向后兼容的，旧前端可忽略新字段
2. **性能**：PreviewPackage 需多次查询 Repository，但预览操作频率低，影响可接受
3. **角色匹配策略**：市场包角色ID可能是无效UUID，采用名称匹配而非ID匹配