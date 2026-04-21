# 五层上下文结构设计文档

> 参考：clowder-ai SystemPromptBuilder + ContextAssembler
> 版本：v1.0.0
> 日期：2026-04-20

---

## 一、架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                    Agent Invocation Prompt                       │
├─────────────────────────────────────────────────────────────────┤
│ L0: 静态身份 + 治理摘要 (GOVERNANCE_L0_DIGEST)                    │
│     - 角色定义                                                    │
│     - 系统提示 (config.SystemPrompt)                             │
│     - 治理摘要 (~150 tokens，抵抗上下文压缩)                       │
├─────────────────────────────────────────────────────────────────┤
│ L1: 链路历史 / Thread 历史                                        │
│     - A2A: PreviousResponses 累积 (五件套交接)                    │
│     - 非 A2A: Token-based 裁剪的 Thread 消息                      │
├─────────────────────────────────────────────────────────────────┤
│ L2: 工作产物 (Artifacts)                                          │
│     - 已生成的代码文件                                            │
│     - 设计文档                                                    │
├─────────────────────────────────────────────────────────────────┤
│ L3: 环境信息                                                      │
│     - Thread 状态                                                 │
│     - 项目路径                                                    │
│     - Git 上下文                                                  │
│     - 指令文件 (CLAUDE.md, AGENTS.md)                             │
├─────────────────────────────────────────────────────────────────┤
│ L4: 动态协作信息 (BuildDynamicSystemPromptFromContext)            │
│     - 下游协作方 (@mention targets)                               │
│     - 跨团队协作方                                                │
│     - 队友名册                                                    │
└─────────────────────────────────────────────────────────────────┘
```

---

## 二、各层详解

### L0: 静态身份 + 治理摘要

**编译函数**: `BuildStaticLayer0(config)`

**内容来源**:
- `config.Name` + `config.Description` → 角色定义
- `config.SystemPrompt` → 系统提示
- `GOVERNANCE_L0_DIGEST` → 治理摘要（从 shared-rules.md 编译）

**Token 预算**: 约 200-300 tokens（含治理摘要）

**关键特性**:
- **抵抗上下文压缩**: 治理摘要通过 `<!-- GOVERNANCE_DIGEST_VERSION -->` 标记，确保不被截断
- **单一真相源**: 治理规则定义在 `docs/governance/shared-rules.md`，其他文件引用不重复

**代码位置**: `context_builder.go:85-115`

```go
func BuildStaticLayer0(config *model.AgentConfig) string {
    var sb strings.Builder
    
    // 角色定义
    sb.WriteString(fmt.Sprintf("你是 %s (%s)。\n\n", config.Name, config.Description))
    
    // 系统提示
    sb.WriteString(config.SystemPrompt)
    sb.WriteString("\n\n")
    
    // 治理摘要（GOVERNANCE_L0_DIGEST）
    sb.WriteString("---\n\n")
    sb.WriteString(BuildGovernanceDigestWithVersion())
    sb.WriteString("\n---\n\n")
    
    return sb.String()
}
```

---

### L1: 链路历史 / Thread 历史

**编译函数**: `BuildChainHistoryLayer(chainHistory)` 或 `ExtractStructuredHistoryWithBudget(messages, modelName, tbm)`

**A2A 调用**:
- `PreviousResponses` → 前序 Agent 响应累积
- `OriginalMessage` → 原始用户消息
- 优先提取五件套交接块 (`<a2a-handoff>`)

**非 A2A 调用**:
- 反向遍历 Thread 消息
- Token-based 裁剪（保留最近上下文）
- 提取：用户请求、关键结论、涉及文件、工具调用摘要、对话参与者

**Token 预算**: 动态计算，约 5% 上下文窗口

**代码位置**: `context_builder.go:242-320`

---

### L2: 工作产物

**编译函数**: `getArtifacts(thread)`

**内容**:
- 已生成的代码文件路径
- 设计文档摘要
- API 定义

**当前状态**: 暂未实现，返回空字符串

---

### L3: 环境信息

**编译函数**: `getEnvironmentInfoEnhanced(tc)`

**内容**:
- Thread 状态（ID、阶段、状态）
- 项目路径
- Git 上下文（可选）
- 指令文件（CLAUDE.md、AGENTS.md）

**代码位置**: `context_builder.go:168-212`

---

### L4: 动态协作信息

**编译函数**: `BuildDynamicSystemPromptFromContext(tc, config)`

**内容**:
- 下游协作方列表（从 Transitions 提取）
- 跨团队协作方（RoutableTeamAgents）
- 队友名册（AllowedAgents）

**关键改进**: 治理规则已移至 L0，此处不再重复

**代码位置**: `context_builder.go:322-396`

---

## 三、与 clowder-ai 对齐

### 3.1 静态/动态分离

| 层级 | clowder-ai | ISDP |
|------|------------|------|
| 静态身份 | `buildStaticIdentity()` + `--append-system-prompt` | `BuildStaticLayer0()` 嵌入 prompt |
| 动态协作 | `buildInvocationContext()` | `BuildDynamicSystemPromptFromContext()` |

**差距**: ISDP 未使用 `--append-system-prompt` 参数，静态部分每次重建

**优化方向**: 
- 通过 ACP 适配器支持 `--system-prompt` 参数
- 静态部分一次注入，抵抗压缩

### 3.2 GOVERNANCE_L0_DIGEST

| 项目 | clowder-ai | ISDP |
|------|------------|------|
| 来源 | `refs/shared-rules.md` | `docs/governance/shared-rules.md` |
| 编译 | `compileGovernanceDigest()` | `BuildGovernanceDigest()` |
| Token | ~150 | ~150 |
| 嵌入位置 | `SystemPromptBuilder` | `BuildStaticLayer0` |

### 3.3 链路历史

| 项目 | clowder-ai | ISDP |
|------|------------|------|
| 数据结构 | `previousResponses[]` | `PreviousResponses[]` |
| 交接格式 | 五件套 (What/Why/Tradeoff/Open/Next) | 五件套 (`<a2a-handoff>`) |
| Token 控制 | `truncateHeadTail()` | `ConstrainHandoffBudget()` |

---

## 四、治理规则 SSOT

### 4.1 单一真相源设计

```
docs/governance/shared-rules.md (SSOT)
    ↓ 编译
internal/service/agent/governance_digest.go
    ↓ 函数
BuildGovernanceDigest() → ~150 tokens
    ↓ 嵌入
BuildStaticLayer0() → L0
    ↓ 调用
每个 Agent Invocation
```

### 4.2 规则编号索引

| 编号 | 规则 | 章节 |
|------|------|------|
| R1 | A2A 出口检查 | shared-rules.md#1.1 |
| R2 | @mention 格式规则 | shared-rules.md#1.2 |
| R3 | 角色边界 | shared-rules.md#1.3 |
| R4 | 根因优先 | shared-rules.md#2.1 |
| R5 | 不确定先确认 | shared-rules.md#2.2 |
| R6 | Scope 守则 | shared-rules.md#2.3 |
| R7 | 五件套交接 | shared-rules.md#3.1 |
| R8 | Token 预算控制 | shared-rules.md#3.2 |

### 4.3 引用方式

其他文件引用规则：

```markdown
> 参考 [A2A 出口检查](../governance/shared-rules.md#11-a2a-出口检查)
```

**禁止**: 在其他文件中重复定义规则内容

---

## 五、Token 预算管理

### 5.1 约束流程

```go
ApplyTokenBudgetConstraint(layers, modelName, tbm) {
    windowSize = tbm.GetContextWindowSize(modelName)
    totalTokens = L0 + L1 + L2 + L3
    
    if totalTokens > windowSize {
        safetyBuffer = windowSize * 5%
        availableBudget = windowSize - L0 - safetyBuffer
        
        // 按优先级裁剪：L3 → L2 → L1
        truncate(layers, availableBudget)
    }
}
```

### 5.2 A2A 动态深度

```go
GetMaxA2ADepth(a2aCtx, tbm) {
    remainingBudget = contextWindow - usedTokens
    avgTurnTokens = tbm.GetAvgTurnTokens(threadID)
    
    return min(remainingBudget / avgTurnTokens, MaxA2ADepth)
}
```

---

## 六、文件清单

| 文件 | 职责 |
|------|------|
| `docs/governance/shared-rules.md` | 治理规则 SSOT |
| `internal/service/agent/governance_digest.go` | 治理摘要编译 |
| `internal/service/agent/context_builder.go` | 五层上下文构建 |
| `internal/service/agent/execution_service.go` | 上下文组装入口 |
| `internal/service/agent/token_budget.go` | Token 预算管理 |
| `internal/service/agent/types.go` | 数据结构定义 |

---

## 七、下一步优化

1. **静态部分 CLI 注入**: 通过 `--system-prompt` 参数注入 L0，减少每次调用重建开销
2. **Artifacts 实现**: 完成 L2 工作产物层
3. **动态平均消耗统计**: 完善 `UpdateAvgTurnTokens()` 滑动平均算法
4. **治理规则版本校验**: 启动时检查 shared-rules.md 版本一致性

---

## 附录：变更历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v1.0.0 | 2026-04-20 | 初始设计，创建治理 SSOT + 五层结构 |