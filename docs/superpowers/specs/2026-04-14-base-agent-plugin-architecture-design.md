# Colink 基础Agent架构重构设计

> 日期：2026-04-14
> 目标：实现插件化架构，新增基础Agent只需实现插件包，核心代码零改动

## 背景

Colink是多Agent协作平台，支持多种基础Agent类型。当前架构存在以下问题：

1. **硬编码工厂模式**：`adapter.go` 使用 switch 语句创建适配器，违反开闭原则
2. **多文件耦合**：新增Agent需修改 4+ 个文件
3. **无法插件化**：内网自研Agent无法独立打包部署
4. **类型混乱**：OpenCode CLI 和 OpenCode(ACP) 两种类型对用户暴露
5. **命名不一致**："Claude Code" 显示名称有空格

## 目标架构

### 三层架构

```
┌────────────────────────────────────────────────────────────────────┐
│  第一层：编排层（不变）                                               │
│  Orchestrator / ExecutionService                                    │
│    • 管理 Agent 生命周期                                             │
│    • 不感知具体 Agent 类型                                           │
│    • 调用 AdapterRegistry.GetAdapter(type)                          │
├────────────────────────────────────────────────────────────────────┤
│  第二层：适配层（新增）                                               │
│  AdapterRegistry                                                    │
│    • 维护 type → PluginMeta 映射                                    │
│    • RegisterPlugin(meta) - 插件注册入口                             │
│    • GetAdapter(type) - 编排层调用                                   │
│    • GetTypes() - 返回已注册类型列表                                  │
├────────────────────────────────────────────────────────────────────┤
│  第三层：插件层（重构）                                               │
│  plugins/                                                           │
│    ├── claude_code/                                                 │
│    ├── open_code/                                                   │
│    └── [internal_agent]/  ←── 新增插件放这里                         │
│                                                                     │
│  每个插件包：                                                         │
│    plugin.go - init() 自动注册                                       │
│    adapter.go - AgentAdapter 实现                                   │
│    config.go - 默认配置                                              │
└────────────────────────────────────────────────────────────────────┘
```

### 自动发现机制

**编译时扫描插件目录，自动生成导入代码：**

1. 构建脚本执行 `tools/genplugins`
2. 扫描 `plugins/` 子目录（排除 `all`）
3. 生成 `plugins/all/all.go` 导入所有插件包
4. Go `init()` 函数自动执行注册

**新增插件流程：**
```
1. 创建 plugins/xxx/ 目录
2. 实现 plugin.go + adapter.go
3. make build（自动发现）
4. 完成！API和页面自动可用
```

### 前端联动

**API 自动返回已注册类型：**
```
GET /api/v1/base_agents/types

响应：
{
  "types": [
    {"type": "claude_code", "name": "ClaudeCode", "description": "..."},
    {"type": "open_code", "name": "OpenCode", "description": "..."},
    {"type": "internal_agent", "name": "内网Agent", "description": "..."}  ← 新插件
  ]
}
```

前端页面动态渲染选项，新增插件后无需改动前端代码。

## 核心接口

### PluginMeta

```go
type PluginMeta struct {
    Type        BaseAgentType  // "claude_code", "open_code"
    Name        string         // 显示名称，无空格："ClaudeCode"
    Description string         // 描述
    Factory     AdapterFactory // func(baseAgent) AgentAdapter
    ConfigDir   string         // 配置目录名：".claude"
    DefaultPath string         // 默认CLI路径："claude"
}
```

### AdapterRegistry

```go
type AdapterRegistry struct {
    plugins map[BaseAgentType]PluginMeta
}

// 全局注册函数（插件init()调用）
func RegisterPlugin(meta PluginMeta)

// 获取适配器（编排层调用）
func GetAdapter(baseAgent *model.BaseAgent) AgentAdapter

// 获取所有类型（API调用）
func GetTypes() []BaseAgentTypeInfo
```

### 插件注册示例

```go
// plugins/claude_code/plugin.go
package claude_code

import "github.com/colink/isdp/internal/service/agent"

func init() {
    agent.RegisterPlugin(agent.PluginMeta{
        Type:        "claude_code",
        Name:        "ClaudeCode",
        Description: "Anthropic Claude CLI",
        Factory:     NewClaudeAdapter,
        ConfigDir:   ".claude",
        DefaultPath: "claude",
    })
}
```

## 目录结构

```
internal/service/agent/
│
├── adapter_registry.go    # 全局注册中心
├── adapter.go             # AgentAdapter 接口定义
├── types.go               # PluginMeta 等类型定义
│
├── plugins/               # 插件目录
│   │
│   ├── all/               # 自动生成，导入所有插件
│   │   └── all.go         # go generate 生成
│   │
│   ├── claude_code/
│   │   ├── plugin.go      # init() 注册
│   │   ├── adapter.go     # ClaudeAdapter
│   │   └── parser.go      # stream-json 解析
│   │
│   ├── open_code/
│   │   ├── plugin.go      # init() 注册
│   │   ├── adapter.go     # OpenCodeAdapter（ACP）
│   │   └── acp_types.go   # ACP 协议类型
│   │
│   └── [internal_agent]/  # 内网插件
│       ├── plugin.go
│       ├── adapter.go
│       └── ...
│
├── orchestrator.go        # 编排层（改用 Registry）
├── execution_service.go   # 执行服务（改用 Registry）
└── base_agent_service.go  # 基础Agent服务（改用 Registry）

tools/
└── genplugins/
    └── main.go            # 扫描插件目录生成 all.go
```

## 改动清单

### 删除

| 文件/内容 | 原因 |
|-----------|------|
| `open_code_adapter.go` | CLI版本，已废弃 |
| `BaseAgentTypeOpenCode` CLI常量 | 合并为 OpenCode(ACP) |
| `NewAdapter()` 工厂函数 | 替换为 Registry |
| `GetTypes()` 硬编码实现 | 替换为 Registry.GetTypes() |

### 新增

| 文件 | 说明 |
|------|------|
| `adapter_registry.go` | 全局注册中心 |
| `types.go` | PluginMeta 定义 |
| `plugins/claude_code/plugin.go` | Claude 插件注册 |
| `plugins/open_code/plugin.go` | OpenCode 插件注册 |
| `tools/genplugins/main.go` | 自动发现工具 |

### 修改

| 文件 | 改动 |
|------|------|
| `model/base_agent.go` | 删除 OpenCode CLI 常量，重命名 |
| `adapter.go` | 删除工厂，保留接口 |
| `base_agent_service.go` | 使用 Registry.GetTypes() |
| `execution_service.go` | 使用 Registry.GetAdapter() |
| `configgen/service.go` | 使用 Registry.GetMeta().ConfigDir |
| `main.go` | 导入 plugins/all 包 |

## ACP 协议

ACP（Agent Communication Protocol）提供标准化事件流格式：

| 事件类型 | 说明 |
|----------|------|
| `tool_use` | 工具调用请求 |
| `tool_result` | 工具执行结果 |
| `thinking` | 思考过程 |
| `usage` | Token用量统计 |
| `error` | 错误信息 |

**优势**：新Agent只需实现ACP协议输出，无需单独适配解析逻辑。

## 内网部署方案

### 方式1：直接放入仓库

```bash
mkdir internal/service/agent/plugins/internal_agent
# 编写插件代码
make build
```

### 方式2：独立仓库 + replace

```go
// go.mod
replace github.com/colink/isdp/plugins/internal_agent => ../internal-agent-plugin
```

### 方式3：构建脚本复制

```bash
cp ../internal-agent/* isdp/internal/service/agent/plugins/internal_agent/
make build
```

无论哪种方式，Colink 核心代码完全不动。

## 约束规则

写入 CLAUDE.md 的架构约束：

1. **编排层禁止感知具体Agent类型**：只调用 Registry.GetAdapter()
2. **新增Agent只写插件包**：放在 plugins/ 目录，实现 init() 注册
3. **AgentAdapter 接口不变**：所有插件实现相同接口
4. **配置从 Registry 获取**：默认路径、配置目录等从 PluginMeta 读取
5. **前端动态获取类型**：通过 API 获取，不硬编码类型列表

## 验收标准

1. 新增插件只需创建 `plugins/xxx/` 目录
2. 编译后 API 自动返回新类型
3. 前端页面自动显示新选项
4. 内网部署无需修改 Colink 核心代码
5. "Claude Code" 改为 "ClaudeCode"（无空格）
6. OpenCode(ACP) 改为 OpenCode（单一类型）