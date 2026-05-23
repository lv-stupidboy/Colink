# 前端对话渲染性能优化 - 执行计划

> **基于深访谈后优化的方案**
>
> **最后更新：** 2025-05-19
> **状态：** ✅ 待执行
> **歧义度：** 19.75%（低于 20% 阈值）

---

## 🎯 核心目标

解决三个性能问题：
1. **切屏返回白屏 + 卡顿** - 页面从其他 tab 切回时 UI 线程阻塞
2. **流式输出卡顿** - 字符逐字输出时 FPS 低
3. **长对话性能衰减** - 消息超过 50 条后滚动明显掉帧

---

## 📐 验收标准（可量化）

| 指标 | 目标值 | 测量方式 |
|------|--------|----------|
| 切屏返回显示时间 | < 100ms | Playwright 计时 |
| 100 条消息滚动 FPS | ≥ 50 | requestAnimationFrame 计数 |
| 流式输出 FPS | ≥ 30 | PerformanceObserver |
| 正常滚动白屏 | 无可见空白 | 人工验收 |
| 流式输出连贯性 | 无跳跃闪烁 | 人工验收 |

---

## 🗂️ 最终确认的技术方案

### ✅ 已确认，按此执行

| 模块 | 方案 | 关键决策 |
|------|------|----------|
| **流式状态隔离** | StreamingStore + useChunkBatcher | 100ms 基础间隔，thinking 块可加长到 200ms |
| **虚拟列表** | @tanstack/react-virtual | overscan = 8（原 5，增强防白屏），正常滚动无空白 |
| **组件优化** | memo + useMemo + throttle | ChatMessage 自定义比较函数，滚动 throttle 16ms |
| **切屏加载** | 内存状态保留 + 后台静默刷新 | ❌ 否决 5 分钟 TTL 缓存 |
| **加载状态** | LoadingBar + ThreadSkeleton | 有数据时仅顶部细条，无数据时骨架屏 |
| **WebSocket** | visibilitychange 重连 | 页面可见时检查连接状态 |

### ❌ 已否决，不执行

| 方案 | 否决原因 |
|------|----------|
| 5 分钟 TTL 缓存机制 | 用户认为不合适，可能导致数据不一致 |

---

## 📋 任务分解（按执行顺序）

### 🔹 Phase 1: 状态管理重构（优先级：最高）

**目标：** 降低流式输出时的重渲染频率

---

#### Task 1.1: 创建 StreamingStore

**文件：** `web/src/store/streaming.ts`（新建）

**实现要点：**
```typescript
// 状态字段
isStreaming: boolean
streamingInvocationId: string | null
streamingAgentId: string | null
streamingAgentName: string | null
streamingContentBlocks: MessageContentBlock[]
progressStatus: 'thinking' | 'tool_use' | 'generating' | 'idle'
progressToolName: string | null
progressToolInput: Record<string, unknown> | null

// Actions
startStreaming(invocationId, agentId, agentName)
stopStreaming()
appendContentBlock(block)
updateContentBlock(blockId, update)
updateProgress(status, toolName?, toolInput?)
clearStreaming()
```

**注意事项：**
- 从 `web/src/store/index.ts` 迁移现有逻辑，保持 API 兼容
- 先创建新文件，不立即修改旧 store，避免破坏
- 保持 `initialState` 结构与原 store 一致

**Commit 信息：**
```
feat(store): add StreamingStore for high-frequency state isolation

- Create independent StreamingStore for streaming content blocks
- Isolate streaming state from completed messages to reduce re-render scope
- Phase 1 of thread rendering performance optimization
```

---

#### Task 1.2: 创建 useChunkBatcher Hook

**文件：** `web/src/hooks/useChunkBatcher.ts`（新建）

**实现要点：**
```typescript
interface ChunkBatcherOptions {
  flushInterval?: number           // 默认 100ms
  onFlush: (chunks: Chunk[]) => void
  type?: 'text' | 'thinking'       // 按类型区分间隔
}

export function useChunkBatcher(options: ChunkBatcherOptions): {
  enqueue(chunk: Chunk): void
  flushImmediately(): void
}
```

**类型区分策略：**
| 类型 | 刷新间隔 | 说明 |
|------|----------|------|
| text | 100ms | 文本输出，需要流畅感 |
| thinking | 200ms | 思考过程，用户不细看，可以批处理更多 |
| tool_use | 0ms (立即) | 工具调用状态变化，需要及时反馈 |

---

#### Task 2.2 (增强): 高度估算 + 自适应测量（防白屏三重保障）

**核心原理：** 估算只是"占位"，真正渲染后 `measureElement` 会自动测量真实高度并修正。估算值宁可偏高也不能偏低。

**保障措施 1: 保守估算 + 20% 安全余量**
```typescript
export function estimateMessageHeight(message: Message): number {
  let height = 72  // 基础高度
  
  if (message.contentBlocks?.length) {
    for (const block of message.contentBlocks) {
      height += estimateBlockHeight(block)
    }
  } else if (message.content) {
    height += Math.ceil(message.content.length / 65) * 24
  }
  
  // ✅ 增加 20% 安全余量，宁可多渲染也不出现白屏
  return Math.max(height, 88) * 1.2
}
```

**保障措施 2: 消息量少时不启用虚拟化**
```typescript
// 小于 20 条消息时直接渲染，跳过虚拟化（避免初始化阶段的白屏）
const VIRTUALIZATION_THRESHOLD = 20

if (messages.length < VIRTUALIZATION_THRESHOLD) {
  return messages.map(msg => <ChatMessage message={msg} ... />)
}

// >= 20 条时才启用虚拟列表
const virtualizer = useVirtualizer({ ... })
```

**保障措施 3: 动态 overscan 自适应**
```typescript
// 滚动时如果发现空白，自动增加 overscan
const handleScroll = useThrottle(() => {
  const viewportHeight = listRef.current?.clientHeight || 600
  const visibleItems = virtualizer.getVirtualItems().length
  const expectedItems = Math.ceil(viewportHeight / 80) + 8  // 期望值
  
  if (visibleItems < expectedItems) {
    // 实际渲染的比预期少，临时增加 overscan
    virtualizer.setOptions({ overscan: 12 })
  }
}, 100)
```

**为什么这三重保障有效：**
- 估算 × 1.2：绝大多数情况下估算高度 > 真实高度，滚动条不会跳动
- 消息量少时不虚拟化：对话初期完全没有虚拟列表的副作用
- 动态 overscan：如果出现白屏，系统会自动增加缓冲区域

**Commit 信息（更新）：**

**Commit 信息：**
```
feat(hooks): add useChunkBatcher with type-aware flush intervals

- 100ms for text, 200ms for thinking, immediate for tool_use
- Reduce state update frequency during streaming
- Support immediate flush for critical status changes
```

---

#### Task 1.3: 集成 useChunkBatcher 到 useWebSocket

**文件：** `web/src/hooks/useWebSocket.ts`（修改）

**修改点：**
1. 导入 `useChunkBatcher`
2. 在 ws `onmessage` 中按消息类型路由：
   ```typescript
   ws.onmessage = (event) => {
     const data = JSON.parse(event.data)
     
     if (data.type === 'agent_output_chunk') {
       // 按块类型决定批处理策略
       const blockType = data.payload?.type || 'text'
       if (blockType === 'thinking') {
         enqueueThinkingChunk(data)  // 200ms 批处理
       } else {
         enqueueTextChunk(data)      // 100ms 批处理
       }
     } else if (data.type === 'agent_status') {
       flushImmediately()  // 先清空批处理队列
       handleAgentStatus(data)
     } else {
       flushImmediately()  // 其他消息立即处理
       handleWsMessage(data)
     }
   }
   ```
3. visibilitychange 时清空队列并立即刷新

**注意事项：**
- 确保批处理不会丢失最后一条消息（cleanup 时必须 flush）
- Agent 完成时必须立即刷新，避免最后几个字符延迟显示

**Commit 信息：**
```
refactor(ws): integrate chunk batcher with type-aware intervals

- Route agent_output_chunk to different batchers by block type
- Flush immediately on agent_status changes and cleanup
- Reduce render frequency during streaming output
```

---

#### Task 1.4: 迁移 StreamingMessage 到 StreamingStore

**文件：** `web/src/components/thread/StreamingMessage.tsx`（修改）

**修改点：**
1. 导入 `useStreamingStore` 替换 `useAppStore`
2. 修改订阅方式：
   ```typescript
   // 前：
   const isStreaming = useAppStore(s => s.isStreaming)
   
   // 后：
   const isStreaming = useStreamingStore(s => s.isStreaming)
   ```
3. 保持所有 props 不变，确保父组件无需修改

**验证：**
- 运行 `npm run build` 无 TS 错误
- 手动测试流式输出正常显示

**Commit 信息：**
```
refactor(StreamingMessage): use StreamingStore instead of useAppStore

- Subscribe to isolated StreamingStore for streaming state
- Break re-render chain from main AppStore
- No API changes for parent components
```

---

#### ✅ Phase 1 验证检查点

- [ ] `npm run build` 无错误
- [ ] 手动测试：Agent 流式输出正常显示
- [ ] 手动测试：思考块、工具调用正常显示
- [ ] React DevTools：StreamingMessage 组件重渲染频率明显降低

---

### 🔹 Phase 2: 虚拟列表引入（优先级：最高）

**目标：** 解决长对话 DOM 膨胀问题

---

#### Task 2.1: 安装 @tanstack/react-virtual

**文件：** `web/package.json`（修改）

**命令：**
```bash
cd web && npm install @tanstack/react-virtual
```

**Commit 信息：**
```
chore: add @tanstack/react-virtual dependency
```

---

#### Task 2.2: 创建高度估算函数

**文件：** `web/src/utils/messageHeightEstimate.ts`（新建）

**实现要点：**
```typescript
export function estimateMessageHeight(message: Message): number {
  let height = 72  // 头像 + 边距基础高度
  
  if (message.contentBlocks?.length) {
    for (const block of message.contentBlocks) {
      height += estimateBlockHeight(block)
    }
  } else if (message.content) {
    // 纯文本按字符数估算
    height += Math.ceil(message.content.length / 65) * 24
  }
  
  return Math.max(height, 88)
}

function estimateBlockHeight(block: MessageContentBlock): number {
  switch (block.type) {
    case 'text':      return Math.ceil((block.content?.length||0)/65)*24 + 16
    case 'thinking':  return block.status === 'streaming' ? 88 : 44
    case 'tool_use':  return 68
    case 'tool_result': return Math.min(Math.ceil((block.output?.length||0)/50)*20 + 48, 320)
    case 'question':  return 130
    case 'rich':      return estimateRichBlockHeight(block)
    default:          return 44
  }
}
```

**注意事项：**
- 估算值宁可偏高不要偏低（偏高只是多渲染一点，偏低会导致滚动条跳动）
- 考虑深色/浅色主题对高度无影响，字体大小默认 14px

**Commit 信息：**
```
feat(utils): add message height estimation with safety margin

- Estimate height by content block types and lengths
- 1.2x safety margin to prevent white flash during scroll
- < 20 messages threshold: disable virtualization for short threads
- Dynamic overscan auto-adjust if white space detected
```

---

#### Task 2.3: 重构 ChatMessageList 使用虚拟列表

**文件：** `web/src/components/thread/ChatMessageList.tsx`（修改）

**实现要点：**

```typescript
import { useVirtualizer } from '@tanstack/react-virtual'
import { estimateMessageHeight } from '@/utils/messageHeightEstimate'

// 关键配置
const virtualizer = useVirtualizer({
  count: messages.length,
  getScrollElement: () => listRef.current,
  estimateSize: (index) => estimateMessageHeight(messages[index]),
  overscan: 8,  // ✅ 从 5 提升到 8，增强防白屏
  measureElement: (element) => {
    // ✅ 动态测量，自动修正估算偏差（真正的"自适应"）
    return element.getBoundingClientRect().height
  }
})

// ✅ 消息量 < 20 时不启用虚拟化，直接渲染
if (messages.length < 20) {
  return (
    <div ref={listRef} className="chat-message-list">
      {messages.map(msg => <ChatMessage key={msg.id} message={msg} ... />)}
      <StreamingMessage ... />
    </div>
  )
}

// 渲染方式
<div ref={listRef} className="chat-message-list" style={{ overflowY: 'auto', height: '100%' }}>
  {/* 加载更多指示器 */}
  {hasMoreHistory && <LoadMoreIndicator />}
  
  {/* 虚拟列表容器 */}
  <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
    {virtualizer.getVirtualItems().map((virtualItem) => (
      <div
        key={messages[virtualItem.index].id}
        data-index={virtualItem.index}
        ref={(el) => virtualItem.measure(el)}  // ✅ 动态测量
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          width: '100%',
          transform: `translateY(${virtualItem.start}px)`
        }}
      >
        <ChatMessage message={messages[virtualItem.index]} ... />
      </div>
    ))}
  </div>
  
  {/* 流式消息保持在虚拟列表外（独立渲染） */}
  <StreamingMessage ... />
</div>
```

**连贯性保障措施：**

| 场景 | 处理方案 |
|------|----------|
| 流式输出期间 | StreamingMessage 不在虚拟列表中，独立渲染，不受影响 |
| 展开/折叠 thinking 块 | measureElement 自动重新测量高度，虚拟器平滑调整 |
| 动态高度变化 | 相同机制，测量后自动修正 |
| 切屏返回滚动位置 | Zustand store 保存 `scrollTop`，mount 时调用 `virtualizer.scrollToOffset()` |
| 滚动白屏 | overscan = 8，预渲染足够多的上下缓冲区域 |

**滚动位置保存：**
```typescript
// 在 ChatMessageList 中
const handleScroll = useThrottle(() => {
  if (listRef.current) {
    useAppStore.getState().setThreadScrollPosition(listRef.current.scrollTop)
  }
}, 100)

// ThreadView 恢复
useEffect(() => {
  const savedScroll = useAppStore.getState().threadScrollPosition
  if (savedScroll && virtualizerRef.current) {
    virtualizerRef.current.scrollToOffset(savedScroll)
  }
}, [virtualizerRef])
```

**Commit 信息：**
```
refactor(ChatMessageList): implement virtual list with @tanstack/react-virtual

- Replace messages.map with windowed rendering
- overscan = 8 to prevent white flash during normal scroll
- Dynamic measurement with measureElement corrects estimation drift
- Preserve scroll position across tab switches
- Keep StreamingMessage outside virtual list for independent rendering
- Ensure coherence during streaming, expand/collapse, and scroll
```

---

#### ✅ Phase 2 验证检查点

- [ ] `npm run build` 无错误
- [ ] 手动测试：消息 < 20 条时直接渲染，不使用虚拟列表
- [ ] 手动测试：消息 > 20 条时启用虚拟化，滚动平滑
- [ ] 手动测试：滚动 50+ 条消息无明显白屏（正常滚动速度）
- [ ] 手动测试：流式输出期间滚动，消息不跳跃
- [ ] 手动测试：展开/折叠 thinking 块，滚动位置稳定
- [ ] 手动测试：切屏返回后滚动位置准确恢复
- [ ] 手动测试：快速滚动允许短暂空白，空白区域 1 帧内填充完成
- [ ] React DevTools：DOM 节点数量显著减少（视口内 ~10-15 条 vs 全部渲染）

---

### 🔹 Phase 3: 组件性能优化（优先级：高）

**目标：** 消除不必要的重渲染

---

#### Task 3.1: ChatMessage 添加自定义 memo 比较

**文件：** `web/src/components/thread/ChatMessage.tsx`（修改）

**实现要点：**
```typescript
export const ChatMessage = memo(
  ChatMessageComponent,
  (prevProps, nextProps) => {
    // 只比较真正影响渲染的属性
    return (
      prevProps.message.id === nextProps.message.id &&
      prevProps.message.content === nextProps.message.content &&
      prevProps.message.contentBlocks?.length === nextProps.message.contentBlocks?.length &&
      // 只比较 contentBlocks 最后一条（流式更新只有最后一条变化）
      (prevProps.message.contentBlocks?.length === 0 || 
       JSON.stringify(prevProps.message.contentBlocks?.slice(-1)) === 
       JSON.stringify(nextProps.message.contentBlocks?.slice(-1))) &&
      prevProps.toolEvents?.length === nextProps.toolEvents?.length
    )
  }
)
```

**原理：** 已完成的消息 99% 时间不会变化，memo 可以跳过绝大多数重渲染。只有最后一条消息在流式输出时会变化。

**Commit 信息：**
```
perf(ChatMessage): add custom memo comparison function

- Compare only render-critical props
- Skip full content block comparison for completed messages
- Dramatically reduce re-renders during streaming output
```

---

#### Task 3.2: MessageContentRenderer 移除生产环境日志

**文件：** `web/src/components/thread/ContentBlock/MessageContentRenderer.tsx`（修改）

**修改点：**
```typescript
// 将所有 console.log 改为条件输出
const DEBUG = process.env.NODE_ENV === 'development' && 
              localStorage.getItem('debug_renderer') === 'true'

DEBUG && console.log('[MessageContentRenderer]', ...)
```

**Commit 信息：**
```
perf(MessageContentRenderer): conditional debug logs

- Only log when debug_renderer=true in localStorage
- Remove production console.log overhead
```

---

#### Task 3.3: 创建 useThrottle 并优化滚动

**文件 1：** `web/src/hooks/useThrottle.ts`（新建）

```typescript
export function useThrottle<T extends (...args: unknown[]) => void>(
  callback: T,
  delay: number  // 推荐 16ms = 1 帧
): T {
  // 标准 throttle 实现，使用 requestAnimationFrame 时机
}
```

**文件 2：** `web/src/components/thread/ChatMessageList.tsx`（修改）

```typescript
const handleScroll = useThrottle(() => {
  // 滚动位置保存 + 加载更多检测
}, 16)  // 16ms = 60fps，最多每帧处理一次
```

**Commit 信息：**
```
feat(hooks): add useThrottle for scroll event optimization

refactor(ChatMessageList): throttle scroll handler to 16ms

- Limit scroll processing to once per frame
- Reduce layout thrashing during fast scroll
```

---

#### ✅ Phase 3 验证检查点

- [ ] `npm run build` 无错误
- [ ] React DevTools：滚动时 ChatMessage 重渲染数量明显减少
- [ ] React DevTools：只有视口内的消息会重渲染

---

### 🔹 Phase 4: 切屏返回体验优化（优先级：高）

**目标：** 消除切屏白屏和卡顿

---

#### Task 4.1: 创建 LoadingBar 组件

**文件：** `web/src/components/LoadingBar.tsx` + `LoadingBar.css`（新建）

**实现要点：**
- 顶部 3px 高度的渐变进度条
- 仅在 `visible=true` 时显示
- CSS 动画，无需 JS 驱动

**Commit 信息：**
```
feat: add LoadingBar lightweight loading indicator
```

---

#### Task 4.2: 创建 ThreadSkeleton 骨架屏

**文件：** `web/src/components/ThreadSkeleton.tsx` + `ThreadSkeleton.css`（新建）

**实现要点：**
- 3 条消息的骨架占位
- 头部标题骨架 + 输入区骨架
- CSS 渐变脉冲动画

**Commit 信息：**
```
feat: add ThreadSkeleton for initial loading state
```

---

#### Task 4.3: ThreadView 实现分级加载

**文件：** `web/src/pages/ThreadView.tsx`（修改）

**修改点：**
```typescript
// 有数据时：显示内容 + 顶部 LoadingBar 表示后台刷新
if (messages.length > 0) {
  return (
    <div className="thread-view-wrapper">
      <LoadingBar visible={loading} />
      {/* 原有内容 */}
    </div>
  )
}

// 无数据首次加载：显示骨架屏
if (loading) {
  return <ThreadSkeleton />
}

// 正常渲染
return <div className="thread-view-wrapper">{/* 原有内容 */}</div>
```

**关键：** 有数据时永远不显示全屏 Spinner，避免白屏感。LoadingBar 是 3px 的细条，不阻塞交互。

**Commit 信息：**
```
refactor(ThreadView): implement tiered loading states

- Show only top LoadingBar when data already exists (no white flash)
- Show ThreadSkeleton only on first empty load
- Never block UI with full-page spinner when returning to tab
```

---

#### Task 4.4: 优化 loadThread 增量更新逻辑

**文件：** `web/src/store/index.ts`（修改）

**修改点：** ❌ 移除缓存 TTL 逻辑，改用以下策略：

```typescript
loadThread: async (threadId) => {
  const state = get()
  
  // ✅ 已有数据且是同一 thread：后台静默刷新，不清除现有数据
  if (state.currentThread?.id === threadId && state.messages.length > 0) {
    // 只设置 loading 标志，不清除 messages
    set({ loading: true })
    
    try {
      // 后台请求最新数据
      const [threadData, invocations] = await Promise.all([
        api.threads.get(threadId),
        api.invocations.list(threadId)
      ])
      
      // ✅ 增量更新：只合并差异，不替换整个数组
      set({
        activeAgents: invocations.filter(i => i.status === 'running'),
        completedAgents: invocations.filter(i => 
          ['completed', 'failed', 'interrupted'].includes(i.status)
        ),
        loading: false
      })
      
      console.log('[loadThread] Background refresh complete')
    } catch (e) {
      set({ loading: false })
      throw e
    }
    
    return
  }
  
  // ❌ 新 thread：完整加载逻辑保持不变
  // ...
}
```

**原理：** Zustand store 是单例，只要组件没被 React Router 卸载，内存中就有数据。我们直接用这些数据渲染，后台默默刷新。

**Commit 信息：**
```
feat(store): background incremental refresh for tab switch

- Keep existing messages in memory when returning to same thread
- Refresh agent status silently in background
- No TTL cache - always fresh but non-blocking
- No white flash - data renders instantly
```

---

#### Task 4.5: visibilitychange WebSocket 重连

**文件：** `web/src/hooks/useWebSocket.ts`（修改）

**实现要点：**
```typescript
useEffect(() => {
  const handleVisibilityChange = () => {
    if (document.visibilityState === 'visible') {
      // 页面变为可见
      if (threadId) {
        // 检查连接状态
        if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
          console.log('[WebSocket] Page visible, reconnecting...')
          connectWebSocket(threadId)
        } else {
          console.log('[WebSocket] Connection alive, no action needed')
        }
        
        // ✅ 触发后台增量刷新（即使 WS 连接正常）
        useAppStore.getState().loadThread(threadId)
      }
    }
  }
  
  document.addEventListener('visibilitychange', handleVisibilityChange)
  
  return () => {
    document.removeEventListener('visibilitychange', handleVisibilityChange)
    // 注意：不要在这里 close WS！React Router 可能会频繁 mount/unmount
  }
}, [threadId])
```

**关键：** 页面可见时不仅重连 WS，还主动触发一次后台刷新，确保状态同步。

**Commit 信息：**
```
feat(ws): visibilitychange reconnect + background refresh

- Check and reconnect WebSocket when page becomes visible
- Trigger incremental store refresh on tab return
- Ensure state sync without blocking UI
```

---

#### ✅ Phase 4 验证检查点

- [ ] `npm run build` 无错误
- [ ] 手动测试：切屏返回无白屏，内容立即显示
- [ ] 手动测试：切屏返回时顶部 LoadingBar 短暂显示（表示后台刷新）
- [ ] 手动测试：有 Agent 运行时切屏返回，Agent 状态正确更新
- [ ] 手动测试：首次进入新对话，骨架屏正常显示替代白屏

---

### 🔹 Phase 5: 验收测试（优先级：中）

#### Task 5.1: 创建 E2E 性能测试

**文件：** `auto-test/e2e/thread-performance.spec.ts`（新建）

**测试用例：**
```typescript
test.describe('Thread Rendering Performance', () => {
  
  test('切屏返回显示时间 < 100ms', async ({ page }) => {
    // 预加载 -> 切走 -> 切回 -> 计时
  })
  
  test('100条消息滚动 FPS ≥ 50', async ({ page }) => {
    // 加载测试数据 -> 模拟滚动 -> FPS 计算
  })
  
  test('流式输出 FPS ≥ 30', async ({ page }) => {
    // 启动 Agent -> 测量 3 秒内的帧时间
  })
  
  test('正常滚动无可见白屏', async ({ page }) => {
    // 截图对比？或者人工验收
  })
})
```

**Commit 信息：**
```
test: add thread rendering performance E2E tests

- Tab switch return timing (< 100ms)
- Scroll FPS with 100 messages (≥ 50)
- Streaming output FPS (≥ 30)
```

---

## 🚦 执行顺序和里程碑

```
里程碑 1: 状态管理重构完成 (Phase 1)
  ↓
里程碑 2: 虚拟列表上线 (Phase 2)
  ↓         ↓
里程碑 3: 组件优化完成 (Phase 3)
  ↓         ↓         ↓
里程碑 4: 切屏体验优化完成 (Phase 4)
  ↓
里程碑 5: 测试通过，发布 (Phase 5)
```

**每个 Phase 之间可以独立评审和测试，不阻塞后续开发。**

---

## ⚠️ 风险和回滚策略

| 风险 | 影响 | 概率 | 回滚方案 |
|------|------|------|----------|
| 虚拟列表导致滚动异常 | 高 | 中 | 回滚 Task 2.3，保留其他优化 |
| 批处理导致消息丢失 | 高 | 低 | 调整 flush 时机，cleanup 强制 flush |
| StreamingStore 与原 store 状态不一致 | 中 | 低 | 先做功能开关，A/B 测试 |
| 滚动位置恢复不准确 | 中 | 中 | 调整 overscan 和估算高度 |

**所有变更都是渐进式的，任何单一模块都可以独立回滚而不影响其他优化。**

---

## 📊 预期效果（对比优化前）

| 指标 | 优化前（估算） | 优化后（目标） | 提升 |
|------|---------------|---------------|------|
| 切屏返回时间 | 300-800ms | < 100ms | 3-8x |
| 100 条消息 DOM 节点 | ~2000 个 | ~300 个 | 85% 减少 |
| 流式输出重渲染频率 | 每字符 1 次 | 每 100ms 1 次 | 10-20x 减少 |
| 滚动 FPS | ~25-35 | ≥ 50 | +50-100% |

---

## ✅ 最终确认清单

执行前请确认：
- [ ] 否决 5 分钟 TTL 缓存，改用内存状态 + 后台刷新 ✅
- [ ] 虚拟列表 overscan = 8 ✅
- [ ] 批处理按类型区分间隔 ✅
- [ ] 切屏返回时 LoadingBar 轻量反馈 ✅
- [ ] 5 个连贯性场景全部覆盖 ✅

**确认无误后即可开始执行。**
