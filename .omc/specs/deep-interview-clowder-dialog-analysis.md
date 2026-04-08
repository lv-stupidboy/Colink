# Deep Interview Spec: clowder-ai 对话框收缩/展开机制移植

## Metadata
- Interview ID: clowder-dialog-analysis-001
- Rounds: 6
- Final Ambiguity Score: 8.75%
- Type: brownfield
- Generated: 2026-04-03
- Threshold: 0.2
- Status: PASSED

## Clarity Breakdown
| Dimension | Score | Weight | Weighted |
|-----------|-------|--------|----------|
| Goal Clarity | 0.95 | 0.35 | 0.3325 |
| Constraint Clarity | 0.85 | 0.25 | 0.2125 |
| Success Criteria Clarity | 0.9 | 0.25 | 0.225 |
| Context Clarity | 0.95 | 0.15 | 0.1425 |
| **Total Clarity** | | | **0.9125** |
| **Ambiguity** | | | **8.75%** |

## Goal
将 clowder-ai 的对话框收缩/展开机制移植到 ISDP，包括：
1. 工具调用输出面板 (CliOutputBlock)
2. 思考过程展示面板 (ThinkingContent)
3. Agent 间协作容器 (A2ACollapsible)
4. 通用 Collapsible 基础组件

## Constraints
- **技术栈差异**：clowder-ai 使用 Tailwind CSS，ISDP 使用 Ant Design
- **移植方式**：直接移植组件逻辑，UI 样式适配 Ant Design
- **状态管理**：两者都使用 Zustand，可直接复用状态管理逻辑
- **事件机制**：保留 `catcafe:chat-layout-changed` 事件用于滚动计算通知

## Non-Goals
- 不修改 clowder-ai 的源代码
- 不引入 Tailwind CSS 到 ISDP
- 不重写 Ant Design 的 Collapse 组件

## Acceptance Criteria
- [ ] 收缩展开状态正确，用户交互流畅
- [ ] 流式输出时自动展开，完成后自动收起
- [ ] 支持全局默认展开/收起配置
- [ ] 导出模式 (`?export=true`) 时所有面板自动展开

## Technical Context

### clowder-ai 关键实现分析

#### 1. CliOutputBlock (`packages/web/src/components/cli-output/CliOutputBlock.tsx`)

**核心特性：**
- 流式状态自动展开：`status === 'streaming'` 时强制展开
- 完成后自动收起：使用 `useRef` 追踪用户交互，非用户操作时自动收起
- 颜色主题：根据 Agent 的 `breedColor` 动态着色
- 事件通知：`catcafe:chat-layout-changed` 通知滚动依赖 UI

**关键代码片段：**
```tsx
// 流式时自动展开
const forceExpanded = status === 'streaming' || isExport;
const [expanded, setExpanded] = useState(forceExpanded || defaultExpanded);
const userInteracted = useRef(false);

// 完成后自动收起（非用户操作时）
useEffect(() => {
  if (prevStatusRef.current === 'streaming' && status !== 'streaming' && !userInteracted.current) {
    setExpanded(false);
  }
  prevStatusRef.current = status;
}, [status]);

// 通知滚动计算
useLayoutEffect(() => {
  if (!hasMounted.current) { hasMounted.current = true; return; }
  window.dispatchEvent(new Event('catcafe:chat-layout-changed'));
}, [expanded]);
```

#### 2. ThinkingContent (`packages/web/src/components/ThinkingContent.tsx`)

**核心特性：**
- 可配置默认展开状态：`defaultExpanded` prop
- 导出模式自动展开：`isExport && expandInExport`
- 预览文本：收起时显示前 60 字符
- 颜色主题：支持 `breedColor` 着色

**关键代码片段：**
```tsx
const [expanded, setExpanded] = useState(shouldExpand);
const preview = content.length > 60 ? `${content.slice(0, 60)}…` : content;

// 导出模式自动展开
const isExport = typeof window !== 'undefined' && 
  new URLSearchParams(window.location.search).get('export') === 'true';
const shouldExpand = (isExport && expandInExport) || defaultExpanded;
```

#### 3. A2ACollapsible (`packages/web/src/components/A2ACollapsible.tsx`)

**核心特性：**
- Agent 间讨论容器
- 左侧边框使用第一个 Agent 的颜色
- 导出模式自动展开

**关键代码片段：**
```tsx
const [expanded, setExpanded] = useState(isExport);
const borderColor = (firstCatId && getCatColor?.(firstCatId)) ?? '#9B7EBD';

// 收起时显示摘要
<span>
  {expanded ? '收起内部讨论' : `查看内部讨论`} ({catLabel}, {count} 条)
</span>
```

### ISDP 现有架构

#### 相关文件
| 文件 | 职责 |
|------|------|
| `isdp/web/src/pages/ThreadView.tsx` | 主视图，包含消息渲染 |
| `isdp/web/src/store/index.ts` | Zustand 状态管理 |
| `isdp/web/src/components/thread/ChatMessageList.tsx` | 消息列表组件 |

#### 状态管理
ISDP 已有 `toolEvents` 状态追踪工具调用事件：
```tsx
const [toolEvents, setToolEvents] = useState<Record<string, ToolEvent[]>>({});
```

### 移植计划

#### 文件创建清单
| 新建文件 | 基于 | 说明 |
|----------|------|------|
| `isdp/web/src/components/CollapsiblePanel.tsx` | CliOutputBlock + ThinkingContent | 通用收缩面板基础组件 |
| `isdp/web/src/components/ToolOutputPanel.tsx` | CliOutputBlock | 工具调用输出面板 |
| `isdp/web/src/components/ThinkingPanel.tsx` | ThinkingContent | 思考过程展示面板 |
| `isdp/web/src/components/AgentCollaborationPanel.tsx` | A2ACollapsible | Agent 协作容器 |

#### 样式适配策略
- **Tailwind → Ant Design**：使用 Ant Design 的 `Collapse` 作为基础结构
- **颜色系统**：保留 `breedColor` → `agentColor` 的动态着色逻辑
- **动画过渡**：使用 CSS transition 或 Ant Design 内置动画

#### 状态管理集成
```tsx
// store/index.ts 新增
interface AppState {
  // ... 现有状态
  
  // 可收缩面板配置
  collapsibleDefaults: {
    toolOutput: 'expanded' | 'collapsed';
    thinking: 'expanded' | 'collapsed';
  };
}

// 新增 actions
setCollapsibleDefaults: (type: 'toolOutput' | 'thinking', state: 'expanded' | 'collapsed') => void;
```

## Ontology (Key Entities)

| Entity | Type | Fields | Relationships |
|--------|------|--------|---------------|
| CollapsiblePanel | core component | expanded, defaultExpanded, onToggle, breedColor | 基础组件 |
| ToolOutputPanel | component | events, status, thinkingMode, defaultExpanded | extends CollapsiblePanel |
| ThinkingPanel | component | content, label, defaultExpanded, expandInExport | extends CollapsiblePanel |
| AgentCollaborationPanel | component | groupId, messages, getCatColor | uses CollapsiblePanel |
| ToolEvent | data | id, name, status, input, output, duration | 被ToolOutputPanel渲染 |

## Ontology Convergence
| Round | Entity Count | New | Changed | Stable | Stability Ratio |
|-------|-------------|-----|---------|--------|----------------|
| 1 | 3 | 3 | - | - | - |
| 2 | 4 | 1 | 0 | 3 | 75% |
| 3 | 4 | 0 | 0 | 4 | 100% |
| 4-6 | 5 | 1 | 0 | 4 | 80% → 100% |

最终收敛到 5 个核心实体，稳定。

## Interview Transcript
<details>
<summary>Full Q&A (6 rounds)</summary>

### Round 1
**Q:** 你希望深入分析哪个方面？
**A:** 收缩/展开机制
**Ambiguity:** 68.5% (Goal: 0.4, Constraints: 0.3, Criteria: 0.3, Context: 0.6)

### Round 2
**Q:** 你分析收缩/展开机制的目的是什么？
**A:** 复用到 ISDP
**Ambiguity:** 50.5% (Goal: 0.7, Constraints: 0.5, Criteria: 0.5, Context: 0.7)

### Round 3
**Q:** ISDP 需要哪些收缩/展开场景？(可多选)
**A:** 工具调用输出, 思考过程展示, Agent 间协作, 通用 Collapsible 组件
**Ambiguity:** 41.5% (Goal: 0.9, Constraints: 0.6, Criteria: 0.6, Context: 0.8)

### Round 4
**Q:** ISDP 使用 Ant Design，希望如何实现收缩组件？
**A:** 移植组件 (推荐)
**Ambiguity:** 38.25% (Goal: 0.95, Constraints: 0.7, Criteria: 0.7, Context: 0.9)

### Round 5
**Q:** 验收标准有哪些？(可多选)
**A:** 功能正确, 智能状态管理, 配置化, 导出模式支持
**Ambiguity:** 32.5% (Goal: 0.95, Constraints: 0.85, Criteria: 0.75, Context: 0.95)

### Round 6
**Q:** 验收标准有哪些？(确认完成)
**A:** 功能正确, 智能状态管理, 配置化, 导出模式支持
**Ambiguity:** 8.75% (Goal: 0.95, Constraints: 0.85, Criteria: 0.9, Context: 0.95)

</details>

## Assumptions Exposed & Resolved
| Assumption | Challenge | Resolution |
|------------|-----------|------------|
| ISDP 需要 Tailwind | 发现 ISDP 用 Ant Design | 改为移植逻辑 + 适配 Ant Design 样式 |
| 只需要一个收缩组件 | 分析发现三种不同场景 | 拆分为通用组件 + 三种专用组件 |
| 用户手动控制展开收起 | 发现流式场景需要智能状态 | 实现 userInteracted 追踪 + 自动状态切换 |