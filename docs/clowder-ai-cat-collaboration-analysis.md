




#   Clowder-AI 猫猫协作机制分析

> 分析日期: 2026-03-31
> 项目路径: D:\Tools\catCoffee\clowder-ai
> 目的: 学习参考多 Agent 协作模式，为 ISDP A2A 系统提供设计思路

---

## 一、猫猫交互的三种方式

Clowder-AI 实现了三种猫猫交互模式，各有不同适用场景和限制：

### 1. 文本 @mention（行首解析）

**特点**：
- 解析逻辑在 `packages/api/src/domains/cats/services/agents/routing/a2a-mentions.ts`
- 只识别**行首**的 @mention（可带空白缩进），行中无效
- 最大 2 个目标猫
- 触发串行执行链（worklist）

**代码证据**：
```typescript
// a2a-mentions.ts
const MAX_A2A_MENTION_TARGETS = 2;

export function parseA2AMentions(text: string, currentCatId?: CatId): CatId[] {
  // 只匹配行首 @mention
  const regex = /^\s*@(\w+)/gm;
  ...
}
```

**示例**：
```
@布偶猫 请帮我分析这个接口设计
```

### 2. targetCats（结构化 MCP 字段）

**特点**：
- 通过 MCP 工具 `post_message` 的 `targetCats` 参数传递
- 绕过文本解析，直接路由到指定猫
- 优先级高于文本 @mention
- 设计文档：`docs/features/F055-a2a-mcp-structured-routing.md`

**代码证据**：
```typescript
// callbacks.ts - post_message 处理
if (body.targetCats && body.targetCats.length > 0) {
  // targetCats 优先，跳过文本解析
  const targetCatIds = body.targetCats.map(c => createCatId(c));
  // 直接路由到指定猫
}
```

**适用场景**：
- Agent 明确知道要路由给哪个猫
- 需要传递结构化上下文
- 避免 @mention 文本污染输出

### 3. multi_mention（正式多猫讨论）

**特点**：
- 专门的 MCP 工具：`cat_cafe_multi_mention`
- 最大 3 个目标猫（并行）
- 有状态机管理：pending → running → partial/done/timeout/failed
- 必须带搜索证据（`searchEvidenceRefs`）或跳过理由（`overrideReason`）
- 超时机制：默认 8 分钟
- 结果聚合：汇总所有猫的回答后回调给发起者

**代码证据**：
```typescript
// multi-mention.ts (shared types)
export type MultiMentionStatus = 'pending' | 'running' | 'partial' | 'done' | 'timeout' | 'failed';
export const MAX_MULTI_MENTION_TARGETS = 3;
export const DEFAULT_TIMEOUT_MINUTES = 8;
```

---

## 二、multi_mention 完整机制详解

### 2.1 触发时机（5 种场景）

来自 `shared-rules.md` §13 元思考触发器：

| 触发器 | 场景 | 默认动作 |
|--------|------|---------|
| **A: 高影响决策** | 架构选型、API 契约、跨模块改动 | 先搜 docs/decisions/ → 再决定是否 multi_mention |
| **B: 跨领域问题** | 涉及前端/安全/性能/UX 等非自身专长 | 先搜对应领域文档 → 再 @ 对应领域的猫 |
| **C: 高不确定性** | 方案不确定、多种选择难以取舍 | 先搜历史讨论 → 再拉猫获取多视角 |
| **D: 信息不足** | 发现自己对上下文了解不够 | 先 search → 再问人 |
| **E: 新领域侦查** | 要写新代码/MCP/集成时 | 先从 feats 顺藤摸瓜 → 再动手 |

### 2.2 调用流程

```
MCP 工具调用 (cat_cafe_multi_mention)
    ↓
POST /api/callbacks/multi-mention
    ↓
MultiMentionOrchestrator.create()
    → 生成 requestId，状态 pending
    ↓
Orchestrator.start()
    → 状态变为 running
    ↓
scheduleTimeout() (默认 8 分钟)
    ↓
并行 dispatch 到各目标猫
    ↓ (两种路径)
┌─────────────────────────────────────────────────┐
│  Path 1: InvocationQueue (F122B B6)             │
│  - 入队 agent entry                             │
│  - registerEntryCompleteHook 监听完成            │
│  - tryAutoExecute 触发自动执行                   │
└─────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────┐
│  Path 2: Legacy Direct Dispatch                 │
│  - routeExecution 直接执行                       │
│  - InvocationTracker 占槽防冲突                  │
│  - 收集 responseText + toolsUsed                 │
└─────────────────────────────────────────────────┘
    ↓
各猫独立响应
    ↓
Orchestrator.recordResponse()
    → 状态可能变为 partial/done
    ↓
所有猫响应完成 → cancelTimeout()
    ↓
flushResult() 汇聚结果
    → 构建汇总消息
    → 存入 messageStore
    → WebSocket 广播
```

### 2.3 依赖信息

调用 `cat_cafe_multi_mention` 时必须携带：

```typescript
{
  invocationId: string;      // 当前调用的 invocation ID
  callbackToken: string;     // 认证令牌
  targets: string[];         // 目标猫 ID (1-3 个)
  question: string;          // 问题内容
  callbackTo: string;        // 回调给谁（发起者）
  context?: string;          // 附加上下文
  timeoutMinutes?: number;   // 超时时间 (默认 8)
  searchEvidenceRefs?: string[];  // 搜索证据（必须）
  overrideReason?: string;   // 跳过搜索的理由
}
```

### 2.4 状态机

```typescript
// MultiMentionOrchestrator.ts
export class MultiMentionOrchestrator {
  private requests = new Map<string, MultiMentionRequest>();

  create(params: MultiMentionCreateParams): MultiMentionRequest {
    // 创建请求，状态 pending
    // 检查幂等性（idempotencyKey）
  }

  start(requestId: string): void {
    // 状态变为 running
  }

  recordResponse(requestId: string, catId: CatId, content: string): MultiMentionStatus {
    // 记录单个猫的响应
    // 返回新状态：partial（部分完成）或 done（全部完成）
  }

  handleTimeout(requestId: string): void {
    // 超时处理，未响应的猫标记为 timeout
    // 触发 flushResult
  }

  getResult(requestId: string): MultiMentionResult {
    // 获取完整结果
  }

  isActiveTarget(threadId: string, catId: CatId): boolean {
    // 反级联保护：检查该猫是否是某个 active multi-mention 的目标
  }
}
```

### 2.5 反级联保护

防止被召唤的猫再次发起 multi_mention，造成级联爆炸：

```typescript
// callback-multi-mention-routes.ts:453-458
if (orch.isActiveTarget(record.threadId, callerCatId)) {
  return reply.status(409).send({
    error: 'Anti-cascade: caller is an active multi-mention target',
    hint: 'Cannot create multi-mention while responding to one',
  });
}
```

---

## 三、猫猫如何知道何时触发多猫沟通

### 三层引导机制

```
┌─────────────────────────────────────────────────────────────┐
│  Level 1: GOVERNANCE_L0_DIGEST (静态身份注入)                │
│  → §13 五触发器（A-E），所有猫都能看到                        │
│  → "先搜后问"原则 + MCP 硬检查提醒                           │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  Level 2: WORKFLOW_TRIGGERS (品种级 SOP)                     │
│  → 布偶猫: "完成开发 → @缅因猫 review"                        │
│  → 缅因猫: "完成 review → @布偶猫 通知"                       │
│  → 状态迁移点的具体触发指引                                   │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  Level 3: Skills (详细流程指南)                              │
│  → collaborative-thinking Mode B                             │
│  → 独立思考 6 阶段流程 + 成本警告 + 防锚定规则                │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  Level 4: MCP Layer Hard Gate                               │
│  → searchEvidenceRefs 或 overrideReason 必须存在            │
│  → 缺少 = 拒绝调用                                           │
└─────────────────────────────────────────────────────────────┘
```

### Level 1: GOVERNANCE_L0_DIGEST

**代码位置**: `SystemPromptBuilder.ts:238-251`

```typescript
const GOVERNANCE_L0_DIGEST = `## 家规（shared-rules.md）
...
调用 cat_cafe_multi_mention 前，**必须先搜后问**（MCP 层硬检查：缺少 searchEvidenceRefs 且无 overrideReason → 拒绝调用）。
...`;
```

这是从 `shared-rules.md` 编译的紧凑摘要，注入到**每只猫的静态身份**中。核心内容：
- 五触发器（A-E）
- 先搜后问原则
- MCP 硬检查提醒

### Level 2: WORKFLOW_TRIGGERS

**代码位置**: `SystemPromptBuilder.ts:255-287`

```typescript
const WORKFLOW_TRIGGERS: Record<string, string> = {
  ragdoll: [
    '## 工作流（主动 @ 触发点）',
    '- 完成开发/修复 → @缅因猫 请 review',
    '- 修完 review 意见 → @缅因猫 确认修复',
    '- 遇到视觉/体验问题 → @暹罗猫 征询',
  ].join('\n'),
  'maine-coon': [
    '## 工作流（主动 @ 触发点）',
    '- 完成 review → @布偶猫 通知结果',
    '- 讨论/独立思考完成，结论需要其他猫跟进 → @ 对应猫',
  ].join('\n'),
  siamese: [...],
};
```

按 breedId 注入品种级 SOP 触发点，告诉猫猫在什么状态迁移时应该主动 @ 其他猫。

### Level 3: Skills 框架

**文件位置**: `cat-cafe-skills/collaborative-thinking/SKILL.md`

提供详细的 Mode B（多猫独立思考）流程：

```markdown
## Mode B: 多猫独立思考

**何时启动 Mode B？** 参见 `shared-rules.md` §13 元思考触发器 A-D。
调 `cat_cafe_multi_mention` 前必须带搜索证据。

**⚠️ 成本警告**：Swarm token 消耗是单猫 N 倍。实现细节不值得开 swarm。

**6 阶段流程**：
Phase 1: 独立思考（并行，禁止互看）
Phase 2: 串行讨论（有分歧才触发，限 2-3 轮）
Phase 3: 铲屎官选扇入者
Phase 4: 扇入综合（会议纪要 + 行动项）
Phase 5: 其他猫审阅补充
Phase 6: 铲屎官反馈 + 最终确认 → 进入 Mode C

**Phase 1 独立性保护规则**：
- 禁止互看：每只猫独立完成，不预测他人观点
- 防锚定：有背景材料时，先形成自己想法再参考
- 展示推理链："我为什么这么想"，不只给结论
- 标注不确定性：区分确信的结论和猜测
```

### Level 4: MCP 层硬约束

**代码位置**: `callback-multi-mention-routes.ts:39-49`

```typescript
const multiMentionSchema = callbackAuthSchema.extend({
  targets: z.array(z.string().min(1)).min(1).max(3),
  question: z.string().min(1).max(5000),
  callbackTo: z.string().min(1),
  context: z.string().max(5000).optional(),
  idempotencyKey: z.string().min(1).max(200).optional(),
  timeoutMinutes: z.number().int().min(3).max(20).optional(),
  searchEvidenceRefs: z.array(z.string()).optional(),  // 搜索证据
  overrideReason: z.string().min(1).max(500).optional(), // 跳过理由
});
```

**强制检查逻辑**：缺少 `searchEvidenceRefs` 且无 `overrideReason` → 拒绝调用

---

## 四、关键文件索引

### 设计文档

| 文件 | 说明 |
|------|------|
| `docs/features/F086-cat-orchestration-multi-mention.md` | multi_mention 核心设计，M1/M2/M3 里程碑 |
| `docs/features/F055-a2a-mcp-structured-routing.md` | targetCats 设计，路由优先级 |
| `docs/features/F079-voting-system.md` | 投票机制 |
| `docs/decisions/002-collaboration-protocol.md` | Why-First 协作协议 |

### 类型定义

| 文件 | 说明 |
|------|------|
| `packages/shared/src/types/a2a.ts` | A2A 协议类型 |
| `packages/shared/src/types/multi-mention.ts` | multi_mention 类型，状态枚举 |

### 核心实现

| 文件 | 说明 |
|------|------|
| `packages/api/src/domains/cats/services/agents/routing/a2a-mentions.ts` | @mention 解析 |
| `packages/api/src/domains/cats/services/agents/routing/MultiMentionOrchestrator.ts` | 状态机管理 |
| `packages/api/src/domains/cats/services/agents/routing/WorklistRegistry.ts` | 串行工作链 |
| `packages/api/src/routes/callback-multi-mention-routes.ts` | HTTP 端点处理 |
| `packages/api/src/routes/callback-a2a-trigger.ts` | A2A 入队逻辑 |
| `packages/api/src/routes/callbacks.ts` | post_message + targetCats |

### Prompt 工程

| 文件 | 说明 |
|------|------|
| `packages/api/src/domains/cats/services/context/SystemPromptBuilder.ts` | 系统提示词构建 |
| `cat-cafe-skills/refs/shared-rules.md` | 家规（规则真相源） |
| `cat-cafe-skills/collaborative-thinking/SKILL.md` | 协作思考 Skill |

---

## 五、设计要点总结

1. **三种交互方式各有分工**：
   - @mention：轻量级串行协作（最多 2 猫）
   - targetCats：结构化路由，避免文本污染
   - multi_mention：正式多猫讨论（最多 3 猫并行）

2. **状态机管理复杂度**：
   - multi_mention 有完整的状态生命周期
   - 超时机制防止无限等待
   - 结果聚合后回调给发起者

3. **反级联保护**：
   - 被召唤的猫不能再发起 multi_mention
   - 返回 409 错误，防止级联爆炸

4. **三层引导机制**：
   - 静态身份注入（L0 摘要）
   - 品种级工作流触发器
   - Skills 详细流程指南
   - MCP 层硬约束

5. **先搜后问原则**：
   - MCP 层强制检查搜索证据
   - 防止滥用 multi_mention
   - 降低无效协作成本

---

## 六、对 ISDP 的借鉴意义

1. **A2A 触发机制**：可参考三层引导机制，在 Agent 系统提示词中注入协作触发点
2. **multi_mention 模式**：可用于 ISDP 的多 Agent 并行讨论场景
3. **状态机管理**：可借鉴 MultiMentionOrchestrator 的设计模式
4. **反级联保护**：ISDP 也需要防止 Agent 级联触发
5. **MCP 硬约束**：强制"先搜后问"降低协作成本

---

## 七、multi_mention 完整调用链路详解

### 7.1 场景示例

假设布偶猫（ragdoll）正在开发一个复杂的 API 接口，遇到了架构选型问题，决定调用 multi_mention 请缅因猫（maine-coon）和暹罗猫（siamese）一起讨论。

### 7.2 完整调用链路

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Phase 1: 发起方猫决定触发 multi_mention                                   │
└─────────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ Phase 2: 猫调用 MCP 工具 cat_cafe_multi_mention                          │
│ - 客户端验证：searchEvidenceRefs 或 overrideReason 必须存在              │
│ - 缺少则返回错误，强制"先搜后问"                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ Phase 3: HTTP POST /api/callbacks/multi-mention                         │
│ - 鉴权验证（invocationId + callbackToken）                               │
│ - 目标猫有效性检查                                                        │
│ - 反级联保护检查（isActiveTarget）                                        │
└─────────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ Phase 4: MultiMentionOrchestrator.create() + start()                   │
│ - 生成 requestId，状态 pending → running                                │
│ - 启动超时定时器（默认 8 分钟）                                           │
└─────────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ Phase 5: dispatchViaQueue() - 并行分发到目标猫                           │
│ - 构建消息内容：[Multi-Mention from ragdoll]\n\n{question}              │
│ - 入队 InvocationQueue（source: 'agent', autoExecute: true）            │
│ - 注册完成钩子（registerEntryCompleteHook）                              │
└─────────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ Phase 6: QueueProcessor.tryAutoExecute() - 自动执行                     │
│ - 扫描所有 autoExecute 条目                                              │
│ - 检查目标猫 slot 是否空闲（processingSlots + InvocationTracker）        │
│ - 空闲则立即执行                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ Phase 7: QueueProcessor.executeEntry() - 执行队列条目                   │
│ - 创建 InvocationRecord                                                  │
│ - InvocationTracker.start() 占槽                                         │
│ - router.routeExecution() 启动目标猫 CLI 进程                            │
│ - 收集响应文本                                                            │
│ - 完成后触发 entryCompleteHook                                           │
└─────────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ Phase 8: 目标猫被唤起                                                    │
│ - CLI 进程启动，读取环境变量（INVOCATION_ID, CALLBACK_TOKEN）            │
│ - 系统提示词注入身份 + 协作规则                                           │
│ - 用户消息内容：[Multi-Mention from ragdoll]\n\n{question}              │
│ - 猫看到消息，理解这是来自其他猫的协作请求                                 │
└─────────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ Phase 9: 目标猫响应                                                      │
│ - 独立思考，给出观点                                                      │
│ - 响应文本被收集（responseText）                                         │
│ - 完成钩子被触发，记录响应到 Orchestrator                                │
└─────────────────────────────────────────────────────────────────────────┘
                                   ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ Phase 10: 结果聚合                                                       │
│ - 所有目标猫响应完成 → Orchestrator.recordResponse() 返回 'done'        │
│ - 取消超时定时器                                                          │
│ - flushResult() 汇总所有响应                                             │
│ - 生成汇总消息存入 thread                                                │
│ - WebSocket 广播给发起方猫                                               │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.3 代码级详细流程

#### Step 1: 发起方猫调用 MCP 工具

**文件**: `packages/mcp-server/src/tools/callback-tools.ts:556-621`

```typescript
// MCP 工具定义
export const multiMentionInputSchema = {
  targets: z.array(z.string().min(1)).min(1).max(3)
    .describe('Cat IDs to invoke in parallel (max 3). Example: ["codex","gemini"]'),
  question: z.string().min(1).max(5000)
    .describe('The question or request for the target cats'),
  callbackTo: z.string().min(1)
    .describe('Cat ID to route all responses back to (required, usually yourself)'),
  searchEvidenceRefs: z.array(z.string()).optional()
    .describe('References to searches you performed before calling this tool'),
  overrideReason: z.string().min(1).max(500).optional()
    .describe('Why you are skipping search evidence'),
  // ...
};

// 处理函数 - 客户端验证
export async function handleMultiMention(input: {...}): Promise<ToolResult> {
  // 🔒 客户端验证：searchEvidenceRefs 或 overrideReason 必须存在
  if (!input.searchEvidenceRefs?.length && !input.overrideReason) {
    return errorResult(
      'multi_mention requires searchEvidenceRefs (what did you search first?) ' +
      'or overrideReason (why are you skipping search?). ' +
      'This enforces the "先搜后问" principle — search before asking.'
    );
  }

  // 调用后端 API
  return callbackPost('/api/callbacks/multi-mention', {
    targets: input.targets,
    question: input.question,
    callbackTo: input.callbackTo,
    // ...
  });
}
```

**关键点**：
- 客户端验证在 MCP 层执行，**先搜后问**原则在调用后端前就强制执行
- 必须提供 `searchEvidenceRefs`（搜索了什么）或 `overrideReason`（为什么跳过搜索）
- 这确保猫不会随意触发 multi_mention，必须先进行调研

#### Step 2: 后端处理 HTTP 请求

**文件**: `packages/api/src/routes/callback-multi-mention-routes.ts:426-529`

```typescript
app.post('/api/callbacks/multi-mention', async (request, reply) => {
  const body = multiMentionSchema.parse(request.body);

  // 1️⃣ 鉴权验证
  const record = registry.verify(body.invocationId, body.callbackToken);
  if (!record) {
    return reply.status(401).send({ error: 'Invalid or expired callback credentials' });
  }

  // 2️⃣ 目标猫有效性检查
  for (const target of body.targets) {
    if (!catRegistry.has(target)) {
      return reply.status(400).send({ error: `Unknown cat: ${target}` });
    }
  }

  // 3️⃣ 反级联保护
  const orch = getMultiMentionOrchestrator();
  if (orch.isActiveTarget(record.threadId, callerCatId)) {
    return reply.status(409).send({
      error: 'Anti-cascade: caller is an active multi-mention target',
    });
  }

  // 4️⃣ 创建请求
  const mmRequest = orch.create({
    threadId: record.threadId,
    initiator: callerCatId,
    callbackTo: createCatId(body.callbackTo),
    targets: targetCatIds,
    question: body.question,
    timeoutMinutes: body.timeoutMinutes ?? 8,
  });

  // 5️⃣ 启动 + 超时调度
  orch.start(mmRequest.id);
  scheduleTimeout(mmRequest.id, mmRequest.timeoutMinutes, request.log);

  // 6️⃣ 并行分发
  if (deps.invocationQueue && deps.queueProcessor) {
    dispatchViaQueue(deps, mmRequest.id, targetCatIds, ...);
  }

  return reply.send({ requestId: mmRequest.id, status: mmRequest.status });
});
```

#### Step 3: 消息构建与入队

**文件**: `packages/api/src/routes/callback-multi-mention-routes.ts:101-164`

```typescript
function dispatchViaQueue(deps, requestId, targetCatIds, question, context, ...) {
  const { invocationQueue, queueProcessor } = deps;
  const orch = getMultiMentionOrchestrator();

  // 🔑 关键：构建消息内容，目标猫会看到这个前缀
  const messageContent = [
    `[Multi-Mention from ${initiator}]`,  // 👈 发起者标识
    question,
    ...(context ? ['---', context] : [])
  ].join('\n\n');

  for (const catId of targetCatIds) {
    // 入队 agent entry
    const result = invocationQueue.enqueue({
      threadId,
      userId,
      content: messageContent,           // 👈 目标猫收到的消息
      source: 'agent',                   // 标记来源是 agent（非用户）
      targetCats: [catId],
      intent: 'execute',
      autoExecute: true,                 // 👈 自动执行标记
      callerCatId: initiator,            // 调用者猫 ID
    });

    // 注册完成钩子 - 用于响应聚合
    if (result.outcome === 'enqueued' || result.outcome === 'merged') {
      queueProcessor.registerEntryCompleteHook(result.entry.id, 
        (_entryId, status, responseText) => {
          // 记录响应
          const newStatus = orch.recordResponse(requestId, catId, responseText);
          // 所有猫都响应完成 → 触发 flushResult
          if (newStatus === 'done') {
            cancelTimeout(requestId);
            void flushResult(deps, requestId, threadId, userId, log);
          }
        }
      );
    }
  }

  // 触发自动执行
  void queueProcessor.tryAutoExecute(threadId);
}
```

**关键点**：
- `[Multi-Mention from ${initiator}]` 前缀告诉目标猫这是协作请求
- `source: 'agent'` 标记这不是用户消息
- `autoExecute: true` 确保立即执行
- `registerEntryCompleteHook` 注册响应收集钩子

#### Step 4: 自动执行队列条目

**文件**: `packages/api/src/domains/cats/services/agents/invocation/QueueProcessor.ts:281-322`

```typescript
async tryAutoExecute(threadId: string): Promise<void> {
  // 获取所有 autoExecute 条目
  const entries = this.deps.queue.listAutoExecute(threadId);

  for (const entry of entries) {
    const entryCat = entry.targetCats[0];
    const sk = QueueProcessor.slotKey(threadId, entryCat);

    // 检查 slot 是否空闲
    if (this.processingSlots.has(sk)) continue;
    if (this.deps.invocationTracker.has(threadId, entryCat)) continue;

    // 标记为 processing
    if (!this.deps.queue.markProcessingById(threadId, entry.id)) continue;
    this.processingSlots.add(sk);

    // 异步执行
    void this.executeEntry(entry).then((status) => {
      this.processingSlots.delete(sk);
      this.onInvocationComplete(threadId, entryCat, status);
    });
  }
}
```

#### Step 5: 执行条目 - 启动目标猫

**文件**: `packages/api/src/domains/cats/services/agents/invocation/QueueProcessor.ts:418-699`

```typescript
private async executeEntry(entry: QueueEntry): Promise<...> {
  const { threadId, userId, content, targetCats, intent, messageId } = entry;
  const primaryCat = targetCats[0];

  try {
    // 1. 创建 InvocationRecord
    const createResult = await invocationRecordStore.create({
      threadId, userId, targetCats, intent,
      idempotencyKey: `queue-${entry.id}`,
    });
    invocationId = createResult.invocationId;

    // 2. 启动 tracker 占槽
    controller = invocationTracker.start(threadId, primaryCat, userId, targetCats);

    // 3. 更新状态为 running
    await invocationRecordStore.update(invocationId, { status: 'running' });

    // 4. 广播 intent_mode
    socketManager.broadcastToRoom(`thread:${threadId}`, 'intent_mode', {
      threadId, mode: intent, targetCats, invocationId,
    });

    // 5. 🚀 核心：执行路由 - 启动目标猫 CLI
    for await (const msg of router.routeExecution(
      userId,
      content,           // 👈 包含 [Multi-Mention from xxx] 的消息
      threadId,
      messageId,
      targetCats,
      { intent },
      { signal: controller.signal, parentInvocationId: invocationId }
    )) {
      // 收集响应文本
      if (hook && msg.catId === primaryCat && msg.type === 'text' && msg.content) {
        responseText += msg.content;
      }
      // 广播消息到前端
      socketManager.broadcastAgentMessage({ ...msg, invocationId }, threadId);
    }

    // 6. 标记成功
    await invocationRecordStore.update(invocationId, { status: 'succeeded' });
    finalStatus = 'succeeded';

  } finally {
    // 7. 清理
    invocationTracker.complete(threadId, primaryCat, controller);
    queue.removeProcessedAcrossUsers(threadId, entry.id);

    // 8. 触发完成钩子 - 收集响应
    const completeHook = this.entryCompleteHooks.get(entry.id);
    if (completeHook) {
      completeHook(entry.id, finalStatus, responseText);
    }
  }
}
```

#### Step 6: 目标猫感知到调用

当目标猫（如缅因猫）的 CLI 进程启动后：

1. **环境变量注入**：
   - `CAT_CAFE_INVOCATION_ID`: 当前调用的 ID
   - `CAT_CAFE_CALLBACK_TOKEN`: 回调认证令牌
   - `CAT_CAFE_API_URL`: API 地址

2. **系统提示词注入**（`SystemPromptBuilder.ts`）：
   ```
   Identity: Maine Coon/宪宪 (@maine-coon, model=Opus-4.6)

   ## 协作
   你可以 @队友: @布偶猫 / @暹罗猫 / ...
   格式：另起一行行首写 @猫名...

   ## 工作流（主动 @ 触发点）
   - 完成 review → @布偶猫 通知结果
   - 讨论/独立思考完成，结论需要其他猫跟进 → @ 对应猫
   ...

   ## 家规（shared-rules.md）
   ...
   ```

3. **用户消息内容**：
   ```
   [Multi-Mention from ragdoll]

   我在设计 API 接口时遇到了架构选型问题...

   ---

   背景：这是一个跨模块的改动，涉及前端和后端...
   ```

4. **猫如何理解**：
   - 看到 `[Multi-Mention from ragdoll]` 前缀 → 知道这是布偶猫发起的协作请求
   - 消息内容是问题 + 背景
   - 这是"多猫独立思考"模式（Mode B）
   - 应该独立给出自己的观点，不受其他猫影响

#### Step 7: 响应聚合

**文件**: `packages/api/src/routes/callback-multi-mention-routes.ts:346-419`

```typescript
async function flushResult(deps, requestId, threadId, userId, log): Promise<void> {
  const orch = getMultiMentionOrchestrator();
  const result = orch.getResult(requestId);

  // 构建汇总消息
  const lines: string[] = [
    `## Multi-Mention 结果汇总`,
    '',
    `**问题**: ${result.request.question}`,
    ''
  ];

  for (const resp of result.responses) {
    const catName = catRegistry.tryGet(resp.catId)?.config.displayName ?? resp.catId;
    if (resp.status === 'received') {
      lines.push(`### ${catName}`);
      lines.push(resp.content || '(空回答)');
      lines.push('');
    } else {
      lines.push(`### ${catName} — ${resp.status === 'timeout' ? '超时' : '失败'}`);
    }
  }

  const content = lines.join('\n');

  // 存储汇总消息
  const stored = await messageStore.append({
    userId,
    catId: result.request.callbackTo,  // 发给发起方猫
    content,
    mentions: [],
    timestamp: Date.now(),
    threadId,
    source: {
      connector: 'multi-mention-result',
      label: 'Multi-Mention 结果',
      meta: {
        initiator: result.request.callbackTo,
        targets: [...result.request.targets],
      }
    },
  });

  // WebSocket 广播
  socketManager.broadcastToRoom(`thread:${threadId}`, 'connector_message', {
    threadId,
    message: { id: stored.id, type: 'connector', content, ... },
  });
}
```

### 7.4 "先搜后问"机制的完整实现

#### 客户端验证（MCP 层）

**文件**: `callback-tools.ts:601-608`

```typescript
export async function handleMultiMention(input: {...}): Promise<ToolResult> {
  // 🔒 硬性要求：必须提供搜索证据或跳过理由
  if (!input.searchEvidenceRefs?.length && !input.overrideReason) {
    return errorResult(
      'multi_mention requires searchEvidenceRefs (what did you search first?) ' +
      'or overrideReason (why are you skipping search?). ' +
      'This enforces the "先搜后问" principle — search before asking.'
    );
  }
  // ...
}
```

#### Schema 定义

```typescript
export const multiMentionInputSchema = {
  // ...
  searchEvidenceRefs: z.array(z.string()).optional()
    .describe('References to searches you performed before calling this tool'),
  overrideReason: z.string().min(1).max(500).optional()
    .describe('Why you are skipping search evidence'),
  // ...
};
```

#### 猫如何满足这个要求

1. **正常流程**：
   ```
   猫思考：我需要调用 multi_mention 请其他猫讨论架构问题

   Step 1: 先搜索相关文档
   → 调用 cat_cafe_search_evidence("API 架构")
   → 找到 docs/decisions/ADR-015-api-design.md

   Step 2: 再调用 multi_mention
   → cat_cafe_multi_mention({
       targets: ["maine-coon", "siamese"],
       question: "API 架构选型问题...",
       searchEvidenceRefs: ["ADR-015-api-design.md", "docs/features/F043.md"],
       callbackTo: "ragdoll"
     })
   ```

2. **跳过搜索**：
   ```
   猫思考：这个场景很紧急，没有时间搜索

   → cat_cafe_multi_mention({
       targets: ["maine-coon"],
       question: "紧急问题...",
       overrideReason: "生产环境紧急故障，需要立即获取其他猫的建议",
       callbackTo: "ragdoll"
     })
   ```

#### 系统提示词中的引导

**文件**: `shared-rules.md:256-269`

```markdown
## 13. 元思考触发器 §13（F086 M2）

调用 `cat_cafe_multi_mention` 前，**必须先搜后问**
（MCP 层硬检查：缺少 `searchEvidenceRefs` 且无 `overrideReason` → 拒绝调用）。

| 触发器 | 场景 | 默认动作 |
|--------|------|---------|
| **A: 高影响决策** | 架构选型、API 契约、跨模块改动 | 先搜 `docs/decisions/` → 再决定是否 multi_mention |
| **B: 跨领域问题** | 涉及前端/安全/性能/UX 等非自身专长 | 先搜对应领域文档 → 再 @ 对应领域的猫 |
| **C: 高不确定性** | 方案不确定、多种选择难以取舍 | 先搜历史讨论 → 再拉猫获取多视角 |
| **D: 信息不足** | 发现自己对上下文了解不够 | 先 search → 再问人 |
| **E: 新领域侦查** | 要写新代码/MCP/集成时 | 先从 feats 顺藤摸瓜 → 再动手 |

**硬检查 vs 软引导**：
- **硬**：`multi_mention` MCP 调用必须带 `searchEvidenceRefs[]`（≥1 条）或 `overrideReason`
- **软**：触发器表是自检参考，不是每次工作都要填表——只在调 `multi_mention` 时强制
```

### 7.5 完整调用时序图

```
布偶猫              MCP Server           API Server          Orchestrator        Queue         QueueProcessor      缅因猫CLI
  │                    │                    │                    │                │                 │               │
  │ 决定触发multi_mention│                    │                    │                │                 │               │
  │───────────────────>│                    │                    │                │                 │               │
  │                    │                    │                    │                │                 │               │
  │                    │ 客户端验证          │                    │                │                 │               │
  │                    │ (searchEvidenceRefs│                    │                │                 │               │
  │                    │  或overrideReason) │                    │                │                 │               │
  │                    │                    │                    │                │                 │               │
  │                    │ POST /multi-mention│                    │                │                 │               │
  │                    │───────────────────>│                    │                │                 │               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │ 鉴权验证            │                │                 │               │
  │                    │                    │ 反级联检查          │                │                 │               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │ create()           │                │                 │               │
  │                    │                    │───────────────────>│                │                 │               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │ start() + timeout  │                │                 │               │
  │                    │                    │───────────────────>│                │                 │               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │ dispatchViaQueue() │                │                 │               │
  │                    │                    │───────────────────>│                │                 │               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │                    │ enqueue()      │                 │               │
  │                    │                    │                    │───────────────>│                 │               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │                    │ registerHook() │                 │               │
  │                    │                    │                    │───────────────────────────────>│               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │                    │ tryAutoExecute()               │               │
  │                    │                    │                    │───────────────────────────────>│               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │                    │                │ executeEntry()  │               │
  │                    │                    │                    │                │────────────────>│               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │                    │                │ routeExecution()│ 启动CLI进程   │
  │                    │                    │                    │                │────────────────>│──────────────>│
  │                    │                    │                    │                │                 │               │
  │                    │                    │                    │                │                 │  [Multi-Mention
  │                    │                    │                    │                │                 │   from ragdoll]
  │                    │                    │                    │                │                 │  {question}    │
  │                    │                    │                    │                │                 │───────────────>│
  │                    │                    │                    │                │                 │               │
  │                    │                    │                    │                │                 │        缅因猫思考并响应
  │                    │                    │                    │                │                 │<──────────────│
  │                    │                    │                    │                │                 │               │
  │                    │                    │                    │                │ hook触发        │               │
  │                    │                    │                    │<───────────────────────────────│               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │                    │ recordResponse()               │               │
  │                    │                    │                    │<───────────────│                 │               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │                    │ (所有猫响应完成)               │               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │ flushResult()      │                │                 │               │
  │                    │                    │<───────────────────│                │                 │               │
  │                    │                    │                    │                │                 │               │
  │                    │                    │ 存储汇总消息        │                │                 │               │
  │                    │                    │ WebSocket广播      │                │                 │               │
  │                    │                    │                    │                │                 │               │
  │ 收到汇总结果        │                    │                    │                │                 │               │
  │<───────────────────│<───────────────────│                    │                │                 │               │
```

---

## 八、猫如何知道"要通知哪些猫"

### 8.1 信息来源架构

猫获取"可通知的队友列表"有三个层次：

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Level 1: 静态配置源 (cat-template.json / cat-config.json)              │
│                                                                         │
│ 定义所有猫的配置：                                                       │
│ - breeds: 品种列表，每个品种包含 variants (变体)                        │
│ - 每个猫有: catId, displayName, mentionPatterns, roleDescription,       │
│           teamStrengths (擅长), caution (注意点)                        │
│ - roster: 协作角色定义 (family, roles, lead, available)                 │
│                                                                         │
│ 文件位置: D:\Tools\catCoffee\clowder-ai\cat-template.json               │
└─────────────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ Level 2: 运行时注册表 (catRegistry)                                      │
│                                                                         │
│ 全局单例，启动时从配置文件加载：                                          │
│ - catRegistry.getAllConfigs() → 所有猫配置                              │
│ - catRegistry.has(catId) → 验证猫是否存在                               │
│ - catRegistry.tryGet(catId) → 获取单个猫配置                            │
│                                                                         │
│ 文件位置: packages/shared/src/registry/CatRegistry.ts                   │
└─────────────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ Level 3: 系统提示词注入 (SystemPromptBuilder)                            │
│                                                                         │
│ 每次猫被调用时，注入以下信息：                                            │
│ 1. ## 协作 - buildCallableMentions() 可 @ 的队友列表                    │
│ 2. ## 队友名册 - buildTeammateRoster() 队友详情表格                     │
│ 3. ## 工作流（主动 @ 触发点）- WORKFLOW_TRIGGERS 品种级 SOP              │
│                                                                         │
│ 文件位置: packages/api/src/domains/cats/services/context/               │
│          SystemPromptBuilder.ts                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 8.2 系统提示词注入详解

#### 8.2.1 可 @ 队友列表 (`buildCallableMentions`)

**代码位置**: `SystemPromptBuilder.ts:146-186`

```typescript
function buildCallableMentions(currentCatId: CatId): CallableMentionsResult {
  // 获取所有其他猫的配置（排除自己）
  const entries = Object.entries(getAllConfigs())
    .filter(([id]) => id !== currentCatId)
    .map(([id, config]) => ({ id, config }));

  // 处理重名情况（同族多分身）
  const byDisplayName = new Map<string, CallableCatEntry[]>();
  for (const entry of entries) {
    const group = byDisplayName.get(entry.config.displayName);
    if (group) {
      group.push(entry);
    } else {
      byDisplayName.set(entry.config.displayName, [entry]);
    }
  }

  // 生成 mention 列表
  const mentions: string[] = [];
  for (const entry of entries) {
    const group = byDisplayName.get(entry.config.displayName) ?? [];
    // 默认变体用 @displayName，其他用唯一句柄
    const mention =
      group.length <= 1 || entry.config.isDefaultVariant
        ? `@${entry.config.displayName}`
        : pickVariantMention(entry.id, entry.config);
    mentions.push(mention);
  }

  return { mentions, hasDuplicateDisplayNames, uniqueHandleExample };
}
```

**注入结果示例**：
```
## 协作
你可以 @队友: @缅因猫 / @暹罗猫 / @codex / @gemini
格式：另起一行行首写 @猫名（行中无效，多猫各占一行），上文或下文写请求均可。
[正确] @缅因猫\n请帮忙  [正确] 内容...\n@缅因猫  [错误] 行中 @缅因猫
```

#### 8.2.2 队友名册表格 (`buildTeammateRoster`)

**代码位置**: `SystemPromptBuilder.ts:294-313`

```typescript
function buildTeammateRoster(currentCatId: CatId): string | null {
  const allConfigs = getAllConfigs();
  const entries = Object.entries(allConfigs).filter(([id]) => id !== currentCatId);

  const rows: string[] = [];
  for (const [id, config] of entries) {
    const label = config.variantLabel
      ? `${config.displayName} ${config.variantLabel}`
      : config.nickname
        ? `${config.displayName}/${config.nickname}`
        : config.displayName;
    const mention = pickVariantMention(id, config);
    const strengths = config.teamStrengths ?? config.roleDescription;
    const caution = config.caution ?? '—';
    rows.push(`| ${label} | ${mention} | ${strengths} | ${caution} |`);
  }

  return [
    '## 队友名册',
    '| 猫猫 | @mention | 擅长 | 注意 |',
    '|------|---------|------|------|',
    ...rows
  ].join('\n');
}
```

**注入结果示例**：
```
## 队友名册
| 猫猫 | @mention | 擅长 | 注意 |
|------|---------|------|------|
| 缅因猫/宪宪 | @缅因猫 | 代码审查和架构设计，专注于代码质量和系统架构 | 有时过于关注细节 |
| 暹罗猫 | @暹罗猫 | 视觉设计师和创意顾问，擅长 UI/UX 设计和视觉表达 | — |
| Codex | @codex | GPT-5.2 驱动的多面手，擅长快速原型和多样化任务 | 非主型号，部分 SOP 不适用 |
```

#### 8.2.3 品种级工作流触发点 (`WORKFLOW_TRIGGERS`)

**代码位置**: `SystemPromptBuilder.ts:255-287`

```typescript
const WORKFLOW_TRIGGERS: Record<string, string> = {
  ragdoll: [
    '## 工作流（主动 @ 触发点）',
    '- 完成开发/修复 → @缅因猫 请 review',
    '- 修完 review 意见 → @缅因猫 确认修复',
    '- 遇到视觉/体验问题 → @暹罗猫 征询',
  ].join('\n'),
  'maine-coon': [
    '## 工作流（主动 @ 触发点）',
    '- 完成 review → @布偶猫 通知结果',
    '- 讨论/独立思考完成，结论需要其他猫跟进 → @ 对应猫',
  ].join('\n'),
  siamese: [
    '## 工作流（主动 @ 触发点）',
    '- 完成设计/视觉资产 → 分别 @布偶猫 和 @缅因猫 请确认',
    '- 遇到技术实现问题 → @布偶猫 征询',
  ].join('\n'),
};
```

### 8.3 配置文件结构

**文件位置**: `D:\Tools\catCoffee\clowder-ai\cat-template.json`

```json
{
  "version": 2,
  "breeds": [
    {
      "id": "ragdoll",
      "catId": "opus",
      "name": "Ragdoll",
      "displayName": "布偶猫",
      "nickname": "宪宪",
      "mentionPatterns": ["@布偶猫", "@ragdoll", "@opus"],
      "roleDescription": "全栈开发工程师，专注于实现和重构",
      "teamStrengths": "全栈开发，快速实现，代码重构",
      "caution": "有时过于追求完美",
      "variants": [
        {
          "id": "opus-46",
          "catId": "opus",
          "provider": "anthropic",
          "defaultModel": "claude-opus-4-6",
          "mcpSupport": true
        }
      ]
    },
    {
      "id": "maine-coon",
      "catId": "codex",
      "displayName": "缅因猫",
      "nickname": "宪宪",
      "mentionPatterns": ["@缅因猫", "@maine-coon", "@codex"],
      "roleDescription": "代码审查和架构设计，专注于代码质量和系统架构",
      "teamStrengths": "架构设计，代码审查，质量把关",
      "variants": [...]
    }
  ],
  "roster": {
    "opus": {
      "family": "anthropic",
      "roles": ["developer", "architect"],
      "lead": true,
      "available": true
    },
    "codex": {
      "family": "openai",
      "roles": ["peer-reviewer", "architect"],
      "lead": true,
      "available": true
    }
  },
  "reviewPolicy": {
    "requireDifferentFamily": true,
    "preferActiveInThread": true,
    "preferLead": true,
    "excludeUnavailable": true
  }
}
```

### 8.4 猫如何选择通知谁

猫选择通知谁基于以下信息：

1. **队友名册** - 知道有哪些猫、它们擅长什么
2. **工作流触发器** - 知道特定场景应该 @ 谁
3. **元思考触发器** - 知道什么场景需要多猫协作

**决策流程示例**：

```
布偶猫思考：

1. 我刚完成了一个 API 接口的开发
   → 查看工作流触发器："完成开发/修复 → @缅因猫 请 review"
   → 决定 @缅因猫

2. 我遇到了一个架构选型问题，不确定用 REST 还是 GraphQL
   → 查看元思考触发器 A（高影响决策）
   → 先搜索 docs/decisions/ADR-xxx-api-design.md
   → 找到相关文档后，决定调用 multi_mention 请 @缅因猫 和 @暹罗猫 讨论

3. 我遇到了一个 UI 实现问题
   → 查看队友名册：暹罗猫擅长 "视觉设计师和创意顾问"
   → 查看 @ 卫生规则：需要对方采取行动
   → 决定 @暹罗猫 征询
```

### 8.5 完整信息流

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 启动时                                                                   │
├─────────────────────────────────────────────────────────────────────────┤
│ 1. loadCatConfig() 从 cat-template.json / cat-config.json 加载配置      │
│ 2. toAllCatConfigs() 将 breeds + variants 展开为 Record<catId, CatConfig>│
│ 3. catRegistry.register() 注册所有猫到全局注册表                          │
└─────────────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ 猫被调用时                                                               │
├─────────────────────────────────────────────────────────────────────────┤
│ 1. buildStaticIdentity(catId) 构建静态身份                               │
│    - buildCallableMentions() → "你可以 @队友: ..."                       │
│    - buildTeammateRoster() → 队友名册表格                                │
│    - WORKFLOW_TRIGGERS[breedId] → 工作流触发点                           │
│                                                                         │
│ 2. buildInvocationContext() 构建动态上下文                               │
│    - teammates: 当前调用的队友                                           │
│    - mode: independent/serial/parallel                                  │
│    - A2A 出口检查提醒                                                    │
│                                                                         │
│ 3. 注入到 CLI 进程的系统提示词                                            │
└─────────────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────────────┐
│ 猫决策时                                                                 │
├─────────────────────────────────────────────────────────────────────────┤
│ 猫阅读系统提示词，理解：                                                  │
│ - 有哪些队友可以 @                                                       │
│ - 每个队友擅长什么                                                       │
│ - 什么场景应该 @ 谁                                                      │
│ - 什么场景应该调用 multi_mention                                         │
│                                                                         │
│ 然后根据当前任务需求做出决策                                              │
└─────────────────────────────────────────────────────────────────────────┘
```

### 8.6 关键代码位置索引

| 功能 | 文件 | 函数/变量 |
|------|------|----------|
| 配置加载 | `packages/api/src/config/cat-config-loader.ts` | `loadCatConfig()`, `toAllCatConfigs()` |
| 全局注册表 | `packages/shared/src/registry/CatRegistry.ts` | `catRegistry` |
| 系统 Prompt 构建 | `packages/api/src/domains/cats/services/context/SystemPromptBuilder.ts` | `buildStaticIdentity()`, `buildCallableMentions()`, `buildTeammateRoster()` |
| 工作流触发器 | `SystemPromptBuilder.ts:255` | `WORKFLOW_TRIGGERS` |
| 元思考触发器 | `cat-cafe-skills/refs/shared-rules.md` | §13 元思考触发器 |