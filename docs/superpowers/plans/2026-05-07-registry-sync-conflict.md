# 联邦技能源手动同步冲突处理实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现联邦技能源页面手动同步按钮检测异源同名 skill 时提供弹窗让用户选择"更新"或"跳过"。

**Architecture:** 后端新增 sync-preview 和 sync-confirm 两个 API，前端复用现有冲突弹窗逻辑（适配 sync 模式）并修改 RegistryManagement 页面同步流程。

**Tech Stack:** Go (后端)、React + Ant Design (前端)、TypeScript

---

## File Structure

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/model/skill.go` | Modify | 新增 SyncPreviewResult、SyncConflictSkill 等类型 |
| `internal/service/skill/registry_service.go` | Modify | 新增 SyncPreview 和 SyncConfirm 方法 |
| `internal/api/registry_handler.go` | Modify | 新增 SyncPreview 和 SyncConfirm API |
| `web/src/types/index.ts` | Modify | 新增 SyncPreviewResult 等前端类型 |
| `web/src/api/client.ts` | Modify | 新增 syncPreview 和 syncConfirm API 方法 |
| `web/src/pages/RegistryManagement/index.tsx` | Modify | 添加冲突检测逻辑和冲突弹窗 |

---

## Task 1: 扩展后端数据模型

**Files:**
- Modify: `internal/model/skill.go` (在 SyncResult 之后添加新类型)

- [ ] **Step 1: 添加 SyncPreviewSkill 结构体**

在 `internal/model/skill.go` 文件中，在 `SyncResult` 结构体定义之后（约第 188 行之后）添加：

```go
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
	LocalSkill  *LocalSkillInfo `json:"localSkill"` // 复用已有类型
}

// SyncPreviewResult 同步预览结果
type SyncPreviewResult struct {
	RegistryID      uuid.UUID           `json:"registryId"`
	RegistryName    string              `json:"registryName"`
	AutoUpdateSkills []*SyncPreviewSkill `json:"autoUpdateSkills"`  // 同源同名
	ConflictSkills  []*SyncConflictSkill `json:"conflictSkills"`    // 异源同名
	NewSkills       []*RemoteSkill      `json:"newSkills"`         // 远程有本地无
	SkippedSkills   []*RemoteSkill      `json:"skippedSkills"`     // 本地有远程无
}

// SyncOperation 同步操作
type SyncOperation struct {
	Action       string    `json:"action"`        // "update" 或 "skip"
	SkillName    string    `json:"skillName"`
	TargetSkillID uuid.UUID `json:"targetSkillId"` // 仅 update 时需要
	Description  string    `json:"description"`   // 远程 skill 描述
}

// SyncConfirmRequest 同步确认请求
type SyncConfirmRequest struct {
	RegistryID uuid.UUID        `json:"registryId"`
	Operations []*SyncOperation `json:"operations"`
}

// SkippedSkill 跳过的 skill
type SkippedSkill struct {
	Name string `json:"name"`
}

// SyncConfirmResult 同步确认结果
type SyncConfirmResult struct {
	Updated     []*Skill       `json:"updated"`
	Skipped     []*SkippedSkill `json:"skipped"`
	AutoUpdated int            `json:"autoUpdated"`  // 自动更新数量
	UserUpdated int            `json:"userUpdated"`  // 用户选择更新数量
	UserSkipped int            `json:"userSkipped"`  // 用户选择跳过数量
}
```

- [ ] **Step 2: 运行后端测试验证模型变更**

```bash
cd D:\workspace\isdp
go build ./internal/model/...
```

Expected: 编译成功，无错误

- [ ] **Step 3: Commit 数据模型变更**

```bash
git add internal/model/skill.go
git commit -m "feat(skill): add sync preview and confirm models for registry conflict handling"
```

---

## Task 2: 实现后端 SyncPreview 方法

**Files:**
- Modify: `internal/service/skill/registry_service.go` (在 Sync 方法之后添加新方法)

- [ ] **Step 1: 添加 SyncPreview 方法**

在 `internal/service/skill/registry_service.go` 文件中，在 `Sync` 方法之后（约第 180 行之后）添加：

```go
// SyncPreview 同步预览（返回冲突情况，不执行更新）
func (s *RegistryService) SyncPreview(ctx context.Context, id uuid.UUID) (*model.SyncPreviewResult, error) {
	registry, err := s.registryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("注册表不存在: %w", err)
	}

	// 扫描联邦源 skills（复用现有扫描逻辑）
	var remoteSkills []*RemoteSkill
	switch registry.Type {
	case model.RegistryTypeGitHub:
		remoteSkills, err = s.syncFromGitHub(ctx, registry)
	case model.RegistryTypeGitLab:
		remoteSkills, err = s.syncFromGitLab(ctx, registry)
	case model.RegistryTypeAPI:
		remoteSkills, err = s.syncFromAPI(ctx, registry)
	case model.RegistryTypeCustom, model.RegistryTypeCodeHub:
		remoteSkills, err = s.syncFromGitRepo(ctx, registry)
	default:
		err = fmt.Errorf("不支持的注册表类型: %s", registry.Type)
	}

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
			result.NewSkills = append(result.NewSkills, &model.RemoteSkill{
				Name:        remoteSkill.Name,
				Description: remoteSkill.Description,
				Path:        "",
			})
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
					SourceType:  string(existing.SourceType),
					Description: existing.Description,
				},
			}
			// 如果本地是 federated，查询联邦源名称
			if existing.SourceType == model.SkillSourceFederated && existing.SourceRegistryID != uuid.Nil {
				sourceRegistry, _ := s.registryRepo.FindByID(ctx, existing.SourceRegistryID)
				if sourceRegistry != nil {
					conflictSkill.LocalSkill.SourceRegistryID = existing.SourceRegistryID
					conflictSkill.LocalSkill.SourceRegistryName = sourceRegistry.Name
				}
			}
			result.ConflictSkills = append(result.ConflictSkills, conflictSkill)
		}
	}

	return result, nil
}
```

- [ ] **Step 2: 运行后端编译验证**

```bash
go build ./internal/service/skill/...
```

Expected: 编译成功，无错误

- [ ] **Step 3: Commit SyncPreview 方法**

```bash
git add internal/service/skill/registry_service.go
git commit -m "feat(skill): add SyncPreview method for conflict detection before sync"
```

---

## Task 3: 实现后端 SyncConfirm 方法

**Files:**
- Modify: `internal/service/skill/registry_service.go` (在 SyncPreview 方法之后添加)

- [ ] **Step 1: 添加 SyncConfirm 方法**

在 `SyncPreview` 方法之后添加：

```go
// SyncConfirm 同步确认（执行用户选择的更新操作）
func (s *RegistryService) SyncConfirm(ctx context.Context, id uuid.UUID, req *model.SyncConfirmRequest) (*model.SyncConfirmResult, error) {
	registry, err := s.registryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("注册表不存在: %w", err)
	}

	result := &model.SyncConfirmResult{}

	// 先执行自动更新（同源）
	// 扫描联邦源获取 skills 信息
	var remoteSkills []*RemoteSkill
	switch registry.Type {
	case model.RegistryTypeGitHub:
		remoteSkills, err = s.syncFromGitHub(ctx, registry)
	case model.RegistryTypeGitLab:
		remoteSkills, err = s.syncFromGitLab(ctx, registry)
	case model.RegistryTypeAPI:
		remoteSkills, err = s.syncFromAPI(ctx, registry)
	case model.RegistryTypeCustom, model.RegistryTypeCodeHub:
		remoteSkills, err = s.syncFromGitRepo(ctx, registry)
	}

	if err != nil {
		return nil, fmt.Errorf("扫描联邦源失败: %w", err)
	}

	// 创建 remoteSkills 的 name -> skill 映射
	remoteSkillMap := make(map[string]*RemoteSkill)
	for _, skill := range remoteSkills {
		remoteSkillMap[skill.Name] = skill
	}

	// 执行自动更新（同源同名）
	for _, skill := range remoteSkills {
		existing, err := s.skillRepo.FindByName(ctx, skill.Name)
		if err != nil {
			continue // 不存在，跳过
		}
		if existing.SourceRegistryID == registry.ID {
			// 同源，自动更新
			existing.Description = skill.Description
			existing.Tags = skill.Tags
			existing.SupportedAgents = skill.SupportedAgents
			existing.UpdatedAt = time.Now()
			if err := s.skillRepo.Update(ctx, existing); err != nil {
				continue
			}
			result.AutoUpdated++
		}
	}

	// 执行用户选择的操作
	for _, op := range req.Operations {
		switch op.Action {
		case "update":
			existing, err := s.skillRepo.FindByID(ctx, op.TargetSkillID)
			if err != nil {
				result.Skipped = append(result.Skipped, &model.SkippedSkill{Name: op.SkillName})
				continue
			}
			// 获取远程 skill 信息
			remoteSkill := remoteSkillMap[op.SkillName]
			if remoteSkill == nil {
				result.Skipped = append(result.Skipped, &model.SkippedSkill{Name: op.SkillName})
				continue
			}
			// 更新元数据
			existing.Description = op.Description
			if len(remoteSkill.Tags) > 0 {
				existing.Tags = remoteSkill.Tags
			}
			if len(remoteSkill.SupportedAgents) > 0 {
				existing.SupportedAgents = remoteSkill.SupportedAgents
			}
			existing.SourceType = model.SkillSourceFederated
			existing.SourceRegistryID = registry.ID
			existing.UpdatedAt = time.Now()
			if err := s.skillRepo.Update(ctx, existing); err != nil {
				result.Skipped = append(result.Skipped, &model.SkippedSkill{Name: op.SkillName})
				continue
			}
			result.Updated = append(result.Updated, existing)
			result.UserUpdated++
		case "skip":
			result.Skipped = append(result.Skipped, &model.SkippedSkill{Name: op.SkillName})
			result.UserSkipped++
		}
	}

	// 更新同步状态
	s.registryRepo.UpdateSyncStatus(ctx, id, model.RegistrySyncSuccess, result.AutoUpdated+result.UserUpdated)

	return result, nil
}
```

- [ ] **Step 2: 检查 skillRepo 是否有 FindByID 方法**

```bash
grep -n "FindByID" internal/repo/skill.go
```

Expected: 如果不存在，需要添加 FindByID 方法

- [ ] **Step 3: 如果需要，添加 FindByID 方法**

如果 `internal/repo/skill.go` 中没有 `FindByID` 方法，在 `FindByName` 方法之后添加：

```go
// FindByID 根据 ID 查找
func (r *SkillRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Skill, error) {
	query := `
		SELECT id, name, description, tags, source_type, source_registry_id, author_id, project_id, supported_agents, use_count, status, is_public, created_at, updated_at
		FROM skills WHERE id = ?
	`
	skill, err := scanSkill(r.DB().QueryRowContext(ctx, query, id))
	if err != nil {
		return nil, err
	}
	return skill, nil
}
```

- [ ] **Step 4: 运行后端编译验证**

```bash
go build ./internal/service/skill/... ./internal/repo/...
```

Expected: 编译成功，无错误

- [ ] **Step 5: Commit SyncConfirm 方法**

```bash
git add internal/service/skill/registry_service.go internal/repo/skill.go
git commit -m "feat(skill): add SyncConfirm method to execute user-selected sync operations"
```

---

## Task 4: 添加后端 API Handler

**Files:**
- Modify: `internal/api/registry_handler.go` (在 Sync 方法之后添加新方法)

- [ ] **Step 1: 添加 SyncPreview Handler**

在 `internal/api/registry_handler.go` 文件中，在 `Sync` 方法之后（约第 146 行之后）添加：

```go
// SyncPreview 同步预览
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

// SyncConfirm 同步确认
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

	// 确保 RegistryID 与 URL 参数一致
	req.RegistryID = id

	result, err := h.registrySvc.SyncConfirm(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
```

- [ ] **Step 2: 注册新路由**

修改 `RegisterRoutes` 方法（约第 163-174 行），添加新路由：

```go
// RegisterRoutes 注册路由
func (h *RegistryHandler) RegisterRoutes(r *gin.RouterGroup) {
	registries := r.Group("/registries")
	{
		registries.GET("", h.List)
		registries.POST("", h.Create)
		registries.GET("/:id", h.Get)
		registries.PUT("/:id", h.Update)
		registries.DELETE("/:id", h.Delete)
		registries.POST("/:id/sync", h.Sync)
		registries.POST("/:id/sync-preview", h.SyncPreview)  // 新增
		registries.POST("/:id/sync-confirm", h.SyncConfirm)  // 新增
		registries.POST("/sync", h.SyncAll)
	}
}
```

- [ ] **Step 3: 运行后端编译验证**

```bash
go build ./internal/api/...
```

Expected: 编译成功，无错误

- [ ] **Step 4: Commit API Handler 变更**

```bash
git add internal/api/registry_handler.go
git commit -m "feat(skill): add sync-preview and sync-confirm API endpoints"
```

---

## Task 5: 扩展前端 TypeScript 类型

**Files:**
- Modify: `web/src/types/index.ts` (在 SyncResult 之后添加新类型)

- [ ] **Step 1: 添加同步相关类型**

在 `web/src/types/index.ts` 文件中，在 `SyncResult` 类型之后（约第 752 行之后）添加：

```typescript
// ========== 同步冲突处理相关类型 ==========

// 同步预览 skill（同源）
export interface SyncPreviewSkill {
  name: string;
  localSkillId: string;
  description: string;
}

// 同步冲突 skill（异源）
export interface SyncConflictSkill {
  name: string;
  description: string;
  localSkill: LocalSkillInfo; // 复用已有类型
}

// 同步预览结果
export interface SyncPreviewResult {
  registryId: string;
  registryName: string;
  autoUpdateSkills: SyncPreviewSkill[]; // 同源同名
  conflictSkills: SyncConflictSkill[];  // 异源同名
  newSkills: RemoteSkill[];             // 远程有本地无
  skippedSkills: RemoteSkill[];         // 本地有远程无
}

// 同步操作
export interface SyncOperation {
  action: 'update' | 'skip';
  skillName: string;
  targetSkillId?: string; // 仅 update 时需要
  description: string;    // 远程 skill 描述
}

// 同步确认请求
export interface SyncConfirmRequest {
  registryId: string;
  operations: SyncOperation[];
}

// 同步确认结果
export interface SyncConfirmResult {
  updated: Skill[];
  skipped: SkippedSkill[];
  autoUpdated: number;  // 自动更新数量
  userUpdated: number;  // 用户选择更新数量
  userSkipped: number;  // 用户选择跳过数量
}

// 跳过的 skill
export interface SkippedSkill {
  name: string;
}
```

- [ ] **Step 2: 运行前端类型检查**

```bash
cd web && npx tsc --noEmit
```

Expected: 类型检查通过，无错误

- [ ] **Step 3: Commit 类型定义变更**

```bash
git add web/src/types/index.ts
git commit -m "feat(skill): add sync preview and confirm types for frontend"
```

---

## Task 6: 扩展前端 API Client

**Files:**
- Modify: `web/src/api/client.ts` (在 registries 对象中添加新方法)

- [ ] **Step 1: 导入新类型**

在 `web/src/api/client.ts` 的类型导入区域（约第 26 行），添加新类型：

```typescript
import type {
  // ... 其他类型
  SyncPreviewResult,
  SyncConfirmRequest,
  SyncConfirmResult,
} from '@/types';
```

- [ ] **Step 2: 添加 syncPreview 和 syncConfirm 方法**

在 `registries` 对象中（约第 544-548 行），添加新方法：

```typescript
  registries = {
    // ... 现有方法
    sync: (id: string): Promise<SyncResult> =>
      this.request(`/registries/${id}/sync`, 'POST'),
    syncPreview: (id: string): Promise<SyncPreviewResult> =>
      this.request(`/registries/${id}/sync-preview`, 'POST'),
    syncConfirm: (id: string, request: SyncConfirmRequest): Promise<SyncConfirmResult> =>
      this.request(`/registries/${id}/sync-confirm`, 'POST', request),
    syncAll: (): Promise<{ message: string; results: SyncResult[] }> =>
      this.request('/registries/sync', 'POST'),
  };
```

- [ ] **Step 3: 运行前端类型检查**

```bash
cd web && npx tsc --noEmit
```

Expected: 类型检查通过，无错误

- [ ] **Step 4: Commit API Client 变更**

```bash
git add web/src/api/client.ts
git commit -m "feat(skill): add syncPreview and syncConfirm API methods"
```

---

## Task 7: 修改 RegistryManagement 页面添加冲突检测

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx`

- [ ] **Step 1: 添加新的状态和类型导入**

在文件顶部导入区域添加新类型：

```typescript
import type {
  SkillRegistry,
  CreateRegistryRequest,
  RegistryType,
  RegistryStatus,
  SyncPreviewResult,
  SyncConflictSkill,
  SyncOperation,
  SyncConfirmResult,
  SkillSourceType,
} from '@/types';
```

添加新的状态变量（约第 44 行之后）：

```typescript
  const [syncPreview, setSyncPreview] = useState<SyncPreviewResult | null>(null);
  const [conflictModalVisible, setConflictModalVisible] = useState(false);
  const [conflictChoices, setConflictChoices] = useState<Record<string, 'update' | 'skip'>>({});
  const [syncingRegistryId, setSyncingRegistryId] = useState<string | null>(null);
  const [syncingRegistryName, setSyncingRegistryName] = useState('');
```

- [ ] **Step 2: 添加辅助函数**

在组件内部添加辅助函数：

```typescript
  // 获取来源类型颜色
  const getSourceTypeColor = (sourceType: SkillSourceType): string => {
    const colors: Record<SkillSourceType, string> = {
      personal: 'blue',
      platform: 'green',
      federated: 'purple',
    };
    return colors[sourceType] || 'default';
  };

  // 获取来源类型标签
  const getSourceTypeLabel = (sourceType: SkillSourceType): string => {
    const labels: Record<SkillSourceType, string> = {
      personal: '个人',
      platform: '平台',
      federated: '联邦源',
    };
    return labels[sourceType] || sourceType;
  };
```

- [ ] **Step 3: 修改 handleSync 函数**

替换 `handleSync` 函数（约第 114-129 行）：

```typescript
  const handleSync = async (id: string) => {
    setSyncingId(id);
    try {
      // 先调用 sync-preview API
      const preview = await api.registries.syncPreview(id);
      
      // 分析冲突情况
      if (preview.conflictSkills.length === 0) {
        // 无冲突，直接执行同步（调用原有 sync API）
        const result = await api.registries.sync(id);
        if (result.error) {
          message.error(`同步失败: ${result.error}`);
        } else {
          message.success(`同步完成：自动更新 ${preview.autoUpdateSkills.length} 个，跳过 ${preview.newSkills.length} 个新 skill`);
          loadRegistries();
        }
      } else {
        // 有冲突，显示弹窗
        setSyncPreview(preview);
        setSyncingRegistryId(id);
        setSyncingRegistryName(preview.registryName);
        setConflictChoices({});
        setConflictModalVisible(true);
      }
    } catch (error: any) {
      message.error(error.response?.data?.error || '同步预览失败');
    } finally {
      setSyncingId(null);
    }
  };
```

- [ ] **Step 4: 添加确认冲突处理函数**

添加新的处理函数：

```typescript
  // 确认冲突选择
  const handleConfirmConflict = async () => {
    if (!syncPreview || !syncingRegistryId) return;
    
    // 检查是否所有冲突项都已选择
    const unselected = syncPreview.conflictSkills.filter(s => !conflictChoices[s.name]);
    if (unselected.length > 0) {
      message.error(`以下 Skill 未选择操作：${unselected.map(s => s.name).join(', ')}`);
      return;
    }

    setConflictModalVisible(false);
    setSyncingId(syncingRegistryId);

    try {
      // 构建同步确认请求
      const operations: SyncOperation[] = [];
      for (const skill of syncPreview.conflictSkills) {
        const choice = conflictChoices[skill.name];
        operations.push({
          action: choice,
          skillName: skill.name,
          targetSkillId: choice === 'update' ? skill.localSkill.id : undefined,
          description: skill.description,
        });
      }

      const result = await api.registries.syncConfirm(syncingRegistryId, {
        registryId: syncingRegistryId,
        operations,
      });

      // 显示结果汇总
      let successMsg = `同步完成：自动更新 ${result.autoUpdated} 个`;
      if (result.userUpdated > 0) {
        successMsg += `，更新 ${result.userUpdated} 个`;
      }
      if (result.userSkipped > 0) {
        successMsg += `，跳过 ${result.userSkipped} 个`;
      }
      message.success(successMsg);

      if (result.skipped.length > 0) {
        message.warning(`跳过 ${result.skipped.length} 个失败项：${result.skipped.map(s => s.name).join(', ')}`);
      }

      loadRegistries();
    } catch (error: any) {
      message.error(error.response?.data?.error || '同步确认失败');
    } finally {
      setSyncingId(null);
      setSyncPreview(null);
      setSyncingRegistryId(null);
    }
  };

  // 全部更新
  const handleAllUpdate = () => {
    if (!syncPreview) return;
    const choices: Record<string, 'update'> = {};
    syncPreview.conflictSkills.forEach(s => choices[s.name] = 'update');
    setConflictChoices(choices);
  };

  // 全部跳过
  const handleAllSkip = () => {
    if (!syncPreview) return;
    const choices: Record<string, 'skip'> = {};
    syncPreview.conflictSkills.forEach(s => choices[s.name] = 'skip');
    setConflictChoices(choices);
  };
```

- [ ] **Step 5: 运行前端编译验证**

```bash
cd web && npx tsc --noEmit
```

Expected: 类型检查通过，无错误

- [ ] **Step 6: Commit 冲突检测逻辑**

```bash
git add web/src/pages/RegistryManagement/index.tsx
git commit -m "feat(skill): add conflict detection logic for registry manual sync"
```

---

## Task 8: 添加冲突弹窗组件

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx` (在 Modal 组件之后添加)

- [ ] **Step 1: 添加冲突弹窗 JSX**

在文件末尾的创建/编辑弹窗（约第 391 行的 `</Modal>`）之后添加：

```tsx
      {/* 同步冲突处理弹窗 */}
      <Modal
        title="同步冲突处理"
        open={conflictModalVisible}
        onCancel={() => setConflictModalVisible(false)}
        width={800}
        footer={[
          <Button key="cancel" onClick={() => setConflictModalVisible(false)}>取消</Button>,
          <Button key="all-skip" onClick={handleAllSkip}>
            全部跳过
          </Button>,
          <Button key="all-update" type="primary" onClick={handleAllUpdate}>
            全部更新
          </Button>,
          <Button key="confirm" type="primary" onClick={handleConfirmConflict}>
            确认同步
          </Button>,
        ]}
      >
        <Text type="secondary" style={{ marginBottom: 16, display: 'block' }}>
          以下 Skill 与本地已有同名 Skill 来源不同，请选择处理方式：
        </Text>
        {syncPreview && (
          <>
            {syncPreview.autoUpdateSkills.length > 0 && (
              <div style={{ marginBottom: 12 }}>
                <Tag color="green">{syncPreview.autoUpdateSkills.length} 个同源 Skill 将自动更新</Tag>
              </div>
            )}
            <Table
              dataSource={syncPreview.conflictSkills}
              columns={[
                {
                  title: '名称',
                  dataIndex: 'name',
                  key: 'name',
                  width: 120,
                },
                {
                  title: '本地来源',
                  key: 'localSource',
                  width: 120,
                  render: (_, record) => {
                    const sourceType = record.localSkill.sourceType as SkillSourceType;
                    return (
                      <Tag color={getSourceTypeColor(sourceType)}>
                        {record.localSkill.sourceRegistryName || getSourceTypeLabel(sourceType)}
                      </Tag>
                    );
                  },
                },
                {
                  title: '远程来源',
                  key: 'remoteSource',
                  width: 120,
                  render: () => (
                    <Tag color="cyan">{syncingRegistryName}</Tag>
                  ),
                },
                {
                  title: '本地描述',
                  key: 'localDesc',
                  width: 200,
                  render: (_, record) => (
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {record.localSkill.description?.slice(0, 50) || '暂无'}
                      {record.localSkill.description?.length > 50 ? '...' : ''}
                    </Text>
                  ),
                },
                {
                  title: '远程描述',
                  dataIndex: 'description',
                  key: 'remoteDesc',
                  width: 200,
                  render: (desc: string) => (
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {desc?.slice(0, 50) || '暂无'}
                      {desc?.length > 50 ? '...' : ''}
                    </Text>
                  ),
                },
                {
                  title: '操作',
                  key: 'action',
                  width: 150,
                  render: (_, record) => (
                    <Radio.Group
                      value={conflictChoices[record.name]}
                      onChange={(e) => {
                        setConflictChoices(prev => ({
                          ...prev,
                          [record.name]: e.target.value,
                        }));
                      }}
                    >
                      <Radio value="skip">跳过</Radio>
                      <Radio value="update">更新</Radio>
                    </Radio.Group>
                  ),
                },
              ]}
              rowKey="name"
              pagination={false}
              size="small"
            />
          </>
        )}
      </Modal>
```

- [ ] **Step 2: 确保 Radio 已导入**

检查导入区域是否包含 Radio：

```typescript
import {
  // ... 其他组件
  Radio, // 添加 Radio
} from 'antd';
```

- [ ] **Step 3: 运行前端开发服务器验证**

```bash
cd web && npm run dev
```

Expected: 前端启动成功，访问 RegistryManagement 页面

- [ ] **Step 4: Commit 冲突弹窗组件**

```bash
git add web/src/pages/RegistryManagement/index.tsx
git commit -m "feat(skill): add sync conflict resolution modal with batch action buttons"
```

---

## Task 9: 后端集成测试

**Files:**
- No new files (手动测试)

- [ ] **Step 1: 启动后端服务**

```bash
cd D:\workspace\isdp
go run ./cmd/server
```

Expected: 后端启动成功，监听端口 26305

- [ ] **Step 2: 测试 sync-preview API**

使用 curl 或 Postman 测试：

```bash
curl -X POST http://localhost:26305/api/v1/registries/<existing-registry-id>/sync-preview
```

Expected: 返回预览结果，包含 autoUpdateSkills、conflictSkills、newSkills 等字段

- [ ] **Step 3: 测试 sync-confirm API**

```bash
curl -X POST http://localhost:26305/api/v1/registries/<id>/sync-confirm \
  -H "Content-Type: application/json" \
  -d '{"registryId": "<id>", "operations": [{"action": "update", "skillName": "test", "targetSkillId": "<skill-id>", "description": "test desc"}]}'
```

Expected: 返回确认结果，包含 updated、autoUpdated、userUpdated 等字段

- [ ] **Step 4: 验证路由注册**

```bash
curl http://localhost:26305/api/v1/registries
```

Expected: 返回联邦源列表

---

## Task 10: 前端集成测试

**Files:**
- No new files (手动测试)

- [ ] **Step 1: 启动后端和前端服务**

```bash
# 后端
cd D:\workspace\isdp
go run ./cmd/server

# 前端
cd web && npm run dev
```

- [ ] **Step 2: 测试无冲突同步**

测试场景：
1. 访问联邦技能源页面
2. 点击同步按钮
3. 验证无冲突时直接执行同步
4. 验证结果提示显示正确数量

Expected: 无弹窗，直接显示结果汇总

- [ ] **Step 3: 测试有冲突同步**

测试场景：
1. 访问联邦技能源页面
2. 点击有异源同名 skill 的联邦源同步按钮
3. 验证弹窗显示
4. 验证弹窗内容：名称、本地来源、远程来源、描述对比

Expected: 弹窗显示，内容完整

- [ ] **Step 4: 测试用户选择操作**

测试场景：
1. 在弹窗中选择"更新"或"跳过"
2. 点击"确认同步"
3. 验证结果提示

Expected: 执行用户选择，显示正确结果

- [ ] **Step 5: 测试批量操作**

测试场景：
1. 点击"全部更新"按钮
2. 验证所有项选择"更新"
3. 点击"全部跳过"按钮
4. 验证所有项选择"跳过"

Expected: 批量选择生效

- [ ] **Step 6: 验证验收标准**

| 标准 | 验证方法 |
|------|----------|
| 无冲突直接执行 | 点击无异源同名 skill 的联邦源同步 |
| 有冲突弹窗显示 | 点击有异源同名 skill 的联邦源同步 |
| 弹窗展示完整信息 | 检查表格列：名称、本地来源、远程来源、描述 |
| 弹窗提供"更新"和"跳过"选项 | 检查 Radio 选项 |
| 批量操作按钮 | 检查"全部更新"、"全部跳过"按钮 |
| 选择"更新"后来源变更 | 更新后检查 skill 来源变为 federated |
| 结果汇总显示正确 | 检查成功消息中的数量 |

- [ ] **Step 7: Final Commit**

```bash
git add -A
git commit -m "feat(skill): complete registry sync conflict handling - integration tested"
```

---

## Self-Review Checklist

**1. Spec Coverage:**

| 需求 | 任务 |
|------|------|
| 新增 sync-preview API | Task 2 + Task 4 |
| 新增 sync-confirm API | Task 3 + Task 4 |
| 冲突检测在前端完成 | Task 7 |
| 弹窗复用现有组件（适配 sync 模式） | Task 8（内嵌弹窗，按钮改为"跳过"） |
| 弹窗触发时机：只在有冲突时弹窗 | Task 7（handleSync 中判断 conflictSkills.length） |
| 弹窗展示详细版信息 | Task 8（完整表格） |
| 批量操作按钮 | Task 8（全部更新、全部跳过） |
| 定时同步逻辑不变 | 未修改，保持原有行为 |

**2. Placeholder Scan:**

- 无 TBD/TODO 占位符
- 所有代码步骤包含完整实现
- 无"类似 Task N"引用

**3. Type Consistency:**

| 类型 | 后端 | 前端 |
|------|------|------|
| SyncPreviewResult | `model.SyncPreviewResult` | `SyncPreviewResult` |
| SyncConflictSkill.localSkill | `*LocalSkillInfo` | `LocalSkillInfo` |
| SyncOperation.action | `string` | `'update' \| 'skip'` |
| SyncConfirmRequest.operations | `[]*SyncOperation` | `SyncOperation[]` |
| conflictChoices | - | `Record<string, 'update' \| 'skip'>` |

类型一致，无冲突。

---

**Plan complete and saved to `docs/superpowers/plans/2026-05-07-registry-sync-conflict.md`.**

**Two execution options:**

1. **Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

2. **Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**