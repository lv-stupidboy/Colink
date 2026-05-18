# 团队包导入 Skill 覆盖保留 ID 修复

## 问题定位

团队包导入时，如果选择覆盖已存在的 Skill，会删除旧 Skill 记录并创建新的（生成新 UUID）。这会导致：
- Skill 是全局共享资源，多个团队的角色可能绑定同一个 Skill
- ID 变化后，其他团队对该 Skill 的绑定关系断开

## 根因分析

`importSkill` 函数（`service.go:1054-1113`）的覆盖逻辑：
1. 检查是否存在同名 Skill
2. 覆盖时：删除旧记录 + 删除旧目录 + 创建新记录（新 UUID）
3. 问题：删除重建导致 ID 变化

## 修复方案

**方案 B：复用现有 Skill ID，只更新内容和属性**

修改后的逻辑：
1. 检查是否存在同名 Skill
2. 覆盖时：
   - 保留现有 Skill 的 ID
   - 清空旧目录内容，复制新内容
   - 使用 `Update` 方法更新属性（Description、Tags、SupportedAgents、IsPublic、SourceType）
3. 不存在时：正常创建新 Skill

## 代码变更

### `internal/service/teampackage/service.go`

修改 `importSkill` 函数（第 1053-1113 行）：

```go
// importSkill 导入单个 Skill
// 覆盖模式下保留现有 ID，只更新内容和属性，避免断开其他团队的绑定关系
func (s *Service) importSkill(ctx context.Context, tempDir string, item model.AssetPackageSkillItem, overwrite bool) (uuid.UUID, model.ImportDetail) {
    // 检查是否已存在相同名称的 Skill
    existing, err := s.skillRepo.FindByName(ctx, item.Name)
    if err == nil && existing != nil {
        if !overwrite {
            detail.Status = "skipped"
            detail.Message = "已存在相同名称的 Skill"
            return existing.ID, detail  // 返回现有 ID
        }
        // 覆盖模式：保留现有 ID，只更新内容和属性
        // 清空旧目录，复制新内容
        targetDir := filepath.Join(s.skillStoragePath, existing.ID.String())
        os.RemoveAll(targetDir)
        copyDir(srcDir, targetDir)

        // 更新属性（保留原 ID）
        existing.Description = item.Description
        existing.Tags = item.Tags
        existing.SupportedAgents = item.SupportedAgents
        existing.IsPublic = item.IsPublic
        existing.SourceType = item.SourceType
        s.skillRepo.Update(ctx, existing)

        return existing.ID, detail  // 返回原 ID
    }
    // 创建新 Skill（不存在同名 Skill）
    skill := &model.Skill{
        ID: uuid.New(),  // 新 ID
        ...
    }
    s.skillRepo.Create(ctx, skill)
    return skill.ID, detail
}
```

### `internal/service/teampackage/service_test.go`

新增测试验证修复逻辑：

- `TestSkillOverwritePreservesID` (TP-TEST-03)：验证覆盖后 ID 保留
- `TestSkillOverwriteBeforeFix` (TP-TEST-04)：演示修复前问题

## 测试验证

```
=== RUN   TestSkillOverwritePreservesID
    service_test.go:121: ✅ 修复验证通过：覆盖后 skill ID 保持为 xxx，Team A 的绑定有效
--- PASS: TestSkillOverwritePreservesID

=== RUN   TestSkillOverwriteBeforeFix
    service_test.go:165: ✅ 正确演示修复前问题：Team A 绑定的 skill ID xxx 已不存在
--- PASS: TestSkillOverwriteBeforeFix
```

## 影响范围

- 团队包导入功能
- Skill 导入覆盖场景
- 不影响其他资产类型（Command、Subagent、Rule、Settings）的导入逻辑

## 风险评估

- **风险等级**：低
- **回滚方案**：恢复原 `importSkill` 函数逻辑（删除重建）
- **兼容性**：不影响现有数据，只是改变了覆盖行为