---
name: registry-sync-conflict
description: 联邦技能源页面手动同步按钮功能增强
type: project
---

# 联邦技能源手动同步冲突处理设计文档

**日期**: 2026-05-07
**状态**: Draft
**版本**: 1.0

---

## 1. 背景

### 1.1 当前状态

联邦技能源页面（`RegistryManagement`）有操作栏中的"同步"按钮：

- 点击后调用 `POST /api/v1/registries/:id/sync`
- 后端扫描联邦源所有 skills
- 只更新本地已存在的同名 skill（不创建新 skill）
- 无冲突检测，无用户交互

### 1.2 现有导入功能（已完成）

联邦源导入 skill 功能已实现冲突检测：
- 扫描 API 返回 `LocalSkillInfo`（本地同名 skill 信息）
- 前端展示冲突弹窗（`ConflictResolutionModal`）
- 用户选择"新建"或"更新"
- 批量操作按钮（"全部新建"、"全部更新"）

### 1.3 问题

用户期望手动同步按钮也能检测异源同名 skill，并提供弹窗让用户选择是否更新。

---

## 2. 需求规格

### 2.1 冲突处理策略

| 场景 | 检测条件 | 行为 |
|------|----------|------|
| 无同名 | 本地无同名 skill | 跳过（不创建新 skill） |
| 自动更新 | 本地有同名，且 `sourceRegistryId` 与当前联邦源相同 | 自动更新，不弹窗 |
| 用户选择 | 本地有同名，且来源不同（其他联邦源/个人/平台） | 弹窗让用户选择"更新"或"跳过" |

### 2.2 弹窗触发时机

**只在有冲突时弹窗**：
- 同源同名 → 自动更新，不弹窗
- 无同名 → 跳过，不弹窗
- 异源同名 → 弹窗让用户选择

### 2.3 弹窗设计

**复用现有 `ConflictResolutionModal` 组件**，适配调整：

| 原导入弹窗 | 同步弹窗适配 |
|-----------|-------------|
| "新建"按钮 | 改为"跳过"按钮 |
| "更新"按钮 | 保持不变 |
| "全部新建" | 改为"全部跳过" |
| "全部更新" | 保持不变 |

**展示内容**（详细版）：
- skill 名称
- 本地来源（sourceType + 联邦源名称如果是 federated）
- 远程来源（当前同步的联邦源名称）
- 本地描述（截断）
- 远程描述（截断）

---

## 3. API 设计

### 3.1 同步预览 API（新增）

```
POST /api/v1/registries/:id/sync-preview
```

**请求**：无参数

**响应**：

```json
{
  "registryId": "...",
  "registryName": "联邦源名称",
  "autoUpdateSkills": [
    {
      "name": "skill-1",
      "localSkillId": "...",
      "description": "..."
    }
  ],
  "conflictSkills": [
    {
      "name": "skill-2",
      "description": "远程描述...",
      "localSkill": {
        "id": "...",
        "sourceType": "personal",
        "sourceRegistryId": null,
        "sourceRegistryName": null,
        "description": "本地描述..."
      }
    }
  ],
  "newSkills": [
    {
      "name": "skill-3",
      "description": "..."
    }
  ],
  "skippedSkills": []
}
```

**字段说明**：

| 字段 | 说明 |
|------|------|
| `autoUpdateSkills` | 同源同名，自动更新（不弹窗） |
| `conflictSkills` | 异源同名，需要用户选择 |
| `newSkills` | 远程有但本地无（手动同步跳过） |
| `skippedSkills` | 本地有但远程无（跳过） |

### 3.2 同步确认 API（新增）

```
POST /api/v1/registries/:id/sync-confirm
```

**请求**：

```json
{
  "registryId": "...",
  "operations": [
    {
      "action": "update",  // 或 "skip"
      "skillName": "...",
      "targetSkillId": "..."  // 仅 update 时需要
    }
  ]
}
```

**响应**：

```json
{
  "updated": [
    { "id": "...", "name": "...", "sourceType": "federated" }
  ],
  "skipped": [
    { "name": "..." }
  ],
  "autoUpdated": 2,  // 自动更新数量（同源）
  "userUpdated": 3,  // 用户选择更新数量
  "userSkipped": 1   // 用户选择跳过数量
}
```

### 3.3 定时同步逻辑（保持不变）

后端定时任务同步只更新同源 skill，保持现有行为：

```go
// internal/service/skill/registry_service.go - Sync 方法保持不变
// 只更新 sourceRegistryId == registry.ID 的 skill
```

---

## 4. 前端交互流程

### 4.1 手动同步流程

```
用户点击同步按钮
    ↓
调用 sync-preview API
    ↓
分析冲突情况
    ├─ 无冲突 → 直接执行（自动更新同源，跳过其他）
    └─ 有冲突 → 弹出 ConflictResolutionModal
            ↓
        用户选择
            ↓
        调用 sync-confirm API
            ↓
        显示结果汇总
```

### 4.2 弹窗展示

**复用 `ConflictResolutionModal` 组件**，配置调整：

```typescript
<ConflictResolutionModal
  mode="sync"  // 新增 mode 参数区分导入/同步
  conflictSkills={conflictSkills}
  registryName={registryName}
  onConfirm={handleSyncConfirm}
  onCancel={handleCancel}
/>
```

**mode="sync" 时的 UI 变化**：
- "新建" → "跳过"
- "全部新建" → "全部跳过"
- 移除导入功能特有的字段（如 `importMode` 选择）

### 4.3 结果提示

```typescript
// 同步完成后显示
message.success(`同步完成：自动更新 ${result.autoUpdated} 个，用户更新 ${result.userUpdated} 个，跳过 ${result.userSkipped} 个`);
```

---

## 5. 后端实现要点

### 5.1 SyncPreview 方法（新增）

```go
// internal/service/skill/registry_service.go

func (s *RegistryService) SyncPreview(ctx context.Context, id uuid.UUID) (*model.SyncPreviewResult, error) {
    registry, err := s.registryRepo.FindByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("注册表不存在: %w", err)
    }

    // 扫描联邦源 skills（复用现有扫描逻辑）
    remoteSkills, err := s.scanRemoteSkills(ctx, registry)
    if err != nil {
        return nil, fmt.Errorf("扫描联邦源失败: %w", err)
    }

    result := &model.SyncPreviewResult{
        RegistryID:   registry.ID,
        RegistryName: registry.Name,
    }

    // 分析冲突情况
    for _, remoteSkill := range remoteSkills {
        existing, err := s.skillRepo.FindByName(ctx, remoteSkill.Name)
        if err != nil {
            // 本地无同名 → newSkills（手动同步跳过）
            result.NewSkills = append(result.NewSkills, remoteSkill)
            continue
        }

        // 本地有同名，检查来源
        if existing.SourceRegistryID == registry.ID {
            // 同源 → autoUpdateSkills
            result.AutoUpdateSkills = append(result.AutoUpdateSkills, &model.SyncPreviewSkill{
                Name:         remoteSkill.Name,
                LocalSkillID: existing.ID,
                Description:  remoteSkill.Description,
            })
        } else {
            // 异源 → conflictSkills
            conflictSkill := &model.SyncConflictSkill{
                Name:        remoteSkill.Name,
                Description: remoteSkill.Description,
                LocalSkill: &model.LocalSkillInfo{
                    ID:          existing.ID,
                    SourceType:  existing.SourceType,
                    Description: existing.Description,
                },
            }
            // 如果本地是 federated，查询联邦源名称
            if existing.SourceType == model.SkillSourceFederated {
                sourceRegistry, _ := s.registryRepo.FindByID(ctx, existing.SourceRegistryID)
                if sourceRegistry != nil {
                    conflictSkill.LocalSkill.SourceRegistryName = sourceRegistry.Name
                }
            }
            result.ConflictSkills = append(result.ConflictSkills, conflictSkill)
        }
    }

    return result, nil
}
```

### 5.2 SyncConfirm 方法（新增）

```go
func (s *RegistryService) SyncConfirm(ctx context.Context, id uuid.UUID, req *model.SyncConfirmRequest) (*model.SyncConfirmResult, error) {
    registry, err := s.registryRepo.FindByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("注册表不存在: %w", err)
    }

    result := &model.SyncConfirmResult{}

    // 执行用户选择的操作
    for _, op := range req.Operations {
        switch op.Action {
        case "update":
            existing, err := s.skillRepo.FindByID(ctx, op.TargetSkillID)
            if err != nil {
                continue
            }
            // 更新元数据
            existing.Description = op.Description
            existing.SourceType = model.SkillSourceFederated
            existing.SourceRegistryID = registry.ID
            existing.UpdatedAt = time.Now()
            s.skillRepo.Update(ctx, existing)
            result.Updated = append(result.Updated, existing)
            result.UserUpdated++
        case "skip":
            result.Skipped = append(result.Skipped, &model.SkippedSkill{Name: op.SkillName})
            result.UserSkipped++
        }
    }

    // 执行自动更新（同源）
    // ... 复用现有 Sync 方法的逻辑

    return result, nil
}
```

### 5.3 API Handler（新增）

```go
// internal/api/registry_handler.go

func (h *RegistryHandler) SyncPreview(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "无效的注册表 ID"})
        return
    }

    result, err := h.registrySvc.SyncPreview(c.Request.Context(), id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, result)
}

func (h *RegistryHandler) SyncConfirm(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "无效的注册表 ID"})
        return
    }

    var req model.SyncConfirmRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    result, err := h.registrySvc.SyncConfirm(c.Request.Context(), id, &req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, result)
}

// 注册路由
func (h *RegistryHandler) RegisterRoutes(r *gin.RouterGroup) {
    registries := r.Group("/registries")
    {
        // ... 现有路由
        registries.POST("/:id/sync-preview", h.SyncPreview)  // 新增
        registries.POST("/:id/sync-confirm", h.SyncConfirm)  // 新增
    }
}
```

---

## 6. 数据模型扩展

### 6.1 新增模型

```go
// internal/model/registry.go

// SyncPreviewResult 同步预览结果
type SyncPreviewResult struct {
    RegistryID      uuid.UUID           `json:"registryId"`
    RegistryName    string              `json:"registryName"`
    AutoUpdateSkills []*SyncPreviewSkill `json:"autoUpdateSkills"`  // 同源同名
    ConflictSkills  []*SyncConflictSkill `json:"conflictSkills"`    // 异源同名
    NewSkills       []*RemoteSkill      `json:"newSkills"`         // 远程有本地无
    SkippedSkills   []*RemoteSkill      `json:"skippedSkills"`     // 本地有远程无
}

// SyncPreviewSkill 同步预览 skill（同源）
type SyncPreviewSkill struct {
    Name         string    `json:"name"`
    LocalSkillID uuid.UUID `json:"localSkillId"`
    Description  string    `json:"description"`
}

// SyncConflictSkill 同步冲突 skill（异源）
type SyncConflictSkill struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    LocalSkill  *LocalSkillInfo `json:"localSkill"`  // 复用已有类型
}

// SyncConfirmRequest 同步确认请求
type SyncConfirmRequest struct {
    RegistryID uuid.UUID            `json:"registryId"`
    Operations []*SyncOperation     `json:"operations"`
}

// SyncOperation 同步操作
type SyncOperation struct {
    Action       string    `json:"action"`        // "update" 或 "skip"
    SkillName    string    `json:"skillName"`
    TargetSkillID uuid.UUID `json:"targetSkillId"` // 仅 update 时需要
    Description  string    `json:"description"`   // 远程 skill 描述
}

// SyncConfirmResult 同步确认结果
type SyncConfirmResult struct {
    Updated     []*Skill `json:"updated"`
    Skipped     []*SkippedSkill `json:"skipped"`
    AutoUpdated int     `json:"autoUpdated"`  // 自动更新数量
    UserUpdated int     `json:"userUpdated"`  // 用户选择更新数量
    UserSkipped int     `json:"userSkipped"`  // 用户选择跳过数量
}

// SkippedSkill 跳过的 skill
type SkippedSkill struct {
    Name string `json:"name"`
}
```

---

## 7. 前端类型扩展

```typescript
// web/src/types/index.ts

interface SyncPreviewResult {
  registryId: string;
  registryName: string;
  autoUpdateSkills: SyncPreviewSkill[];
  conflictSkills: SyncConflictSkill[];
  newSkills: RemoteSkill[];
  skippedSkills: RemoteSkill[];
}

interface SyncPreviewSkill {
  name: string;
  localSkillId: string;
  description: string;
}

interface SyncConflictSkill {
  name: string;
  description: string;
  localSkill: LocalSkillInfo;  // 复用已有类型
}

interface SyncConfirmRequest {
  registryId: string;
  operations: SyncOperation[];
}

interface SyncOperation {
  action: 'update' | 'skip';
  skillName: string;
  targetSkillId?: string;
  description: string;
}

interface SyncConfirmResult {
  updated: Skill[];
  skipped: SkippedSkill[];
  autoUpdated: number;
  userUpdated: number;
  userSkipped: number;
}
```

---

## 8. 冲突弹窗组件适配

### 8.1 ConflictResolutionModal 扩展

```typescript
// web/src/components/ConflictResolutionModal/index.tsx

interface ConflictResolutionModalProps {
  mode: 'import' | 'sync';  // 新增：区分导入/同步模式
  conflictSkills: SyncConflictSkill[];
  registryName: string;
  onConfirm: (decisions: ConflictDecision[]) => void;
  onCancel: () => void;
}

// 根据 mode 调整 UI
const getActionLabel = (mode: string, action: string) => {
  if (mode === 'sync') {
    return action === 'update' ? '更新' : '跳过';
  }
  return action === 'update' ? '更新' : '新建';
};

const getBatchActionLabel = (mode: string, action: string) => {
  if (mode === 'sync') {
    return action === 'update' ? '全部更新' : '全部跳过';
  }
  return action === 'update' ? '全部更新' : '全部新建';
};
```

---

## 9. API Client 扩展

```typescript
// web/src/api/client.ts

export const registries = {
  // ... 现有方法
  
  syncPreview: async (id: string): Promise<SyncPreviewResult> => {
    const response = await axios.post(`/api/v1/registries/${id}/sync-preview`);
    return response.data;
  },
  
  syncConfirm: async (id: string, request: SyncConfirmRequest): Promise<SyncConfirmResult> => {
    const response = await axios.post(`/api/v1/registries/${id}/sync-confirm`, request);
    return response.data;
  },
};
```

---

## 10. 测试场景

| 场景 | 输入 | 期望输出 |
|------|------|----------|
| 无冲突同步 | 联邦源无同名 skill 或全部同源 | 直接执行，不弹窗 |
| 有冲突同步 | 存在异源同名 skill | 弹窗让用户选择 |
| 用户选择更新 | 冲突弹窗选择"更新" | 更新 skill，来源变更为联邦源 |
| 用户选择跳过 | 冲突弹窗选择"跳过" | 不更新 skill，保持原有状态 |
| 批量全部更新 | 冲突弹窗选择"全部更新" | 所有冲突项都更新 |
| 批量全部跳过 | 冲突弹窗选择"全部跳过" | 所有冲突项都跳过 |
| 混合场景 | 同源 + 异源 + 无同名 | 同源自动更新，异源弹窗，无同名跳过 |

---

## 11. 实施范围

### 11.1 本次实施包含

- 后端新增 `SyncPreview` API 和服务方法
- 后端新增 `SyncConfirm` API 和服务方法
- 后端新增相关数据模型
- 前端新增 API 调用方法
- 前端 `ConflictResolutionModal` 组件扩展（支持 sync 模式）
- 前端 `RegistryManagement` 页面 `handleSync` 逻辑修改

### 11.2 本次实施不包含

- 定时同步逻辑修改（保持不变，只更新同源）
- 新建 skill 功能（手动同步不创建新 skill）
- 历史版本管理

---

## 12. 验收标准

1. 点击同步按钮时，无冲突直接执行（显示结果汇总）
2. 点击同步按钮时，有冲突弹出弹窗展示详细信息
3. 弹窗展示完整信息（名称、本地来源、远程来源、描述对比）
4. 弹窗提供"更新"和"跳过"两个选项
5. 弹窗提供"全部更新"和"全部跳过"批量操作按钮
6. 选择"更新"后，skill 来源变更为联邦源
7. 选择"跳过"后，skill 保持原有状态
8. 同步完成后显示结果汇总（自动更新、用户更新、跳过数量）