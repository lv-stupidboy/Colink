# 联邦源同步行为修改实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修改联邦源同步行为，不再自动添加新 skill，只更新本地已存在的 skill（按名称匹配）

**Architecture:** 修改 `RegistryService.Sync` 方法，删除"不存在则创建"的逻辑；修改前端同步成功提示，去掉 `skillsAdded` 显示

**Tech Stack:** Go (后端), React + Ant Design (前端)

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/service/skill/registry_service.go` | 修改 | 后端同步逻辑 |
| `web/src/pages/RegistryManagement/index.tsx` | 修改 | 前端同步成功提示 |

---

### Task 1: 修改后端同步逻辑

**Files:**
- Modify: `internal/service/skill/registry_service.go:159-194`

- [ ] **Step 1: 修改 Sync 方法，删除"创建新技能"逻辑**

将第 159-194 行的同步循环修改为：

```go
// 同步技能到本地（只更新已存在的）
for _, remoteSkill := range skills {
    existing, err := s.skillRepo.FindByName(ctx, remoteSkill.Name)
    if err != nil {
        // 不存在，跳过（不自动添加）
        continue
    }
    // 已存在，更新技能
    existing.Description = remoteSkill.Description
    existing.Tags = remoteSkill.Tags
    existing.SupportedAgents = remoteSkill.SupportedAgents
    existing.UpdatedAt = time.Now()
    if err := s.skillRepo.Update(ctx, existing); err != nil {
        continue
    }
    result.SkillsUpdated++
}
```

- [ ] **Step 2: 验证编译通过**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交后端修改**

```bash
git add internal/service/skill/registry_service.go
git commit -m "refactor: 联邦源同步改为只更新不新增"
```

---

### Task 2: 修改前端同步成功提示

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx:121`

- [ ] **Step 1: 修改同步成功提示文本**

将第 121 行从：
```tsx
message.success(`同步成功：新增 ${result.skillsAdded}，更新 ${result.skillsUpdated}`);
```

修改为：
```tsx
message.success(`同步成功：更新 ${result.skillsUpdated} 个技能`);
```

- [ ] **Step 2: 提交前端修改**

```bash
git add web/src/pages/RegistryManagement/index.tsx
git commit -m "refactor: 联邦源同步提示去掉 skillsAdded 显示"
```

---

### Task 3: 验证功能

- [ ] **Step 1: 启动后端服务**

Run: `go run ./cmd/server`
Expected: 服务启动成功

- [ ] **Step 2: 启动前端开发服务器**

Run: `cd web && npm run dev`
Expected: 前端启动成功

- [ ] **Step 3: 测试同步功能**

1. 打开联邦源页面
2. 点击现有联邦源的"同步"按钮
3. 验证提示显示"同步成功：更新 X 个技能"
4. 验证不会自动添加新 skill

- [ ] **Step 4: 合并提交并推送**

```bash
git push origin feature/codehub-registry-support
```

---

## 自审检查

**1. Spec coverage:** 
- ✓ 后端删除"创建新技能"逻辑 - Task 1
- ✓ 前端去掉 skillsAdded 显示 - Task 2
- ✓ 功能验证 - Task 3

**2. Placeholder scan:** 无 TBD/TODO

**3. Type consistency:** 方法名、变量名一致