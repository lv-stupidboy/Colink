# 实现计划：消息渲染组件重实现

## 需求总结
参考 clowder-ai 项目重新实现 ISDP 的消息渲染组件，包括：品种样式、@提及高亮、思考块、文件路径链接。

## RALPLAN-DR Summary

### Principles
1. **组件解耦** - 消息渲染组件独立于 ThreadView，易于测试和复用
2. **渐进增强** - 在现有 MessageContent 基础上增加新功能，保持向后兼容
3. **性能优先** - 使用 memo 和 useCallback 优化渲染性能
4. **类型安全** - 完整的 TypeScript 类型定义

### Decision Drivers
1. **品牌一致性** - 不同 Agent 有视觉区分，提升用户体验
2. **开发效率** - 复用 clowder-ai 验证过的设计模式
3. **维护成本** - 代码结构清晰，易于后续扩展

### Viable Options

| Option | Pros | Cons |
|--------|------|------|
| **A. 新建独立组件** | 完全控制，不影响现有代码 | 需要迁移成本 |
| **B. 扩展现有组件** | 改动小，向后兼容 | 代码可能变复杂 |
| **C. 替换现有组件** | 最干净，无历史包袱 | 需要完整测试 |

**推荐：Option A** - 新建独立组件，在 ThreadView 中逐步替换。

---

## Acceptance Criteria
- [ ] 品种样式生效：不同 Agent 气泡有不同的圆角样式（白色背景）
- [ ] @提及高亮正确：@Agent 名字用对应 Agent 的颜色渲染
- [ ] 思考块交互正常：可展开/折叠，默认折叠
- [ ] 文件路径链接可用：点击打开 VSCode

---

## Implementation Steps

### Step 1: 创建 Agent 样式配置 (30min)
**文件**: `isdp/web/src/config/agentStyles.ts`

定义 Agent 样式映射（breed 样式），基于 Agent 角色类型：
- `requirement`: 需求分析师样式
- `architect`: 架构师样式
- `developer`: 开发者样式
- `reviewer`: 评审者样式
- `testengineer`: 测试工程师样式
- `devops`: 运维工程师样式
- `fullstack_engineer`: 全栈工程师样式
- `custom`: 自定义样式

每个样式包含：
```typescript
interface AgentStyleConfig {
  radius: string;      // 气泡圆角
  font?: string;      // 字体（可选）
  color: string;      // 主色调（用于 @提及高亮）
}
```

### Step 2: 创建 @提及高亮工具函数 (45min)
**文件**: `isdp/web/src/utils/mentionHighlight.ts`

功能：
1. `getMentionRegex()` - 返回匹配 @Agent名 的正则表达式
2. `highlightMentions(text, agentConfigs)` - 高亮文本中的 @提及
3. `getMentionAgentId(mentionText)` - 从提及文本获取 Agent ID

关键逻辑：
- 从 store 获取 Agent 配置列表
- 根据 Agent 名字匹配颜色
- 渲染为带背景色的 span 元素

### Step 3: 创建文件路径链接组件 (30min)
**文件**: `isdp/web/src/components/thread/FilePathLink.tsx`

功能：
1. 正则匹配文件路径（支持相对路径和绝对路径）
2. 支持行号（如 `src/file.ts:10`）
3. 点击生成 `vscode://file` 协议链接
4. Cmd/Ctrl+Click 打开 VSCode

### Step 4: 创建思考块组件 (45min)
**文件**: `isdp/web/src/components/thread/ThinkingBlock.tsx`

功能：
1. 可折叠面板，默认折叠
2. 显示 "Thinking" 标签和内容预览（前 60 字符）
3. 展开/折叠动画
4. 支持深色主题样式

参考 clowder-ai 的 `ThinkingContent.tsx`：
- 使用 brain icon
- 预览文本截断
- 展开/折叠状态管理

### Step 5: 重构 MessageContent 组件 (60min)
**文件**: `isdp/web/src/components/thread/MessageContent.tsx`

增强现有组件：
1. 集成 @提及高亮
2. 集成文件路径链接
3. 保持现有 Markdown 渲染功能

修改点：
- 在文本处理流程中先处理 @提及和文件路径
- 然后传递给 ReactMarkdown

### Step 6: 创建新的 ChatMessage 组件 (90min)
**文件**: `isdp/web/src/components/thread/ChatMessage.tsx`

主要功能：
1. 用户消息气泡（右侧，绿色/白色）
2. Agent 消息气泡（左侧，根据角色样式）
3. 系统消息（居中，蓝色背景）
4. 支持流式渲染状态
5. 支持 thinking 内容显示

Props:
```typescript
interface ChatMessageProps {
  message: Message;
  agentConfig?: AgentConfig;
  isStreaming?: boolean;
  thinkingContent?: string;
}
```

### Step 7: 创建 ChatMessageList 组件 (30min)
**文件**: `isdp/web/src/components/thread/ChatMessageList.tsx`

功能：
1. 消息列表渲染
2. 自动滚动到底部
3. 加载状态处理
4. 空状态展示

### Step 8: 更新 ThreadView 使用新组件 (45min)
**文件**: `isdp/web/src/pages/ThreadView.tsx`

修改点：
1. 引入 ChatMessageList 替换现有消息渲染
2. 传递 Agent 配置给消息组件
3. 保留现有的 WebSocket 和 store 逻辑

### Step 9: 添加样式文件 (30min)
**文件**: `isdp/web/src/components/thread/ChatMessage.css`

样式包括：
- 消息气泡基础样式
- 品种样式圆角
- @提及高亮样式
- 思考块样式
- 文件路径链接样式

### Step 10: 测试和调试 (60min)

测试用例：
1. 用户消息正确显示（右侧，白色背景）
2. 不同角色 Agent 消息有不同的圆角样式
3. @提及正确高亮，颜色匹配 Agent 配置
4. 文件路径可点击，正确生成 VSCode 链接
5. 思考块可展开/折叠
6. 流式消息正确渲染

---

## Files to Create/Modify

| 文件 | 操作 | 说明 |
|------|------|------|
| `src/config/agentStyles.ts` | 新建 | Agent 样式配置 |
| `src/utils/mentionHighlight.ts` | 新建 | @提及高亮工具 |
| `src/components/thread/FilePathLink.tsx` | 新建 | 文件路径链接组件 |
| `src/components/thread/ThinkingBlock.tsx` | 新建 | 思考块组件 |
| `src/components/thread/ChatMessage.tsx` | 新建 | 消息气泡组件 |
| `src/components/thread/ChatMessageList.tsx` | 新建 | 消息列表组件 |
| `src/components/thread/ChatMessage.css` | 新建 | 消息样式 |
| `src/components/thread/MessageContent.tsx` | 修改 | 增强 Markdown 渲染 |
| `src/pages/ThreadView.tsx` | 修改 | 使用新组件 |

---

## Risks and Mitigations

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 样式冲突 | 中 | 使用 CSS Modules 或唯一类名前缀 |
| 性能问题 | 中 | 使用 memo 和虚拟滚动 |
| 向后兼容 | 低 | 保留旧组件，渐进替换 |
| thinking 字段缺失 | 高 | 检查后端支持，若无则暂不实现或在 metadata 中存储 |
| @提及颜色映射 | 中 | 从 store 获取 agentConfigs，fallback 使用默认颜色 |
| 文件路径项目根 | 中 | 从 currentProject?.localPath 获取项目根目录 |

---

## Verification Steps

1. **功能验证**
   - 发送用户消息，验证气泡样式
   - 触发 Agent 回复，验证不同角色的气泡样式
   - 输入 @提及，验证高亮颜色
   - 输入文件路径，验证链接可点击

2. **UI 验证**
   - 对比 clowder-ai 的视觉效果
   - 验证响应式布局
   - 验证暗色模式（如有）

3. **性能验证**
   - 长消息列表滚动流畅度
   - 快速发送消息时的渲染性能

---

## ADR (Architecture Decision Record)

### Decision
新建独立的消息渲染组件（ChatMessage、ChatMessageList），而非扩展现有的 MessageContent。

### Drivers
1. 需要实现品种样式，现有组件结构不支持
2. 需要支持思考块，需要新的数据结构和渲染逻辑
3. 保持代码清晰，避免过度复杂的条件渲染

### Alternatives considered
1. **扩展现有 MessageContent** - 会导致组件过于庞大，维护困难
2. **完全重写 ThreadView** - 改动范围太大，风险高

### Why chosen
新建组件可以在不影响现有功能的情况下渐进式替换，降低风险。

### Consequences
- 需要维护两套消息渲染代码（过渡期）
- 最终可以删除旧的 MessageCard 组件

### Follow-ups
1. 添加单元测试
2. 添加 Storybook 组件展示
3. 考虑支持自定义主题

---

## Architect Review Notes

### 改进建议 (2026-04-02)

1. **数据模型问题**: ISDP 的 Message 类型没有 `thinking` 字段，需要：
   - 检查后端是否支持 thinking 内容
   - 或者将 thinking 作为 metadata 的一部分存储

2. **样式映射简化**: 直接使用 `AgentRole` 作为样式 key：
   ```typescript
   const AGENT_STYLES: Record<AgentRole, AgentStyleConfig> = {
     requirement: { radius: 'rounded-2xl rounded-bl-sm', color: '#1890ff' },
     architect: { radius: 'rounded-2xl rounded-br-sm', color: '#722ed1' },
     // ...
   };
   ```

3. **混合方案**: 
   - 创建独立的 `ThinkingBlock` 组件
   - 扩展现有 `MessageContent` 添加 @提及和文件路径链接
   - 创建 `ChatMessage` 容器组件整合样式

---

## Changelog
- 2026-04-02: Initial plan created by Planner
- 2026-04-02: Architect review completed, added improvement notes