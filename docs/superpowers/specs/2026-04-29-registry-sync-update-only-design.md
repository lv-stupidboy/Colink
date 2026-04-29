---
name: 联邦源同步行为修改
description: 联邦源同步时不再自动添加新 skill，只更新已存在的 skill
type: project
---

# 联邦源同步行为修改设计

## 背景

当前联邦源同步逻辑会自动将远程仓库中所有 skill 添加到本地数据库。用户反馈不应自动添加未添加的 skill，应只更新已添加的相同 skill。

## 需求确认

1. **"已添加的 skill"定义**：用户手动通过其他方式添加的 skill（如新建 skill 页面）
2. **匹配规则**：按名称匹配（远程 skill 名称与本地 skill 名称相同则更新）
3. **同步行为**：不自动创建新 skill，只更新本地已有的 skill

## 设计

### 修改文件

1. `internal/service/skill/registry_service.go` - 后端同步逻辑
2. `web/src/pages/RegistryManagement/index.tsx` - 前端同步成功提示

### 后端修改内容

修改 `Sync` 方法（第 159-194 行）的同步逻辑：

**当前逻辑：**
```go
for _, remoteSkill := range skills {
    existing, err := s.skillRepo.FindByName(ctx, remoteSkill.Name)
    if err != nil {
        // 不存在，创建新技能
        skill := &model.Skill{...}
        s.skillRepo.Create(ctx, skill)
        result.SkillsAdded++
    } else {
        // 已存在，更新技能
        existing.Description = remoteSkill.Description
        ...
        s.skillRepo.Update(ctx, existing)
        result.SkillsUpdated++
    }
}
```

**修改后逻辑：**
```go
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

### 前端修改内容

修改同步成功提示（第 121 行）：

**当前显示：**
```tsx
message.success(`同步成功：新增 ${result.skillsAdded}，更新 ${result.skillsUpdated}`);
```

**修改后显示：**
```tsx
message.success(`同步成功：更新 ${result.skillsUpdated} 个技能`);
```

去掉 `skillsAdded` 显示，避免"新增 0"给用户造成困惑。

### 影响

- `SyncResult.SkillsAdded` 始终为 0（后端可保留字段，前端显示时忽略）
- 用户需要通过新建 skill 页面手动添加 skill 后，同步才能更新

## 测试验证

1. 创建联邦源并同步，验证不自动添加新 skill
2. 手动创建 skill（名称与远程 skill 相同），再次同步，验证 skill 被更新