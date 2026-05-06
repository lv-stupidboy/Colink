# Skill 重名允许设计

## 需求背景

当前 Skills 管理中，skill 新建时不允许重名（数据库唯一约束）。需求变更为：
1. **允许 skill 重名**：所有来源类型的 skill 都可以使用相同名称（platform、personal、federated）
2. **角色关联使用 ID**：已有实现使用 skill.id 进行关联（无需变更）
3. **下拉框显示描述区分**：角色选择时用括号展示描述来区分同名 skill

## 当前实现分析

### 数据库层
- `skills` 表有 `UNIQUE KEY uk_skills_name (name)` 唯一约束 - 阻止重名
- `source_registry_id` 字段已存在但未参与唯一约束
- `agent_skill_bindings`、`subagent_skill_bindings`、`command_skill_bindings` 均使用 `skill_id` 关联（已符合需求）

### Service 层
- `skill/service.go:Create()` 检查名称重复返回 "技能名称已存在"
- `skill/skill_scanner.go:ImportSkills()` 用 `FindByName` 检查，跳过同名

### 前端
- Agent 角色绑定 Skills 下拉列表使用 `s.id` 作为 value，`s.name` 作为 label
- 已正确使用 ID 进行关联（无需变更）

## 设计方案

### 核心变更：允许所有来源类型重名

**唯一性规则变更**：
| 变更前 | 变更后 |
|--------|--------|
| `name` 全局唯一 | 无唯一约束，所有来源类型都可重名 |

**设计理由**：
- 用户可能创建多个同名但描述不同的 skill（如不同版本的 "code-review"）
- 联邦源导入的 skill 可能与本地 skill 同名
- 通过 **描述** 在下拉框区分同名 skill，用户可直观选择

### 数据库变更

**删除唯一约束**：

```sql
-- v1.2.5/sqlite/00011_drop_skill_name_unique.sql
-- 删除原有的名称唯一约束，允许 skill 重名

-- +goose Up
ALTER TABLE skills DROP INDEX uk_skills_name;

-- +goose Down
ALTER TABLE skills ADD UNIQUE INDEX uk_skills_name (name);
```

**无需应用层校验**：
- 删除约束后，创建 skill 不检查名称唯一性
- 联邦导入时也不检查名称唯一性（允许同名共存）
- 简化实现，无复杂条件判断

### Service 层变更

#### 1. 修改 skill/service.go Create 方法

删除名称唯一性检查：

```go
// Create 创建Skill
func (s *Service) Create(ctx context.Context, req *model.CreateSkillRequest) (*model.Skill, error) {
    // 删除原有的名称唯一性检查（允许重名）
    // 直接创建，不检查 FindByName
    
    // ... 创建逻辑保持不变
}
```

#### 2. 修改 skill/skill_scanner.go 导入检查逻辑

联邦导入时删除名称唯一性检查：

```go
// ImportSkills 批量导入技能
func (s *SkillScanner) ImportSkills(ctx context.Context, req *model.BatchImportRequest) (*model.BatchImportResult, error) {
    // 删除原有的 FindByName 检查（允许同名共存）
    // 直接导入，不检查是否已存在同名 skill
    
    // ... 导入逻辑保持不变
}
```

#### 3. 可选优化：扫描结果显示同名提示

扫描联邦源时，可显示是否有同名 skill（提示信息，不阻止导入）：

```go
// RemoteSkill 可新增字段用于前端提示
type RemoteSkill struct {
    HasSameNameLocal bool `json:"hasSameNameLocal"` // 本地是否有同名 skill
}
```

### Model 层变更

无需新增字段（可选优化见 Service 层）。

### 前端变更

#### Skill 绑定下拉列表显示优化

Agent 角色、Subagent、Command 绑定 Skills 的下拉列表中，**用括号展示描述区分同名 skill**：

```tsx
// AgentRoleList.tsx / SubagentList.tsx / CommandList.tsx
// Skills 绑定的 Select 下拉框

options={skills.map(s => ({
    label: `${s.name} (${s.description || '暂无描述'})`,  // 括号展示描述
    value: s.id,
}))}

// 或者使用自定义渲染（更美观）
render={(option) => (
    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
        <span style={{ fontWeight: 500 }}>{option.data?.name}</span>
        <span style={{ fontSize: 12, color: '#999', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
            ({option.data?.description || '暂无描述'})
        </span>
    </div>
)}
```

**显示效果示例**：
- `code-review (代码质量审查，关注安全和性能)`
- `code-review (GitLab MR 审查，检查合规性)`
- `test-gen (单元测试生成)`

## 实施步骤

### Phase 1: 数据库变更
1. 创建 `sql-change/v1.2.5/sqlite/` 目录
2. 编写 `00011_drop_skill_name_unique.sql` 迁移脚本删除 `uk_skills_name` 约束
3. 更新 `init/init-sqlite.sql` 同步变更

### Phase 2: 后端实现
1. `internal/service/skill/service.go` 删除 Create 方法的名称唯一性检查
2. `internal/service/skill/skill_scanner.go` 删除 ImportSkills 的名称检查

### Phase 3: 前端优化
1. AgentRoleList.tsx Skills 下拉列表用括号展示描述
2. SubagentList.tsx Skills 下拉列表用括号展示描述
3. CommandList.tsx Skills 下拉列表用括号展示描述

## 测试要点

1. **创建同名 skill**：
   - 创建同名 platform skill → 成功（允许重名）
   - 创建同名 personal skill → 成功（允许重名）
   - 创建同名 federated skill → 成功（允许重名）

2. **角色绑定**：
   - 绑定同名 skill（不同描述）→ 正确使用 ID 关联
   - 下拉列表显示 `name (description)` 格式区分同名 skill

3. **联邦导入**：
   - 导入与本地同名的 skill → 成功（共存）
   - 导入与其他联邦源同名的 skill → 成功（共存）

## 影响范围

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `sql-change/v1.2.5/sqlite/00011_drop_skill_name_unique.sql` | 新增 | 数据库迁移脚本（删除唯一约束） |
| `internal/service/skill/service.go` | 修改 | 删除 Create 名称唯一性检查 |
| `internal/service/skill/skill_scanner.go` | 修改 | 删除 ImportSkills 名称检查 |
| `web/src/pages/AgentRoleList.tsx` | 修改 | Skills 下拉用括号展示描述 |
| `web/src/pages/SubagentList.tsx` | 修改 | Skills 下拉用括号展示描述 |
| `web/src/pages/CommandList.tsx` | 修改 | Skills 下拉用括号展示描述 |
| `init/init-sqlite.sql` | 修改 | 同步迁移内容 |

此方案兼容 SQLite 和 MySQL，实现简洁。