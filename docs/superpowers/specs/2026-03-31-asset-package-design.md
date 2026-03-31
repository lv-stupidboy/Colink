# 资产包系统设计

## 概述

资产包系统用于支持多种资产的批量导入导出，以及资产版本管理能力。资产包作为传输和版本管理的载体，导入后资产散开管理，AgentRole 仍绑定单个资产。

## 核心概念

### 资产包

一组资产的集合，用于批量导入/导出和版本管理。

### 资产类型（5种）

| 类型 | 说明 | 存储方式 |
|------|------|----------|
| Skill | 技能 | 文件系统（目录形式）+ 数据库 |
| Command | 命令 | 文件系统 + 数据库 |
| Subagent | 子代理 | 文件系统 + 数据库（移除content字段） |
| Rule | 规则 | 文件系统 + 数据库 |
| Settings | 配置目录 | 文件系统（目录形式），数据库存元数据 |

### 移除的资产类型

- Hooks：统一由 Settings 管理
- Plugins：移除

## 版本管理

### 版本格式

**资产包版本**：`v{主版本}.{次版本}.{修订号}-{YYYYMMDD}-{HHMMSS}`

示例：
- `v1.0.0-20240331-143052`
- `v1.1.0-20240415-102030`
- `v2.0.0-20240501-093015`

### 资产唯一性

`资产名称 + 语义化版本`（如 `代码审查 v1.0.0`）

### 版本规则

- 导出时必须指定版本号（语义化版本）
- 系统自动追加时间戳
- 导入时检测 `名称+版本` 冲突，冲突则跳过
- 同一资产可以有多个版本共存
- 资产包内所有资产共享同一版本号

## 数据模型

### 新增表

```sql
-- 资产包
CREATE TABLE asset_packages (
  id UUID PRIMARY KEY,
  name VARCHAR(255) NOT NULL,           -- 用户命名
  version VARCHAR(50) NOT NULL,         -- v1.0.0-20240331-143052
  description TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Settings 资产
CREATE TABLE settings (
  id UUID PRIMARY KEY,
  name VARCHAR(255) NOT NULL,           -- 用户命名的目录名
  description TEXT,
  directory_path VARCHAR(500),          -- 存储路径
  version VARCHAR(20),                  -- 语义化版本，如 "1.0.0"
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- AgentRole 与 Settings 绑定
CREATE TABLE agent_settings_bindings (
  id UUID PRIMARY KEY,
  agent_role_id UUID NOT NULL REFERENCES agent_role_configs(id),
  settings_id UUID NOT NULL REFERENCES settings(id),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 现有表修改

```sql
-- 为 commands 添加版本字段
ALTER TABLE commands ADD COLUMN version VARCHAR(20);

-- 为 subagents 添加版本字段，移除 content 字段
ALTER TABLE subagents ADD COLUMN version VARCHAR(20);
ALTER TABLE subagents DROP COLUMN content;

-- 为 rules 添加版本字段
ALTER TABLE rules ADD COLUMN version VARCHAR(20);
```

## 资产包文件结构

### ZIP 包结构

```
资产包名称_v1.0.0-20240331-143052.zip
├── manifest.json           # 元数据
├── skills/
│   ├── brainstorming/      # skill目录
│   │   ├── SKILL.md
│   │   └── spec-document-reviewer-prompt.md
│   └── writing-plans/      # 另一个skill目录
│       └── SKILL.md
├── commands/
│   ├── review.md
│   └── test.md
├── subagents/
│   ├── 审查员.md
│   └── 测试员.md
├── rules/
│   └── 提交规范.md
└── settings/
    └── 团队配置/
        ├── settings.json
        └── CLAUDE.md
```

### manifest.json 结构

```json
{
  "name": "团队开发规范包",
  "version": "v1.0.0-20240331-143052",
  "exportedAt": "2024-03-31T14:30:52Z",
  "description": "包含代码审查、测试等规范",
  "assets": {
    "skills": [
      {
        "name": "brainstorming",
        "version": "1.0.0"
      }
    ],
    "commands": [
      {
        "name": "review",
        "version": "1.0.0",
        "boundSkills": ["技能名称"]
      }
    ],
    "subagents": [
      {
        "name": "审查员",
        "version": "1.0.0",
        "boundSkills": ["技能名称"]
      }
    ],
    "rules": [
      {
        "name": "提交规范",
        "version": "1.0.0"
      }
    ],
    "settings": [
      {
        "name": "团队配置"
      }
    ]
  }
}
```

## 功能设计

### 资产包管理页面

| 功能 | 说明 |
|------|------|
| 列表 | 显示资产包名称、版本、描述、资产数量、创建时间 |
| 导入 | 上传ZIP，解析manifest.json，导入资产 |
| 导出 | 选择资产 → 指定名称和版本号 → 导出ZIP |
| 详情 | 查看包内资产清单 |
| 删除 | 删除资产包记录（不影响已导入的资产） |

### 导入流程

1. 用户上传 ZIP 文件
2. 解压并解析 manifest.json
3. 校验资产包格式
4. 遍历每种资产类型：
   - 检查 `名称+版本` 是否已存在
   - 存在则跳过，不存在则导入
   - 恢复资产间的绑定关系（Command-Skill、Subagent-Skill）
5. 显示导入结果：成功数、跳过数、失败详情

### 导出流程

1. 展示资产选择界面（按类型分组）
2. 用户多选资产
3. 用户输入资产包名称、语义化版本号、描述
4. 系统自动追加时间戳生成完整版本号
5. 打包资产文件和 manifest.json
6. 生成 ZIP 供下载

## API 设计

### 资产包 API

```
GET    /api/asset-packages              # 资产包列表
GET    /api/asset-packages/:id          # 资产包详情
POST   /api/asset-packages/import       # 导入资产包
POST   /api/asset-packages/export       # 导出资产包（返回ZIP）
DELETE /api/asset-packages/:id          # 删除资产包
```

### Settings API

```
GET    /api/settings                    # Settings列表
GET    /api/settings/:id                # Settings详情
POST   /api/settings                    # 上传Settings（目录）
PUT    /api/settings/:id                # 更新Settings元数据
DELETE /api/settings/:id                # 删除Settings
POST   /api/agent-roles/:id/settings    # 绑定Settings到AgentRole
DELETE /api/agent-roles/:id/settings/:settingsId  # 解绑
```

## 配置生成集成

### 当前流程

AgentRole 绑定资产 → 生成配置到 `{dataDir}/{agentID}/`

### 新增内容

1. Settings 资产参与配置生成
2. 生成时将 Settings 目录内容拷贝到配置目录
3. 资产版本信息可选记录到生成的配置中（用于追溯）

### 生成目录结构

```
{dataDir}/{agentID}/
├── skills/
├── commands/
├── agents/
├── rules/
├── settings/              # 新增
│   └── 团队配置/
│       ├── settings.json
│       └── CLAUDE.md
├── CLAUDE.md
└── settings.json
```

## 迁移计划

### 数据迁移

1. 为 commands、subagents、rules 表添加 version 字段
2. 移除 subagents 表的 content 字段
3. 将现有 subagents content 写入文件系统
4. 现有资产版本默认设为 "1.0.0"

### 代码修改

1. 新增 Settings 模型和相关 CRUD
2. 新增 AssetPackage 模型和相关 CRUD
3. 修改 Subagent 存储逻辑（文件系统）
4. 新增导入导出服务
5. 新增前端资产包管理页面
6. 修改配置生成服务支持 Settings

## 文件存储结构

```
data/agent-assets/
├── skills/
│   └── {skill-name}/
│       └── SKILL.md
├── commands/
│   └── {command-name}.md
├── subagents/
│   └── {subagent-name}.md      # 改为文件存储
├── rules/
│   └── {rule-name}.md
└── settings/
    └── {settings-name}/         # 目录形式
        └── ...
```