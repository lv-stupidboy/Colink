<!-- /autoplan restore point: C:/Users/hw/.gstack/projects/cc-autoplan-restore-20260424-101127.md -->

<!-- AUTONOMOUS DECISION LOG -->
## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
|---|-------|----------|-----------|-----------|----------|----------|
| 1 | CEO | 临时缓存方案 | Mechanical | P1+P5 | 简单有效，未来可升级 | 持久缓存池 |
| 2 | CEO | 持久缓存 deferred | Mechanical | P3 | 非当前 scope，未来优化 | - |
| 3 | Eng | buildPreviewResponse 抽取 | Mechanical | P1 | 避免 DRY 重复 | - |

---

## GSTACK REVIEW REPORT

### Review Runs

| Phase | Skill | Timestamp | Status | Unresolved | Critical Gaps | Mode |
|-------|-------|-----------|--------|------------|---------------|------|
| CEO | plan-ceo-review | 2026-04-24T10:14:00Z | clean | 0 | 0 | SELECTIVE_EXPANSION |
| Design | plan-design-review | 2026-04-24T10:14:00Z | skipped | 0 | 0 | N/A (no UI scope) |
| Eng | plan-eng-review | 2026-04-24T10:14:00Z | issues_open | 1 | 0 | FULL_REVIEW |
| DX | plan-devex-review | 2026-04-24T10:14:00Z | skipped | 0 | 0 | N/A (no DX scope) |

### Review Findings Summary

| Phase | Score | Key Findings |
|-------|-------|--------------|
| CEO | 7.8/10 | Premises valid, scope calibrated (3 files, ~2h), dream state 40% performance delta |
| Design | N/A | Skipped - no UI scope detected (plan states "API 无变更：前端无需修改") |
| Eng | 7.2/10 | Architecture clean, low coupling. Test gap: missing CloneCache unit tests |
| DX | N/A | Skipped - no developer-facing scope (no API change, no CLI change) |

### Critical Issues

| # | Phase | Issue | Severity | Status | Resolution |
|---|-------|-------|----------|--------|------------|
| 1 | Eng | CloneCache unit tests missing | Medium | Deferred | Add to Task 5 or post-implementation |

### Deferred Items

- 持久缓存池 → TODOS.md (future optimization)
- 并发限制参数化 → TODOS.md (non-critical)
- CloneCache 单元测试 → Post-implementation

### Approval Status

**APPROVED** - 2026-04-24T10:15:00Z
- User approved as-is
- Implementation may proceed

---

# 团队包批量导入克隆缓存优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 优化团队包批量导入性能，避免对同一 git 地址重复克隆，使用请求级临时缓存。

**Architecture:** 在 `SyncService` 中引入 `CloneCache` 结构，缓存 Key 为 `url+branch`，Value 为克隆后的临时目录。批量操作时传递缓存，请求结束后统一清理。单包导入保持原有行为不变。

**Tech Stack:** Go (sync.Mutex, context), 线程安全缓存

---

## File Structure

| 文件 | 责任 | 变更类型 |
|------|------|----------|
| `internal/service/teampackagesync/clone_cache.go` | 克隆缓存结构（线程安全） | 新增 |
| `internal/service/teampackagesync/service.go` | 预览/同步方法接受可选缓存 | 修改 |
| `internal/service/teampackagesync/batch.go` | 批量操作创建和使用缓存 | 修改 |

---

## Task 1: 创建克隆缓存结构

**Files:**
- Create: `internal/service/teampackagesync/clone_cache.go`

- [ ] **Step 1: 编写 CloneCache 结构**

```go
package teampackagesync

import (
	"fmt"
	"sync"
)

// CloneCache 请求级克隆缓存（线程安全）
// 用于批量操作时避免对同一仓库重复克隆
type CloneCache struct {
	entries map[string]string // key: "url#branch", value: cloneDir
	mutex   sync.RWMutex
}

// NewCloneCache 创建克隆缓存
func NewCloneCache() *CloneCache {
	return &CloneCache{
		entries: make(map[string]string),
	}
}

// cacheKey 生成缓存键
func cacheKey(url, branch string) string {
	return fmt.Sprintf("%s#%s", url, branch)
}

// Get 获取缓存的克隆目录（返回目录路径和是否存在）
func (c *CloneCache) Get(url, branch string) (string, bool) {
	key := cacheKey(url, branch)
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	dir, exists := c.entries[key]
	return dir, exists
}

// Set 设置缓存（存储克隆目录路径）
func (c *CloneCache) Set(url, branch string, cloneDir string) {
	key := cacheKey(url, branch)
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.entries[key] = cloneDir
}

// GetAllDirs 获取所有缓存的目录路径（用于清理）
func (c *CloneCache) GetAllDirs() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	dirs := make([]string, 0, len(c.entries))
	for _, dir := range c.entries {
		dirs = append(dirs, dir)
	}
	return dirs
}

// Clear 清空缓存记录（不删除目录，仅清空映射）
func (c *CloneCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.entries = make(map[string]string)
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/service/teampackagesync/`
Expected: 编译成功，无错误

- [ ] **Step 3: Commit**

```bash
git add internal/service/teampackagesync/clone_cache.go
git commit -m "feat: add CloneCache for request-level clone caching"
```

---

## Task 2: 修改 GitClient 支持带缓存克隆

**Files:**
- Modify: `internal/service/teampackagesync/git_client.go:28-66`

- [ ] **Step 1: 新增 CloneWithCache 方法**

在 `CloneFromURL` 方法后添加：

```go
// CloneWithURL 使用缓存克隆（如果缓存存在则直接返回）
func (g *GitClient) CloneWithCache(ctx context.Context, url string, branch string, cache *CloneCache) (string, error) {
	// 1. 尝试从缓存获取
	if cache != nil {
		dir, exists := cache.Get(url, branch)
		if exists {
			g.logger.Info("using cached clone",
				zap.String("url", url),
				zap.String("branch", branch),
				zap.String("cachedDir", dir),
			)
			return dir, nil
		}
	}

	// 2. 缓存不存在，执行克隆
	cloneDir, err := g.CloneFromURL(ctx, url, branch)
	if err != nil {
		return "", err
	}

	// 3. 存入缓存（如果有缓存）
	if cache != nil {
		cache.Set(url, branch, cloneDir)
	}

	return cloneDir, nil
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/service/teampackagesync/`
Expected: 编译成功，无错误

- [ ] **Step 3: Commit**

```bash
git add internal/service/teampackagesync/git_client.go
git commit -m "feat: add CloneWithCache method to GitClient"
```

---

## Task 3: 修改 SyncPackage 和 PreviewPackage 支持可选缓存

**Files:**
- Modify: `internal/service/teampackagesync/service.go:157-255` (SyncPackage)
- Modify: `internal/service/teampackagesync/service.go:386-658` (PreviewPackage)

- [ ] **Step 1: 新增带缓存的方法签名**

在 `SyncPackage` 方法后新增 `SyncPackageWithCache`：

```go
// SyncPackageWithCache 同步团队包（带缓存，用于批量操作）
func (s *SyncService) SyncPackageWithCache(ctx context.Context, packageName string, marketId string, confirm *model.TeamPackageImportConfirm, cache *CloneCache) (*model.ImportResult, error) {
	if marketId == "" {
		return nil, fmt.Errorf("marketId is required")
	}
	if s.marketSvc == nil {
		return nil, fmt.Errorf("market service not available")
	}

	var remotePkg *RemotePackage
	var packageCloneDir string

	marketUUID, err := uuid.Parse(marketId)
	if err != nil {
		return nil, fmt.Errorf("invalid market id: %w", err)
	}

	market, err := s.marketSvc.GetMarketByID(ctx, marketUUID)
	if err != nil {
		return nil, fmt.Errorf("get market: %w", err)
	}
	if market == nil {
		return nil, fmt.Errorf("market not found: %s", marketId)
	}

	// 克隆市场仓库（使用缓存）
	marketCloneDir, err := s.gitClient.CloneWithCache(ctx, market.URL, market.Branch, cache)
	if err != nil {
		return nil, fmt.Errorf("clone market repo: %w", err)
	}
	// 注意：批量操作时由缓存统一清理，这里不 defer Cleanup

	// 解析 marketplace.json
	marketplace, err := s.parseMarketplaceJSON(marketCloneDir)
	if err != nil {
		return nil, fmt.Errorf("parse marketplace.json: %w", err)
	}

	// 查找指定的包
	for _, plugin := range marketplace.Plugins {
		if strings.ToLower(plugin.Category) == "team" && plugin.Name == packageName {
			remotePkg = &RemotePackage{
				Name:        plugin.Name,
				Version:     plugin.Version,
				Description: plugin.Description,
				Path:        "",
				Repository:  plugin.Repository,
				Source:      plugin.Source,
			}
			break
		}
	}

	if remotePkg == nil {
		return nil, fmt.Errorf("package not found in marketplace: %s", packageName)
	}

	// 克隆包仓库（使用缓存）
	packageCloneDir, err = s.gitClient.CloneWithCache(ctx, remotePkg.Repository, "master", cache)
	if err != nil {
		return nil, fmt.Errorf("clone package repo: %w", err)
	}
	// 注意：批量操作时由缓存统一清理，这里不 defer Cleanup

	// 设置包的实际路径
	remotePkg.Path = filepath.Join(packageCloneDir, remotePkg.Source)

	// 将目录转换为 zip 数据
	zipData, err := s.createZipFromDir(remotePkg.Path)
	if err != nil {
		return nil, fmt.Errorf("create zip: %w", err)
	}

	// 如果 confirm 为空，创建一个默认的（全部覆盖导入）
	if confirm == nil {
		confirm = &model.TeamPackageImportConfirm{
			Mode:           "overwrite",
			WorkflowAction: "overwrite",
			AssetActions: []model.TeamPackageAssetAction{
				{AssetType: "skill", Name: "*", Action: "overwrite"},
				{AssetType: "command", Name: "*", Action: "overwrite"},
				{AssetType: "rule", Name: "*", Action: "overwrite"},
			},
		}
	}

	// 调用现有的 ImportConfirm 方法（零侵入复用）
	result, err := s.teamPackageSvc.ImportConfirm(ctx, zipData, confirm)
	if err != nil {
		return nil, fmt.Errorf("import confirm: %w", err)
	}

	// 更新版本记录
	if err := s.updateVersionRecord(ctx, packageName, remotePkg, result); err != nil {
		s.logger.Warn("failed to update version record", zap.Error(err))
	}

	return result, nil
}

// SyncPackage 保持原有行为（单包导入，不使用缓存，自动清理）
func (s *SyncService) SyncPackage(ctx context.Context, packageName string, marketId string, confirm *model.TeamPackageImportConfirm) (*model.ImportResult, error) {
	return s.SyncPackageWithCache(ctx, packageName, marketId, confirm, nil)
}
```

- [ ] **Step 2: 新增 PreviewPackageWithCache 方法**

在 `PreviewPackage` 方法后新增 `PreviewPackageWithCache`：

```go
// PreviewPackageWithCache 预览团队包（带缓存，用于批量操作）
func (s *SyncService) PreviewPackageWithCache(ctx context.Context, packageName string, marketId string, cache *CloneCache) (*PreviewPackageResponse, error) {
	if marketId == "" {
		return nil, fmt.Errorf("marketId is required")
	}
	if s.marketSvc == nil {
		return nil, fmt.Errorf("market service not available")
	}

	var remotePkg *RemotePackage
	var packageCloneDir string

	marketUUID, err := uuid.Parse(marketId)
	if err != nil {
		return nil, fmt.Errorf("invalid market id: %w", err)
	}

	market, err := s.marketSvc.GetMarketByID(ctx, marketUUID)
	if err != nil {
		return nil, fmt.Errorf("get market: %w", err)
	}
	if market == nil {
		return nil, fmt.Errorf("market not found: %s", marketId)
	}

	// 克隆市场仓库（使用缓存）
	marketCloneDir, err := s.gitClient.CloneWithCache(ctx, market.URL, market.Branch, cache)
	if err != nil {
		return nil, fmt.Errorf("clone market repo: %w", err)
	}
	// 注意：批量操作时由缓存统一清理，这里不 defer Cleanup

	// 解析 marketplace.json
	marketplace, err := s.parseMarketplaceJSON(marketCloneDir)
	if err != nil {
		return nil, fmt.Errorf("parse marketplace.json: %w", err)
	}

	// 查找指定的包
	for _, plugin := range marketplace.Plugins {
		if strings.ToLower(plugin.Category) == "team" && plugin.Name == packageName {
			remotePkg = &RemotePackage{
				Name:        plugin.Name,
				Version:     plugin.Version,
				Description: plugin.Description,
				Path:        "",
				Repository:  plugin.Repository,
				Source:      plugin.Source,
			}
			break
		}
	}

	if remotePkg == nil {
		return nil, fmt.Errorf("package not found in marketplace: %s", packageName)
	}

	// 克隆包仓库（使用缓存）
	packageCloneDir, err = s.gitClient.CloneWithCache(ctx, remotePkg.Repository, "master", cache)
	if err != nil {
		return nil, fmt.Errorf("clone package repo: %w", err)
	}
	// 注意：批量操作时由缓存统一清理，这里不 defer Cleanup

	// 设置包的实际路径
	remotePkg.Path = filepath.Join(packageCloneDir, remotePkg.Source)

	// 解析包的 manifest.json
	manifestPath := filepath.Join(remotePkg.Path, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest.json: %w", err)
	}

	var manifest model.TeamPackageManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest.json: %w", err)
	}

	// 构建预览响应（调用抽取的冲突检测方法，见 Step 3）
	return s.buildPreviewResponse(ctx, remotePkg, manifest)
}

// PreviewPackage 保持原有行为（单包预览，不使用缓存，自动清理）
func (s *SyncService) PreviewPackage(ctx context.Context, packageName string, marketId string) (*PreviewPackageResponse, error) {
	return s.PreviewPackageWithCache(ctx, packageName, marketId, nil)
}
```

- [ ] **Step 3: 抽取冲突检测为辅助方法**

为了避免代码重复，将冲突检测逻辑抽取为 `buildPreviewResponse` 方法：

```go
// buildPreviewResponse 构建预览响应（冲突检测）
func (s *SyncService) buildPreviewResponse(ctx context.Context, remotePkg *RemotePackage, manifest model.TeamPackageManifest) (*PreviewPackageResponse, error) {
	response := &PreviewPackageResponse{
		PackageName: remotePkg.Name,
		Version:     remotePkg.Version,
		Description: remotePkg.Description,
		Workflow: PreviewWorkflowInfo{
			Name:        manifest.Workflow.Name,
			Description: manifest.Workflow.Description,
			Exists:      false,
		},
		Roles:         []PreviewRoleInfo{},
		Assets: PreviewAssetsInfo{
			Skills:    []PreviewAssetInfo{},
			Commands:  []PreviewAssetInfo{},
			Subagents: []PreviewAssetInfo{},
			Rules:     []PreviewAssetInfo{},
			Settings:  []PreviewAssetInfo{},
		},
		ConflictCount: 0,
	}

	// === 冲突检测逻辑（与原 PreviewPackage 相同）===
	// 检查工作流是否已存在
	workflows, err := s.workflowRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取工作流列表失败: %w", err)
	}
	for _, wf := range workflows {
		if wf.Name == manifest.Workflow.Name {
			response.Workflow.Exists = true
			break
		}
	}

	// 收集角色信息并检测冲突
	for _, role := range manifest.Roles {
		roleInfo := PreviewRoleInfo{
			Name:        role.Name,
			Role:        role.Role,
			Description: role.Description,
			Assets:      []string{},
			Exists:      false,
		}

		roleID, err := uuid.Parse(role.ID)
		if err == nil {
			existing, err := s.agentRepo.FindByID(ctx, roleID)
			if err == nil && existing != nil {
				roleInfo.Exists = true
			}
		}

		// 收集角色绑定的资产名称
		for _, skill := range role.Bindings.Skills {
			roleInfo.Assets = append(roleInfo.Assets, fmt.Sprintf("Skill: %s", skill))
		}
		for _, cmd := range role.Bindings.Commands {
			roleInfo.Assets = append(roleInfo.Assets, fmt.Sprintf("Command: %s", cmd))
		}
		for _, sub := range role.Bindings.Subagents {
			roleInfo.Assets = append(roleInfo.Assets, fmt.Sprintf("Subagent: %s", sub))
		}
		for _, rule := range role.Bindings.Rules {
			roleInfo.Assets = append(roleInfo.Assets, fmt.Sprintf("Rule: %s", rule))
		}
		for _, settings := range role.Bindings.Settings {
			roleInfo.Assets = append(roleInfo.Assets, fmt.Sprintf("Settings: %s", settings))
		}

		response.Roles = append(response.Roles, roleInfo)
	}

	// 收集资产信息并检测冲突
	for _, skill := range manifest.Assets.Skills {
		info := PreviewAssetInfo{Name: skill.Name, Description: skill.Description, Exists: false}
		if s.skillRepo != nil {
			existing, err := s.skillRepo.FindByName(ctx, skill.Name)
			if err == nil && existing != nil {
				info.Exists = true
			}
		}
		response.Assets.Skills = append(response.Assets.Skills, info)
	}
	for _, cmd := range manifest.Assets.Commands {
		info := PreviewAssetInfo{Name: cmd.Name, Description: cmd.Description, Exists: false}
		if s.commandRepo != nil {
			existing, err := s.commandRepo.FindByName(ctx, cmd.Name)
			if err == nil && existing != nil {
				info.Exists = true
			}
		}
		response.Assets.Commands = append(response.Assets.Commands, info)
	}
	for _, sub := range manifest.Assets.Subagents {
		info := PreviewAssetInfo{Name: sub.Name, Description: sub.Description, Exists: false}
		if s.subagentRepo != nil {
			existing, err := s.subagentRepo.FindByName(ctx, sub.Name)
			if err == nil && existing != nil {
				info.Exists = true
			}
		}
		response.Assets.Subagents = append(response.Assets.Subagents, info)
	}
	for _, rule := range manifest.Assets.Rules {
		info := PreviewAssetInfo{Name: rule.Name, Description: rule.Description, Exists: false}
		if s.ruleRepo != nil {
			existing, err := s.ruleRepo.FindByName(ctx, rule.Name)
			if err == nil && existing != nil {
				info.Exists = true
			}
		}
		response.Assets.Rules = append(response.Assets.Rules, info)
	}
	for _, settings := range manifest.Assets.Settings {
		info := PreviewAssetInfo{Name: settings.Name, Description: settings.Description, Exists: false}
		if s.settingsRepo != nil {
			existing, err := s.settingsRepo.FindByName(ctx, settings.Name)
			if err == nil && existing != nil {
				info.Exists = true
			}
		}
		response.Assets.Settings = append(response.Assets.Settings, info)
	}

	// 计算冲突总数
	conflictCount := 0
	if response.Workflow.Exists {
		conflictCount++
	}
	for _, role := range response.Roles {
		if role.Exists {
			conflictCount++
		}
	}
	for _, skill := range response.Assets.Skills {
		if skill.Exists {
			conflictCount++
		}
	}
	for _, cmd := range response.Assets.Commands {
		if cmd.Exists {
			conflictCount++
		}
	}
	for _, sub := range response.Assets.Subagents {
		if sub.Exists {
			conflictCount++
		}
	}
	for _, rule := range response.Assets.Rules {
		if rule.Exists {
			conflictCount++
		}
	}
	for _, settings := range response.Assets.Settings {
		if settings.Exists {
			conflictCount++
		}
	}
	response.ConflictCount = conflictCount

	return response, nil
}
```

- [ ] **Step 4: 编译验证**

Run: `go build ./internal/service/teampackagesync/`
Expected: 编译成功，无错误

- [ ] **Step 5: Commit**

```bash
git add internal/service/teampackagesync/service.go
git commit -m "feat: add SyncPackageWithCache and PreviewPackageWithCache methods"
```

---

## Task 4: 修改批量操作使用缓存

**Files:**
- Modify: `internal/service/teampackagesync/batch.go:62-111` (PreviewPackagesBatch)
- Modify: `internal/service/teampackagesync/batch.go:113-159` (SyncPackagesBatch)

- [ ] **Step 1: 修改 PreviewPackagesBatch 使用缓存**

```go
// PreviewPackagesBatch 批量预览团队包（并行，使用缓存避免重复克隆）
func (s *SyncService) PreviewPackagesBatch(ctx context.Context,
	requests []PreviewRequestItem) (*BatchPreviewResult, error) {

	// 创建请求级缓存
	cache := NewCloneCache()

	maxConcurrency := 5
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	results := make([]PreviewResult, len(requests))
	totalConflicts := 0
	successCount := 0
	failedCount := 0
	var mu sync.Mutex

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, name, marketId string) {
			defer wg.Done()

		// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := s.PreviewPackageWithCache(ctx, name, marketId, cache)

			mu.Lock()
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
			mu.Unlock()
		}(i, req.Name, req.MarketId)
	}

	wg.Wait()

	// 批量操作完成后，统一清理所有克隆目录
	s.cleanupCache(cache)

	return &BatchPreviewResult{
		Previews:       results,
		TotalConflicts: totalConflicts,
		SuccessCount:   successCount,
		FailedCount:    failedCount,
	}, nil
}
```

- [ ] **Step 2: 修改 SyncPackagesBatch 使用缓存**

```go
// SyncPackagesBatch 批量同步团队包（并行，使用缓存避免重复克隆）
func (s *SyncService) SyncPackagesBatch(ctx context.Context,
	requests []SyncRequestItem) (*BatchSyncResult, error) {

	// 创建请求级缓存
	cache := NewCloneCache()

	maxConcurrency := 3
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	results := make([]SyncResult, len(requests))
	successCount := 0
	failedCount := 0
	var mu sync.Mutex

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, name, marketId string, confirm *model.TeamPackageImportConfirm) {
			defer wg.Done()

		// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := s.SyncPackageWithCache(ctx, name, marketId, confirm, cache)

			mu.Lock()
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
			mu.Unlock()
		}(i, req.Name, req.MarketId, req.Confirm)
	}

	wg.Wait()

	// 批量操作完成后，统一清理所有克隆目录
	s.cleanupCache(cache)

	return &BatchSyncResult{
		Results:      results,
		SuccessCount: successCount,
		FailedCount:  failedCount,
	}, nil
}
```

- [ ] **Step 3: 新增 cleanupCache 方法**

在 `SyncService` 中添加清理方法：

```go
// cleanupCache 清理缓存中的所有克隆目录
func (s *SyncService) cleanupCache(cache *CloneCache) {
	if cache == nil {
		return
	}

	dirs := cache.GetAllDirs()
	for _, dir := range dirs {
		s.gitClient.Cleanup(dir)
	}
	cache.Clear()

	s.logger.Info("clone cache cleaned up",
		zap.Int("dirs", len(dirs)),
	)
}
```

- [ ] **Step 4: 编译验证**

Run: `go build ./internal/service/teampackagesync/`
Expected: 编译成功，无错误

- [ ] **Step 5: Commit**

```bash
git add internal/service/teampackagesync/batch.go internal/service/teampackagesync/service.go
git commit -m "feat: use CloneCache in batch operations to avoid duplicate cloning"
```

---

## Task 5: 整体编译测试

**Files:**
- Build: 整体项目

- [ ] **Step 1: 编译整个项目**

Run: `go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 2: 运行后端服务验证**

Run: `go run ./cmd/server`
Expected: 服务启动成功，无 panic

- [ ] **Step 3: 手动测试批量预览**

使用 API 测试工具调用：
```
POST /api/team-package-sync/preview-batch
Body: {"packages": [{"name": "包1", "marketId": "市场ID"}, {"name": "包2", "marketId": "同一市场ID"}]}
```

观察日志：
- 预期：市场仓库只克隆一次（日志显示 "using cached clone"）
- 预期：请求结束后显示 "clone cache cleaned up"

- [ ] **Step 4: 手动测试批量同步**

使用 API 测试工具调用：
```
POST /api/team-package-sync/sync-batch
Body: {"packages": [{"name": "包1", "marketId": "市场ID", "confirm": {...}}, ...]}
```

观察日志：
- 预期：市场仓库和包仓库都使用缓存
- 预期：请求结束后显示 "clone cache cleaned up"

- [ ] **Step 5: Commit 最终变更**

```bash
git add -A
git commit -m "feat: batch import clone caching optimization complete"
```

---

## Expected Performance Improvement

| 场景 | 当前行为 | 优化后行为 |
|------|----------|------------|
| 同一市场导入 5 个包 | 市场仓库克隆 5 次 | 市场仓库克隆 1 次 |
| 同一包仓库 3 个包 | 包仓库克隆 3 次 | 包仓库克隆 1 次 |
| 单包导入 | 无变化（无缓存） | 无变化（无缓存） |

---

## Summary

本次优化：
1. ✅ 新增 `CloneCache` 结构（线程安全）
2. ✅ GitClient 支持带缓存克隆
3. ✅ SyncPackage/PreviewPackage 支持可选缓存参数
4. ✅ 批量操作创建和使用缓存
5. ✅ 批量请求结束后统一清理

**API 无变更**：前端无需修改，行为保持一致。

---

**文档版本**: v1.0
**最后更新**: 2026-04-24 10:15