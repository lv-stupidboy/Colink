# 团队包导入 BaseAgentID 保留问题修复

**日期**: 2026-05-15
**提交**: efa4066
**问题**: 市场导入和本地导入团队包时，已存在角色的 BaseAgentID 被重置为空

## 问题分析

### 根本原因

`importRole` 方法在导入角色时：
1. TeamPackageRole 模型中没有 BaseAgentID 字段
2. 创建新角色时未设置 BaseAgentID
3. 导致 BaseAgentID 为空（uuid.Nil）

### 影响范围

- 市场导入团队包（`teampackagesync/service.go` 的 `SyncPackageWithCache`）
- 本地导入团队包（`teampackage/service.go` 的 `ImportConfirm`）

## 需求确认（已实现）

| 场景 | BaseAgentID 处理策略 |
|------|---------------------|
| **导出** | 导出 BaseAgentID 和 BaseAgentName 到团队包 |
| **新建角色** | 包中值 → 系统默认 → 空（三级 fallback） |
| **覆盖已存在角色** | 包中值 → 本地值（两级 fallback） |

## 实现说明

### 模型修改

`TeamPackageRole` 添加可选字段：
- `BaseAgentID string` - 基础 Agent ID
- `BaseAgentName string` - 基础 Agent 名称（用于显示）

### 导出逻辑

导出团队包时，如果角色有配置基础 Agent，记录其 ID 和名称到 manifest。

### 导入逻辑

**新建角色（本地不存在）**：
```
包中 BaseAgentID → 系统默认 BaseAgent → 空
```

**覆盖已存在角色**：
```
包中 BaseAgentID → 本地已有 BaseAgentID
```

## 实际修改

### 文件 1: `internal/model/team_package.go`

添加 BaseAgentID 和 BaseAgentName 字段到 TeamPackageRole 结构体。

### 文件 2: `internal/service/teampackage/service.go`

1. Service 结构体添加 `baseAgentRepo` 依赖
2. NewService 添加 `baseAgentRepo` 参数
3. Export 方法：导出时填充 BaseAgentID 和 BaseAgentName
4. importRole 方法：实现 BaseAgentID fallback 策略

### 文件 3: `cmd/server/main.go`

teampackage.NewService 调用添加 `baseAgentRepo` 参数。

## 验收标准

- [x] 导出团队包时包含角色的 BaseAgentID 和 BaseAgentName
- [x] 新建角色时：包中 BaseAgentID → 系统默认 → 空
- [x] 覆盖已存在角色时：包中 BaseAgentID → 本地值
- [x] Go 编译成功
- [x] 代码已提交（不 push）

## 测试建议

1. 创建一个团队包，角色配置不同的基础 Agent
2. 导出团队包，验证 manifest.json 包含 baseAgentId 和 baseAgentName
3. 导入团队包：
   - 新建角色场景：验证 BaseAgentID 正确设置（包中值→默认→空）
   - 覆盖已存在角色场景：验证本地 BaseAgentID 被保留（当包中为空）