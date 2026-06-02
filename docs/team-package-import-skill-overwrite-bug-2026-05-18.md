# 团队包导入 Skill 覆盖导致其他团队绑定丢失 Bug 分析

**日期**: 2026-05-18
**严重程度**: P0 - 数据完整性问题
**影响范围**: 跨团队数据污染

## Bug 描述

用户导入团队包时，如果团队包中有角色关联的 Skills 与已存在的其他团队的 skill 名称相同，选择"覆盖"后，导入成功但**其他团队的角色下的同名 skill 消失**。

## 问题根因

**位置**: `internal/service/teampackage/service.go:1054-1113` (`importSkill` 函数)

**问题描述**:

当用户选择"overwrite"模式且存在同名 skill 时：

```go
// 第 1068-1076 行：覆盖模式处理
if overwrite {
    // 1. 删除旧 skill 记录
    if err := s.skillRepo.Delete(ctx, existing.ID); err != nil { ... }
    os.RemoveAll(oldDir) // 删除旧文件目录
}

// 第 1078-1092 行：创建新 skill（使用新 ID）
skill := &model.Skill{
    ID: uuid.New(), // 问题关键：新 ID！
    Name: item.Name,
    // ...
}
```

**问题链路**:

1. **删除旧 skill** → skill 表中记录被删除
2. **没有清理绑定记录** → `agent_skill_bindings` 表中仍保留指向旧 skill ID 的记录
3. **创建新 skill（新 ID）** → 新 skill 与旧绑定无关
4. **旧绑定成为孤儿记录** → 指向不存在的 skill ID
5. **其他团队查询失败** → 绑定记录存在但 skill 不存在

**验证**: 没有调用任何绑定清理方法：
- `AgentSkillBindingRepository.DeleteBySkillID` 不存在
- `CommandSkillBindingRepository.DeleteBySkillID` 不存在
- `SubagentSkillBindingRepository.DeleteBySkillID` 不存在

## 数据模型分析

```
skills 表 (全局共享资源)
┌─────────────────────────────┐
│ id: uuid (唯一)             │ ← 删除后 ID 变化
│ name: string (全局唯一)     │
└─────────────────────────────┘
        ↓ 外键关联
agent_skill_bindings 表
┌─────────────────────────────┐
│ agent_role_id: uuid         │ ← 其他团队的角色
│ skill_id: uuid              │ ← 指向已删除的 skill
└─────────────────────────────┘
```

**关键点**: Skill 是全局共享资源，多个团队的角色可以绑定同一个 skill。

## 修复方案对比

| 方案 | 描述 | 优点 | 缺点 |
|------|------|------|------|
| **A: 删除+清理绑定** | 删除 skill 时同步清理所有绑定 | 数据一致性 | **其他团队的绑定也会被删除** (问题更严重) |
| **B: 更新而非删除** | 复用现有 skill ID，只更新内容 | **保留现有绑定关系**，其他团队不受影响 | 需确保其他属性正确更新 |

**推荐方案 B**，理由：
1. Skill 是全局共享资源，不应因导入团队包而影响其他团队
2. 复用 ID 能保持所有现有绑定关系
3. 符合"共享资源"的设计理念 - name 唯一意味着同一 skill

## 修复方案 B 详细步骤

### 修改 `importSkill` 函数

**位置**: `internal/service/teampackage/service.go:1054-1113`

**修改逻辑**:

```go
// 检查是否已存在相同名称的 Skill
existing, err := s.skillRepo.FindByName(ctx, item.Name)
if err == nil && existing != nil {
    if !overwrite {
        detail.Status = "skipped"
        detail.Message = "已存在相同名称的 Skill"
        return existing.ID, detail // 返回现有 ID
    }

    // 覆盖模式：更新现有 skill，保留 ID
    // 1. 删除旧文件目录（使用 ID 作为目录名）
    oldDir := filepath.Join(s.skillStoragePath, existing.ID.String())
    os.RemoveAll(oldDir)

    // 2. 复制新文件到 ID 目录
    srcDir := filepath.Join(tempDir, "assets", "skills", item.Name)
    targetDir := filepath.Join(s.skillStoragePath, existing.ID.String())
    if err := copyDir(srcDir, targetDir); err != nil {
        detail.Status = "failed"
        detail.Message = fmt.Sprintf("复制 Skill 目录失败: %v", err)
        return uuid.Nil, detail
    }

    // 3. 更新 skill 属性（保留 ID 和绑定关系）
    existing.Description = item.Description
    existing.Tags = item.Tags
    existing.SupportedAgents = item.SupportedAgents
    existing.IsPublic = item.IsPublic
    existing.UpdatedAt = time.Now()

    if err := s.skillRepo.Update(ctx, existing); err != nil {
        detail.Status = "failed"
        detail.Message = fmt.Sprintf("更新 Skill 记录失败: %v", err)
        return uuid.Nil, detail
    }

    detail.Status = "success"
    detail.ID = existing.ID.String()
    return existing.ID, detail
}

// 创建新 Skill（不存在同名 skill 时）
skill := &model.Skill{
    ID: uuid.New(),
    Name: item.Name,
    // ...
}
```

### 关键改动点

1. **不删除 skill 记录** - 保留 `existing.ID`
2. **不删除绑定记录** - 绑定自然保持
3. **更新属性而非重建** - `skillRepo.Update(ctx, existing)`
4. **返回现有 ID** - 确保 `skillNameToID` 映射正确

### 文件目录处理

当前代码使用 `skill.Name` 作为目录名：
```go
oldDir := filepath.Join(s.skillStoragePath, existing.Name) // 错误
```

实际上 skill 目录应该使用 `skill.ID` 作为目录名（见第1096行）：
```go
targetDir := filepath.Join(s.skillStoragePath, skill.ID.String())
```

修改为使用 ID 目录：
```go
oldDir := filepath.Join(s.skillStoragePath, existing.ID.String())
```

## 测试验证

### 测试场景

1. **前置条件**: Team A 的角色 R1 绑定 skill S (name="brainstorming")
2. **操作**: 导入 Team B 的团队包，包含同名 skill S (name="brainstorming")，选择 overwrite
3. **预期结果**:
   - Team B 导入成功
   - Team A 的角色 R1 仍然绑定 skill S
   - skill ID 不变

### 测试代码

```go
func TestSkillOverwritePreservesBindings(t *testing.T) {
    // Setup: 创建 skill S1，Team A 的角色 R1 绑定 S1
    // Operation: 导入同名 skill (overwrite=true)
    // Verify: R1 的绑定仍然存在，skill ID 不变
}
```

## 影响评估

| 受影响场景 | 修复前 | 修复后 |
|-----------|--------|--------|
| 导入同名 skill (overwrite) | 其他团队绑定丢失 | 绑定保留，内容更新 |
| 导入同名 skill (skip) | 正常 | 正常 |
| 导入新 skill | 正常 | 正常 |

## 结论

**根因**: `importSkill` 函数在覆盖模式下删除重建 skill，导致 ID 变化，绑定关系断裂。

**修复**: 采用"更新而非删除"策略，复用现有 skill ID，保留所有绑定关系。

**优先级**: P0 - 这是数据完整性问题，影响跨团队数据。