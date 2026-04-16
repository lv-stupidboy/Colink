# 对话滚动优化与角色指示器设计

**日期**: 2026-04-15
**状态**: 已确认
**作者**: Claude

---

## 需求概述

1. **自动滚动优化**: 用户上拉查看历史时，停止自动滚动到底部；接近底部时自动恢复
2. **角色指示器**: 在滚动条轨道显示角色头像指示器，点击可快速跳转到对应消息

---

## 整体架构

### 技术方案

使用 **IntersectionObserver** 监听底部锚点判断是否接近底部，配合 CSS 绝对定位的角色指示器层实现快速定位功能。

### 文件结构

```
web/src/components/thread/
├── ChatMessageList.tsx      # 修改：使用 hook 控制自动滚动
├── ChatMessage.tsx          # 修改：添加 data-* 属性
├── MessageScrollIndicator.tsx # 新增：角色指示器组件
├── useAutoScrollControl.ts   # 新增：自动滚动控制 hook
└── ThreadView.css           # 修改：添加指示器样式
```

---

## 第一部分：自动滚动控制

### 核心机制

使用 IntersectionObserver 监听底部锚点元素：

```typescript
// useAutoScrollControl.ts
const useAutoScrollControl = (containerRef: RefObject<HTMLElement>) => {
  const [isNearBottom, setIsNearBottom] = useState(true);
  const bottomAnchorRef = useRef<HTMLDivElement>(null);
  
  useEffect(() => {
    if (!bottomAnchorRef.current) return;
    
    const observer = new IntersectionObserver(
      ([entry]) => {
        setIsNearBottom(entry.isIntersecting);
      },
      { threshold: 0.1, root: containerRef.current }
    );
    
    observer.observe(bottomAnchorRef.current);
    return () => observer.disconnect();
  }, [containerRef]);
  
  return { isNearBottom, bottomAnchorRef };
};
```

### 滚动行为改变

**现有逻辑（需修改）：**
- `ThreadView.tsx:533-534`: `useEffect(() => { scrollToBottom(); }, [messages])` → 删除
- `ChatMessageList.tsx:79-84`: 硬性自动滚动 → 改为条件判断

**新逻辑：**
- 新消息到达时，检查 `isNearBottom`
- 只有 `isNearBottom === true` 时才执行滚动
- 用户上拉导致 `isNearBottom === false`，新消息不触发滚动
- 用户下拉接近底部，`isNearBottom` 恢复 true，后续新消息自动滚动

---

## 第二部分：角色指示器

### 指示器位置计算

在消息列表容器右侧添加独立的指示器层（绝对定位）：

```typescript
interface IndicatorPosition {
  messageId: string;
  role: 'user' | 'agent' | 'system';
  agentId?: string;
  agentName?: string;
  y: number; // 指示器 Y 坐标
}

const updateIndicatorPositions = () => {
  const container = containerRef.current;
  if (!container) return;
  
  const messageElements = container.querySelectorAll('[data-message-id]');
  const scrollHeight = container.scrollHeight;
  const containerHeight = container.clientHeight;
  
  const positions: IndicatorPosition[] = [];
  messageElements.forEach((el) => {
    const messageId = el.getAttribute('data-message-id');
    const role = el.getAttribute('data-message-role');
    const agentId = el.getAttribute('data-agent-id');
    const agentName = el.getAttribute('data-agent-name');
    
    const ratio = (el as HTMLElement).offsetTop / scrollHeight;
    positions.push({
      messageId,
      role,
      agentId,
      agentName,
      y: ratio * containerHeight,
    });
  });
  
  return positions;
};
```

### 指示器渲染

**显示内容：**
- 每条消息都显示一个指示器（用户选择"每次发言都显示")
- 使用角色头像作为指示器图标
- 系统角色用皇冠图标，用户角色用用户图标

**样式设计：**
```css
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
  width: 16px;
  height: 16px;
  border-radius: 50%;
  cursor: pointer;
  transition: transform 0.2s;
}

.indicator-item:hover {
  transform: scale(1.3);
}
```

### 跳转交互

点击指示器：
- 获取对应的 `messageId`
- 查找 DOM 元素 `document.querySelector('[data-message-id="${messageId}"]')`
- 使用 `scrollIntoView({ block: 'center' })` 让消息居中显示

### 数据属性添加

给 ChatMessage 组件添加 data 属性：

```tsx
<div 
  data-message-id={message.id}
  data-message-role={message.role}
  data-agent-id={message.agentId}
  data-agent-name={message.agentName}
>
```

---

## 第三部分：整合方案

### ChatMessageList.tsx 修改

```typescript
// 使用 hook 控制
const { isNearBottom, bottomAnchorRef } = useAutoScrollControl(listRef);

// 条件滚动
useEffect(() => {
  if (autoScroll && isNearBottom && bottomAnchorRef.current) {
    bottomAnchorRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }
}, [messages.length, autoScroll, isNearBottom]);
```

### ThreadView.tsx 修改

**删除硬性滚动：**
```typescript
// 删除第533-534行
useEffect(() => {
  scrollToBottom();
}, [messages]);
```

**添加指示器：**
```tsx
<div className="thread-messages">
  <ChatMessageList ... />
  <MessageScrollIndicator
    messages={messages}
    agentConfigs={mentionableAgents}
    containerRef={listRef}
    onJumpToMessage={scrollToMessage}
  />
  <div ref={messagesEndRef} />
</div>
```

### CSS 样式调整

```css
.thread-messages {
  flex: 1;
  overflow-y: auto;
  padding-right: 32px;  /* 为指示器留出空间 */
}
```

---

## 实现步骤

**第一阶段：自动滚动控制**
1. 创建 `useAutoScrollControl.ts` hook
2. 修改 `ChatMessageList.tsx`，移除硬性自动滚动
3. 修改 `ThreadView.tsx`，删除强制滚动逻辑
4. 验证自动滚动控制正常

**第二阶段：角色指示器**
1. 给 `ChatMessage.tsx` 添加 data 属性
2. 创建 `MessageScrollIndicator.tsx` 组件
3. 在 `ThreadView.tsx` 添加指示器层
4. 实现位置计算和点击跳转

**第三阶段：样式优化**
1. 添加指示器样式和 hover 效果
2. 调整消息区域 padding
3. 添加深色模式适配

---

## 测试要点

**自动滚动测试：**
- 用户在底部时，新消息自动滚动
- 用户上拉时，新消息不触发滚动
- 用户下拉接近底部时，恢复自动滚动

**角色指示器测试：**
- 每条消息都有对应指示器
- 指示器位置与消息位置对应
- 点击跳转正确
- hover 显示 tooltip

**边缘情况：**
- 消息列表为空时正常
- 滚动到顶部/底部时指示器正确
- 消息内容过长时指示器不重叠

---

## 不实现的功能（YAGNI）

- ❌ 手动切换自动滚动的按钮
- ❌ 指示器拖拽跳转
- ❌ 指示器进度条样式
- ❌ 虚拟滚动优化