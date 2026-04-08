# Deep Interview Spec: ISDP 右侧状态栏

## Metadata
- Interview ID: di-20260402-statusbar
- Rounds: 8
- Final Ambiguity Score: 12.8%
- Type: brownfield
- Generated: 2026-04-02
- Threshold: 20%
- Status: PASSED

## Clarity Breakdown
| Dimension | Score | Weight | Weighted |
|-----------|-------|--------|----------|
| Goal Clarity | 0.90 | 35% | 0.315 |
| Constraint Clarity | 0.85 | 25% | 0.2125 |
| Success Criteria | 0.90 | 25% | 0.225 |
| Context Clarity | 0.80 | 15% | 0.12 |
| **Total Clarity** | | | **0.8725** |
| **Ambiguity** | | | **12.8%** |

## Goal

在 ISDP 的 ThreadView 页面右侧实现一个固定状态栏面板，实时展示多 Agent 协作过程中的状态信息，包括：Agent 状态、Token 统计、任务进度、消息统计。

## Constraints

### 技术约束
1. **数据来源**: 通过 Claude Code CLI 的 `--output-format stream-json` 获取 usage 数据
2. **前端框架**: React + Ant Design + Zustand（与现有 ISDP 技术栈一致）
3. **布局**: 状态栏固定在 ThreadView 右侧，与现有 RightPanel 并存
4. **实时性**: 通过 WebSocket 推送状态更新

### 数据模型约束
1. **多 Agent 任务列表**: 每个 AgentInvocation 有独立的 taskProgress 字段
2. **Usage 字段**: 需从 CLI 输出解析 message_start、message_delta、result 事件

### 现有代码上下文
- `useAppStore` 已有 `progressState`, `streamingMessages`, `activeAgents` 字段
- `RightPanel` 组件已有代码预览和沙箱 Tab
- `claude_adapter.go` 已解析 stream-json，但未提取 usage

## Non-Goals

1. **会话链面板 (SessionChainPanel)**: 不在本期实现范围
2. **审计/证据面板 (AuditExplorerPanel)**: 不在本期实现范围
3. **运行日志按钮 (RuntimeLogsButton)**: 不在本期实现范围
4. **ThinkingModeToggle / BubbleDisplayToggle / RevealWhispersButton**: clowder-ai 特有功能，不实现

## Acceptance Criteria

- [ ] **AC1: Agent 状态实时显示**
  - 右侧面板显示当前活跃的 Agent 列表
  - 每个 Agent 显示状态标识（pending/streaming/done/error）
  - 显示调用时长（实时计时或已完成时长）
  - 显示 Agent 名称和颜色标识

- [ ] **AC2: Token 统计展示**
  - 显示 input tokens 和 output tokens（带动画计数）
  - 显示缓存命中率（cache_read_tokens / input_tokens）
  - 显示 API 成本（costUsd）
  - 显示 API 响应时长（durationApiMs）

- [ ] **AC3: 任务进度面板**
  - 每个 Agent 显示独立的任务列表
  - 每个任务显示状态图标（completed/in_progress/pending）
  - 显示任务进度统计（X/Y 已完成）
  - 支持中断/继续任务操作

- [ ] **AC4: 消息统计面板**
  - 显示消息总数
  - 显示 Agent 消息数
  - 显示系统消息数
  - 显示用户消息数

## Assumptions Exposed & Resolved
| Assumption | Challenge | Resolution |
|------------|-----------|------------|
| Token 数据需要调用 Claude API 获取 | ISDP 使用 CLI 方式启动，是否可以获取 token？ | CLI 的 stream-json 输出包含 usage 字段，可解析获取 |
| 状态栏替换现有 RightPanel | 用户是否想要替换还是新增？ | 用户选择 ThreadView 右侧固定，与 RightPanel 并存 |
| 单 Agent 任务列表 | 多 Agent 协作时任务如何组织？ | 用户确认多 Agent 各自任务列表模式 |

## Technical Context

### 后端改动

#### 1. 扩展 `claude_adapter.go` 解析 usage
```go
// parseStreamJSONLine 需新增处理
case "message_start":
    // 解析 message.usage 字段
case "message_delta":
    // 解析 usage 字段
case "result":
    // 解析 usage, total_cost_usd, duration_ms 等字段
```

#### 2. 扩展 `Chunk` 类型
```go
type Chunk struct {
    Type     ChunkType
    Content  string
    ToolName string
    ToolInput map[string]interface{}
    // 新增
    Usage    *TokenUsage
}

type TokenUsage struct {
    InputTokens           int64
    OutputTokens          int64
    CacheReadTokens       int64
    CacheCreationTokens   int64
    CostUsd               float64
    DurationMs            int64
    DurationApiMs         int64
}
```

#### 3. 扩展 `AgentInvocation` 模型
```go
type AgentInvocation struct {
    // 现有字段...
    Usage       *TokenUsage    `json:"usage"`
    TaskProgress *TaskProgress `json:"taskProgress"`
}

type TaskProgress struct {
    SnapshotStatus string       `json:"snapshotStatus"` // completed/interrupted/running
    Tasks          []TaskItem   `json:"tasks"`
}

type TaskItem struct {
    ID     string `json:"id"`
    Title  string `json:"title"`
    Status string `json:"status"` // completed/in_progress/pending
}
```

#### 4. WebSocket 推送扩展
- 新增 `usage_update` 事件类型
- 新增 `task_progress_update` 事件类型

### 前端改动

#### 1. 新增 `StatusPanel` 组件
```
isdp/web/src/components/thread/StatusPanel/
├── index.tsx              # 主组件
├── AgentStatusCard.tsx    # Agent 状态卡片
├── TokenUsage.tsx         # Token 统计展示
├── TaskProgressPanel.tsx  # 任务进度面板
└── MessageStats.tsx       # 消息统计面板
```

#### 2. 扩展 `useAppStore`
```typescript
interface AppState {
  // 新增
  agentUsage: Record<string, TokenUsage>;
  agentTaskProgress: Record<string, TaskProgress>;
}

interface TokenUsage {
  inputTokens?: number;
  outputTokens?: number;
  cacheReadTokens?: number;
  costUsd?: number;
  durationApiMs?: number;
}

interface TaskProgress {
  snapshotStatus: 'completed' | 'interrupted' | 'running';
  tasks: TaskItem[];
}
```

#### 3. ThreadView 布局调整
```
┌──────────────────────────────────────────────────────────┐
│ ThreadView                                               │
├────────────────────────────┬─────────────────────────────┤
│                            │ StatusPanel (固定宽度 320px)│
│     消息列表区域            │ ├─ AgentStatusCard         │
│                            │ ├─ TokenUsage              │
│                            │ ├─ TaskProgressPanel       │
│                            │ └─ MessageStats            │
├────────────────────────────┴─────────────────────────────┤
│ 输入区域                                                 │
└──────────────────────────────────────────────────────────┘
```

## Ontology (Key Entities)

| Entity | Type | Fields | Relationships |
|--------|------|--------|---------------|
| StatusPanel | UI Component | width, visible | 展示 AgentInvocation 数据 |
| AgentStatusCard | UI Component | agentId, status, duration | 属于 StatusPanel |
| TokenUsage | Data Model | inputTokens, outputTokens, costUsd | 属于 AgentInvocation |
| TaskProgress | Data Model | snapshotStatus, tasks[] | 属于 AgentInvocation |
| TaskItem | Data Model | id, title, status | 属于 TaskProgress |

## Ontology Convergence

| Round | Entity Count | New | Changed | Stable | Stability Ratio |
|-------|-------------|-----|---------|--------|----------------|
| 1 | 4 | 4 | - | - | N/A |
| 2 | 5 | 1 | 0 | 4 | 80% |
| 3-8 | 5 | 0 | 0 | 5 | 100% |

**Ontology 已收敛** — 5 个核心实体在后续轮次保持稳定。

## Interview Transcript
<details>
<summary>Full Q&A (8 rounds)</summary>

### Round 1
**Q:** 你希望实现 clowder-ai 状态栏中的哪些核心功能？
**A:** 状态监控 + 进度追踪
**Ambiguity:** 65% (Goal: 0.6, Constraints: 0.5, Criteria: 0.4)

### Round 2
**Q:** 状态栏应该放在什么位置？
**A:** ThreadView右侧固定
**Ambiguity:** 50% (Goal: 0.7, Constraints: 0.5, Criteria: 0.4)

### Round 3
**Q:** 状态栏需要的数据是否已经在ISDP中可用？
**A:** 前后端都需要改动
**Ambiguity:** 40% (Goal: 0.75, Constraints: 0.6, Criteria: 0.5)

### Round 4
**Q:** 状态栏的主要使用场景是什么？
**A:** 两者兼顾（开发调试 + 业务用户）
**Ambiguity:** 35% (Goal: 0.8, Constraints: 0.65, Criteria: 0.55)

### Round 5
**Q:** Agent的任务列表是如何组织的？
**A:** 多Agent各自任务列表
**Ambiguity:** 30% (Goal: 0.85, Constraints: 0.7, Criteria: 0.6)

### Round 6 (Clarification)
**Q:** Token使用量统计的数据从哪里来？
**A:** ISDP是通过CLI方式启动claudecode的，是否可以获取到token的信息？
**Clarified:** 是的，CLI 的 stream-json 输出包含 usage 字段

### Round 7
**Q:** 请选择状态栏必须具备的功能（可多选）
**A:** Agent状态实时显示, Token统计展示, 任务进度面板, 消息统计面板
**Ambiguity:** 21% (Goal: 0.88, Constraints: 0.8, Criteria: 0.85)

### Round 8
**Q:** 实现策略：你希望如何安排开发节奏？
**A:** 本次完整实现
**Ambiguity:** 12.8% (Goal: 0.90, Constraints: 0.85, Criteria: 0.90)

</details>