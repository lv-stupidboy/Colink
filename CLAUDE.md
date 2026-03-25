# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

ISDP (Intelligent Software Development Platform) - 智能软件开发平台，通过多 Agent 协同开发系统实现从想法到产品的快速落地。

## 常用命令

### 后端 (Go)
```bash
make build        # 构建到 bin/isdp
make run          # 运行服务 (go run ./cmd/server)
make test         # 运行测试 (覆盖率报告)
go test ./internal/service/agent -v  # 运行单个包测试
```

### 前端 (React + Vite)
```bash
cd web && npm run dev       # 启动开发服务器
cd web && npm run build     # 构建生产版本
cd web && npm run lint      # ESLint 检查
cd web && npm run test      # 运行测试运行器
cd web && npm run test:e2e  # Playwright E2E 测试
```

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