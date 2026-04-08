# ISDP 右侧状态栏实现计划

## RALPLAN-DR Summary

### Principles
1. **数据先行**: 后端数据模型和解析逻辑先于前端组件实现
2. **增量推送**: 使用 WebSocket 实时推送 usage 和 taskProgress 更新
3. **模块化组件**: 前端组件按功能拆分，便于测试和维护
4. **渐进增强**: 先实现核心状态显示，再添加高级功能

### Decision Drivers
1. **实时性要求**: Agent 执行过程中需要实时显示状态和 Token 消耗
2. **数据来源**: Token 信息来自 CLI stream-json 输出，需要解析特定事件
3. **布局约束**: 状态栏与现有 RightPanel 并存，需要调整 ThreadView 布局

### Viable Options

| Option | Approach | Pros | Cons |
|--------|----------|------|------|
| **A. 独立 StatusPanel** | 新建独立组件，与 RightPanel 并列显示 | 清晰分离，互不干扰 | 占用更多水平空间 |
| **B. 合并到 RightPanel** | 作为新 Tab 添加到现有 RightPanel | 复用现有布局逻辑 | 切换时看不到状态 |
| **C. 可折叠侧边栏** | 状态栏可折叠/展开 | 节省空间 | 增加交互复杂度 |

**Chosen: Option A** - 用户明确要求 ThreadView 右侧固定，与 RightPanel 并存。

---

## Requirements Summary

在 ISDP 的 ThreadView 页面右侧实现一个固定状态栏面板，实时展示：
1. **Agent 状态**: 当前活跃 Agent 列表、状态标识、调用时长
2. **Token 统计**: input/output tokens、缓存命中率、API 成本
3. **任务进度**: 每个 Agent 的任务列表和完成状态
4. **消息统计**: 总消息数、Agent 消息数、系统消息数

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

- [ ] **AC4: 消息统计面板**
  - 显示消息总数
  - 显示 Agent 消息数
  - 显示系统消息数
  - 显示用户消息数

---

## Implementation Steps

### Phase 1: 后端数据模型扩展 (预计 2h)

#### Step 1.1: 扩展 AgentInvocation 模型
**文件**: `isdp/internal/model/agent_invocation.go`

```go
// 新增字段
type AgentInvocation struct {
    // 现有字段...
    
    // Token 使用统计
    InputTokens         int64      `json:"inputTokens,omitempty"`
    OutputTokens        int64      `json:"outputTokens,omitempty"`
    CacheReadTokens     int64      `json:"cacheReadTokens,omitempty"`
    CacheCreationTokens int64      `json:"cacheCreationTokens,omitempty"`
    CostUsd             float64    `json:"costUsd,omitempty"`
    DurationMs          int64      `json:"durationMs,omitempty"`
    DurationApiMs       int64      `json:"durationApiMs,omitempty"`
    
    // 任务进度
    TaskProgress        *TaskProgress `json:"taskProgress,omitempty"`
}

type TaskProgress struct {
    SnapshotStatus string     `json:"snapshotStatus"` // completed/interrupted/running
    Tasks          []TaskItem `json:"tasks"`
}

type TaskItem struct {
    ID     string `json:"id"`
    Title  string `json:"title"`
    Status string `json:"status"` // completed/in_progress/pending
}
```

#### Step 1.2: 扩展 Chunk 类型
**文件**: `isdp/internal/service/agent/types.go`

```go
// 新增 ChunkType
const (
    // 现有...
    ChunkTypeUsage ChunkType = "usage"  // Token 使用更新
)

// 扩展 Chunk 结构
type Chunk struct {
    Type      ChunkType
    Content   string
    ToolName  string
    ToolInput map[string]interface{}
    // 新增
    Usage     *TokenUsage
}

type TokenUsage struct {
    InputTokens           int64
    OutputTokens          int64
    CacheReadTokens       int64
    CacheCreationTokens   int64
    CostUsd               float64
    DurationMs            int64
    DurationApiMs         int64
    NumTurns              int
}
```

#### Step 1.3: 解析 CLI usage 事件
**文件**: `isdp/internal/service/agent/claude_adapter.go`

修改 `parseStreamJSONLine` 方法，新增解析：

```go
// 在 parseStreamJSONLine 中新增 case
case "message_start":
    // 解析 message.usage 字段
    msg := msg.Message
    if msg.Usage != nil {
        // 提取 input_tokens, cache_read_input_tokens, cache_creation_input_tokens
    }
    
case "message_delta":
    // 解析 usage 字段（output_tokens 通常在这里）
    
case "result":
    // 解析完整 usage: input_tokens, output_tokens, cache_read_input_tokens
    // 解析 total_cost_usd, duration_ms, duration_api_ms, num_turns
```

#### Step 1.4: 数据库迁移
**文件**: `isdp/sql-change/migrations/YYYYMMDDNN_add_invocation_usage.sql`

```sql
ALTER TABLE agent_invocations 
ADD COLUMN input_tokens BIGINT DEFAULT 0,
ADD COLUMN output_tokens BIGINT DEFAULT 0,
ADD COLUMN cache_read_tokens BIGINT DEFAULT 0,
ADD COLUMN cache_creation_tokens BIGINT DEFAULT 0,
ADD COLUMN cost_usd DECIMAL(10,6) DEFAULT 0,
ADD COLUMN duration_ms BIGINT DEFAULT 0,
ADD COLUMN duration_api_ms BIGINT DEFAULT 0,
ADD COLUMN task_progress JSON DEFAULT NULL;
```

---

### Phase 2: WebSocket 推送扩展 (预计 1h)

#### Step 2.1: 新增 WebSocket 事件类型
**文件**: `isdp/internal/api/websocket.go`

```go
// 新增事件类型
const (
    // 现有...
    EventTypeUsageUpdate      = "usage_update"
    EventTypeTaskProgressUpdate = "task_progress_update"
)

type UsageUpdatePayload struct {
    InvocationID string      `json:"invocationId"`
    Usage        TokenUsage  `json:"usage"`
}

type TaskProgressUpdatePayload struct {
    InvocationID string       `json:"invocationId"`
    TaskProgress TaskProgress `json:"taskProgress"`
}
```

#### Step 2.2: 在执行服务中推送更新
**文件**: `isdp/internal/service/agent/execution_service.go`

在处理 Chunk 时，检测 usage 类型并推送：

```go
func (s *ExecutionService) handleChunk(invocationID string, chunk Chunk) {
    if chunk.Type == ChunkTypeUsage && chunk.Usage != nil {
        s.wsManager.Broadcast(WebSocketMessage{
            Type: "usage_update",
            Payload: UsageUpdatePayload{
                InvocationID: invocationID,
                Usage:        *chunk.Usage,
            },
        })
    }
}
```

---

### Phase 3: 前端 Store 扩展 (预计 1h)

#### Step 3.1: 扩展 AppState
**文件**: `isdp/web/src/store/index.ts`

```typescript
interface AppState {
  // 现有字段...
  
  // 新增：Agent usage 和 taskProgress
  agentUsage: Record<string, TokenUsage>;
  agentTaskProgress: Record<string, TaskProgress>;
}

interface TokenUsage {
  inputTokens?: number;
  outputTokens?: number;
  cacheReadTokens?: number;
  cacheCreationTokens?: number;
  costUsd?: number;
  durationApiMs?: number;
}

interface TaskProgress {
  snapshotStatus: 'completed' | 'interrupted' | 'running';
  tasks: TaskItem[];
}

interface TaskItem {
  id: string;
  title: string;
  status: 'completed' | 'in_progress' | 'pending';
}

// 新增 actions
interface AppActions {
  // 现有...
  updateAgentUsage: (invocationId: string, usage: TokenUsage) => void;
  updateAgentTaskProgress: (invocationId: string, progress: TaskProgress) => void;
}
```

#### Step 3.2: WebSocket 消息处理
**文件**: `isdp/web/src/pages/ThreadView.tsx`

在 `handleWsMessage` 中新增：

```typescript
case 'usage_update':
  updateAgentUsage(
    data.payload.invocationId,
    data.payload.usage
  );
  break;

case 'task_progress_update':
  updateAgentTaskProgress(
    data.payload.invocationId,
    data.payload.taskProgress
  );
  break;
```

---

### Phase 4: 前端组件实现 (预计 3h)

#### Step 4.1: 创建 StatusPanel 组件目录
**目录**: `isdp/web/src/components/thread/StatusPanel/`

```
StatusPanel/
├── index.tsx              # 主组件
├── StatusPanel.css        # 样式
├── AgentStatusCard.tsx    # Agent 状态卡片
├── TokenUsage.tsx         # Token 统计展示
├── TaskProgressPanel.tsx  # 任务进度面板
└── MessageStats.tsx       # 消息统计面板
```

#### Step 4.2: 主组件实现
**文件**: `isdp/web/src/components/thread/StatusPanel/index.tsx`

```tsx
import React from 'react';
import { Card, Tag, Spin, Progress, Statistic, Row, Col } from 'antd';
import { useAppStore } from '@/store';
import { AgentStatusCard } from './AgentStatusCard';
import { TokenUsage } from './TokenUsage';
import { TaskProgressPanel } from './TaskProgressPanel';
import { MessageStats } from './MessageStats';
import './StatusPanel.css';

interface StatusPanelProps {
  threadId: string;
  width?: number;
}

export const StatusPanel: React.FC<StatusPanelProps> = ({ threadId, width = 320 }) => {
  const { activeAgents, agentUsage, agentTaskProgress, messages } = useAppStore();
  
  // 计算消息统计
  const messageStats = {
    total: messages.length,
    agent: messages.filter(m => m.role === 'agent').length,
    system: messages.filter(m => m.role === 'system').length,
    user: messages.filter(m => m.role === 'user').length,
  };
  
  return (
    <aside className="status-panel" style={{ width }}>
      <div className="status-panel-header">
        <h3>状态栏</h3>
      </div>
      
      {/* Agent 状态 */}
      <AgentStatusCard 
        activeAgents={activeAgents}
        agentUsage={agentUsage}
      />
      
      {/* Token 统计 */}
      {Object.keys(agentUsage).length > 0 && (
        <TokenUsage usage={agentUsage} />
      )}
      
      {/* 任务进度 */}
      {Object.keys(agentTaskProgress).length > 0 && (
        <TaskProgressPanel progress={agentTaskProgress} />
      )}
      
      {/* 消息统计 */}
      <MessageStats stats={messageStats} />
    </aside>
  );
};
```

#### Step 4.3: AgentStatusCard 组件
**文件**: `isdp/web/src/components/thread/StatusPanel/AgentStatusCard.tsx`

```tsx
import React from 'react';
import { Card, Tag, Avatar, Badge } from 'antd';
import { RobotOutlined } from '@ant-design/icons';
import type { AgentInvocation, TokenUsage } from '@/types';

interface Props {
  activeAgents: AgentInvocation[];
  agentUsage: Record<string, TokenUsage>;
}

const statusColors = {
  pending: 'default',
  running: 'processing',
  streaming: 'processing',
  completed: 'success',
  failed: 'error',
};

export const AgentStatusCard: React.FC<Props> = ({ activeAgents, agentUsage }) => {
  return (
    <Card size="small" title="当前调用" className="status-card">
      {activeAgents.length === 0 ? (
        <div className="idle-status">空闲</div>
      ) : (
        <div className="agent-list">
          {activeAgents.map(agent => (
            <div key={agent.id} className="agent-item">
              <div className="agent-header">
                <Avatar size="small" icon={<RobotOutlined />} />
                <span className="agent-name">{agent.agentName || agent.id}</span>
                <Tag color={statusColors[agent.status]}>{agent.status}</Tag>
              </div>
              {agentUsage[agent.id] && (
                <div className="agent-usage">
                  <span>{agentUsage[agent.id].inputTokens}↓</span>
                  <span>{agentUsage[agent.id].outputTokens}↑</span>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
};
```

#### Step 4.4: TokenUsage 组件
**文件**: `isdp/web/src/components/thread/StatusPanel/TokenUsage.tsx`

```tsx
import React from 'react';
import { Card, Progress, Statistic, Row, Col } from 'antd';
import type { TokenUsage } from '@/types';

interface Props {
  usage: Record<string, TokenUsage>;
}

const formatTokens = (n: number): string => {
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return String(n);
};

export const TokenUsage: React.FC<Props> = ({ usage }) => {
  // 聚合所有 Agent 的 usage
  const total = Object.values(usage).reduce(
    (acc, u) => ({
      input: acc.input + (u.inputTokens || 0),
      output: acc.output + (u.outputTokens || 0),
      cache: acc.cache + (u.cacheReadTokens || 0),
      cost: acc.cost + (u.costUsd || 0),
    }),
    { input: 0, output: 0, cache: 0, cost: 0 }
  );
  
  const cacheRate = total.input > 0 
    ? Math.round((total.cache / total.input) * 100) 
    : 0;
  
  return (
    <Card size="small" title="Token 统计" className="status-card">
      <Row gutter={8}>
        <Col span={12}>
          <Statistic 
            title="输入" 
            value={formatTokens(total.input)}
            suffix="↓"
          />
        </Col>
        <Col span={12}>
          <Statistic 
            title="输出" 
            value={formatTokens(total.output)}
            suffix="↑"
          />
        </Col>
      </Row>
      
      {cacheRate > 0 && (
        <div className="cache-rate">
          <span>缓存命中率</span>
          <Progress percent={cacheRate} size="small" />
        </div>
      )}
      
      {total.cost > 0 && (
        <Statistic 
          title="成本" 
          value={`$${total.cost.toFixed(4)}`}
          valueStyle={{ color: '#faad14' }}
        />
      )}
    </Card>
  );
};
```

#### Step 4.5: TaskProgressPanel 组件
**文件**: `isdp/web/src/components/thread/StatusPanel/TaskProgressPanel.tsx`

```tsx
import React from 'react';
import { Card, List, Tag, Progress } from 'antd';
import { CheckCircleOutlined, ClockCircleOutlined, MinusCircleOutlined } from '@ant-design/icons';
import type { TaskProgress, TaskItem } from '@/types';

interface Props {
  progress: Record<string, TaskProgress>;
}

const TaskStatusIcon: React.FC<{ status: string }> = ({ status }) => {
  if (status === 'completed') return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
  if (status === 'in_progress') return <ClockCircleOutlined style={{ color: '#1890ff' }} />;
  return <MinusCircleOutlined style={{ color: '#d9d9d9' }} />;
};

export const TaskProgressPanel: React.FC<Props> = ({ progress }) => {
  return (
    <Card size="small" title="任务进度" className="status-card">
      {Object.entries(progress).map(([agentId, tp]) => {
        const completed = tp.tasks.filter(t => t.status === 'completed').length;
        const total = tp.tasks.length;
        
        return (
          <div key={agentId} className="task-group">
            <div className="task-header">
              <span>Agent: {agentId}</span>
              <Tag>{completed}/{total}</Tag>
            </div>
            <Progress 
              percent={Math.round((completed / total) * 100)} 
              size="small"
              status={tp.snapshotStatus === 'running' ? 'active' : undefined}
            />
            <List
              size="small"
              dataSource={tp.tasks}
              renderItem={(task: TaskItem) => (
                <List.Item className="task-item">
                  <TaskStatusIcon status={task.status} />
                  <span className="task-title">{task.title}</span>
                </List.Item>
              )}
            />
          </div>
        );
      })}
    </Card>
  );
};
```

#### Step 4.6: MessageStats 组件
**文件**: `isdp/web/src/components/thread/StatusPanel/MessageStats.tsx`

```tsx
import React from 'react';
import { Card, Descriptions } from 'antd';

interface Props {
  stats: {
    total: number;
    agent: number;
    system: number;
    user: number;
  };
}

export const MessageStats: React.FC<Props> = ({ stats }) => {
  return (
    <Card size="small" title="消息统计" className="status-card">
      <Descriptions column={1} size="small">
        <Descriptions.Item label="总数">{stats.total}</Descriptions.Item>
        <Descriptions.Item label="Agent消息">{stats.agent}</Descriptions.Item>
        <Descriptions.Item label="系统消息">{stats.system}</Descriptions.Item>
        <Descriptions.Item label="用户消息">{stats.user}</Descriptions.Item>
      </Descriptions>
    </Card>
  );
};
```

---

### Phase 5: ThreadView 布局集成 (预计 1h)

#### Step 5.1: 修改 ThreadView 布局
**文件**: `isdp/web/src/pages/ThreadView.tsx`

在非 Solo 模式下，添加 StatusPanel：

```tsx
import { StatusPanel } from '@/components/thread/StatusPanel';

// 在 JSX 中，消息区和右侧面板之间添加 StatusPanel
<div className="thread-content">
  {/* 消息区域 */}
  <div className="thread-messages">
    {/* 现有消息渲染 */}
  </div>
  
  {/* 状态栏 - 新增 */}
  {!soloMode && currentThread && (
    <StatusPanel threadId={currentThread.id} width={320} />
  )}
  
  {/* 现有 RightPanel */}
  {rightPanelVisible && (
    <RightPanel ... />
  )}
</div>
```

#### Step 5.2: 添加样式
**文件**: `isdp/web/src/pages/ThreadView.css`

```css
.thread-content {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.thread-messages {
  flex: 1;
  overflow-y: auto;
}

.status-panel {
  width: 320px;
  flex-shrink: 0;
  border-left: 1px solid var(--border-color);
  background: var(--bg-container);
  overflow-y: auto;
  padding: 12px;
}

.status-panel-header {
  margin-bottom: 12px;
}

.status-panel-header h3 {
  margin: 0;
  font-size: 14px;
}

.status-card {
  margin-bottom: 12px;
}

.status-card .ant-card-head {
  min-height: 32px;
  padding: 0 12px;
}

.status-card .ant-card-body {
  padding: 12px;
}

.agent-item {
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
}

.agent-item:last-child {
  border-bottom: none;
}

.agent-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.agent-name {
  flex: 1;
  font-size: 13px;
}

.agent-usage {
  display: flex;
  gap: 12px;
  margin-top: 4px;
  font-size: 12px;
  color: #666;
}

.task-item {
  padding: 4px 0;
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.task-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| CLI output format 变化 | Medium | High | 解析时做防御性检查，缺失字段时优雅降级 |
| WebSocket 连接断开 | Medium | Medium | 断线重连时重新同步状态，显示连接状态指示 |
| 性能影响（高频更新） | Low | Medium | 使用防抖/节流，批量更新 UI |
| 布局在小屏幕上拥挤 | Medium | Low | 响应式设计，小屏幕时可折叠 |

---

## Verification Steps

### 单元测试
1. **后端**: 测试 `parseStreamJSONLine` 对 message_start、message_delta、result 事件的解析
2. **后端**: 测试 TokenUsage 结构的正确序列化
3. **前端**: 测试 StatusPanel 组件渲染
4. **前端**: 测试 store 的 updateAgentUsage action

### 集成测试
1. 启动 Agent 执行，验证 WebSocket 收到 usage_update 事件
2. 验证前端 StatusPanel 正确显示 Token 统计
3. 验证消息统计与实际消息数量一致

### E2E 测试
1. 创建新 Thread，发送消息触发 Agent
2. 验证状态栏显示 Agent 状态为 "running"
3. 等待 Agent 完成，验证状态变为 "completed"
4. 验证 Token 统计数据正确显示

---

## File Change Summary

| File | Change Type | Description |
|------|-------------|-------------|
| `isdp/internal/model/agent_invocation.go` | Modify | 添加 Usage 和 TaskProgress 字段 |
| `isdp/internal/service/agent/types.go` | Modify | 添加 TokenUsage 类型，扩展 Chunk |
| `isdp/internal/service/agent/claude_adapter.go` | Modify | 解析 usage 事件 |
| `isdp/internal/api/websocket.go` | Modify | 添加 usage_update 事件类型 |
| `isdp/sql-change/migrations/*` | Create | 数据库迁移脚本 |
| `isdp/web/src/store/index.ts` | Modify | 添加 agentUsage, agentTaskProgress 状态 |
| `isdp/web/src/components/thread/StatusPanel/*` | Create | 新增 StatusPanel 组件 |
| `isdp/web/src/pages/ThreadView.tsx` | Modify | 集成 StatusPanel |
| `isdp/web/src/pages/ThreadView.css` | Modify | 添加状态栏样式 |
| `isdp/web/src/types/index.ts` | Modify | 添加 TokenUsage, TaskProgress 类型 |

---

## Estimated Timeline

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| Phase 1: 后端数据模型 | 2h | None |
| Phase 2: WebSocket 推送 | 1h | Phase 1 |
| Phase 3: 前端 Store | 1h | None |
| Phase 4: 前端组件 | 3h | Phase 3 |
| Phase 5: 布局集成 | 1h | Phase 4 |
| **Total** | **8h** | |

---

## ADR (Architecture Decision Record)

### Decision
使用独立的 StatusPanel 组件，与现有 RightPanel 并存显示。

### Drivers
1. 用户明确要求状态栏固定在 ThreadView 右侧
2. 实时性要求：状态信息需要始终可见，不应被 Tab 切换隐藏
3. 关注点分离：状态监控 vs 代码预览/沙箱是不同的功能域

### Alternatives Considered
1. **合并到 RightPanel 作为新 Tab**: 会隐藏状态信息，不满足实时性需求
2. **可折叠侧边栏**: 增加交互复杂度，用户未要求

### Why Chosen
独立 StatusPanel 确保 Agent 状态信息始终可见，符合用户"固定在右侧"的需求。

### Consequences
- 需要调整 ThreadView 布局以容纳两个右侧面板
- 在小屏幕上可能需要考虑响应式设计

### Follow-ups
- 监控实际使用情况，收集用户反馈
- 考虑未来添加可折叠/展开功能

---

## Changelog (Architect + Critic Improvements)

### Applied Improvements

1. **类型定义位置优化**: 新增 `isdp/web/src/types/status.ts` 存放 TokenUsage、TaskProgress 类型定义
2. **响应式布局**: 添加媒体查询说明，小屏幕（<1440px）时自动隐藏 StatusPanel 或提供折叠按钮
3. **TaskProgress 数据来源说明**: 标注为 Phase 2 迭代内容，本期仅实现 Agent 状态和 Token 统计
4. **Usage 持久化**: 在 AgentInvocation 完成时写入数据库，支持历史查询

### TaskProgress 数据来源说明

**本期实现范围**: Agent 状态 + Token 统计 + 消息统计

**后续迭代**: TaskProgress 任务进度面板

TaskProgress 数据来源选项：
- **Option A**: Agent 在输出中使用特定格式（如 `[TASK: id|title|status]`）标记任务
- **Option B**: 后端解析 Agent 输出，推断任务列表
- **Option C**: 通过 MCP 工具让 Agent 主动报告任务进度

建议在后续迭代中评估这些选项。

### 响应式布局设计

```css
/* 默认显示 StatusPanel */
.status-panel {
  display: block;
}

/* 小屏幕隐藏 StatusPanel */
@media (max-width: 1439px) {
  .status-panel {
    display: none;
  }
  
  /* 可选：显示展开按钮 */
  .status-panel-toggle {
    display: flex;
  }
}
```