# 资产包轻量化与团队包设计

## 背景

当前资产包设计引入了版本概念和元数据表，增加了管理负担。需要简化为纯粹的导入导出工具，同时新增团队包功能，支持工作流及角色的批量导入导出。

## 目标

1. 资产包轻量化：移除版本概念，删除元数据表，只保留导入导出功能
2. 新增团队包：支持工作流 + 角色 + 关联资产的批量导入导出
3. 调整菜单结构：新增"管理工具"二级菜单，团队包和资产包作为三级菜单

## 数据结构设计

### 资产包 manifest.json

```json
{
  "exportedAt": "2026-04-01T16:30:00Z",
  "assets": {
    "skills": [{ "name": "code-review", "file": "skills/code-review.md" }],
    "commands": [{ "name": "commit", "file": "commands/commit.md", "boundSkills": ["git-ops"] }],
    "subagents": [{ "name": "explorer", "file": "agents/explorer.md", "boundSkills": ["search"] }],
    "rules": [{ "name": "coding-standard", "file": "rules/coding-standard.md" }],
    "settings": [{ "name": "default", "dir": "settings/default/" }]
  }
}
```

### 团队包 manifest.json

```json
{
  "exportedAt": "2026-04-01T16:30:00Z",
  "workflow": {
    "id": "uuid",
    "name": "standard-dev",
    "description": "标准开发流程",
    "agentIds": ["uuid1", "uuid2"],
    "transitions": [
      {
        "fromAgentId": "uuid1",
        "toAgentId": "uuid2",
        "type": "sequence",
        "triggerHint": "@developer 当需要实现时",
        "waitFor": []
      }
    ],
    "checkpoints": ["design-review", "code-review"],
    "estimatedTime": "2h",
    "isSystem": false,
    "isDefault": false
  },
  "roles": [
    {
      "id": "uuid",
      "name": "architect",
      "role": "architect",
      "description": "架构师",
      "systemPrompt": "...",
      "maxTokens": 4000,
      "temperature": 0.7,
      "mentionPatterns": ["@architect", "@架构师"],
      "bindings": {
        "skills": ["design-patterns"],
        "commands": ["review"],
        "subagents": ["planner"],
        "rules": ["architecture-rule"],
        "settings": ["arch-settings"]
      }
    }
  ],
  "assets": {
    "skills": [...],
    "commands": [...],
    "subagents": [...],
    "rules": [...],
    "settings": [...]
  }
}
```

### ZIP 文件结构

**资产包：**
```
asset-{timestamp}.zip
├── manifest.json
├── skills/
├── commands/
├── agents/
├── rules/
└── settings/
```

**团队包：**
```
team-{workflow-name}-{timestamp}.zip
├── manifest.json
├── roles/
│   ├── architect.json
│   └── developer.json
├── skills/
├── commands/
├── agents/
├── rules/
└── settings/
```

## 后端 API 设计

### 资产包 API

| API | 方法 | 说明 |
|-----|------|------|
| `/api/asset-packages/import` | POST | 上传 ZIP 文件导入资产 |
| `/api/asset-packages/export` | POST | 勾选资产 ID，返回 ZIP 文件 |

**Import 响应：**
```json
{
  "success": 5,
  "skipped": 2,
  "failed": 0,
  "details": [
    { "assetType": "skill", "name": "code-review", "status": "success" },
    { "assetType": "skill", "name": "git-ops", "status": "skipped", "message": "已存在同名资产" }
  ]
}
```

**Export 请求：**
```json
{
  "skillIds": ["uuid1", "uuid2"],
  "commandIds": ["uuid3"],
  "subagentIds": [],
  "ruleIds": ["uuid4"],
  "settingsIds": []
}
```

### 团队包 API

| API | 方法 | 说明 |
|-----|------|------|
| `/api/team-packages/import` | POST | 上传 ZIP 文件，返回预览信息 |
| `/api/team-packages/import/confirm` | POST | 确认导入，指定覆盖策略 |
| `/api/team-packages/export` | POST | 选择工作流 ID，返回 ZIP 文件 |

**Import 第一步（上传预览）：**
```json
{
  "workflow": { "name": "standard-dev", "exists": false },
  "roles": [
    { "name": "architect", "exists": true, "localId": "uuid-local" },
    { "name": "developer", "exists": false }
  ],
  "assets": {
    "skills": [
      { "name": "design-patterns", "exists": true },
      { "name": "code-review", "exists": false }
    ]
  }
}
```

**Import 第二步（确认导入）：**
```json
{
  "mode": "overwrite",
  "workflowAction": "overwrite",
  "roleActions": [
    { "name": "architect", "action": "skip" },
    { "name": "developer", "action": "overwrite" }
  ],
  "assetActions": [
    { "assetType": "skill", "name": "design-patterns", "action": "skip" }
  ]
}
```

## 前端页面设计

### 菜单结构

```
Agent团队
├── 团队管理（工作流）
├── 角色管理
├── 角色资产
│   ├── Commands
│   ├── Subagents
│   ├── Skills
│   ├── Rules
│   ├── Settings
│   └── Plugins
└── 管理工具 ← 新增
    ├── 团队包 ← 第一个
    └── 资产包 ← 第二个
```

### 页面路由

| 路由 | 页面 | 说明 |
|------|------|------|
| `/management-tools/team-package` | TeamPackagePage | 团队包导入导出 |
| `/management-tools/asset-package` | AssetPackagePage | 资产包导入导出 |

### 页面布局

两个页面均采用左右两卡片布局：
- 左侧：导入区域（拖拽上传 + 预览/结果展示）
- 右侧：导出区域（选择/勾选 + 导出按钮）

## 需要删除的内容

### 数据库表

| 表名 | 说明 |
|------|------|
| `asset_packages` | 资产包元数据表 |

### 后端文件

| 文件 | 操作 |
|------|------|
| `internal/model/asset_package.go` | 删除 |
| `internal/repo/asset_package.go` | 删除 |
| `internal/api/asset_package_handler.go` | 重写 |
| `internal/service/assetpackage/service.go` | 重写 |

### 迁移脚本

| 文件 | 说明 |
|------|------|
| `sql-change/migrations/202603310001_add_asset_package_tables.sql` | 删除 |

## 新增文件清单

### 后端新增

| 文件 | 说明 |
|------|------|
| `internal/model/team_package.go` | 团队包数据结构 |
| `internal/service/teampackage/service.go` | 团队包服务 |
| `internal/api/team_package_handler.go` | 团队包 API |

### 前端新增

| 文件 | 说明 |
|------|------|
| `web/src/pages/TeamPackagePage.tsx` | 团队包页面 |
| `web/src/pages/AssetPackagePage.tsx` | 重写资产包页面 |
| `web/src/api/teamPackage.ts` | 团队包 API |
| `web/src/api/assetPackage.ts` | 简化资产包 API |

### 数据库迁移

| 文件 | 说明 |
|------|------|
| `sql-change/migrations/202604010001_drop_asset_packages_table.sql` | 删除 asset_packages 表 |

## 验证方法

1. 资产包导出：选择多个技能、命令等，下载 ZIP，验证内容完整
2. 资产包导入：上传 ZIP，验证导入结果正确
3. 团队包导出：选择工作流，验证包含完整的工作流、角色、资产
4. 团队包导入：上传 ZIP，验证预览正确，覆盖/跳过策略生效