# CLI Output 展示优化代码审查报告

> 生成日期: 2026-05-08
> 审查者: Colink质量审核员

## 验收结果：部分通过

---

## P0 核心痛点逐项验收

### 1. 工具行摘要 - ✅ 通过

**规格要求**：无需展开即可看到关键信息（Read、Bash、Skill、Task 等工具的结构化摘要）

**实现情况**：

`ToolBlock.tsx` 已完整实现 `generateToolSummary` 函数，覆盖所有要求的工具类型：
- Read/Write/Edit: 显示路径
- Bash: 显示命令
- Skill: 显示 skill 名
- Task: 显示 @agent + description
- Grep/Glob: 显示 pattern + path
- WebFetch/WebSearch: 显示 URL/query
- NotebookEdit: 显示 notebook[cell]
- AskUserQuestion: 显示问题摘要

渲染逻辑正确使用 `summary.name` 和 `summary.param`。

---

### 2. CLI Output Block 三层折叠 - ⚠️ 部分通过

**规格要求**：
- 第 1 层：整体折叠/展开，摘要行显示工具数量+耗时+stdout预览
- 第 2 层：tools区独立折叠，stdout区始终可见
- 第 3 层：单个工具可展开看input/output

**实现情况**：

`MessageContentRenderer.tsx` 中 `ToolGroupBlock` 组件：
- **第 1 层整体折叠** - ✅ 已实现（`blockExpanded` 状态）
- **第 2 层 tools 区折叠** - ✅ 已实现（`toolsCollapsed` 状态）
- **第 3 层单个工具展开** - ✅ 已实现（`ToolCallRow` 的 `expanded` 状态）
- **stdout 区始终可见** - ❌ **未实现** - stdout 区根本没有渲染

---

### 3. stdout 预览 - ❌ 未实现

**规格要求**：折叠状态显示前48字符，从 text 块提取

**问题分析**：

| 问题 | 文件位置 | 说明 |
|------|----------|------|
| aggregateBlocks 未收集 text 块 | MessageContentRenderer.tsx:177-227 | 没有将紧跟 tool_use 的 text 块收集为 stdoutBlocks |
| ToolGroupBlockProps 缺少 stdoutBlocks | MessageContentRenderer.tsx:286-289 | 接口定义没有 stdoutBlocks 属性 |
| buildStdoutPreview 从 richBlocks 提取 | MessageContentRenderer.tsx:291-312 | 应从 text 块提取，而非 richBlocks |
| CLI Block 摘要行无 stdout 预览 | MessageContentRenderer.tsx:360-380 | 没有显示 stdout 前48字符预览 |

---

### 4. 自动折叠 UX - ✅ 通过

**规格要求**：
- streaming 开始：自动展开
- streaming 结束：自动折叠（除非用户操作过）

**实现情况**（MessageContentRenderer.tsx:334-352）：

```typescript
// streaming 时自动展开
useEffect(() => {
  if (anyStreaming) {
    if (!blockExpanded) setBlockExpanded(true);
    if (toolsCollapsed) setToolsCollapsed(false);
  }
}, [anyStreaming]);

// 完成后自动折叠（除非用户操作过）
useEffect(() => {
  if (prevStreamingRef.current && !anyStreaming) {
    if (!userInteracted.current) {
      setBlockExpanded(false);
      setToolsCollapsed(true);
    }
  }
  prevStreamingRef.current = anyStreaming;
}, [anyStreaming]);
```

实现正确，用户交互追踪有效。

---

## P1 体验优化验收

### 5. 视觉层级 - ✅ 通过

- 工具名加粗：CSS `.tool-call-name` 设置 `font-weight: 600`
- 参数次要颜色：CSS `.tool-call-param` 设置 `color: #6B7280`

---

### 6. 长路径/命令截断 - ✅ 通过

截断辅助函数完整实现：
- `truncatePath` - 路径截断（保留文件名和关键目录）
- `truncateCommand` - 命令截断（保留命令名）
- `truncateDescription` - 描述截断
- `truncateUrl` - URL 截断（保留域名）
- `truncateQuery` - 查询截断

---

### 7. 流式状态高亮 - ✅ 通过

CSS 中 `.tool-call-row.streaming .tool-call-param` 设置 `color: #C084FC`。

ToolBlock.tsx 中 streaming 状态颜色正确传递。

---

## 发现的问题汇总

### Critical 问题

| # | 问题 | 修复方案 |
|---|------|----------|
| C1 | stdout 预览未实现 | 修改 buildStdoutPreview 从 text 块提取，CLI Block 摘要行添加 stdout 预览 |
| C2 | stdout 区未渲染 | ToolGroupBlock 添加 stdout 区渲染逻辑 |
| C3 | aggregateBlocks 未收集 text 块 | 修改函数逻辑，将紧跟 tool_use 的 text 块收集为 stdoutBlocks |
| C4 | ToolGroupBlockProps 缺少 stdoutBlocks | 接口添加 stdoutBlocks?: TextBlockType[] |

---

### Important 问题

| # | 问题 | 说明 |
|---|------|------|
| I1 | 深色背景硬编码 | Task 2.5 使用硬编码 `#1A1625`，CSS 缺少 `.cli-output-block` 专属样式 |

---

## 修复方案（参考实施计划 Step 2.1-2.6）

### 1. 修改 aggregateBlocks 函数

收集 text 块作为 stdoutBlocks：

```typescript
function aggregateBlocks(blocks: MessageContentBlock[]): Array<MessageContentBlock | { type: 'tool_use_group'; tools: ToolUseBlock[]; stdoutBlocks?: TextBlockType[]; richBlocks?: RichBlock[] }> {
  // ...
  let currentStdoutBlocks: TextBlockType[] = [];

  for (const block of blocks) {
    if (block.type === 'tool_use') {
      currentToolGroup.push(block as ToolUseBlock);
    } else if (block.type === 'text') {
      if (currentToolGroup.length > 0) {
        currentStdoutBlocks.push(block as TextBlockType);
      } else {
        result.push(block);
      }
    }
    // ...
  }

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

### 2. 修改 ToolGroupBlockProps 接口

添加 stdoutBlocks 属性：

```typescript
interface ToolGroupBlockProps {
  tools: ToolUseBlock[];
  stdoutBlocks?: TextBlockType[];
  defaultExpanded?: boolean;
}
```

### 3. 修改 buildStdoutPreview 函数

从 text 块提取前48字符：

```typescript
function buildStdoutPreview(stdoutBlocks?: TextBlockType[]): string {
  if (!stdoutBlocks || stdoutBlocks.length === 0) return '';
  let preview = '';
  for (const block of stdoutBlocks) {
    const content = block.content || '';
    for (const char of content) {
      if (/\s/.test(char)) {
        preview = preview && !preview.endsWith(' ') ? `${preview} ` : preview;
      } else {
        preview = `${preview}${char}`;
      }
      if (preview.length > 48) {
        return `${preview.slice(0, 48)}…`;
      }
    }
  }
  return preview.trimEnd();
}
```

### 4. 修改 ToolGroupBlock 渲染

添加 stdout 预览和 stdout 区：

```typescript
// CLI Block 摘要行
const stdoutPreview = buildStdoutPreview(stdoutBlocks);
let summary = `CLI 输出 · ${statusText} · ${toolCount} 工具`;
if (stdoutPreview && !blockExpanded) {
  summary += ` · stdout: ${stdoutPreview}`;
}

// 第 2 层展开后，stdout 区渲染
{stdoutBlocks && stdoutBlocks.length > 0 && (
  <>
    <div className="cli-output-stdout-divider">─── stdout ───</div>
    <div className="cli-output-stdout">
      {stdoutBlocks.map((block, i) => (
        <React.Fragment key={i}>
          {block.content}
          {i < stdoutBlocks.length - 1 && '\n'}
        </React.Fragment>
      ))}
    </div>
  </>
)}
```

### 5. 修改渲染逻辑传递 stdoutBlocks

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

---

## 下一步

@Colink开发工程师 请修复 stdout 预览和 stdout 区渲染功能（详见本审查报告修复方案）