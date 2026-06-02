# 团队包导入角色Skill绑定丢失问题分析

**日期**: 2026-05-18
**问题**: 导入团队包后，角色下没有绑定skill

## 问题复现

用户导入两个团队包：
- `CoreDev全流程开发团队1`
- `CoreDev全流程开发团队 (2)`

导入后发现角色没有绑定skill。

## 根因分析

**文件**: `internal/service/teampackage/service.go`

### Bug位置

第885-903行，角色跳过导入的处理逻辑：

```go
if action == "skip" {
    // 尝试解析原始ID，如果无效则跳过
    originalID, err := uuid.Parse(roleItem.ID)
    if err == nil {
        existing, _ := s.agentRepo.FindByID(ctx, originalID)
        if existing != nil {
            roleNameToID[roleItem.Name] = existing.ID
            originalRoleIDToNewID[roleItem.ID] = existing.ID
        }
    }
    result.Skipped++
    ...
    continue
}
```

### 问题说明

1. **只按ID查找**：代码尝试用原始ID（`roleItem.ID`）查找本地角色
2. **跨客户端场景**：团队包导出时角色ID是导出客户端的本地UUID，导入到另一个客户端时这些ID不存在
3. **映射缺失**：当原始ID在本地不存在时，`roleNameToID[roleItem.Name]` 不会被更新
4. **绑定失败**：后续绑定恢复（第929-995行）依赖 `roleNameToID` 映射，映射缺失导致绑定失败

### 正确做法对比

在 `importRole` 内部跳过时（第912-922行），代码按**名称**查找：

```go
case "skipped":
    result.Skipped++
    // 查找已存在的角色
    agents, _ := s.agentRepo.List(ctx)
    for _, agent := range agents {
        if agent.Name == roleItem.Name {
            roleNameToID[roleItem.Name] = agent.ID
            originalRoleIDToNewID[originalIDStr] = agent.ID
            break
        }
    }
```

### Manifest内容确认

团队包manifest中角色确实包含bindings信息：

```json
{
  "name": "需求分析师",
  "bindings": {
    "skills": ["brainstorming", "writing-plans"],
    "commands": ["brainstorm", "write-plan"]
  }
}
```

## 修复方案

修改第885-903行的跳过处理逻辑，按**名称**查找本地角色（与importRole内部保持一致）：

```go
if action == "skip" {
    // 按名称查找已存在的角色
    agents, _ := s.agentRepo.List(ctx)
    for _, agent := range agents {
        if agent.Name == roleItem.Name {
            roleNameToID[roleItem.Name] = agent.ID
            // 也更新 originalRoleIDToNewID 映射（如果有有效原始ID）
            if originalID, err := uuid.Parse(roleItem.ID); err == nil {
                originalRoleIDToNewID[roleItem.ID] = agent.ID
            }
            break
        }
    }
    result.Skipped++
    result.Details = append(result.Details, model.ImportDetail{
        AssetType: "role",
        Name:      roleItem.Name,
        Status:    "skipped",
        Message:   "用户选择跳过",
    })
    continue
}
```

## 影响范围

- 团队包导入功能
- 跨客户端导入场景
- 角色绑定恢复（skills, commands, subagents, rules, settings）

## 测试验证

修复后应验证：
1. 导入相同名称但不同ID的团队包
2. 选择跳过角色导入
3. 确认角色绑定关系正确恢复

---

## 修复记录

**修复时间**: 2026-05-18 21:02
**修复人**: SuperPowers全栈开发工程师
**修改文件**: `internal/service/teampackage/service.go:885-904`

**修改内容**: 将"skip"角色的处理逻辑从"按原始ID查找"改为"按名称查找"，与 `importRole` 内部保持一致。

**验证状态**: 编译通过，等待测试验证