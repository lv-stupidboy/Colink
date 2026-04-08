# 实现计划：Chat UX Improvements

## 需求总结
优化 ISDP 对话框用户体验，参考 clowder-ai 实现：
1. CLI 输出块可折叠，streaming 结束后自动收起
2. 完整工具调用列表（名称、状态、耗时、详情）
3. 消息悬停工具栏（删除、分支操作）

## RALPLAN-DR Summary

### Principles
1. **最小侵入性** - 复用现有数据流和组件结构，不破坏现有功能
2. **渐进增强** - 新功能作为可选组件添加，保持向后兼容
3. **状态驱动** - 扩展现有 progressState 机制，添加 toolEvents 存储
4. **用户优先** - 自动收起不打断用户交互，提供手动控制

### Decision Drivers
1. **现有基础设施** - WebSocket 已传递 toolName/toolInput，只需扩展存储
2. **组件复用** - ChatMessage/ChatMessageList 已有 progress 状态渲染逻辑
3. **用户体验一致性** - 参考 clowder-ai 验证过的交互模式

### Viable Options

| Option | Pros | Cons |
|--------|------|------|
| **A. 扩展现有组件** | 改动小，复用已有逻辑 | 组件可能变复杂 |
| **B. 新建独立组件** | 完全控制，不影响现有代码 | 需要迁移成本 |

**推荐：Option A** - 扩展现有组件，将 CliOutputBlock 作为 ChatMessage 的子组件。

---

## Acceptance Criteria
- [ ] CLI 输出块组件：显示工具列表，streaming 结束后自动收起
- [ ] 工具列表：展示工具名称、状态图标、耗时、可展开查看详情
- [ ] 悬停工具栏：显示删除/分支按钮，分支创建新对话
- [ ] 自动收起逻辑：用户未交互时，streaming 结束后自动折叠

---

## Implementation Steps

### Step 1: 扩展类型定义 (15min)
**文件**: `isdp/web/src/types/index.ts`

新增类型：
```typescript
// 工具调用事件
export interface ToolEvent {
  id: string;
  invocationId: string;
  name: string;           // Bash, Read, Edit, etc.
  status: 'running' | 'success' | 'failed';
  input?: Record<string, unknown>;
  output?: string;
  startedAt: number;
  completedAt?: number;
  duration?: number;      // ms
}
```

### Step 2: 添加 toolEvents 本地状态 (30min)
**文件**: `isdp/web/src/pages/ThreadView.tsx`

> **Architect 决定**：使用 ThreadView 本地 state 而非全局 Zustand store，
> 避免 store 无限增长，invocation 完成时自动清理。

在 ThreadView 组件中添加本地状态：
```typescript
// 本地工具事件状态（非全局 store）
const [toolEvents, setToolEvents] = useState<Record<string, ToolEvent[]>>({});

// 添加工具事件
const addToolEvent = useCallback((invocationId: string, event: ToolEvent) => {
  setToolEvents(prev => ({
    ...prev,
    [invocationId]: [...(prev[invocationId] || []), event]
  }));
}, []);

// 更新工具事件
const updateToolEvent = useCallback((invocationId: string, eventId: string, update: Partial<ToolEvent>) => {
  setToolEvents(prev => ({
    ...prev,
    [invocationId]: prev[invocationId]?.map(e =>
      e.id === eventId ? { ...e, ...update } : e
    ) || []
  }));
}, []);

// invocation 完成后延迟清理
const clearToolEvents = useCallback((invocationId: string, delay = 5000) => {
  setTimeout(() => {
    setToolEvents(prev => {
      const next = { ...prev };
      delete next[invocationId];
      return next;
    });
  }, delay);
}, []);
```

### Step 3: 更新 ThreadView WebSocket 处理 (30min)
**文件**: `isdp/web/src/pages/ThreadView.tsx`

修改 `agent_output_chunk` 处理逻辑，将 tool_use 事件存入 toolEvents：
- 解析 `data.payload.toolName`, `data.payload.toolInput`
- 创建 ToolEvent 对象，调用 `addToolEvent`
- 工具完成时更新 status 和 duration

### Step 4: 创建 CliOutputBlock 组件 (60min)
**文件**: `isdp/web/src/components/thread/CliOutputBlock.tsx`

功能：
1. 接收 `invocationId` 和 `toolEvents` 作为 props
2. 显示工具列表（可折叠）
3. 自动收起逻辑：`prevStatus === 'streaming' && status !== 'streaming' && !userInteracted`
4. 每个工具行显示：状态图标、名称、耗时、可展开详情

参考 clowder-ai 的 CliOutputBlock.tsx 结构。

### Step 5: 创建 ToolRow 组件 (30min)
**文件**: `isdp/web/src/components/thread/ToolRow.tsx`

功能：
1. 显示单个工具调用
2. 状态图标：running (旋转)、success (绿色勾)、failed (红色叉)
3. 点击展开显示 input/output 详情

### Step 6: 创建 MessageActions 组件 (45min)
**文件**: `isdp/web/src/components/thread/MessageActions.tsx`

功能：
1. 包裹 ChatMessage，悬停时显示工具栏
2. 删除按钮：调用 API 删除消息
3. 分支按钮：调用 `/api/threads/{threadId}/branch` 创建新对话

CSS: `opacity-0 group-hover:opacity-100`

### Step 7: 更新 ChatMessage 组件 (30min)
**文件**: `isdp/web/src/components/thread/ChatMessage.tsx`

修改：
1. 引入 CliOutputBlock 组件
2. 从 store 获取 toolEvents
3. 在消息气泡下方渲染 CliOutputBlock
4. 用 MessageActions 包裹消息

### Step 8: 添加样式 (20min)
**文件**: `isdp/web/src/components/thread/CliOutputBlock.css`

样式包括：
- 工具列表容器样式
- 工具行样式（不同状态的颜色）
- 展开/折叠动画
- 悬停工具栏样式

### Step 9: 测试和调试 (30min)

测试用例：
1. 发送消息触发 Agent，验证工具列表正确显示
2. streaming 结束后验证自动收起
3. 用户交互后验证不再自动收起
4. 悬停消息验证工具栏显示
5. 点击分支验证新对话创建

---

## Files to Create/Modify

| 文件 | 操作 | 说明 |
|------|------|------|
| `src/types/index.ts` | 修改 | 新增 ToolEvent 类型 |
| `src/pages/ThreadView.tsx` | 修改 | 添加 toolEvents 本地状态、WebSocket 处理 |
| `src/components/thread/CliOutputBlock.tsx` | 新建 | CLI 输出块组件（memo 优化） |
| `src/components/thread/ToolRow.tsx` | 新建 | 工具行组件 |
| `src/components/thread/MessageActions.tsx` | 新建 | 消息操作工具栏 |
| `src/components/thread/ChatMessage.tsx` | 修改 | 集成 CliOutputBlock 和 MessageActions |
| `src/components/thread/CliOutputBlock.css` | 新建 | 样式文件 |
| `src/components/thread/index.ts` | 修改 | 导出新组件 |

---

## Risks and Mitigations

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 工具事件存储增长 | 中 | Agent 完成时清理对应 invocationId 的事件 |
| 自动收起打断用户 | 低 | 使用 userInteracted ref 跟踪用户交互 |
| 分支 API 不存在 | 高 | 检查后端 API，若不存在则先实现后端 |
| 样式冲突 | 低 | 使用唯一类名前缀 `.cli-output-` |

---

## Verification Steps

1. **功能验证**
   - 触发 Agent 执行，观察工具列表显示
   - 等待 streaming 结束，验证自动收起
   - 点击展开工具，查看详情
   - 悬停消息，点击分支按钮

2. **性能验证**
   - 长对话中工具列表不影响滚动流畅度
   - 快速发送消息时组件正常渲染

3. **边界情况**
   - 空工具列表时不显示 CliOutputBlock
   - 用户交互后 streaming 结束不自动收起

---

## ADR (Architecture Decision Record)

### Decision
使用 ThreadView 本地 state 管理 toolEvents，而非全局 Zustand store。

### Drivers
1. 工具事件是 invocation 生命周期内的临时数据，无需持久化
2. 全局 store 会导致内存无限增长
3. invocation 完成后可立即清理，无需用户手动操作

### Alternatives considered
1. **全局 Zustand store** - 便于跨组件访问，但需要手动清理机制
2. **React Context** - 可行但增加了 context 复杂度

### Why chosen
本地 state 最简单，自动跟随 ThreadView 组件生命周期，无需额外清理逻辑。

### Consequences
- toolEvents 无法在 ThreadView 外访问（但当前无此需求）
- 页面刷新后工具列表丢失（可接受，因为是临时数据）

### Follow-ups
- 如果未来需要在其他地方访问工具列表，可考虑迁移到 Zustand

---

## Changelog
- 2026-04-02: Initial plan created by Planner
- 2026-04-02: Architect review - 建议改用本地 state，添加 memo/useCallback
- 2026-04-02: Critic approved with improvements - 更新 Step 2 为本地 state