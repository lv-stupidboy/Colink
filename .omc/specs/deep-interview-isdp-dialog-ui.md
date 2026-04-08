# Deep Interview Spec: ISDP 对话 UI 重构

## Metadata
- Interview ID: ui-refactor-20260407
- Rounds: 3
- Final Ambiguity Score: 13%
- Type: brownfield
- Generated: 2026-04-07
- Threshold: 0.2
- Status: PASSED

## Clarity Breakdown
| Dimension | Score | Weight | Weighted |
|-----------|-------|--------|----------|
| Goal Clarity | 0.95 | 0.35 | 0.333 |
| Constraint Clarity | 0.85 | 0.25 | 0.213 |
| Success Criteria Clarity | 0.85 | 0.25 | 0.213 |
| Context Clarity | 0.85 | 0.15 | 0.128 |
| **Total Clarity** | | | **0.875** |
| **Ambiguity** | | | **13%** |

## Goal
重构 ISDP 对话 UI，借鉴 Clowder AI 的设计模式，解决以下问题：
1. 工具调用列表太长刷屏
2. 无法识别当前正在执行哪个工具
3. 思考过程占用太多空间
4. 工具结果不清晰

## Constraints
- 保持 ISDP 亮色主题风格（不使用 Clowder 的深色主题）
- 内容块嵌套在 Agent 对话气泡内，按返回顺序穿插
- 采用两层折叠结构：CLI Output 整体折叠 → 工具列表单独折叠
- 活跃工具高亮样式保持现有（左边框 + 背景），不增加动画效果
- 工具列表按时间顺序显示（和现有逻辑相同）
- 重新实现现有组件（不重构，直接重写）

## Non-Goals
- 不调整活跃工具高亮样式（保持现有设计）
- 不改变工具列表显示顺序
- 不使用深色主题背景

## Acceptance Criteria
- [ ] CLI Output Block 实现两层折叠：整体折叠 + 工具列表折叠
- [ ] 工具汇总行显示 "3 tools" 格式，点击可展开工具列表
- [ ] 思考块折叠时显示前60字符预览 + 🧠图标
- [ ] 工具行点击展开显示完整输入+输出详情
- [ ] stdout 输出单独分区（分隔线 + "--- stdout ---" 标题）
- [ ] streaming 时自动展开，完成后自动折叠（但尊重用户交互）
- [ ] 实际执行 Agent 测试验证效果满意

## Assumptions Exposed & Resolved
| Assumption | Challenge | Resolution |
|------------|-----------|------------|
| 需要三层折叠 | Clowder 只有两层 | 确认采用两层折叠：整体→列表 |
| 需要活跃工具脉冲动画 | 用户说高亮样式不调整 | 保持现有左边框+背景高亮 |
| 重构现有组件 | 用户明确要求重新实现 | 直接重写组件 |

## Technical Context
现有组件结构：
- `CliOutputBlock.tsx` — 已有折叠逻辑，需要重写
- `ToolRow.tsx` — 已有基本设计，需要增强详情展开
- `ThinkingBlock.tsx` — 需要添加预览和智能折叠
- `ContentBlock.css` — 样式文件需要更新

参考 Clowder 组件：
- `CliOutputBlock.tsx` — 两层折叠 + 工具汇总行 + stdout 分区
- `ThinkingContent.tsx` — 60字符预览 + 脑图标 + 智能折叠
- `ToolRow` — 状态图标 + 扳手图标 + 工具名 + 参数预览

## Ontology (Key Entities)
| Entity | Type | Fields | Relationships |
|--------|------|--------|---------------|
| CLI Output Block | core UI | events, status, breedColor, expanded, userInteracted | contains Tool Row, stdout Section |
| Tool Summary Row | UI element | toolCount, expanded, onToggle | controls Tool Row visibility |
| Tool Row | UI element | event, isActive, accentColor, expanded | displays tool_use/tool_result |
| Thinking Block | UI element | content, expanded, preview, breedColor | displays thinking content |
| stdout Section | UI element | textEvents | displays stdout output |
| User Interaction Ref | state | userInteracted (boolean) | prevents auto-collapse override |

## Ontology Convergence
| Round | Entity Count | New | Changed | Stable | Stability Ratio |
|-------|-------------|-----|---------|--------|----------------|
| 1 | 4 | 4 | - | - | - |
| 2 | 5 | 1 | - | 4 | 80% |
| 3 | 6 | 1 | - | 5 | 83% |

## Interview Transcript
<details>
<summary>Full Q&A (3 rounds)</summary>

### Round 1
**Q:** 工具调用列表刷屏问题需要工具汇总行吗？思考过程折叠时需要预览吗？工具结果详情展开显示什么？stdout输出需要分区吗？
**A:** 工具汇总行需要，和Clowder一样；思考预览60字符+🧠图标；工具详情展开显示输入+输出；stdout需要分区显示
**Ambiguity:** 56% (Goal: 0.7, Constraints: 0.5, Criteria: 0.6, Context: 0.8)

### Round 2
**Q:** 折叠层级需要几层？高亮样式是否完全保持现状？智能折叠逻辑是否和Clowder一样？
**A:** 两层折叠（整体→列表）；只保留现有高亮；和Clowder一样的智能折叠逻辑
**Ambiguity:** 35% (Goal: 0.85, Constraints: 0.75, Criteria: 0.6, Context: 0.8)

### Round 3
**Q:** 如何验证实现效果？是否重构现有组件？工具列表显示顺序？
**A:** 实际执行测试；重新实现（不重构）；按时间顺序（和现有）
**Ambiguity:** 13% (Goal: 0.95, Constraints: 0.85, Criteria: 0.85, Context: 0.85)

</details>

## Implementation Plan

### Step 1: Rewrite CliOutputBlock
- 添加两层折叠结构
- 添加工具汇总行（ToolSummaryRow）
- 添加 stdout 分区（分隔线 + "--- stdout ---"）
- 实现 streaming 自动展开逻辑
- 实现 userInteracted ref 防止自动折叠覆盖用户操作

### Step 2: Rewrite ToolRow
- 点击展开显示完整输入+输出
- 保持现有高亮样式（左边框 + 背景）

### Step 3: Rewrite ThinkingBlock
- 折叠时显示前60字符预览
- 添加 🧠 脑图标
- 实现智能折叠逻辑（streaming展开→完成后折叠）

### Step 4: Update CSS
- 更新 ContentBlock.css 或 CliOutputBlock.css
- 确保样式符合 ISDP 亮色主题

### Step 5: Test
- 实际执行 Agent，验证效果