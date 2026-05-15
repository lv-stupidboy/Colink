# Skill 同步刷新 Agent Configs 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 联邦源同步/导入更新 skill 后，自动刷新已生成配置的角色关联的 skill 文件

**Architecture:** 在 Skill 更新流程完成后调用刷新方法，查询角色关联 → 过滤已生成配置的角色 → 复制 skill 文件到 agent-configs 目录

**Tech Stack:** Go backend (skill_scanner.go, registry_service.go, skill.go model), 复用 configgen/downloader.go 复制方法

---

## File Structure

**Modify:**
- `internal/model/skill.go` — 添加 `RefreshError` 结构体，扩展返回结果
- `internal/service/skill/skill_scanner.go` — 添加依赖注入，实现刷新方法，在更新流程调用
- `internal/service/skill/registry_service.go` — 添加依赖注入，在 SyncConfirm 调用刷新
- `cmd/server/main.go` — 更新 SkillScanner 和 RegistryService 构造函数参数

---

## Tasks

### Task 1: 扩展数据模型添加 RefreshError

**Files:**
- Modify: `internal/model/skill.go`

- [ ] **Step 1: 添加 RefreshError 结构体**

在 `skill.go` 文件末尾添加：

```go
// RefreshError 刷新配置目录错误
type RefreshError struct {
	AgentRoleID   uuid.UUID `json:"agentRoleId"`
	AgentRoleName string    `json:"agentRoleName"`
	Error         string    `json:"error"`
}
```

- [ ] **Step 2: 扩展 BatchImportResult**

找到 `BatchImportResult` 结构体，添加字段：

```go
ConfigRefreshErrors []RefreshError `json:"configRefreshErrors,omitempty"` // 配置刷新错误列表
```

- [ ] **Step 3: 扩展 SyncConfirmResult**

找到 `SyncConfirmResult` 结构体，添加字段：

```go
ConfigRefreshErrors []RefreshError `json:"configRefreshErrors,omitempty"` // 配置刷新错误列表
```

- [ ] **Step 4: 运行后端编译验证**

Run: `cd D:/workspace/isdp && go build ./cmd/server`
Expected: 编译成功

- [ ] **Step 5: Commit 数据模型变更**

```bash
git add internal/model/skill.go
git commit -m "feat(model): add RefreshError and config refresh errors to import/sync results"
```

---

### Task 2: 扩展 SkillScanner 依赖注入

**Files:**
- Modify: `internal/service/skill/skill_scanner.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: 扩展 SkillScanner 结构体**

找到 `SkillScanner` 结构体定义（约第 30 行），添加新字段：

```go
type SkillScanner struct {
	registryRepo     *repo.SkillRegistryRepository
	skillRepo        *repo.SkillRepository
	bindingRepo      *repo.AgentSkillBindingRepository  // 新增
	agentConfigRepo  *repo.AgentConfigRepository        // 新增
	storagePath      string
	agentConfigPath  string                              // 新增：agent-configs 目录路径
	logger           *zap.Logger
	cloneTimeout     time.Duration
	scanPoolSize     int
	importPoolSize   int
}
```

- [ ] **Step 2: 扩展 NewSkillScanner 函数**

修改 `NewSkillScanner` 函数签名和实现（约第 40 行）：

```go
func NewSkillScanner(
	registryRepo *repo.SkillRegistryRepository,
	skillRepo *repo.SkillRepository,
	bindingRepo *repo.AgentSkillBindingRepository,      // 新增
	agentConfigRepo *repo.AgentConfigRepository,        // 新增
	storagePath string,
	agentConfigPath string,                              // 新增
	logger *zap.Logger,
) *SkillScanner {
	return &SkillScanner{
		registryRepo:     registryRepo,
		skillRepo:        skillRepo,
		bindingRepo:      bindingRepo,
		agentConfigRepo:  agentConfigRepo,
		storagePath:      storagePath,
		agentConfigPath:  agentConfigPath,
		logger:           logger,
		cloneTimeout:     120 * time.Second,
		scanPoolSize:     10,
		importPoolSize:   5,
	}
}
```

- [ ] **Step 3: 更新 main.go 构造函数调用**

找到 `cmd/server/main.go:228`，更新调用：

```go
skillScanner := skill.NewSkillScanner(
	registryRepo, skillRepo,
	agentSkillBindingRepo, agentConfigRepo,  // 新增
	cfg.GetSkillStoragePath(),
	cfg.AgentConfig.DataDir,                  // 新增：agent-configs 目录
	logger,
)
```

- [ ] **Step 4: 运行后端编译验证**

Run: `cd D:/workspace/isdp && go build ./cmd/server`
Expected: 编译成功

- [ ] **Step 5: Commit 依赖注入变更**

```bash
git add internal/service/skill/skill_scanner.go cmd/server/main.go
git commit -m "feat(skill): add agentConfigRepo and bindingRepo dependencies to SkillScanner"
```

---

### Task 3: 实现 refreshAgentConfigsForSkill 方法

**Files:**
- Modify: `internal/service/skill/skill_scanner.go`

- [ ] **Step 1: 添加 refreshAgentConfigsForSkill 方法**

在 `SkillScanner` 结构体方法区域添加新方法（建议在 `updateSkillFiles` 方法之后）：

```go
// refreshAgentConfigsForSkill 刷新角色配置目录中的 skill 文件
// 参数：skillID - 被更新的 skill ID
// 返回：刷新错误列表（空表示全部成功）
func (s *SkillScanner) refreshAgentConfigsForSkill(ctx context.Context, skillID uuid.UUID) []model.RefreshError {
	if s.agentConfigPath == "" || s.bindingRepo == nil {
		return nil // 未配置 agent-configs 目录，跳过刷新
	}

	// 1. 获取 skill 信息
	skill, err := s.skillRepo.FindByID(ctx, skillID)
	if err != nil {
		s.logger.Warn("获取 skill 信息失败，跳过刷新", zap.String("skillId", skillID.String()), zap.Error(err))
		return nil
	}

	// 2. 查询关联角色 ID 列表
	agentRoleIDs, err := s.bindingRepo.FindBySkillID(ctx, skillID)
	if err != nil {
		s.logger.Warn("查询角色关联失败，跳过刷新", zap.String("skillId", skillID.String()), zap.Error(err))
		return nil
	}
	if len(agentRoleIDs) == 0 {
		s.logger.Info("skill 未被任何角色关联，无需刷新", zap.String("skillId", skillID.String()))
		return nil
	}

	// 3. 遍历角色，过滤已生成配置的，执行刷新
	var refreshErrors []model.RefreshError
	srcDir := filepath.Join(s.storagePath, skill.ID.String())

	for _, agentRoleID := range agentRoleIDs {
		// 查询角色配置信息
		agentConfig, err := s.agentConfigRepo.FindByID(ctx, agentRoleID)
		if err != nil {
			s.logger.Warn("获取角色配置失败", zap.String("agentRoleId", agentRoleID.String()), zap.Error(err))
			continue
		}

		// 过滤：只刷新已生成配置的角色
		if agentConfig.ConfigGeneratedAt == nil {
			s.logger.Info("角色未生成配置，跳过刷新",
				zap.String("agentRoleId", agentRoleID.String()),
				zap.String("agentRoleName", agentConfig.Name))
			continue
		}

		// 刷新 skill 文件
		dstDir := filepath.Join(s.agentConfigPath, agentRoleID.String(), "skills", skill.Name)
		if err := s.updateSkillFiles(srcDir, dstDir); err != nil {
			refreshErrors = append(refreshErrors, model.RefreshError{
				AgentRoleID:   agentRoleID,
				AgentRoleName: agentConfig.Name,
				Error:         err.Error(),
			})
			s.logger.Warn("刷新角色配置 skill 文件失败",
				zap.String("agentRoleId", agentRoleID.String()),
				zap.String("skillName", skill.Name),
				zap.Error(err))
		} else {
			s.logger.Info("刷新角色配置 skill 文件成功",
				zap.String("agentRoleId", agentRoleID.String()),
				zap.String("agentRoleName", agentConfig.Name),
				zap.String("skillName", skill.Name))
		}
	}

	return refreshErrors
}
```

- [ ] **Step 2: 运行后端编译验证**

Run: `cd D:/workspace/isdp && go build ./cmd/server`
Expected: 编译成功

- [ ] **Step 3: Commit 刷新方法实现**

```bash
git add internal/service/skill/skill_scanner.go
git commit -m "feat(skill): implement refreshAgentConfigsForSkill method"
```

---

### Task 4: 在 ImportSkills 更新流程调用刷新

**Files:**
- Modify: `internal/service/skill/skill_scanner.go`

- [ ] **Step 1: 在更新模式成功后调用刷新**

找到 ImportSkills 方法中更新模式成功的代码（约第 645-648 行），在 `skillRepo.Update` 成功后添加刷新调用：

```go
// 保存更新
if err := s.skillRepo.Update(ctx, existing); err != nil {
	nameMu.Unlock()
	errChan <- fmt.Errorf("更新 Skill 记录 %s 失败: %w", item.Name, err)
	return
}

// 刷新关联角色的配置目录（新增）
refreshErrors := s.refreshAgentConfigsForSkill(ctx, existing.ID)
if len(refreshErrors) > 0 {
	s.logger.Warn("刷新角色配置失败", zap.String("skillId", existing.ID.String()), zap.Int("errors", len(refreshErrors)))
}

nameMu.Unlock()
updateChan <- existing
userUpdateChan <- struct{}{}
```

- [ ] **Step 2: 收集刷新错误到结果**

找到结果收集区域（约第 717 行），需要在 `BatchImportResult` 构建时收集刷新错误。由于刷新在 goroutine 中执行，需要添加错误通道：

首先在通道定义区域（约第 582 行）添加：

```go
refreshErrChan := make(chan []model.RefreshError, len(req.Skills))
```

然后在 goroutine 中发送刷新错误：

```go
refreshErrors := s.refreshAgentConfigsForSkill(ctx, existing.ID)
if len(refreshErrors) > 0 {
	refreshErrChan <- refreshErrors
}
```

在通道关闭区域（约第 700 行）添加：

```go
close(refreshErrChan)
```

在结果收集区域添加：

```go
var allRefreshErrors []model.RefreshError
for errs := range refreshErrChan {
	allRefreshErrors = append(allRefreshErrors, errs...)
}
```

在返回结果构建处（约第 730 行）添加：

```go
result := &model.BatchImportResult{
	// ... 其他字段
	ConfigRefreshErrors: allRefreshErrors,
}
```

- [ ] **Step 3: 运行后端编译验证**

Run: `cd D:/workspace/isdp && go build ./cmd/server`
Expected: 编译成功

- [ ] **Step 4: Commit ImportSkills 刷新调用**

```bash
git add internal/service/skill/skill_scanner.go
git commit -m "feat(skill): call refreshAgentConfigs in ImportSkills update mode"
```

---

### Task 5: 扩展 RegistryService 依赖注入并调用刷新

**Files:**
- Modify: `internal/service/skill/registry_service.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: 扩展 RegistryService 结构体**

找到 `RegistryService` 结构体（约第 18 行），添加新字段：

```go
type RegistryService struct {
	registryRepo     *repo.SkillRegistryRepository
	skillRepo        *repo.SkillRepository
	skillScanner     *SkillScanner
	bindingRepo      *repo.AgentSkillBindingRepository  // 新增
	agentConfigRepo  *repo.AgentConfigRepository        // 新增
	agentConfigPath  string                              // 新增
}
```

- [ ] **Step 2: 扩展 NewRegistryService 函数**

修改 `NewRegistryService` 函数签名和实现（约第 25 行）：

```go
func NewRegistryService(
	registryRepo *repo.SkillRegistryRepository,
	skillRepo *repo.SkillRepository,
	skillScanner *SkillScanner,
	bindingRepo *repo.AgentSkillBindingRepository,      // 新增
	agentConfigRepo *repo.AgentConfigRepository,        // 新增
	agentConfigPath string,                              // 新增
) *RegistryService {
	return &RegistryService{
		registryRepo:     registryRepo,
		skillRepo:        skillRepo,
		skillScanner:     skillScanner,
		bindingRepo:      bindingRepo,
		agentConfigRepo:  agentConfigRepo,
		agentConfigPath:  agentConfigPath,
	}
}
```

- [ ] **Step 3: 添加 refreshAgentConfigsForSkill 方法（RegistryService 版本）**

在 `SyncConfirm` 方法之后添加：

```go
// refreshAgentConfigsForSkill 刷新角色配置目录中的 skill 文件
func (s *RegistryService) refreshAgentConfigsForSkill(ctx context.Context, skillID uuid.UUID) []model.RefreshError {
	if s.agentConfigPath == "" || s.bindingRepo == nil {
		return nil
	}

	// 获取 skill 信息
	skill, err := s.skillRepo.FindByID(ctx, skillID)
	if err != nil {
		return nil
	}

	// 查询关联角色 ID 列表
	agentRoleIDs, err := s.bindingRepo.FindBySkillID(ctx, skillID)
	if err != nil || len(agentRoleIDs) == 0 {
		return nil
	}

	// 遍历角色，执行刷新
	var refreshErrors []model.RefreshError
	srcDir := filepath.Join(s.skillScanner.storagePath, skill.ID.String())

	for _, agentRoleID := range agentRoleIDs {
		agentConfig, err := s.agentConfigRepo.FindByID(ctx, agentRoleID)
		if err != nil || agentConfig.ConfigGeneratedAt == nil {
			continue
		}

		dstDir := filepath.Join(s.agentConfigPath, agentRoleID.String(), "skills", skill.Name)
		if err := s.skillScanner.updateSkillFiles(srcDir, dstDir); err != nil {
			refreshErrors = append(refreshErrors, model.RefreshError{
				AgentRoleID:   agentRoleID,
				AgentRoleName: agentConfig.Name,
				Error:         err.Error(),
			})
		}
	}

	return refreshErrors
}
```

- [ ] **Step 4: 在 SyncConfirm 更新成功后调用刷新**

找到 `SyncConfirm` 方法中用户选择更新成功的代码（约第 340-346 行），在 `result.Updated` 添加后调用刷新：

```go
result.Updated = append(result.Updated, existing)
result.UserUpdated++

// 刷新关联角色的配置目录（新增）
refreshErrors := s.refreshAgentConfigsForSkill(ctx, existing.ID)
result.ConfigRefreshErrors = append(result.ConfigRefreshErrors, refreshErrors...)
```

同样在自动更新区域（约第 308-311 行）添加刷新调用：

```go
if err := s.skillRepo.Update(ctx, existing); err != nil {
	continue
}
result.AutoUpdated++

// 刷新关联角色的配置目录（新增）
s.refreshAgentConfigsForSkill(ctx, existing.ID)
```

- [ ] **Step 5: 更新 main.go 构造函数调用**

找到 `cmd/server/main.go:229`，更新调用：

```go
registryService := skill.NewRegistryService(
	registryRepo, skillRepo, skillScanner,
	agentSkillBindingRepo, agentConfigRepo,  // 新增
	cfg.AgentConfig.DataDir,                  // 新增
)
```

- [ ] **Step 6: 运行后端编译验证**

Run: `cd D:/workspace/isdp && go build ./cmd/server`
Expected: 编译成功

- [ ] **Step 7: Commit RegistryService 变更**

```bash
git add internal/service/skill/registry_service.go cmd/server/main.go
git commit -m "feat(skill): add config refresh to RegistryService SyncConfirm"
```

---

### Task 6: 集成测试验证

**Files:**
- Manual test

- [ ] **Step 1: 启动后端服务**

Run: `cd D:/workspace/isdp && go run ./cmd/server`
Expected: 服务启动成功

- [ ] **Step 2: 测试 ImportSkills 更新模式刷新**

使用 API 测试或前端操作：
1. 创建一个角色配置并绑定 skill
2. 生成角色配置
3. 从联邦源导入更新该 skill
4. 检查 `agent-configs/{agent-id}/skills/{skill-name}/` 目录文件是否更新

- [ ] **Step 3: 测试 SyncConfirm 更新模式刷新**

使用 API 测试：
1. 调用 `sync-preview` API
2. 调用 `sync-confirm` API 选择更新
3. 检查返回结果中的 `configRefreshErrors` 字段
4. 检查 agent-configs 目录文件是否更新

- [ ] **Step 4: 测试无关联角色场景**

更新一个未被任何角色关联的 skill，确认刷新不执行且无错误

- [ ] **Step 5: 测试角色未生成配置场景**

更新一个角色绑定但未生成配置的 skill，确认刷新不执行

---

### Task 7: 前端展示刷新错误（可选）

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx`
- Modify: `web/src/pages/RegistryManagement/index.tsx`

- [ ] **Step 1: 在 SkillLibrary 导入结果展示刷新错误**

如果 `result.configRefreshErrors` 有内容，显示警告消息：

```tsx
if (result.configRefreshErrors?.length > 0) {
  message.warning(`配置刷新失败 ${result.configRefreshErrors.length} 个：${result.configRefreshErrors.map(e => e.agentRoleName).join(', ')}`);
}
```

- [ ] **Step 2: 在 RegistryManagement 同步结果展示刷新错误**

```tsx
if (result.configRefreshErrors?.length > 0) {
  message.warning(`配置刷新失败 ${result.configRefreshErrors.length} 个`);
}
```

- [ ] **Step 3: 运行前端构建验证**

Run: `cd D:/workspace/isdp/web && npm run build`
Expected: 构建成功

- [ ] **Step 4: Commit 前端变更**

```bash
git add web/src/pages/SkillLibrary/index.tsx web/src/pages/RegistryManagement/index.tsx
git commit -m "feat(frontend): show config refresh errors in import/sync result"
```

---

## Self-Review

**1. Spec coverage:**
- ✅ RefreshError 模型定义（Task 1）
- ✅ 扩展返回结果（Task 1）
- ✅ 实现 refreshAgentConfigsForSkill 方法（Task 3, Task 5）
- ✅ 在 ImportSkills 更新流程调用（Task 4）
- ✅ 在 SyncConfirm 更新流程调用（Task 5）
- ✅ 查询关联角色（Task 3）
- ✅ 过滤已生成配置的角色（Task 3）
- ✅ 复制 skill 文件（Task 3，复用 updateSkillFiles）
- ✅ 返回刷新错误（Task 4, Task 5）
- ✅ 前端展示错误（Task 7，可选）

**2. Placeholder scan:** 无 TBD/TODO

**3. Type consistency:**
- `RefreshError` 结构体定义一致
- `FindBySkillID` 方法签名正确
- `ConfigGeneratedAt` 字段名正确

---

**Plan complete and saved to `docs/superpowers/plans/2026-05-08-skill-sync-refresh-agent-configs.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**