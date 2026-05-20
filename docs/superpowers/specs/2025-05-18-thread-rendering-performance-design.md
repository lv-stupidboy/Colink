# 前端对话渲染性能优化设计

> 日期：2025-05-18
> 作者：Claude
> 状态：审查通过
> 审查结果：Ambiguity 12.75% ≤ 20% threshold

## 问题背景

当前前端 Agent 对话框渲染存在以下性能问题：

| 问题 | 场景 | 影响 |
|------|------|------|
| WebSocket 高频更新 | 流式输出 | 每秒数十次状态更新，触发组件重渲染 |
| 订阅粒度过粗 | 全局 | ThreadView 约 40 个 selector，任何状态变化都可能触发重渲染 |
| 缺少虚拟列表 | 长对话 | messages.map 直接渲染，消息量大时性能差 |
| 切屏返回清空状态 | 切屏返回 | 白屏 + 重新请求 API，用户体验差 |
| 组件内复杂逻辑 | 全局 | aggregateBlocks 计算、大量 console.log |
| 缺少 React 优化 | 全局 | 未使用 memo、useMemo 等性能优化手段 |

## 设计目标

- 切屏返回：无白屏，数据保留，增量更新
- 流式输出：平滑渲染，无明显卡顿
- 长对话：虚拟列表支撑，100+ 消息无性能衰减

**核心假设**：先优化前端，如效果不佳再排查后端（审查 Round 4 确认）。

## 优化方案：渐进式四阶段

### 技术依赖关系

```
阶段 1：状态管理重构（基础层）
    ↓
阶段 2：虚拟列表引入（渲染层）
    ↓
阶段 3：组件性能优化（组件层）
    ↓
阶段 4：切屏返回缓存（应用层）
```

---

## 阶段 1：状态管理重构

### 1.1 流式状态隔离

将流式状态从主 store 分离，创建独立的 `StreamingStore`：

```
useAppStore（低频状态）
├── currentThread
├── currentProject
├── messages（已完成消息）
├── activeAgents
└── ...

StreamingStore（高频状态）
├── isStreaming
├── streamingInvocationId
├── streamingAgentId
├── streamingAgentName
├── streamingContentBlocks
├── progressStatus
├── progressToolName
└── progressToolInput
```

**StreamingStore 完整接口定义**（审查 Round 7 确认）：

```typescript
interface StreamingState {
  isStreaming: boolean;
  streamingInvocationId: string | null;
  streamingAgentId: string | null;
  streamingAgentName: string | null;
  streamingContentBlocks: MessageContentBlock[];
  progressStatus: 'thinking' | 'tool_use' | 'generating' | 'idle';
  progressToolName: string | null;
  progressToolInput: Record<string, unknown> | null;
}
```

**Store 迁移策略**（审查 Round 1 确认）：渐进迁移，新建 StreamingStore 后逐步将流式状态移入，保持向后兼容。

**效果：** `StreamingMessage` 组件只订阅 `StreamingStore`，已完成消息列表订阅 `useAppStore`，互不影响。

### 1.2 批量更新策略

WebSocket chunk 每 100ms 合并一次，减少状态更新频率：

```typescript
const chunkBuffer: Chunk[] = [];
let flushTimer: ReturnType<typeof setTimeout> | null = null;

function enqueueChunk(chunk: Chunk) {
  chunkBuffer.push(chunk);
  if (!flushTimer) {
    flushTimer = setTimeout(() => {
      flushChunks(chunkBuffer);
      chunkBuffer.length = 0;
      flushTimer = null;
    }, 100);
  }
}
```

**阈值依据**（审查 Round 8 确认）：100ms 为经验值，平衡延迟和更新频率，实际效果需验证。可根据 WebSocket 推送频率调整。

### 1.3 降低订阅粒度

**改动点：**

1. 低频状态合并订阅
2. 高频状态使用 `useShallow` 比较引用
3. Actions 使用 `useAppStore.getState()` 获取，避免订阅

---

## 阶段 2：虚拟列表引入

### 2.1 依赖选择

引入 **@tanstack/react-virtual**：
- 轻量（~3KB）、无样式依赖
- 支持动态高度
- React 18 兼容

### 2.2 替换渲染方式

```tsx
import { useVirtualizer } from '@tanstack/react-virtual';

const virtualizer = useVirtualizer({
  count: messages.length,
  getScrollElement: () => listRef.current,
  estimateSize: (index) => estimateMessageHeight(messages[index]),
  overscan: 5,
  measureElement: (element) => element.getBoundingClientRect().height, // 动态测量
});

{virtualizer.getVirtualItems().map((virtualItem) => (
  <ChatMessage
    key={messages[virtualItem.index].id}
    ref={(el) => virtualizer.measureElement(el)} // 注册测量
    style={{ position: 'absolute', top: virtualItem.start }}
    ...
  />
))}
```

### 2.3 动态高度估算

**高度估算偏差处理**（审查 Round 2 确认）：使用 `measureElement` 动态测量真实高度并修正。

初始估算函数作为 fallback：

```typescript
function estimateMessageHeight(message: Message): number {
  let height = 60; // 基础高度（头像+名称+时间戳）
  
  if (message.contentBlocks) {
    for (const block of message.contentBlocks) {
      if (block.type === 'text') {
        height += Math.ceil(block.content.length / 50) * 20;
      } else if (block.type === 'thinking') {
        height += block.status === 'streaming' ? 80 : 40;
      } else if (block.type === 'tool_use') {
        height += 60;
      } else if (block.type === 'question') {
        height += 120;
      }
    }
  }
  
  return Math.max(height, 80);
}
```

### 2.4 滚动控制适配

使用 `virtualizer.scrollToIndex(messages.length - 1)` 替代 `scrollIntoView`。

---

## 阶段 3：组件性能优化

### 3.1 ChatMessage 组件

**自定义 memo 比较：**

```tsx
const ChatMessage = memo(
  ({ message, ... }) => { ... },
  (prevProps, nextProps) => {
    return (
      prevProps.message.id === nextProps.message.id &&
      prevProps.message.content === nextProps.message.content &&
      prevProps.isStreaming === nextProps.isStreaming &&
      prevProps.progress?.status === nextProps.progress?.status
    );
  }
);
```

**内容缓存：**

```tsx
const contentBlocks = useMemo(() => {
  return message.contentBlocks?.length > 0
    ? message.contentBlocks.map(filterA2AHandoff)
    : contentToBlocks(message.content);
}, [message.contentBlocks, message.content]);
```

### 3.2 MessageContentRenderer

**聚合逻辑缓存：**

```tsx
const aggregatedBlocks = useMemo(
  () => aggregateBlocks(filteredBlocks),
  [filteredBlocks]
);
```

**移除调试日志：**

```tsx
if (process.env.NODE_ENV === 'development' && localStorage.getItem('debug_logs')) {
  console.log('[MessageContentRenderer] ...');
}
```

### 3.3 StreamingMessage

合并订阅为单一 selector：

```tsx
const streamingState = useAppStore((s) => ({
  isStreaming: s.isStreaming,
  contentBlocks: s.streamingContentBlocks,
  agentId: s.streamingAgentId,
  agentName: s.streamingAgentName,
  invocationId: s.streamingInvocationId,
}), shallowCompare);
```

### 3.4 滚动处理优化

使用 throttle 函数：

```tsx
import { throttle } from 'lodash-es';

const handleScroll = useMemo(
  () => throttle((e: Event) => {
    const { scrollTop, scrollHeight, clientHeight } = listRef.current;
    setIsNearBottom(scrollHeight - scrollTop - clientHeight < 50);
  }, 100),
  []
);
```

---

## 阶段 4：切屏返回缓存策略

### 4.1 增量加载策略

```typescript
loadThread: async (threadId) => {
  const state = get();
  
  // 已有数据且是同一个 thread
  if (state.currentThread?.id === threadId && state.messages.length > 0) {
    // 增量更新：只检查变化
    const invocations = await api.invocations.list(threadId);
    const runningCount = invocations.filter(i => i.status === 'running').length;
    
    if (state.activeAgents.length === runningCount) {
      // 无变化，跳过更新
      return;
    }
    
    // 有变化，增量更新
    set({
      activeAgents: invocations.filter(i => i.status === 'running'),
    });
    return;
  }
  
  // 新 thread 或无数据，完整加载
  set({ loading: true });
  // ... 原有逻辑
}
```

### 4.2 WebSocket 重连策略

使用 `visibilitychange` 事件管理连接：

```typescript
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible') {
    if (!wsConnectedRef.current && currentThreadId) {
      connectWebSocket(currentThreadId);
    }
  }
});
```

### 4.3 分级加载状态

```tsx
// 有缓存数据时
if (messages.length > 0) {
  return (
    <div className="thread-view">
      {loading && <LoadingBar />}  {/* 轻量指示器 */}
      <ChatMessageList messages={messages} ... />
    </div>
  );
}

// 无缓存数据时
if (loading) {
  return <ThreadSkeleton />;
}
```

### 4.4 数据保鲜策略

**后端依赖澄清**（审查 Round 3 确认）：

| 事件 | 状态 | 处理方式 |
|------|------|----------|
| `recover_invocation_state` | ✓ 已存在 | 切屏返回 WebSocket 重连时触发 |
| `invocation_recovery` | ✓ 已存在 | 恢复运行中 Agent 状态 |
| `agent_state_restore` | ✓ 已存在 | 恢复累积输出 |
| `thread_updated` | ❌ 不存在 | **移除依赖** |

**数据保鲜策略**（简化为 TTL + invocation_recovery + 手动刷新）：

- 缓存 TTL：5 分钟内信任缓存
- WebSocket 重连时 `invocation_recovery` 自动同步状态
- 用户手动刷新：提供刷新按钮

```typescript
interface AppState {
  threadLoadedAt: number | null;
}

const CACHE_TTL = 5 * 60 * 1000; // 5 分钟

function isCacheValid(state: AppState): boolean {
  return state.threadLoadedAt && Date.now() - state.threadLoadedAt < CACHE_TTL;
}
```

---

## 涉及文件

| 文件 | 改动类型 | 阶段 |
|------|----------|------|
| `web/src/store/index.ts` | 重构 | 1 |
| `web/src/store/streaming.ts` | 新增 | 1 |
| `web/src/hooks/useWebSocket.ts` | 重构 | 1 |
| `web/src/components/thread/ChatMessageList.tsx` | 重构 | 2, 3 |
| `web/src/components/thread/ChatMessage.tsx` | 优化 | 3 |
| `web/src/components/thread/StreamingMessage.tsx` | 优化 | 1, 3 |
| `web/src/components/thread/ContentBlock/MessageContentRenderer.tsx` | 优化 | 3 |
| `web/src/pages/ThreadView.tsx` | 重构 | 1, 3, 4 |
| `web/src/components/ThreadSkeleton.tsx` | 新增 | 4 |
| `web/src/components/LoadingBar.tsx` | 新增 | 4 |
| `auto-test/e2e/thread-performance.spec.ts` | 新增 | 验收测试 |

---

## 验收标准

| 场景 | 指标 | 测量方法 |
|------|------|----------|
| 切屏返回 | 无白屏，显示时间 < 100ms | Playwright 性能测试 |
| 流式输出 | FPS ≥ 30，无明显卡顿 | Playwright 性能测试 |
| 100 条消息 | 滚动流畅，FPS ≥ 50 | Playwright 性能测试 |
| 500 条消息 | 滚动流畅，FPS ≥ 30 | Playwright 性能测试 |

**性能测试文件**（审查 Round 9 确认）：`auto-test/e2e/thread-performance.spec.ts`

示例测试用例：

```typescript
// auto-test/e2e/thread-performance.spec.ts
import { test, expect } from '@playwright/test';

test('切屏返回性能', async ({ page }) => {
  // 预加载 thread 页面
  await page.goto('/thread/test-thread-id');
  await page.waitForSelector('.chat-message-list');
  
  // 切换到其他页面
  await page.goto('/projects');
  await page.waitForLoadState('networkidle');
  
  // 返回并测量显示时间
  const startTime = Date.now();
  await page.goto('/thread/test-thread-id');
  await page.waitForSelector('.chat-message-list', { state: 'visible' });
  const elapsed = Date.now() - startTime;
  
  expect(elapsed).toBeLessThan(100);
});

test('流式输出 FPS', async ({ page }) => {
  // 启动 Agent 并测量渲染帧率
  await page.goto('/thread/test-thread-id');
  await page.click('[data-testid="start-agent"]');
  
  // 使用 Playwright 的 performance API 测量
  const metrics = await page.evaluate(() => {
    return performance.getEntriesByType('measure')
      .filter(e => e.name.startsWith('render-frame'))
      .map(e => e.duration);
  });
  
  const avgFrameTime = metrics.reduce((a, b) => a + b, 0) / metrics.length;
  const fps = 1000 / avgFrameTime;
  
  expect(fps).toBeGreaterThanOrEqual(30);
});
```

---

## 审查记录

| 轮次 | 问题 | 确认方案 |
|------|------|----------|
| Round 1 | Store 迁移策略 | 渐进迁移 |
| Round 2 | 高度估算偏差 | 动态测量 + measureElement |
| Round 3 | thread_updated 不存在 | 移除依赖，复用 invocation_recovery |
| Round 4 | 核心假设验证 | 先前端后后端 |
| Round 5 | FPS 测量方案 | Playwright 性能测试 |
| Round 6 | 自动化工具选择 | Playwright |
| Round 7 | StreamingStore 字段 | 完整流式状态（8 字段） |
| Round 8 | 批量更新阈值 | 100ms 经验值 + 验证 |
| Round 9 | 测试文件位置 | auto-test/e2e/thread-performance.spec.ts |

**最终 Ambiguity Score：12.75% ≤ 20% threshold ✓**