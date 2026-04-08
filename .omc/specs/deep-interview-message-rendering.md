# Deep Interview Spec: 消息渲染组件重实现

## Metadata
- Interview ID: di-clowder-ui-001
- Rounds: 5
- Final Ambiguity Score: 14%
- Type: brownfield
- Generated: 2026-04-02
- Threshold: 20%
- Status: PASSED

## Clarity Breakdown
| Dimension | Score | Weight | Weighted |
|-----------|-------|--------|----------|
| Goal Clarity | 0.95 | 0.40 | 0.38 |
| Constraint Clarity | 0.80 | 0.30 | 0.24 |
| Success Criteria | 0.80 | 0.30 | 0.24 |
| **Total Clarity** | | | **0.86** |
| **Ambiguity** | | | **14%** |

## Goal
参考 clowder-ai 项目重新实现 ISDP 的消息渲染组件，包括：品种样式、@提及高亮、思考块、文件路径链接。

## Constraints
- 统一使用白色背景，不需要颜色配置
- 重新实现消息组件，替换现有 ThreadView 中的消息渲染
- 需要兼容 ISDP 现有的 Zustand store 和数据模型
- 使用 React + TypeScript + Ant Design 技术栈

## Non-Goals
- 不实现悄悄话（whisper）功能
- 不实现回复标签（reply pill）功能
- 不实现 CLI Output 块（已有独立组件）
- 不实现 Rich Blocks（card, diff 等）

## Acceptance Criteria
- [ ] 品种样式生效：不同 Agent 气泡有不同的圆角样式
- [ ] @提及高亮正确：@Agent 名字用对应 Agent 的颜色渲染
- [ ] 思考块交互正常：可展开/折叠，默认折叠
- [ ] 文件路径链接可用：点击打开 VSCode

## Assumptions Exposed & Resolved
| Assumption | Challenge | Resolution |
|------------|-----------|------------|
| 需要颜色配置 | 用户是否需要自定义颜色？ | 统一白色背景，不需要颜色配置 |
| ISDP AgentConfig 有颜色字段 | 现有模型是否支持？ | 不需要，统一白色背景 |

## Technical Context

### clowder-ai 参考组件
| 组件 | 路径 | 用途 |
|------|------|------|
| ChatMessage | `packages/web/src/components/ChatMessage.tsx` | 消息气泡主组件 |
| MarkdownContent | `packages/web/src/components/MarkdownContent.tsx` | Markdown 渲染 + @提及高亮 + 文件路径链接 |
| ThinkingContent | `packages/web/src/components/ThinkingContent.tsx` | 思考块组件 |

### 关键实现细节

**1. 品种样式 (BREED_STYLES)**
```typescript
const BREED_STYLES: Record<string, { radius: string; font?: string }> = {
  ragdoll: { radius: 'rounded-2xl rounded-bl-sm' },
  'maine-coon': { radius: 'rounded-2xl rounded-br-sm', font: 'font-mono' },
  siamese: { radius: 'rounded-2xl rounded-tr-sm' },
  'dragon-li': { radius: 'rounded-lg rounded-tl-sm', font: 'font-mono' },
};
```

**2. @提及高亮**
- 使用正则匹配 `@Agent名`
- 从 Agent 配置获取颜色
- 渲染为带背景色的 span

**3. 思考块**
- 可折叠面板
- 默认折叠
- 显示 "Thinking" 标签和预览

**4. 文件路径链接**
- 正则匹配文件路径（支持行号）
- 点击打开 VSCode (vscode://file 协议)
- Cmd/Ctrl+Click 打开 VSCode，普通点击打开工作区面板

## Ontology (Key Entities)
| Entity | Type | Fields | Relationships |
|--------|------|--------|---------------|
| ChatMessage | core domain | id, type, catId, content, thinking, timestamp | 属于 Thread |
| Agent | supporting | id, name, breedId, color | 被消息引用 |
| ThinkingBlock | supporting | content, expanded | 嵌入 ChatMessage |

## Ontology Convergence
| Round | Entity Count | New | Changed | Stable | Stability Ratio |
|-------|-------------|-----|---------|--------|----------------|
| 1 | 2 | 2 | - | - | - |
| 2 | 3 | 1 | 0 | 2 | 67% |
| 3 | 4 | 1 | 0 | 3 | 75% |
| 4 | 5 | 1 | 0 | 4 | 80% |
| 5 | 5 | 0 | 0 | 5 | 100% |

## Interview Transcript
<details>
<summary>Full Q&A (5 rounds)</summary>

### Round 1
**Q:** 你想参考 clowder-ai 对话展示的哪些具体方面？
**A:** Message rendering
**Ambiguity:** 71% (Goal: 0.5, Constraints: 0.3, Criteria: 0.2)

### Round 2
**Q:** 你想实现消息渲染的哪些功能？
**A:** 品种样式, @提及高亮, 思考块, 文件路径链接
**Ambiguity:** 57% (Goal: 0.7, Constraints: 0.3, Criteria: 0.2)

### Round 3
**Q:** 你希望如何集成到 ISDP 现有代码？
**A:** 重新实现
**Ambiguity:** 43% (Goal: 0.8, Constraints: 0.5, Criteria: 0.2)

### Round 4
**Q:** 完成后需要满足哪些验收标准？
**A:** 品种样式生效, @提及高亮正确, 思考块交互正常, 文件路径链接可用
**Ambiguity:** 22% (Goal: 0.9, Constraints: 0.6, Criteria: 0.8)

### Round 5
**Q:** ISDP 的 AgentConfig 模型是否已有颜色/样式字段？
**A:** 颜色不要配置了，统一底色改成白色吧
**Ambiguity:** 14% (Goal: 0.95, Constraints: 0.8, Criteria: 0.8)

</details>