# Agent 资产管理设计文档

> 创建日期：2026-03-23
> 状态：设计阶段

## 一、背景

将团队基于 Claude Code 的实践范式沉淀到 ISDP 平台，包括 Command、Subagent、Skill、Rule、Hook 等资产的管理能力。将 Agent 角色升级为一级菜单，统一管理所有相关资产。

## 二、目标

1. 将 Agent 角色菜单改为一级菜单，下设多个二级菜单
2. 新增 Command（命令）、Rule（规约）资产管理能力
3. 建立清晰的资产绑定关系：Agent→Command/Subagent/Rule，Command/Subagent→Skill
4. 配置生成时支持复制所有相关资产文件

## 三、整体架构

### 3.1 资产层次关系

```
┌─────────────────────────────────────────────────────────┐
│  Agent 角色（编排层）                                     │
│  ├── 绑定 Command（命令）                                │
│  ├── 绑定 Subagent（子代理）                             │
│  ├── 绑定 Rule（规约）- 实例类型                         │
│  └── 绑定公共 Rule（默认包含，可取消）                    │
└─────────────────────────────────────────────────────────┘
           │                    │
           ▼                    ▼
┌──────────────────┐    ┌──────────────────┐
│  Command         │    │  Subagent        │
│  └── 绑定 Skill  │    │  └── 绑定 Skill  │
└──────────────────┘    └──────────────────┘
           │                    │
           └──────────┬─────────┘
                      ▼
              ┌──────────────┐
              │  Skill       │
              │  （知识层）   │
              └──────────────┘
```

### 3.2 核心设计原则

- **数据库只存元数据，内容存文件**
- 五种资产复用，通过绑定关系组合
- 配置生成时按需复制文件

### 3.3 文件存储结构

```
{data-dir}/
├── skills/
│   └── {skill-name}/
│       ├── SKILL.md        # 主文件（必须有）
│       └── ... (其他文件/目录)
├── commands/{name}.md
├── subagents/{name}.md
└── rules/{name}.md
```

## 四、数据模型设计

### 4.1 新增数据表

```sql
-- 命令表
CREATE TABLE commands (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 规约表
CREATE TABLE rules (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    scope VARCHAR(20) NOT NULL DEFAULT 'instance',  -- public / instance
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 4.2 绑定关系表

```sql
-- Agent-Command 绑定（新增）
CREATE TABLE agent_command_bindings (
    id VARCHAR(64) PRIMARY KEY,
    agent_role_id VARCHAR(64) NOT NULL,
    command_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, command_id)
);

-- Agent-Rule 绑定（新增）
CREATE TABLE agent_rule_bindings (
    id VARCHAR(64) PRIMARY KEY,
    agent_role_id VARCHAR(64) NOT NULL,
    rule_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, rule_id)
);

-- Command-Skill 绑定（新增）
CREATE TABLE command_skill_bindings (
    id VARCHAR(64) PRIMARY KEY,
    command_id VARCHAR(64) NOT NULL,
    skill_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(command_id, skill_id)
);

-- Subagent-Skill 绑定（新增）
CREATE TABLE subagent_skill_bindings (
    id VARCHAR(64) PRIMARY KEY,
    subagent_id VARCHAR(64) NOT NULL,
    skill_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(subagent_id, skill_id)
);
```

> **注意**：`agent_subagent_bindings` 表已存在，无需新建。`agent_skill_bindings` 表已存在，本次重构后可考虑移除（Agent 不再直接绑定 Skill）。

### 4.3 修改现有表

**`subagents` 表：移除 `skill_id` 字段**

迁移策略：

```sql
-- 1. 创建 subagent_skill_bindings 表（见 4.2 节）

-- 2. 迁移现有数据：将 subagents.skill_id 迁移到绑定表
INSERT INTO subagent_skill_bindings (id, subagent_id, skill_id, created_at)
SELECT
    UUID() as id,
    id as subagent_id,
    skill_id,
    NOW() as created_at
FROM subagents
WHERE skill_id IS NOT NULL AND skill_id != '';

-- 3. 验证迁移成功后，移除 skill_id 字段
ALTER TABLE subagents DROP COLUMN skill_id;
```

> **注意**：迁移脚本需要在执行前备份数据，执行后验证数据完整性。

## 五、前端菜单与路由设计

### 5.1 菜单结构

```
Agent 角色（一级菜单）
├── 角色管理
├── 命令集
├── 子代理
├── 技能库
├── 规约
├── 插件（占位，暂不实现）
├── 钩子（占位，暂不实现）
└── 设置（占位，暂不实现）
```

### 5.2 路由配置

| 菜单 | 路由 | 页面组件 |
|-----|------|---------|
| Agent 角色 → 角色管理 | `/agents/roles` | `AgentRoleList.tsx`（迁移） |
| Agent 角色 → 命令集 | `/agents/commands` | `CommandList.tsx`（新建） |
| Agent 角色 → 子代理 | `/agents/subagents` | `SubagentList.tsx`（迁移） |
| Agent 角色 → 技能库 | `/agents/skills` | `SkillLibrary.tsx`（迁移） |
| Agent 角色 → 规约 | `/agents/rules` | `RuleList.tsx`（新建） |
| Agent 角色 → 插件 | `/agents/plugins` | 占位页面 |
| Agent 角色 → 钩子 | `/agents/hooks` | 占位页面 |
| Agent 角色 → 设置 | `/agents/settings` | 占位页面 |

### 5.3 路由重定向

- `/agents` 重定向到 `/agents/roles`
- 旧路由兼容：`/skills` → `/agents/skills`，`/subagents` → `/agents/subagents`

## 六、页面功能设计

### 6.1 Agent 角色管理页面增强

在编辑弹窗中新增资产绑定配置：

```
编辑 Agent 角色
├── 基础信息
│   ├── 名称
│   ├── 基础 Agent
│   ├── 描述
│   └── 系统提示词
│
└── 资产绑定（新增）
    ├── 绑定命令（多选）
    ├── 绑定子代理（多选）
    └── 绑定规约（多选）
```

### 6.2 规约绑定的交互设计

**公共规约处理逻辑：**

1. **创建 Agent 时**：自动为所有 `scope='public'` 的规约创建绑定记录（存入 `agent_rule_bindings` 表）
2. **编辑 Agent 时**：
   - 公共规约默认显示为已选中，用户可取消
   - 实例规约（`scope='instance'`）默认未选中，需手动绑定
3. **配置生成时**：只需查询 `agent_rule_bindings` 表即可获取所有规约

**前端交互：**

```
绑定规约
┌─────────────────────────────────────────────────────────┐
│ [公共规约] - 标签标识                                      │
│ ☑ code-standards      代码规范（默认选中，可取消）         │
│ ☑ security-rules      安全规约（默认选中，可取消）         │
│                                                         │
│ [实例规约]                                                │
│ ☐ team-workflow       团队工作流（需手动绑定）            │
│ ☐ project-specific    项目特定规约                       │
└─────────────────────────────────────────────────────────┘
```

> **设计决策**：公共规约也存入绑定表，保持数据模型简单。生成配置时无需额外判断 scope，统一从绑定表查询。

### 6.3 命令集管理页面

```
命令集管理
├── 列表展示
│   ├── 名称
│   ├── 描述
│   ├── 关联技能数
│   └── 创建时间
│
├── 操作
│   ├── 新建（上传 .md 文件）
│   ├── 编辑（修改元数据 + 内容预览）
│   ├── 删除
│   └── 查看详情
│
└── 编辑弹窗
    ├── 名称（必填，唯一）
    ├── 描述
    ├── 绑定技能（多选）
    └── 内容预览（只读，来自文件）
```

### 6.4 规约管理页面

```
规约管理
├── 列表展示
│   ├── 名称
│   ├── 描述
│   ├── 范围（公共/实例）
│   └── 创建时间
│
├── 操作
│   ├── 新建（上传 .md 文件 + 选择范围）
│   ├── 编辑（修改元数据 + 内容预览）
│   ├── 删除
│   └── 查看详情
│
└── 编辑弹窗
    ├── 名称（必填，唯一）
    ├── 描述
    ├── 使用范围（公共/实例）
    └── 内容预览（只读，来自文件）
```

## 七、API 接口设计

### 7.1 新增 API

```
# Command API
GET    /api/v1/commands                    # 列表
GET    /api/v1/commands/:id                # 详情
POST   /api/v1/commands                    # 创建（元数据）
POST   /api/v1/commands/upload             # 上传 .md 文件
PUT    /api/v1/commands/:id                # 更新元数据
DELETE /api/v1/commands/:id                # 删除
GET    /api/v1/commands/:id/skills         # 获取绑定的技能
POST   /api/v1/commands/:id/skills         # 绑定技能
DELETE /api/v1/commands/:id/skills/:skill_id  # 解绑技能

# Rule API
GET    /api/v1/rules                       # 列表
GET    /api/v1/rules/:id                   # 详情
POST   /api/v1/rules                       # 创建（元数据）
POST   /api/v1/rules/upload                # 上传 .md 文件
PUT    /api/v1/rules/:id                   # 更新元数据
DELETE /api/v1/rules/:id                   # 删除

# Agent 扩展 API
GET    /api/v1/agents/:id/commands         # 获取绑定的命令
POST   /api/v1/agents/:id/commands         # 绑定命令
DELETE /api/v1/agents/:id/commands/:command_id  # 解绑命令

GET    /api/v1/agents/:id/rules            # 获取绑定的规约
POST   /api/v1/agents/:id/rules            # 绑定规约
DELETE /api/v1/agents/:id/rules/:rule_id   # 解绑规约
```

### 7.2 修改现有 API

```
# Subagent API 修改
GET    /api/v1/subagents/:id/skills        # 获取绑定的技能
POST   /api/v1/subagents/:id/skills        # 绑定技能
DELETE /api/v1/subagents/:id/skills/:skill_id  # 解绑技能
```

## 八、配置生成流程

### 8.1 生成逻辑

```
输入：Agent 角色 ID

步骤 1：创建目标目录
{config-dir}/.claude/
├── commands/
├── agents/
├── skills/
└── rules/

步骤 2：复制 Command 文件
→ 查询 agent_command_bindings
→ 复制 {data-dir}/commands/{name}.md

步骤 3：复制 Subagent 文件
→ 查询 agent_subagent_bindings
→ 复制 {data-dir}/subagents/{name}.md

步骤 4：复制 Skill 文件（去重）
→ 通过 command_skill_bindings 获取 Command 关联的 Skill
→ 通过 subagent_skill_bindings 获取 Subagent 关联的 Skill
→ 合并去重 skill_ids
→ 复制 {data-dir}/skills/{name}/ 目录

步骤 5：复制 Rule 文件
→ 查询 agent_rule_bindings（包含公共和实例）
→ 复制 {data-dir}/rules/{name}.md
```

### 8.2 输出结果

```
{
  "agentId": "xxx",
  "configPath": "/path/to/.claude",
  "commandsCount": 3,
  "subagentsCount": 2,
  "skillsCount": 5,
  "rulesCount": 4,
  "generatedAt": "2026-03-23T..."
}
```

## 九、命名规范

所有资产（Skill、Command、Subagent、Rule）统一命名规范：

- 只允许小写字母、数字、中划线
- 必须以字母开头
- 正则表达式：`^[a-z][a-z0-9-]*$`

## 十、文件变更清单

### 10.1 后端新增

```
internal/model/command.go
internal/model/rule.go
internal/repo/command.go
internal/repo/rule.go
internal/repo/agent_command_binding.go
internal/repo/agent_rule_binding.go
internal/repo/command_skill_binding.go
internal/repo/subagent_skill_binding.go
internal/service/command/service.go
internal/service/rule/service.go
internal/api/command_handler.go
internal/api/rule_handler.go
sql-change/migrations/202603230001_add_command_rule_tables.sql
```

### 10.2 后端修改

```
internal/model/subagent.go           # 移除 skill_id
internal/repo/subagent.go            # 移除 skill_id 相关
internal/service/configgen/service.go # 扩展配置生成逻辑
internal/api/subagent_handler.go     # 新增技能绑定 API
pkg/config/config.go                 # 新增存储路径配置
cmd/server/main.go                   # 注册新服务和 Handler
```

### 10.3 前端新增

```
src/pages/CommandList.tsx
src/pages/RuleList.tsx
src/pages/PlaceholderPage.tsx        # 占位页面
```

### 10.4 前端修改

```
src/layouts/MainLayout.tsx           # 菜单结构调整
src/App.tsx                          # 路由结构调整
src/pages/AgentRoleList.tsx          # 新增绑定 Command/Rule
src/pages/SubagentList.tsx           # 新增绑定 Skill
src/api/client.ts                    # 新增 API
src/types/index.ts                   # 新增类型
```

## 十一、暂不实现的功能

以下功能保留菜单占位，暂不实现：

| 功能 | 路由 | 说明 |
|-----|------|------|
| 钩子 | `/agents/hooks` | 后续版本实现 |
| 插件 | `/agents/plugins` | 后续版本实现 |
| 设置 | `/agents/settings` | 后续版本实现 |