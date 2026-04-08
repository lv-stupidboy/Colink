# Implementation Plan: 对话页面进度感知与内容折叠优化

## ✅ IMPLEMENTATION COMPLETE

**所有验收标准已通过，构建成功。**

## RALPLAN-DR Summary

### Principles
1. **最小视觉干扰**：默认折叠，用户按需展开
2. **实时进度反馈**：工具执行时显示动画/进度条，消除"卡住"感知
3. **信息密度平衡**：一行显示核心信息，详情隐藏在折叠内
4. **渐进增强**：先纯前端优化，后端支持作为增强

### Decision Drivers
1. 用户反馈"满屏思考文本"影响体验
2. 长工具执行时无心跳，用户不确定是否正常执行
3. 工具链无整体进度，用户无法预估完成时间
4. 参考 Trae/Cursor 的成熟设计模式

### Viable Options

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **A: 纯前端优化** | 只改组件，无后端改动 | 改动范围小，风险低 | 无法获取真实工具进度，只能显示时间 |
| **B: 前端+后端心跳** | 新增工具执行心跳事件 | 可显示真实进度 | 需改后端，Claude CLI 未必支持 |
| **C: 估算进度** | 根据工具类型估算进度 | 无需后端改动 | 不够准确 |

**Chosen: A + C**：先纯前端优化（低风险），用时间计数+动画代替真实进度，后续可扩展后端支持。

---

## Implementation Steps

### Phase 1: Thinking 折叠优化 (优先级: 高)

**目标**：Thinking 内容默认折叠，实时更新，消除满屏文本

**Architect 发现的关键问题**：
- `thinking` chunk 已存在于 ThreadView.tsx:568，但只调用 `updateProgress(invocId, 'thinking')`
- Store 没有 `streamingThinkingContent` 字段来存储思考内容
- 思考内容没有传递到 ThinkingBlock 组件

#### 1.1 新增 Store 状态 (store/index.ts)
```typescript
// 新增状态
streamingThinkingContent: string | null;

// 新增 action
appendThinkingChunk: (chunk: string, invocationId: string) => void;
clearThinkingContent: () => void;
```

#### 1.2 修改 ThreadView.tsx thinking chunk 处理
```typescript
if (chunkType === 'thinking') {
  const thinkingText = data.payload.chunk as string;
  if (thinkingText) {
    appendThinkingChunk(thinkingText, invocId);
  }
  updateProgress(invocId, 'thinking');
}
```

#### 1.3 修改 StreamingMessage.tsx
- 订阅 `streamingThinkingContent` 状态
- 传递到 ChatMessage 的 `progress.thinkingText`

#### 1.4 增强 ThinkingBlock.tsx
- 流式状态：显示 "Thinking..." 标题 + 脉冲动画
- 内容实时更新：折叠状态下也更新内容
- 完成状态：标题变为 "Thought" (过去时)，动画停止
- 预览文本：折叠时显示前 60 字符预览

**Files to modify:**
- `isdp/web/src/store/index.ts` - 新增 streamingThinkingContent
- `isdp/web/src/pages/ThreadView.tsx` - 处理 thinking chunk 内容
- `isdp/web/src/components/thread/StreamingMessage.tsx` - 传递 thinking 内容
- `isdp/web/src/components/thread/ThinkingBlock.tsx` - 流式样式

### Phase 2: CLI 工具进度优化 (优先级: 高)

**目标**：CLI 块标题显示进度，工具行显示完整信息+动画

#### 2.1 增强 CliOutputBlock.tsx
- **标题进度**：`CLI Output · 3/5 tools · streaming`
- **状态指示**：streaming 时自动展开，done 后保持展开
- **工具计数**：显示 completed/total

#### 2.2 增强 ToolRow.tsx
- **进度条**：running 状态时显示骨架屏/脉冲动画
- **时间计数**：实时更新执行时间 (如 "2.3s")
- **状态图标动画**：running 时 LoadingOutlined 旋转
- **完成时**：显示总耗时

**Files to modify:**
- `isdp/web/src/components/thread/CliOutputBlock.tsx`
- `isdp/web/src/components/thread/ToolRow.tsx`

### Phase 3: 视觉优化 (优先级: 中)

**目标**：整体干净，间距合理

#### 3.1 CSS 优化
- 统一折叠块圆角 (8px)
- 统一内边距 (8px 12px)
- 折叠/展开动画过渡 (150ms)

#### 3.2 布局优化
- CLI 块与 Thinking 块间距统一
- 消息气泡内边距优化

**Files to modify:**
- `isdp/web/src/components/thread/CliOutputBlock.css`
- `isdp/web/src/components/thread/ThinkingBlock.tsx` (内联样式)

### Phase 4: 后端增强 (可选，优先级: 低)

**目标**：提供更精确的工具进度

#### 4.1 新增 WebSocket 事件
- `tool_progress`: `{ toolId, progress: 0-100, message }`
- `tool_chain_progress`: `{ completed, total }`

#### 4.2 Claude Adapter 改造
- 解析 CLI 输出中的进度信息
- 发送进度事件

**Files to modify:**
- `isdp/internal/service/agent/claude_adapter.go`
- `isdp/internal/service/agent/execution_service.go`

---

## Acceptance Criteria Checklist

| Criteria | Implementation | Phase |
|----------|---------------|-------|
| Thinking 折叠 + 实时更新 | ThinkingBlock 增强 | 1 |
| CLI 标题显示进度 | CliOutputBlock 增强 | 2 |
| 工具行完整信息 | ToolRow 增强 | 2 |
| 视觉干净无满屏文本 | CSS + 布局优化 | 3 |

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Thinking 内容过长影响性能 | 截断预览，折叠时只渲染预览 |
| 工具执行时间计数不准确 | 使用 startedAt 时间戳计算，避免 setInterval |
| 动画影响低端设备性能 | CSS 动画优先，减少 JS 动画 |

---

## Test Plan

### Manual Testing
1. 触发 Agent 执行，观察 Thinking 块是否折叠显示
2. 展开折叠，确认内容实时更新
3. 观察工具行是否显示进度动画和时间计数
4. 确认 CLI 块标题显示进度

### Integration Testing
- 确认 WebSocket 消息正常处理
- 确认组件渲染无报错

---

## Estimated Effort

| Phase | Effort | Risk |
|-------|--------|------|
| Phase 1 | 2h | Low |
| Phase 2 | 2h | Low |
| Phase 3 | 1h | Low |
| Phase 4 | 3h | Medium (optional) |

**Total: 5h (without Phase 4)**