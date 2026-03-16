# A2A @mention 路由验证设计

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 @mention 路由验证逻辑，使 Agent 只能路由到工作流模板中已配置的 Agent 实例。

**Tech Stack:** Go, existing Orchestrator architecture

---

## Problem Statement

当前 A2A @mention 路由存在以下问题：

1. **路由验证逻辑未生效**：`ValidateRouting` 和 `CanRouteTo` 配置存在但未被调用
2. **路由范围不受控**：Agent 可以 @mention 任意角色，不受工作流模板限制
3. **重复代码**：`getAllowedRoutes` 和 `getDefaultRouting` 定义了相同的路由规则

---

## Solution

### 核心原则

- @mention 只能路由到**当前工作流模板中已配置的 Agent 实例**
- 支持两种格式：`@role`（角色别名）和 `@agent-name`（实例名称）
- 无工作流模板时回退到原有逻辑

### 数据来源

```
Thread → WorkflowTemplate → AgentIDs → AgentConfigs
```

| 数据 | 来源表 | 字段 |
|------|--------|------|
| 会话绑定的模板 | threads | workflow_template_id |
| Agent ID 列表 | workflow_templates | agent_ids (JSON) |
| Agent 详情 | agent_configs | id, name, role |

---

## Implementation Details

### 1. 新增数据结构

**File:** `internal/service/agent/orchestrator.go`

```go
// ParsedMention @mention 解析结果
type ParsedMention struct {
    Role      model.AgentRole // 角色类型（可能为空）
    AgentName string          // Agent 实例名称（可能为空）
    Raw       string          // 原始 mention 文本
}
```

### 2. 修改 parseMentions 函数

**位置:** `orchestrator.go` 第 354-375 行

**改动:** 返回类型从 `[]model.AgentRole` 改为 `[]ParsedMention`

```go
// parseMentions 解析@mention
// 支持: @developer (角色) 或 @前端开发 (实例名称)
func (o *Orchestrator) parseMentions(content string) []ParsedMention {
    var mentions []ParsedMention
    lines := strings.Split(content, "\n")
    count := 0

    for _, line := range lines {
        line = strings.TrimSpace(line)
        if count >= 2 {
            break
        }
        if strings.HasPrefix(line, "@") {
            mention := strings.Fields(line[1:])[0]
            if mention != "" {
                // 尝试解析为角色
                role := parseAgentRole(mention)

                mentions = append(mentions, ParsedMention{
                    Role:      role,
                    AgentName: mention,
                    Raw:       mention,
                })
                count++
            }
        }
    }
    return mentions
}
```

### 3. 修改 checkRouting 函数

**位置:** `orchestrator.go` 第 321-352 行

```go
// checkRouting 检查路由
func (o *Orchestrator) checkRouting(ctx context.Context, threadID uuid.UUID, currentConfig *model.AgentRoleConfig, output string) {
    mentions := o.parseMentions(output)

    if len(mentions) == 0 {
        // 检查信号路由
        o.checkSignalRouting(ctx, threadID, currentConfig, output)
        return
    }

    // 获取工作流模板中的 Agent 列表
    allowedAgents := o.getAllowedAgentsFromWorkflow(ctx, threadID)

    for _, mention := range mentions {
        var targetConfig *model.AgentRoleConfig

        if mention.Role != "" {
            // 按 role 查找
            targetConfig = o.findAgentByRole(allowedAgents, mention.Role)
        } else {
            // 按 name 查找
            targetConfig = o.findAgentByName(allowedAgents, mention.AgentName)
        }

        if targetConfig == nil {
            logInfo("路由被拒绝：目标不在工作流模板中",
                zap.String("mention", mention.Raw),
                zap.String("threadId", threadID.String()))
            continue
        }

        // 使用工作流模板中指定的 Agent 实例
        o.SpawnAgent(ctx, &SpawnRequest{
            ThreadID: threadID,
            ConfigID: targetConfig.ID,
            Role:     targetConfig.Role,
            Input:    output,
        })
    }
}
```

### 4. 新增辅助函数

**位置:** `orchestrator.go` 在 `checkRouting` 函数后添加

```go
// getAllowedAgentsFromWorkflow 从工作流模板获取允许路由的 Agent 列表
func (o *Orchestrator) getAllowedAgentsFromWorkflow(ctx context.Context, threadID uuid.UUID) []*model.AgentRoleConfig {
    // 1. 获取 Thread
    thread, err := o.threadRepo.FindByID(ctx, threadID)
    if err != nil || thread.WorkflowTemplateID == nil {
        return nil
    }

    // 2. 获取工作流模板
    workflow, err := o.workflowRepo.FindByID(ctx, *thread.WorkflowTemplateID)
    if err != nil || workflow == nil {
        return nil
    }

    // 3. 解析 AgentIDs JSON
    var agentIDs []string
    if len(workflow.AgentIDs) == 0 {
        return nil
    }
    if err := json.Unmarshal(workflow.AgentIDs, &agentIDs); err != nil {
        return nil
    }

    // 4. 查询每个 Agent 的配置
    var agents []*model.AgentRoleConfig
    for _, idStr := range agentIDs {
        id, err := uuid.Parse(idStr)
        if err != nil {
            continue
        }
        agent, err := o.configSvc.GetByID(ctx, id)
        if err == nil {
            agents = append(agents, agent)
        }
    }

    return agents
}

// findAgentByRole 在 Agent 列表中按角色查找
func (o *Orchestrator) findAgentByRole(agents []*model.AgentRoleConfig, role model.AgentRole) *model.AgentRoleConfig {
    for _, agent := range agents {
        if agent.Role == role {
            return agent
        }
    }
    return nil
}

// findAgentByName 在 Agent 列表中按名称查找
func (o *Orchestrator) findAgentByName(agents []*model.AgentRoleConfig, name string) *model.AgentRoleConfig {
    for _, agent := range agents {
        if agent.Name == name {
            return agent
        }
    }
    return nil
}

// checkSignalRouting 检查信号路由（原有逻辑提取）
func (o *Orchestrator) checkSignalRouting(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string) {
    for _, signal := range config.RoutingConfig.RouteOnSignal {
        if strings.Contains(output, signal) {
            nextPhase := o.workflow.GetNextPhase(getPhaseFromSignal(signal))
            nextRole := o.workflow.GetPhaseAgent(nextPhase)
            o.SpawnAgent(ctx, &SpawnRequest{
                ThreadID: threadID,
                Role:     nextRole,
                Input:    output,
            })
            break
        }
    }
}
```

### 5. 修改 executeAgent 调用

**位置:** `orchestrator.go` 第 228 行

**改动:** `checkRouting` 调用参数增加 `config`

```go
// 原代码
o.checkRouting(ctx, req.ThreadID, config, output)

// 保持不变，checkRouting 内部逻辑已修改
```

---

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│  Agent 执行完成                                                  │
│  output: "@前端开发 请实现登录页面，@reviewer 完成后请评审"         │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  checkRouting()                                                 │
│  1. parseMentions(output)                                       │
│     → [                                                          │
│         {Role: "", AgentName: "前端开发"},                       │
│         {Role: "reviewer", AgentName: "reviewer"}               │
│       ]                                                          │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  getAllowedAgentsFromWorkflow(threadID)                         │
│  1. threadRepo.FindByID(threadID)                               │
│     → thread.workflow_template_id                                │
│  2. workflowRepo.FindByID(templateID)                           │
│     → agent_ids: ["uuid-1", "uuid-2", "uuid-3"]                 │
│  3. configSvc.GetByID() for each ID                             │
│     → [                                                          │
│         {ID: uuid-1, Name: "需求分析", Role: "requirement"},     │
│         {ID: uuid-2, Name: "前端开发", Role: "developer"},       │
│         {ID: uuid-3, Name: "代码评审", Role: "reviewer"},        │
│       ]                                                          │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  匹配 @mention                                                   │
│  mention[0]: Role="" → findAgentByName("前端开发") → uuid-2 ✓   │
│  mention[1]: Role="reviewer" → findAgentByRole() → uuid-3 ✓     │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  SpawnAgent(ConfigID: uuid-2, ...)  // 前端开发                  │
│  SpawnAgent(ConfigID: uuid-3, ...)  // 代码评审                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Edge Cases

| 场景 | 处理方式 |
|------|----------|
| Thread 未绑定工作流模板 | `getAllowedAgentsFromWorkflow` 返回 nil，跳过路由 |
| @mention 角色不在模板中 | 记录日志，跳过该路由 |
| @mention 名称不在模板中 | 记录日志，跳过该路由 |
| Agent 配置被删除 | `GetByID` 失败，跳过该 Agent |
| 工作流模板 AgentIDs 为空 | 返回 nil，跳过路由 |

---

## Code Cleanup

### 可删除的重复代码

**File:** `internal/service/a2a/mention_parser.go`

- `getAllowedRoutes` 函数可删除（与 `config_service.go` 中的 `getDefaultRouting` 重复）
- `ValidateRouting` 函数可删除（新逻辑在 `checkRouting` 中实现）

**注意:** 如果 `MentionParser` 在其他地方有使用，需评估后再删除。

---

## Testing Checklist

- [ ] @role 格式路由正常工作
- [ ] @agent-name 格式路由正常工作
- [ ] 路由被限制在工作流模板内的 Agent
- [ ] 不在模板中的 @mention 被正确拒绝
- [ ] Thread 未绑定模板时回退逻辑正常
- [ ] 信号路由正常工作
- [ ] 日志正确记录路由决策

---

## Files Changed

| File | Change |
|------|--------|
| `internal/service/agent/orchestrator.go` | 修改 `parseMentions`，修改 `checkRouting`，新增辅助函数 |
| `internal/service/a2a/mention_parser.go` | 可选：删除重复代码 |