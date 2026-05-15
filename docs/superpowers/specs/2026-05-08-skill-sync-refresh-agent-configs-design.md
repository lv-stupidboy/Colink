# Skill 同步刷新 Agent Configs 设计文档

**日期**: 2026-05-08
**状态**: 待审核

## 背景

当前联邦源同步/导入更新 skill 时，只刷新 `agent-assets/skills/{skill-id}/` 目录，不会刷新 `agent-configs/{agent-id}/skills/` 目录。

问题：
- 角色配置生成后，skill 文件被复制到 `agent-configs/{agent-id}/skills/{skill-name}/`
- 联邦源更新 skill 内容后，agent-configs 下的 skill 文件不会同步更新
- Agent 执行时可能使用过期的 skill 文件

## 需求

联邦源同步/导入更新 skill 成功后，立即刷新已生成配置的角色关联的 skill 文件。

### 决策记录

| 决策项 | 选择 | 原因 |
|--------|------|------|
| 触发时机 | 立即刷新 | 保证 Agent 执行时使用最新 skill 内容 |
| 刷新范围 | 只刷新已生成配置的角色 | 避免刷新从未生成过配置的角色，减少不必要操作 |
| 失败处理 | 收集并返回失败信息 | 用户需要知道刷新状态，便于排查问题 |
| 实现方案 | 直接调用刷新 | 实现简单，失败信息可立即返回 |

## 技术设计

### 数据流

```
联邦源同步/导入更新 Skill
       ↓
更新 agent-assets/skills/{skill-id}/
       ↓
查询 agent_skill_bindings 表
       ↓
获取关联角色 ID 列表
       ↓
过滤 config_generated_at 不为空的角色
       ↓
刷新 agent-configs/{agent-id}/skills/{skill-name}/
       ↓
返回刷新结果（成功/失败）
```

### 涉及的表和字段

| 表名 | 字段 | 说明 |
|------|------|------|
| `agent_skill_bindings` | `agent_role_id`, `skill_id` | 角色-Skill 关联 |
| `agent_configs` | `config_generated_at`, `config_path` | 角色配置生成状态 |

### 新增方法

**`refreshAgentConfigsForSkill(ctx context.Context, skillID uuid.UUID) []RefreshError`**

参数：
- `skillID`: 被更新的 skill ID

返回：
- 刷新失败列表（空表示全部成功）

逻辑：
1. 查询 `agent_skill_bindings` 获取关联角色 ID
2. 查询 `agent_configs` 过滤 `config_generated_at IS NOT NULL`
3. 获取 skill 信息（名称、存储路径）
4. 遍历每个角色：
   - 从 `agent-assets/skills/{skill-id}/` 复制到 `agent-configs/{agent-id}/skills/{skill-name}/`
   - 失败时记录错误
5. 返回错误列表

### 调用时机

在以下方法完成后调用刷新：

1. **`ImportSkills`**（更新模式）
   - 位置：`skill_scanner.go`
   - 触发：用户选择"更新"已有 skill

2. **`SyncConfirm`**（用户选择更新）
   - 位置：`registry_service.go`
   - 触发：联邦源手动同步确认

### 返回结果扩展

扩展 `BatchImportResult` 和 `SyncConfirmResult`：

```go
type RefreshError struct {
    AgentRoleID   uuid.UUID `json:"agentRoleId"`
    AgentRoleName string    `json:"agentRoleName"`
    Error         string    `json:"error"`
}

// BatchImportResult 添加字段
ConfigRefreshErrors []RefreshError `json:"configRefreshErrors,omitempty"`

// SyncConfirmResult 添加字段
ConfigRefreshErrors []RefreshError `json:"configRefreshErrors,omitempty"`
```

### 文件复制逻辑

复用 `configgen/downloader.go` 中的 `copyDirWithRetry` 方法：

```go
// 源目录: agent-assets/skills/{skill-id}/
srcDir := filepath.Join(skillStoragePath, skill.ID.String())

// 目标目录: agent-configs/{agent-id}/skills/{skill-name}/
dstDir := filepath.Join(agentConfigPath, agentRoleID.String(), "skills", skill.Name)
```

## 依赖

- `AgentSkillBindingRepository.FindBySkillID`: 获取关联角色
- `AgentConfigRepository.FindByID`: 获取角色配置信息
- `SkillRepository.FindByID`: 获取 skill 名称
- `configgen.Downloader.copyDirWithRetry`: 复制目录

## 错误处理

| 错误类型 | 处理方式 |
|----------|----------|
| 查询失败 | 跳过该 skill，记录日志 |
| 目标目录不存在 | 创建目录 |
| 复制失败 | 记录错误，继续处理其他角色 |
| 权限问题 | 返回错误，提示用户检查权限 |

## 测试场景

1. 更新 skill 后，角色配置目录中 skill 文件同步更新
2. skill 未被任何角色关联，不执行刷新
3. 角色未生成配置，不执行刷新
4. 复制失败时，返回错误信息
5. 多个角色关联同一 skill，全部刷新

## 实施任务预估

1. 扩展返回结果模型（添加 `RefreshError` 和 `ConfigRefreshErrors`）
2. 实现 `refreshAgentConfigsForSkill` 方法
3. 在 `ImportSkills` 更新流程中调用刷新
4. 在 `SyncConfirm` 更新流程中调用刷新
5. 前端展示刷新失败警告提示（可选）
6. 单元测试

---

**审核通过后，将进入实施计划编写阶段。**