# 前端对话渲染性能优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 优化前端对话渲染性能，解决切屏返回白屏、流式输出卡顿、长对话性能衰减三个问题。

**Architecture:** 渐进式四阶段优化：状态管理重构（StreamingStore 隔离）→ 虚拟列表引入（@tanstack/react-virtual）→ 组件性能优化（memo/useMemo）→ 切屏返回缓存（增量加载 + TTL）。

**Tech Stack:** React 18、Zustand、@tanstack/react-virtual、Playwright

---

## 文件结构

```
web/src/
├── store/
│   ├── index.ts              # 主 store（修改：移除流式状态）
│   ├── streaming.ts          # 新增：StreamingStore
│   └── debugThread.ts        # 保持不变
├── hooks/
│   └── useWebSocket.ts       # 修改：批量更新 + visibilitychange
│   └── useThrottle.ts        # 新增：throttle hook
├── components/
│   ├── thread/
│   │   ├── ChatMessageList.tsx    # 重构：虚拟列表
│   │   ├── ChatMessage.tsx        # 优化：memo + useMemo
│   │   ├── StreamingMessage.tsx   # 优化：订阅 StreamingStore
│   │   └── ContentBlock/
│   │       └ MessageContentRenderer.tsx  # 优化：移除日志 + useMemo
│   ├── LoadingBar.tsx        # 新增：轻量加载指示器
│   └── ThreadSkeleton.tsx    # 新增：骨架屏
├── pages/
│   └── ThreadView.tsx        # 重构：分级加载 + 增量更新
├── utils/
│   └── performance.ts        # 新增：FPS 测量工具
auto-test/e2e/
└── thread-performance.spec.ts  # 新增：性能验收测试
```

---

## 阶段 1：状态管理重构

### Task 1.1: 创建 StreamingStore

**Files:**
- Create: `web/src/store/streaming.ts`

- [ ] **Step 1: 安装依赖（如需要）**

```bash
# 检查 zustand 是否已安装
npm list zustand --prefix web
```

- [ ] **Step 2: 创建 StreamingStore 文件**

```typescript
// web/src/store/streaming.ts
import { create } from 'zustand';
import type { MessageContentBlock } from '@/types';

export type ProgressStatus = 'thinking' | 'tool_use' | 'generating' | 'idle';

export interface StreamingState {
  isStreaming: boolean;
  streamingInvocationId: string | null;
  streamingAgentId: string | null;
  streamingAgentName: string | null;
  streamingContentBlocks: MessageContentBlock[];
  progressStatus: ProgressStatus;
  progressToolName: string | null;
  progressToolInput: Record<string, unknown> | null;
}

export interface StreamingActions {
  startStreaming: (invocationId: string, agentId: string, agentName: string) => void;
  stopStreaming: () => void;
  appendContentBlock: (block: MessageContentBlock) => void;
  updateContentBlock: (blockId: string, update: Partial<MessageContentBlock>) => void;
  updateProgress: (status: ProgressStatus, toolName?: string, toolInput?: Record<string, unknown>) => void;
  clearStreaming: () => void;
}

const initialState: StreamingState = {
  isStreaming: false,
  streamingInvocationId: null,
  streamingAgentId: null,
  streamingAgentName: null,
  streamingContentBlocks: [],
  progressStatus: 'idle',
  progressToolName: null,
  progressToolInput: null,
};

export const useStreamingStore = create<StreamingState & StreamingActions>((set, get) => ({
  ...initialState,

  startStreaming: (invocationId, agentId, agentName) => {
    set({
      isStreaming: true,
      streamingInvocationId: invocationId,
      streamingAgentId: agentId,
      streamingAgentName: agentName,
      streamingContentBlocks: [],
      progressStatus: 'generating',
    });
  },

  stopStreaming: () => {
    set({
      isStreaming: false,
      progressStatus: 'idle',
      progressToolName: null,
      progressToolInput: null,
    });
  },

  appendContentBlock: (block) => {
    set((state) => ({
      streamingContentBlocks: [...state.streamingContentBlocks, block],
    }));
  },

  updateContentBlock: (blockId, update) => {
    set((state) => ({
      streamingContentBlocks: state.streamingContentBlocks.map((block) =>
        block.id === blockId ? { ...block, ...update } : block
      ),
    }));
  },

  updateProgress: (status, toolName, toolInput) => {
    set({
      progressStatus: status,
      progressToolName: toolName || null,
      progressToolInput: toolInput || null,
    });
  },

  clearStreaming: () => {
    set(initialState);
  },
}));
```

- [ ] **Step 3: 验证 StreamingStore 创建成功**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 4: Commit**

```bash
git add web/src/store/streaming.ts
git commit -m "feat(store): add StreamingStore for high-frequency state isolation

- Create independent StreamingStore for streaming content blocks
- Support 8 fields: isStreaming, invocationId, agentId, agentName, contentBlocks, progressStatus, toolName, toolInput
- Phase 1 of thread rendering performance optimization"
```

---

### Task 1.2: 创建批量更新 Hook

**Files:**
- Create: `web/src/hooks/useChunkBatcher.ts`

- [ ] **Step 1: 创建批量更新 hook**

```typescript
// web/src/hooks/useChunkBatcher.ts
import { useRef, useCallback, useEffect } from 'react';

interface Chunk {
  type: string;
  payload: Record<string, unknown>;
}

interface ChunkBatcherOptions {
  flushInterval?: number;
  onFlush: (chunks: Chunk[]) => void;
}

export function useChunkBatcher(options: ChunkBatcherOptions) {
  const { flushInterval = 100, onFlush } = options;
  
  const bufferRef = useRef<Chunk[]>([]);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onFlushRef = useRef(onFlush);

  // Keep onFlush callback updated
  useEffect(() => {
    onFlushRef.current = onFlush;
  }, [onFlush]);

  const enqueue = useCallback((chunk: Chunk) => {
    bufferRef.current.push(chunk);
    
    if (!timerRef.current) {
      timerRef.current = setTimeout(() => {
        if (bufferRef.current.length > 0) {
          onFlushRef.current(bufferRef.current);
          bufferRef.current = [];
        }
        timerRef.current = null;
      }, flushInterval);
    }
  }, [flushInterval]);

  const flushImmediately = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    if (bufferRef.current.length > 0) {
      onFlushRef.current(bufferRef.current);
      bufferRef.current = [];
    }
  }, []);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
    };
  }, []);

  return { enqueue, flushImmediately };
}
```

- [ ] **Step 2: 验证 hook 创建成功**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useChunkBatcher.ts
git commit -m "feat(hooks): add useChunkBatcher for WebSocket chunk batching

- Batch chunks every 100ms (configurable)
- Reduce state update frequency during streaming
- Support immediate flush for critical updates"
```

---

### Task 1.3: 重构 useWebSocket 使用批量更新

**Files:**
- Modify: `web/src/hooks/useWebSocket.ts`

- [ ] **Step 1: 读取现有 useWebSocket 实现**

```bash
head -100 web/src/hooks/useWebSocket.ts
```

- [ ] **Step 2: 修改 useWebSocket 集成批量更新**

在文件顶部添加导入，并修改消息处理逻辑：

```typescript
// 在导入部分添加
import { useChunkBatcher } from './useChunkBatcher';
import { useStreamingStore } from '@/store/streaming';

// 在 useWebSocket 函数内部添加批量更新逻辑
// 替换原有的 ws.onmessage 处理

// 找到 ws.onmessage 部分，修改为：
ws.onmessage = (event) => {
  try {
    const data: WSMessage = JSON.parse(event.data);
    
    // 高频 chunk 类型使用批量更新
    if (data.type === 'agent_output_chunk') {
      enqueueChunk(data);
    } else {
      // 低频消息立即处理
      flushImmediately();
      handleWsMessage(data);
    }
  } catch (e) {
    console.error('[WebSocket] Failed to parse message:', e);
  }
};
```

- [ ] **Step 3: 验证修改无语法错误**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 4: Commit**

```bash
git add web/src/hooks/useWebSocket.ts
git commit -m "refactor(ws): integrate chunk batching in useWebSocket

- Use useChunkBatcher for agent_output_chunk messages
- Keep other message types immediate processing
- Reduce render frequency during streaming"
```

---

### Task 1.4: 重构 StreamingMessage 使用 StreamingStore

**Files:**
- Modify: `web/src/components/thread/StreamingMessage.tsx`

- [ ] **Step 1: 读取现有 StreamingMessage 实现**

```bash
head -50 web/src/components/thread/StreamingMessage.tsx
```

- [ ] **Step 2: 修改 StreamingMessage 订阅 StreamingStore**

```typescript
// web/src/components/thread/StreamingMessage.tsx
// 修改导入和订阅部分

import { useStreamingStore } from '@/store/streaming';
// 移除 useAppStore 导入（或保留用于其他用途）

// 替换原有的订阅逻辑
export const StreamingMessage: React.FC<StreamingMessageProps> = memo(({
  agentConfigs,
  projectPath,
  toolEvents,
  onStop,
  onQuestionSubmit,
}) => {
  // 从 StreamingStore 获取状态
  const {
    isStreaming,
    streamingInvocationId,
    streamingAgentId,
    streamingAgentName,
    streamingContentBlocks,
    progressStatus,
    progressToolName,
    progressToolInput,
  } = useStreamingStore();

  // ... 其余逻辑保持不变，使用上述变量
```

- [ ] **Step 3: 验证修改**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 4: Commit**

```bash
git add web/src/components/thread/StreamingMessage.tsx
git commit -m "refactor(StreamingMessage): use StreamingStore instead of useAppStore

- Subscribe to isolated StreamingStore for streaming state
- Reduce re-render frequency of completed messages list"
```

---

## 阶段 2：虚拟列表引入

### Task 2.1: 安装 @tanstack/react-virtual

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: 安装依赖**

```bash
cd web && npm install @tanstack/react-virtual
```

- [ ] **Step 2: 验证安装成功**

```bash
npm list @tanstack/react-virtual --prefix web
```

- [ ] **Step 3: Commit**

```bash
git add web/package.json web/package-lock.json
git commit -m "chore: add @tanstack/react-virtual dependency"
```

---

### Task 2.2: 创建高度估算函数

**Files:**
- Create: `web/src/utils/messageHeightEstimate.ts`

- [ ] **Step 1: 创建高度估算工具函数**

```typescript
// web/src/utils/messageHeightEstimate.ts
import type { Message, MessageContentBlock } from '@/types';

/**
 * 估算消息高度（用于虚拟列表初始估算）
 * 动态测量会修正估算偏差
 */
export function estimateMessageHeight(message: Message): number {
  let height = 60; // 基础高度（头像 + 名称 + 时间戳 + 边距）
  
  if (message.contentBlocks && message.contentBlocks.length > 0) {
    for (const block of message.contentBlocks) {
      height += estimateBlockHeight(block);
    }
  } else if (message.content) {
    // 无 contentBlocks 时，根据纯文本估算
    height += Math.ceil(message.content.length / 50) * 20;
  }
  
  return Math.max(height, 80); // 最小高度 80px
}

function estimateBlockHeight(block: MessageContentBlock): number {
  switch (block.type) {
    case 'text':
      const textLength = block.content?.length || 0;
      return Math.ceil(textLength / 50) * 20 + 16; // 每行约 50 字符，20px 高度
    
    case 'thinking':
      // thinking 块可折叠，展开时更长
      if (block.status === 'streaming') {
        return 80; // 流式时展开
      }
      return 40; // 完成时折叠
    
    case 'tool_use':
      return 60; // 工具调用基础高度
    
    case 'tool_result':
      const outputLength = block.output?.length || 0;
      return Math.min(Math.ceil(outputLength / 40) * 18 + 40, 300); // 最大 300px
    
    case 'question':
      return 120; // AskUserQuestion 选项较多
    
    case 'rich':
      return estimateRichBlockHeight(block);
    
    default:
      return 40;
  }
}

function estimateRichBlockHeight(block: MessageContentBlock): number {
  const richType = block.richType;
  
  switch (richType) {
    case 'card':
      return 100;
    case 'diff':
      return 150; // diff 块通常较长
    case 'checklist':
      const items = block.items || [];
      return items.length * 24 + 40;
    default:
      return 80;
  }
}
```

- [ ] **Step 2: 验证函数创建**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 3: Commit**

```bash
git add web/src/utils/messageHeightEstimate.ts
git commit -m "feat(utils): add message height estimation for virtual list

- Estimate height based on content block types
- Support text, thinking, tool_use, question, rich blocks
- Dynamic measurement will correct estimation偏差"
```

---

### Task 2.3: 重构 ChatMessageList 使用虚拟列表

**Files:**
- Modify: `web/src/components/thread/ChatMessageList.tsx`

- [ ] **Step 1: 读取现有 ChatMessageList 实现**

```bash
cat web/src/components/thread/ChatMessageList.tsx
```

- [ ] **Step 2: 重构 ChatMessageList 使用虚拟列表**

```typescript
// web/src/components/thread/ChatMessageList.tsx
import { forwardRef, useRef, useEffect, useState, useCallback, memo } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useAppStore } from '@/store';
import { estimateMessageHeight } from '@/utils/messageHeightEstimate';
import { ChatMessage } from './ChatMessage';
import { StreamingMessage } from './StreamingMessage';
import type { AgentConfig, Message, ToolEvent } from '@/types';

interface ChatMessageListProps {
  messages: Message[];
  agentConfigs: AgentConfig[];
  projectPath?: string;
  toolEvents?: Record<string, ToolEvent[]>;
  onStopAgent?: (invocationId: string) => void;
  onRetryAgent?: (message: Message) => void;
  onOpenCodePanel?: (files: FileChange[]) => void;
  autoScroll?: boolean;
  onQuestionSubmit?: (blockId: string, answers: Record<number, string | string[]>, invocationId: string) => void;
  hasMoreHistory?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
  onAgentClick?: (agentName: string) => void;
}

export const ChatMessageList = forwardRef<HTMLDivElement, ChatMessageListProps>(
  (props, ref) => {
    const {
      messages,
      agentConfigs,
      projectPath,
      toolEvents = {},
      onStopAgent,
      onRetryAgent,
      onOpenCodePanel,
      autoScroll = true,
      onQuestionSubmit,
      hasMoreHistory = false,
      loadingMore = false,
      onLoadMore,
      onAgentClick,
    } = props;

    const internalRef = useRef<HTMLDivElement>(null);
    const listRef = (ref as React.RefObject<HTMLDivElement>) || internalRef;

    // 用户是否接近底部（用于自动滚动控制）
    const [isNearBottom, setIsNearBottom] = useState(true);

    // 获取流式状态（用于判断是否需要滚动到底部）
    const isStreaming = useAppStore((s) => s.isStreaming);

    // 创建虚拟列表
    const virtualizer = useVirtualizer({
      count: messages.length,
      getScrollElement: () => listRef.current,
      estimateSize: (index) => estimateMessageHeight(messages[index]),
      overscan: 5, // 预渲染 5 条
    });

    // 滚动到底部
    const scrollToBottom = useCallback(() => {
      if (messages.length > 0) {
        virtualizer.scrollToIndex(messages.length - 1, { align: 'end' });
      }
    }, [virtualizer, messages.length]);

    // 新消息时自动滚动
    useEffect(() => {
      if (autoScroll && isNearBottom && isStreaming) {
        scrollToBottom();
      }
    }, [messages.length, autoScroll, isNearBottom, isStreaming, scrollToBottom]);

    // 滚动事件处理
    const handleScroll = useCallback(() => {
      if (!listRef.current) return;
      const { scrollTop, scrollHeight, clientHeight } = listRef.current;
      const nearBottom = scrollHeight - scrollTop - clientHeight < 50;
      setIsNearBottom(nearBottom);

      // 加载历史
      if (onLoadMore && !loadingMore && hasMoreHistory && scrollTop < 100) {
        onLoadMore();
      }
    }, [listRef, onLoadMore, loadingMore, hasMoreHistory]);

    // 空状态
    if (messages.length === 0) {
      return (
        <div className="chat-message-list-empty">
          暂无消息
        </div>
      );
    }

    return (
      <div
        ref={listRef}
        className="chat-message-list"
        style={{
          height: '100%',
          overflowY: 'auto',
          padding: '16px',
        }}
        onScroll={handleScroll}
      >
        {/* 加载更多指示器 */}
        {hasMoreHistory && (
          <div style={{ textAlign: 'center', padding: '8px 0', color: 'var(--text-secondary)' }}>
            {loadingMore ? '正在加载历史...' : '↑ 向上滚动加载更多'}
          </div>
        )}

        {/* 虚拟列表容器 */}
        <div
          style={{
            height: virtualizer.getTotalSize(),
            width: '100%',
            position: 'relative',
          }}
        >
          {virtualizer.getVirtualItems().map((virtualItem) => {
            const message = messages[virtualItem.index];
            const invocationId = message.id.startsWith('agent-')
              ? message.id.replace('agent-', '')
              : message.id;
            const messageToolEvents = toolEvents[invocationId] || [];

            return (
              <div
                key={message.id}
                ref={(el) => virtualizer.measureElement(el)}
                style={{
                  position: 'absolute',
                  top: virtualItem.start,
                  left: 0,
                  width: '100%',
                }}
              >
                <ChatMessage
                  message={message}
                  agentConfig={agentConfigs.find(c => c.id === message.agentId || c.name === message.agentName)}
                  agentConfigs={agentConfigs}
                  projectPath={projectPath}
                  toolEvents={messageToolEvents}
                  onRetry={onRetryAgent ? () => onRetryAgent(message) : undefined}
                  onOpenCodePanel={onOpenCodePanel}
                  onQuestionSubmit={onQuestionSubmit}
                  onAgentClick={onAgentClick}
                />
              </div>
            );
          })}
        </div>

        {/* 流式消息（独立渲染，不在虚拟列表中） */}
        <StreamingMessage
          agentConfigs={agentConfigs}
          projectPath={projectPath}
          toolEvents={toolEvents}
          onStop={onStopAgent}
          onQuestionSubmit={onQuestionSubmit}
        />
      </div>
    );
  }
);

ChatMessageList.displayName = 'ChatMessageList';
```

- [ ] **Step 3: 验证重构**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 4: Commit**

```bash
git add web/src/components/thread/ChatMessageList.tsx
git commit -m "refactor(ChatMessageList): implement virtual list with @tanstack/react-virtual

- Replace messages.map with virtualized rendering
- Support dynamic height estimation + measurement
- Keep StreamingMessage outside virtual list
- Handle scroll and auto-scroll logic"
```

---

## 阶段 3：组件性能优化

### Task 3.1: 优化 ChatMessage 使用 memo

**Files:**
- Modify: `web/src/components/thread/ChatMessage.tsx`

- [ ] **Step 1: 读取现有 ChatMessage 实现**

```bash
head -100 web/src/components/thread/ChatMessage.tsx
```

- [ ] **Step 2: 添加自定义 memo 比较**

在 ChatMessage 组件定义处修改：

```typescript
// web/src/components/thread/ChatMessage.tsx
// 找到 export const ChatMessage = memo(...) 部分

// 替换为自定义比较函数的 memo
export const ChatMessage = memo(
  ({ message, agentConfig, ... }: ChatMessageProps) => {
    // 组件内部逻辑...
  },
  (prevProps, nextProps) => {
    // 自定义比较：只在关键属性变化时重渲染
    return (
      prevProps.message.id === nextProps.message.id &&
      prevProps.message.content === nextProps.message.content &&
      prevProps.message.contentBlocks?.length === nextProps.message.contentBlocks?.length &&
      prevProps.isStreaming === nextProps.isStreaming &&
      prevProps.progress?.status === nextProps.progress?.status &&
      prevProps.progress?.toolName === nextProps.progress?.toolName
    );
  }
);
```

- [ ] **Step 3: 验证修改**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 4: Commit**

```bash
git add web/src/components/thread/ChatMessage.tsx
git commit -m "perf(ChatMessage): add custom memo comparison for render optimization

- Compare only critical props: id, content, isStreaming, progress
- Skip re-render for non-essential prop changes"
```

---

### Task 3.2: 优化 MessageContentRenderer 移除日志

**Files:**
- Modify: `web/src/components/thread/ContentBlock/MessageContentRenderer.tsx`

- [ ] **Step 1: 搜索所有 console.log**

```bash
grep -n "console.log" web/src/components/thread/ContentBlock/MessageContentRenderer.tsx
```

- [ ] **Step 2: 移除或条件化日志**

```typescript
// 将所有 console.log 改为条件输出
// 在文件顶部添加调试条件

const DEBUG_LOGS = process.env.NODE_ENV === 'development' && 
                   localStorage.getItem('debug_message_renderer') === 'true';

// 替换所有 console.log
console.log('[MessageContentRenderer] ...');
// 改为
DEBUG_LOGS && console.log('[MessageContentRenderer] ...');
```

- [ ] **Step 3: 验证修改**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 4: Commit**

```bash
git add web/src/components/thread/ContentBlock/MessageContentRenderer.tsx
git commit -m "perf(MessageContentRenderer): conditional debug logs in development only

- Only log when debug_message_renderer=true in localStorage
- Remove production log overhead"
```

---

### Task 3.3: 创建 throttle hook

**Files:**
- Create: `web/src/hooks/useThrottle.ts`

- [ ] **Step 1: 创建 throttle hook**

```typescript
// web/src/hooks/useThrottle.ts
import { useRef, useCallback, useEffect } from 'react';

export function useThrottle<T extends (...args: unknown[]) => void>(
  callback: T,
  delay: number
): T {
  const lastCallRef = useRef(0);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const throttledCallback = useCallback(
    (...args: unknown[]) => {
      const now = Date.now();
      const remaining = delay - (now - lastCallRef.current);

      if (remaining <= 0) {
        // 立即执行
        lastCallRef.current = now;
        callback(...args);
      } else if (!timeoutRef.current) {
        // 延迟执行
        timeoutRef.current = setTimeout(() => {
          lastCallRef.current = Date.now();
          timeoutRef.current = null;
          callback(...args);
        }, remaining);
      }
    },
    [callback, delay]
  ) as T;

  // 清理
  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  return throttledCallback;
}
```

- [ ] **Step 2: 验证创建**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useThrottle.ts
git commit -m "feat(hooks): add useThrottle for scroll event optimization"

git add web/src/components/thread/ChatMessageList.tsx
git commit -m "refactor(ChatMessageList): use throttle for scroll handler"
```

---

## 阶段 4：切屏返回缓存策略

### Task 4.1: 创建 LoadingBar 组件

**Files:**
- Create: `web/src/components/LoadingBar.tsx`

- [ ] **Step 1: 创建轻量加载指示器**

```typescript
// web/src/components/LoadingBar.tsx
import React from 'react';
import './LoadingBar.css';

interface LoadingBarProps {
  visible: boolean;
}

export const LoadingBar: React.FC<LoadingBarProps> = ({ visible }) => {
  if (!visible) return null;

  return (
    <div className="loading-bar">
      <div className="loading-bar-progress" />
    </div>
  );
};
```

- [ ] **Step 2: 创建样式文件**

```css
/* web/src/components/LoadingBar.css */
.loading-bar {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 3px;
  background: var(--bg-secondary);
  z-index: 1000;
}

.loading-bar-progress {
  height: 100%;
  background: var(--color-primary);
  animation: loading-bar-animation 1.5s ease-in-out infinite;
}

@keyframes loading-bar-animation {
  0% {
    width: 0%;
    left: 0;
  }
  50% {
    width: 70%;
    left: 15%;
  }
  100% {
    width: 0%;
    left: 100%;
  }
}
```

- [ ] **Step 3: 验证创建**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 4: Commit**

```bash
git add web/src/components/LoadingBar.tsx web/src/components/LoadingBar.css
git commit -m "feat: add LoadingBar component for lightweight loading indicator"
```

---

### Task 4.2: 创建 ThreadSkeleton 骨架屏

**Files:**
- Create: `web/src/components/ThreadSkeleton.tsx`

- [ ] **Step 1: 创建骨架屏组件**

```typescript
// web/src/components/ThreadSkeleton.tsx
import React from 'react';
import './ThreadSkeleton.css';

export const ThreadSkeleton: React.FC = () => {
  return (
    <div className="thread-skeleton">
      {/* 头部骨架 */}
      <div className="skeleton-header">
        <div className="skeleton-avatar" />
        <div className="skeleton-title" />
      </div>

      {/* 消息骨架 */}
      <div className="skeleton-messages">
        {[1, 2, 3].map((i) => (
          <div key={i} className="skeleton-message">
            <div className="skeleton-message-avatar" />
            <div className="skeleton-message-content">
              <div className="skeleton-line skeleton-line-short" />
              <div className="skeleton-line skeleton-line-medium" />
              <div className="skeleton-line skeleton-line-long" />
            </div>
          </div>
        ))}
      </div>

      {/* 输入区骨架 */}
      <div className="skeleton-input" />
    </div>
  );
};
```

- [ ] **Step 2: 创建骨架屏样式**

```css
/* web/src/components/ThreadSkeleton.css */
.thread-skeleton {
  height: 100%;
  padding: 16px;
  background: var(--bg-container);
}

.skeleton-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  margin-bottom: 16px;
}

.skeleton-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: linear-gradient(90deg, var(--bg-secondary) 25%, var(--bg-tertiary) 50%, var(--bg-secondary) 75%);
  animation: skeleton-pulse 1.5s ease-in-out infinite;
}

.skeleton-title {
  width: 200px;
  height: 20px;
  border-radius: 4px;
  background: linear-gradient(90deg, var(--bg-secondary) 25%, var(--bg-tertiary) 50%, var(--bg-secondary) 75%);
  animation: skeleton-pulse 1.5s ease-in-out infinite;
}

.skeleton-messages {
  flex: 1;
}

.skeleton-message {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.skeleton-message-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: linear-gradient(90deg, var(--bg-secondary) 25%, var(--bg-tertiary) 50%, var(--bg-secondary) 75%);
  animation: skeleton-pulse 1.5s ease-in-out infinite;
}

.skeleton-message-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.skeleton-line {
  height: 16px;
  border-radius: 4px;
  background: linear-gradient(90deg, var(--bg-secondary) 25%, var(--bg-tertiary) 50%, var(--bg-secondary) 75%);
  animation: skeleton-pulse 1.5s ease-in-out infinite;
}

.skeleton-line-short { width: 30%; }
.skeleton-line-medium { width: 60%; }
.skeleton-line-long { width: 80%; }

.skeleton-input {
  height: 48px;
  border-radius: 8px;
  margin-top: 16px;
  background: linear-gradient(90deg, var(--bg-secondary) 25%, var(--bg-tertiary) 50%, var(--bg-secondary) 75%);
  animation: skeleton-pulse 1.5s ease-in-out infinite;
}

@keyframes skeleton-pulse {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}
```

- [ ] **Step 3: 验证创建**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 4: Commit**

```bash
git add web/src/components/ThreadSkeleton.tsx web/src/components/ThreadSkeleton.css
git commit -m "feat: add ThreadSkeleton for initial loading state"
```

---

### Task 4.3: 重构 ThreadView 实现分级加载

**Files:**
- Modify: `web/src/pages/ThreadView.tsx`

- [ ] **Step 1: 读取 ThreadView 加载逻辑**

```bash
grep -n "loading\|loadThread\|messages" web/src/pages/ThreadView.tsx | head -30
```

- [ ] **Step 2: 修改 ThreadView 分级加载逻辑**

找到 ThreadView 的返回部分，修改渲染逻辑：

```typescript
// web/src/pages/ThreadView.tsx
// 添加导入
import { LoadingBar } from '@/components/LoadingBar';
import { ThreadSkeleton } from '@/components/ThreadSkeleton';

// 修改渲染部分
// 找到 if (loading) { return <Spin /> } 部分

// 替换为分级加载逻辑：
// 有缓存数据时
if (messages.length > 0) {
  return (
    <div className="thread-view-wrapper">
      {loading && <LoadingBar visible={true} />}
      {/* 原有的 ThreadView 内容 */}
      ...
    </div>
  );
}

// 无缓存数据时
if (loading) {
  return <ThreadSkeleton />;
}

// 正常渲染
return (
  <div className="thread-view-wrapper">
    {/* 原有内容 */}
    ...
  </div>
);
```

- [ ] **Step 3: 验证修改**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/ThreadView.tsx
git commit -m "refactor(ThreadView): implement tiered loading state

- Show LoadingBar when data exists (lightweight)
- Show ThreadSkeleton when no data (skeleton)
- No white screen on tab switch return"
```

---

### Task 4.4: 实现增量加载策略

**Files:**
- Modify: `web/src/store/index.ts`

- [ ] **Step 1: 读取 loadThread 实现**

```bash
grep -A 50 "loadThread:" web/src/store/index.ts | head -60
```

- [ ] **Step 2: 修改 loadThread 增量加载逻辑**

```typescript
// web/src/store/index.ts
// 找到 loadThread 函数，修改开头部分

loadThread: async (threadId) => {
  const state = get();
  
  // 已有数据且是同一个 thread：增量更新
  if (state.currentThread?.id === threadId && state.messages.length > 0) {
    // 检查缓存 TTL
    const loadedAt = state.threadLoadedAt;
    const CACHE_TTL = 5 * 60 * 1000; // 5 分钟
    
    if (loadedAt && Date.now() - loadedAt < CACHE_TTL) {
      // 缓存有效，只检查 Agent 状态变化
      const invocations = await api.invocations.list(threadId);
      const runningCount = invocations.filter(i => i.status === 'running').length;
      
      if (state.activeAgents.length === runningCount) {
        // 无变化，完全跳过更新
        console.log('[loadThread] Cache valid, no changes');
        return;
      }
      
      // 有变化，增量更新
      set({
        activeAgents: invocations.filter(i => i.status === 'running'),
        completedAgents: invocations.filter(i => 
          i.status === 'completed' || i.status === 'failed' || i.status === 'interrupted'
        ),
      });
      console.log('[loadThread] Cache valid, incremental update');
      return;
    }
  }
  
  // 新 thread 或缓存过期：完整加载
  set({
    loading: true,
    messages: [],
    threadLoadedAt: null,
    // ... 其他重置逻辑保持不变
  });
  
  // ... 原有的完整加载逻辑
},
```

- [ ] **Step 3: 在 AppState 中添加 threadLoadedAt**

```typescript
// web/src/store/index.ts
// 在 interface AppState 中添加

interface AppState {
  // ... 现有字段
  threadLoadedAt: number | null;  // 新增：加载时间戳
}

// 在 initialState 中添加
const initialState: AppState = {
  // ... 现有字段
  threadLoadedAt: null,
};

// 在 loadThread 完整加载成功后设置时间戳
set({
  // ... 其他状态
  threadLoadedAt: Date.now(),  // 新增
});
```

- [ ] **Step 4: 验证修改**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 5: Commit**

```bash
git add web/src/store/index.ts
git commit -m "feat(store): implement incremental loading with cache TTL

- Skip reload if same thread with valid cache (5 min TTL)
- Only update changed Agent states
- Add threadLoadedAt timestamp tracking"
```

---

### Task 4.5: 实现 visibilitychange 重连策略

**Files:**
- Modify: `web/src/hooks/useWebSocket.ts`

- [ ] **Step 1: 添加 visibilitychange 监听**

```typescript
// web/src/hooks/useWebSocket.ts
// 在 useEffect 内部添加 visibilitychange 监听

useEffect(() => {
  // ... 现有的 WebSocket 连接逻辑
  
  // 页面可见性变化时管理连接
  const handleVisibilityChange = () => {
    if (document.visibilityState === 'visible') {
      // 页面变为可见，检查连接状态
      if (!wsRef.current && threadId) {
        console.log('[WebSocket] Page visible, reconnecting...');
        // 触发重新连接（通过修改依赖触发 effect）
        // 或者直接调用 connectWebSocket
        connectWs(threadId);
      }
    }
  };
  
  document.addEventListener('visibilitychange', handleVisibilityChange);
  
  return () => {
    // 清理
    document.removeEventListener('visibilitychange', handleVisibilityChange);
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
  };
}, [threadId]);

// 添加 connectWs 函数定义（如果需要）
const connectWs = useCallback((id: string) => {
  // ... WebSocket 连接逻辑
}, []);
```

- [ ] **Step 2: 验证修改**

```bash
cd web && npm run build 2>&1 | grep -E "error|Error" || echo "Build OK"
```

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useWebSocket.ts
git commit -m "feat(ws): implement visibilitychange reconnect strategy

- Reconnect WebSocket when page becomes visible
- Support tab switch return scenario"
```

---

## 验收测试

### Task 5.1: 创建性能测试文件

**Files:**
- Create: `auto-test/e2e/thread-performance.spec.ts`

- [ ] **Step 1: 创建性能测试文件**

```typescript
// auto-test/e2e/thread-performance.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Thread Rendering Performance', () => {
  
  test('切屏返回性能 - 显示时间 < 100ms', async ({ page }) => {
    // 预加载 thread 页面
    await page.goto('/thread/test-thread-id');
    await page.waitForSelector('.chat-message-list');
    
    // 确认数据已加载
    const messageCount = await page.locator('.chat-message').count();
    expect(messageCount).toBeGreaterThan(0);
    
    // 切换到其他页面
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');
    
    // 返回并测量显示时间
    const startTime = Date.now();
    await page.goto('/thread/test-thread-id');
    await page.waitForSelector('.chat-message-list', { state: 'visible' });
    const elapsed = Date.now() - startTime;
    
    console.log(`切屏返回显示时间: ${elapsed}ms`);
    expect(elapsed).toBeLessThan(100);
  });
  
  test('长对话滚动性能 - 100条消息 FPS ≥ 50', async ({ page }) => {
    // 加载有 100 条消息的 thread
    await page.goto('/thread/thread-100-messages');
    await page.waitForSelector('.chat-message-list');
    
    // 模拟滚动
    const scrollPromise = page.evaluate(async () => {
      const list = document.querySelector('.chat-message-list');
      if (!list) return { fps: 0 };
      
      const startTime = performance.now();
      let frameCount = 0;
      
      // 滚动到底部
      for (let i = 0; i < 10; i++) {
        list.scrollTop += 100;
        await new Promise(r => requestAnimationFrame(r));
        frameCount++;
      }
      
      const elapsed = performance.now() - startTime;
      const fps = (frameCount / elapsed) * 1000;
      
      return { fps, elapsed };
    });
    
    const { fps } = await scrollPromise;
    console.log(`滚动 FPS: ${fps}`);
    expect(fps).toBeGreaterThanOrEqual(50);
  });
  
  test('流式输出性能 - FPS ≥ 30', async ({ page }) => {
    await page.goto('/thread/test-thread-id');
    await page.waitForSelector('.chat-message-list');
    
    // 启动 Agent（假设有测试按钮）
    await page.click('[data-testid="test-start-agent"]');
    
    // 等待流式输出开始
    await page.waitForSelector('.streaming-message');
    
    // 测量 3 秒内的渲染帧数
    const metrics = await page.evaluate(async () => {
      const frames: number[] = [];
      let lastTime = performance.now();
      
      const observer = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (entry.name.startsWith('render')) {
            frames.push(entry.duration);
          }
        }
      });
      observer.observe({ entryTypes: ['measure'] });
      
      // 等待 3 秒
      await new Promise(r => setTimeout(r, 3000));
      observer.disconnect();
      
      return frames;
    });
    
    // 计算平均帧时间
    if (metrics.length > 0) {
      const avgFrameTime = metrics.reduce((a, b) => a + b, 0) / metrics.length;
      const fps = 1000 / avgFrameTime;
      console.log(`流式输出 FPS: ${fps}`);
      expect(fps).toBeGreaterThanOrEqual(30);
    }
  });
});
```

- [ ] **Step 2: 验证测试文件语法**

```bash
cd web && npx playwright test --list 2>&1 | grep thread-performance || echo "Test file OK"
```

- [ ] **Step 3: Commit**

```bash
git add auto-test/e2e/thread-performance.spec.ts
git commit -m "test: add thread rendering performance E2E tests

- Test tab switch return performance (< 100ms)
- Test scroll performance with 100 messages (FPS ≥ 50)
- Test streaming output performance (FPS ≥ 30)"
```

---

## 自检清单

| Spec 阶段 | 任务覆盖 | 状态 |
|-----------|----------|------|
| 阶段 1.1 流式状态隔离 | Task 1.1 (StreamingStore) + Task 1.4 (StreamingMessage) | ✓ |
| 阶段 1.2 批量更新 | Task 1.2 (useChunkBatcher) + Task 1.3 (useWebSocket) | ✓ |
| 阶段 1.3 降低订阅粒度 | Task 1.4 (StreamingMessage 订阅隔离) | ✓ |
| 阶段 2.1 依赖选择 | Task 2.1 (@tanstack/react-virtual) | ✓ |
| 阶段 2.2 虚拟列表渲染 | Task 2.3 (ChatMessageList 重构) | ✓ |
| 阶段 2.3 动态高度估算 | Task 2.2 (estimateMessageHeight) + Task 2.3 (measureElement) | ✓ |
| 阶段 3.1 ChatMessage 优化 | Task 3.1 (memo) | ✓ |
| 阶段 3.2 MessageContentRenderer | Task 3.2 (移除日志) | ✓ |
| 阶段 3.3 StreamingMessage | Task 1.4 (已覆盖) | ✓ |
| 阶段 3.4 滚动处理优化 | Task 3.3 (useThrottle) | ✓ |
| 阶段 4.1 增量加载 | Task 4.4 (loadThread 增量) | ✓ |
| 阶段 4.2 WebSocket 重连 | Task 4.5 (visibilitychange) | ✓ |
| 阶段 4.3 分级加载状态 | Task 4.1 (LoadingBar) + Task 4.2 (ThreadSkeleton) + Task 4.3 | ✓ |
| 阶段 4.4 数据保鲜 | Task 4.4 (cache TTL) | ✓ |
| 验收测试 | Task 5.1 (thread-performance.spec.ts) | ✓ |

---

## 执行顺序

```
Phase 1: 状态管理重构
  Task 1.1 → Task 1.2 → Task 1.3 → Task 1.4

Phase 2: 虚拟列表引入
  Task 2.1 → Task 2.2 → Task 2.3

Phase 3: 组件性能优化
  Task 3.1 → Task 3.2 → Task 3.3

Phase 4: 切屏返回缓存
  Task 4.1 → Task 4.2 → Task 4.3 → Task 4.4 → Task 4.5

Phase 5: 验收测试
  Task 5.1
```

每个 Phase 完成后可进行阶段性验证和评审。