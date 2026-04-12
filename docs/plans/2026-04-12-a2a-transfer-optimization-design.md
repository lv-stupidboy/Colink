# A2A 传输优化设计文档

**日期**: 2026-04-12
**作者**: Claude Code
**状态**: 已批准

---

## 问题描述

当前 A2A（Agent-to-Agent）传输机制存在以下问题：

1. **历史消息冗余传递**：给下一个 Agent 传递了历史消息（Layer1），但：
   - 跨角色调用（SessionStrategyNew）：CLI 使用新会话，历史不相关
   - 同角色调用（SessionStrategyResume）：CLI --resume 自动恢复历史，无需重复传递

2. **前序输出未过滤**：完整传递前序 Agent 的输出，内容可能过于冗长

3. **前端缺少 A2A 传递信息显示**：用户无法直观了解 A2A 之间传递了什么信息

---

## 解决方案概述

采用方案 B：修改现有 flow，利用 fullPrompt 机制

**核心改动**：
- 后端：根据会话策略跳过 Layer1 构建，结构化过滤前序输出
- 前端：在调用日志面板区分显示 A2A 输入

---

## 详细设计

### 一、后端改动

#### 1. buildContextLayers 修改

**文件**: `internal/service/agent/execution_service.go:932-957`

**改动**：根据 `SpawnRequest.SessionStrategy` 决定是否构建 Layer1（历史消息）

```go
// Layer 1: Thread历史 - 根据会话策略决定是否传递
// A2A 机制下，无论是跨角色还是同角色，都不需要传递历史消息：
// - 跨角色：CLI 使用新会话，历史不相关
// - 同角色：CLI --resume 自动恢复历史，无需重复传递
if req.SessionStrategy == SessionStrategyNew || req.SessionStrategy == SessionStrategyResume {
    layers.Layer1 = "" // A2A 调用不传递历史
} else {
    // 非 A2A 调用（用户直接触发）：保持传递历史
    messages, err := es.msgRepo.FindByThreadID(ctx, threadID, 100)
    if err != nil {
        return nil, err
    }
    layers.Layer1 = es.formatMessages(messages)
}
```

**注意**：需要将 `req` 参数传递到 `buildContextLayers` 方法

#### 2. 新增结构化过滤函数 filterStructuredOutput

**文件**: `internal/service/agent/execution_service.go`

**功能**：从前序 Agent 输出中提取关键信息

**保留内容类型**：
- 文件路径引用：`file://xxx`、`path:xxx`、`./xxx`、`xxx.go` 等
- 关键结论标记：`结论:`、`结果:`、`关键点:`、`总结:`、`建议:` 等
- 工具调用结果：从 contentBlocks 中提取

**实现**：
```go
// filterStructuredOutput 从前序 Agent 输出中提取结构化关键信息
// 保留：文件路径引用、关键结论标记、工具调用结果
func (es *ExecutionService) filterStructuredOutput(output string, contentBlocks []ContentBlockData) string {
    var result []string

    // 1. 提取文件路径引用
    filePatterns := []string{
        `file://[^\s]+`,           // file://xxx
        `path:\s*[^\s]+`,          // path: xxx
        `\.\/[^\s]+`,              // ./xxx
        `[a-zA-Z0-9_\-]+\.[a-z]+`, // xxx.go, xxx.ts 等（但排除常见干扰词）
    }
    for _, pattern := range filePatterns {
        re := regexp.MustCompile(pattern)
        matches := re.FindAllString(output, -1)
        result = append(result, matches...)
    }

    // 2. 提取关键结论标记后的内容
    conclusionPatterns := []string{
        `结论[:：]\s*[^\n]+`,
        `结果[:：]\s*[^\n]+`,
        `关键点[:：]\s*[^\n]+`,
        `总结[:：]\s*[^\n]+`,
        `建议[:：]\s*[^\n]+`,
        `要点[:：]\s*[^\n]+`,
    }
    for _, pattern := range conclusionPatterns {
        re := regexp.MustCompile(pattern)
        matches := re.FindAllString(output, -1)
        result = append(result, matches...)
    }

    // 3. 提取工具调用结果（如果有）
    for _, block := range contentBlocks {
        if block.Type == "tool_use" && block.Output != "" {
            result = append(result, fmt.Sprintf("[%s] %s", block.ToolName, block.Output))
        }
    }

    // 去重并返回
    seen := make(map[string]bool)
    var unique []string
    for _, item := range result {
        item = strings.TrimSpace(item)
        if item != "" && !seen[item] {
            seen[item] = true
            unique = append(unique, item)
        }
    }

    return strings.Join(unique, "\n")
}
```

#### 3. 修改 buildA2AInput

**文件**: `internal/service/agent/execution_service.go:1319-1368`

**改动**：
- 添加 `sessionStrategy` 参数
- 添加会话策略信息部分
- 使用 `filterStructuredOutput` 处理前序输出

```go
func (es *ExecutionService) buildA2AInput(
    ctx context.Context,
    threadID uuid.UUID,
    fromAgent *model.AgentRoleConfig,
    a2aCtx *A2AContext,
    output string,
    contentBlocks []ContentBlockData,
    sessionStrategy SessionStrategy,
) string {
    var sb strings.Builder

    // 1. 协作规则（有触发者信息时注入）
    if a2aCtx != nil && a2aCtx.FromAgent != nil {
        sb.WriteString("## 协作规则\n\n")
        sb.WriteString("A2A 出口检查：回复前问\"到我这里结束了吗？\"不是 → 谁需要动 → 末尾另起一行行首写 @句柄。\n\n")
        sb.WriteString("---\n\n")
    }

    // 2. 会话策略信息（新增）
    sb.WriteString("## 会话策略\n\n")
    if sessionStrategy == SessionStrategyResume {
        sb.WriteString("**类型**: Resume（恢复会话）\n")
        sb.WriteString("**说明**: CLI 将使用 --resume 恢复之前的会话上下文\n\n")
    } else {
        sb.WriteString("**类型**: New（新会话）\n")
        sb.WriteString("**说明**: CLI 将使用全新会话，不继承历史上下文\n\n")
    }
    sb.WriteString("---\n\n")

    // 3. 原始请求
    originalMessage := es.getLastUserMessage(ctx, threadID)
    if originalMessage != "" {
        sb.WriteString("## 原始请求\n\n")
        sb.WriteString(originalMessage)
        sb.WriteString("\n\n---\n\n")
    }

    // 4. 前序分析（使用结构化过滤）
    if fromAgent != nil {
        sb.WriteString("## 前序分析（结构化摘要）\n\n")
        sb.WriteString(fmt.Sprintf("**来自**: %s\n", fromAgent.Name))
        if fromAgent.Role != "" {
            sb.WriteString(fmt.Sprintf("**角色**: %s\n", es.getRoleDescription(fromAgent.Role)))
        }
        if fromAgent.Description != "" {
            sb.WriteString(fmt.Sprintf("**擅长**: %s\n", fromAgent.Description))
        }
        sb.WriteString("\n")

        // 结构化过滤后的输出
        filteredOutput := es.filterStructuredOutput(output, contentBlocks)
        if filteredOutput != "" {
            sb.WriteString(filteredOutput)
        } else {
            sb.WriteString("(无关键结构化信息)")
        }
        sb.WriteString("\n\n---\n\n")
    }

    // 5. 触发者信息
    if a2aCtx != nil && a2aCtx.FromAgent != nil {
        sb.WriteString(fmt.Sprintf("**Direct message from %s; reply to %s**\n",
            a2aCtx.FromAgent.Name,
            a2aCtx.FromAgent.Name))
    }

    return sb.String()
}
```

**调用处改动**：
- `checkRouting`: 需要传递 `contentBlocks` 和 `sessionStrategy`
- `checkSignalRouting`: 同样需要传递这些参数

#### 4. 修改 formatFullPrompt

**文件**: `internal/service/agent/execution_service.go:2462-2502`

**改动**：区分 A2A 输入和普通用户输入

```go
// 用户输入部分：区分 A2A 输入和普通输入
if strings.Contains(input, "## 会话策略") || strings.Contains(input, "## 前序分析") {
    // A2A 输入：使用特殊标签
    sb.WriteString("<a2a_input>\n")
    sb.WriteString(input)
    sb.WriteString("\n</a2a_input>\n")
} else {
    // 普通用户输入
    sb.WriteString("<user>\n")
    sb.WriteString(input)
    sb.WriteString("\n</user>\n")
}
```

---

### 二、前端改动

#### 1. AgentInvocationLogPanel 改动

**文件**: `web/src/components/thread/StatusPanel/AgentInvocationLogPanel.tsx`

**改动内容**：
- 新增 `parseA2AInput` 函数解析 A2A 输入
- 修改输入显示区域，区分 A2A 输入和普通用户输入
- 使用 Collapse 组件展开显示各部分内容

**新增解析函数**：
```tsx
interface A2AInputInfo {
  isA2A: boolean;
  triggerInfo: string;      // 触发者信息
  sessionStrategy: string;  // 会话策略
  originalRequest: string;  // 原始请求
  filteredOutput: string;   // 过滤后的前序输出
}

const parseA2AInput = (fullPrompt: string): A2AInputInfo | null => {
  // 检查是否包含 <a2a_input> 标签
  const a2aMatch = fullPrompt.match(/<a2a_input>([\s\S]*?)<\/a2a_input>/);
  if (!a2aMatch) return null;

  const a2aContent = a2aMatch[1];

  // 解析各部分
  const triggerMatch = a2aContent.match(/\*\*来自\*\*:\s*(.+)/);
  const strategyMatch = a2aContent.match(/\*\*类型\*\*:\s*(.+)/);
  const requestMatch = a2aContent.match(/## 原始请求\s+([\s\S]*?)---/);
  const outputMatch = a2aContent.match(/## 前序分析[\s\S]*?\n\n([\s\S]*?)---/);

  return {
    isA2A: true,
    triggerInfo: triggerMatch?.[1]?.trim() || '',
    sessionStrategy: strategyMatch?.[1]?.trim() || '',
    originalRequest: requestMatch?.[1]?.trim() || '',
    filteredOutput: outputMatch?.[1]?.trim() || '',
  };
};
```

**UI 显示改动**：
```tsx
// 在组件中判断
const a2aInfo = inv.fullPrompt ? parseA2AInput(inv.fullPrompt) : null;

// 渲染时区分显示
{a2aInfo ? (
  <div className="a2a-input-section">
    <Tag color="blue">A2A 输入</Tag>
    <div className="a2a-meta">
      <span>触发者: {a2aInfo.triggerInfo}</span>
      <span>会话策略: {a2aInfo.sessionStrategy}</span>
    </div>
    <Collapse ghost>
      <Collapse.Panel key="request" header="原始请求">
        <pre>{a2aInfo.originalRequest}</pre>
      </Collapse.Panel>
      <Collapse.Panel key="output" header="前序输出摘要">
        <pre>{a2aInfo.filteredOutput}</pre>
      </Collapse.Panel>
    </Collapse>
  </div>
) : (
  // 普通用户输入显示（保持现有逻辑）
  <div className="user-input">...</div>
)}
```

#### 2. CSS 样式适配

**文件**: `web/src/components/thread/StatusPanel/AgentInvocationLogPanel.css` 或相关样式

**新增样式**（遵循深色模式适配规则）：
```css
/* A2A 输入区域 */
.a2a-input-section {
  background: var(--bg-container);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 12px;
  margin-bottom: 8px;
}

.a2a-meta {
  display: flex;
  gap: 16px;
  color: var(--text-secondary);
  margin-top: 8px;
  margin-bottom: 8px;
}

.a2a-meta span {
  font-size: 12px;
}

/* 深色模式适配 */
[data-theme='dark'] .a2a-input-section {
  background: var(--bg-elevated);
}

[data-theme='dark'] .a2a-meta {
  color: var(--text-secondary);
}
```

---

## 改动文件清单

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/service/agent/execution_service.go` | 修改 | buildContextLayers、buildA2AInput、formatFullPrompt、新增 filterStructuredOutput |
| `web/src/components/thread/StatusPanel/AgentInvocationLogPanel.tsx` | 修改 | 新增 parseA2AInput、修改输入显示逻辑 |
| `web/src/components/thread/StatusPanel/AgentInvocationLogPanel.css` | 新增 | A2A 输入样式 |

---

## 测试要点

1. **后端测试**：
   - A2A 调用时 Layer1 是否为空
   - filterStructuredOutput 是否正确提取关键信息
   - buildA2AInput 输出格式是否正确
   - fullPrompt 中是否正确标记 `<a2a_input>`

2. **前端测试**：
   - A2A 输入是否正确识别和显示
   - 深色模式下样式是否正常
   - 展开/折叠功能是否正常

3. **集成测试**：
   - 完整 A2A 调用流程验证
   - 前端显示与后端传递信息一致性

---

## 实现步骤

1. 后端：修改 buildContextLayers，跳过 Layer1 构建
2. 后端：新增 filterStructuredOutput 函数
3. 后端：修改 buildA2AInput，应用过滤并添加会话策略信息
4. 后端：修改 formatFullPrompt，区分 A2A 输入
5. 前端：新增 parseA2AInput 解析函数
6. 前端：修改 AgentInvocationLogPanel 显示逻辑
7. 前端：新增 CSS 样式
8. 测试验证