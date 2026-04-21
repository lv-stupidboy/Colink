# clowder-ai A2A (Agent-to-Agent) 机制深度分析

## 概述

clowder-ai 是一个多智能体协作平台（猫咖系统），实现了完整的 A2A 机制，让多个 AI Agent（猫猫）能够通过 @mention 进行协作和任务交接。

## 核心设计原则

### 1. 行首 @mention 路由规则

**核心规则**：只有行首（可带空白缩进）的 @mention 才触发 A2A 路由。

```
[有效路由]
@布偶猫 请 review 这段代码     ✓
  @缅因猫 请确认这个修复       ✓
@codex\n下一个你！             ✓

[无效路由]
之前布偶猫说的 @布偶猫 方案不错  ✗ (行中 mention)
```

**设计理由**：
- 避免在引用/讨论历史时的意外触发
- 明确表达路由意图（新起一行 = 明确的交接动作）

### 2. Pattern → AgentID 直接映射

每个 Agent 配置中定义 `mentionPatterns` 字段：

```typescript
// packages/shared/src/types/cat.ts
interface CatConfig {
  readonly id: CatId;
  readonly mentionPatterns: readonly string[];
  // ...
}

// 示例配置
opus: {
  mentionPatterns: ['@opus', '@布偶猫', '@布偶', '@ragdoll', '@宪宪'],
}
codex: {
  mentionPatterns: ['@codex', '@缅因猫', '@缅因', '@maine', '@砚砚'],
}
```

**匹配策略**：
- 最长匹配优先（避免 `@opus-45` 误命中 `@opus`）
- Token boundary 检测（后面的字符必须是分隔符或结束）

### 3. A2A 输入格式（关键设计）

**核心原则**：不要传递上游 Agent 的原始输出（包含 @mention）

应该传递：
1. **原始用户消息**
2. **格式化的前序响应上下文**（而非原始输出）

这避免了下游 Agent 再次触发相同的 @mention（递归问题）。

## 核心架构组件

### 1. A2A Mention 解析器

**文件**：`packages/api/src/domains/cats/services/agents/routing/a2a-mentions.ts`

```typescript
export function parseA2AMentions(text: string, currentCatId?: CatId): CatId[] {
  // 1. 剥离围栏代码块 (```...```)
  const stripped = text.replace(/```[\s\S]*?```/g, '');

  // 2. 从 catRegistry 获取所有 Agent 的 mentionPatterns
  const allConfigs = catRegistry.getAllConfigs();

  // 3. 构建 patterns 并按长度降序排序（最长优先）
  const entries = [];
  for (const [id, config] of Object.entries(allConfigs)) {
    if (currentCatId && id === currentCatId) continue; // 过滤自调用
    for (const pattern of config.mentionPatterns) {
      entries.push({ catId: id, pattern: pattern.toLowerCase() });
    }
  }
  entries.sort((a, b) => b.pattern.length - a.pattern.length);

  // 4. 行首匹配 + token boundary 检测
  const found = [];
  const lines = stripped.split(/\r?\n/);
  for (const line of lines) {
    const leadingWs = line.match(/^\s*/)?.[0].length ?? 0;
    const normalized = line.slice(leadingWs).toLowerCase();
    if (!normalized.startsWith('@')) continue;

    for (const entry of entries) {
      if (!normalized.startsWith(entry.pattern)) continue;
      const charAfter = normalized[entry.pattern.length];
      const isBoundary = !charAfter || TOKEN_BOUNDARY_RE.test(charAfter);
      if (isBoundary && !found.includes(entry.catId)) {
        found.push(entry.catId);
      }
      break; // longest-match-first
    }
  }

  return found.slice(0, MAX_A2A_MENTION_TARGETS); // 安全上限 2 个
}
```

**安全限制**：
- `MAX_A2A_MENTION_TARGETS = 2`：单条消息最多触发 2 个 Agent
- `MAX_A2A_DEPTH = 15`：最大链式深度（可通过 `MAX_A2A_DEPTH` 环境变量配置）

### 2. WorklistRegistry（工作列表注册器）

**文件**：`packages/api/src/domains/cats/services/agents/routing/WorklistRegistry.ts`

**核心功能**：统一 A2A 路径，避免双路径问题

```typescript
interface WorklistEntry {
  list: CatId[];              // 待执行的 Agent 列表
  originalCount: number;      // 用户最初选择的 Agent 数量
  a2aCount: number;           // A2A 深度计数
  maxDepth: number;           // 最大允许深度
  executedIndex: number;      // 当前执行到哪个 Agent
  a2aFrom: Map<CatId, CatId>; // 记录每个 Agent 是被谁 @mention 的
  a2aTriggerMessageId: Map<CatId, string>; // 触发消息 ID（用于 replyTo）
}

// 注册工作列表
export function registerWorklist(threadId: string, worklist: CatId[], maxDepth: number): WorklistEntry;

// 推送新的 A2A 目标（由 callback 或 text-scan 触发）
export function pushToWorklist(threadId: string, cats: CatId[], callerCatId?: CatId): PushResult;

// 取消注册
export function unregisterWorklist(threadId: string, owner?: WorklistEntry): void;
```

**设计亮点**：
- 原始用户目标保持回复用户（不被 A2A sender 覆盖）
- 已执行的 Agent 可以被重新入队（支持 A→B→A 的 ping-pong）
- Caller guard：只有当前正在执行的 Agent 才能推送新目标

### 3. routeSerial（串行路由策略）

**文件**：`packages/api/src/domains/cats/services/agents/routing/route-serial.ts`

**核心流程**：

```typescript
export async function* routeSerial(deps, targetCats, message, userId, threadId, options) {
  // 1. 初始化工作列表
  const worklist = [...targetCats];
  const worklistEntry = registerWorklist(threadId, worklist, maxDepth);

  let index = 0;
  while (index < worklist.length) {
    const catId = worklist[index];

    // 2. 构建当前 Agent 的 prompt
    const invocationContext = buildInvocationContext({
      catId,
      mode: worklist.length > 1 ? 'serial' : 'independent',
      chainIndex: index + 1,
      chainTotal: worklist.length,
      teammates: worklist.filter(id => id !== catId),
      a2aEnabled: worklistEntry.a2aCount < maxDepth,
      directMessageFrom: worklistEntry.a2aFrom.get(catId), // A2A 来源
    });

    // 3. 执行 Agent
    for await (const msg of invokeSingleCat(deps, {...})) {
      // 收集响应文本
      if (msg.type === 'text') textContent += msg.content;
      yield msg;
    }

    // 4. A2A mention 检测（响应完成后）
    const a2aMentions = parseA2AMentions(textContent, catId);

    // 5. 扩展工作列表
    if (a2aMentions.length > 0 && worklistEntry.a2aCount < maxDepth) {
      for (const nextCat of a2aMentions) {
        if (!worklist.slice(index + 1).includes(nextCat)) {
          worklist.push(nextCat);
          worklistEntry.a2aCount++;
          worklistEntry.a2aFrom.set(nextCat, catId);
        }
      }
    }

    // 6. 发送 a2a_handoff 事件（前端显示）
    for (const newCat of worklist.slice(handoffEmitted)) {
      yield { type: 'a2a_handoff', catId, content: `${catId} → ${newCat}` };
    }

    index++;
  }

  // 7. 清理
  unregisterWorklist(threadId, worklistEntry);
}
```

### 4. SystemPromptBuilder（系统提示构建器）

**文件**：`packages/api/src/domains/cats/services/context/SystemPromptBuilder.ts`

**A2A 相关注入**：

```typescript
// 静态身份（session 级别，注入一次）
export function buildStaticIdentity(catId: CatId, options) {
  // ...
  
  // A2A 协作格式
  const { mentions: callableMentions } = buildCallableMentions(catId);
  lines.push('## 协作');
  lines.push(`你可以 @队友: ${callableMentions.join(' / ')}`);
  lines.push('格式：另起一行行首写 @猫名（行中无效，多猫各占一行）');
  
  // 队友名册
  const rosterLines = buildTeammateRoster(catId);
  
  // 工作流触发点
  const triggers = WORKFLOW_TRIGGERS[config.breedId];
  lines.push(triggers); // 如：完成开发 → @缅因猫 请 review
  
  return lines.join('\n');
}

// 动态上下文（每次调用）
export function buildInvocationContext(context: InvocationContext) {
  // ...
  
  // A2A 直消息回复目标
  if (context.directMessageFrom) {
    lines.push(`Direct message from ${fromLabel}; reply to ${fromLabel}`);
  }
  
  // 队友列表（当前参与此调用的）
  if (context.teammates.length > 0) {
    lines.push('你的队友：');
    for (const id of context.teammates) {
      lines.push(`- ${getConfig(id).displayName}: ${getConfig(id).roleDescription}`);
    }
  }
  
  // 链式上下文
  if (context.mode === 'serial') {
    lines.push(`当前模式：你是第 ${chainIndex}/${chainTotal} 只被召唤的猫`);
  }
  
  // A2A 出口检查提醒
  if (context.a2aEnabled) {
    lines.push('A2A 出口检查：回复前问"到我这里结束了吗？"');
  }
  
  return lines.join('\n');
}
```

### 5. callback-a2a-trigger（MCP 回调触发器）

**文件**：`packages/api/src/routes/callback-a2a-trigger.ts`

**功能**：处理 MCP `post_message` 工具中的 @mention

```typescript
export async function enqueueA2ATargets(deps, opts) {
  const { targetCats, threadId, callerCatId, triggerMessageId } = opts;

  // F122B: 优先使用 InvocationQueue（统一调度）
  if (deps.invocationQueue) {
    for (const catId of targetCats) {
      // 深度限制
      if (deps.invocationQueue.countAgentEntriesForThread(threadId) >= MAX_A2A_DEPTH) break;
      
      // 重复检测
      if (deps.invocationQueue.hasQueuedAgentForCat(threadId, catId)) continue;
      
      // 入队
      deps.invocationQueue.enqueue({
        threadId, userId, content, source: 'agent',
        targetCats: [catId], intent: 'execute', autoExecute: true,
        callerCatId,
      });
    }
    return { enqueued, fallback: false };
  }

  // Legacy path: 推送到父工作列表
  if (hasWorklist(threadId)) {
    const pushResult = pushToWorklist(threadId, targetCats, callerCatId, parentInvocationId, triggerMessageId);
    return { enqueued: pushResult.added, fallback: false };
  }

  // Fallback: 独立启动（无父调用时）
  await triggerA2AInvocation(deps, opts);
  return { enqueued: targetCats, fallback: true };
}
```

### 6. AgentRouter（Agent 路由器）

**文件**：`packages/api/src/domains/cats/services/agents/routing/AgentRouter.ts`

**核心功能**：
- 解析用户消息中的 @mention
- 路由到目标 Agent
- 管理线程参与者

```typescript
export class AgentRouter {
  // 解析 @mentions（用户消息入口）
  private async parseAllMentions(message: string, threadId: string): Promise<CatId[]> {
    // 群组 mention 优先 (@all, @全体, @thread, @全体参与者)
    const groupResult = await this.parseGroupMentions(message, threadId);
    if (groupResult !== null) return groupResult;
    
    // 个人 mention
    return this.parseMentions(message);
  }

  // 解析个人 @mentions
  private parseMentions(message: string): CatId[] {
    const lowerMessage = this.normalizeSpeechMentions(message).toLowerCase();
    
    // 收集所有 patterns，按长度降序
    const allPatterns = [];
    for (const config of catRegistry.getAllConfigs()) {
      for (const pattern of config.mentionPatterns) {
        allPatterns.push({ pattern: pattern.toLowerCase(), catId: config.id });
      }
    }
    allPatterns.sort((a, b) => b.pattern.length - a.pattern.length);

    // 匹配 + 消耗区间排除
    const consumed = [];
    const mentions = [];
    for (const { pattern, catId } of allPatterns) {
      let searchFrom = 0;
      while (searchFrom < lowerMessage.length) {
        const pos = lowerMessage.indexOf(pattern, searchFrom);
        if (pos === -1) break;
        
        const end = pos + pattern.length;
        const charAfter = lowerMessage[end];
        const isBoundary = !charAfter || BOUNDARY_RE.test(charAfter);
        const isConsumed = consumed.some(([s, e]) => pos >= s && pos < e);
        
        if (isBoundary && !isConsumed) {
          consumed.push([pos, end]);
          mentions.push({ catId, position: pos });
        }
        searchFrom = pos + 1;
      }
    }

    // 按首次出现排序
    return mentions.sort((a, b) => a.position - b.position).map(m => m.catId);
  }

  // 执行路由
  async *routeExecution(userId, message, threadId, userMessageId, targetCats, intent, options) {
    // 根据意图选择策略
    if (intent.intent === 'ideate' && targetCats.length > 1) {
      yield* routeParallel(...); // 并行独立思考
    } else {
      yield* routeSerial(...);   // 串行执行（支持 A2A 链）
    }
  }
}
```

### 7. ContextAssembler（上下文组装器）

**文件**：`packages/api/src/domains/cats/services/context/ContextAssembler.ts`

**功能**：为每个 Agent 组装历史上下文

```typescript
export function assembleContext(messages: StoredMessage[], options): AssembledContext {
  const maxMessages = options.maxMessages ?? 20;
  const maxContentLength = options.maxContentLength ?? 1500;
  const maxTotalTokens = options.maxTotalTokens ?? 2000;

  // 按时间取最近的 N 条
  const recent = messages.slice(-maxMessages);

  // 格式化每条消息
  const formatted = recent.map(m => formatMessage(m, { truncate: maxContentLength }));

  // Token 预算裁剪（从最近开始保留）
  let totalTokens = overheadTokens;
  let startIndex = formatted.length;
  for (let i = formatted.length - 1; i >= 0; i--) {
    if (totalTokens + estimateTokens(formatted[i]) > maxTotalTokens) break;
    totalTokens += estimateTokens(formatted[i]);
    startIndex = i;
  }

  return {
    contextText: `[对话历史 - 最近 ${included.length} 条]\n${included.join('\n')}\n[/对话历史]`,
    messageCount: included.length,
    estimatedTokens: totalTokens,
  };
}
```

## 数据流和调用链路

### 用户消息触发 A2A

```
用户: "@布偶猫 请分析这个架构"
      ↓
AgentRouter.parseAllMentions()
      → 解析出 ['opus']
      ↓
AgentRouter.routeExecution(targetCats=['opus'], intent='execute')
      ↓
routeSerial()
      → registerWorklist(['opus'])
      → invokeSingleCat(opus)
      → 收集 opus 响应
      → parseA2AMentions(opus响应)
      → 如果有 "@缅因猫 请 review" → pushToWorklist(['codex'])
      → invokeSingleCat(codex) (继续执行)
      ↓
前端收到:
  - opus 的 stream 输出
  - a2a_handoff: "布偶猫 → 缅因猫"
  - codex 的 stream 输出
```

### MCP post_message 触发 A2A

```
Agent (via MCP tool): cat_cafe_post_message("@codex 我完成了开发")
      ↓
callback-a2a-trigger.enqueueA2ATargets()
      → 检查 InvocationQueue / WorklistRegistry
      → pushToWorklist 或 enqueue
      ↓
routeSerial 继续:
      → 当前 opus 完成后
      → 执行 codex
      ↓
codex 收到的 prompt 包含:
      "Direct message from 布偶猫; reply to 布偶猫"
```

## 与 ISDP A2A 的对比要点

### clowder-ai 的优势

1. **清晰的行首检测规则**
   - 明确区分"讨论历史"和"路由意图"
   - 避免引用/复述时的意外触发

2. **统一的 WorklistRegistry**
   - 避免双路径问题（text-scan vs callback）
   - 共享 AbortController、深度限制
   - Caller guard 防止 preempted invocation 污染

3. **完整的 SystemPrompt 注入**
   - 静态身份（session 级别）
   - 动态上下文（每次调用）
   - A2A 协作指南、队友名册、工作流触发点

4. **ContextAssembler 历史组装**
   - Token 预算控制
   - 消息截断策略
   - 富块摘要（避免浪费 tokens）

### ISDP 可借鉴的实现

1. **行首 @mention 检测**
   ```go
   // internal/parser/mention.go
   func ParseA2AMentions(text string, currentCatId string) []string {
       lines := strings.Split(text, "\n")
       for _, line := range lines {
           trimmed := strings.TrimLeft(line, " \t")
           if !strings.HasPrefix(trimmed, "@") {
               continue
           }
           // 匹配 mention patterns...
       }
   }
   ```

2. **WorklistRegistry 模式**
   ```go
   // internal/service/mention/worklist.go
   type WorklistEntry struct {
       List           []string
       OriginalCount  int
       A2ACount       int
       MaxDepth       int
       ExecutedIndex  int
       A2AFrom        map[string]string // target → sender
   }
   ```

3. **buildA2AInput 函数**
   ```go
   // internal/service/agent/execution_service.go
   func buildA2AInput(originalMessage string, prevResponses []Response) string {
       // 不传递原始 Agent 输出
       // 而是传递: 原始用户消息 + 格式化的上下文
       parts := []string{originalMessage}
       for _, r := range prevResponses {
           parts = append(parts, fmt.Sprintf("[%s]: %s", r.CatId, sanitizeContent(r.Content)))
       }
       return strings.Join(parts, "\n\n---\n\n")
   }
   ```

4. **SystemPrompt 注入**
   ```go
   // internal/service/context/system_prompt.go
   func BuildInvocationContext(catId string, mode string, teammates []string, directMessageFrom string) string {
       lines := []string{}
       if directMessageFrom != "" {
           lines = append(lines, fmt.Sprintf("Direct message from %s; reply to %s", directMessageFrom, directMessageFrom))
       }
       if len(teammates) > 0 {
           lines = append(lines, "你的队友：")
           for _, id := range teammates {
               lines = append(lines, fmt.Sprintf("- %s: %s", id, getRoleDescription(id)))
           }
       }
       if mode == "serial" {
           lines = append(lines, "A2A 出口检查：回复前问'到我这里结束了吗？'")
       }
       return strings.Join(lines, "\n")
   }
   ```

## 关键文件路径汇总

| 文件 | 职责 |
|------|------|
| `a2a-mentions.ts` | @mention 解析，行首检测，token boundary |
| `WorklistRegistry.ts` | 工作列表管理，A2A 目标入队，caller guard |
| `route-serial.ts` | 串行路由策略，A2A 式调用，响应收集 |
| `AgentRouter.ts` | 用户消息解析，路由决策，意图识别 |
| `SystemPromptBuilder.ts` | 系统提示构建，A2A 协作指南，队友名册 |
| `ContextAssembler.ts` | 历史上下文组装，token 预算控制 |
| `callback-a2a-trigger.ts` | MCP 回调处理，A2A 目标入队（callback path） |
| `invoke-single-cat.ts` | 单 Agent 调用，CLI 执行，session 管理 |
| `cat.ts` (shared) | CatConfig 定义，mentionPatterns 字段 |

## 总结

clowder-ai 的 A2A 机制是一个完整、成熟的多 Agent 协作实现：

1. **设计清晰**：行首 @mention = 路由意图，避免意外触发
2. **架构统一**：WorklistRegistry 避免 dual-path 问题
3. **上下文完整**：SystemPrompt 注入协作指南、队友信息
4. **安全可控**：深度限制、目标上限、caller guard

ISDP 项目可以借鉴这些设计原则，特别是：
- 行首检测规则
- WorklistRegistry 模式
- buildA2AInput 正确构造输入
- SystemPrompt A2A 注入