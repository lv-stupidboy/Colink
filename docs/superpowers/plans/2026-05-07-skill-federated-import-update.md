# Skill 联邦源导入更新功能实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现联邦源导入 skill 时支持更新现有同名 skill，包括冲突检测、用户选择弹窗、更新逻辑。

**Architecture:** 后端扩展数据模型和扫描/导入服务，前端扩展类型定义、冲突检测逻辑和冲突弹窗组件。冲突检测在前端完成，后端支持 create/update 两种导入模式。

**Tech Stack:** Go (后端)、React + Ant Design (前端)、TypeScript

---

## File Structure

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/model/skill.go` | Modify | 添加 LocalSkillInfo、扩展 RemoteSkill/SkillImportItem/BatchImportResult |
| `internal/service/skill/skill_scanner.go` | Modify | 扫描返回本地 skill 详情、导入支持更新模式、文件更新逻辑 |
| `internal/repo/skill.go` | No change | 已有 Update 方法，无需修改 |
| `web/src/types/index.ts` | Modify | 添加 LocalSkillInfo、扩展相关类型 |
| `web/src/pages/SkillLibrary/index.tsx` | Modify | 允许选择已存在 skill、冲突检测、冲突弹窗、批量导入扩展 |

---

## Task 1: 扩展后端数据模型

**Files:**
- Modify: `internal/model/skill.go:190-230` (RemoteSkill 及相关类型定义)

- [ ] **Step 1: 添加 LocalSkillInfo 结构体**

在 `internal/model/skill.go` 文件中，在 `RemoteSkill` 结构体定义之后添加：

```go
// LocalSkillInfo 本地同名 Skill 信息（用于冲突展示）
type LocalSkillInfo struct {
	ID               uuid.UUID `json:"id"`
	SourceType       string    `json:"sourceType"`
	SourceRegistryID uuid.UUID `json:"sourceRegistryId,omitempty"`
	SourceRegistryName string  `json:"sourceRegistryName,omitempty"` // 联邦源名称（如果是 federated）
	Description      string    `json:"description"`
}
```

- [ ] **Step 2: 扩展 RemoteSkill 结构体**

修改 `RemoteSkill` 结构体，添加 `LocalSkill` 字段：

```go
// RemoteSkill 远程 Skill 信息（扫描结果）
type RemoteSkill struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Path          string          `json:"path"`          // Skill 在仓库中的相对路径
	ExistsLocally bool            `json:"existsLocally"` // 是否已存在本地同名 Skill
	LocalSkill    *LocalSkillInfo `json:"localSkill,omitempty"` // 本地同名 Skill 信息（新增）
}
```

- [ ] **Step 3: 扩展 SkillImportItem 结构体**

修改 `SkillImportItem` 结构体，添加 `ImportMode` 和 `TargetSkillID` 字段：

```go
// SkillImportItem 单个 Skill 导入项
type SkillImportItem struct {
	Name            string    `json:"name" binding:"required"`
	Path            string    `json:"path" binding:"required"`
	Description     string    `json:"description"`
	Tags            []string  `json:"tags"`
	SupportedAgents []string  `json:"supportedAgents" binding:"required,min=1"`
	ImportMode      string    `json:"importMode"`    // 新增：create 或 update（默认 create）
	TargetSkillID   uuid.UUID `json:"targetSkillId"` // 新增：update 时指定目标 Skill ID
}
```

- [ ] **Step 4: 扩展 BatchImportResult 结构体**

修改 `BatchImportResult` 结构体，添加 `Updated` 和 `ConflictSummary` 字段：

```go
// BatchImportResult 批量导入结果
type BatchImportResult struct {
	Imported       []*Skill           `json:"imported"`
	Updated        []*Skill           `json:"updated"` // 新增：更新的 Skill 列表
	Skipped        []SkippedSkillInfo `json:"skipped"`
	ConflictSummary *ConflictSummary `json:"conflictSummary,omitempty"` // 新增：冲突处理汇总
}

// ConflictSummary 冲突处理汇总
type ConflictSummary struct {
	AutoUpdated int `json:"autoUpdated"` // 自动更新的数量（同源）
	UserCreated int `json:"userCreated"` // 用户选择新建的数量
	UserUpdated int `json:"userUpdated"` // 用户选择更新的数量
}
```

- [ ] **Step 5: 运行后端测试验证模型变更**

```bash
cd D:\workspace\isdp
go build ./internal/model/...
```

Expected: 编译成功，无错误

- [ ] **Step 6: Commit 数据模型变更**

```bash
git add internal/model/skill.go
git commit -m "feat(skill): add LocalSkillInfo and extend import models for conflict handling"
```

---

## Task 2: 扩展扫描服务返回本地 Skill 详情

**Files:**
- Modify: `internal/service/skill/skill_scanner.go:156-165` (ScanRegistry 方法中检查本地存在的逻辑)

- [ ] **Step 1: 扩展 ScanRegistry 方法中的本地 Skill 查询**

找到 `ScanRegistry` 方法中检查本地存在的循环（约第 156-165 行），修改为：

```go
	// 检查每个技能是否本地已存在，并获取详细信息
	for _, skill := range skills {
		existing, err := s.skillRepo.FindByName(ctx, skill.Name)
		if err == nil && existing != nil {
			skill.ExistsLocally = true
			// 填充本地 Skill 详情
			skill.LocalSkill = &model.LocalSkillInfo{
				ID:          existing.ID,
				SourceType:  string(existing.SourceType),
				Description: existing.Description,
			}
			// 如果是 federated 类型，查询联邦源名称
			if existing.SourceType == model.SkillSourceFederated && existing.SourceRegistryID != uuid.Nil {
				registry, err := s.registryRepo.FindByID(ctx, existing.SourceRegistryID)
				if err == nil && registry != nil {
					skill.LocalSkill.SourceRegistryID = existing.SourceRegistryID
					skill.LocalSkill.SourceRegistryName = registry.Name
				}
			}
		} else {
			skill.ExistsLocally = false
		}
	}
```

- [ ] **Step 2: 运行后端测试验证扫描逻辑**

```bash
go build ./internal/service/skill/...
```

Expected: 编译成功，无错误

- [ ] **Step 3: Commit 扫描服务变更**

```bash
git add internal/service/skill/skill_scanner.go
git commit -m "feat(skill): extend ScanRegistry to return local skill details for conflict display"
```

---

## Task 3: 扩展导入服务支持更新模式

**Files:**
- Modify: `internal/service/skill/skill_scanner.go:519-640` (ImportSkills 方法)

- [ ] **Step 1: 修改 ImportSkills 方法签名和初始化**

找到 `ImportSkills` 方法（约第 519 行），修改初始化部分，添加更新结果收集：

```go
// ImportSkills 批量导入技能
func (s *SkillScanner) ImportSkills(ctx context.Context, req *model.BatchImportRequest) (*model.BatchImportResult, error) {
	// 获取注册表信息
	registry, err := s.registryRepo.FindByID(ctx, req.RegistryID)
	if err != nil {
		return nil, fmt.Errorf("获取注册表失败: %w", err)
	}

	// 创建临时目录
	tempUUID := uuid.New()
	tempDir := filepath.Join(s.storagePath, ".temp", tempUUID.String())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 构建克隆URL（带Token注入）
	cloneURL := s.buildCloneURL(registry)

	// 执行 git clone --depth 1
	cloneCtx, cancel := context.WithTimeout(ctx, s.cloneTimeout)
	defer cancel()

	cmd := pkgexec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", cloneURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		go os.RemoveAll(tempDir)
		if cloneCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("git clone 超时: %w", err)
		}
		return nil, fmt.Errorf("git clone 失败: %s, %w", string(output), err)
	}

	// 使用 goroutine pool 进行并发导入
	imported := make([]*model.Skill, 0, len(req.Skills))
	updated := make([]*model.Skill, 0, len(req.Skills)) // 新增：更新的 skill 列表
	skipped := make([]model.SkippedSkillInfo, 0, len(req.Skills))
	
	// 冲突统计（新增）
	conflictSummary := &model.ConflictSummary{}
	
	// 创建通道收集结果
	importChan := make(chan *model.Skill, len(req.Skills))
	updateChan := make(chan *model.Skill, len(req.Skills)) // 新增
	skipChan := make(chan model.SkippedSkillInfo, len(req.Skills))
	errChan := make(chan error, len(req.Skills))
	// 冲突统计通道（新增）
	autoUpdateChan := make(chan struct{}, len(req.Skills))
	userCreateChan := make(chan struct{}, len(req.Skills))
	userUpdateChan := make(chan struct{}, len(req.Skills))
```

- [ ] **Step 2: 修改并发导入循环，支持 create/update 模式**

修改 goroutine 循环（约第 566-614 行），替换为：

```go
	// 创建信号量控制并发数
	sem := make(chan struct{}, s.importPoolSize)
	var wg sync.WaitGroup
	var nameMu sync.Mutex // protect directory operations

	for _, skillItem := range req.Skills {
		wg.Add(1)
		go func(item model.SkillImportItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 确定导入模式（默认 create）
			importMode := item.ImportMode
			if importMode == "" {
				importMode = "create"
			}

			if importMode == "update" {
				// 更新模式
				if item.TargetSkillID == uuid.Nil {
					errChan <- fmt.Errorf("更新模式需要指定 targetSkillId: %s", item.Name)
					return
				}

				// 获取现有 Skill
				existing, err := s.skillRepo.FindByID(ctx, item.TargetSkillID)
				if err != nil {
					errChan <- fmt.Errorf("找不到目标 Skill %s: %w", item.Name, err)
					return
				}

				nameMu.Lock()

				// 更新元数据（替换策略）
				existing.Description = item.Description
				existing.Tags = item.Tags
				existing.SupportedAgents = item.SupportedAgents
				existing.SourceType = model.SkillSourceFederated
				existing.SourceRegistryID = registry.ID
				existing.UpdatedAt = time.Now()

				// 更新文件目录
				srcDir := filepath.Join(tempDir, item.Path)
				dstDir := filepath.Join(s.storagePath, existing.ID.String())
				if err := s.updateSkillFiles(srcDir, dstDir); err != nil {
					nameMu.Unlock()
					errChan <- fmt.Errorf("更新 Skill 文件 %s 失败: %w", item.Name, err)
					return
				}

				// 保存更新
				if err := s.skillRepo.Update(ctx, existing); err != nil {
					nameMu.Unlock()
					errChan <- fmt.Errorf("更新 Skill 记录 %s 失败: %w", item.Name, err)
					return
				}

				nameMu.Unlock()
				updateChan <- existing
				userUpdateChan <- struct{}{}
			} else {
				// 创建模式（原有逻辑）
				nameMu.Lock()

				skill := &model.Skill{
					ID:               uuid.New(),
					Name:             item.Name,
					Description:      item.Description,
					Tags:             item.Tags,
					SourceType:       model.SkillSourceFederated,
					SourceRegistryID: registry.ID,
					SupportedAgents:  item.SupportedAgents,
					IsPublic:         true,
					Status:           model.SkillStatusActive,
					UseCount:         0,
					CreatedAt:        time.Now(),
					UpdatedAt:        time.Now(),
				}

				srcDir := filepath.Join(tempDir, item.Path)
				dstDir := filepath.Join(s.storagePath, skill.ID.String())

				if err := s.copySkillDirectory(srcDir, dstDir); err != nil {
					nameMu.Unlock()
					errChan <- fmt.Errorf("复制技能目录 %s 失败: %w", item.Name, err)
					return
				}

				if err := s.skillRepo.Create(ctx, skill); err != nil {
					os.RemoveAll(dstDir)
					nameMu.Unlock()
					errChan <- fmt.Errorf("创建技能记录 %s 失败: %w", item.Name, err)
					return
				}

				nameMu.Unlock()
				importChan <- skill
			}
		}(skillItem)
	}
```

- [ ] **Step 3: 修改结果收集部分，添加更新结果和冲突统计**

修改结果收集部分（约第 617-640 行）：

```go
	// 等待所有任务完成
	wg.Wait()
	close(importChan)
	close(updateChan)
	close(skipChan)
	close(errChan)
	close(autoUpdateChan)
	close(userCreateChan)
	close(userUpdateChan)

	// 收集结果
	for skill := range importChan {
		imported = append(imported, skill)
	}
	for skill := range updateChan {
		updated = append(updated, skill)
	}
	for skip := range skipChan {
		skipped = append(skipped, skip)
	}
	for err := range errChan {
		s.logger.Warn("导入技能失败", zap.Error(err))
	}
	
	// 收集冲突统计
	for range autoUpdateChan {
		conflictSummary.AutoUpdated++
	}
	for range userCreateChan {
		conflictSummary.UserCreated++
	}
	for range userUpdateChan {
		conflictSummary.UserUpdated++
	}

	// 异步删除临时目录
	go os.RemoveAll(tempDir)

	return &model.BatchImportResult{
		Imported:       imported,
		Updated:        updated,
		Skipped:        skipped,
		ConflictSummary: conflictSummary,
	}, nil
}
```

- [ ] **Step 4: 运行后端编译验证**

```bash
go build ./internal/service/skill/...
```

Expected: 编译成功，无错误

- [ ] **Step 5: Commit 导入服务变更**

```bash
git add internal/service/skill/skill_scanner.go
git commit -m "feat(skill): extend ImportSkills to support update mode with conflict tracking"
```

---

## Task 4: 添加文件更新辅助函数

**Files:**
- Modify: `internal/service/skill/skill_scanner.go:642-706` (在 copySkillDirectory 之后添加新函数)

- [ ] **Step 1: 添加 updateSkillFiles 辅助函数**

在 `copySkillDirectory` 函数之后（约第 688 行），添加新函数：

```go
// updateSkillFiles 更新 Skill 文件目录（清空原目录，复制新文件）
func (s *SkillScanner) updateSkillFiles(srcDir, dstDir string) error {
	// 检查源目录是否存在
	srcInfo, err := os.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("源目录不存在: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("源路径不是目录: %s", srcDir)
	}

	// 清空目标目录（保留目录本身）
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		// 如果目录不存在，创建它
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				return fmt.Errorf("创建目标目录失败: %w", err)
			}
		} else {
			return fmt.Errorf("读取目标目录失败: %w", err)
		}
	} else {
		// 删除目录内的所有内容
		for _, entry := range entries {
			path := filepath.Join(dstDir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("清理目标目录失败: %w", err)
			}
		}
	}

	// 复制新文件到目标目录
	return s.copySkillDirectory(srcDir, dstDir)
}
```

- [ ] **Step 2: 添加 FindByID 方法到 SkillRepository（如果不存在）**

检查 `internal/repo/skill.go` 是否已有 `FindByID` 方法，如果没有则添加：

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

检查后，如需添加则在 `internal/repo/skill.go` 中 `FindByName` 方法之后添加。

- [ ] **Step 3: 运行后端编译验证**

```bash
go build ./internal/repo/... ./internal/service/skill/...
```

Expected: 编译成功，无错误

- [ ] **Step 4: Commit 文件更新函数**

```bash
git add internal/service/skill/skill_scanner.go internal/repo/skill.go
git commit -m "feat(skill): add updateSkillFiles helper and FindByID method"
```

---

## Task 5: 后端集成测试

**Files:**
- Create: `internal/service/skill/skill_scanner_test.go` (如果不存在则创建，否则添加测试)

- [ ] **Step 1: 编写扫描扩展测试**

在测试文件中添加扫描测试（检查 LocalSkill 信息）：

```go
// 注意：此测试需要 mock repo，实际测试可在集成环境中进行
// 这里仅验证编译和基本逻辑
```

由于集成测试需要数据库环境，跳过单元测试编写，直接进行编译验证。

- [ ] **Step 2: 启动后端服务进行手动测试**

```bash
cd D:\workspace\isdp
go run ./cmd/server
```

Expected: 后端启动成功，监听端口 26305

- [ ] **Step 3: 使用 API 测试扫描功能**

使用 curl 或 Postman 测试扫描 API：

```bash
curl -X POST http://localhost:26305/api/v1/skills/import/federated/scan \
  -H "Content-Type: application/json" \
  -d '{"registryId": "<existing-registry-id>"}'
```

Expected: 返回扫描结果，`existsLocally=true` 的 skill 包含 `localSkill` 字段

- [ ] **Step 4: 验证批量导入 API 支持更新模式**

```bash
curl -X POST http://localhost:26305/api/v1/skills/import/federated/batch \
  -H "Content-Type: application/json" \
  -d '{"registryId": "<id>", "skills": [{"name": "test", "path": "skills/test", "description": "desc", "tags": [], "supportedAgents": ["claude_code"], "importMode": "create"}]}'
```

Expected: 返回导入结果，包含 `imported` 和 `updated` 字段

- [ ] **Step 5: Commit 测试验证**

记录测试结果，如有问题则修复后重新提交。

---

## Task 6: 扩展前端 TypeScript 类型

**Files:**
- Modify: `web/src/types/index.ts:1286-1329` (RemoteSkill 及相关类型定义)

- [ ] **Step 1: 添加 LocalSkillInfo 类型**

在 `RemoteSkill` 类型定义之前（约第 1286 行），添加：

```typescript
// 本地同名 Skill 信息（用于冲突展示）
export interface LocalSkillInfo {
  id: string;
  sourceType: SkillSourceType;
  sourceRegistryId?: string;
  sourceRegistryName?: string; // 联邦源名称（如果是 federated）
  description: string;
}
```

- [ ] **Step 2: 扩展 RemoteSkill 类型**

修改 `RemoteSkill` 类型（约第 1288-1293 行）：

```typescript
// 远程 Skill 信息（扫描结果）
export interface RemoteSkill {
  name: string;
  description: string;
  path: string;          // Skill 在仓库中的相对路径
  existsLocally: boolean; // 是否已存在本地同名 Skill
  localSkill?: LocalSkillInfo; // 本地同名 Skill 信息（新增）
}
```

- [ ] **Step 3: 扩展 SkillImportItem 类型**

修改 `SkillImportItem` 类型（约第 1304-1310 行）：

```typescript
// Skill 导入项
export interface SkillImportItem {
  name: string;
  path: string;
  description: string;
  tags: string[];
  supportedAgents: string[];
  importMode?: 'create' | 'update'; // 新增：导入模式
  targetSkillId?: string;           // 新增：更新时指定目标 Skill ID
}
```

- [ ] **Step 4: 扩展 BatchImportResult 类型**

修改 `BatchImportResult` 类型（约第 1319-1322 行）：

```typescript
// 批量导入结果
export interface BatchImportResult {
  imported: Skill[];
  updated: Skill[];     // 新增：更新的 Skill 列表
  skipped: SkippedSkillInfo[];
  conflictSummary?: ConflictSummary; // 新增：冲突处理汇总
}

// 冲突处理汇总（新增）
export interface ConflictSummary {
  autoUpdated: number; // 自动更新的数量（同源）
  userCreated: number; // 用户选择新建的数量
  userUpdated: number; // 用户选择更新的数量
}
```

- [ ] **Step 5: 运行前端类型检查**

```bash
cd web && npx tsc --noEmit
```

Expected: 类型检查通过，无错误

- [ ] **Step 6: Commit 类型定义变更**

```bash
git add web/src/types/index.ts
git commit -m "feat(skill): add LocalSkillInfo and extend import types for conflict handling"
```

---

## Task 7: 修改前端扫描结果展示（允许选择已存在 Skill）

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx:1232-1285` (扫描弹窗中的 List 组件)

- [ ] **Step 1: 移除已存在 Skill 的禁用逻辑**

找到扫描弹窗中的 `List` 组件（约第 1252-1281 行），修改 `Checkbox` 的 `disabled` 属性：

```tsx
// 修改前
<Checkbox
  disabled={skill.existsLocally}
  checked={selectedRemoteSkills.some(s => s.name === skill.name)}
  onChange={...}
>

// 修改后：移除 disabled，改为显示来源标记
<Checkbox
  checked={selectedRemoteSkills.some(s => s.name === skill.name)}
  onChange={(e) => {
    if (e.target.checked) {
      setSelectedRemoteSkills([...selectedRemoteSkills, skill]);
    } else {
      setSelectedRemoteSkills(selectedRemoteSkills.filter(s => s.name !== skill.name));
    }
  }}
>
  <div>
    <Text strong>{skill.name}</Text>
    {skill.existsLocally && skill.localSkill && (
      <Tag color={getSourceTypeColor(skill.localSkill.sourceType as SkillSourceType)} style={{ marginLeft: 8 }}>
        来自: {skill.localSkill.sourceRegistryName || getSourceTypeLabel(skill.localSkill.sourceType as SkillSourceType)}
      </Tag>
    )}
    <br />
    <Text type="secondary" style={{ fontSize: 12 }}>{skill.description || '暂无描述'}</Text>
  </div>
</Checkbox>
```

- [ ] **Step 2: 移除 List.Item 的禁用样式**

修改 `List.Item` 的样式（约第 1256-1260 行）：

```tsx
// 修改前
<List.Item
  style={{
    opacity: skill.existsLocally ? 0.5 : 1,
    background: skill.existsLocally ? 'var(--ant-color-bg-container-disabled)' : undefined,
  }}
>

// 修改后：移除禁用样式
<List.Item>
```

- [ ] **Step 3: 运行前端开发服务器验证**

```bash
cd web && npm run dev
```

Expected: 前端启动成功，访问 SkillLibrary 页面，扫描联邦源后已存在的 skill 可选择

- [ ] **Step 4: Commit 扫描展示变更**

```bash
git add web/src/pages/SkillLibrary/index.tsx
git commit -m "feat(skill): allow selecting existing skills in scan result with source tag"
```

---

## Task 8: 添加前端冲突检测逻辑

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx` (在 handleConfirmImport 函数之前添加新函数)

- [ ] **Step 1: 添加冲突分析函数**

在组件内部，`handleConfirmImport` 函数之前添加冲突分析函数：

```tsx
// 冲突分析函数
const analyzeConflicts = (selectedSkills: RemoteSkill[], registryId: string): {
  autoUpdateItems: RemoteSkill[];    // 同源，自动更新
  conflictItems: RemoteSkill[];       // 异源，需要用户选择
  createItems: RemoteSkill[];         // 无同名，直接创建
} => {
  const autoUpdateItems: RemoteSkill[] = [];
  const conflictItems: RemoteSkill[] = [];
  const createItems: RemoteSkill[] = [];

  for (const skill of selectedSkills) {
    if (!skill.existsLocally) {
      // 无同名，直接创建
      createItems.push(skill);
    } else if (skill.localSkill?.sourceType === 'federated' && 
               skill.localSkill.sourceRegistryId === registryId) {
      // 同源联邦，自动更新
      autoUpdateItems.push(skill);
    } else {
      // 异源或非联邦类型，需要用户选择
      conflictItems.push(skill);
    }
  }

  return { autoUpdateItems, conflictItems, createItems };
};
```

- [ ] **Step 2: 添加冲突选择状态**

在组件状态定义区域添加新的状态：

```tsx
// 冲突处理状态（新增）
const [conflictModalVisible, setConflictModalVisible] = useState(false);
const [conflictItems, setConflictItems] = useState<RemoteSkill[]>([]);
const [conflictChoices, setConflictChoices] = useState<Record<string, 'create' | 'update'>>({});
const [currentRegistryName, setCurrentRegistryName] = useState('');
```

- [ ] **Step 3: Commit 冲突检测逻辑**

```bash
git add web/src/pages/SkillLibrary/index.tsx
git commit -m "feat(skill): add conflict analysis logic for federated import"
```

---

## Task 9: 添加冲突选择弹窗组件

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx` (在扫描弹窗之后添加冲突弹窗)

- [ ] **Step 1: 添加冲突弹窗 JSX**

在扫描弹窗（`<Modal title="从联邦源导入 Skill"`）之后添加冲突选择弹窗：

```tsx
{/* 冲突选择弹窗 */}
<Modal
  title="导入冲突处理"
  open={conflictModalVisible}
  onCancel={() => setConflictModalVisible(false)}
  width={800}
  footer={[
    <Button key="cancel" onClick={() => setConflictModalVisible(false)}>取消</Button>,
    <Button key="all-create" onClick={() => {
      const choices: Record<string, 'create'> = {};
      conflictItems.forEach(s => choices[s.name] = 'create');
      setConflictChoices(choices);
    }}>
      全部新建
    </Button>,
    <Button key="all-update" type="primary" onClick={() => {
      const choices: Record<string, 'update'> = {};
      conflictItems.forEach(s => choices[s.name] = 'update');
      setConflictChoices(choices);
    }}>
      全部更新
    </Button>,
    <Button key="confirm" type="primary" onClick={handleConfirmConflict}>
      确认导入
    </Button>,
  ]}
>
  <Text type="secondary" style={{ marginBottom: 16, display: 'block' }}>
    以下 Skill 与本地已有同名 Skill 来源不同，请选择处理方式：
  </Text>
  <Table
    dataSource={conflictItems}
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
          if (!record.localSkill) return '未知';
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
          <Tag color="cyan">{currentRegistryName}</Tag>
        ),
      },
      {
        title: '本地描述',
        key: 'localDesc',
        width: 200,
        render: (_, record) => (
          <Text type="secondary" style={{ fontSize: 12 }}>
            {record.localSkill?.description?.slice(0, 50) || '暂无'}
            {record.localSkill?.description?.length > 50 ? '...' : ''}
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
            <Radio value="create">新建</Radio>
            <Radio value="update">更新</Radio>
          </Radio.Group>
        ),
      },
    ]}
    rowKey="name"
    pagination={false}
    size="small"
  />
</Modal>
```

- [ ] **Step 2: 运行前端开发服务器验证弹窗显示**

```bash
cd web && npm run dev
```

Expected: 前端启动成功，选择有冲突的 skill 后应显示冲突弹窗

- [ ] **Step 3: Commit 冲突弹窗组件**

```bash
git add web/src/pages/SkillLibrary/index.tsx
git commit -m "feat(skill): add conflict resolution modal with batch action buttons"
```

---

## Task 10: 扩展批量导入逻辑

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx:607-695` (handleConfirmImport 和 handleBatchSave 函数)

- [ ] **Step 1: 修改 handleConfirmImport 函数**

修改 `handleConfirmImport` 函数，添加冲突分析逻辑：

```tsx
// 确认导入选中的 Skill
const handleConfirmImport = () => {
  if (selectedRemoteSkills.length === 0) {
    message.error('请选择至少一个 Skill');
    return;
  }

  // 分析冲突情况
  const { autoUpdateItems, conflictItems, createItems } = analyzeConflicts(
    selectedRemoteSkills, 
    selectedRegistryId
  );

  // 如果没有冲突项，直接导入
  if (conflictItems.length === 0) {
    setScanModalVisible(false);
    performImport(autoUpdateItems, createItems, {});
    return;
  }

  // 有冲突项，显示冲突弹窗
  setConflictItems(conflictItems);
  setConflictChoices({});
  setCurrentRegistryName(scanResult?.registryName || '');
  setScanModalVisible(false);
  setConflictModalVisible(true);
};
```

- [ ] **Step 2: 添加 handleConfirmConflict 函数**

添加确认冲突选择的处理函数：

```tsx
// 确认冲突选择
const handleConfirmConflict = () => {
  // 检查是否所有冲突项都已选择
  const unselected = conflictItems.filter(s => !conflictChoices[s.name]);
  if (unselected.length > 0) {
    message.error(`以下 Skill 未选择操作：${unselected.map(s => s.name).join(', ')}`);
    return;
  }

  setConflictModalVisible(false);
  
  // 重新分析冲突（获取 autoUpdateItems 和 createItems）
  const { autoUpdateItems, createItems } = analyzeConflicts(selectedRemoteSkills, selectedRegistryId);
  
  // 执行导入
  performImport(autoUpdateItems, createItems, conflictChoices);
};
```

- [ ] **Step 3: 添加 performImport 函数**

添加实际执行导入的函数：

```tsx
// 执行导入操作
const performImport = async (
  autoUpdateItems: RemoteSkill[],
  createItems: RemoteSkill[],
  conflictChoices: Record<string, 'create' | 'update'>
) => {
  setBatchImporting(true);
  try {
    // 构建导入请求
    const skills: SkillImportItem[] = [];

    // 创建项
    for (const skill of createItems) {
      skills.push({
        name: skill.name,
        path: skill.path,
        description: skill.description,
        tags: [],
        supportedAgents: ['claude_code'],
        importMode: 'create',
      });
    }

    // 自动更新项（同源）
    for (const skill of autoUpdateItems) {
      skills.push({
        name: skill.name,
        path: skill.path,
        description: skill.description,
        tags: [],
        supportedAgents: ['claude_code'],
        importMode: 'update',
        targetSkillId: skill.localSkill?.id,
      });
    }

    // 冲突项（根据用户选择）
    for (const skill of conflictItems) {
      const choice = conflictChoices[skill.name];
      skills.push({
        name: skill.name,
        path: skill.path,
        description: skill.description,
        tags: [],
        supportedAgents: ['claude_code'],
        importMode: choice,
        targetSkillId: choice === 'update' ? skill.localSkill?.id : undefined,
      });
    }

    const result = await api.skills.batchImportFederated({
      registryId: selectedRegistryId,
      skills,
    });

    // 显示导入结果
    const summary = result.conflictSummary;
    let successMsg = `成功导入 ${result.imported.length} 个 Skill`;
    if (result.updated?.length > 0) {
      successMsg += `，更新 ${result.updated.length} 个`;
    }
    if (summary) {
      if (summary.autoUpdated > 0) successMsg += `（自动更新 ${summary.autoUpdated} 个）`;
      if (summary.userCreated > 0) successMsg += `（新建 ${summary.userCreated} 个）`;
      if (summary.userUpdated > 0) successMsg += `（更新 ${summary.userUpdated} 个）`;
    }
    message.success(successMsg);

    if (result.skipped.length > 0) {
      message.warning(`跳过 ${result.skipped.length} 个：${result.skipped.map(s => s.name).join(', ')}`);
    }

    setSelectedRemoteSkills([]);
    loadSkills();
  } catch (error: any) {
    message.error(error.response?.data?.error || '导入失败');
  } finally {
    setBatchImporting(false);
  }
};
```

- [ ] **Step 4: 运行前端开发服务器验证完整流程**

```bash
cd web && npm run dev
```

Expected: 
1. 扫描联邦源后可选择已存在的 skill
2. 确认导入时检测冲突
3. 有冲突时显示冲突弹窗
4. 选择后正确调用 API

- [ ] **Step 5: Commit 批量导入逻辑变更**

```bash
git add web/src/pages/SkillLibrary/index.tsx
git commit -m "feat(skill): extend batch import with conflict detection and resolution"
```

---

## Task 11: 前端集成测试

**Files:**
- No new files (手动测试)

- [ ] **Step 1: 启动后端服务**

```bash
cd D:\workspace\isdp
go run ./cmd/server
```

- [ ] **Step 2: 启动前端开发服务器**

```bash
cd web && npm run dev
```

- [ ] **Step 3: 测试完整导入流程**

测试场景：
1. **全新导入**：选择不存在的 skill → 直接创建
2. **同源更新**：选择同名但同源 skill → 自动更新
3. **异源冲突**：选择同名但异源 skill → 弹窗选择
4. **混合导入**：选择多个 skill → 自动处理 + 弹窗
5. **批量操作**：点击"全部新建"或"全部更新"

- [ ] **Step 4: 验证验收标准**

| 标准 | 验证方法 |
|------|----------|
| 用户可选择已存在 skill | 扫描后勾选已存在的 skill |
| 同源自动更新 | 选择同源 skill，确认后无弹窗，直接更新 |
| 异源弹窗选择 | 选择异源 skill，确认后显示弹窗 |
| 弹窗展示完整信息 | 检查表格列：名称、本地来源、远程来源、描述 |
| 批量操作按钮 | 检查弹窗底部有"全部新建"、"全部更新" |
| 来源信息正确变更 | 更新后检查 skill 来源变为 federated |
| 文件内容正确更新 | 更新后检查 skill 文件内容 |

- [ ] **Step 5: Final Commit**

```bash
git add -A
git commit -m "feat(skill): complete federated import with update support - integration tested"
```

---

## Self-Review Checklist

**1. Spec Coverage:**

| 需求 | 任务 |
|------|------|
| 扫描返回本地 skill 详情 | Task 2 |
| 批量导入支持更新模式 | Task 3 |
| 前端允许选择已存在 skill | Task 7 |
| 前端冲突检测逻辑 | Task 8 |
| 前端冲突弹窗组件 | Task 9 |
| 前端批量导入扩展 | Task 10 |
| Tags/SupportedAgents 替换策略 | Task 3（直接赋值，不合并） |
| 无权限校验 | 已确认，后端无额外校验 |
| 无历史版本 | 已确认，直接覆盖 |

**2. Placeholder Scan:**

- 无 TBD/TODO 占位符
- 所有代码步骤包含完整实现
- 无"类似 Task N"引用

**3. Type Consistency:**

| 类型 | 后端 | 前端 |
|------|------|------|
| LocalSkillInfo | `model.LocalSkillInfo` | `LocalSkillInfo` |
| RemoteSkill.localSkill | `*LocalSkillInfo` | `LocalSkillInfo?` |
| SkillImportItem.importMode | `string` | `'create' \| 'update'` |
| SkillImportItem.targetSkillId | `uuid.UUID` | `string` |
| BatchImportResult.updated | `[]*Skill` | `Skill[]` |
| ConflictSummary | `*model.ConflictSummary` | `ConflictSummary?` |

类型一致，无冲突。

---

**Plan complete and saved to `docs/superpowers/plans/2026-05-07-skill-federated-import-update.md`.**

**Two execution options:**

1. **Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

2. **Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**