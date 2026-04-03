# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

ISDP (Intelligent Software Development Platform) - 智能软件开发平台，通过多 Agent 协同开发系统实现从想法到产品的快速落地。

## 常用命令

### 完整构建（发布新版本）

```bash
cd installer && ./build.ps1    # Windows
cd installer && ./build.sh     # Unix/Linux/macOS
```

构建流程：清理旧产物 → 构建后端 → 构建前端 → 打包安装器 → 创建发布包

### 后端 (Go)

```bash
cd isdp && make build          # 构建到 bin/isdp-server.exe
cd isdp && make run            # 运行服务 (go run ./cmd/server)
cd isdp && go run ./cmd/server # 开发调试
cd isdp && make test           # 运行测试 (覆盖率报告)
```

### 前端 (React + Vite)

```bash
cd isdp/web && npm run dev     # 启动开发服务器
cd isdp/web && npm run build   # 构建生产版本
cd isdp/web && npm run lint    # ESLint 检查
```

### 版本号管理

版本号存储在 `isdp/VERSION` 文件中：
- 基础版本号：如 `0.3.0`
- 构建时自动追加时间戳：如 `v0.3.0-20260327-234317`
- 发布新版本时只需更新 VERSION 文件

## 架构

```
┌─────────────────────────────────────────────┐
│  Web前端 (React + Ant Design + Zustand)     │
└─────────────────────────────────────────────┘
                    ↓ REST/WebSocket
┌─────────────────────────────────────────────┐
│  后端服务 (Go + Gin)                         │
│  internal/                                   │
│    api/       → HTTP 处理器                  │
│    service/   → 业务逻辑层                   │
│      agent/   → Agent 引擎核心               │
│      sandbox/ → Docker 沙箱管理              │
│    repo/      → 数据访问层                   │
│    model/     → 数据模型                     │
└─────────────────────────────────────────────┘
                    ↓ CLI Spawn
┌─────────────────────────────────────────────┐
│  Agent 实例 (Claude CLI / OpenCode 适配器)  │
└─────────────────────────────────────────────┘
```

## 关键模块

- **Agent 引擎** (`internal/service/agent/`): Orchestrator 编排器、ExecutionService 执行服务、ContextBuilder 上下文构建器
- **A2A 路由** (`internal/service/a2a/`): `@Agent名` 触发协作，MCP 工具回传
- **沙箱服务** (`internal/service/sandbox/`): Docker 容器管理，端口自动分配 (30000-40000)
- **工作流引擎** (`internal/service/workflow/`): 阶段流转 (需求→设计→实现→审查→测试→部署)

## 配置

### 配置文件管理

项目采用**配置模板 + 本地配置**的协作方式：

| 文件 | 用途 | Git 管理 |
|------|------|----------|
| `configs/config.yaml.example` | 配置模板，记录所有可用配置项和默认值 | ✅ 提交到仓库 |
| `configs/config.yaml` | 本地真实配置，包含敏感信息 | ❌ 不提交（已在 .gitignore） |

### 新增配置项流程

1. **更新配置模板** `config.yaml.example`
   - 添加新配置项及注释说明
   - 使用占位符代替敏感信息（如 `<密码>`）

2. **更新本地配置** `config.yaml`
   - 同步添加新配置项
   - 填入真实值

3. **更新配置结构体** `pkg/config/config.go`
   - 在 `Config` 结构体中添加对应字段
   - 添加默认值（`setDefaults` 函数）

### 示例：添加技能配置

```yaml
# config.yaml.example（模板）
skill:
  # 技能使用次数统计更新间隔，默认 1h
  use_count_update_interval: "1h"

# config.yaml（本地真实配置）
skill:
  use_count_update_interval: "1h"
```

### 重要规则

- **敏感信息绝不提交**：数据库密码、API密钥等
- **模板与本地同步**：新增配置项时，两个文件都要更新
- **默认值在代码中设置**：确保未配置时有合理默认值

## API 命名规范

### JSON 字段命名

**统一使用 camelCase**，与 JavaScript/TypeScript 生态保持一致。

### 后端模型 (Go)

在 `internal/model/` 下的结构体中，JSON 标签必须使用 camelCase：

```go
// ✅ 正确
type AgentConfig struct {
    ID           uuid.UUID `json:"id"`
    Name         string    `json:"name"`
    BaseAgentID  uuid.UUID `json:"baseAgentId"`
    SystemPrompt string    `json:"systemPrompt"`
    MaxTokens    int       `json:"maxTokens"`
    IsSystem     bool      `json:"isSystem"`
    CreatedAt    time.Time `json:"createdAt"`
}

// ❌ 错误
type AgentConfig struct {
    BaseAgentID  uuid.UUID `json:"base_agent_id"`  // 不要用 snake_case
    SystemPrompt string    `json:"system_prompt"`
    MaxTokens    int       `json:"max_tokens"`
    IsSystem     bool      `json:"is_system"`
    CreatedAt    time.Time `json:"created_at"`
}
```

### 前端类型 (TypeScript)

在 `web/src/types/` 下的接口定义使用 camelCase：

```typescript
// ✅ 正确
interface AgentConfig {
  id: string;
  baseAgentId?: string;
  systemPrompt: string;
  maxTokens: number;
  isSystem: boolean;
  createdAt: string;
}
```

### 常见字段映射

| 数据库字段 (snake_case) | JSON/API 字段 (camelCase) |
|------------------------|--------------------------|
| `agent_id` | `agentId` |
| `is_system` | `isSystem` |
| `is_default` | `isDefault` |
| `created_at` | `createdAt` |
| `updated_at` | `updatedAt` |
| `max_tokens` | `maxTokens` |
| `system_prompt` | `systemPrompt` |
| `base_agent_id` | `baseAgentId` |
| `config_path` | `configPath` |
| `estimated_time` | `estimatedTime` |

### 空值处理

- **空数组**：返回 `[]` 而非 `{}` 或 `null`
- **空对象**：可选字段返回 `null` 或省略
- **前端防御**：使用 `Array.isArray()` 检查数组类型

```typescript
// ✅ 正确：防御性检查
const transitions = Array.isArray(template.transitions) ? template.transitions : [];

// ❌ 错误：空对象 {} 会导致数组方法报错
const transitions = template.transitions || [];
```

## 术语规范

### 资产类型命名

在 UI 界面、文档、代码注释中，资产类型统一使用以下英文命名：

| 资产类型 | 英文名称 | 说明 |
|----------|----------|------|
| 技能 | **Skills** | 技能包，包含提示词模板和工作流 |
| 命令 | **Commands** | 斜杠命令，如 `/commit`、`/review` |
| 子代理 | **Subagents** | 子代理配置，可被主 Agent 调用 |
| 规则 | **Rules** | 规约文件，定义编码规范和约束 |
| 配置 | **Settings** | 配置目录，包含项目级配置文件 |

### 团队相关命名

| 概念 | 英文名称 | 说明 |
|------|----------|------|
| 团队 | **Team** | 一个工作流及其关联的角色和资产 |
| 角色 | **Role** | Agent 角色配置 |
| 团队包 | **Team Package** | 导出的团队配置包（ZIP 文件） |
| 资产包 | **Asset Package** | 导出的资产集合（ZIP 文件） |

**注意：** 这些术语在 UI 显示时使用英文，后端代码中仍使用原有命名（如 workflow、agentRoleConfig 等）。

## 服务端口

- **后端**: 8080
- **前端**: 3000

遇到端口冲突时，先停掉占用端口的进程再启动：

```bash
# Windows 查看端口占用
netstat -ano | findstr ":3000 :8080"

# 停掉进程
taskkill //F //PID <PID>

# 然后启动服务
cd isdp && go run ./cmd/server      # 后端
cd web && npm run dev               # 前端
```

## 数据库变更

### 目录结构

数据库变更脚本位于 `isdp/sql-change/` 目录：

```
isdp/sql-change/
├── init_db_mysql.sql      # 初始数据库快照（新建环境时执行一次，后续不再修改）
└── migrations/            # 增量变更脚本（所有变更都必须创建迁移文件）
    ├── 202603200001_add_thread_name.sql
    ├── 202603200002_add_workflow_transitions.sql
    └── ...
```

### 归档规则

1. **任何数据库结构变更必须创建迁移文件**
   - 新建表
   - 添加/删除/修改字段
   - 添加/删除索引
   - 添加/删除外键约束

2. **迁移文件命名规范**
   - 格式: `YYYYMMDDNN_description.sql`
   - YYYYMMDD: 日期（如 20260321）
   - NN: 当日序号（01, 02, 03...）
   - description: 简短描述（小写下划线分隔）
   - 示例: `202603210001_add_skill_tables.sql`

3. **迁移文件内容规范**
   ```sql
   -- 文件路径（注释说明）
   -- 变更说明：简要描述本次变更内容
   -- 作者：XXX
   -- 日期：YYYY-MM-DD

   SET NAMES utf8mb4;

   -- DDL 语句...

   -- 回滚语句（如需回滚执行以下语句）
   -- DROP TABLE IF EXISTS xxx;
   ```

4. **执行流程**
   - 新环境初始化: 先执行 `init_db_mysql.sql`，再按顺序执行所有 migrations
   - 已有环境更新: 按顺序执行新的 migrations 脚本
   - 执行命令（在 isdp 目录下执行）:
     ```bash
     mysqlsh --sql -h <host> -P 3306 -u <user> -p<password> -D <database> -f sql-change/migrations/xxx.sql
     ```

5. **init_db_mysql.sql 不再修改**
   - 该文件是初始版本快照，代表项目某个历史节点的完整状态
   - 所有后续变更都通过 migrations 增量实现

6. **MySQL 兼容性规范**
   - **不要使用 `DROP COLUMN IF EXISTS`**：此语法需要 MySQL 8.0.23+，阿里云 RDS MySQL 5.7 不支持
   - 删除列时使用普通语法：`ALTER TABLE xxx DROP COLUMN field_name`
   - `DROP TABLE IF EXISTS` 是允许的，MySQL 5.7+ 都支持

## 变更记录

### 文件位置

`docs/CHANGELOG.md` - 记录项目的开发变更历史

### 更新时机

每次完成一个完整功能模块或重要修复后，需要更新CHANGELOG：

1. **新功能开发完成** - 如新增技能库、知识库等模块
2. **重要Bug修复** - 涉及多个文件或架构层面的修复
3. **架构调整** - 重构、依赖升级等
4. **配置变更** - 新增配置项或修改配置结构

### 更新规范

1. **最新记录在最上方** - 新增条目添加到文件开头，紧跟标题之后
2. **格式统一** - 使用标准的章节结构：
   ```
   ## YYYY-MM-DD 功能名称

   ### 背景
   为什么需要这次变更

   ### 目标
   这次变更要达成什么

   ### 核心变更
   #### 后端改动
   #### 前端改动
   #### 数据库变更

   ### 新增/修改文件列表
   | 文件 | 改动类型 | 说明 |
   |------|----------|------|

   ### 验证方法
   如何验证变更正确

   ### 影响范围
   影响了哪些模块
   ```

3. **提交前更新** - 在 `git push` 之前更新CHANGELOG，与代码一起提交

### 示例

```markdown
## 2026-03-21 技能库完整功能实现

### 背景
项目需要实现完整的技能库管理功能...

### 目标
1. 实现技能数据的完整CRUD功能
2. 实现Agent与Skill的多对多绑定关系...

### 核心变更
#### 后端改动
- 新增 `internal/model/skill.go` - Skill模型
...
```

## 安装程序 (Installer)

### 目录结构

安装程序位于 `installer/` 目录：

```
installer/
├── src/
│   ├── main/
│   │   ├── index.ts           # ISDP Setup 入口（安装/升级/卸载）
│   │   ├── launcher-entry.ts  # ISDP Launcher 入口（服务管理）
│   │   ├── service-manager.ts # 服务进程管理
│   │   └── installer.ts       # 安装逻辑
│   ├── preload/               # Electron preload 脚本
│   └── renderer/              # React 前端界面
├── build/                     # 构建资源（图标等）
├── electron-builder.yml       # Setup 打包配置
├── electron-builder.launcher.yml  # Launcher 打包配置
├── build.ps1                  # Windows 完整构建脚本
└── build.sh                   # Unix 完整构建脚本
```

### 构建流程

**一键构建（推荐）：**

```bash
# Windows
cd installer
.\build.ps1

# Unix/Linux/macOS
cd installer
./build.sh
```

构建脚本会依次执行：
1. 构建 Go 后端 (`isdp/isdp-server.exe`)
2. 构建前端 (`isdp/web/dist`)
3. 构建 Electron 安装器代码
4. 打包 Launcher (`ISDP.exe`)
5. 打包 Setup (`ISDP Setup.exe`)
6. 创建发布 ZIP 包 (`release/ISDP-*.zip`)

**单独构建：**

```bash
cd installer
npm run build              # 只构建 Electron 代码
npm run package:launcher   # 只打包 Launcher
npm run package:setup      # 只打包 Setup
```

### Setup 与 Launcher 职责

| 程序 | 职责 | 启动时机 |
|------|------|----------|
| **ISDP Setup.exe** | 安装、升级、卸载 | 用户手动运行安装包 |
| **ISDP.exe (Launcher)** | 服务启停、日志查看、配置管理 | 用户通过桌面快捷方式启动 |

**Setup 功能：**
- 检测已安装版本，提供升级/卸载选项
- 选择安装目录
- 检测依赖（Node.js、Git、Claude CLI）
- 配置数据库连接
- 创建桌面快捷方式和开始菜单项

**Launcher 功能：**
- 启动/停止后端服务
- 打开控制台 (http://localhost:8080)
- 查看日志、配置、数据目录
- 服务状态监控

### 安装后目录结构

```
{安装目录}/
├── ISDP.exe              # Launcher 启动器
├── isdp-server.exe       # 后端服务
├── web/                  # 前端静态文件
│   ├── index.html
│   └── assets/
└── data/                 # 用户数据目录（升级时保留）
    ├── configs/
    │   └── config.yaml   # 配置文件
    ├── logs/             # 日志文件
    ├── agent-assets/     # Agent 资产
    ├── agent-configs/    # Agent 配置
    ├── repos/            # 代码仓库
    └── *.db              # SQLite 数据库（如使用）
```

### 服务启动命令

后端服务在安装目录运行，通过参数指定配置文件：

```bash
isdp-server.exe -config "{安装目录}/data/configs/config.yaml"
```

工作目录为安装目录，这样服务可以正确找到 `./web` 静态文件。

## Data 目录约定

### 目录结构

`data/` 目录存放所有用户数据和运行时数据，升级安装时保留：

```
data/
├── configs/
│   └── config.yaml       # 主配置文件
├── logs/
│   └── server.log        # 服务日志
├── agent-assets/         # Agent 资产文件（技能包、知识库等）
├── agent-configs/        # Agent 运行时配置
├── repos/                # 克隆的代码仓库
└── isdp.db               # SQLite 数据库（开发环境）
```

### 重要规则

1. **升级保留**：安装器升级时会保留整个 `data/` 目录
2. **配置不覆盖**：如果配置文件已存在，升级时不会覆盖
3. **相对路径**：配置文件中使用 `./data` 相对路径，基于安装目录
4. **敏感数据**：数据库密码等敏感信息存放在 `data/configs/config.yaml`，不提交到 Git

### 配置文件路径优先级

Go 后端按以下顺序查找配置文件：

1. 命令行参数：`-config <路径>`
2. 环境变量：`ISDP_CONFIG`
3. 安装路径：`data/configs/config.yaml`
4. 开发路径：`configs/config.yaml`

### 开发环境 vs 生产环境

| 环境 | 配置路径 | 数据目录 |
|------|----------|----------|
| 开发 | `isdp/configs/config.yaml` | `isdp/data/` |
| 生产 | `{安装目录}/data/configs/config.yaml` | `{安装目录}/data/` |