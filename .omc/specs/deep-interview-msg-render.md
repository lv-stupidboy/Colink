# Deep Interview Spec: ISDP 消息渲染优化

## Metadata
- Interview ID: msg-render-compare-001
- Rounds: 2
- Final Ambiguity Score: 12.75%
- Type: brownfield
- Generated: 2026-04-02
- Threshold: 20%
- Status: PASSED

## Clarity Breakdown
| Dimension | Score | Weight | Weighted |
|-----------|-------|--------|----------|
| Goal Clarity | 90% | 35% | 31.5% |
| Constraint Clarity | 80% | 25% | 20% |
| Success Criteria | 95% | 25% | 23.75% |
| Context Clarity | 80% | 15% | 12% |
| **Total Clarity** | | | **87.25%** |
| **Ambiguity** | | | **12.75%** |

## Goal
参考 clowder-ai 的实现，优化 ISDP 的消息渲染组件，提升用户体验：
1. 实时显示 Agent 工具调用状态（正在执行什么命令）
2. 思考内容以深色终端风格展示
3. 代码块渲染优化（复制按钮、语言标签、语法高亮）
4. 工具调用显示主要参数

## Constraints
- **必须保留**：A2A 路由逻辑、@mention 触发、工作流流转
- **可以重构**：组件内部实现、视觉样式、状态管理方式
- **参考实现**：`D:\Tools\catCoffee\clowder-ai` 的消息渲染模式

## Non-Goals
- 不改变后端 WebSocket 消息格式
- 不修改 Agent 执行逻辑
- 不改变数据库 schema

## Acceptance Criteria
- [ ] **实时工具状态**：Agent 执行工具时，用户能看到当前正在做什么（如 "正在执行 Bash: npm test"）
- [ ] **思考块样式**：思考内容用深色终端风格展示，可折叠，有 60 字符预览截断
- [ ] **代码块优化**：代码块带复制按钮、语言标签、语法高亮
- [ ] **工具参数显示**：工具列表显示工具名称 + 主要参数（如 "Read foo.ts"）

## Technical Context

### clowder-ai 参考实现

| 文件 | 行数 | 关键特性 |
|------|------|----------|
| `ChatMessage.tsx` | 370 | 细粒度订阅 `useChatStore((s) => s.threads)`，early return 处理不同消息类型 |
| `CliOutputBlock.tsx` | 450 | `tintedDark()` 生成深色背景，`ToolRow` 显示工具状态/参数，自动折叠 |
| `MarkdownContent.tsx` | 320 | `CodeBlock` 带复制按钮，`highlightMentions` 高亮 @mention，`FilePathLink` VSCode 链接 |
| `ThinkingContent.tsx` | 147 | 可折叠深色面板，`BrainIcon`，60 字符预览，breed 色彩 |
| `ThinkingIndicator.tsx` | 177 | 状态指示器，liveness warning 超时检测，取消按钮 |
| `toCliEvents.ts` | 82 | 统一工具事件格式，`extractPrimaryArg` 提取主要参数 |

### ISDP 当前实现问题

| 文件 | 行数 | 问题 |
|------|------|------|
| `ChatMessage.tsx` | 423 | 职责混合，缺少细粒度订阅 |
| `CliOutputBlock.tsx` | 131 | 缺少参数显示和实时状态指示 |
| `ToolRow.tsx` | 116 | 结构简单，不显示工具参数 |
| `ThinkingBlock.tsx` | 165 | 浅色背景，无预览截断 |

### 关键改进点

1. **CliOutputBlock 增强**
   - 添加 `toCliEvents` 适配器，提取工具主要参数
   - `ToolRow` 显示工具名称 + 参数（如 "Read foo.ts"）
   - streaming 状态时高亮当前执行的工具

2. **ThinkingBlock 样式优化**
   - 深色背景：`tintedDark(breedColor, 0.25)` 生成
   - 添加 60 字符预览截断
   - BrainIcon 图标

3. **MarkdownContent 代码块**
   - `CodeBlock` 组件添加复制按钮
   - 语言标签显示

## Ontology (Key Entities)

| Entity | Type | Fields | Relationships |
|--------|------|--------|---------------|
| ChatMessage | core domain | role, content, timestamp, toolEvents, thinking | contains ThinkingBlock, CliOutputBlock |
| CliOutputBlock | core domain | events[], status, breedColor | contains ToolRow[] |
| ToolRow | core domain | name, primaryArg, status, duration | child of CliOutputBlock |
| ThinkingBlock | core domain | content, preview, expanded, breedColor | child of ChatMessage |
| MarkdownContent | supporting | content, codeBlocks[], mentions[] | used by ChatMessage |

## Implementation Plan

### Phase 1: CliOutputBlock 增强
1. 创建 `toCliEvents.ts` 适配器
2. 增强 `ToolRow` 显示工具参数
3. 添加 streaming 状态高亮

### Phase 2: ThinkingBlock 样式优化
1. 添加 `tintedDark` 函数生成深色背景
2. 添加预览截断逻辑
3. 更新 BrainIcon

### Phase 3: MarkdownContent 优化
1. 增强 CodeBlock 组件
2. 添加复制按钮和语言标签

## Interview Transcript
<details>
<summary>Full Q&A (2 rounds)</summary>

### Round 1
**Q:** 关于「自动路由逻辑」，你提到不要破坏它。具体来说，哪些功能是必须保留的？
**A:** 只保留核心路由逻辑（A2A 路由、@mention 触发、工作流流转，其他可重构）
**Ambiguity:** 65% → 25.5%

### Round 2
**Q:** 改进完成后，你期望看到哪些具体的用户体验提升？（多选）
**A:**
- 实时显示工具调用状态
- 思考块像 clowder-ai 一样展示
- 代码块渲染优化
- 工具调用显示参数
**Ambiguity:** 25.5% → 12.75%
</details>