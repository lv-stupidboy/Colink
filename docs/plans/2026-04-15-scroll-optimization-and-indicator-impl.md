# Scroll Optimization and Role Indicator Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现对话自动滚动优化（用户上拉时停止滚动）和角色指示器（滚动条轨道显示角色头像，点击跳转）

**Architecture:** 使用 IntersectionObserver 监听底部锚点判断是否接近底部，CSS 绝对定位指示器层，通过 data-* 属性获取消息位置

**Tech Stack:** React, TypeScript, IntersectionObserver API, CSS Absolute Positioning

---

## Phase 1: 自动滚动控制

### Task 1: 创建 useAutoScrollControl Hook

**Files:**
- Create: `web/src/components/thread/useAutoScrollControl.ts`

**Step 1: 创建 hook 文件**

```typescript
// web/src/components/thread/useAutoScrollControl.ts
import { useState, useEffect, useRef, useCallback, RefObject } from 'react';

interface AutoScrollControlResult {
  isNearBottom: boolean;
  bottomAnchorRef: RefObject<HTMLDivElement>;
  scrollToBottom: () => void;
}

/**
 * 自动滚动控制 Hook
 * 使用 IntersectionObserver 监听底部锚点，判断用户是否接近底部
 * 
 * @param containerRef - 消息列表容器 ref
 * @param threshold - 底部阈值（px），在此范围内视为接近底部
 */
export const useAutoScrollControl = (
  containerRef: RefObject<HTMLElement>,
  threshold: number = 100
): AutoScrollControlResult => {
  const [isNearBottom, setIsNearBottom] = useState(true);
  const bottomAnchorRef = useRef<HTMLDivElement>(null);
  const isObservingRef = useRef(false);

  // IntersectionObserver 监听底部锚点
  useEffect(() => {
    if (!bottomAnchorRef.current || !containerRef.current) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        // 当底部锚点进入视口时，认为接近底部
        setIsNearBottom(entry.isIntersecting);
      },
      {
        root: containerRef.current,
        threshold: 0.1,
        rootMargin: `0px 0px ${threshold}px 0px`, // 扩大底部检测范围
      }
    );

    observer.observe(bottomAnchorRef.current);
    isObservingRef.current = true;

    return () => {
      observer.disconnect();
      isObservingRef.current = false;
    };
  }, [containerRef, threshold]);

  // 手动滚动到底部
  const scrollToBottom = useCallback(() => {
    if (bottomAnchorRef.current) {
      bottomAnchorRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
      setIsNearBottom(true);
    }
  }, []);

  return {
    isNearBottom,
    bottomAnchorRef,
    scrollToBottom,
  };
};

export default useAutoScrollControl;
```

**Step 2: 提交**

```bash
git add web/src/components/thread/useAutoScrollControl.ts
git commit -m "feat(thread): add useAutoScrollControl hook with IntersectionObserver"
```

---

### Task 2: 修改 ChatMessageList 使用 Hook

**Files:**
- Modify: `web/src/components/thread/ChatMessageList.tsx:79-84`

**Step 1: 导入 hook**

在文件顶部添加导入：

```typescript
import { useAutoScrollControl } from './useAutoScrollControl';
```

**Step 2: 修改组件使用 hook**

修改 ChatMessageList 组件：

```typescript
// 替换原有的 listRef 和 bottomRef
export const ChatMessageList: React.FC<ChatMessageListProps> = memo(({
  messages,
  agentConfigs,
  projectPath,
  toolEvents = {},
  onStopAgent,
  onRetryAgent,
  onOpenCodePanel,
  loading = false,
  autoScroll = true,
  onQuestionSubmit,
}) => {
  const listRef = useRef<HTMLDivElement>(null);
  
  // 使用自动滚动控制 hook
  const { isNearBottom, bottomAnchorRef, scrollToBottom } = useAutoScrollControl(listRef);

  // 条件自动滚动：只有接近底部时才滚动
  useEffect(() => {
    if (autoScroll && isNearBottom && bottomAnchorRef.current) {
      bottomAnchorRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
    }
  }, [messages.length, autoScroll, isNearBottom]);

  // ... 其余代码保持不变，只需将底部锚点改为使用 bottomAnchorRef
```

**Step 3: 替换底部锚点渲染**

找到原有的 `<div ref={bottomRef} ... />`，替换为：

```typescript
{/* 底部锚点 - 用于 IntersectionObserver */}
<div ref={bottomAnchorRef} style={{ height: '1px' }} />
```

**Step 4: 提交**

```bash
git add web/src/components/thread/ChatMessageList.tsx
git commit -m "feat(thread): integrate useAutoScrollControl in ChatMessageList"
```

---

### Task 3: 删除 ThreadView 硬性滚动

**Files:**
- Modify: `web/src/pages/ThreadView.tsx:532-534`

**Step 1: 删除强制滚动 useEffect**

找到并删除以下代码（约第532-534行）：

```typescript
// 删除这段代码
useEffect(() => {
  scrollToBottom();
}, [messages]);
```

**Step 2: 删除未使用的 scrollToBottom 函数（如果不再需要）**

如果 `scrollToBottom` 函数只在上述 useEffect 中使用，可以删除：

```typescript
// 删除（约第1010行）
const scrollToBottom = () => {
  messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
};
```

**Step 3: 提交**

```bash
git add web/src/pages/ThreadView.tsx
git commit -m "feat(thread): remove hard auto-scroll from ThreadView"
```

---

## Phase 2: 角色指示器

### Task 4: 给 ChatMessage 添加 data 属性

**Files:**
- Modify: `web/src/components/thread/ChatMessage.tsx`

**Step 1: 找到消息根元素**

先读取 ChatMessage.tsx 文件，找到消息渲染的根 div 元素。

**Step 2: 添加 data 属性**

在消息根元素上添加 data 属性：

```typescript
// 在消息容器 div 上添加
<div
  data-message-id={message.id}
  data-message-role={message.role}
  data-agent-id={message.agentId || ''}
  data-agent-name={message.agentName || ''}
  className="chat-message"
  // ... 其他属性
>
```

**Step 3: 提交**

```bash
git add web/src/components/thread/ChatMessage.tsx
git commit -m "feat(thread): add data attributes to ChatMessage for indicator positioning"
```

---

### Task 5: 创建 MessageScrollIndicator 组件

**Files:**
- Create: `web/src/components/thread/MessageScrollIndicator.tsx`

**Step 1: 创建指示器组件**

```typescript
// web/src/components/thread/MessageScrollIndicator.tsx
import React, { useEffect, useState, useCallback, RefObject } from 'react';
import { Avatar, Tooltip } from 'antd';
import { UserOutlined, CrownOutlined, RobotOutlined } from '@ant-design/icons';
import type { Message, AgentConfig } from '@/types';

interface IndicatorItem {
  messageId: string;
  role: 'user' | 'agent' | 'system';
  agentId?: string;
  agentName?: string;
  y: number;
}

interface MessageScrollIndicatorProps {
  messages: Message[];
  agentConfigs: AgentConfig[];
  containerRef: RefObject<HTMLDivElement>;
  onJumpToMessage?: (messageId: string) => void;
}

/**
 * 消息滚动指示器组件
 * 在滚动条轨道右侧显示角色头像，点击可跳转到对应消息
 */
const MessageScrollIndicator: React.FC<MessageScrollIndicatorProps> = ({
  messages,
  agentConfigs,
  containerRef,
  onJumpToMessage,
}) => {
  const [indicators, setIndicators] = useState<IndicatorItem[]>([]);

  // 计算指示器位置
  const updateIndicators = useCallback(() => {
    if (!containerRef.current) return;

    const container = containerRef.current;
    const containerHeight = container.clientHeight;
    const scrollHeight = container.scrollHeight;

    // 获取所有消息元素
    const messageElements = container.querySelectorAll('[data-message-id]');

    const newIndicators: IndicatorItem[] = [];

    messageElements.forEach((el) => {
      const messageId = el.getAttribute('data-message-id');
      const role = el.getAttribute('data-message-role') as 'user' | 'agent' | 'system';
      const agentId = el.getAttribute('data-agent-id');
      const agentName = el.getAttribute('data-agent-name');

      if (!messageId || !role) return;

      // 计算位置比例
      const element = el as HTMLElement;
      const ratio = element.offsetTop / scrollHeight;
      const y = ratio * containerHeight;

      newIndicators.push({
        messageId,
        role,
        agentId: agentId || undefined,
        agentName: agentName || undefined,
        y,
      });
    });

    setIndicators(newIndicators);
  }, [containerRef]);

  // 监听容器变化更新指示器
  useEffect(() => {
    updateIndicators();

    // 监听滚动事件更新位置
    const container = containerRef.current;
    if (!container) return;

    const handleScroll = () => {
      // 使用 requestAnimationFrame 节流
      requestAnimationFrame(updateIndicators);
    };

    container.addEventListener('scroll', handleScroll, { passive: true });
    return () => container.removeEventListener('scroll', handleScroll);
  }, [containerRef, messages, updateIndicators]);

  // 消息变化时更新
  useEffect(() => {
    // 延迟更新，等待 DOM 渲染完成
    const timer = setTimeout(updateIndicators, 100);
    return () => clearTimeout(timer);
  }, [messages, updateIndicators]);

  // 跳转到消息
  const handleJump = useCallback((messageId: string) => {
    if (onJumpToMessage) {
      onJumpToMessage(messageId);
    } else {
      // 默认跳转逻辑
      const element = document.querySelector(`[data-message-id="${messageId}"]`);
      if (element) {
        element.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    }
  }, [onJumpToMessage]);

  // 获取角色配置
  const getAgentConfig = useCallback((agentId?: string, agentName?: string) => {
    if (agentId) {
      return agentConfigs.find((c) => c.id === agentId);
    }
    if (agentName) {
      return agentConfigs.find((c) => c.name === agentName);
    }
    return undefined;
  }, [agentConfigs]);

  // 渲染指示器图标
  const renderIndicatorIcon = (indicator: IndicatorItem) => {
    const agentConfig = getAgentConfig(indicator.agentId, indicator.agentName);

    if (indicator.role === 'user') {
      return <UserOutlined style={{ color: 'var(--text-primary)' }} />;
    }

    if (indicator.role === 'system') {
      return <CrownOutlined style={{ color: '#faad14' }} />;
    }

    // Agent 角色
    if (agentConfig?.isSystem) {
      return <CrownOutlined style={{ color: '#faad14' }} />;
    }

    return <RobotOutlined style={{ color: 'var(--color-primary)' }} />;
  };

  // 获取显示名称
  const getDisplayName = (indicator: IndicatorItem) => {
    if (indicator.role === 'user') return '用户';
    if (indicator.role === 'system') return '系统';
    return indicator.agentName || 'Agent';
  };

  // 消息为空时不渲染
  if (messages.length === 0) {
    return null;
  }

  return (
    <div className="message-scroll-indicators">
      {indicators.map((indicator) => (
        <Tooltip
          key={indicator.messageId}
          title={getDisplayName(indicator)}
          placement="left"
        >
          <div
            className="indicator-item"
            style={{ top: indicator.y }}
            onClick={() => handleJump(indicator.messageId)}
          >
            <Avatar size={16} icon={renderIndicatorIcon(indicator)} />
          </div>
        </Tooltip>
      ))}
    </div>
  );
};

export default MessageScrollIndicator;
```

**Step 2: 提交**

```bash
git add web/src/components/thread/MessageScrollIndicator.tsx
git commit -m "feat(thread): add MessageScrollIndicator component with role indicators"
```

---

### Task 6: 在 ThreadView 添加指示器

**Files:**
- Modify: `web/src/pages/ThreadView.tsx`
- Modify: `web/src/pages/ThreadView.css`

**Step 1: 导入指示器组件**

```typescript
import MessageScrollIndicator from '@/components/thread/MessageScrollIndicator';
```

**Step 2: 在消息区域添加指示器**

找到 `.thread-messages` 区域（约第1762-1787行 Solo模式和第1879-1899行普通模式），添加指示器：

```typescript
// Solo 模式下（约第1762-1787行）
<div className="thread-messages">
  {messages.length === 0 ? (
    // 空状态渲染...
  ) : (
    <ChatMessageList
      messages={messages}
      // ... 其他 props
    />
  )}
  {/* 添加角色指示器 */}
  <MessageScrollIndicator
    messages={messages}
    agentConfigs={mentionableAgents}
    containerRef={/* 需要传递 ChatMessageList 的 listRef */}
  />
  <div ref={messagesEndRef} />
</div>

// 正常模式（约第1879-1899行）同样添加
```

**Step 3: 需要从 ChatMessageList 暴露 listRef**

修改 ChatMessageList，使用 forwardRef 暴露 listRef：

```typescript
// ChatMessageList.tsx
import { forwardRef } from 'react';

export const ChatMessageList = forwardRef<HTMLDivElement, ChatMessageListProps>(
  ({ messages, ... }, ref) => {
    // 使用传入的 ref 或内部 ref
    const internalRef = useRef<HTMLDivElement>(null);
    const listRef = ref || internalRef;

    // ... 其余代码
  }
);
```

**Step 4: 添加 CSS 样式**

在 ThreadView.css 中添加：

```css
/* 消息滚动指示器 */
.message-scroll-indicators {
  position: absolute;
  right: 8px;
  top: 0;
  width: 24px;
  height: 100%;
  pointer-events: auto;
  z-index: 10;
}

.indicator-item {
  position: absolute;
  right: 4px;
  width: 16px;
  height: 16px;
  cursor: pointer;
  transition: transform 0.2s ease;
}

.indicator-item:hover {
  transform: scale(1.3);
}

/* 调整消息区域为指示器留出空间 */
.thread-messages {
  padding-right: 32px;
}

/* 深色模式适配 */
[data-theme='dark'] .indicator-item {
  background: var(--bg-container);
}
```

**Step 5: 提交**

```bash
git add web/src/pages/ThreadView.tsx web/src/pages/ThreadView.css
git commit -m "feat(thread): integrate MessageScrollIndicator in ThreadView"
```

---

### Task 7: ChatMessageList 使用 forwardRef

**Files:**
- Modify: `web/src/components/thread/ChatMessageList.tsx`

**Step 1: 改用 forwardRef**

```typescript
import { forwardRef, useRef, memo, useEffect } from 'react';

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
      loading = false,
      autoScroll = true,
      onQuestionSubmit,
    } = props;

    // 使用传入的 ref，或内部创建
    const internalRef = useRef<HTMLDivElement>(null);
    const listRef = (ref as RefObject<HTMLDivElement>) || internalRef;

    // 使用 hook
    const { isNearBottom, bottomAnchorRef } = useAutoScrollControl(listRef);

    // ... 其余逻辑不变，渲染时使用 listRef
```

**Step 2: 更新 displayName**

```typescript
ChatMessageList.displayName = 'ChatMessageList';
```

**Step 3: 提交**

```bash
git add web/src/components/thread/ChatMessageList.tsx
git commit -m "feat(thread): use forwardRef in ChatMessageList to expose container ref"
```

---

## Phase 3: 测试与优化

### Task 8: 前端编译验证

**Step 1: 运行 TypeScript 检查**

```bash
cd web && npx tsc --noEmit
```

Expected: 无错误

**Step 2: 运行构建**

```bash
npm run build
```

Expected: 构建成功

**Step 3: 如有错误，修复并提交**

```bash
git add <修改的文件>
git commit -m "fix(thread): resolve TypeScript/build errors"
```

---

### Task 9: 功能手动测试

**Step 1: 启动前后端**

```bash
# 后端
go run ./cmd/server

# 前端
cd web && npm run dev
```

**Step 2: 测试自动滚动控制**

测试点：
- 发送消息，验证自动滚动到底部
- 上拉查看历史消息，发送新消息，验证不自动滚动
- 下拉接近底部，发送新消息，验证恢复自动滚动

**Step 3: 测试角色指示器**

测试点：
- 验证指示器在滚动条右侧显示
- 验证每条消息都有对应指示器
- 点击指示器，验证跳转到正确消息
- hover 指示器，验证显示角色名称 tooltip

**Step 4: 如有问题，修复并提交**

---

### Task 10: 最终提交与推送

**Step 1: 检查所有更改**

```bash
git status
git log --oneline -10
```

**Step 2: 推送到远程**

```bash
git push origin cc
```

---

## 完成标志

- ✅ 用户上拉时自动滚动停止
- ✅ 用户接近底部时自动滚动恢复
- ✅ 滚动条右侧显示角色指示器
- ✅ 点击指示器可跳转到对应消息
- ✅ hover 显示角色名称 tooltip
- ✅ TypeScript 编译无错误
- ✅ 深色模式样式正确
- ✅ 所有更改已推送