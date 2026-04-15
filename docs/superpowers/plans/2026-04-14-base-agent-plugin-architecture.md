# 基础Agent插件化架构重构实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现插件化架构，新增基础Agent只需创建插件包，核心代码零改动

**Architecture:** 三层架构 - 编排层（不变）→ 适配层（Registry）→ 插件层（自动注册）。使用 Go init() 函数自动注册，构建脚本自动扫描插件目录生成导入代码。

**Tech Stack:** Go + Gin + init() 自动注册 + go generate 自动发现

---

## 文件结构

### 新增文件
```
internal/service/agent/
├── adapter_registry.go       # 全局注册中心
├── plugin_types.go           # PluginMeta 等类型定义
│
├── plugins/
│   ├── all/
│   │   └── all.go            # go generate 生成，导入所有插件
│   │
│   ├── claude_code/
│   │   ├── plugin.go         # init() 注册
│   │   ├── adapter.go        # ClaudeAdapter（从 claude_adapter.go 迁移）
│   │   └── parser.go         # stream-json 解析器
│   │
│   └── open_code/
│   │   ├── plugin.go         # init() 注册
│   │   ├── adapter.go        # OpenCodeACPAdapter（重命名）
│   │   └── acp_types.go      # ACP 协议类型（复用）

tools/
└── genplugins/
    └── main.go               # 扫描插件目录生成 all.go
```

### 修改文件
```
internal/model/base_agent.go           # 删除 CLI 常量，重命名 ACP → OpenCode
internal/service/agent/adapter.go       # 删除工厂函数，保留接口
internal/service/agent/base_agent_service.go  # 使用 Registry.GetTypes()
internal/service/agent/execution_service.go    # 使用 Registry.GetAdapter()
internal/service/configgen/service.go          # 使用 Registry.GetConfigDir()
cmd/server/main.go                      # 导入 plugins/all 包
web/src/types/index.ts                  # 移除 open_code_acp 类型
```

### 删除文件
```
internal/service/agent/open_code_adapter.go    # CLI 版本适配器
```

---

## Task 1: 创建 PluginMeta 类型定义

**Files:**
- Create: `internal/service/agent/plugin_types.go`

- [ ] **Step 1: 创建 plugin_types.go 文件**

```go
// internal/service/agent/plugin_types.go
package agent

import (
	"github.com/anthropic/isdp/internal/model"
)

// PluginMeta 插件元数据
type PluginMeta struct {
	Type        model.BaseAgentType // "claude_code", "open_code"
	Name        string              // 显示名称："ClaudeCode", "OpenCode"
	Description string              // 描述
	Factory     AdapterFactory      // func(baseAgent) AgentAdapter
	ConfigDir   string              // 配置目录名：".claude", ".opencode"
	DefaultPath string              // 默认CLI路径："claude", "opencode"
}

// AdapterFactory 适配器工厂函数
type AdapterFactory func(baseAgent *model.BaseAgent) AgentAdapter

// PluginTypeInfo 插件类型信息（用于API返回）
type PluginTypeInfo struct {
	Type        model.BaseAgentType `json:"type"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
}
```

- [ ] **Step 2: 验证文件创建**

Run: `ls internal/service/agent/plugin_types.go`
Expected: 文件存在

- [ ] **Step 3: Commit**

```bash
git add internal/service/agent/plugin_types.go
git commit -m "feat(agent): add PluginMeta types for plugin architecture"
```

---

## Task 2: 创建 AdapterRegistry 注册中心

**Files:**
- Create: `internal/service/agent/adapter_registry.go`

- [ ] **Step 1: 创建 adapter_registry.go 文件**

```go
// internal/service/agent/adapter_registry.go
package agent

import (
	"fmt"
	"sync"

	"github.com/anthropic/isdp/internal/model"
)

// AdapterRegistry 全局适配器注册中心
type AdapterRegistry struct {
	plugins map[model.BaseAgentType]PluginMeta
	mu      sync.RWMutex
}

// 全局注册中心实例
var globalRegistry = &AdapterRegistry{
	plugins: make(map[model.BaseAgentType]PluginMeta),
}

// RegisterPlugin 注册插件（插件 init() 调用）
func RegisterPlugin(meta PluginMeta) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if _, exists := globalRegistry.plugins[meta.Type]; exists {
		panic(fmt.Sprintf("plugin %s already registered", meta.Type))
	}

	globalRegistry.plugins[meta.Type] = meta
}

// GetAdapter 获取适配器（编排层调用）
func GetAdapter(baseAgent *model.BaseAgent) AgentAdapter {
	if baseAgent == nil {
		return nil
	}

	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	meta, exists := globalRegistry.plugins[baseAgent.Type]
	if !exists {
		return nil
	}

	return meta.Factory(baseAgent)
}

// GetTypes 获取所有已注册类型（API调用）
func GetTypes() []PluginTypeInfo {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	types := make([]PluginTypeInfo, 0, len(globalRegistry.plugins))
	for _, meta := range globalRegistry.plugins {
		types = append(types, PluginTypeInfo{
			Type:        meta.Type,
			Name:        meta.Name,
			Description: meta.Description,
		})
	}
	return types
}

// GetMeta 获取插件元数据
func GetMeta(typ model.BaseAgentType) *PluginMeta {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	meta, exists := globalRegistry.plugins[typ]
	if !exists {
		return nil
	}
	return &meta
}

// GetConfigDir 获取配置目录名
func GetConfigDir(typ model.BaseAgentType) string {
	meta := GetMeta(typ)
	if meta == nil {
		return ".claude" // 默认
	}
	return meta.ConfigDir
}
```

- [ ] **Step 2: 验证文件创建**

Run: `ls internal/service/agent/adapter_registry.go`
Expected: 文件存在

- [ ] **Step 3: Commit**

```bash
git add internal/service/agent/adapter_registry.go
git commit -m "feat(agent): add AdapterRegistry for plugin management"
```

---

## Task 3: 创建 plugins 目录结构

**Files:**
- Create: `internal/service/agent/plugins/all/all.go`（初始空导入）
- Create: `internal/service/agent/plugins/claude_code/plugin.go`
- Create: `internal/service/agent/plugins/open_code/plugin.go`

- [ ] **Step 1: 创建 plugins/all/all.go（初始版本）**

```go
// internal/service/agent/plugins/all/all.go
// Code generated by tools/genplugins. DO NOT EDIT.
package all

// 插件包导入列表（自动生成）
import (
	_ "github.com/anthropic/isdp/internal/service/agent/plugins/claude_code"
	_ "github.com/anthropic/isdp/internal/service/agent/plugins/open_code"
)
```

- [ ] **Step 2: 创建 plugins/claude_code/plugin.go**

```go
// internal/service/agent/plugins/claude_code/plugin.go
package claude_code

import (
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
)

func init() {
	agent.RegisterPlugin(agent.PluginMeta{
		Type:        model.BaseAgentTypeClaudeCode,
		Name:        "ClaudeCode",
		Description: "Anthropic Claude CLI - 使用 claude 命令行工具",
		Factory:     NewClaudeAdapter,
		ConfigDir:   ".claude",
		DefaultPath: "claude",
	})
}
```

- [ ] **Step 3: 创建 plugins/open_code/plugin.go**

```go
// internal/service/agent/plugins/open_code/plugin.go
package open_code

import (
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
)

func init() {
	agent.RegisterPlugin(agent.PluginMeta{
		Type:        model.BaseAgentTypeOpenCode,
		Name:        "OpenCode",
		Description: "OpenCode CLI via ACP - 结构化输出",
		Factory:     NewOpenCodeAdapter,
		ConfigDir:   ".opencode",
		DefaultPath: "opencode",
	})
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/service/agent/plugins/
git commit -m "feat(agent): create plugins directory with claude_code and open_code"
```

---

## Task 4: 迁移 ClaudeAdapter 到插件包

**Files:**
- Create: `internal/service/agent/plugins/claude_code/adapter.go`
- Create: `internal/service/agent/plugins/claude_code/parser.go`
- Modify: `internal/service/agent/claude_adapter.go` → 删除

- [ ] **Step 1: 复制 claude_adapter.go 到插件包**

将 `internal/service/agent/claude_adapter.go` 内容复制到 `internal/service/agent/plugins/claude_code/adapter.go`，修改 package 名和导出函数：

```go
// internal/service/agent/plugins/claude_code/adapter.go
package claude_code

import (
	// ... imports 保持不变
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
)

// ClaudeAdapter Claude CLI适配器
type ClaudeAdapter struct {
	// ... 字段保持不变
}

// NewClaudeAdapter 创建Claude适配器（导出函数，供 plugin.go 调用）
func NewClaudeAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
	// ... 实现保持不变
	return adapter
}

// ... 其他方法保持不变，实现 agent.AgentAdapter 接口
```

- [ ] **Step 2: 复制解析器相关代码到 parser.go**

将 claude_adapter.go 中的 stream-json 解析相关代码提取到 parser.go。

- [ ] **Step 3: 删除原 claude_adapter.go**

Run: `rm internal/service/agent/claude_adapter.go`

- [ ] **Step 4: 验证编译**

Run: `go build ./internal/service/agent/plugins/claude_code`
Expected: 编译成功

- [ ] **Step 5: Commit**

```bash
git add internal/service/agent/plugins/claude_code/
git rm internal/service/agent/claude_adapter.go
git commit -m "refactor(agent): migrate ClaudeAdapter to plugin package"
```

---

## Task 5: 迁移 OpenCodeACPAdapter 到插件包并重命名

**Files:**
- Create: `internal/service/agent/plugins/open_code/adapter.go`
- Create: `internal/service/agent/plugins/open_code/acp_adapter.go`
- Create: `internal/service/agent/plugins/open_code/acp_types.go`
- Modify: `internal/service/agent/open_code_acp_adapter.go` → 删除
- Modify: `internal/service/agent/acp_adapter.go` → 移动到插件包
- Modify: `internal/service/agent/acp_types.go` → 移动到插件包

- [ ] **Step 1: 移动 ACP 基础代码到插件包**

复制以下文件到 `plugins/open_code/`:
- `acp_adapter.go` → `plugins/open_code/acp_adapter.go`
- `acp_types.go` → `plugins/open_code/acp_types.go`
- `acp_event_parser.go` → `plugins/open_code/acp_event_parser.go`
- `acp_jsonrpc.go` → `plugins/open_code/acp_jsonrpc.go`

修改 package 名为 `open_code`。

- [ ] **Step 2: 重命名 OpenCodeACPAdapter 为 OpenCodeAdapter**

```go
// internal/service/agent/plugins/open_code/adapter.go
package open_code

import (
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
)

// OpenCodeAdapter OpenCode CLI适配器（原 OpenCodeACPAdapter）
type OpenCodeAdapter struct {
	*BaseACPAdapter
}

// NewOpenCodeAdapter 创建OpenCode适配器（导出函数）
func NewOpenCodeAdapter(baseAgent *model.BaseAgent) agent.AgentAdapter {
	// ... 实现保持不变，从 open_code_acp_adapter.go 复制
}
```

- [ ] **Step 3: 删除原文件**

```bash
rm internal/service/agent/open_code_acp_adapter.go
rm internal/service/agent/acp_adapter.go
rm internal/service/agent/acp_types.go
rm internal/service/agent/acp_event_parser.go
rm internal/service/agent/acp_jsonrpc.go
```

- [ ] **Step 4: 删除 CLI 版本适配器**

```bash
rm internal/service/agent/open_code_adapter.go
```

- [ ] **Step 5: Commit**

```bash
git add internal/service/agent/plugins/open_code/
git rm internal/service/agent/open_code_acp_adapter.go
git rm internal/service/agent/acp_adapter.go
git rm internal/service/agent/acp_types.go
git rm internal/service/agent/acp_event_parser.go
git rm internal/service/agent/acp_jsonrpc.go
git rm internal/service/agent/open_code_adapter.go
git commit -m "refactor(agent): migrate OpenCodeACPAdapter to plugin, rename to OpenCodeAdapter"
```

---

## Task 6: 更新 model/base_agent.go 类型常量

**Files:**
- Modify: `internal/model/base_agent.go`

- [ ] **Step 1: 删除 CLI 版本常量，重命名 ACP 常量**

```go
// internal/model/base_agent.go
package model

// BaseAgentType 基础Agent类型
type BaseAgentType string

const (
	BaseAgentTypeClaudeCode BaseAgentType = "claude_code"
	BaseAgentTypeOpenCode   BaseAgentType = "open_code"  // 原 open_code_acp
	// 删除: BaseAgentTypeOpenCodeCLI (原 open_code)
	// 删除: BaseAgentTypeOpenCodeACP (已合并为 OpenCode)
)
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/base_agent.go
git commit -m "refactor(model): remove OpenCode CLI type, rename ACP to OpenCode"
```

---

## Task 7: 更新 adapter.go 删除工厂函数

**Files:**
- Modify: `internal/service/agent/adapter.go`

- [ ] **Step 1: 删除 NewAdapter 工厂函数**

删除 adapter.go 中的 `NewAdapter` 函数（第50-66行），保留接口定义。

```go
// internal/service/agent/adapter.go
package agent

import (
	"context"
	"os/exec"
)

// AgentAdapter 接口定义保持不变
type AgentAdapter interface {
	Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error)
	ExecuteWithStream(ctx context.Context, req *ExecutionRequest, onChunk func(Chunk)) (*ExecutionResult, error)
	StartSession(ctx context.Context, sessionID string, req *ExecutionRequest) error
	ResumeSession(ctx context.Context, sessionID string, input string, onChunk func(Chunk)) error
	StopSession(sessionID string) error
	GetSessionStatus(sessionID string) SessionStatus
	CheckHealth(ctx context.Context) error
	GetCurrentProcess() *exec.Cmd
}

// 删除 NewAdapter 函数

// 辅助函数 maskToken 保持不变
func maskToken(token string) string {
	// ...
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/service/agent/adapter.go
git commit -m "refactor(agent): remove NewAdapter factory function"
```

---

## Task 8: 更新 base_agent_service.go 使用 Registry

**Files:**
- Modify: `internal/service/agent/base_agent_service.go`

- [ ] **Step 1: 修改 GetTypes 方法**

```go
// internal/service/agent/base_agent_service.go

// GetTypes 获取支持的基础Agent类型
func (s *BaseAgentService) GetTypes() []model.BaseAgentTypeInfo {
	types := GetTypes()
	result := make([]model.BaseAgentTypeInfo, len(types))
	for i, t := range types {
		result[i] = model.BaseAgentTypeInfo{
			Type:        t.Type,
			Name:        t.Name,
			Description: t.Description,
		}
	}
	return result
}
```

- [ ] **Step 2: 修改 TestConnection 方法**

```go
// TestConnection 测试基础Agent连接
func (s *BaseAgentService) TestConnection(ctx context.Context, id uuid.UUID) error {
	agent, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	adapter := GetAdapter(agent)
	if adapter == nil {
		return errors.New("unsupported agent type")
	}

	return adapter.CheckHealth(ctx)
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/service/agent/base_agent_service.go
git commit -m "refactor(agent): use Registry in BaseAgentService"
```

---

## Task 9: 更新 execution_service.go 使用 Registry

**Files:**
- Modify: `internal/service/agent/execution_service.go`

- [ ] **Step 1: 修改 getAdapter 方法**

```go
// internal/service/agent/execution_service.go

func (es *ExecutionService) getAdapter(ctx context.Context, config *model.AgentRoleConfig, baseAgent *model.BaseAgent) (AgentAdapter, error) {
	// 如果有 BaseAgent，使用 Registry 获取适配器
	if baseAgent != nil {
		adapter := GetAdapter(baseAgent)
		if adapter == nil {
			return nil, fmt.Errorf("不支持的基础Agent类型: %s", baseAgent.Type)
		}
		return adapter, nil
	}

	// 如果配置了BaseAgentID但没有传入baseAgent，尝试获取
	if config.BaseAgentID != uuid.Nil && es.baseAgentRepo != nil {
		ba, err := es.baseAgentRepo.FindByID(ctx, config.BaseAgentID)
		if err == nil {
			adapter := GetAdapter(ba)
			if adapter != nil {
				return adapter, nil
			}
		}
	}

	// 向后兼容：使用默认适配器
	if es.defaultAdapter != nil {
		return es.defaultAdapter, nil
	}

	return nil, errors.New("no adapter available")
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/service/agent/execution_service.go
git commit -m "refactor(agent): use Registry.GetAdapter in ExecutionService"
```

---

## Task 10: 更新 configgen/service.go 使用 Registry

**Files:**
- Modify: `internal/service/configgen/service.go`

- [ ] **Step 1: 修改 getConfigDirName 方法**

```go
// internal/service/configgen/service.go

import (
	"github.com/anthropic/isdp/internal/service/agent"
)

// getConfigDirName 获取配置目录名称
func (s *Service) getConfigDirName(baseAgentType string) string {
	return agent.GetConfigDir(model.BaseAgentType(baseAgentType))
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/service/configgen/service.go
git commit -m "refactor(configgen): use Registry.GetConfigDir"
```

---

## Task 11: 更新 main.go 导入插件包

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: 添加插件包导入**

```go
// cmd/server/main.go
package main

import (
	// ... 其他导入

	// 导入所有插件包（触发 init() 自动注册）
	_ "github.com/anthropic/isdp/internal/service/agent/plugins/all"
)

func main() {
	// ... main 函数保持不变
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(server): import plugins/all to trigger auto-registration"
```

---

## Task 12: 更新前端 TypeScript 类型

**Files:**
- Modify: `web/src/types/index.ts`
- Modify: `web/src/pages/Settings/BaseAgentSettings.tsx`

- [ ] **Step 1: 更新 web/src/types/index.ts**

```typescript
// web/src/types/index.ts

// 移除 open_code_acp，只保留 claude_code 和 open_code
export type BaseAgentType = 'claude_code' | 'open_code';
```

- [ ] **Step 2: 更新颜色映射（如有）**

如果 `BaseAgentSettings.tsx` 中有 `getTypeColor` 函数，移除 `open_code_acp` 相关逻辑。

- [ ] **Step 3: Commit**

```bash
git add web/src/types/index.ts web/src/pages/Settings/BaseAgentSettings.tsx
git commit -m "refactor(web): remove open_code_acp type, unify to open_code"
```

---

## Task 13: 创建自动发现工具 genplugins

**Files:**
- Create: `tools/genplugins/main.go`

- [ ] **Step 1: 创建 tools/genplugins/main.go**

```go
// tools/genplugins/main.go
package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	pluginsDir := "internal/service/agent/plugins"

	// 扫描子目录
	dirs := []string{}
	filepath.WalkDir(pluginsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != pluginsDir && !strings.Contains(path, "all") {
			relPath := filepath.Base(path)
			dirs = append(dirs, relPath)
		}
		return nil
	})

	// 生成导入代码
	imports := ""
	for _, dir := range dirs {
		imports += fmt.Sprintf("\t_ \"github.com/anthropic/isdp/internal/service/agent/plugins/%s\"\n", dir)
	}

	content := fmt.Sprintf(`// Code generated by tools/genplugins. DO NOT EDIT.
package all

import (
%s)
`, imports)

	// 写入文件
	allPath := filepath.Join(pluginsDir, "all", "all.go")
	os.WriteFile(allPath, []byte(content), 0644)

	fmt.Printf("Generated %s with %d plugins\n", allPath, len(dirs))
}
```

- [ ] **Step 2: 测试工具**

Run: `go run tools/genplugins/main.go`
Expected: 生成 `plugins/all/all.go`

- [ ] **Step 3: Commit**

```bash
git add tools/genplugins/
git commit -m "feat(tools): add genplugins for auto-discovering plugin packages"
```

---

## Task 14: 更新 Makefile/构建脚本

**Files:**
- Modify: `Makefile` 或构建脚本

- [ ] **Step 1: 添加 genplugins 步骤到构建流程**

```makefile
# Makefile
build:
	go run tools/genplugins/main.go
	go build ./cmd/server
```

- [ ] **Step 2: Commit**

```bash
git add Makefile
git commit -m "feat(build): add genplugins step to build process"
```

---

## Task 15: 整体验证和测试

- [ ] **Step 1: 全量编译**

Run: `go build ./cmd/server`
Expected: 编译成功，无错误

- [ ] **Step 2: 启动服务**

Run: `./bin/isdp-server.exe`
Expected: 服务启动成功

- [ ] **Step 3: 测试 API**

```bash
curl http://localhost:8080/api/v1/base_agents/types
```
Expected: 返回 `[{"type":"claude_code","name":"ClaudeCode",...}, {"type":"open_code","name":"OpenCode",...}]`

- [ ] **Step 4: 验收确认**

- 新增插件只需创建 `plugins/xxx/` 目录
- API 返回的类型列表来自 Registry
- 前端页面动态获取类型
- OpenCode CLI 版本已删除，ACP 版本重命名为 open_code
- 显示名称无空格：ClaudeCode、OpenCode

- [ ] **Step 5: 最终 Commit**

```bash
git add -A
git commit -m "feat(agent): complete plugin architecture refactor"
```

---

## 后续任务（可选）

### Task 16: 更新 CLAUDE.md 架构约束

- [ ] 在 CLAUDE.md 中添加插件架构约束规则

### Task 17: 添加插件开发文档

- [ ] 创建 `docs/plugin-development.md` 说明如何新增插件