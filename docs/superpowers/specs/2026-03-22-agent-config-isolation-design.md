# Agent配置隔离与Subagent融合设计文档

## 背景

项目已完成技能库管理功能，现需要将团队在Claude Code沉淀的Subagent范式融入平台，让用户可以直接应用这一套工作流。

当前存在的问题：
1. **配置生成粒度过大** - 现有配置生成按项目粒度，所有Agent角色共享同一份配置
2. **技能无法隔离** - 不同Agent角色绑定的技能混在一起，无法独立管理
3. **Subagent未纳入管理** - 团队沉淀的Subagent范式未融入平台

## 目标

1. 配置生成改为**Agent角色粒度**，每个Agent角色拥有独立的配置目录
2. 支持Skills和Subagents两种范式的管理
3. 启动CLI时按Agent角色拼接配置目录，实现配置隔离
4. 兼容Claude Code和OpenCode（通过oh-my-opencode）两种CLI工具

## 目录结构设计

### 全局配置池

```
{isdp-data-dir}/                    # ISDP数据目录（默认 ./data/，可配置）
  agents/                           # Agent角色配置池
    {agent-role-id}/                # 按Agent角色UUID隔离
      .claude/                      # 兼容Claude Code目录结构
        settings.json               # CLI配置（权限、模型、MCP服务器等）
        CLAUDE.md                   # 系统提示文件
        skills/                     # 该Agent绑定的技能
          代码审查.md
          单元测试.md
        agents/                     # 该Agent可调用的子代理
          test-runner.md
          code-reviewer.md
```

### 目录说明

| 目录/文件 | 用途 | 格式 |
|----------|------|------|
| `settings.json` | CLI配置：权限、模型、MCP服务器、环境变量等 | JSON |
| `CLAUDE.md` | Agent角色的系统提示，定义角色行为 | Markdown |
| `skills/*.md` | 技能文件：可复用的提示词模板 | Markdown |
| `agents/*.md` | 子代理文件：可委派任务的独立代理 | Markdown |

## 配置生成流程

### 触发时机

1. **编辑后提示** - Agent角色编辑保存后，提示用户"是否生成配置？"
2. **独立入口** - Agent角色列表页面提供"生成配置"按钮，可随时触发

### 生成流程

```
用户点击"生成配置"
    ↓
选择目标BaseAgent类型（Claude Code / OpenCode）
    ↓
获取Agent角色配置（名称、系统提示、绑定的技能、绑定的子代理）
    ↓
创建/更新配置目录
    ├── 生成 settings.json（权限、模型配置）
    ├── 生成 CLAUDE.md（系统提示）
    ├── 解压技能包到 skills/ 目录
    └── 复制子代理文件到 agents/ 目录
    ↓
返回生成结果（目录路径、技能数量、子代理数量）
```

## CLI启动方式

### Claude Code

```bash
CLAUDE_CONFIG_DIR={isdp-data-dir}/agents/{agent-role-id}/.claude claude [options]
```

关键参数说明：
- `CLAUDE_CONFIG_DIR` - 指定配置目录，替代默认的 `~/.claude/`
- Claude Code 会自动读取该目录下的 `settings.json` 和 `CLAUDE.md`
- `skills/` 目录下的技能可通过 `/skill-name` 调用
- `agents/` 目录下的子代理可通过 Agent 工具调用

### OpenCode + oh-my-opencode

```bash
OPENCODE_CONFIG_DIR={isdp-data-dir}/agents/{agent-role-id}/.claude opencode run [options]
```

说明：
- OpenCode 通过 oh-my-opencode 插件兼容 `.claude/` 目录结构
- 配置格式与 Claude Code 保持一致

## 数据模型变更

### AgentRoleConfig 扩展

```go
type AgentRoleConfig struct {
    // ... 现有字段 ...

    // 新增字段
    ConfigGeneratedAt *time.Time `json:"config_generated_at,omitempty"` // 配置最后生成时间
    ConfigPath        string     `json:"config_path,omitempty"`         // 配置目录路径（相对或绝对）
}
```

### 新增 Subagent 模型

```go
// Subagent 子代理模型
type Subagent struct {
    ID          uuid.UUID `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Content     string    `json:"content"`      // Markdown内容
    SkillID     uuid.UUID `json:"skill_id"`     // 所属技能包ID（可选）
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// AgentSubagentBinding Agent角色与子代理绑定
type AgentSubagentBinding struct {
    ID          uuid.UUID `json:"id"`
    AgentRoleID uuid.UUID `json:"agent_role_id"`
    SubagentID  uuid.UUID `json:"subagent_id"`
    CreatedAt   time.Time `json:"created_at"`
}
```

## API 设计

### 配置生成 API

```
POST /api/v1/agents/{id}/config/generate
```

请求体：
```json
{
  "base_agent_type": "claude_code",  // claude_code | open_code
  "clean_existing": false
}
```

响应：
```json
{
  "agent_id": "uuid",
  "config_path": "./data/agents/{agent-role-id}/.claude",
  "skills_count": 3,
  "subagents_count": 2,
  "generated_at": "2026-03-22T12:00:00Z"
}
```

### 子代理管理 API

```
GET    /api/v1/subagents           # 列出所有子代理
POST   /api/v1/subagents           # 创建子代理
GET    /api/v1/subagents/{id}      # 获取子代理详情
PUT    /api/v1/subagents/{id}      # 更新子代理
DELETE /api/v1/subagents/{id}      # 删除子代理

GET    /api/v1/agents/{id}/subagents          # 获取Agent绑定的子代理
POST   /api/v1/agents/{id}/subagents          # 绑定子代理到Agent
DELETE /api/v1/agents/{id}/subagents/{sub_id} # 解除绑定
```

## 前端改动

### Agent角色列表页面

1. 新增"生成配置"按钮（每行一个）
2. 新增"配置状态"列，显示：
   - 未生成
   - 已生成（显示时间）
3. 生成后显示配置目录路径

### Agent角色编辑页面

1. 保存成功后弹出提示："是否立即生成配置？"
2. 新增"子代理绑定"区域（类似技能绑定）

### 新增子代理管理页面

1. 路由：`/subagents`
2. 功能：子代理的增删改查
3. 支持上传子代理Markdown文件

## 配置文件格式

### settings.json 示例

```json
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "${API_TOKEN}",
    "ANTHROPIC_BASE_URL": "${API_URL}"
  },
  "permissions": {
    "allow": [
      "Read(*)",
      "Write(*)",
      "Edit(*)",
      "Bash(npm *)",
      "Bash(git *)"
    ],
    "deny": []
  },
  "model": "claude-sonnet-4-6",
  "mcpServers": {
    "isdp-a2a": {
      "command": "node",
      "args": ["${ISDP_MCP_SERVER}"],
      "env": {
        "ISDP_AGENT_ID": "${AGENT_ID}"
      }
    }
  }
}
```

### CLAUDE.md 示例

```markdown
# 前端开发专家

你是一个专注于前端开发的专家Agent。

## 职责

- 实现React/Vue组件
- 编写TypeScript代码
- 优化前端性能

## 技能

- 代码审查
- 单元测试

## 协作

- 可调用 @test-runner 子代理运行测试
- 可调用 @code-reviewer 子代理审查代码
```

### 子代理文件示例 (agents/test-runner.md)

```markdown
---
name: test-runner
description: 运行测试并生成报告
---

# Test Runner

你是一个测试运行器子代理。

## 职责

1. 分析项目测试配置
2. 运行相关测试
3. 生成测试报告

## 权限

- 只读权限
- 可执行测试命令
```

## 实现计划

### Phase 1: 数据层

1. 数据库迁移：添加 subagents 表和 agent_subagent_bindings 表
2. 新增 Subagent 模型和 Repository
3. 扩展 AgentRoleConfig 模型

### Phase 2: 服务层

1. 重构 ConfigGen 服务：改为按Agent角色粒度生成
2. 新增 Subagent Service
3. 修改 Adapter 启动逻辑：支持指定配置目录

### Phase 3: API层

1. 新增配置生成 API
2. 新增子代理管理 API
3. 修改现有 API 返回配置生成状态

### Phase 4: 前端

1. Agent列表页添加"生成配置"功能
2. Agent编辑页添加配置生成提示
3. 新增子代理管理页面

## 影响范围

- **后端**：配置生成服务重构、新增子代理模块、Adapter启动逻辑
- **前端**：Agent列表页、Agent编辑页、新增子代理管理页
- **数据库**：新增2张表
- **兼容性**：不影响现有功能，配置生成改为手动触发

## 验证方法

1. 创建Agent角色，绑定技能和子代理
2. 点击"生成配置"按钮
3. 检查配置目录结构是否正确
4. 使用 Claude Code 启动：`CLAUDE_CONFIG_DIR=./data/agents/{id}/.claude claude`
5. 验证技能和子代理是否可用
6. 对 OpenCode 进行同样验证