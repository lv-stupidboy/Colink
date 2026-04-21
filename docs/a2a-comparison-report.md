# A2A 交互系统技术对比报告

> clowder-ai vs ISDP 对比分析
> 生成日期: 2026-04-08

## 目录

1. [执行摘要](#执行摘要)
2. [架构概览](#架构概览)
3. [A2A 触发机制对比](#a2a-触发机制对比)
4. [上下文构建对比](#上下文构建对比)
5. [系统提示构建对比](#系统提示构建对比)
6. [状态管理对比](#状态管理对比)
7. [关键差异总结](#关键差异总结)
8. [附录：核心代码对比](#附录核心代码对比)

---

## 执行摘要

### 核心发现

| 维度 | clowder-ai | ISDP | 差异程度 |
|------|-----------|------|---------|
| A2A 触发机制 | 行首 mention + worklist 动态扩展 | 行首 mention + 预定义 allowedAgents | 中等 |
| 上下文构建 | 多层上下文组装 + 前序响应格式化 | 简单拼接（原始消息 + 前序响应） | 较大 |
| 系统提示构建 | 静态身份 + 动态上下文 + Pack 注入 | 简单系统提示 | 较大 |
| 状态管理 | Session Chain + Seal 机制 + 压缩检测 | 简单 A2A Context + 深度限制 | 较大 |

### 关键结论

1. **clowder-ai 更成熟**：具备完整的 session 生命周期管理、上下文压缩检测、Pack 插件系统
2. **ISDP 更简洁**：核心功能完整，但缺少高级特性（session 持久化、动态上下文组装）
3. **两者 Mention 解析规则一致**：都采用行首检测 + token boundary + 长匹配优先

---

## 架构概览

### clowder-ai 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    Agent Router (route-serial.ts)               │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────────────┐  │
│  │ Worklist    │  │ Session      │  │ Context Assembler     │  │
│  │ Registry    │  │ Manager      │  │                       │  │
│  └──────┬──────┘  └──────┬───────┘  └───────────┬───────────┘  │
│         │                │                       │              │
│         ▼                ▼                       ▼              │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              SystemPromptBuilder                         │   │
│  │  ┌───────────────┐  ┌──────────────┐  ┌──────────────┐  │   │
│  │  │ StaticIdentity│  │ InvocationCtx│  │ Pack Blocks  │  │   │
│  │  └───────────────┘  └──────────────┘  └──────────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│                              ▼                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              invoke-single-cat.ts                        │   │
│  │  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐  │   │
│  │  │ Claude/Codex│  │ Gemini       │  │ A2AAgentService│  │   │
│  │  │ AgentService│  │ AgentService │  │ (JSON-RPC)     │  │   │
│  │  └─────────────┘  └──────────────┘  └────────────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### ISDP 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                 ExecutionService (execution_service.go)         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌────────────────┐  │
│  │ ThreadContext   │  │ A2A Context     │  │ Mention Parser │  │
│  │ (allowedAgents) │  │ (depth, invoked)│  │                │  │
│  └────────┬────────┘  └────────┬────────┘  └───────┬────────┘  │
│           │                    │                    │           │
│           ▼                    ▼                    ▼           │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              buildA2AInput()                             │   │
│  │  原始消息 + [Agent 已经分析了这个问题：] + 前序响应      │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│                              ▼                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              SpawnAgent()                                │   │
│  │  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐  │   │
│  │  │ Claude CLI  │  │ OpenCode     │  │ Other Adapters │  │   │
│  │  │ Adapter     │  │ Adapter      │  │                │  │   │
│  │  └─────────────┘  └──────────────┘  └────────────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## A2A 触发机制对比

### Mention 解析规则

**两者共同规则（参考 clowder-ai F046）：**

1. **行首检测**：只有行首（可带空白缩进）的 `@mention` 才触发路由
2. **Token Boundary**：避免 `@opus-45` 误命中 `@opus`
3. **长匹配优先**：按 pattern 长度降序匹配
4. **自调用过滤**：不触发自己
5. **数量限制**：单条消息最多触发 2 个 Agent

**代码对比：**

| 特性 | clowder-ai | ISDP |
|------|-----------|------|
| 行首检测 | `leadingWs = rawLine.match(/^\s*/)?.[0].length` | `leadingWs := countLeadingWhitespace(line)` |
| Token Boundary | `TOKEN_BOUNDARY_RE = /[\s,.:;!?()[\]{}<>，。！？、：；（）【】《》「」『』〈〉]/` | `tokenBoundaryRegex = regexp.MustCompile(...)` |
| 长匹配优先 | `entries.sort((a, b) => b.pattern.length - a.pattern.length)` | `sort.Slice(patterns, func(i, j int) bool { return len(patterns[i].Pattern) > len(patterns[j].Pattern) })` |
| 最大数量 | `MAX_A2A_MENTION_TARGETS = 2` | `MaxA2AMentionTargets = 2` |

### 路由匹配机制

**clowder-ai：动态 Worklist 模式**

```typescript
// route-serial.ts
const worklist = [...targetCats];
const worklistEntry = registerWorklist(threadId, worklist, maxDepth, options.parentInvocationId);

// 动态扩展：A2A mention 可追加到 worklist
while (index < worklist.length) {
  const catId = worklist[index]!;
  // ... 执行 cat ...
  
  // 检查 A2A mention
  const mentions = parseA2AMentions(response, catId);
  for (const targetId of mentions) {
    if (!worklist.includes(targetId) && a2aCount < maxDepth) {
      worklist.push(targetId);
      worklistEntry.a2aFrom.set(targetId, catId); // 记录谁触发的
    }
  }
}
```

**特点：**
- 使用 `WorklistRegistry` 管理动态扩展的调用队列
- 支持 `a2aFrom` 映射，下游 Agent 知道是谁 @ 的自己
- 深度限制：`MAX_A2A_DEPTH = 15`（环境变量可配置）

**ISDP：预定义 AllowedAgents 模式**

```go
// execution_service.go
func (es *ExecutionService) checkRouting(ctx context.Context, threadID uuid.UUID, currentConfig *model.AgentRoleConfig, output string) {
    mentionIDs := es.parseMentions(output)
    
    // 获取允许路由的 Agent（从 WorkflowTemplate）
    agentMap := es.getAllowedAgentsMap(ctx, threadID)
    
    for _, agentID := range mentionIDs {
        targetConfig, exists := agentMap[agentID]
        if !exists {
            // 目标不在工作流团队中，拒绝路由
            continue
        }
        
        // 检查深度限制
        if a2aCtx.Depth >= MaxA2ADepth {
            continue
        }
        
        // 触发 Agent
        es.SpawnAgent(ctx, &SpawnRequest{...})
    }
}
```

**特点：**
- `allowedAgents` 从 `WorkflowTemplate` 预先获取
- 使用 `a2aContexts` map 管理 A2A 状态（深度、已调用 Agent）
- 深度限制：`MaxA2ADepth = 10`

### 核心差异

| 维度 | clowder-ai | ISDP |
|------|-----------|------|
| 路由范围 | 全局 Cat Registry | Workflow 定义的 allowedAgents |
| 动态扩展 | 支持（worklist 动态 push） | 不支持（只能路由到预定义 Agent） |
| 触发者追踪 | `a2aFrom` map | 无 |
| 深度限制 | 15（可配置） | 10（硬编码） |
| 并发支持 | Worklist + parentInvocationId 隔离 | 简单 mutex |

---

## 上下文构建对比

### clowder-ai：多层上下文组装

**ContextAssembler + 前序响应格式化：**

```typescript
// route-serial.ts
let prompt = message;
if (!incrementalMode && previousResponses.length > 0) {
  const contextParts = previousResponses.map((r) => `[${r.catId} responded: ${r.content}]`);
  prompt = `${message}\n\n${contextParts.join('\n')}`;
}

// assembleContext() 处理 token 预算
const contextBudget = getCatContextBudget(catId);
const assembledContext = await assembleContext({
  threadId,
  catId,
  budget: contextBudget,
  history,
  // ...
});
```

**上下文注入组件：**

| 组件 | 来源 | 说明 |
|------|------|------|
| `message` | 用户输入 | 原始用户消息 |
| `previousResponses` | 前序 Agent 输出 | 格式化为 `[catId responded: content]` |
| `staticIdentity` | CatConfig | 身份、性格、协作规则、MCP 工具文档 |
| `invocationContext` | 运行时 | 队友、模式、链位置、路由策略、SOP 阶段 |
| `packBlocks` | Pack 系统 | 技能包、工作流、约束注入 |
| `directMessageFrom` | A2A 触发者 | "Direct message from X; reply to X" |
| `activeSignals` | 信号系统 | 关联的 Signal 文章 |
| `missionPrefix` | 外部项目 | dispatch 上下文 |

### ISDP：简单拼接模式

```go
// execution_service.go
func (es *ExecutionService) buildA2AInput(ctx context.Context, threadID uuid.UUID, fromAgent *model.AgentRoleConfig, output string) string {
    // 1. 获取原始用户消息
    originalMessage := es.getLastUserMessage(ctx, threadID)

    // 2. 移除纯 @mention 行
    strippedOutput := es.stripPureMentionLines(output)

    // 3. 构建格式化输入
    var sb strings.Builder

    if originalMessage != "" {
        sb.WriteString(originalMessage)
        sb.WriteString("\n\n---\n\n")
    }

    sb.WriteString(fmt.Sprintf("[%s 已经分析了这个问题：]\n\n", fromAgent.Name))
    sb.WriteString(strippedOutput)

    return sb.String()
}
```

**输出格式示例：**

```
用户原始消息

---

[Developer 已经分析了这个问题：]

分析内容...（已移除纯 @mention 行）
```

### 核心差异

| 维度 | clowder-ai | ISDP |
|------|-----------|------|
| Token 预算 | 有（`getCatContextBudget`） | 无 |
| 前序响应格式 | `[catId responded: content]` | `[Agent 已经分析了这个问题：]` |
| 上下文压缩 | 支持（`assembleIncrementalContext`） | 不支持 |
| 信号/任务注入 | 支持 | 不支持 |
| A2A 触发者信息 | "Direct message from X" | 仅 Agent 名字 |
| Pack 插件系统 | 支持 | 不支持 |

---

## 系统提示构建对比

### clowder-ai：静态身份 + 动态上下文

**SystemPromptBuilder 结构：**

```typescript
// SystemPromptBuilder.ts

// 静态身份（session 级别，--append-system-prompt）
export function buildStaticIdentity(catId: CatId, options?: StaticIdentityOptions): string {
  const lines: string[] = [];
  
  // 1. 身份
  lines.push(`你是 ${nameLabel}，由 ${providerLabel} 提供的 AI 猫猫。`);
  lines.push(`角色：${config.roleDescription}`);
  lines.push(`性格：${config.personality}`);
  
  // 2. Pack Masks（角色覆盖）
  if (options?.packBlocks?.masksBlock) {
    lines.push(options.packBlocks.masksBlock);
  }
  
  // 3. 协作格式
  lines.push(`你可以 @队友: ${callableMentions.join(' / ')}`);
  lines.push('格式：另起一行行首写 @猫名');
  
  // 4. 队友名册
  lines.push(buildTeammateRoster(catId));
  
  // 5. 工作流触发
  lines.push(WORKFLOW_TRIGGERS[config.breedId]);
  
  // 6. Pack 工作流
  if (packBlocks?.workflowsBlock) {
    lines.push(packBlocks.workflowsBlock);
  }
  
  // 7. 铲屎官引用
  lines.push(`${ccName}（铲屎官/CVO）。重要决策由${ccName}拍板。`);
  
  // 8. L0 治理规则
  lines.push(GOVERNANCE_L0_DIGEST);
  
  // 9. Pack 约束
  if (packBlocks?.guardrailBlock) {
    lines.push(packBlocks.guardrailBlock);
  }
  
  // 10. MCP 工具文档（仅 Claude）
  if (options?.mcpAvailable) {
    lines.push(MCP_TOOLS_SECTION);
  }
  
  return lines.join('\n');
}

// 动态上下文（每次调用）
export function buildInvocationContext(context: InvocationContext): string {
  const lines: string[] = [];
  
  // 1. 身份常量（防压缩）
  lines.push(`Identity: ${config.displayName} (@${context.catId}, model=${runtimeModel})`);
  
  // 2. A2A 直接消息回复目标
  if (context.directMessageFrom) {
    lines.push(`Direct message from ${fromLabel}; reply to ${fromLabel}`);
  }
  
  // 3. 队友列表
  if (context.teammates.length > 0) {
    lines.push('你的队友：');
    for (const id of context.teammates) {
      lines.push(`- ${tmName}：${c.roleDescription}`);
    }
  }
  
  // 4. 模式上下文
  if (context.mode === 'serial') {
    lines.push(`当前模式：你是第 ${context.chainIndex}/${context.chainTotal} 只被召唤的猫`);
  }
  
  // 5. A2A 出口检查
  if (context.a2aEnabled) {
    lines.push('A2A 出口检查：回复前问"到我这里结束了吗？"');
  }
  
  // 6. 路由策略提示
  if (context.routingPolicy) {
    lines.push(`Routing: ${parts.join('; ')}`);
  }
  
  // 7. SOP 阶段提示
  if (context.sopStageHint) {
    lines.push(`SOP: ${featureId} stage=${stage}`);
  }
  
  // 8. Voice Mode
  if (context.voiceMode) {
    lines.push('Voice Mode ON: 铲屎官正在语音陪伴模式');
  }
  
  // 9. Bootcamp Mode
  if (context.bootcampState) {
    lines.push(`Bootcamp Mode: phase=${phase}`);
  }
  
  return lines.join('\n');
}
```

**注入位置：**

```typescript
// invoke-single-cat.ts
const effectivePrompt =
  injectSystemPrompt && params.systemPrompt
    ? `${params.systemPrompt}\n\n---\n\n${promptWithMission}`
    : promptWithMission;
```

### ISDP：简单系统提示

ISDP 的系统提示构建相对简单，主要通过 Agent 配置的 `SystemPrompt` 字段注入。

**关键代码：**

```go
// AgentRoleConfig 模型
type AgentRoleConfig struct {
    ID           uuid.UUID `json:"id"`
    Name         string    `json:"name"`
    Role         string    `json:"role"`
    SystemPrompt string    `json:"systemPrompt"` // 系统提示
    // ...
}
```

### 核心差异

| 维度 | clowder-ai | ISDP |
|------|-----------|------|
| 静态身份 | 完整（身份+性格+协作+治理） | 简单（配置字段） |
| 动态上下文 | 丰富（队友+模式+策略+SOP） | 无 |
| Pack 插件 | 支持（masks+workflows+guardrails） | 不支持 |
| A2A 格式指导 | 明确（行首 @ + 格式说明） | 无 |
| 队友名册 | 自动生成（擅长+注意事项） | 无 |
| 治理规则 | 内置 L0 摘要 | 无 |
| 压缩恢复 | 支持重新注入 | 不支持 |

---

## 状态管理对比

### clowder-ai：Session Chain + Seal 机制

**Session Chain 架构：**

```typescript
// SessionChainStore
interface SessionRecord {
  id: string;              // session ID
  cliSessionId: string;    // CLI session ID
  threadId: string;
  catId: CatId;
  userId: string;
  status: 'active' | 'sealed' | 'sealing';
  seq: number;             // session 序号
  messageCount: number;    // 消息计数
  contextHealth?: ContextHealth;  // 上下文健康度
  compressionCount: number; // 压缩次数
  sealedAt?: number;
  sealReason?: string;
}
```

**Seal 机制（上下文压缩/归档）：**

```typescript
// invoke-single-cat.ts
// 检测上下文健康度
const health: ContextHealth = {
  usedTokens,
  windowTokens: windowSize,
  fillRatio: Math.min(usedTokens / windowSize, 1.0),
  source,
  measuredAt: Date.now(),
};

// 策略驱动的 seal 决策
const action = shouldTakeAction(
  health.fillRatio,
  health.windowTokens,
  health.usedTokens,
  activeRecord?.compressionCount ?? 0,
  strategy,
);

switch (action.type) {
  case 'seal':
    const sealResult = await deps.sessionSealer.requestSeal({
      sessionId: activeRecord.id,
      reason: action.reason,
    });
    // 写入 transcript + digest
    deps.sessionSealer.finalize({ sessionId: activeRecord.id });
    break;
  case 'warn':
    // 发送警告
    break;
}
```

**压缩检测（非 Claude Provider）：**

```typescript
// 检测上下文压缩
const cKey = `${userId}:${catId}:${threadId}`;
const prevFill = _prevContextFill.get(cKey);
_prevContextFill.set(cKey, usedTokens);
if (prevFill && usedTokens < prevFill * 0.4) {
  // token 数量骤降 >60%，说明发生了压缩
  _needsReinjection.add(cKey);
}
```

**A2A 状态管理：**

```typescript
// WorklistRegistry
interface WorklistEntry {
  cats: CatId[];
  a2aCount: number;           // A2A 深度计数
  maxDepth: number;           // 最大深度
  executedIndex: number;      // 已执行索引
  a2aFrom: Map<CatId, CatId>; // 触发者映射
  a2aTriggerMessageId: Map<CatId, string>; // 触发消息 ID
}
```

### ISDP：简单 A2A Context

```go
// execution_service.go
type A2AContext struct {
    Depth          int                // 当前深度
    InvokedAgents  map[uuid.UUID]bool // 已调用的 Agent
    OriginalSender uuid.UUID          // 原始发起者
}

// a2aContexts map 管理
es.a2aContexts[threadID] = &A2AContext{
    Depth:          0,
    InvokedAgents:  make(map[uuid.UUID]bool),
    OriginalSender: currentConfig.ID,
}

// 深度检查
if a2aCtx.Depth >= MaxA2ADepth {
    logInfo("A2A 深度达到上限，停止路由")
    continue
}

// 已调用检查
if a2aCtx.InvokedAgents[targetConfig.ID] {
    logInfo("Agent 已被调用过，跳过")
    continue
}
```

### 核心差异

| 维度 | clowder-ai | ISDP |
|------|-----------|------|
| Session 持久化 | 支持（Redis + Transcript） | 不支持 |
| 上下文健康检测 | 支持（fillRatio + 压缩检测） | 不支持 |
| Seal 机制 | 支持（transcript + digest 写入） | 不支持 |
| A2A 深度追踪 | 支持（a2aCount） | 支持（Depth） |
| 已调用追踪 | 无（worklist 去重） | 支持（InvokedAgents） |
| 触发者追踪 | 支持（a2aFrom） | 不支持 |
| 触发消息追踪 | 支持（a2aTriggerMessageId） | 不支持 |

---

## 关键差异总结

### 架构差异

| 维度 | clowder-ai | ISDP | 影响 |
|------|-----------|------|------|
| **语言** | TypeScript | Go | - |
| **路由范围** | 全局 Cat Registry | Workflow allowedAgents | clowder-ai 更灵活，ISDP 更安全 |
| **动态扩展** | 支持 worklist 动态 push | 不支持 | clowder-ai 支持更复杂的协作链 |
| **Pack 系统** | 支持（插件化技能/工作流） | 不支持 | clowder-ai 扩展性更强 |

### 功能差异

| 功能 | clowder-ai | ISDP | 说明 |
|------|-----------|------|------|
| Session 持久化 | ✅ | ❌ | clowder-ai 支持跨会话上下文 |
| 上下文压缩检测 | ✅ | ❌ | clowder-ai 可检测并恢复压缩 |
| Seal 机制 | ✅ | ❌ | clowder-ai 支持上下文归档 |
| 触发者追踪 | ✅ | ❌ | clowder-ai 下游知道谁 @ 的自己 |
| 博弈场景 | ✅（一个 pattern 多 Agent） | ✅ | 两者都支持 |

### 代码质量差异

| 维度 | clowder-ai | ISDP |
|------|-----------|------|
| 测试覆盖 | 完善（大量 .test.ts） | 基础（_test.go） |
| 文档规范 | 完善（F-xxx feature specs） | 简单（CLAUDE.md） |
| 错误处理 | 完善（分类 + 重试） | 基础 |
| 可配置性 | 高（环境变量 + 配置文件） | 中等 |

---

## 附录：核心代码对比

### A. Mention 解析对比

**clowder-ai (a2a-mentions.ts):**

```typescript
export function parseA2AMentions(text: string, currentCatId?: CatId): CatId[] {
  // 1. 剥离代码块
  const stripped = text.replace(/```[\s\S]*?```/g, '');

  // 2. 构建 patterns 并按长度排序
  const entries: MentionPatternEntry[] = [];
  for (const [id, config] of Object.entries(allConfigs)) {
    if (currentCatId && id === currentCatId) continue;
    for (const pattern of config.mentionPatterns) {
      entries.push({ catId: id, pattern: pattern.toLowerCase() });
    }
  }
  entries.sort((a, b) => b.pattern.length - a.pattern.length);

  // 3. 行首匹配 + token boundary
  const found: CatId[] = [];
  const lines = stripped.split(/\r?\n/);
  for (const rawLine of lines) {
    const leadingWs = rawLine.match(/^\s*/)?.[0].length ?? 0;
    const normalized = rawLine.slice(leadingWs).toLowerCase();
    if (!normalized.startsWith('@')) continue;

    for (const entry of entries) {
      if (!normalized.startsWith(entry.pattern)) continue;
      const charAfter = normalized[entry.pattern.length];
      const isBoundary = !charAfter || TOKEN_BOUNDARY_RE.test(charAfter);
      if (isBoundary) {
        found.push(entry.catId);
        break;
      }
    }
  }

  return found.slice(0, MAX_A2A_MENTION_TARGETS);
}
```

**ISDP (mention.go):**

```go
func ParseA2AMentionsMulti(text string, currentAgentID string, patterns []MentionPattern) [][]string {
  if text == "" {
    return nil
  }

  // 1. 剥离代码块
  stripped := codeBlockRegex.ReplaceAllString(text, "")

  // 2. 按长度降序排列
  sort.Slice(patterns, func(i, j int) bool {
    return len(patterns[i].Pattern) > len(patterns[j].Pattern)
  })

  // 3. 逐行解析
  lines := strings.Split(stripped, "\n")
  found := make([][]string, 0, MaxA2AMentionTargets)

  for _, line := range lines {
    leadingWs := countLeadingWhitespace(line)
    normalized := strings.ToLower(line[leadingWs:])

    if !strings.HasPrefix(normalized, "@") {
      continue
    }

    for _, entry := range patterns {
      patternLower := strings.ToLower(entry.Pattern)
      if !strings.HasPrefix(normalized, patternLower) {
        continue
      }

      // Token boundary 检查
      charAfter := ""
      if len(normalized) > len(patternLower) {
        charAfter = string(normalized[len(patternLower)])
      }
      isBoundary := charAfter == "" || tokenBoundaryRegex.MatchString(charAfter)

      if isBoundary {
        // 过滤自调用
        filtered := make([]string, 0)
        for _, agentID := range entry.AgentIDs {
          if agentID != currentAgentID {
            filtered = append(filtered, agentID)
          }
        }
        if len(filtered) > 0 {
          found = append(found, filtered)
        }
        break
      }
    }
  }

  return found
}
```

### B. 上下文构建对比

**clowder-ai 上下文组装：**

```typescript
// route-serial.ts
let prompt = message;
if (previousResponses.length > 0) {
  const contextParts = previousResponses.map((r) => `[${r.catId} responded: ${r.content}]`);
  prompt = `${message}\n\n${contextParts.join('\n')}`;
}

// SystemPromptBuilder.ts
export function buildSystemPrompt(context: InvocationContext): string {
  const staticPart = buildStaticIdentity(context.catId, {
    mcpAvailable: context.mcpAvailable,
    packBlocks: context.packBlocks,
  });
  const dynamicPart = buildInvocationContext(context);
  return `${staticPart}\n\n${dynamicPart}`;
}
```

**ISDP 上下文构建：**

```go
func (es *ExecutionService) buildA2AInput(ctx context.Context, threadID uuid.UUID, fromAgent *model.AgentRoleConfig, output string) string {
  originalMessage := es.getLastUserMessage(ctx, threadID)
  strippedOutput := es.stripPureMentionLines(output)

  var sb strings.Builder
  if originalMessage != "" {
    sb.WriteString(originalMessage)
    sb.WriteString("\n\n---\n\n")
  }
  sb.WriteString(fmt.Sprintf("[%s 已经分析了这个问题：]\n\n", fromAgent.Name))
  sb.WriteString(strippedOutput)

  return sb.String()
}
```

---

## 参考文件

### clowder-ai

- `packages/api/src/domains/cats/services/agents/routing/a2a-mentions.ts` - Mention 解析
- `packages/api/src/domains/cats/services/agents/routing/route-serial.ts` - 串行路由
- `packages/api/src/domains/cats/services/context/SystemPromptBuilder.ts` - 系统提示构建
- `packages/api/src/domains/cats/services/agents/invocation/invoke-single-cat.ts` - 单猫调用

### ISDP

- `isdp/internal/parser/mention.go` - Mention 解析
- `isdp/internal/service/agent/execution_service.go` - 执行服务（A2A 路由 + 上下文构建）
- `isdp/internal/service/a2a/mention_parser.go` - A2A 配置