# Deep Interview Spec: Chat UX Improvements

## Metadata
- Interview ID: di-chat-ux-001
- Rounds: 3
- Final Ambiguity Score: 11.2%
- Type: brownfield
- Generated: 2026-04-02
- Threshold: 0.2
- Status: PASSED

## Clarity Breakdown
| Dimension | Score | Weight | Weighted |
|-----------|-------|--------|----------|
| Goal Clarity | 0.90 | 0.35 | 0.315 |
| Constraint Clarity | 0.85 | 0.25 | 0.213 |
| Success Criteria Clarity | 0.90 | 0.25 | 0.225 |
| Context Clarity | 0.90 | 0.15 | 0.135 |
| **Total Clarity** | | | **88.8%** |
| **Ambiguity** | | | **11.2%** |

## Goal
优化 ISDP 对话框的用户体验，参考 clowder-ai 实现：
1. CLI 输出块可折叠，streaming 结束后自动收起
2. 完整工具调用列表（名称、状态、耗时、详情）
3. 消息悬停工具栏（删除、分支操作）

## Constraints
- 复用现有 WebSocket 数据流（agent_output_chunk 已传递工具事件）
- 保持与现有 ChatMessage/ChatMessageList 组件的兼容性
- 参考 clowder-ai 的设计模式，但适配 ISDP 的数据结构

## Non-Goals
- A2A 折叠（ISDP 当前无 A2A 链概念）
- 编辑消息功能（分支后在新线程中编辑）

## Acceptance Criteria
- [ ] CLI 输出块组件：显示工具列表，streaming 结束后自动收起
- [ ] 工具列表：展示工具名称、状态图标、耗时、可展开查看详情
- [ ] 悬停工具栏：显示删除/分支按钮，分支创建新对话
- [ ] 自动收起逻辑：用户未交互时，streaming 结束后自动折叠

## Assumptions Exposed & Resolved
| Assumption | Challenge | Resolution |
|------------|-----------|------------|
| 工具调用需要后端新增接口 | 检查 WebSocket 数据流 | 已有 agent_output_chunk 传递 toolName/toolInput |
| 需要全部消息可折叠 | 追问收起范围 | 用户选择只对 CLI 输出块（工具调用区域）折叠 |

## Technical Context

### ISDP 现有数据流
```
WebSocket agent_output_chunk
  ├─ type: 'tool_use' | 'text' | 'thinking'
  ├─ toolName: string
  ├─ toolInput: Record<string, unknown>
  └─ invocationId: string

Store:
  progressState[invocationId] = {
    status: 'thinking' | 'tool_use' | 'generating' | 'idle',
    toolName?: string,
    toolInput?: Record<string, unknown>
  }

  streamingMessages[invocationId] = {
    content: string,
    agentId: string,
    agentName?: string
  }
```

### 需要扩展的数据结构
```typescript
// 新增 ToolEvent 类型
interface ToolEvent {
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

// 扩展 Store
toolEvents: Record<string, ToolEvent[]>  // invocationId -> ToolEvent[]
```

### clowder-ai 参考模式
1. **自动收起**: `prevStatus === 'streaming' && status !== 'streaming' && !userInteracted`
2. **悬停工具栏**: CSS `opacity-0 group-hover:opacity-100`
3. **工具行**: `[status icon] [wrench] [name] [detail] [result]`

## Ontology (Key Entities)

| Entity | Type | Fields | Relationships |
|--------|------|--------|---------------|
| CliOutputBlock | core domain | status, toolName, toolInput, duration, expanded | contains ToolRow |
| ToolRow | core domain | name, status, detail, duration | belongs to CliOutputBlock |
| MessageActions | supporting | message, threadId, toolbar buttons | wraps ChatMessage |
| ToolEventStore | supporting | id, agentId, events, status | feeds CliOutputBlock |

## Ontology Convergence

| Round | Entity Count | New | Changed | Stable | Stability Ratio |
|-------|-------------|-----|---------|--------|----------------|
| 1 | 3 | 3 | - | - | - |
| 2 | 4 | 1 | 0 | 3 | 100% |
| 3 | 4 | 0 | 0 | 4 | 100% |

## Interview Transcript
<details>
<summary>Full Q&A (3 rounds)</summary>

### Round 1
**Q:** 自动收起功能应该应用在什么范围？
**A:** CLI 输出块（推荐）
**Ambiguity:** 100% (初始状态)

### Round 2
**Q:** 是否需要像 clowder-ai 一样，显示完整的工具调用列表？
**A:** 完整工具列表（推荐）
**Ambiguity:** 估算 ~30%

### Round 3
**Q:** 是否需要添加悬停工具栏？
**A:** 需要（参考 clowder-ai）
**Ambiguity:** 11.2% (达标)
</details>