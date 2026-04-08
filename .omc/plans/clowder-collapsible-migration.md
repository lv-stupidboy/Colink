# Implementation Plan: clowder-ai 收缩/展开机制移植

## RALPLAN-DR Summary

### Principles (3-5)
1. **逻辑复用优先**：移植 clowder-ai 的核心逻辑，仅适配 UI 层
2. **渐进式集成**：先通用组件，再专用组件，最后集成到 ChatMessageList
3. **智能状态管理**：保留 userInteracted 追踪 + 自动状态切换机制
4. **事件通知机制**：保留 `isdp:chat-layout-changed` 事件用于滚动计算
5. **配置化默认值**：支持全局 + 线程级别的展开/收起默认值

### Decision Drivers (Top 3)
1. **技术栈差异**：clowder-ai 用 Tailwind，ISDP 用 Ant Design
2. **流式场景需求**：工具调用输出需要在流式时自动展开，完成后自动收起
3. **代码复用性**：需要一个通用基础组件避免重复逻辑

### Viable Options (>=2)

| Option | Pros | Cons |
|--------|------|------|
| **A. 直接移植 Tailwind 样式** | 100% 还原视觉效果 | 引入 Tailwind 依赖，与 Ant Design 冲突 |
| **B. Ant Design Collapse 封装** | 与现有 UI 一致，无新依赖 | 需要覆盖默认样式实现动态着色 |
| **C. 自定义 CSS + React 状态** | 完全控制，无依赖 | 需要重新实现动画和可访问性 |
| **D. 混合方案 (推荐)** | 获得框架 ARIA 支持 + 灵活定制 | 需要禁用部分内置功能 |

**推荐：Option D — 混合方案**：
- 使用 Ant Design `Collapse.Panel` 作为基础容器（获得 ARIA 和键盘支持）
- 使用 **CSS 变量** 传递 `accentColor`，避免 `!important` 样式覆盖
- 完全自定义状态管理逻辑，不依赖 Collapse 的 activeKey 受控模式

---

## Implementation Steps

### Step 1: 创建通用 CollapsiblePanel 基础组件
**文件**: `isdp/web/src/components/CollapsiblePanel.tsx`

**核心功能**:
- 可控/不可控模式切换
- 导出模式检测与自动展开
- 布局变化事件通知
- **CSS 变量传递主题色**（避免 Ant Design 样式覆盖）
- **React.memo 包裹** 避免高频消息流中的不必要重渲染

**混合方案实现**:
```tsx
<Collapse
  activeKey={expanded ? ['1'] : []}
  onChange={() => {}} // 禁用内置状态切换，使用自定义逻辑
  className="collapsible-panel"
  style={{ '--accent-color': accentColor } as React.CSSProperties}
>
  <Collapse.Panel
    header={header}
    key="1"
    showArrow={false} // 自定义箭头实现动画控制
  >
    {children}
  </Collapse.Panel>
</Collapse>
```

**接口设计**:
```tsx
interface CollapsiblePanelProps {
  header: React.ReactNode;
  children: React.ReactNode;
  defaultExpanded?: boolean;
  expanded?: boolean;
  onToggle?: (expanded: boolean) => void;
  accentColor?: string;
  expandInExport?: boolean;
  className?: string;
}
```

### Step 2: 创建 useCollapsibleState Hook
**文件**: `isdp/web/src/hooks/useCollapsibleState.ts`

**核心功能**:
- 封装智能状态逻辑（userInteracted 追踪）
- 流式完成后自动收起（非用户操作时）
- 导出模式自动展开
- 布局变化事件通知

**接口设计**:
```tsx
interface UseCollapsibleStateOptions {
  defaultExpanded?: boolean;
  forceExpanded?: boolean;  // 流式状态时传入
  expandInExport?: boolean;
  onToggle?: (expanded: boolean) => void;
}

interface UseCollapsibleStateReturn {
  expanded: boolean;
  toggle: () => void;
  userInteracted: React.MutableRefObject<boolean>;
}

function useCollapsibleState(options: UseCollapsibleStateOptions): UseCollapsibleStateReturn;
```

### Step 3: 创建 ToolOutputPanel 工具输出面板
**文件**: `isdp/web/src/components/ToolOutputPanel.tsx`

**核心功能**:
- 流式状态自动展开/收起（使用 `useCollapsibleState` hook）
- 工具事件列表渲染
- 状态摘要显示
- **React.memo 优化**

**接口设计**:
```tsx
interface ToolOutputPanelProps {
  events: ToolEvent[];
  status: 'streaming' | 'done' | 'failed';
  defaultExpanded?: boolean;
  accentColor?: string;
}
```

### Step 4: 创建 ThinkingPanel 思考面板
**文件**: `isdp/web/src/components/ThinkingPanel.tsx`

**核心功能**:
- 预览文本显示（收起时）
- 可配置默认展开状态（使用 `useCollapsibleState` hook）
- 支持 label 自定义
- **React.memo 优化**

**接口设计**:
```tsx
interface ThinkingPanelProps {
  content: string;
  label?: string;
  defaultExpanded?: boolean;
  expandInExport?: boolean;
  accentColor?: string;
}
```

### Step 5: 创建 AgentCollaborationPanel 协作容器
**文件**: `isdp/web/src/components/AgentCollaborationPanel.tsx`

**核心功能**:
- Agent 间讨论分组（使用 `useCollapsibleState` hook）
- 左侧彩色边框（CSS 变量实现）
- 摘要行显示参与者
- **React.memo 优化**

**接口设计**:
```tsx
interface AgentCollaborationPanelProps {
  groupId: string;
  messages: Message[];
  renderMessage: (msg: Message) => React.ReactNode;
  getAgentColor?: (agentId: string) => string | undefined;
}
```

### Step 6: 状态管理集成
**文件**: `isdp/web/src/store/index.ts`

**新增状态**:
```tsx
collapsibleDefaults: {
  toolOutput: 'collapsed' as 'expanded' | 'collapsed';
  thinking: 'collapsed' as 'expanded' | 'collapsed';
};
```

**新增 Actions**:
- `setCollapsibleDefaults(type, state)`

### Step 7: 集成到 ChatMessageList
**文件**: `isdp/web/src/components/thread/ChatMessageList.tsx`

**修改内容**:
- 渲染 ThinkingPanel 用于思考内容
- 渲染 ToolOutputPanel 用于工具调用
- 集成 AgentCollaborationPanel 用于 A2A 消息分组

### Step 8: 样式文件创建
**文件**: `isdp/web/src/components/CollapsiblePanels.css`

**样式内容**:
- 面板容器样式
- 动态着色变量
- 过渡动画

---

## File Changes Summary

| File | Action | Description |
|------|--------|-------------|
| `isdp/web/src/hooks/useCollapsibleState.ts` | CREATE | 智能状态管理 Hook |
| `isdp/web/src/components/CollapsiblePanel.tsx` | CREATE | 通用收缩面板基础组件 |
| `isdp/web/src/components/ToolOutputPanel.tsx` | CREATE | 工具输出面板 |
| `isdp/web/src/components/ThinkingPanel.tsx` | CREATE | 思考面板 |
| `isdp/web/src/components/AgentCollaborationPanel.tsx` | CREATE | Agent 协作容器 |
| `isdp/web/src/components/CollapsiblePanels.css` | CREATE | 样式文件（CSS 变量） |
| `isdp/web/src/store/index.ts` | MODIFY | 新增 collapsibleDefaults 状态 |
| `isdp/web/src/components/thread/ChatMessageList.tsx` | MODIFY | 集成新组件 |
| `isdp/web/src/types/index.ts` | MODIFY | 新增 ToolEvent 类型定义 |

---

## Acceptance Criteria

| AC | 测试方法 |
|----|----------|
| 收缩展开状态正确 | 手动测试：点击 header 切换状态 |
| 流式输出自动展开/收起 | 启动 Agent 调用，观察面板行为 |
| 全局默认配置 | 修改 store 中的 collapsibleDefaults，验证默认状态 |
| 导出模式自动展开 | 访问 `?export=true`，验证所有面板展开 |

---

## Verification Steps

1. **单元测试**: CollapsiblePanel 状态切换逻辑
2. **集成测试**: ToolOutputPanel 与 toolEvents 状态同步
3. **E2E 测试**: 完整消息流中的面板行为
4. **手动验证**: 流式场景 + 导出模式

---

## ADR (Architect Approved)

### Decision
使用 **混合方案** 实现可收缩面板：Ant Design Collapse 容器 + CSS 变量 + 自定义状态管理

### Drivers
- 技术栈一致性（Ant Design）
- 动态着色需求（Agent 主题色）
- 智能状态管理（流式自动展开/收起）

### Alternatives Considered
- **直接移植 Tailwind 样式**：被拒绝，因引入额外依赖且与现有 UI 冲突
- **纯 CSS 自定义**：被拒绝，因需要重新实现可访问性（ARIA）
- **纯 Ant Design Collapse**：被拒绝，因样式覆盖复杂且不支持动态着色

### Why Chosen
混合方案平衡了：
- 与现有 UI 的一致性（使用 Ant Design 容器）
- 灵活的动态着色（CSS 变量）
- 可控的状态管理（自定义 Hook）

### Consequences
- 需要禁用 Collapse 的内置状态切换
- 使用 CSS 变量传递主题色
- 需要自定义箭头组件实现动画控制

### Follow-ups
- 后续可考虑抽象为独立 npm 包
- 添加更多自定义主题选项