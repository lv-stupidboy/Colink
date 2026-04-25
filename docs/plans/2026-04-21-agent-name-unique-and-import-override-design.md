# Agent角色名称唯一性校验与导入覆盖设计

## 问题描述

用户报告了两个问题：
1. **新建角色时出现同名角色** - 当前系统没有名称唯一性校验，导致可以创建同名角色
2. **导入角色时新增而非覆盖** - 导入团队包时，如果存在同名角色，当前逻辑是按ID判断是否存在，导致新增而非覆盖

## 问题根因分析

### 1. 新建角色无校验
- `internal/service/agent/config_service.go` 的 `Create` 方法直接创建，未检查名称唯一性
- `internal/repo/agent_config.go` 缺少 `FindByName` 方法

### 2. 导入角色按ID判断
- `internal/service/teampackage/service.go` 的 `importRole` 方法按 ID 检查是否存在
- 导入时即使有同名角色，因为 ID 不同，仍会新增

## 设计方案

### 方案一：全局名称唯一 + 导入覆盖（推荐）

**新建角色时：**
- 全局校验名称唯一性（系统角色和自定义角色都不能同名）
- 创建前检查是否存在同名角色，若存在则拒绝创建
- 更新时同样校验名称是否与其他角色冲突

**导入角色时：**
- 按**名称**判断是否存在
- 存在同名角色时，**覆盖**该角色（更新属性，保留ID）
- 不存在时，新建角色

**Trade-off：**
- ✅ 简单直观，易于理解和维护
- ✅ 导入行为符合用户期望（覆盖而非新增）
- ❌ 系统角色名称被占用时需特殊处理（导入的系统角色设为自定义）

### 方案二：按角色类型区分 + 导入覆盖

**新建角色时：**
- 同一角色类型（如 agent、workflow_agent）内名称唯一
- 不同类型可以有同名角色

**导入角色时：**
- 按名称 + 类型判断是否存在
- 存在时覆盖，不存在时新建

**Trade-off：**
- ✅ 更灵活，支持不同类型同名
- ❌ 实现复杂，用户体验可能困惑
- ❌ 导入逻辑复杂，需同时匹配名称和类型

## 推荐方案：方案一

采用全局名称唯一 + 导入覆盖方案，符合用户直觉，实现简单。

## 实施步骤

### 步骤 1：添加 FindByName 方法
- 文件：`internal/repo/agent_config.go`
- 新增 `FindByName(ctx, name) (*model.AgentRoleConfig, error)` 方法

### 步骤 2：新建角色添加名称校验
- 文件：`internal/service/agent/config_service.go`
- `Create` 方法开始时检查同名角色是否存在
- 存在则返回错误 `ErrAgentNameExists`
- `Update` 方法同样校验名称是否与其他角色冲突

### 步骤 3：导入角色改为按名称覆盖
- 文件：`internal/service/teampackage/service.go`
- `importRole` 方法改为按名称检查是否存在
- 存在同名角色时，更新该角色（保留原ID）
- 清理旧绑定关系，重新绑定新关系

### 步骤 4：前端表单添加实时校验
- 文件：`web/src/pages/AgentRoleList.tsx`
- 名称输入时校验是否已存在同名角色
- 显示错误提示

### 步骤 5：清理已存在的重复角色
- 编写 SQL 查询找出重复名称的角色
- 保留最早创建的角色，删除后续重复的
- 执行清理脚本

## 影响范围

| 组件 | 改动点 | 影响程度 |
|------|--------|----------|
| repo/agent_config.go | 新增 FindByName | 低 |
| service/agent/config_service.go | Create/Update 校验 | 中 |
| service/teampackage/service.go | importRole 逻辑 | 中 |
| api/agent_handler.go | 返回错误信息 | 低 |
| web/src/pages/AgentRoleList.tsx | 表单校验 | 低 |

## 数据库变更

无需数据库结构变更，仅需执行数据清理脚本。

## 验证要点

1. 新建角色：输入已存在的名称，应返回错误
2. 更新角色：修改名称为其他角色名称，应返回错误
3. 导入团队包：同名角色应被覆盖而非新增
4. 复制角色：复制后名称应为"原名(副本)"，若已存在应自动递增

## 下一步

设计文档已完成，触发计划审查场景。