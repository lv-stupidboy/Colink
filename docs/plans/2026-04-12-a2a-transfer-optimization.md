# A2A 传输优化实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 优化 A2A 传输机制，根据会话策略跳过历史消息传递，结构化过滤前序输出，前端区分显示 A2A 输入信息

**Architecture:** 
- 后端：修改 buildContextLayers 跳过 Layer1，新增 filterStructuredOutput 结构化过滤，修改 buildA2AInput 添加会话策略信息
- 前端：修改 AgentInvocationLogPanel 解析 `<a2a_input>` 标签并特殊显示

**Tech Stack:** Go + Gin（后端）、React + Ant Design（前端）

---

## Task 1: 后端 - 修改 buildContextLayers 参数传递

**Files:**
- Modify: `internal/service/agent/execution_service.go`

**背景**: buildContextLayers 需要接收 SpawnRequest 参数来判断会话策略

### Step 1: 修改 buildContextLayers 函数签名

在 `internal/service/agent/execution_service.go` 找到 `buildContextLayers` 函数定义（约 932 行），修改签名添加 `req *SpawnRequest` 参数：

```go
// 原签名
func (es *ExecutionService) buildContextLayers(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig) (*ContextLayers, error)

// 新签名
func (es *ExecutionService) buildContextLayers(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, req *SpawnRequest) (*ContextLayers, error)
```

### Step 2: 修改 buildContextLayers 内部 Layer1 构建逻辑

在函数内部（约 945-950 行），将 Layer1 构建改为根据会话策略判断：

```go
// Layer 1: Thread历史 - 根据会话策略决定是否传递
// A2A 机制下，无论是跨角色还是同角色，都不需要传递历史消息：
// - 跨角色：CLI 使用新会话，历史不相关
// - 同角色：CLI --resume 自动恢复历史，无需重复传递
if req != nil && (req.SessionStrategy == SessionStrategyNew || req.SessionStrategy == SessionStrategyResume) {
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

### Step 3: 更新 executeAgent 中的 buildContextLayers 调用

在 `executeAgent` 函数中（约 339 行），更新调用添加 `req` 参数：

```go
// 原调用
contextLayers, err := es.buildContextLayers(ctx, req.ThreadID, config)

// 新调用
contextLayers, err := es.buildContextLayers(ctx, req.ThreadID, config, req)
```

### Step 4: 验证构建

```bash
cd D:/CoLinkProject/Colink-0412/isdp
go build ./cmd/server
```

Expected: 构建成功，无编译错误

### Step 5: Commit

```bash
git add internal/service/agent/execution_service.go
git commit -m "refactor(agent): buildContextLayers 支持会话策略参数，A2A 调用跳过历史传递

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: 后端 - 新增 filterStructuredOutput 函数

**Files:**
- Modify: `internal/service/agent/execution_service.go`

### Step 1: 在 execution_service.go 文件末尾添加 filterStructuredOutput 函数

在 `execution_service.go` 文件末尾（约 2517 行之后）添加新函数：

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
    }
    for _, pattern := range filePatterns {
        re := regexp.MustCompile(pattern)
        matches := re.FindAllString(output, -1)
        result = append(result, matches...)
    }

    // 2. 提取代码文件引用（排除干扰词）
    // 匹配 xxx.go, xxx.ts, xxx.py 等文件名
    codeFilePattern := `[a-zA-Z0-9_\-]+\.(go|ts|tsx|js|jsx|py|java|kt|rs|c|cpp|h|sql|yaml|yml|json|md)`
    re := regexp.MustCompile(codeFilePattern)
    matches := re.FindAllString(output, -1)
    // 过滤掉常见干扰词
    excludeWords := map[string]bool{
        "true.md": true, "false.md": true, "null.json": true,
    }
    for _, m := range matches {
        if !excludeWords[m] {
            result = append(result, m)
        }
    }

    // 3. 提取关键结论标记后的内容
    conclusionPatterns := []string{
        `结论[:：]\s*[^\n]+`,
        `结果[:：]\s*[^\n]+`,
        `关键点[:：]\s*[^\n]+`,
        `总结[:：]\s*[^\n]+`,
        `建议[:：]\s*[^\n]+`,
        `要点[:：]\s*[^\n]+`,
        `分析结果[:：]\s*[^\n]+`,
    }
    for _, pattern := range conclusionPatterns {
        re := regexp.MustCompile(pattern)
        matches := re.FindAllString(output, -1)
        result = append(result, matches...)
    }

    // 4. 提取工具调用结果（如果有）
    for _, block := range contentBlocks {
        if block.Type == "tool_use" && block.Output != "" {
            // 截取工具输出前 200 字符（避免过长）
            outputStr := block.Output
            if len(outputStr) > 200 {
                outputStr = outputStr[:200] + "..."
            }
            result = append(result, fmt.Sprintf("[%s] %s", block.ToolName, outputStr))
        }
    }

    // 去重并返回
    seen := make(map[string]bool)
    var unique []string
    for _, item := range result {
        item = strings.TrimSpace(item)
        if item != "" && !seen[item] && len(item) > 3 { // 过滤过短内容
            seen[item] = true
            unique = append(unique, item)
        }
    }

    if len(unique) == 0 {
        return "(无关键结构化信息)"
    }

    return strings.Join(unique, "\n")
}
```

### Step 2: 确保必要的 import 已添加

检查文件顶部 import 区域是否包含 `regexp`：
```go
import (
    "regexp"
    // ... 其他 imports
)
```

如果缺少，添加 `regexp` 到 import 列表。

### Step 3: 验证构建

```bash
cd D:/CoLinkProject/Colink-0412/isdp
go build ./cmd/server
```

Expected: 构建成功

### Step 4: Commit

```bash
git add internal/service/agent/execution_service.go
git commit -m "feat(agent): 新增 filterStructuredOutput 结构化过滤函数

从前序输出中提取：文件路径引用、关键结论标记、工具调用结果

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 3: 后端 - 修改 buildA2AInput 函数

**Files:**
- Modify: `internal/service/agent/execution_service.go`

### Step 1: 修改 buildA2AInput 函数签名

找到 `buildA2AInput` 函数（约 1319 行），修改签名添加参数：

```go
// 原签名
func (es *ExecutionService) buildA2AInput(ctx context.Context, threadID uuid.UUID, fromAgent *model.AgentRoleConfig, a2aCtx *A2AContext, output string) string

// 新签名
func (es *ExecutionService) buildA2AInput(
    ctx context.Context,
    threadID uuid.UUID,
    fromAgent *model.AgentRoleConfig,
    a2aCtx *A2AContext,
    output string,
    contentBlocks []ContentBlockData,
    sessionStrategy SessionStrategy,
) string
```

### Step 2: 修改 buildA2AInput 函数体

完全替换函数体：

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
        sb.WriteString(filteredOutput)
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

### Step 3: 更新 checkRouting 中的 buildA2AInput 调用

找到 `checkRouting` 函数中调用 `buildA2AInput` 的位置（约 1274 行），更新参数：

```go
// 原调用
a2aInput := es.buildA2AInput(ctx, threadID, currentConfig, a2aCtx, output)

// 新调用 - 需要获取 contentBlocks
var contentBlocks []ContentBlockData
es.mu.Lock()
if agent, ok := es.runningAgents[invocation.ID]; ok {
    agent.ContentBlocksMu.Lock()
    contentBlocks = make([]ContentBlockData, len(agent.AccumulatedContentBlocks))
    copy(contentBlocks, agent.AccumulatedContentBlocks)
    agent.ContentBlocksMu.Unlock()
}
es.mu.Unlock()

a2aInput := es.buildA2AInput(ctx, threadID, currentConfig, a2aCtx, output, contentBlocks, sessionStrategy)
```

**注意**: `checkRouting` 没有 invocation ID，需要从 RunningAgent 中获取。由于 `checkRouting` 是在 `executeAgent` 完成后调用，此时 invocation 已不在 runningAgents 中。

实际改用更简单的方式：直接传递 nil contentBlocks（前序输出已经包含了工具调用结果）

```go
// 简化调用 - 不传递 contentBlocks
a2aInput := es.buildA2AInput(ctx, threadID, currentConfig, a2aCtx, output, nil, sessionStrategy)
```

### Step 4: 验证构建

```bash
cd D:/CoLinkProject/Colink-0412/isdp
go build ./cmd/server
```

Expected: 构建成功

### Step 5: Commit

```bash
git add internal/service/agent/execution_service.go
git commit -m "feat(agent): buildA2AInput 改进 - 添加会话策略信息、结构化过滤前序输出

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: 后端 - 修改 formatFullPrompt 区分 A2A 输入

**Files:**
- Modify: `internal/service/agent/execution_service.go`

### Step 1: 找到 formatFullPrompt 函数中的用户输入部分

在 `formatFullPrompt` 函数（约 2497-2501 行）找到用户输入格式化部分：

```go
// 用户输入
sb.WriteString("<user>\n")
sb.WriteString(input)
sb.WriteString("\n</user>\n")
```

### Step 2: 修改为区分 A2A 输入

替换用户输入部分为：

```go
// 用户输入部分：区分 A2A 输入和普通输入
// A2A 输入包含 "## 会话策略" 或 "## 前序分析" 特征
if strings.Contains(input, "## 会话策略") || strings.Contains(input, "## 前序分析") {
    sb.WriteString("<a2a_input>\n")
    sb.WriteString(input)
    sb.WriteString("\n</a2a_input>\n")
} else {
    sb.WriteString("<user>\n")
    sb.WriteString(input)
    sb.WriteString("\n</user>\n")
}
```

### Step 3: 验证构建

```bash
cd D:/CoLinkProject/Colink-0412/isdp
go build ./cmd/server
```

Expected: 构建成功

### Step 4: Commit

```bash
git add internal/service/agent/execution_service.go
git commit -m "feat(agent): formatFullPrompt 区分 A2A 输入标签

使用 <a2a_input> 标签标记 A2A 输入，便于前端区分显示

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: 前端 - 新增 parseA2AInput 解析函数

**Files:**
- Modify: `web/src/components/thread/StatusPanel/AgentInvocationLogPanel.tsx`

### Step 1: 在文件顶部添加 A2AInputInfo 类型定义

在 import 语句之后添加：

```tsx
// A2A 输入解析结果
interface A2AInputInfo {
  isA2A: boolean;
  triggerInfo: string;      // 触发者信息
  sessionStrategy: string;  // 会话策略类型
  originalRequest: string;  // 原始用户请求
  filteredOutput: string;   // 过滤后的前序输出
}
```

### Step 2: 在组件内部添加 parseA2AInput 函数

在 `AgentInvocationLogPanel` 组件内部，render 之前添加解析函数：

```tsx
// 解析 A2A 输入信息
const parseA2AInput = (fullPrompt: string): A2AInputInfo | null => {
  // 检查是否包含 <a2a_input> 标签
  const a2aMatch = fullPrompt.match(/<a2a_input>([\s\S]*?)<\/a2a_input>/);
  if (!a2aMatch) return null;

  const a2aContent = a2aMatch[1];

  // 解析触发者信息
  const triggerMatch = a2aContent.match(/\*\*来自\*\*:\s*(.+)/);
  
  // 解析会话策略
  const strategyMatch = a2aContent.match(/\*\*类型\*\*:\s*(.+)/);
  
  // 解析原始请求
  const requestMatch = a2aContent.match(/## 原始请求\s+([\s\S]*?)---/);
  
  // 解析前序输出摘要（在 "## 前序分析" 和下一个 "---" 之间）
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

### Step 3: 验证前端构建

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run build
```

Expected: 构建成功

### Step 4: Commit

```bash
git add web/src/components/thread/StatusPanel/AgentInvocationLogPanel.tsx
git commit -m "feat(frontend): AgentInvocationLogPanel 新增 parseA2AInput 解析函数

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 6: 前端 - 修改 AgentInvocationLogPanel 显示逻辑

**Files:**
- Modify: `web/src/components/thread/StatusPanel/AgentInvocationLogPanel.tsx`

### Step 1: 添加 Collapse 组件 import

检查是否已导入 Collapse，如果没有则添加：

```tsx
import { Collapse, Tag, Typography, Button, Space, Tooltip, message } from 'antd';
```

### Step 2: 在渲染区域判断并区分显示 A2A 输入

找到输入显示区域（约 113-160 行），修改为区分显示：

找到类似以下代码：
```tsx
const hasFullPrompt = inv.fullPrompt && inv.fullPrompt.length > 0;
// ...
{hasFullPrompt ? '完整提示词' : '用户输入'}
```

替换为：
```tsx
const hasFullPrompt = inv.fullPrompt && inv.fullPrompt.length > 0;
const a2aInfo = hasFullPrompt ? parseA2AInput(inv.fullPrompt) : null;
const isA2AInput = a2aInfo !== null;

// 在显示标签部分
{isA2AInput ? (
  <Tag color="blue">A2A 输入</Tag>
) : (
  <Tag>{hasFullPrompt ? '完整提示词' : '用户输入'}</Tag>
)}
```

### Step 3: 在内容显示区域添加 A2A 信息展示

在显示内容部分添加 A2A 信息的折叠展示：

```tsx
{isA2AInput && a2aInfo ? (
  <div className="a2a-input-section">
    <div className="a2a-meta">
      <span>触发者: {a2aInfo.triggerInfo}</span>
      <span>会话策略: {a2aInfo.sessionStrategy}</span>
    </div>
    <Collapse ghost size="small">
      <Collapse.Panel key="request" header="原始请求">
        <pre className="a2a-content">{a2aInfo.originalRequest}</pre>
      </Collapse.Panel>
      <Collapse.Panel key="output" header="前序输出摘要">
        <pre className="a2a-content">{a2aInfo.filteredOutput}</pre>
      </Collapse.Panel>
    </Collapse>
  </div>
) : (
  // 保持原有显示逻辑
  <div className={hasFullPrompt ? 'full-prompt-content' : 'user-input'}>
    {/* ... 原有代码 */}
  </div>
)}
```

### Step 4: 验证前端构建

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run build
```

Expected: 构建成功

### Step 5: Commit

```bash
git add web/src/components/thread/StatusPanel/AgentInvocationLogPanel.tsx
git commit -m "feat(frontend): AgentInvocationLogPanel 区分显示 A2A 输入信息

显示触发者、会话策略、原始请求、前序输出摘要

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 7: 前端 - 新增 A2A 输入样式

**Files:**
- Create: `web/src/components/thread/StatusPanel/AgentInvocationLogPanel.css`（如果不存在）
- 或 Modify: 现有样式文件

### Step 1: 检查样式文件位置

```bash
ls -la web/src/components/thread/StatusPanel/*.css
```

如果存在样式文件，直接修改；如果不存在，创建新文件。

### Step 2: 添加 A2A 输入样式

在样式文件中添加：

```css
/* A2A 输入区域样式 */
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
  margin-bottom: 12px;
  font-size: 12px;
}

.a2a-content {
  background: var(--bg-elevated);
  padding: 8px;
  border-radius: 4px;
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  font-size: 12px;
}

/* 深色模式适配 */
[data-theme='dark'] .a2a-input-section {
  background: rgba(255, 255, 255, 0.04);
}

[data-theme='dark'] .a2a-content {
  background: rgba(255, 255, 255, 0.06);
}

/* Collapse 样式调整 */
.a2a-input-section .ant-collapse {
  background: transparent;
}

.a2a-input-section .ant-collapse-header {
  padding: 8px 0 !important;
  font-size: 13px;
}
```

### Step 3: 确保样式文件被导入

在 `AgentInvocationLogPanel.tsx` 中确保导入样式：

```tsx
import './AgentInvocationLogPanel.css'; // 添加这行（如果文件名不同则调整）
```

### Step 4: 验证前端构建

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run build
```

Expected: 构建成功

### Step 5: Commit

```bash
git add web/src/components/thread/StatusPanel/AgentInvocationLogPanel.css
git commit -m "style(frontend): AgentInvocationLogPanel A2A 输入样式

深色模式适配，使用 CSS 变量

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 8: 验证与测试

**Files:**
- 无新文件

### Step 1: 启动后端服务

```bash
cd D:/CoLinkProject/Colink-0412/isdp
go run ./cmd/server
```

Expected: 服务启动成功

### Step 2: 启动前端开发服务器

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run dev
```

Expected: 前端启动成功

### Step 3: 手动测试 A2A 流程

1. 打开浏览器访问前端
2. 创建一个包含多个 Agent 的工作流
3. 发起任务让第一个 Agent 执行
4. 等待第一个 Agent 完成并 @ 下一个 Agent
5. 查看第二个 Agent 的调用日志：
   - 确认输入区域显示 "A2A 输入" 标签
   - 确认显示触发者信息
   - 确认显示会话策略
   - 确认展开查看原始请求和前序输出摘要

### Step 4: 检查深色模式

切换深色主题，确认 A2A 输入区域样式正常显示。

### Step 5: 最终 Commit（如果需要调整）

如果有任何修复：

```bash
git add -A
git commit -m "fix: A2A 传输优化测试修复

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## 总结

改动文件：
- `internal/service/agent/execution_service.go` - 4处修改
- `web/src/components/thread/StatusPanel/AgentInvocationLogPanel.tsx` - 2处修改
- `web/src/components/thread/StatusPanel/AgentInvocationLogPanel.css` - 新增样式

关键改动：
- A2A 调用跳过 Layer1（历史消息）传递
- 前序输出结构化过滤（文件路径、关键结论、工具结果）
- 前端区分显示 A2A 输入信息