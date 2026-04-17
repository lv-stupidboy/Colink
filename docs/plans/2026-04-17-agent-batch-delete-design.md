# Agent 批量删除功能设计

## 背景

用户在管理 Agent 角色时，需要批量删除多个自定义角色。当前只能逐个删除，效率较低。

## 需求

1. **前端**：在 AgentRoleList 页面勾选多个自定义角色，一键批量删除
2. **后端**：提供批量删除 API，一次性处理多个 Agent
3. **限制**：
   - 仅自定义角色可批量删除（系统预置角色不可删除）
   - 任一 Agent 被工作流引用则拒绝整个操作

## 设计方案

### 后端 API

**接口**：`POST /api/v1/agents/batch-delete`

**请求体**：
```json
{
  "ids": ["uuid1", "uuid2"]
}
```

**响应**：
- 成功：`204 No Content`
- 失败：`400 Bad Request`
```json
{
  "error": "部分Agent被工作流引用，无法删除",
  "referencedAgents": [
    {"id": "uuid1", "name": "开发者", "workflowNames": ["团队A"]}
  ]
}
```

**核心逻辑**：
1. 验证所有 ID 都是自定义角色
2. 批量检查工作流引用
3. 任一有引用 → 返回失败详情
4. 全部无引用 → 删除 Agent 及所有绑定关系（skill、subagent、command、rule、settings）

### 前端交互

1. **表格勾选**：自定义角色表格添加 `rowSelection`
2. **批量删除按钮**：表格上方显示，仅在勾选后启用
3. **确认弹窗**：
   - 显示勾选的 Agent 名称列表
   - 调用批量删除 API
   - 若失败显示哪些 Agent 被引用
   - 成功后刷新列表

## 关键文件

**后端**：
- `internal/api/agent_handler.go` - 新增 `BatchDelete` 方法
- `internal/repo/agent_config.go` - 新增 `DeleteBatch` 方法
- `internal/service/agent/config_service.go` - 新增 `DeleteBatch` 方法

**前端**：
- `web/src/pages/AgentRoleList.tsx` - 添加勾选和批量删除 UI
- `web/src/api/client.ts` - 新增 `batchDelete` API 调用

## 验证方式

1. 启动前后端服务
2. 创建多个自定义 Agent 角色
3. 勾选多个角色，点击批量删除
4. 验证：无引用时成功删除；有引用时显示错误信息