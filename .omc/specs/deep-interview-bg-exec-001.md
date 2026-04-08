# Deep Interview Spec: Agent 后台执行与内容恢复

## Metadata
- Interview ID: bg-exec-001
- Rounds: 5
- Final Ambiguity Score: 15%
- Type: brownfield
- Generated: 2026-04-07
- Threshold: 20%
- Status: PASSED

## Clarity Breakdown
| Dimension | Score | Weight | Weighted |
|-----------|-------|--------|----------|
| Goal Clarity | 0.95 | 0.35 | 0.3325 |
| Constraint Clarity | 0.85 | 0.25 | 0.2125 |
| Success Criteria | 0.85 | 0.25 | 0.2125 |
| Context Clarity | 0.70 | 0.15 | 0.1050 |
| **Total Clarity** | | | **0.8625** |
| **Ambiguity** | | | **13.75%** |

## Goal
实现 Agent 后台执行能力，确保用户离开对话页面后再返回时，能够完整恢复 Agent 的输出内容（包括 thinking、CLI output、tool_use 等所有内容块）。

## Constraints
- 必须实时增量持久化内容块（满足后端重启恢复验收标准）
- 所有 Agent 都支持后台执行（多 Agent 并发场景）
- 失败时保留已累积的内容块（失败现场可追溯）
- 前端重连时无缝展示断开期间的内容

## Non-Goals
- 不实现 Agent 暂停/恢复功能（继续执行而非暂停等待）
- 不实现跨设备同步（同一用户多端登录场景）

## Acceptance Criteria
- [ ] **基本场景**：离开页面 5 分钟，返回后能看到断开期间的全部内容块
- [ ] **长时间执行**：Agent 执行 10 分钟，用户在第 5 分钟离开，第 9 分钟返回，内容完整恢复
- [ ] **失败保留**：Agent 执行失败，返回页面能看到失败前的 thinking 和 tool_use
- [ ] **后端重启恢复**：Agent 执行过程中后端重启，重启后用户返回能看到已累积的内容
- [ ] **多 Agent 并发**：多个 Agent 同时执行时，各自内容完整恢复

## 问题根因分析

### 当前架构的数据流

```
┌─────────────────────────────────────────────────────────────────┐
│  用户消息 → 后端创建 Invocation → Agent CLI 执行 → 流式输出      │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│  后端 RunningAgent 结构                                          │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │ AccumulatedContentBlocks []ContentBlockData (内存)          │ │
│  │ ↓ 完成时才持久化 ↓                                           │ │
│  │ saveAgentMessageWithReturn() → messages.content_blocks      │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│  WebSocket 推送                                                  │
│  agent_output_chunk → 前端 streamingContentBlocks (Zustand)     │
│  agent_message → 前端 messages (持久化到 DB 后)                 │
└─────────────────────────────────────────────────────────────────┘
```

### 丢失原因分析

| 问题点 | 当前状态 | 导致的丢失 | 影响场景 |
|--------|----------|------------|----------|
| **后端累积** | 内存中，完成后才持久化 | Agent 未完成时退出 → 累积内容全丢失 | 离开页面、后端重启 |
| **前端状态** | `loadThread()` 清空流式状态 | 返回页面 → 看不到断开期间的内容 | 离开页面后返回 |
| **WebSocket** | 重连不推送历史 | 重连成功 → 只能收到新内容 | 短暂断网、页面切换 |

### Clowder-ai 参考设计

| 模式 | 实现 | 适用场景 |
|------|------|----------|
| **RichBlockBuffer** | 内存缓冲 + TTL 15min | MCP 回调创建的 rich blocks |
| **TaskProgressStore** | 内存/Redis 持久化 | 任务进度快照 |
| **StartupReconciler** | 启动扫描孤儿调用 | 后端重启恢复 |
| **requestStreamCatchUp** | 前端请求历史恢复 | 流事件丢失 |
| **done_timeout** | 5分钟超时 + 后台通知 | 长时间执行 |

## 技术方案

### 方案概述

采用 **实时增量持久化 + WebSocket 重连恢复** 的混合方案：

1. **后端**：每收到一个 content block 就增量持久化到数据库
2. **前端**：WebSocket 重连时请求恢复未完成 invocation 的累积内容
3. **启动恢复**：后端启动时扫描 running 状态的 invocation，恢复其累积内容

### 详细设计

#### 1. 后端：增量持久化 ContentBlock

**修改 `execution_service.go` 的 `broadcastChunk` 函数：**

```go
// broadcastChunk 中，每个内容块到达时立即持久化
func (s *ExecutionService) broadcastChunk(...) {
    // 1. 累积到内存（保持不变）
    runningAgent.ContentBlocksMu.Lock()
    runningAgent.AccumulatedContentBlocks = append(runningAgent.AccumulatedContentBlocks, blockData)
    runningAgent.ContentBlocksMu.Unlock()

    // 2. 【新增】增量持久化到数据库
    s.persistContentBlock(invocationID, blockData)

    // 3. WebSocket 广播（保持不变）
    s.wsHub.BroadcastToThread(...)
}
```

**新增 `persistContentBlock` 函数：**

```go
// persistContentBlock 增量持久化单个内容块
func (s *ExecutionService) persistContentBlock(invocationID uuid.UUID, block ContentBlockData) error {
    // 方案A：追加到 invocation 记录的 content_blocks 字段
    // 方案B：写入独立的 content_blocks 表（推荐，支持增量更新）

    // 使用 UPSERT 语义，支持重试和去重
    return s.contentBlockRepo.Upsert(invocationID, block)
}
```

#### 2. 数据库：新增 content_blocks 表

```sql
CREATE TABLE invocation_content_blocks (
    id VARCHAR(36) PRIMARY KEY,
    invocation_id VARCHAR(36) NOT NULL,
    type VARCHAR(20) NOT NULL,  -- thinking, text, tool_use, tool_result
    content TEXT,
    tool_name VARCHAR(100),
    tool_id VARCHAR(36),
    input JSON,
    output TEXT,
    is_error BOOLEAN DEFAULT FALSE,
    status VARCHAR(20),  -- streaming, completed
    timestamp BIGINT NOT NULL,
    started_at BIGINT,
    completed_at BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_invocation_id (invocation_id),
    INDEX idx_timestamp (timestamp)
);
```

#### 3. 前端：WebSocket 重连恢复

**修改 `useWebSocket.ts`：**

```typescript
// WebSocket 重连时请求恢复
ws.onopen = () => {
  setConnected(true);
  onConnectRef.current?.();

  // 【新增】请求恢复未完成的 invocation 内容
  if (threadId) {
    send({
      type: 'recover_invocation_state',
      threadId,
    });
  }
};
```

#### 4. 后端：处理恢复请求

**新增 WebSocket 消息类型处理：**

```go
case "recover_invocation_state":
    threadID := msg.ThreadID
    // 查找该 thread 下所有 running 状态的 invocation
    invocations := s.invocationRepo.FindRunningByThread(threadID)
    for _, inv := range invocations {
        // 从数据库读取已持久化的 content_blocks
        blocks := s.contentBlockRepo.FindByInvocation(inv.ID)
        // 推送给前端
        s.wsHub.BroadcastToThread(threadID, WSMessage{
            Type: "invocation_recovery",
            Payload: map[string]interface{}{
                "invocationId": inv.ID,
                "contentBlocks": blocks,
                "status": inv.Status,
            },
        })
    }
```

#### 5. 前端：处理恢复消息

**修改 `ThreadView.tsx` 的 WebSocket 消息处理：**

```typescript
case 'invocation_recovery':
    const { invocationId, contentBlocks, status } = msg.payload;
    // 恢复流式状态
    if (status === 'running') {
        setStreaming(true);
        setStreamingInvocationId(invocationId);
    }
    // 设置内容块
    setStreamingContentBlocks(contentBlocks);
    break;
```

#### 6. 启动恢复（参考 clowder-ai StartupReconciler）

**新增 `startup_reconciler.go`：**

```go
// OnStartup 后端启动时扫描孤儿 invocation
func (s *ExecutionService) OnStartup() {
    // 查找所有 running 状态的 invocation
    running := s.invocationRepo.FindAllRunning()
    for _, inv := range running {
        // 检查进程是否还在运行
        if !s.isProcessAlive(inv.ProcessID) {
            // 标记为中断
            s.invocationRepo.UpdateStatus(inv.ID, "interrupted")
            // 内容块已持久化，用户返回时可以看到
        }
    }
}
```

## 实现优先级

| 优先级 | 任务 | 工作量 |
|--------|------|--------|
| P0 | 增量持久化 content_blocks | 2天 |
| P1 | WebSocket 重连恢复 | 1天 |
| P2 | 前端恢复消息处理 | 0.5天 |
| P3 | 启动恢复扫描 | 0.5天 |

## Ontology (Key Entities)

| Entity | Type | Fields | Relationships |
|--------|------|--------|---------------|
| Invocation | core domain | threadId, agentId, status, contentBlocks | belongs to Thread, has many ContentBlocks |
| ContentBlock | core domain | id, type, content, status, timestamp | belongs to Invocation |
| Thread | core domain | id, status, invocations | has many Invocations, Messages |
| WebSocket | infrastructure | id, status, connected | connects Thread to Backend |

## 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| 增量持久化增加数据库压力 | 批量写入 + 节流（每 500ms 或每 10 个块） |
| 内容块重复 | 使用 UPSERT 语义，按 id 去重 |
| 大量内容块影响查询 | 分页加载 + 归档旧块 |