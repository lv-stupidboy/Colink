# 开发范式融入 ISDP 平台设计文档

**日期：** 2026-03-21
**版本：** 1.0
**状态：** 待审批

---

## 1. 概述

### 1.1 背景

团队基于 ClaudeCode 沉淀了一套开发范式，通过 command、subagent、skill、rule 等内容组合，覆盖需求分析设计、开发、审查、测试、CICD 等步骤。现需将这套范式融入 ISDP 平台，同时考虑 OpenCode 及后续其他编程智能体的兼容。

### 1.2 目标

- 将团队沉淀的开发范式内置为平台解决方案，开箱即用
- 支持用户自定义扩展 Agent 角色和技能
- 兼容多编程智能体（ClaudeCode、OpenCode 等）
- 建立联邦制技能库生态

### 1.3 核心概念对照

| 团队范式概念 | ISDP 对应概念 | 说明 |
|-------------|--------------|------|
| Command | 快捷动作 | CLI 下保留命令形式，平台转化为快捷操作按钮 |
| Subagent | AgentRoleConfig | 一个 Subagent 对应一个 AgentRole 配置 |
| Skill | Skill | 可复用的技能文件，通过平台技能库管理 |
| Rule | Skill (type=rule) | 作为 Skill 的一种类型管理 |
| Knowledge | KnowledgeBase | 知识库，通过 MCP 供 Agent 查询 |

---

## 2. 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                           ISDP 平台架构                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                      前端 (React)                             │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ │   │
│  │  │项目空间 │ │Agent角色│ │ 工作流  │ │ 技能库  │ │ 知识库  │ │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                ↓ REST                                │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                      后端服务 (Go)                            │   │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐ │   │
│  │  │ Agent 引擎 │ │ Skill 服务 │ │ Knowledge  │ │ Registry   │ │   │
│  │  │ (现有)     │ │ (新增)     │ │ Service    │ │ Service    │ │   │
│  │  │            │ │            │ │ (新增)     │ │ (新增)     │ │   │
│  │  └────────────┘ └────────────┘ └────────────┘ └────────────┘ │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                ↓                                     │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                      存储层                                    │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐                      │   │
│  │  │  MySQL   │ │  Redis   │ │ 本地文件  │                      │   │
│  │  │(元数据)  │ │ (预留)   │ │  系统    │                      │   │
│  │  │  ✓现有   │ │ 暂未启用  │ │  新增    │                      │   │
│  │  └──────────┘ └──────────┘ └──────────┘                      │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                ↓ CLI Spawn                           │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                      智能体层                                  │   │
│  │  ┌──────────────────┐    ┌──────────────────┐                │   │
│  │  │   ClaudeCode     │    │    OpenCode      │                │   │
│  │  │   .claude/       │    │   .opencode/     │                │   │
│  │  └──────────────────┘    └──────────────────┘                │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 3. 数据模型设计

### 3.1 Skill 表

```sql
CREATE TABLE skills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(255),
    description TEXT,
    type VARCHAR(50) DEFAULT 'skill',     -- skill | rule
    category VARCHAR(100),                 -- 开发规范、中间件、前端、后端...

    -- 来源信息
    source_type VARCHAR(50) NOT NULL,      -- built_in | uploaded | federated
    source_registry_id UUID,               -- 联邦来源 ID（federated 类型）
    author_id UUID,                        -- 创建者
    project_id UUID,                       -- 所属项目（uploaded 类型，可选）

    -- 安装信息
    install_source JSON,                   -- 不同智能体的安装地址
    -- 示例: {"claude_code": "https://...", "open_code": "https://..."}

    -- 兼容性
    supported_agents JSON,                 -- ["claude_code", "open_code"]

    -- 版本
    version VARCHAR(50) DEFAULT '1.0.0',

    -- 统计数据
    use_count INT DEFAULT 0,
    star_count INT DEFAULT 0,
    favorite_count INT DEFAULT 0,

    -- 状态
    status VARCHAR(50) DEFAULT 'active',   -- active | deprecated
    is_public BOOLEAN DEFAULT false,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### 3.2 AgentRole 与 Skill 关联表

```sql
CREATE TABLE agent_skill_bindings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_role_id UUID NOT NULL REFERENCES agent_configs(id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(agent_role_id, skill_id)
);
```

**设计说明：**
- 只保留关联表，去掉 `agent_configs` 中的冗余字段
- 绑定即意味着写入配置，不区分静态/动态
- 关联表是配置生成的依据

### 3.3 Skill 收藏记录

```sql
CREATE TABLE skill_favorites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(skill_id, user_id)
);
```

### 3.4 知识库表（简化版）

```sql
CREATE TABLE knowledge_bases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50),                      -- document | api_doc | best_practice | domain
    file_path VARCHAR(500),                -- 本地文件路径
    project_id UUID,                       -- 所属项目（可选，null 表示全局）
    is_public BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### 3.5 联邦技能源配置

```sql
CREATE TABLE skill_registries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(255),
    type VARCHAR(50) NOT NULL,             -- github | gitlab | api | custom
    url VARCHAR(500) NOT NULL,
    auth_config JSON,                      -- 认证配置（加密存储）
    sync_interval INT DEFAULT 3600,        -- 同步间隔（秒）
    last_sync_at TIMESTAMP,
    sync_status VARCHAR(50) DEFAULT 'pending', -- pending | success | failed
    skill_count INT DEFAULT 0,
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW()
);
```

---

## 4. AgentRole 与 Skill 关系

### 4.1 关系模型

```
┌─────────────────────────────────────────────────────────────────────┐
│                    AgentRole 配置结构                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  AgentRoleConfig                                                     │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  id: UUID                                                    │    │
│  │  name: "Developer"                                           │    │
│  │  system_prompt: "你是开发工程师..."                           │    │
│  │  base_agent_id: UUID (关联 ClaudeCode/OpenCode)              │    │
│  │                                                              │    │
│  │  关联关系（通过 agent_skill_bindings 表）                     │    │
│  │  bound_skills: [                                             │    │
│  │    { skill_id: "java-coding-standards" },                    │    │
│  │    { skill_id: "tdd-workflow" },                             │    │
│  │    { skill_id: "react-best-practices" },                     │    │
│  │  ]                                                           │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 4.2 关联表用途

| 用途 | 说明 |
|------|------|
| 可视化展示 | Agent 详情页显示关联的 skill 列表 |
| 配置生成 | 启动 Agent 时，下载关联的 Skill 到项目 |
| 反向查询 | 查某个 skill 被哪些 Agent 使用 |
| 统计推荐 | 基于「同类 Agent 常用 skill」推荐 |

### 4.3 AI 辅助提取

提供「智能提取 Skill」功能：

1. 用户输入 system_prompt
2. AI 分析提示词，识别 skill 引用
3. 展示提取结果，用户确认/修正/补充
4. 保存到关联表

### 4.4 内置范式示例

```
系统内置 AgentRole:

Designer (/design)
├── design-workflow
└── requirement-doc-template

Developer (/dev)
├── code-rules (rule)
├── tdd-workflow
├── [动态感知] java-coding-standards
├── [动态感知] react-best-practices
└── [动态感知] mysql-iac-skill

Reviewer (/review)
├── code-review-rules (rule)
└── security-check

Tester (/test)
├── test-workflow
└── e2e-test-guide
```

---

## 5. 配置生成机制

### 5.1 触发方式

- **项目级手动触发**：项目设置页 → "同步配置" 按钮
- **幂等性处理**：重复执行时提示覆盖/合并/跳过

### 5.2 生成流程

```
┌─────────────────────────────────────────────────────────────────────┐
│                    配置生成流程                                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  输入:                                                               │
│  - project_id (项目 ID)                                              │
│  - base_agent_type (claude_code | open_code)                        │
│                                                                      │
│  步骤:                                                               │
│  1. 获取项目关联的所有 AgentRole                                     │
│  2. 获取每个 AgentRole 关联的 Skill                                  │
│  3. 根据 base_agent_type 下载对应格式的 Skill 文件                   │
│  4. 生成到项目的 .claude/ 或 .opencode/ 目录                         │
│                                                                      │
│  输出目录结构:                                                        │
│  .claude/                                                           │
│  ├── skills/                                                        │
│  │   ├── java-coding-standards.md                                   │
│  │   └── tdd-workflow.md                                            │
│  └── rules/                                                         │
│      └── code-rules.md                                              │
│                                                                      │
│  或                                                                  │
│                                                                      │
│  .opencode/                                                         │
│  ├── tool/                                                          │
│  │   └── xxx.ts                                                     │
│  └── agent/                                                         │
│      └── developer.md                                               │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 5.3 Skill 安装方式

平台代理下载 Skill 文件到项目目录：

1. 查询 Skill 的 `install_source` 字段
2. 根据目标智能体类型获取对应下载地址
3. 平台代理下载文件到项目的配置目录

---

## 6. 技能库管理

### 6.1 来源类型

| 类型 | 说明 | 存储方式 |
|------|------|----------|
| **built_in** | 平台内置，系统维护 | install_source 指向官方地址 |
| **uploaded** | 用户上传 | install_source 存储平台托管地址 |
| **federated** | 联邦同步 | install_source 指向原始来源地址 |

### 6.2 激励机制

- **使用次数**：统计 Skill 被下载/使用的次数
- **收藏数**：用户收藏计数
- **点赞数**：用户点赞计数

### 6.3 前端页面

```
技能库页面:

┌─────────────────────────────────────────────────────────────────────┐
│  筛选栏: [类型] [分类] [来源] [兼容智能体] [搜索]                    │
├─────────────────────────────────────────────────────────────────────┤
│  Skill 卡片列表                                                      │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │ ☕ Java 编码规范                        ⭐ 128  ❤️ 56         │  │
│  │ 遵循阿里巴巴 Java 开发手册...                                  │  │
│  │ [ClaudeCode] [OpenCode]  使用次数: 1.2k                        │  │
│  │                                          [查看] [使用]         │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  来源标签页: [全部] [内置] [社区] [企业内部] [我的]                  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 7. 联邦制技能源

### 7.1 架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                    联邦制技能源                                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ISDP 平台                                                           │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  Skill 注册表 (skills 表)                                    │    │
│  │  - 统一索引、搜索、元数据                                     │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                          ↑ 同步                                      │
│         ┌────────────────┼────────────────┐                         │
│         │                │                │                         │
│  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐                 │
│  │ 内置 Skill  │  │ 用户上传    │  │ 外部来源    │                 │
│  │   源       │  │   源       │  │   源       │                 │
│  └─────────────┘  └─────────────┘  └─────────────┘                 │
│                                           ↑                         │
│                          ┌───────────────┤                         │
│                          │               │                         │
│                   ┌──────┴──────┐ ┌──────┴──────┐                  │
│                   │ GitHub 仓库 │ │ 企业内部    │                  │
│                   │ GitLab      │ │ 其他平台    │                  │
│                   └─────────────┘ └─────────────┘                  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 7.2 支持的来源类型

| 类型 | 说明 | 示例 |
|------|------|------|
| **github** | GitHub 仓库 | github.com/org/awesome-skills |
| **gitlab** | GitLab/Gitee 仓库 | gitlab.company.com/skills |
| **api** | 遵循统一 API 规范 | internal-registry.company.com |
| **custom** | 自定义来源 | 后续扩展 |

### 7.3 同步流程

1. 管理员配置技能源（URL、认证信息）
2. 手动或定时触发同步
3. 拉取 Skill 列表和元数据
4. 存入 skills 表（source_type = 'federated'）
5. 用户在技能库中浏览、使用

---

## 8. 知识库（简化版）

### 8.1 存储方式

- **元数据**：存入 `knowledge_bases` 表
- **文件**：存入本地文件系统 `/data/knowledge/`

### 8.2 使用方式

- Agent 通过 MCP 查询知识库
- 当前简化版：关键词匹配
- 后续扩展：向量检索、语义搜索

---

## 9. 智能体兼容性

### 9.1 格式对照

| 概念 | ClaudeCode | OpenCode |
|------|------------|----------|
| 配置目录 | `.claude/` | `.opencode/` |
| Skill 文件 | `.claude/skills/*.md` | `.opencode/tool/*.ts` 或 `*.md` |
| Agent 文件 | `.claude/agents/*.md` | `.opencode/agent/*.md` |
| 命令文件 | `.claude/commands/*.md` | `.opencode/command/*.md` |
| 主配置 | `CLAUDE.md` | `opencode.jsonc` |

### 9.2 兼容策略

- Skill 存储时提供不同智能体的文件格式
- 配置生成时根据目标智能体选择对应格式
- 格式转化功能预留接口，后续实现

---

## 10. 实施计划

### 10.1 第一阶段：基础能力

- [ ] 创建 skills、agent_skill_bindings、skill_favorites 表
- [ ] 实现 Skill CRUD API
- [ ] 实现 Agent 与 Skill 关联管理
- [ ] 实现技能库前端页面

### 10.2 第二阶段：配置生成

- [ ] 实现 Skill 下载服务
- [ ] 实现项目配置生成
- [ ] 实现 AI 辅助提取 Skill

### 10.3 第三阶段：联邦生态

- [ ] 创建 skill_registries 表
- [ ] 实现技能源管理 API
- [ ] 实现 GitHub 来源同步
- [ ] 实现技能源管理前端

### 10.4 第四阶段：知识库

- [ ] 创建 knowledge_bases 表
- [ ] 实现知识库 CRUD
- [ ] 实现 MCP 查询接口

---

## 11. 附录

### 11.1 确认的设计决策

| 决策点 | 结论 |
|--------|------|
| 整体方案 | 混合模式：内置范式平台托管 + 用户自定义本地优先 |
| Command 定位 | CLI 保留命令形式，平台转化为快捷动作按钮 |
| Skill 存储 | 不存文件内容，只存元数据和安装地址 |
| Agent 配置文件 | 暂不生成，后续探索 |
| Agent 与 Skill 关联 | 只保留关联表，去掉字段冗余 |
| 绑定类型 | 不区分静态/动态，绑定即写入配置 |
| 激励机制 | 使用次数、收藏数、点赞数 |

### 11.2 待确认事项

- [ ] ClaudeCode/OpenCode 的命令安装方式确认
- [ ] 格式转化工具的具体实现方案

---

*文档结束*