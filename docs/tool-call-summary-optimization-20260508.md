# CLI Output 展示优化设计文档

## 背景

用户反馈：colink 项目中工具调用展示不够直观，需要展开才能看到关键信息。对比 cat-cafe 项目，其 CLI Output Block 采用三层折叠设计，工具行直接显示结构化摘要，stdout 预览始终可见，无需展开即可了解 Agent 执行了什么。

参考文档：`/Users/cc/Workspace/Code/cat-cafe/clowder-ai/packages/api/uploads/doc-04a5685f83d9-colink-vs-catcafe-对话展示对比分析.md`

## 目标

实现 cat-cafe F097 设计的 CLI Output Block 三层折叠方案：

1. **工具调用行摘要**：无需展开即可看到关键信息（路径、命令、skill名等）
2. **CLI Output Block 三层折叠**：整体→tools区→单个工具
3. **stdout 预览**：折叠状态显示前 48 字符
4. **自动折叠 UX**：streaming 展开 → done 自动折叠

## 设计方案

### 方案概述：CLI Output Block 三层折叠设计

参考 cat-cafe F097 设计，CLI Output Block 作为统一折叠单元，包含 tools + stdout。

```
第 1 层：CLI 输出块整体
  ┌─ CLI 输出 · 已完成 · 9 tools · 1m49s  🐾 共享给其他猫  ▶ ─┐
  ↕ 点击展开/折叠

第 2 层：tools 区 + stdout 区
  ┌─ CLI 输出 · 已完成 · 9 tools · 1m49s  🐾 共享给其他猫  ▼ ─┐
  │ ▶ 9 tools（已折叠）          ← 点击可展开全部工具        │
  │ ─── stdout ───                                             │
  │ 重构完成，所有测试通过。主要改动...                        │
  ↕ 点击 "9 tools" 行

第 3 层：展开工具列表（每个工具可独立展开看细节）
  │   ✓ Read src/components/index.ts                       ▶  │
  │   ✓ Bash pnpm test   12 passed                         ▶  │ ← 点击看输入输出
```

### 自动折叠行为

| 阶段 | CLI 输出块 | tools 区 | 用户操作过则 |
|------|-----------|---------|------------|
| streaming（执行中） | 展开 | 展开 | — |
| done（刚完成） | 折叠 | 折叠 | 不自动折叠 |
| done（用户手动展开后） | 展开 | 折叠（只看 stdout） | 保持用户选择 |

### 核心设计决策

| # | 决策 | 理由 |
|---|------|------|
| KD-1 | CLI Output Block 作为统一折叠单元 | 聚合工具调用，一眼看整体 |
| KD-2 | stdout 在块展开时始终可见 | stdout 是执行结果摘要，比工具细节更重要 |
| KD-3 | 深色 terminal substrate | 内层"执行日志"一眼成立，视觉区分明显 |
| KD-4 | 自动折叠 UX | 执行完成后自动折叠，避免历史消息占用大量空间 |

---

## 详细设计

### 1. 工具调用行摘要格式

为每种工具类型定义专门的摘要格式：

| 工具类型 | 摘要格式示例 | 关键字段 |
|---------|-------------|---------|
| Read | `Read src/api/handler.go` | file_path |
| Write | `Write src/config.yaml` | file_path |
| Edit | `Edit handler.go:42-50` | file_path + start_line |
| Bash | `Bash npm run build` | command |
| Grep | `Grep "Handler" in src/` | pattern + path |
| Glob | `Glob src/**/*.tsx` | pattern |
| Skill | `Skill brainstorming` | skill name |
| Task | `Task @planner 设计登录流程` | subagent_name + description |
| NotebookEdit | `NotebookEdit analysis.ipynb[3]` | notebook_path + cell_number |
| WebFetch | `WebFetch https://example.com` | url |
| WebSearch | `WebSearch "Claude API docs"` | query |
| AskUserQuestion | `Ask 选项确认` | question summary |

### 2. 视觉层级

工具调用行的视觉层级设计：

```
[状态图标] [扳手图标] [工具名(加粗)] [参数(次要颜色)] [耗时]
```

- **工具名**：font-weight: 500，使用主要颜色
- **参数**：font-weight: normal，使用次要颜色（如 #64748B）
- **耗时**：显示在右侧，小字号

### 3. 实现方式

#### 3.1 添加摘要生成函数

在 `ToolBlock.tsx` 中添加 `generateToolSummary` 函数：

```typescript
interface ToolSummary {
  name: string;       // 工具名
  param: string;      // 关键参数摘要
  paramDetail?: string; // 参数详情（可选显示）
}

function generateToolSummary(toolName: string, input?: Record<string, unknown>): ToolSummary {
  // 根据工具类型生成摘要
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
        param: input?.pattern as string,
      };
    case 'Skill':
      return {
        name: toolName,
        param: input?.skill as string || 'unknown',
      };
    case 'Task':
      return {
        name: toolName,
        param: `@${input?.subagent_name || 'agent'} ${truncateDescription(input?.description as string)}`,
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
        param: truncateQuestion(input?.questions?.[0]?.question as string),
      };
    case 'TodoWrite':
      return {
        name: toolName,
        param: `${(input?.todos as any[])?.length || 0} items`,
      };
    case 'EnterPlanMode':
    case 'ExitPlanMode':
      return { name: toolName, param: '' };
    default:
      return generateDefaultSummary(toolName, input);
  }
}
```

#### 3.2 修改 ToolCallRow 组件

更新渲染逻辑，使用结构化摘要：

```tsx
// 原代码（提取主要参数）
const primaryArgKeys = ['file_path', 'command', 'pattern', 'url', 'query', 'path', 'content'];
let primaryArg = '';
for (const key of primaryArgKeys) {
  // ...
}

// 新代码（结构化摘要）
const summary = generateToolSummary(toolName, input);

// 渲染
<span className="tool-call-name" style={{ fontWeight: 500 }}>
  {summary.name}
</span>
{summary.param && (
  <span className="tool-call-param" style={{ color: '#64748B', marginLeft: 4 }}>
    {summary.param}
  </span>
)}
```

#### 3.3 添加辅助函数

```typescript
// 路径截断（保留文件名和关键目录）
function truncatePath(path: string, maxLen = 50): string {
  if (!path) return '';
  if (path.length <= maxLen) return path;

  // 优先保留文件名（最后一个 / 后的内容）
  const parts = path.split('/');
  const fileName = parts.pop() || '';

  // 如果只剩文件名且超长，截断文件名
  if (parts.length === 0) {
    return fileName.length > maxLen ? `...${fileName.slice(-maxLen + 3)}` : fileName;
  }

  // 尝试保留：前两级目录 + 文件名
  if (parts.length >= 2 && fileName.length + parts[0].length + parts[1].length + 10 <= maxLen) {
    return `${parts[0]}/${parts[1]}/.../${fileName}`;
  }

  // 退化为只保留文件名
  return fileName.length > maxLen - 3 ? `...${fileName.slice(-maxLen + 3)}` : `.../${fileName}`;
}

// 命令截断（保留命令名和关键参数）
function truncateCommand(cmd: string, maxLen = 40): string {
  if (!cmd) return '';
  if (cmd.length <= maxLen) return cmd;

  // 尝试保留第一个单词（命令名）
  const firstWord = cmd.split(' ')[0];
  if (firstWord.length >= maxLen) {
    return `${firstWord.slice(0, maxLen - 3)}...`;
  }

  // 保留命令名 + 部分参数
  return `${cmd.slice(0, maxLen - 3)}...`;
}

// 描述截断（保留核心意图）
function truncateDescription(desc: string, maxLen = 25): string {
  if (!desc) return '';
  if (desc.length <= maxLen) return desc;
  return `${desc.slice(0, maxLen)}...`;
}

// URL 截断（保留域名和路径关键部分）
function truncateUrl(url: string, maxLen = 45): string {
  if (!url) return '';
  if (url.length <= maxLen) return url;

  try {
    const parsed = new URL(url);
    // 保留域名 + 路径前 20 字符
    const domain = parsed.hostname;
    const path = parsed.pathname.slice(0, 20);
    const result = `${domain}${path}${parsed.pathname.length > 20 ? '...' : ''}`;
    return result.length > maxLen ? result.slice(0, maxLen - 3) + '...' : result;
  } catch {
    // 解析失败，简单截断
    return `${url.slice(0, maxLen - 3)}...`;
  }
}

// 搜索查询截断
function truncateQuery(query: string, maxLen = 30): string {
  if (!query) return '';
  if (query.length <= maxLen) return query;
  return `${query.slice(0, maxLen)}...`;
}

// 问题截断
function truncateQuestion(question: string, maxLen = 20): string {
  if (!question) return '';
  if (question.length <= maxLen) return question;
  return `${question.slice(0, maxLen)}...`;
}

// 默认摘要生成（提取首个关键参数）
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

### 4. 样式更新

在 `ContentBlock.css` 中添加：

```css
.tool-call-name {
  font-weight: 500;
  color: #262626;
}

.tool-call-param {
  color: #64748B;
  font-size: 11px;
  margin-left: 4px;
}

/* streaming 状态下的样式 */
.tool-call-row.streaming .tool-call-name {
  color: var(--accent-color, #7C3AED);
}

.tool-call-row.streaming .tool-call-param {
  color: var(--accent-color-light, #C084FC);
}
```

### 5. CLI Output Block 组件设计

#### 5.1 新增 CliOutputBlock 组件

创建 `web/src/components/thread/ContentBlock/CliOutputBlock.tsx`：

```typescript
interface CliOutputBlockProps {
  events: CliEvent[];          // 工具事件列表
  status: CliStatus;           // streaming/done/failed/interrupted
  defaultExpanded?: boolean;   // 默认展开状态
  breedColor?: string;         // Agent 品种色（用于 accent）
}

// CliEvent 类型定义
interface CliEvent {
  id: string;
  kind: 'tool_use' | 'tool_result' | 'text';
  label?: string;              // 工具摘要（如 "Read src/api/handler.go"）
  detail?: string;             // 工具详情（input/output）
  timestamp?: number;
}

// CliStatus 类型定义
type CliStatus = 'streaming' | 'done' | 'failed' | 'interrupted';
```

#### 5.2 摘要行构建

```typescript
function buildSummary(events: CliEvent[], status: CliStatus): string {
  const toolCount = events.filter(e => e.kind === 'tool_use').length;
  const statusLabel = STATUS_LABEL[status];
  const timestamps = events.map(e => e.timestamp).filter(Boolean);
  const duration = timestamps.length >= 2 && status !== 'streaming'
    ? ` · ${formatDuration(Math.max(...timestamps) - Math.min(...timestamps))}`
    : '';

  if (status === 'streaming') {
    const last = [...events].reverse().find(e => e.kind === 'tool_use');
    return `CLI 输出 · ${statusLabel}${last ? ` · ${last.label}...` : ''}`;
  }

  // stdout 预览
  const textPreview = buildTextPreview(events);
  if (toolCount > 0) {
    const stdout = textPreview ? ` · stdout: ${textPreview}` : '';
    return `CLI 输出 · ${statusLabel} · ${toolCount} 工具${duration}${stdout}`;
  }

  const lineCount = events
    .filter(e => e.kind === 'text')
    .reduce((n, e) => n + (e.content?.split('\n').length ?? 0), 0);
  return `CLI 输出 · ${statusLabel} · ${lineCount} 行${duration}`;
}

function buildTextPreview(events: CliEvent[], maxChars = 48): string {
  let preview = '';
  for (const event of events) {
    if (event.kind !== 'text') continue;
    for (const char of event.content ?? '') {
      preview = appendPreviewChar(preview, char);
      if (preview.length > maxChars) {
        return `${preview.slice(0, maxChars)}…`;
      }
    }
  }
  return preview.trimEnd();
}
```

#### 5.3 自动折叠逻辑

```typescript
// 用户交互追踪
const userInteracted = useRef(false);
const prevStatusRef = useRef(status);

useEffect(() => {
  // streaming 开始：强制展开
  if (prevStatusRef.current !== 'streaming' && status === 'streaming') {
    userInteracted.current = false;
    setExpanded(true);
  }
  // streaming 结束：自动折叠（除非用户操作过）
  else if (prevStatusRef.current === 'streaming' && status !== 'streaming' && !userInteracted.current) {
    setExpanded(defaultExpanded);
  }
  prevStatusRef.current = status;
}, [status, defaultExpanded]);

const handleToggle = () => {
  userInteracted.current = true;
  setExpanded(v => !v);
};
```

### 7. MessageContentRenderer 集成

修改 `MessageContentRenderer.tsx` 的聚合逻辑：

```typescript
// 原代码：聚合为 tool_use_group
if (block.type === 'tool_use') {
  currentToolGroup.push(block as ToolUseBlock);
}

// 新代码：聚合为 CLI Output Block
// 同时收集 text 块作为 stdout
if (block.type === 'tool_use') {
  currentToolGroup.push(block as ToolUseBlock);
} else if (block.type === 'text' && isInToolContext) {
  // text 块属于 CLI Output 的 stdout
  currentTextContent += (block as TextBlock).content;
}
```

### 8. 支持的工具类型清单

需要处理的工具类型（按 Claude CLI 工具集）：

| 分类 | 工具 | 关键字段 |
|------|------|---------|
| 文件操作 | Read, Write, Edit | file_path |
| 搜索 | Grep, Glob | pattern + path |
| 执行 | Bash | command |
| 网络 | WebFetch, WebSearch | url / query |
| Notebook | NotebookEdit | notebook_path + cell_number |
| Agent | Task, Skill | subagent_name / skill |
| 交互 | AskUserQuestion | question |
| 其他 | TodoWrite, EnterPlanMode, ScheduleWakeup... | 首个关键参数 |

## 测试验收标准

### P0：核心痛点

1. **工具行摘要**：无需展开即可看到关键信息
   - Read: `Read src/api/handler.go`
   - Bash: `Bash npm run build`
   - Skill: `Skill brainstorming`
   - Task: `Task @planner 设计登录...`

2. **CLI Output Block 三层折叠**：
   - 第 1 层：整体折叠/展开，摘要行显示工具数量 + 耗时 + stdout 预览
   - 第 2 层：tools 区独立折叠，stdout 区始终可见
   - 第 3 层：单个工具可展开看 input/output

3. **stdout 预览**：折叠状态显示前 48 字符

4. **自动折叠 UX**：
   - streaming 开始：自动展开
   - streaming 结束：自动折叠（除非用户操作过）

### P1：体验优化

5. **视觉层级**：工具名加粗突出，参数次要颜色
6. **长路径/命令截断**：保留关键信息
7. **流式状态高亮**：正在执行的工具使用 accent 色高亮

## 不在本次范围

以下优化不在本次范围，可后续迭代：
- 品种色个性化（Agent 颜色影响 CLI Block accent）- 需要后端 AgentConfig 增加 color 字段
- SVG Icons 替换 Emoji - 保持现有 Emoji 图标风格
- Thinking 区块优化（深色背景 + Brain 图标）
- Agent 品种样式系统（BREED_STYLES 圆角/字体）
- 消息气泡 hover 动效
- A2A 触发者追踪（a2aFrom map）
- Rich Blocks 安全检查（provenance 验证）
- Session Chain 展示
- Thinking 模式切换（debug/play）

## 参考

### cat-cafe 文档
- `F097-CLI-output-collapsible-UX.md` - CLI Output Block 三层折叠设计
- `CliOutputBlock.tsx` - CLI Output Block 组件实现
- `ThinkingContent.tsx` - Thinking 区块实现（深色背景 + Brain 图标）

### Colink 文档
- `ToolBlock.tsx` - 当前工具块实现
- `MessageContentRenderer.tsx` - 消息内容渲染器
- `/Users/cc/Workspace/Code/cat-cafe/clowder-ai/packages/api/uploads/doc-04a5685f83d9-colink-vs-catcafe-对话展示对比分析.md` - 对比分析报告

### Claude CLI 工具集
- 文件操作：Read, Write, Edit, Glob, Grep
- 执行：Bash
- 网络：WebFetch, WebSearch
- Notebook：NotebookEdit
- Agent：Task, Skill
- 交互：AskUserQuestion, TodoWrite, EnterPlanMode, ExitPlanMode