---
name: skill-path-distinction
description: 联邦源重复 skill 按路径区分功能设计
type: project
---

# 联邦源重复 Skill 按路径区分功能设计

## 背景

当前联邦源存在多个同名 skill 时，导入和同步存在以下问题：

- **导入**：直接创建新记录，多个同名 skill 会创建多条记录，无法区分来源
- **同步**：使用 `FindByName` 只返回第一条，后续同名 skill 的更新被忽略
- **展示**：前端无法区分同名 skill 来自哪个目录

## 解决方案

新增 `SourcePath` 字段，存储联邦源仓库中的相对路径，用于精确区分同名 skill。

## 设计决策

| 决策点 | 选择 | 原因 |
|--------|------|------|
| 实现方式 | 新增 `SourcePath` 字段 | Name 保持简洁，路径单独存储 |
| 唯一性约束 | 不加约束 | 允许重名，用户通过路径区分 |
| 同步匹配 | 精确匹配 `(Name, SourceRegistryID, SourcePath)` | 确保每个路径对应独立记录 |
| 前端展示 | 卡片式展示路径 | 用户快速区分同名 skill |

## 数据模型变更

### Skill 表新增字段

```sql
-- SQLite
ALTER TABLE skills ADD COLUMN source_path TEXT DEFAULT '';

-- MySQL
ALTER TABLE skills ADD COLUMN source_path VARCHAR(255) DEFAULT '';
```

### Go Model 更新

```go
type Skill struct {
    // ... 现有字段
    SourcePath string `json:"sourcePath,omitempty"` // 联邦源仓库相对路径
}
```

### TypeScript 类型更新

```typescript
interface Skill {
  // ... 现有字段
  sourcePath?: string;
}
```

---

**Why:** 联邦源仓库可能在不同目录下存在同名 skill（如 `skills/review/SKILL.md` 和 `skills/code-review/SKILL.md`），需要用路径区分。

**How to apply:** 导入时存储路径，同步时精确匹配，展示时显示路径。

---

## 后端改动

### 1. 导入逻辑 (`ImportSkills`)

存储 `SourcePath` 字段：

```go
skill := &model.Skill{
    // ... 现有字段
    SourcePath: item.Path, // 存储仓库相对路径
}
```

### 2. 同步匹配逻辑

新增 `FindBySourcePath` 方法：

```go
// FindBySourcePath 根据名称 + 联邦源ID + 路径查找
func (r *SkillRepository) FindBySourcePath(ctx context.Context, name string, registryID uuid.UUID, path string) (*model.Skill, error) {
    query := `
        SELECT id, name, description, tags, source_type, source_registry_id, source_path, ...
        FROM skills 
        WHERE name = ? AND source_registry_id = ? AND source_path = ?
    `
    // ...
}
```

同步时使用精确匹配：

```go
// SyncPreview 中
existing, err := s.skillRepo.FindBySourcePath(ctx, remoteSkill.Name, registry.ID, remoteSkill.Path)
```

### 3. 扫描结果返回路径

`RemoteSkill.Path` 已有，无需改动。确保前端传递路径信息。

---

## 前端改动

### 1. Skill 卡片展示路径

在 skill 卡片中新增一行显示路径（仅联邦类型）：

```tsx
{/* 路径区域 - 仅联邦类型显示 */}
{skill.sourceType === 'federated' && skill.sourcePath && (
  <div style={{ marginBottom: 4 }}>
    <Text type="secondary" style={{ fontSize: 12 }}>
      路径: {skill.sourcePath}
    </Text>
  </div>
)}
```

### 2. 导入时传递路径

`SkillImportItem` 已有 `Path` 字段，无需改动。

### 3. 冲突弹窗展示路径

冲突弹窗中显示完整路径信息：

```tsx
{
  title: '路径',
  dataIndex: 'path',
  key: 'path',
  width: 150,
  render: (path: string) => <Text type="secondary">{path || '-'}</Text>,
}
```

---

## 兼容性考虑

### 已有数据迁移

- 已有 skill 的 `source_path` 默认为空字符串
- 精确匹配逻辑：始终使用 `(Name, SourceRegistryID, SourcePath)` 三元组
- 路径不匹配（空 vs 非空）→ 视为异源冲突，触发弹窗让用户选择

### API 兼容

- `sourcePath` 字段可选，不影响现有 API
- 新增 `FindBySourcePath` 方法，不影响现有 `FindByName`

---

## 测试要点

| 测试项 | 验证内容 |
|--------|----------|
| 导入同名 skill | 不同路径分别创建记录 |
| 同步同名 skill | 精确匹配各路径的记录 |
| 前端展示 | 卡片显示路径信息 |
| 兼容旧数据 | 空路径的 skill 正常匹配 |

---

## 实施步骤

1. **数据库迁移** — 新增 `source_path` 列
2. **后端模型** — Skill 结构体新增字段
3. **后端 Repo** — 新增 `FindBySourcePath` 方法
4. **后端 Service** — 导入存储路径、同步精确匹配
5. **前端类型** — Skill 类型新增 `sourcePath`
6. **前端展示** — 卡片显示路径、弹窗展示路径
7. **集成测试** — 导入/同步/展示验证