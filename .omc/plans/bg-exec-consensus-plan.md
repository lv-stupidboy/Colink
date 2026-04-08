# Agent 后台执行实施计划

## RALPLAN-DR Summary

### Principles
1. **增量持久化优先**：每个内容块到达时立即持久化，不等待完成
2. **幂等恢复**：WebSocket 重连可多次请求恢复，结果一致
3. **失败保留现场**：失败/中断的 invocation 保留已累积内容
4. **最小侵入**：复用现有 `ContentBlockData` 结构，减少改动

### Decision Drivers
1. **后端重启恢复**是硬性要求 → 必须持久化到数据库
2. **增量持久化**增加数据库压力 → 需要节流策略
3. **前端状态管理**复杂度 → 复用现有 Zustand store

### Viable Options

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A. 独立 content_blocks 表** | 支持增量更新、查询灵活 | 新增表和 repo |
| **B. 追加到 invocation.content_blocks** | 无需新表 | 大 JSON 更新效率低 |
| **C. Redis 缓冲 + 异步持久化** | 高性能、低延迟 | 复杂度高、Redis 依赖 |

**选择方案 A**：独立表，支持增量更新，查询效率高。

---

## 实施步骤

### Phase 1: 后端增量持久化 (P0)

#### Step 1.1: 数据库迁移
**文件**: `isdp/sql-change/migrations/202604070001_add_invocation_content_blocks.sql`

```sql
-- 1. 添加 process_id 字段用于进程追踪
ALTER TABLE agent_invocations ADD COLUMN process_id VARCHAR(36) DEFAULT NULL;

-- 2. 创建内容块表
CREATE TABLE invocation_content_blocks (
    id VARCHAR(36) PRIMARY KEY,
    invocation_id VARCHAR(36) NOT NULL,
    type VARCHAR(20) NOT NULL,
    content TEXT,
    tool_name VARCHAR(100),
    tool_id VARCHAR(36),
    input JSON,
    output TEXT,
    is_error BOOLEAN DEFAULT FALSE,
    status VARCHAR(20),
    timestamp BIGINT NOT NULL,
    started_at BIGINT,
    completed_at BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_invocation_id (invocation_id),
    INDEX idx_timestamp (timestamp),
    FOREIGN KEY (invocation_id) REFERENCES agent_invocations(id) ON DELETE CASCADE
);
```

#### Step 1.2: 新增 Model 和 Repo
**文件**: `isdp/internal/model/content_block.go`

```go
package model

import "time"

type InvocationContentBlock struct {
    ID            string                 `json:"id"`
    InvocationID  string                 `json:"invocationId"`
    Type          string                 `json:"type"`
    Content       string                 `json:"content,omitempty"`
    ToolName      string                 `json:"toolName,omitempty"`
    ToolID        string                 `json:"toolId,omitempty"`
    Input         map[string]interface{} `json:"input,omitempty"`
    Output        string                 `json:"output,omitempty"`
    IsError       bool                   `json:"isError,omitempty"`
    Status        string                 `json:"status,omitempty"`
    Timestamp     int64                  `json:"timestamp"`
    StartedAt     int64                  `json:"startedAt,omitempty"`
    CompletedAt   int64                  `json:"completedAt,omitempty"`
    CreatedAt     time.Time              `json:"createdAt"`
}
```

**文件**: `isdp/internal/repo/content_block.go`

```go
package repo

type ContentBlockRepository struct {
    db *gorm.DB
}

func (r *ContentBlockRepository) Upsert(block *model.InvocationContentBlock) error
func (r *ContentBlockRepository) FindByInvocation(invocationID string) ([]model.InvocationContentBlock, error)
func (r *ContentBlockRepository) DeleteByInvocation(invocationID string) error
func (r *ContentBlockRepository) BatchUpsert(blocks []model.InvocationContentBlock) error
```

#### Step 1.3: 修改 execution_service.go
**文件**: `isdp/internal/service/agent/execution_service.go`

**改动点**:
1. 注入 `ContentBlockRepository`
2. 在 `broadcastChunk` 中添加增量持久化逻辑
3. 添加节流机制（每 500ms 或每 10 个块批量写入）

```go
// 在 broadcastChunk 中添加
func (s *ExecutionService) broadcastChunk(...) {
    // ... 现有逻辑 ...

    // 增量持久化（节流）
    s.contentBlockBuffer = append(s.contentBlockBuffer, blockData)
    if len(s.contentBlockBuffer) >= 10 || time.Since(s.lastFlush) > 500*time.Millisecond {
        s.flushContentBlocks(invocationID)
    }
}

func (s *ExecutionService) flushContentBlocks(invocationID uuid.UUID) {
    if len(s.contentBlockBuffer) == 0 {
        return
    }
    blocks := s.contentBlockBuffer
    s.contentBlockBuffer = nil
    s.lastFlush = time.Now()

    // 同步写入确保数据安全
    if err := s.contentBlockRepo.BatchUpsert(blocks); err != nil {
        log.Error("failed to persist content blocks", zap.Error(err))
    }
}
```

#### Step 1.4: Agent 执行退出时确保刷新
**文件**: `isdp/internal/service/agent/execution_service.go`

```go
func (s *ExecutionService) executeAgent(ctx context.Context, invocation *model.AgentInvocation, ...) {
    // 在函数开始时注册 defer 刷新
    defer func() {
        s.flushContentBlocks(invocation.ID)
    }()

    // ... 现有执行逻辑 ...
}
```

---

### Phase 2: WebSocket 重连恢复 (P1)

#### Step 2.1: 后端处理恢复请求
**文件**: `isdp/internal/ws/handler.go`

```go
case "recover_invocation_state":
    threadID := msg.ThreadID
    invocations := s.orchestrator.GetRunningInvocationsByThread(threadID)

    for _, inv := range invocations {
        blocks := s.contentBlockRepo.FindByInvocation(inv.ID.String())
        s.hub.BroadcastToThread(threadID, WSMessage{
            Type: "invocation_recovery",
            Payload: map[string]interface{}{
                "invocationId": inv.ID.String(),
                "contentBlocks": blocks,
                "status": inv.Status,
                "agentId": inv.AgentConfigID.String(),
                "agentName": inv.Role,
            },
        })
    }
```

#### Step 2.2: 前端请求恢复
**文件**: `isdp/web/src/hooks/useWebSocket.ts`

```typescript
ws.onopen = () => {
  setConnected(true);
  onConnectRef.current?.();

  // 请求恢复未完成的 invocation
  if (threadId) {
    send({
      type: 'recover_invocation_state',
      threadId,
    });
  }
};
```

#### Step 2.3: 前端处理恢复消息（含去重）
**文件**: `isdp/web/src/store/index.ts`

```typescript
// 新增 action（含去重逻辑）
recoverInvocationState: (invocationId: string, contentBlocks: MessageContentBlock[], status: string) => {
  set((state) => {
    if (status === 'running') {
      // 去重：合并现有块和恢复块，按 id 去重
      const existingIds = new Set(state.streamingContentBlocks.map(b => b.id));
      const newBlocks = contentBlocks.filter(b => !existingIds.has(b.id));
      const mergedBlocks = [...state.streamingContentBlocks, ...newBlocks];

      return {
        isStreaming: true,
        streamingInvocationId: invocationId,
        streamingContentBlocks: mergedBlocks,
      };
    }
    return state;
  });
},
```

**文件**: `isdp/web/src/pages/ThreadView.tsx`

```typescript
case 'invocation_recovery':
  const { invocationId, contentBlocks, status, agentId, agentName } = msg.payload;
  recoverInvocationState(invocationId, contentBlocks, status);
  if (status === 'running') {
    updateAgentStatus(invocationId, 'running', agentName);
  }
  break;
```

---

### Phase 3: 启动恢复 (P3)

#### Step 3.1: 启动扫描
**文件**: `isdp/internal/service/agent/startup_reconciler.go`

```go
package agent

type StartupReconciler struct {
    invocationRepo *repo.InvocationRepository
    contentBlockRepo *repo.ContentBlockRepository
}

func (r *StartupReconciler) Reconcile() {
    running := r.invocationRepo.FindByStatus("running")
    for _, inv := range running {
        // 检查进程是否存活
        if !r.isProcessAlive(inv.ProcessID) {
            r.invocationRepo.UpdateStatus(inv.ID, "interrupted")
        }
    }
}
```

#### Step 3.2: 服务启动时调用
**文件**: `isdp/cmd/server/main.go`

```go
func main() {
    // ... 初始化 ...

    // 启动恢复
    reconciler := agent.NewStartupReconciler(invocationRepo, contentBlockRepo)
    reconciler.Reconcile()

    // ... 启动服务 ...
}
```

---

## 测试计划

### 单元测试

| 测试文件 | 测试内容 |
|----------|----------|
| `content_block_repo_test.go` | Upsert、FindByInvocation、BatchUpsert |
| `execution_service_test.go` | 增量持久化、节流逻辑 |
| `startup_reconciler_test.go` | 孤儿 invocation 检测 |

### 集成测试

1. **基本场景测试**:
   - 启动 Agent
   - 发送消息触发 thinking + tool_use
   - 离开页面
   - 返回页面，验证内容恢复

2. **后端重启测试**:
   - 启动 Agent
   - 在执行过程中重启后端
   - 返回页面，验证已持久化内容可见

### 手动验证步骤

```
1. 创建对话，发送消息触发 Agent 执行
2. 等待 thinking 内容出现
3. 导航到其他页面（如项目列表）
4. 等待 30 秒
5. 返回对话页面
6. 验证：thinking 内容完整显示
```

---

## 风险缓解

| 风险 | 缓解措施 | 负责人 |
|------|----------|--------|
| 增量持久化增加数据库压力 | 节流（500ms/10块）+ 批量 INSERT | 后端 |
| 内容块重复 | UPSERT 语义，按 id 去重 | 后端 |
| 大量内容块影响查询 | 复合索引 + 分页加载 | 后端 |
| WebSocket 重连风暴 | 防抖，最多 3 次重试 | 前端 |
| 进程异常退出丢失缓冲 | defer 中强制同步刷新 | 后端 |
| 前端恢复消息重复 | 按 block.id 去重合并 | 前端 |
| 长期数据量增长 | 按时间分区或归档旧数据 | 后端 |

## 性能预估

| 指标 | 预估值 | MySQL 能力 | 结论 |
|------|--------|------------|------|
| 单 Agent 写入 QPS | < 5 | 1000-5000 | ✅ 无瓶颈 |
| 多 Agent 并发写入 QPS | < 100 | 1000-5000 | ✅ 无瓶颈 |
| 批量写入延迟 | < 50ms | - | ✅ 可接受 |

---

## 实施顺序

1. **Week 1**: Phase 1 (P0) - 后端增量持久化
2. **Week 2**: Phase 2 (P1) - WebSocket 重连恢复
3. **Week 3**: Phase 3 (P3) - 启动恢复 + 测试

## ADR

**Decision**: 使用独立 `invocation_content_blocks` 表存储内容块

**Drivers**:
- 后端重启恢复要求持久化
- 增量更新效率
- 查询灵活性

**Alternatives Considered**:
- 追加到 invocation.content_blocks：大 JSON 更新效率低
- Redis 缓冲：复杂度高，增加依赖

**Why Chosen**: 独立表支持增量更新，查询效率高，无额外依赖

**Consequences**:
- 新增数据库表和 repo
- 需要迁移脚本
- 查询时需要 JOIN

**Follow-ups**:
- 考虑未来归档旧内容块
- 监控表大小，适时分表