# CLI Output 展示优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 CLI Output Block 三层折叠设计，工具调用行显示结构化摘要，stdout 预览，自动折叠 UX。

**Architecture:** 修改现有 ToolBlock.tsx 和 ToolGroupBlock 组件，添加工具摘要生成函数，在 ToolGroupBlock 中集成 stdout 预览和三层折叠逻辑，保持现有 CSS 样式风格但优化视觉层级。

**Tech Stack:** React, TypeScript, Ant Design, CSS Variables

---

## 文件结构

| 文件 | 负责内容 | 操作 |
|------|----------|------|
| `web/src/components/thread/ContentBlock/ToolBlock.tsx` | 工具摘要生成函数 + 截断辅助函数 | 修改 |
| `web/src/components/thread/ContentBlock/MessageContentRenderer.tsx` | ToolGroupBlock 三层折叠 + stdout 预览 + 自动折叠 | 修改 |
| `web/src/components/thread/ContentBlock/ContentBlock.css` | 深色背景 + 视觉层级样式 | 修改 |

---

## Task 1: 工具摘要生成函数

**Files:**
- Modify: `web/src/components/thread/ContentBlock/ToolBlock.tsx`

**Goal:** 为每种工具类型定义专门的摘要格式，实现结构化显示。

### Step 1.1: 添加 ToolSummary 接口和辅助函数

在 `ToolBlock.tsx` 文件顶部（import 之后）添加：

```typescript
/**
 * 工具摘要结构
 */
interface ToolSummary {
  name: string;       // 工具名
  param: string;      // 关键参数摘要
}

/**
 * 路径截断（保留文件名和关键目录）
 */
function truncatePath(path: string, maxLen = 50): string {
  if (!path) return '';
  if (path.length <= maxLen) return path;

  const parts = path.split('/');
  const fileName = parts.pop() || '';

  if (parts.length === 0) {
    return fileName.length > maxLen ? `...${fileName.slice(-maxLen + 3)}` : fileName;
  }

  if (parts.length >= 2 && fileName.length + parts[0].length + parts[1].length + 10 <= maxLen) {
    return `${parts[0]}/${parts[1]}/.../${fileName}`;
  }

  return fileName.length > maxLen - 3 ? `...${fileName.slice(-maxLen + 3)}` : `.../${fileName}`;
}

/**
 * 命令截断
 */
function truncateCommand(cmd: string, maxLen = 40): string {
  if (!cmd) return '';
  if (cmd.length <= maxLen) return cmd;

  const firstWord = cmd.split(' ')[0];
  if (firstWord.length >= maxLen) {
    return `${firstWord.slice(0, maxLen - 3)}...`;
  }

  return `${cmd.slice(0, maxLen - 3)}...`;
}

/**
 * 描述截断
 */
function truncateDescription(desc: string, maxLen = 25): string {
  if (!desc) return '';
  if (desc.length <= maxLen) return desc;
  return `${desc.slice(0, maxLen)}...`;
}

/**
 * URL 截断
 */
function truncateUrl(url: string, maxLen = 45): string {
  if (!url) return '';
  if (url.length <= maxLen) return url;

  try {
    const parsed = new URL(url);
    const domain = parsed.hostname;
    const path = parsed.pathname.slice(0, 20);
    const result = `${domain}${path}${parsed.pathname.length > 20 ? '...' : ''}`;
    return result.length > maxLen ? result.slice(0, maxLen - 3) + '...' : result;
  } catch {
    return `${url.slice(0, maxLen - 3)}...`;
  }
}

/**
 * 查询截断
 */
function truncateQuery(query: string, maxLen = 30): string {
  if (!query) return '';
  if (query.length <= maxLen) return query;
  return `${query.slice(0, maxLen)}...`;
}

/**
 * 问题截断
 */
function truncateQuestion(question: string, maxLen = 20): string {
  if (!question) return '';
  if (question.length <= maxLen) return question;
  return `${question.slice(0, maxLen)}...`;
}
```

- [ ] **Step 1.1: 添加 ToolSummary 接口和截断辅助函数**

打开 `web/src/components/thread/ContentBlock/ToolBlock.tsx`，在 import 语句之后添加上述代码。

---

### Step 1.2: 添加 generateToolSummary 函数

在截断函数之后添加：

```typescript
/**
 * 生成工具摘要
 * 根据工具类型返回结构化的名称和参数摘要
 */
function generateToolSummary(toolName: string, input?: Record<string, unknown>): ToolSummary {
  switch (toolName) {
    case 'Read':
    case 'Write':
      return {
        name: toolName,
        param: truncatePath(input?.file_path as string),
      };
    case 'Edit':
      return {
        name: toolName,
        param: `${truncatePath(input?.file_path as string)}:${input?.start_line}-${input?.end_line}`,
      };
    case 'Bash':
      return {
        name: toolName,
        param: truncateCommand(input?.command as string),
      };
    case 'Grep':
      return {
        name: toolName,
        param: `"${input?.pattern}" in ${truncatePath(input?.path as string)}`,
      };
    case 'Glob':
      return {
        name: toolName,
        param: input?.pattern as string || '',
      };
    case 'Skill':
      return {
        name: toolName,
        param: (input?.skill as string) || (input?.args as string) || 'unknown',
      };
    case 'Task':
      const subagent = (input?.subagent_name as string) || (input?.subagent_type as string) || 'agent';
      return {
        name: toolName,
        param: `@${subagent} ${truncateDescription((input?.description as string) || '')}`,
      };
    case 'NotebookEdit':
      return {
        name: toolName,
        param: `${truncatePath(input?.notebook_path as string)}[${input?.cell_number}]`,
      };
    case 'WebFetch':
      return {
        name: toolName,
        param: truncateUrl(input?.url as string),
      };
    case 'WebSearch':
      return {
        name: toolName,
        param: `"${truncateQuery(input?.query as string)}"`,
      };
    case 'AskUserQuestion':
      return {
        name: 'Ask',
        param: truncateQuestion((input?.questions as any[])?.[0]?.question as string),
      };
    case 'TodoWrite':
      return {
        name: toolName,
        param: `${(input?.todos as any[])?.length || 0} items`,
      };
    case 'EnterPlanMode':
    case 'ExitPlanMode':
    case 'ScheduleWakeup':
    case 'CronCreate':
    case 'CronDelete':
    case 'CronList':
      return { name: toolName, param: '' };
    default:
      return generateDefaultSummary(toolName, input);
  }
}

/**
 * 默认摘要生成（提取首个关键参数）
 */
function generateDefaultSummary(toolName: string, input?: Record<string, unknown>): ToolSummary {
  const keys = ['file_path', 'command', 'pattern', 'url', 'query', 'path', 'content', 'name', 'id'];
  for (const key of keys) {
    const val = input?.[key];
    if (typeof val === 'string' && val.length > 0) {
      return {
        name: toolName,
        param: truncatePath(val, 40),
      };
    }
  }
  return { name: toolName, param: '' };
}
```

- [ ] **Step 1.2: 添加 generateToolSummary 函数**

在 Step 1.1 添加的截断函数之后添加 generateToolSummary 和 generateDefaultSummary 函数。

---

### Step 1.3: 导出摘要函数供其他组件使用

在文件末尾的 export 语句中添加：

```typescript
// 在现有 export 语句后添加
export { generateToolSummary, truncatePath, truncateCommand, truncateDescription, truncateUrl, truncateQuery };
export type { ToolSummary };
```

- [ ] **Step 1.3: 导出摘要函数**

修改文件末尾的 export 语句，导出新增的函数和类型。

---

### Step 1.4: 修改 ToolCallRow 使用结构化摘要

修改 `ToolCallRow` 组件的实现，替换原有的参数提取逻辑：

找到以下代码块（约第175-184行）：

```typescript
// 提取主要参数
const primaryArgKeys = ['file_path', 'command', 'pattern', 'url', 'query', 'path', 'content'];
let primaryArg = '';
for (const key of primaryArgKeys) {
  const val = input?.[key];
  if (typeof val === 'string' && val.length > 0) {
    primaryArg = val.length > 60 ? `${val.slice(0, 60)}...` : val;
    break;
  }
}
```

替换为：

```typescript
// 使用结构化摘要
const summary = generateToolSummary(toolName, input);
```

- [ ] **Step 1.4: 修改 ToolCallRow 参数提取逻辑**

替换原有的 primaryArgKeys 循环为 generateToolSummary 调用。

---

### Step 1.5: 更新 ToolCallRow 渲染逻辑

修改渲染部分，使用 summary.name 和 summary.param：

找到以下代码块（约第211-224行）：

```typescript
{/* 工具名称 */}
<span
  className="tool-call-name"
  style={{ color: status === 'streaming' ? accentColor : undefined }}
>
  {toolName}
</span>

{/* 主要参数 */}
{primaryArg && (
  <span className="tool-call-input">
    {primaryArg}
  </span>
)}
```

替换为：

```typescript
{/* 工具名称 */}
<span
  className="tool-call-name"
  style={{ color: status === 'streaming' ? accentColor : undefined }}
>
  {summary.name}
</span>

{/* 关键参数 */}
{summary.param && (
  <span
    className="tool-call-param"
    style={{ color: status === 'streaming' ? '#C084FC' : '#64748B' }}
  >
    {summary.param}
  </span>
)}
```

- [ ] **Step 1.5: 更新 ToolCallRow 渲染**

替换渲染部分，使用 summary.name 和 summary.param，并调整参数样式颜色。

---

### Step 1.6: 验证工具摘要显示

- [ ] **Step 1.6: 验证工具摘要显示**

在终端运行：

```bash
cd web && npm run dev
```

打开浏览器，创建一个 Agent 对话，执行包含以下工具的操作：
- Read 文件
- Bash 命令
- Skill 调用

预期结果：工具行显示格式化的摘要（如 `Read src/api/handler.go`），参数颜色为灰色 #64748B。

---

## Task 2: stdout 预览功能

**Files:**
- Modify: `web/src/components/thread/ContentBlock/MessageContentRenderer.tsx`

**Goal:** 在 CLI Output Block 摘要行显示 stdout 前 48 字符预览。

### Step 2.1: 添加 stdout 预览函数

在 `MessageContentRenderer.tsx` 文件中，找到 `formatDuration` 函数（约第230行），在其后添加：

```typescript
/** stdout 预览最大字符数 */
const STDOUT_PREVIEW_MAX_CHARS = 48;

/**
 * 构建 stdout 预览
 * 从 text 块中提取前 48 字符
 */
function buildStdoutPreview(blocks: MessageContentBlock[]): string {
  let preview = '';
  for (const block of blocks) {
    if (block.type !== 'text') continue;
    const content = (block as TextBlockType).content || '';
    for (const char of content) {
      // 空格压缩为单个空格
      if (/\s/.test(char)) {
        preview = preview && !preview.endsWith(' ') ? `${preview} ` : preview;
      } else {
        preview = `${preview}${char}`;
      }
      if (preview.length > STDOUT_PREVIEW_MAX_CHARS) {
        return `${preview.slice(0, STDOUT_PREVIEW_MAX_CHARS)}…`;
      }
    }
  }
  return preview.trimEnd();
}

/**
 * 从工具组中提取 stdout 内容块
 */
function extractStdoutBlocks(blocks: MessageContentBlock[]): TextBlockType[] {
  return blocks.filter(b => b.type === 'text') as TextBlockType[];
}
```

- [ ] **Step 2.1: 添加 stdout 预览函数**

在 formatDuration 函数后添加 stdout 预览相关函数。

---

### Step 2.2: 修改 aggregateBlocks 保留 stdout 信息

修改 `aggregateBlocks` 函数，使其返回聚合结果中包含 stdout 信息：

找到函数签名（约第177行）：

```typescript
function aggregateBlocks(blocks: MessageContentBlock[]): Array<MessageContentBlock | { type: 'tool_use_group'; tools: ToolUseBlock[]; richBlocks?: RichBlock[] }>
```

修改返回类型为：

```typescript
function aggregateBlocks(blocks: MessageContentBlock[]): Array<MessageContentBlock | { type: 'tool_use_group'; tools: ToolUseBlock[]; stdoutBlocks?: TextBlockType[]; richBlocks?: RichBlock[] }>
```

- [ ] **Step 2.2: 修改 aggregateBlocks 返回类型**

更新 aggregateBlocks 函数的返回类型定义。

---

### Step 2.3: 修改 aggregateBlocks 逻辑收集 stdout

修改 aggregateBlocks 函数内部逻辑，收集 stdout 块：

找到循环体（约第182-210行），修改为：

```typescript
function aggregateBlocks(blocks: MessageContentBlock[]): Array<MessageContentBlock | { type: 'tool_use_group'; tools: ToolUseBlock[]; stdoutBlocks?: TextBlockType[]; richBlocks?: RichBlock[] }> {
  const result: Array<MessageContentBlock | { type: 'tool_use_group'; tools: ToolUseBlock[]; stdoutBlocks?: TextBlockType[]; richBlocks?: RichBlock[] }> = [];
  let currentToolGroup: ToolUseBlock[] = [];
  let currentStdoutBlocks: TextBlockType[] = [];
  let currentRichBlocks: RichBlock[] = [];

  for (const block of blocks) {
    if (block.type === 'tool_use') {
      // 累积 tool_use 块
      currentToolGroup.push(block as ToolUseBlock);
    } else if (block.type === 'text') {
      // 判断是否属于工具组的 stdout（紧跟在 tool_use 之后）
      if (currentToolGroup.length > 0) {
        currentStdoutBlocks.push(block as TextBlockType);
      } else {
        // 独立的 text 块，直接输出
        result.push(block);
      }
    } else if (block.type === 'rich') {
      // 累积 rich 块
      currentRichBlocks.push(block as RichBlock);
    } else {
      // 遇到非 tool_use/rich/text 块，先输出累积的块
      if (currentToolGroup.length > 0) {
        result.push({
          type: 'tool_use_group',
          tools: currentToolGroup,
          stdoutBlocks: currentStdoutBlocks,
          richBlocks: currentRichBlocks.length > 0 ? currentRichBlocks : undefined,
        });
        currentToolGroup = [];
        currentStdoutBlocks = [];
        currentRichBlocks = [];
      }
      result.push(block);
    }
  }

  // 处理末尾的累积块
  if (currentToolGroup.length > 0) {
    result.push({
      type: 'tool_use_group',
      tools: currentToolGroup,
      stdoutBlocks: currentStdoutBlocks,
      richBlocks: currentRichBlocks.length > 0 ? currentRichBlocks : undefined,
    });
  }

  return result;
}
```

- [ ] **Step 2.3: 修改 aggregateBlocks 逻辑**

更新 aggregateBlocks 函数内部逻辑，收集 stdout 块并处理独立 text 块。

---

### Step 2.4: 修改 ToolGroupBlock 显示 stdout 预览

修改 `ToolGroupBlock` 组件，添加 stdout 预览和三层折叠：

找到 `ToolGroupBlockProps` 接口（约第284行），修改为：

```typescript
interface ToolGroupBlockProps {
  tools: ToolUseBlock[];
  stdoutBlocks?: TextBlockType[];
  defaultExpanded?: boolean;
}
```

- [ ] **Step 2.4: 修改 ToolGroupBlockProps 接口**

添加 stdoutBlocks 属性。

---

### Step 2.5: 修改 ToolGroupBlock 组件实现

找到 `ToolGroupBlock` 组件实现（约第288行），替换为：

```typescript
const ToolGroupBlock: React.FC<ToolGroupBlockProps> = memo(({ tools, stdoutBlocks = [], defaultExpanded = false }) => {
  if (tools.length === 0) return null;

  const anyStreaming = tools.some(t => t.status === 'streaming');
  const anyFailed = tools.some(t => t.status === 'failed');
  const totalDuration = tools.reduce((sum, t) => sum + (t.duration || 0), 0);

  // 三层折叠状态
  const [blockExpanded, setBlockExpanded] = useState(anyStreaming || defaultExpanded);
  const [toolsExpanded, setToolsExpanded] = useState(anyStreaming);
  
  // 用户交互追踪
  const userInteracted = React.useRef(false);
  const toolsUserInteracted = React.useRef(false);

  // streaming 时自动展开 CLI Block
  useEffect(() => {
    if (anyStreaming && !blockExpanded) {
      setBlockExpanded(true);
      setToolsExpanded(true);
      userInteracted.current = false;
      toolsUserInteracted.current = false;
    }
  }, [anyStreaming]);

  // 完成后自动折叠
  const prevStreamingRef = React.useRef(anyStreaming);
  useEffect(() => {
    if (prevStreamingRef.current && !anyStreaming) {
      if (!userInteracted.current) {
        setBlockExpanded(defaultExpanded);
      }
      if (!toolsUserInteracted.current) {
        setToolsExpanded(false);
      }
    }
    prevStreamingRef.current = anyStreaming;
  }, [anyStreaming, defaultExpanded]);

  // 构建摘要行
  const toolCount = tools.length;
  const statusText = anyStreaming ? 'running' : anyFailed ? 'failed' : 'completed';
  const stdoutPreview = buildStdoutPreview(stdoutBlocks);
  
  let summary = `CLI 输出 · ${statusText} · ${toolCount} 工具`;
  if (totalDuration > 0 && !anyStreaming) {
    summary += ` · ${formatDuration(totalDuration)}`;
  }
  if (stdoutPreview && !blockExpanded) {
    summary += ` · stdout: ${stdoutPreview}`;
  }

  const accentColor = '#7C3AED';
  const surfaceColor = '#1A1625';  // 深色背景

  const handleBlockToggle = () => {
    userInteracted.current = true;
    setBlockExpanded(v => !v);
  };

  const handleToolsToggle = () => {
    toolsUserInteracted.current = true;
    setToolsExpanded(v => !v);
  };

  return (
    <div className="cli-output-block" style={{ marginTop: 8, backgroundColor: surfaceColor, borderRadius: 8, overflow: 'hidden' }}>
      {/* 第 1 层：CLI Block Header */}
      <button
        type="button"
        onClick={handleBlockToggle}
        className="cli-output-header"
        style={{
          width: '100%',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          padding: '8px 12px',
          fontSize: 11,
          fontFamily: 'JetBrains Mono, monospace',
          color: '#94A3B8',
          backgroundColor: surfaceColor,
          border: 'none',
          cursor: 'pointer',
        }}
      >
        <ChevronIcon expanded={blockExpanded} color={accentColor} />
        <WrenchIcon color="#6B7280" />
        <span style={{ fontWeight: 500 }}>{summary}</span>
      </button>

      {/* 第 2 层：展开后显示 tools 区 + stdout 区 */}
      {blockExpanded && (
        <div style={{ backgroundColor: '#1F1B2E' }}>
          <div style={{ height: 1, backgroundColor: '#334155' }} />
          
          {/* tools 区（第 2 层可独立折叠） */}
          {toolCount > 0 && (
            <div style={{ padding: '4px 12px' }}>
              <button
                type="button"
                onClick={handleToolsToggle}
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                  padding: '4px 8px',
                  fontSize: 11,
                  fontFamily: 'inherit',
                  color: '#94A3B8',
                  backgroundColor: 'transparent',
                  border: 'none',
                  cursor: 'pointer',
                  borderRadius: 4,
                }}
              >
                <ChevronIcon expanded={toolsExpanded} color={accentColor} />
                <span>{toolsExpanded ? `${toolCount} 工具` : `${toolCount} 工具 (已折叠)`}</span>
              </button>
              
              {/* 第 3 层：展开显示工具列表 */}
              {toolsExpanded && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 2, marginTop: 4 }}>
                  {tools.map((tool, index) => (
                    <ToolCallRow
                      key={tool.id || `tool-${index}`}
                      toolName={tool.toolName}
                      input={tool.input}
                      output={tool.output}
                      status={tool.status}
                      duration={tool.duration}
                      startedAt={tool.startedAt}
                      defaultExpanded={tool.status === 'streaming'}
                      accentColor={accentColor}
                    />
                  ))}
                </div>
              )}
            </div>
          )}

          {/* stdout 区（始终可见） */}
          {stdoutBlocks.length > 0 && (
            <>
              {toolCount > 0 && <div style={{ height: 1, backgroundColor: '#334155' }} />}
              <div style={{ padding: '8px 12px 4px 12px', fontSize: 10, fontFamily: 'JetBrains Mono, monospace', color: '#475569' }}>
                ─── stdout ───
              </div>
              <div style={{ padding: '8px 12px 10px 12px', fontSize: 11, fontFamily: 'inherit', color: '#CBD5E1', lineHeight: 1.5 }}>
                {stdoutBlocks.map((block, i) => (
                  <React.Fragment key={i}>
                    {block.content}
                    {i < stdoutBlocks.length - 1 && '\n'}
                  </React.Fragment>
                ))}
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
});

ToolGroupBlock.displayName = 'ToolGroupBlock';
```

- [ ] **Step 2.5: 修改 ToolGroupBlock 组件**

替换 ToolGroupBlock 组件实现，添加三层折叠和 stdout 显示逻辑。

---

### Step 2.6: 更新渲染逻辑传递 stdoutBlocks

修改渲染逻辑，传递 stdoutBlocks 给 ToolGroupBlock：

找到渲染部分（约第91-98行）：

```typescript
case 'tool_use_group':
  return (
    <ToolGroupBlock
      key={`tool-group-${index}`}
      tools={block.tools as ToolUseBlock[]}
      defaultExpanded={defaultExpanded}
    />
  );
```

修改为：

```typescript
case 'tool_use_group':
  return (
    <ToolGroupBlock
      key={`tool-group-${index}`}
      tools={block.tools as ToolUseBlock[]}
      stdoutBlocks={block.stdoutBlocks as TextBlockType[]}
      defaultExpanded={defaultExpanded}
    />
  );
```

- [ ] **Step 2.6: 更新渲染逻辑传递 stdoutBlocks**

修改 tool_use_group case，传递 stdoutBlocks 属性。

---

### Step 2.7: 验证 stdout 预览显示

- [ ] **Step 2.7: 验证 stdout 预览显示**

在浏览器中测试：
- Agent 执行工具后，CLI Block 摘要行应显示 stdout 前 48 字符
- 展开 CLI Block 后，stdout 区应始终可见
- tools 区可独立折叠

预期结果：
- 折叠状态：`CLI 输出 · completed · 3 工具 · stdout: 重构完成，所有测试通过...`
- 展开状态：stdout 区显示完整内容

---

## Task 3: CSS 样式更新

**Files:**
- Modify: `web/src/components/thread/ContentBlock/ContentBlock.css`

**Goal:** 添加深色背景样式和视觉层级样式。

### Step 3.1: 添加 CLI Output Block 样式

在 `ContentBlock.css` 文件末尾添加：

```css
/* ========== CLI Output Block 样式 ========== */

.cli-output-block {
  margin: 8px 0;
  font-family: 'JetBrains Mono', 'SF Mono', 'Consolas', monospace;
}

.cli-output-header:hover {
  background-color: rgba(124, 58, 237, 0.1) !important;
}

/* 工具参数样式（视觉层级） */
.tool-call-param {
  color: #64748B;
  font-size: 11px;
  margin-left: 4px;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* streaming 状态下参数高亮 */
.tool-call-row.streaming .tool-call-param {
  color: #C084FC;
}

/* stdout 区域样式 */
.cli-output-stdout {
  padding: 8px 12px 10px 12px;
  font-size: 11px;
  color: #CBD5E1;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

/* stdout 标题分隔线 */
.cli-output-stdout-divider {
  padding: 8px 12px 4px 12px;
  font-size: 10px;
  font-family: 'JetBrains Mono', monospace;
  color: #475569;
}
```

- [ ] **Step 3.1: 添加 CLI Output Block 样式**

在 ContentBlock.css 文件末尾添加上述样式。

---

### Step 3.2: 添加深色主题适配

在刚添加的样式之后添加深色主题适配：

```css
/* ========== CLI Output Block 深色主题适配 ========== */

[data-theme='dark'] .cli-output-block {
  border-color: var(--border-color);
}

[data-theme='dark'] .cli-output-header {
  color: var(--text-secondary) !important;
}

[data-theme='dark'] .cli-output-header:hover {
  background-color: var(--color-primary-opacity-10) !important;
}

[data-theme='dark'] .tool-call-param {
  color: var(--text-secondary);
}

[data-theme='dark'] .tool-call-row.streaming .tool-call-param {
  color: var(--color-primary-light);
}

[data-theme='dark'] .cli-output-stdout {
  color: var(--text-primary);
}

[data-theme='dark'] .cli-output-stdout-divider {
  color: var(--text-secondary);
}
```

- [ ] **Step 3.2: 添加深色主题适配**

添加深色主题下的样式适配。

---

### Step 3.3: 验证样式显示

- [ ] **Step 3.3: 验证样式显示**

在浏览器中：
1. 测试浅色主题下的 CLI Block 显示
2. 切换到深色主题，验证样式适配

预期结果：CLI Block 显示深色背景，工具参数为灰色，streaming 状态参数高亮。

---

## Task 4: 自动折叠 UX 完善

**Files:**
- Modify: `web/src/components/thread/ContentBlock/MessageContentRenderer.tsx`

**Goal:** 完善自动折叠逻辑，确保 streaming → done 正确折叠。

### Step 4.1: 添加 useLayoutEffect 触发布局事件

在 ToolGroupBlock 组件中，添加布局变化事件触发：

在 `blockExpanded` 状态定义之后添加：

```typescript
// 触发布局变化事件（用于滚动控制）
const hasMounted = React.useRef(false);
React.useLayoutEffect(() => {
  if (!hasMounted.current) {
    hasMounted.current = true;
    return;
  }
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event('cli-output-layout-changed'));
  }
}, [blockExpanded]);
```

- [ ] **Step 4.1: 添加布局变化事件**

在 ToolGroupBlock 中添加 useLayoutEffect 触发布局变化事件。

---

### Step 4.2: 验证自动折叠行为

- [ ] **Step 4.2: 验证自动折叠行为**

测试场景：
1. Agent 开始执行工具 → CLI Block 自动展开
2. Agent 执行完成 → CLI Block 自动折叠（除非用户点击过）
3. 用户手动展开后 → 保持展开状态

预期结果：
- streaming 时自动展开
- done 时自动折叠
- 用户操作后不自动折叠

---

## Task 5: 集成测试和验收

### Step 5.1: 功能验收测试

- [ ] **Step 5.1: 功能验收测试**

执行以下测试场景：

**P0 核心痛点验收：**

1. **工具行摘要**：
   - 创建对话，让 Agent 执行 Read 文件
   - 验证：工具行显示 `Read src/xxx/file.go`
   - 让 Agent 执行 Bash 命令
   - 验证：工具行显示 `Bash npm run xxx`
   - 让 Agent 执行 Skill
   - 验证：工具行显示 `Skill xxx`

2. **CLI Block 三层折叠**：
   - 执行多个工具
   - 验证：摘要行显示工具数量 + 耗时 + stdout 预览
   - 点击展开 CLI Block
   - 验证：tools 区可独立折叠，stdout 区始终可见
   - 点击单个工具行
   - 验证：可展开看 input/output

3. **stdout 预览**：
   - 执行产生 stdout 输出的操作
   - 验证：折叠状态显示前 48 字符预览

4. **自动折叠 UX**：
   - 执行工具期间观察
   - 验证：streaming 时自动展开
   - 执行完成后观察
   - 验证：done 后自动折叠（未操作时）
   - 手动展开后再次执行
   - 验证：用户操作后不自动折叠

---

### Step 5.2: 视觉验收

- [ ] **Step 5.2: 视觉验收**

检查视觉层级：
- 工具名加粗，颜色为 #374151
- 参数灰色 #64748B，字号 11px
- streaming 时工具名和参数高亮紫色

---

### Step 5.3: 提交代码

- [ ] **Step 5.3: 提交代码**

```bash
git add web/src/components/thread/ContentBlock/ToolBlock.tsx web/src/components/thread/ContentBlock/MessageContentRenderer.tsx web/src/components/thread/ContentBlock/ContentBlock.css
git commit -m "feat: CLI Output 展示优化

- 工具调用行显示结构化摘要（工具名 + 关键参数）
- CLI Output Block 三层折叠（整体→tools区→单个工具）
- stdout 预览显示前48字符
- 自动折叠 UX（streaming 展开 → done 自动折叠）
- 深色背景视觉风格

参考: cat-cafe F097 设计

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## 参考资源

| 文件 | 用途 |
|------|------|
| `/Users/cc/Workspace/Code/cat-cafe/clowder-ai/packages/web/src/components/cli-output/CliOutputBlock.tsx` | cat-cafe CLI Output Block 参考 |
| `docs/tool-call-summary-optimization-20260508.md` | 规格文档 |
| `web/src/components/thread/ContentBlock/ToolBlock.tsx` | 当前工具块实现 |
| `web/src/components/thread/ContentBlock/MessageContentRenderer.tsx` | 当前消息内容渲染器 |

---

## Self-Review 检查清单

- [ ] 规格文档所有 P0 要求都有对应任务覆盖
- [ ] 无 TBD/TODO/占位符
- [ ] 类型定义一致（ToolSummary、TextBlockType）
- [ ] 所有代码步骤都有完整代码块
- [ ] 测试验收标准明确