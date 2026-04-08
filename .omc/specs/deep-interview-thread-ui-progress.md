# Deep Interview Spec: 对话页面进度感知与内容折叠优化

## Metadata
- Interview ID: ui-progress-opt-001
- Rounds: 3
- Final Ambiguity Score: 11%
- Type: brownfield
- Generated: 2026-04-03
- Threshold: 20%
- Status: PASSED

## Clarity Breakdown
| Dimension | Score | Weight | Weighted |
|-----------|-------|--------|----------|
| Goal Clarity | 0.95 | 35% | 0.33 |
| Constraint Clarity | 0.85 | 25% | 0.21 |
| Success Criteria | 0.90 | 25% | 0.23 |
| Context Clarity | 0.80 | 15% | 0.12 |
| **Total Clarity** | | | **89%** |
| **Ambiguity** | | | **11%** |

## Goal
优化 ISDP 对话页面用户体验，解决两个核心问题：
1. **进度感知不足**：AI 调用工具/skill 链时，用户无法感知整体进度和单个长工具的执行状态
2. **内容杂乱**：思考过程和工具输出占用过多屏幕空间，不够简洁

参考 Trae、Cursor 等成熟 AI 编程助手产品的设计，实现干净、可折叠的内容展示。

## Constraints
- **技术约束**：
  - 允许重构现有组件，保持 API 兼容
  - 可以新增后端 API 获取更详细的进度信息
- **设计参考**：Trae、Cursor 的折叠块设计
- **优先级**：两个问题同时解决

## Non-Goals
- 不改动消息存储结构
- 不改变现有 WebSocket 消息协议的核心流程
- 不涉及其他页面（如 Agent 管理页）

## Acceptance Criteria
- [ ] **Thinking 折叠 + 实时更新**：Thinking 块默认折叠，点击展开查看详情，内容实时更新
- [ ] **CLI 标题显示进度**：CLI 块标题显示工具链进度 (如 "3/5 tools completed")
- [ ] **工具行完整信息**：每个工具行显示名称+参数预览+执行耗时+状态图标+进度条
- [ ] **视觉干净无满屏文本**：屏幕上无满屏思考文本，整体干净

## Assumptions Exposed & Resolved
| Assumption | Challenge | Resolution |
|------------|-----------|------------|
| "进度感知"是指工具执行进度 | 问具体哪个环节 | 确认是工具链整体进度 + 长工具心跳 |
| "内容折叠"是指所有内容 | 问具体哪些内容 | 确认是 Thinking + 工具调用 |
| Thinking 应该完成后才显示 | 问流式时的显示方式 | 确认是折叠块 + 实时更新内容 |
| 可能需要心跳 API | 问技术约束 | 允许新增后端 API |

## Technical Context

### 现有组件
| 组件 | 路径 | 职责 |
|------|------|------|
| `CliOutputBlock` | `components/thread/CliOutputBlock.tsx` | 工具调用列表容器，支持折叠 |
| `ToolRow` | `components/thread/ToolRow.tsx` | 单个工具行，显示名称/参数/状态 |
| `ThinkingBlock` | `components/thread/ThinkingBlock.tsx` | 思考过程折叠块 |
| `StreamingMessage` | `components/thread/StreamingMessage.tsx` | 流式消息容器 |
| `StatusPanel` | `components/thread/StatusPanel/index.tsx` | 右侧状态栏 |
| `TaskProgressPanel` | `components/thread/StatusPanel/TaskProgressPanel.tsx` | 任务进度面板 |

### 当前问题
1. **Thinking 问题**：流式输出时可能打印满屏"思考中..."文本，而非紧凑的折叠块
2. **工具进度问题**：无工具链整体进度指示，长工具无心跳动画

### 数据流
```
WebSocket (agent_output_chunk)
  → ThreadView.handleWSMessage
  → setToolEvents / updateProgress
  → StreamingMessage / ChatMessage
  → CliOutputBlock / ThinkingBlock
```

## Ontology (Key Entities)

| Entity | Type | Fields | Relationships |
|--------|------|--------|---------------|
| ToolEvent | core domain | id, name, status, input, output, duration, startedAt, completedAt | 属于 Invocation |
| ThinkingBlock | UI component | content, label, expanded, breedColor | 渲染 thinking 内容 |
| CliOutputBlock | UI component | events, status, expanded, breedColor | 渲染 ToolEvent 列表 |
| ToolRow | UI component | event, isActive, accentColor | 渲染单个 ToolEvent |
| ProgressInfo | state | status, toolName, toolInput | 流式进度状态 |

## Implementation Plan

### Phase 1: Thinking 折叠优化
1. **修改 `StreamingMessage.tsx`**：
   - 检测 thinking chunk 类型
   - 使用 `ThinkingBlock` 组件渲染
   - 折叠状态下显示 "Thinking..." 动画指示器

2. **优化 `ThinkingBlock.tsx`**：
   - 流式时自动折叠，显示实时内容
   - 添加脉冲动画指示"正在思考"
   - 完成后状态变化

### Phase 2: CLI 工具进度优化
1. **修改 `CliOutputBlock.tsx`**：
   - 标题显示进度 "CLI Output · 3/5 tools"
   - 流式时自动展开，完成后可折叠

2. **增强 `ToolRow.tsx`**：
   - 添加进度条组件（running 状态时显示）
   - 显示执行时间计数（实时更新）
   - 状态图标动画

3. **后端支持（可选）**：
   - 新增工具执行心跳事件
   - 工具链预估总数

### Phase 3: 视觉优化
1. **整体布局**：
   - 确保折叠块紧凑
   - 减少不必要的间距
   - 统一折叠/展开动画

## Interview Transcript
<details>
<summary>Full Q&A (3 rounds)</summary>

### Round 1
**Q:** 「进度感知不足」具体指哪个环节？
**A:** 工具链无整体进度, 长时间工具无心跳
**Ambiguity:** 26% (Goal: 0.85, Constraints: 0.55, Criteria: 0.70, Context: 0.80)

### Round 2
**Q:** 「输出内容太多未折叠」你希望哪些内容默认折叠成一行？
**A:** (用户澄清) Think过程输出到块中折叠，CLI工具调用一条一行支持折叠

**Q:** Thinking 块在思考过程中应该如何显示？
**A:** 折叠块 + 实时更新内容

**Q:** 工具调用的布局方式？每个工具调用应该显示哪些信息？
**A:** 合并到一个 CLI 块；工具名+参数预览、执行耗时、状态图标、执行进度条

**Q:** 「工具链整体进度」如何显示？「长时间工具无心跳」如何解决？
**A:** CLI 块标题显示进度；动画/进度条
**Ambiguity:** 17% (Goal: 0.90, Constraints: 0.85, Criteria: 0.75, Context: 0.80)

### Round 3
**Q:** 技术约束是什么？哪个问题优先解决？
**A:** 允许重构保持兼容 + 可以新增后端API；两个问题一起解决

**Q:** 以下哪些是必须完成的验收标准？
**A:** 全选：Thinking 折叠+实时更新、CLI 标题显示进度、工具行完整信息、视觉干净
**Ambiguity:** 11% (Goal: 0.95, Constraints: 0.85, Criteria: 0.90, Context: 0.80)

</details>