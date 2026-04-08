# Deep Interview Spec: Agent调用日志面板

## Metadata
- Interview ID: 550e8400-e29b-41d4-a716-446655440001
- Rounds: 5
- Final Ambiguity Score: 4.6%
- Type: brownfield
- Generated: 2026-04-08
- Threshold: 0.2
- Status: PASSED

## Clarity Breakdown
| Dimension | Score | Weight | Weighted |
|-----------|-------|--------|----------|
| Goal Clarity | 0.95 | 35% | 0.333 |
| Constraint Clarity | 0.90 | 25% | 0.225 |
| Success Criteria | 0.95 | 25% | 0.238 |
| Context Clarity | 0.85 | 15% | 0.128 |
| **Total Clarity** | | | **0.954** |
| **Ambiguity** | | | **4.6%** |

## Goal
在右侧状态栏底部增加 Agent 调用日志入口，采用两层结构展示：
1. 第一层：Agent 列表（名称 + 最近状态）
2. 第二层：点击 Agent 后显示其所有调用记录（时间、状态、输入内容、耗时）

## Constraints
- 入口按钮位于状态栏底部
- 点击后展开内嵌面板（非弹窗/Drawer）
- Agent 列表按最近调用时间排序
- 不限制显示数量
- 正在运行的 Agent 实时更新输入内容
- 不显示触发方式（暂无数据库字段支持）

## Non-Goals
- 不实现触发方式显示（需要新增数据库字段，暂不处理）
- 不实现筛选/搜索功能
- 不实现日志复制功能
- 不实现点击跳转到消息功能

## Acceptance Criteria
- [ ] 状态栏底部有入口按钮，点击展开内嵌面板
- [ ] 展开后显示 Agent 列表（名称 + 最近状态）
- [ ] Agent 列表按最近调用时间排序
- [ ] 点击 Agent 显示该 Agent 的所有调用记录
- [ ] 每条调用记录显示：时间、状态、输入内容、耗时
- [ ] 正在运行的 Agent 实时更新输入内容
- [ ] 可滚动查看所有历史调用记录
- [ ] 不限制显示数量

## Assumptions Exposed & Resolved
| Assumption | Challenge | Resolution |
|------------|-----------|------------|
| "日志" = 调用记录列表 | 询问日志具体内容 | 确定为两层结构：Agent列表 → 调用详情 |
| 显示触发方式 | 发现数据库无此字段 | 暂不显示触发方式 |
| 所有Agent杂乱展示 | 用户提出UX优化 | 采用两层结构，先显示Agent列表 |

## Technical Context
- 状态栏组件：`isdp/web/src/components/thread/StatusPanel/index.tsx`
- Agent调用模型：`isdp/internal/model/agent_invocation.go`
- WebSocket实时推送：`isdp/web/src/hooks/useWebSocket.ts`
- 现有组件：AgentStatusCard、AgentHistoryCard、TokenUsage、MessageStats、TaskProgressPanel

## Ontology (Key Entities)
| Entity | Type | Fields | Relationships |
|--------|------|--------|---------------|
| AgentInvocationLogPanel | core domain | agentList, selectedAgent, expanded | 嵌入 StatusPanel |
| AgentListItem | supporting | agentName, recentStatus, lastInvokedAt | 来自 AgentInvocation 聚合 |
| InvocationDetail | supporting | startedAt, status, input, durationMs | 来自 AgentInvocation |
| StatusPanel | existing container | children: AgentInvocationLogPanel | 已存在 |
| AgentInvocation | data source | id, agentName, status, input, startedAt, durationMs | 数据库模型 |

## Ontology Convergence
| Round | Entity Count | New | Changed | Stable | Stability Ratio |
|-------|-------------|-----|---------|--------|----------------|
| 1 | 4 | 4 | - | - | - |
| 3 | 4 | 0 | 0 | 4 | 100% |
| 5 | 5 | 1 | 0 | 4 | 80% |

## Interview Transcript
<details>
<summary>Full Q&A (5 rounds)</summary>

### Round 1
**Q:** 您说的「Agent调用日志」具体想展示什么信息？
**A:** Agent的输入

### Round 2
**Q:** 这个「Agent输入日志」应该放在状态栏的什么位置？
**A:** 独立弹窗查看 → 内嵌展开面板

**Q:** Agent 调用日志应该按什么顺序排列？
**A:** 时间倒序（最新在上）

**Q:** 每条 Agent 调用记录应该展示哪些信息？
**A:** 基本信息、输入内容、触发方式

**Q:** Agent 输入内容应该以什么方式展示？
**A:** 完整原始文本

**Q:** 现有 AgentInvocation 模型没有「触发方式」字段。如何处理？
**A:** 暂不显示触发方式

**Q:** 日志面板应该如何获取数据？
**A:** 实时推送更新

**Q:** 日志面板应该显示多少条调用记录？
**A:** 不限制数量

### Round 3
**Q:** Agent 列表（第一层）应该显示什么信息？
**A:** 名称 + 最近状态

**Q:** 点击 Agent 后，详情（第二层）应该显示什么？
**A:** 每次调用基本信息、每次调用输入内容、耗时信息

**Q:** Agent 列表如何排序？
**A:** 最近调用优先

**Q:** 对于正在运行的 Agent，点击后如何展示？
**A:** 实时更新（推荐）

### Round 5
**Q:** 这 8 条验收标准是否完整？
**A:** 已完整，开始实现
</details>