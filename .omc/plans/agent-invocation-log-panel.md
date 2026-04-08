# Agent调用日志面板 - 实现计划

## 需求摘要
在右侧状态栏底部增加 Agent 调用日志入口，采用两层结构：
1. **第一层**：Agent 列表（名称 + 最近状态），按最近调用时间排序
2. **第二层**：点击 Agent 后展示该 Agent 的所有调用记录（时间、状态、输入内容、耗时）

## 验收标准
- [ ] AC1: 状态栏底部有入口按钮，点击展开内嵌面板
- [ ] AC2: 展开后显示 Agent 列表（名称 + 最近状态）
- [ ] AC3: Agent 列表按最近调用时间排序
- [ ] AC4: 点击 Agent 显示该 Agent 的所有调用记录
- [ ] AC5: 每条调用记录显示：时间、状态、输入内容、耗时
- [ ] AC6: 正在运行的 Agent 实时更新输入内容（延迟 < 100ms）
- [ ] AC7: 可滚动查看所有历史调用记录
- [ ] AC8: 不限制显示数量

## RALPLAN-DR 摘要

### 原则 (Principles)
1. **复用优先**：复用现有 Store 中的 `activeAgents` 和 `completedAgents` 数据，抽取共享 UI 组件
2. **组件独立**：新建独立组件 `AgentInvocationLogPanel`，与现有组件解耦
3. **最小改动**：不修改后端 API 和数据库，仅前端展示层改动
4. **实时更新**：利用现有 WebSocket 机制实现实时数据同步
5. **性能优化**：使用 selector 记忆化避免重复计算

### 决策驱动因素 (Decision Drivers)
1. **DD1**: 用户需要快速定位 Agent 调用，避免杂乱展示
2. **DD2**: 数据已在 Store 中可用，无需新增 API 调用
3. **DD3**: 需要与现有 StatusPanel 风格保持一致
4. **DD4**: 需要支持实时更新，用户可追踪正在运行的 Agent

### 可行选项 (Viable Options)

#### Option A: 新建独立组件 + Store 数据聚合（已选择）
- **优点**: 组件独立、易维护、不干扰现有组件、支持两层结构的独立状态管理
- **缺点**: 需要新增 Store selector 进行数据聚合
- **适用场景**: 需要独立交互模式和状态管理的复杂 UI

#### Option B: 扩展 AgentHistoryCard 组件（已拒绝）
- **优点**: 代码复用、改动最小
- **缺点**: 组件职责混淆、难以实现两层结构（需要管理 selectedAgent 状态）、与现有"历史参与"功能语义冲突
- **拒绝理由**: 两层结构需要独立状态管理，扩展现有组件会导致职责混淆，且 AgentHistoryCard 的定位是"已完成的 Agent 历史"，无法支持"正在运行的 Agent"场景

**选择**: Option A（独立组件）

---

## 实现步骤

### Step 1: 创建 Store Selector（数据聚合）
**文件**: `isdp/web/src/store/selectors/agentInvocations.ts`（新建）

```typescript
import { createSelector } from 'reselect';
import type { AppState } from '../index';
import type { AgentInvocation } from '@/types';

export interface AgentLogItem {
  agentConfigId: string;
  agentName: string;
  recentStatus: InvocationStatus;
  lastInvokedAt: string;
  invocations: AgentInvocation[];
}

// 使用 createSelector 实现记忆化
export const selectAgentLogList = createSelector(
  [(state: AppState) => state.activeAgents, (state: AppState) => state.completedAgents],
  (activeAgents, completedAgents): AgentLogItem[] => {
    // 合并并按 agentConfigId 分组
    // 按 lastInvokedAt 排序
    // 返回 AgentLogItem[]
  }
);
```

### Step 2: 抽取共享 UI 组件
**文件**: `isdp/web/src/components/thread/StatusPanel/shared/`（新建目录）

- `AgentStatusBadge.tsx` - 状态徽章组件（从 AgentStatusCard 提取）
- `DurationDisplay.tsx` - 已存在，直接复用
- `TimeDisplay.tsx` - 时间格式化组件（从 AgentHistoryCard 提取）

### Step 3: 创建 AgentInvocationLogPanel 组件
**文件**: `isdp/web/src/components/thread/StatusPanel/AgentInvocationLogPanel.tsx`（新建）

组件结构：
```
AgentInvocationLogPanel
├── 入口按钮（展开/收起）
├── 展开内容
│   ├── AgentList（第一层）
│   │   └── AgentListItem[]（名称 + AgentStatusBadge）
│   └── InvocationDetail（第二层，选中 Agent 时显示）
│       └── InvocationRecord[]（TimeDisplay + AgentStatusBadge + 输入内容 + DurationDisplay）
```

**性能优化**:
- 使用 `useMemo` 缓存聚合数据
- 使用 `useCallback` 缓存事件处理函数
- 输入内容使用 `React.memo` 防止不必要的重渲染

### Step 4: 添加样式
**文件**: `isdp/web/src/components/thread/StatusPanel/StatusPanel.css`（修改）

新增样式：
- `.log-panel-trigger` - 入口按钮
- `.log-panel-content` - 展开内容容器
- `.agent-log-list` - Agent 列表
- `.agent-log-item` - Agent 条目（可点击）
- `.agent-log-item--selected` - 选中状态
- `.invocation-detail` - 调用详情
- `.invocation-record` - 调用记录
- `.invocation-input` - 输入内容（最大高度限制 + 滚动）

### Step 5: 集成到 StatusPanel
**文件**: `isdp/web/src/components/thread/StatusPanel/index.tsx`（修改）

在组件末尾添加：
```tsx
<AgentInvocationLogPanel />
```

### Step 6: 实现实时更新
**文件**: `isdp/web/src/store/index.ts`（确认无需修改）

现有 `updateAgentStatus` 和 `updateInvocationStatus` action 已触发组件重新渲染。Selector 记忆化确保性能。

---

## 风险和缓解措施

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 大量 Agent 调用导致性能问题 | 中 | Selector 记忆化 + React.memo；虚拟滚动作为后续优化 |
| 输入内容过长导致布局问题 | 低 | 最大高度限制（如 200px）+ 滚动 |
| 实时更新导致频繁渲染 | 中 | useMemo/useCallback 优化；React DevTools 验证渲染次数 |
| XSS 安全风险 | 低 | React 默认转义用户输入；避免使用 dangerouslySetInnerHTML |

---

## ADR (Architecture Decision Record)

### Decision
采用新建独立组件 `AgentInvocationLogPanel` + Store Selector 数据聚合的方案。

### Drivers
- 用户需要两层结构的交互模式（Agent 列表 → 调用详情）
- 数据已在 Store 中可用
- 需要实时更新支持

### Alternatives considered
1. **扩展 AgentHistoryCard**: 拒绝，因为职责混淆且难以支持两层结构
2. **新建 Drawer/Modal**: 拒绝，用户明确要求内嵌展开面板

### Why chosen
独立组件可以清晰管理两层结构的状态（expanded、selectedAgent），与现有组件解耦，便于维护和扩展。

### Consequences
- 新增约 300 行代码（组件 + selector + 样式）
- 需要确保与现有组件风格一致
- 后续可能需要虚拟滚动优化

### Follow-ups
- [ ] 性能监控：如果 Agent 数量超过 100，考虑虚拟滚动
- [ ] 功能扩展：输入内容复制按钮

---

## 验证步骤

### 功能测试
1. 启动后端服务，打开前端页面
2. 创建新 Thread，触发 Agent 调用
3. 验证入口按钮可见，点击可展开面板
4. 验证 Agent 列表正确显示（名称 + 状态徽章）
5. 点击 Agent，验证调用记录详情显示
6. 验证调用记录包含：时间、状态、输入内容、耗时

### 实时更新测试
1. 触发多个 Agent 并行执行
2. 验证列表实时更新
3. 点击运行中的 Agent，验证输入内容实时刷新
4. 使用 React DevTools 验证渲染次数合理

### 边界测试
1. 无 Agent 调用时显示空状态
2. 大量调用记录时可滚动查看
3. 输入内容过长时可滚动查看

---

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `store/selectors/agentInvocations.ts` | 新建 | 数据聚合 selector（使用 reselect） |
| `components/thread/StatusPanel/shared/AgentStatusBadge.tsx` | 新建 | 共享状态徽章组件 |
| `components/thread/StatusPanel/shared/TimeDisplay.tsx` | 新建 | 共享时间格式化组件 |
| `components/thread/StatusPanel/AgentInvocationLogPanel.tsx` | 新建 | 主组件 |
| `components/thread/StatusPanel/StatusPanel.css` | 修改 | 新增样式 |
| `components/thread/StatusPanel/index.tsx` | 修改 | 集成新组件 |

## 预估工作量
- Step 1: Store Selector - 30 分钟
- Step 2: 抽取共享组件 - 30 分钟
- Step 3: 主组件开发 - 1.5 小时
- Step 4: 样式添加 - 30 分钟
- Step 5: 集成测试 - 30 分钟
- Step 6: 实时更新调试 - 30 分钟

**总计**: 约 4 小时

---

## 改进日志

| 来源 | 改进内容 | 状态 |
|------|----------|------|
| Architect | 抽取共享 UI 组件避免重复代码 | ✅ 已纳入 Step 2 |
| Architect | selector 使用 createSelector 实现记忆化 | ✅ 已纳入 Step 1 |
| Architect | 输入内容添加最大高度限制和复制按钮 | ✅ 已纳入样式 + Follow-ups |
| Critic | 明确记录拒绝 Option B 的理由 | ✅ 已添加到选项对比 |
| Critic | 补充 XSS 安全考虑 | ✅ 已纳入风险缓解措施 |
| Critic | AC6 量化实时更新延迟要求 | ✅ 已更新 AC6 |