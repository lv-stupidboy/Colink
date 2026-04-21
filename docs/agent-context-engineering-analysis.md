# Agent 上下文工程对比分析：clowder-ai vs ISDP

> 分析日期：2026-04-17
> 分析师：Deep Interview (di-agent-context-analysis-001)
> 方法：逐文件对比 clowder-ai 与 ISDP 的 Agent 上下文工程实现
> 原则：每个差异点均有代码证据，不臆造不浅层

---

## 目录

1. [Prompt 构建策略](#1-prompt-构建策略)
2. [Token 预算与上下文窗口](#2-token-预算与上下文窗口)
3. [A2A 协作上下文](#3-a2a-协作上下文)
4. [架构与代码质量](#4-架构与代码质量)
5. [优化优先级清单](#5-优化优先级清单)

---

## 1. Prompt 构建策略

### 1.1 静态/动态身份分离

**clowder-ai：明确的静态 + 动态两层架构**

```
buildSystemPrompt() = buildStaticIdentity() + buildInvocationContext()
```

- **`buildStaticIdentity()`** (`SystemPromptBuilder.ts:345-438`): 会话级别注入，包含身份定义、性格、协作规则、队友名册、工作流触发器、家规(L0 digest)、铲屎官参考、MCP 工具文档。**不变内容，通过 `--append-system-prompt` 注入，抵抗上下文压缩。**
- **`buildInvocationContext()`** (`SystemPromptBuilder.ts:445-622`): 每次调用动态构建，包含当前模式、队友列表、prompt tags、活跃参与者、路由策略、SOP 阶段提示、语音模式、Bootcamp 状态等。**可变内容，每次注入。**

**ISDP：混合架构，未分离**

```
buildContextLayers() → Layer0 = buildDynamicSystemPromptFromContext()
```

- `buildDynamicSystemPromptFromContext()` (`execution_service.go:1221-1262`): 将系统提示、下游协作方信息、角色触发提示 **全部混在一起**，每次调用重新构建
- 没有 `--system-prompt` / `--append-system-prompt` 级别的静态注入机制

**差距**：ISDP 每次调用都重建完整 system prompt，消耗更多 token。clowder-ai 的静态部分 ~150-200 tokens，通过 CLI 参数一次注入，每次调用只注入动态部分。

**优化方案**：
1. 将 `config.SystemPrompt` 中不变的协作提示（`roleTriggerHints`、下游协作方列表）提取为静态部分
2. 通过 ACP 适配器的 `--system-prompt` 参数注入（Claude CLI 支持）
3. 动态部分（当前模式、会话策略等）通过 `--append` 或 prompt 中注入

### 1.2 队友感知系统

**clowder-ai：三层队友感知**

1. **`buildCallableMentions()`** (`SystemPromptBuilder.ts:146-186`): 构建可 @ 的队友列表，处理同名冲突（变体猫用 `@id` 区分）
2. **`buildTeammateRoster()`** (`SystemPromptBuilder.ts:294-313`): 名册表格，含擅长和注意事项
3. **`buildInvocationContext().teammates`** (`SystemPromptBuilder.ts:471-480`): 当前调用中实际参与的队友

**ISDP：单层队友感知**

- `roleTriggerHints` (`execution_service.go:1195-1208`): 硬编码的角色 → @mention 映射
- `transitions` 遍历生成下游协作方列表 (`execution_service.go:1228-1262`)
- **没有同名处理、没有变体支持、没有动态名册**

**差距**：ISDP 的队友感知是静态配置的硬编码映射，无法适应动态变化的 Agent 团队。clowder-ai 的三层设计覆盖了全局名册、当前调用、同名消歧三种场景。

### 1.3 治理规则注入

**clowder-ai**：`GOVERNANCE_L0_DIGEST` (`SystemPromptBuilder.ts:238-251`) — 约 200 tokens 的家规，始终注入，原则、世界观、纪律、魔法词。

**ISDP**：**无类似机制**。系统提示中没有任何全局性的质量约束或行为规范。

### 1.4 指令文件发现

**clowder-ai**：通过 F129 pack blocks 机制，编译后的 `CompiledPackBlocks` 注入到静态身份（`packBlocks`, `masksBlock`, `guardrailBlock`, `defaultsBlock`, `worldDriverSummary`）。

**ISDP**：`project_context.go:DiscoverInstructionFiles()` — 扫描 `CLAUDE.md`、`CLAUDE.local.md`、`.claude/CLAUDE.md`、`.claude/instructions.md`，支持内容去重（SHA256 hash）和字符截断。

**评估**：ISDP 在指令文件发现方面实现完整，但注入时机在 Layer3（环境信息），而非 system prompt 级别。clowder-ai 将其注入到静态身份（持久化），ISDP 每次重建。

---

## 2. Token 预算与上下文窗口

### 2.1 Token 估算精度

**clowder-ai**：`estimateTokens()` — 简化版字符/4，与 ISDP 相同。

**ISDP**：`EstimateTokens()` (`token_budget.go:174-181`) — 简化版字符/4。

**差距**：两者精度相同，都是粗略估算。clowder-ai 的 `estimateTokens` 在 `ContextAssembler.ts` 中用于实际预算分配，而 ISDP 的 `EstimateTokens` 主要用于 `BudgetForContext()` 和 `EstimateTokenBudget()` 辅助函数，**未在 A2A 链路中实际执行预算裁剪**。

### 2.2 上下文窗口管理

**clowder-ai**：
- `context-window-sizes.ts`：支持 Claude/Codex/Gemini 全系列模型，Gemini 支持 1,000,000 tokens
- 实际运行时通过 CLI 报告的 `modelUsage[model].contextWindow` 获取精确值，fallback 到硬编码表
- 支持 prefix match

**ISDP**：
- `token_budget.go:56-67`：仅支持 Claude + GPT 系列，**缺少 Gemini 支持**
- 无运行时获取机制，纯静态 fallback 表

**优化方案**：添加 Gemini 模型支持（`gemini-2.5-pro: 1_000_000` 等），并实现运行时获取（从 CLI Usage 报告）。

### 2.3 历史消息截断

**clowder-ai：Token-based 预算分配 + Head-Tail 截断**

```typescript
// ContextAssembler.ts:107-125
// Token-based：从最近消息反向遍历，按 token 预算裁剪
let totalTokens = overheadTokens;
for (let i = formatted.length - 1; i >= 0; i--) {
  const lineTokens = estimateTokens(`${formatted[i]!}\n`);
  if (totalTokens + lineTokens > maxTotalTokens) break;
  totalTokens += lineTokens;
  startIndex = i;
}
// Head-Tail 截断：40% 开头 + 60% 结尾
// ContextAssembler.ts:68-76
const headSize = Math.floor(available * 0.4);
const tailSize = available - headSize;
```

**ISDP：消息数量限制 + 字符截断**

```go
// context_builder.go:89
messages, err := b.msgRepo.FindByThreadID(ctx, threadID, 50)
// token_budget.go:186
// Head-Tail 截断：与 clowder-ai 相同的 40/60 算法
```

**差距**：
1. clowder-ai 用 **token 预算** 决定包含多少消息（动态），ISDP 用 **固定数量 50**（静态）
2. clowder-ai 的 `truncateHeadTail` 在消息级别应用，ISDP 的 `TruncateHeadTail` 用于 A2A 前序响应截断
3. clowder-ai 排除未投递消息 (`isDelivered` 过滤, `ContextAssembler.ts:100`)，ISDP 无此过滤

### 2.4 Token 预算在 A2A 中的实际应用

**clowder-ai**：每个 invocation 的 `AssembledContext.estimatedTokens` 被记录并用于后续调用的预算计算。

**ISDP**：
- `TokenBudgetManager` 有完整的 API（`CalculateMaxA2ADepth`, `BudgetForContext`, `GetRemainingBudget`）
- 但 `execution_service.go` 中 **未实际调用这些函数来裁剪上下文**
- `UpdateAvgTurnTokens` 和 `UpdateUsageFromCLI` 虽有定义，但**调用链路不完整**

**差距**：`TokenBudgetManager` 是一个"半成品"——API 设计完整，但在核心流程 `buildContextLayers` 中 **没有实际使用它来约束上下文大小**。上下文构建后直接拼接，没有 token 级别的预算检查。

---

## 3. A2A 协作上下文

### 3.1 链路历史传递

**clowder-ai**：`route-serial` 机制，通过 `previousResponses` 累积前序 Agent 的输出。

**ISDP**：`A2AContext` 结构体 (`execution_service.go:77-92`) + `ChainHistory` (`A2AChainContext`)

```go
type A2AContext struct {
    Depth           int
    InvokedAgents   map[uuid.UUID]bool
    CompletedAgents map[uuid.UUID]bool
    PreviousResponses []ChainResponse  // 前序响应累积
    OriginalMessage   string           // 原始用户消息
    ChainHistory      *A2AChainContext // 链路历史上下文
}
```

**差距**：数据结构设计完整，但注入格式有冗余。`buildChainHistoryLayer()` (`execution_service.go:1100-1191`) 中 **TokenBudget 和 ActiveParticipants 被注入了两次**（第 1113-1118 行和第 1172-1178 行重复，第 1131-1138 行和第 1182-1188 行重复）。

### 3.2 A2A 输入格式

**clowder-ai**（根据项目记忆）：不传递上游 Agent 的原始输出（包含 @mention），而是传递原始用户消息 + 格式化的前序响应上下文。`buildA2AInput()` 负责构造。

**ISDP**：`buildA2AInput()` (`execution_service.go:1609-1712`) 构造了 7 个部分：
1. 协作规则
2. 会话策略
3. 原始请求
4. 前序分析（结构化过滤后的输出）
5. 触发者信息
6. Token 预算
7. 活跃参与者

**差距**：
1. ISDP 的 `buildA2AInput` 注入的"前序分析"包含上游 Agent 的完整输出（经 `filterStructuredOutput` 过滤），**可能包含 @mention**，导致下游 Agent 再次触发相同 @mention
2. clowder-ai 有明确的 `mention_patterns` → `AgentID` 映射 + `ParseForAgents()` 限制匹配范围，ISDP 的 mention 解析在 `execution_service.go:checkRouting()` 中实现，但**缺少当前工作流的 `allowedAgents` 限制**（已部分实现但未完全闭环）

### 3.3 深度控制

**clowder-ai**：通过 `TokenBudget` 动态计算允许的 A2A 深度。

**ISDP**：
- `MaxA2ADepth = 15` (`execution_service.go:34`) — 硬编码常量
- `TokenBudgetManager.CalculateMaxA2ADepth()` 有算法但**未在 `checkRouting` 中实际调用**

**差距**：深度限制是静态的 (15)，没有根据剩余 token 预算动态调整。可能导致 token 耗尽。

### 3.4 会话策略（Resume vs New）

**clowder-ai**：通过 `--resume` / 新会话管理，session context snapshot 支持 (`codex-session-context-snapshot.ts`)。

**ISDP**：`SessionStrategy` 枚举 (`SessionStrategyResume` / `SessionStrategyNew`)，CLI session ID 缓存 (`cliSessions map`)，`shouldAutoResume()` 自动判断。

**评估**：ISDP 的会话策略实现完整，有自动判断和缓存机制。但在 `buildA2AInput` 中只是简单注入了一段说明文本，**CLI 适配器层面**的实现需要进一步验证。

---

## 4. 架构与代码质量

### 4.1 架构分层

**clowder-ai**：
```
context/
├── ContextAssembler.ts      # 消息历史组装
├── SystemPromptBuilder.ts   # System Prompt 构建（静态 + 动态）
├── IntentParser.ts          # 意图解析
├── prompt-digest.ts         # Prompt 审计
└── rich-block-rules.ts      # 富消息规则
```
- 清晰的单一职责：`SystemPromptBuilder` 只管 prompt 构建，`ContextAssembler` 只管消息历史
- 纯函数设计（`buildStaticIdentity`, `buildInvocationContext`），同输入同输出，易于测试

**ISDP**：
```
service/agent/
├── context_builder.go       # 上下文构建（4 层）
├── project_context.go       # 项目上下文
├── token_budget.go          # Token 预算
├── execution_service.go     # 执行服务（2500+ 行，混合了上下文构建、A2A、CLI 执行等）
└── ...
```
- `execution_service.go` 是 **God Object** — 混合了上下文构建、Agent 执行、A2A 路由、内容块持久化、WS 广播、session 管理等职责
- `buildContextLayers`、`buildA2AInput`、`buildChainHistoryLayer` 等方法作为 `ExecutionService` 的方法，**与执行逻辑耦合**，难以独立测试

### 4.2 代码重复

1. **`buildChainHistoryLayer` 中的重复注入**（见 3.1）
2. **`buildA2AInput` vs `BuildA2AInputWithOptions`**：两个版本功能重叠，`BuildA2AInputWithOptions` 有独立的逻辑实现，不是简单的包装器
3. **`buildContextLayers` 和 `ContextBuilder.BuildWithOptions`**：两套上下文构建逻辑共存，`execution_service.go:1060` 实现了自己的版本

### 4.3 测试覆盖

**clowder-ai**：`prompt-digest.ts` 有独立的单元测试文件。

**ISDP**：
- `token_budget_test.go`：存在
- `a2a_context_acceptance_test.go`：存在
- `a2a_input_test.go`：存在

但 `execution_service.go` 中核心方法（`buildContextLayers`、`buildDynamicSystemPromptFromContext`）**缺少独立单元测试**，因为它们耦合在庞大的 `ExecutionService` 中。

### 4.4 Prompt 审计

**clowder-ai**：`prompt-digest.ts` — 记录 prompt length + SHA256 hash，可选首尾片段（默认关闭保护隐私）。

**ISDP**：`invocation.FullPrompt` 存储完整 prompt（`execution_service.go:423`），**没有摘要机制，没有 hash，没有隐私保护**。

**差距**：ISDP 存储完整 prompt 占用大量数据库空间，且可能泄露敏感信息（如 API key 相关上下文）。

---

## 5. 优化优先级清单

### 高优先级

| # | 优化项 | 影响 | 工作量 |
|---|--------|------|--------|
| H1 | **TokenBudgetManager 实际接入 buildContextLayers** | 防止上下文溢出，控制 A2A 深度 | 中 |
| H2 | **修复 buildChainHistoryLayer 中的重复注入** | 减少 prompt token 消耗 | 小 |
| H3 | **A2A 输入中 @mention 过滤** | 防止下游 Agent 重复触发 | 中 |
| H4 | **拆分 execution_service.go 中的上下文构建逻辑** | 提高可测试性 | 大 |

### 中优先级

| # | 优化项 | 影响 | 工作量 |
|---|--------|------|--------|
| M1 | **静态/动态 System Prompt 分离** | 减少每次调用的 token 消耗 | 中 |
| M2 | **Token 估算改为基于实际 CLI Usage 报告** | 提高预算精度 | 小 |
| M3 | **历史消息改为 Token-based 裁剪** | 精确控制上下文大小 | 中 |
| M4 | **添加 Prompt Digest 审计** | 减少数据库空间占用 | 小 |
| M5 | **添加 Gemini 模型支持** | 支持更多模型 | 小 |

### 低优先级

| # | 优化项 | 影响 | 工作量 |
|---|--------|------|--------|
| L1 | **治理规则注入机制（类似 L0 digest）** | 全局质量约束 | 中 |
| L2 | **队友名册动态化** | 适应动态 Agent 团队 | 小 |
| L3 | **ContextAssembler 的 delivered 消息过滤** | 排除未投递消息 | 小 |
| L4 | **动态 A2A 深度计算** | 根据剩余 token 自动调整 | 中 |

---

## 附录：关键文件索引

### clowder-ai
- `packages/api/src/domains/cats/services/context/ContextAssembler.ts` — 消息历史组装
- `packages/api/src/domains/cats/services/context/SystemPromptBuilder.ts` — System Prompt 构建
- `packages/api/src/domains/cats/services/context/prompt-digest.ts` — Prompt 审计
- `packages/api/src/domains/cats/services/context/rich-block-rules.ts` — 富消息规则
- `packages/api/src/config/context-window-sizes.ts` — 模型窗口大小

### ISDP
- `internal/service/agent/execution_service.go` — 执行服务 + 上下文构建
- `internal/service/agent/context_builder.go` — 4 层上下文构建
- `internal/service/agent/project_context.go` — 项目上下文
- `internal/service/agent/token_budget.go` — Token 预算
- `internal/model/agent_invocation.go` — 调用模型
- `internal/model/workflow_template.go` — 工作流模型
