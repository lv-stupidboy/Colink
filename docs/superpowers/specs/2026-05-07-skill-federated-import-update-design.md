---
name: skill-federated-import-update
description: 联邦源导入 skill 时支持更新现有同名 skill
type: project
---

# Skill 联邦源导入更新功能设计文档

**日期**: 2026-05-07
**状态**: Draft
**版本**: 1.0

---

## 1. 背景

### 1.1 当前状态

- skill 数据模型允许重名（无唯一约束）
- 联邦源扫描时，`existsLocally=true` 的 skill 在前端被**禁用**，无法选择
- 批量导入时直接创建新记录，没有"更新"逻辑
- 用户无法通过联邦源导入来更新已有的 skill

### 1.2 问题

用户希望通过联邦源导入同名 skill 时，能够选择**更新**现有记录，而非只能创建新记录。

### 1.3 现有数据结构

```go
// internal/model/skill.go
type Skill struct {
    ID               uuid.UUID       `json:"id"`
    Name             string          `json:"name"`
    Description      string          `json:"description,omitempty"`
    Tags             []string        `json:"tags,omitempty"`
    SourceType       SkillSourceType `json:"sourceType"`       // platform/personal/federated
    SourceRegistryID uuid.UUID       `json:"sourceRegistryId,omitempty"`
    // ... 其他字段
}

type SkillSourceType string
const (
    SkillSourcePlatform  SkillSourceType = "platform"  // 平台内置
    SkillSourcePersonal  SkillSourceType = "personal"  // 个人上传
    SkillSourceFederated SkillSourceType = "federated" // 联邦同步
)
```

---

## 2. 需求规格

### 2.1 冲突处理策略

| 场景 | 检测条件 | 行为 |
|------|----------|------|
| 无冲突 | 本地无同名 skill | 直接创建新记录 |
| 自动更新 | 本地有同名 skill，且 `sourceRegistryId` 与当前联邦源相同 | 自动更新，无需用户确认 |
| 用户选择 | 本地有同名 skill，且来源不同（其他联邦源/个人/平台） | 弹窗让用户选择 |

### 2.2 冲突弹窗设计

**触发时机**：批量导入确认时，检测到需要用户选择的冲突项

**展示内容**（完整信息）：

| 列 | 内容 |
|----|------|
| 名称 | skill 名称 |
| 本地来源 | `sourceType` + 联邦源名称（如果是 federated） |
| 远程来源 | 当前导入的联邦源名称 |
| 本地描述 | 本地 skill 的描述（截断） |
| 远程描述 | 远程 skill 的描述（截断） |

**用户操作**：

每个冲突项可选择：
- **新建** — 创建新的 skill 记录（允许重名）
- **更新** — 覆盖现有 skill，同时修改来源为联邦源

**批量操作按钮**：
- **全部新建** — 所有冲突项都选择"新建"
- **全部更新** — 所有冲突项都选择"更新"
- **取消** — 取消整个导入操作

### 2.3 更新行为

选择"更新"时的操作：

1. 覆盖现有 skill 的以下字段：
   - `description`（从远程 skill 替换）
   - `tags`（从远程 skill 替换）
   - `supportedAgents`（从远程 skill 替换）

2. 修改来源信息：
   - `sourceType` → `federated`
   - `sourceRegistryId` → 当前联邦源 ID

3. 更新文件目录：
   - 清空原有 skill 目录内容
   - 复制远程 skill 文件到原目录（保持原 skill ID）

### 2.4 扫描结果展示变更

**当前**：`existsLocally=true` 的 skill 被禁用（不可选择）

**变更后**：`existsLocally=true` 的 skill **可选择**

展示区分：
- 已存在本地：显示来源标记（如 "来自: 我的联邦源"）
- 不存在本地：正常显示

---

## 3. API 设计

### 3.1 扫描结果扩展

**响应结构调整**：

```json
// GET /api/v1/skills/import/federated/scan
{
  "registryId": "...",
  "registryName": "联邦源名称",
  "skills": [
    {
      "name": "skill-name",
      "description": "...",
      "path": "skills/skill-name",
      "existsLocally": true,
      "localSkill": {  // 新增：本地同名 skill 信息（如果存在）
        "id": "...",
        "sourceType": "personal",
        "sourceRegistryId": "...",
        "sourceRegistryName": "另一个联邦源",  // 如果是 federated
        "description": "本地描述..."
      }
    }
  ]
}
```

### 3.2 批量导入请求扩展

**请求结构调整**：

```json
// POST /api/v1/skills/import/federated/batch
{
  "registryId": "...",
  "skills": [
    {
      "name": "...",
      "path": "...",
      "description": "...",
      "tags": [...],
      "supportedAgents": [...],
      "importMode": "create"  // 新增：create 或 update
    }
  ],
  "updateSkillId": "..."  // 新增：如果 importMode=update，指定要更新的 skill ID
}
```

**简化方案**：改为扁平结构，每个 skill 明确指定操作：

```json
{
  "registryId": "...",
  "operations": [
    {
      "action": "create",  // 或 "update"
      "skill": {
        "name": "...",
        "path": "...",
        // ...
      },
      "targetSkillId": "..."  // 仅 update 时需要
    }
  ]
}
```

**决策**：保持原有 `skills` 数组结构，通过扩展字段 `importMode` 和 `updateSkillId` 实现。

### 3.3 批量导入响应扩展

```json
{
  "imported": [...],   // 创建的新 skill
  "updated": [...],    // 更新的 skill（新增）
  "skipped": [...],
  "conflictSummary": {  // 新增：冲突处理汇总
    "autoUpdated": 2,   // 自动更新的数量
    "userCreated": 1,   // 用户选择新建的数量
    "userUpdated": 3    // 用户选择更新的数量
  }
}
```

---

## 4. 前端交互流程

### 4.1 扫描阶段

1. 用户选择联邦源 → 点击"导入"
2. 调用扫描 API → 返回 skill 列表（含 `localSkill` 信息）
3. 展示列表：
   - `existsLocally=false`：正常显示，可选择
   - `existsLocally=true`：显示来源标记，可选择

### 4.2 确认导入阶段

1. 用户勾选 skill → 点击"确认导入"
2. 前端分析冲突情况：
   - 无冲突项 → 直接调用批量导入 API
   - 有冲突项 → 弹出冲突选择弹窗

3. 冲突弹窗：
   - 展示所有冲突项（完整信息）
   - 用户逐个选择或批量选择
   - 确认后调用批量导入 API（带 `importMode` 字段）

4. 导入完成：
   - 显示导入结果汇总
   - 刷新 skill 列表

### 4.3 冲突分析逻辑（前端）

```typescript
function analyzeConflicts(selectedSkills: RemoteSkill[], registryId: string): {
  autoUpdateItems: RemoteSkill[];    // 同源，自动更新
  conflictItems: RemoteSkill[];       // 异源，需要用户选择
  createItems: RemoteSkill[];         // 无同名，直接创建
} {
  // 分类逻辑...
}
```

---

## 5. 后端实现要点

### 5.1 扫描服务扩展

`SkillScanner.ScanRegistry()` 需扩展：

```go
// 查询本地同名 skill 的详细信息
for _, skill := range skills {
    if skill.ExistsLocally {
        existing, err := s.skillRepo.FindByName(ctx, skill.Name)
        if existing != nil {
            skill.LocalSkill = &LocalSkillInfo{
                ID:               existing.ID,
                SourceType:       existing.SourceType,
                SourceRegistryID: existing.SourceRegistryID,
                Description:      existing.Description,
            }
            // 如果是 federated，查询联邦源名称
            if existing.SourceType == model.SkillSourceFederated {
                registry, _ := s.registryRepo.FindByID(ctx, existing.SourceRegistryID)
                if registry != nil {
                    skill.LocalSkill.SourceRegistryName = registry.Name
                }
            }
        }
    }
}
```

### 5.2 批量导入服务扩展

`SkillScanner.ImportSkills()` 需支持更新操作：

```go
func (s *SkillScanner) ImportSkills(ctx context.Context, req *model.BatchImportRequest) (*model.BatchImportResult, error) {
    // ...
    for _, item := range req.Skills {
        switch item.ImportMode {
        case "create":
            // 原有的创建逻辑
        case "update":
            // 新增的更新逻辑
            existing, err := s.skillRepo.FindByID(ctx, item.TargetSkillID)
            if err != nil {
                return nil, fmt.Errorf("找不到目标 skill: %w", err)
            }
            // 更新元数据
            existing.Description = item.Description
            existing.Tags = mergeTags(existing.Tags, item.Tags)
            existing.SupportedAgents = mergeAgents(existing.SupportedAgents, item.SupportedAgents)
            existing.SourceType = model.SkillSourceFederated
            existing.SourceRegistryID = req.RegistryID
            // 更新文件目录
            s.updateSkillFiles(existing.ID, srcDir)
            // 保存
            s.skillRepo.Update(ctx, existing)
        }
    }
}
```

### 5.3 文件更新逻辑

更新时如何处理 skill 目录？

**方案 A**：删除旧目录，创建新目录（按旧 ID）
- 问题：如果用户有引用文件路径，会丢失

**方案 B**：直接覆盖旧目录内容
- 优点：保持 ID 和路径不变
- 实现：清空旧目录，复制新文件

**决策**：采用方案 B，保持原 skill ID 和目录名不变。

---

## 6. 数据模型扩展

### 6.1 RemoteSkill 扩展

```go
// internal/model/skill.go
type RemoteSkill struct {
    Name          string          `json:"name"`
    Description   string          `json:"description"`
    Path          string          `json:"path"`
    ExistsLocally bool            `json:"existsLocally"`
    LocalSkill    *LocalSkillInfo `json:"localSkill,omitempty"` // 新增
}

type LocalSkillInfo struct {
    ID               uuid.UUID `json:"id"`
    SourceType       string    `json:"sourceType"`
    SourceRegistryID uuid.UUID `json:"sourceRegistryId,omitempty"`
    SourceRegistryName string  `json:"sourceRegistryName,omitempty"` // 联邦源名称（如果是 federated）
    Description      string    `json:"description"`
}
```

### 6.2 SkillImportItem 扩展

```go
type SkillImportItem struct {
    Name            string          `json:"name" binding:"required"`
    Path            string          `json:"path" binding:"required"`
    Description     string          `json:"description"`
    Tags            []string        `json:"tags"`
    SupportedAgents []string        `json:"supportedAgents" binding:"required,min=1"`
    ImportMode      string          `json:"importMode"`       // 新增：create 或 update
    TargetSkillID   uuid.UUID       `json:"targetSkillId"`    // 新增：update 时指定目标 ID
}
```

### 6.3 BatchImportResult 扩展

```go
type BatchImportResult struct {
    Imported       []*Skill           `json:"imported"`
    Updated        []*Skill           `json:"updated"`         // 新增
    Skipped        []SkippedSkillInfo `json:"skipped"`
    ConflictSummary *ConflictSummary `json:"conflictSummary,omitempty"` // 新增
}

type ConflictSummary struct {
    AutoUpdated int `json:"autoUpdated"`
    UserCreated int `json:"userCreated"`
    UserUpdated int `json:"userUpdated"`
}
```

---

## 7. 前端类型扩展

```typescript
// web/src/types/index.ts
interface RemoteSkill {
  name: string;
  description: string;
  path: string;
  existsLocally: boolean;
  localSkill?: LocalSkillInfo;  // 新增
}

interface LocalSkillInfo {
  id: string;
  sourceType: 'platform' | 'personal' | 'federated';
  sourceRegistryId?: string;
  sourceRegistryName?: string;  // 联邦源名称
  description: string;
}

interface SkillImportItem {
  name: string;
  path: string;
  description?: string;
  tags?: string[];
  supportedAgents: string[];
  importMode?: 'create' | 'update';  // 新增
  targetSkillId?: string;            // 新增
}

interface BatchImportResult {
  imported: Skill[];
  updated: Skill[];                  // 新增
  skipped: SkippedSkillInfo[];
  conflictSummary?: ConflictSummary; // 新增
}
```

---

## 8. 测试场景

| 场景 | 输入 | 期望输出 |
|------|------|----------|
| 全新导入 | 选择不存在的 skill | 直接创建，无弹窗 |
| 同源更新 | 选择同名但同源 skill | 自动更新，无弹窗 |
| 异源冲突 | 选择同名但异源 skill | 弹窗让用户选择 |
| 混合导入 | 选择多个 skill，部分冲突 | 无冲突部分自动处理，冲突部分弹窗 |
| 批量新建 | 冲突弹窗选择"全部新建" | 所有冲突项创建新记录 |
| 批量更新 | 冲突弹窗选择"全部更新" | 所有冲突项更新现有记录 |

---

## 9. 开放问题（已确认）

| 问题 | 答案 |
|------|------|
| 批量导入时弹窗如何处理？ | 统一弹窗，一个弹窗列出所有冲突 |
| 更新时是否保留历史版本？ | 不保留 |
| 更新时是否显示版本变更说明？ | 不显示 |
| 更新时是否记录更新来源？ | 不记录 |
| 冲突弹窗展示哪些信息？ | 完整信息（名称 + 本地来源 + 远程来源 + 描述对比） |
| 是否需要批量操作按钮？ | 需要（"全部新建"、"全部更新"） |
| 更新时 Tags/SupportedAgents 策略？ | 替换（完全使用远程 skill 的值） |
| 新建同名 skill 如何区分？ | 不加处理，允许完全重名，用户通过来源标签区分 |
| 是否需要权限校验？ | 不需要，用户可更新任何同名 skill（包括 platform） |

---

## 10. 实施范围

### 10.1 本次实施包含

- 扫描 API 返回本地 skill 详细信息
- 批量导入 API 支持更新模式
- 前端扫描结果展示变更（允许选择已存在项）
- 前端冲突检测与弹窗组件
- 前端批量导入逻辑扩展

### 10.2 本次实施不包含

- 历史版本管理
- 版本变更说明展示
- 更新来源记录
- 定时自动同步更新

---

## 11. 验收标准

1. 用户可以选择已存在本地的 skill 进行导入
2. 同源 skill 自动更新，无需用户确认
3. 异源 skill 弹窗让用户选择"新建"或"更新"
4. 弹窗展示完整冲突信息（名称、来源、描述）
5. 弹窗提供批量操作按钮（"全部新建"、"全部更新"）
6. 更新后 skill 的来源信息正确变更
7. 更新后 skill 文件内容正确更新