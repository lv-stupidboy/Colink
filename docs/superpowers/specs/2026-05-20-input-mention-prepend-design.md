---
name: input-mention-prepend
description: Agent 完成后智能前置 @mention，不清空已有内容，保持光标相对偏移
metadata:
  type: project
---

# 对话输入框 @mention 智能前置设计

## 背景

当前实现：Agent 完成后，`prefilledMention` 处理逻辑会**直接替换整个输入框内容**，清空用户已输入的文字。

**问题代码**：`ThreadInput.tsx:126`
```typescript
setInputValue(`@${prefilledMention} `);  // 清空已有内容
```

## 需求

1. **不清空已有内容** — 保留用户已输入的文字
2. **智能判断是否添加** — 内容以 `@` 开头时不添加
3. **前置 @mention** — 在内容最前面插入 `@AgentName `
4. **光标相对偏移不变** — 光标相对于原内容的位置保持不变

### 示例

| 场景 | 原内容 | 最终内容 |
|------|--------|----------|
| 空输入框 | `""` | `"@开发者 "` |
| 有内容，光标在末尾 | `"你好你好"` | `"@开发者 你好你好"` |
| 有内容，光标在中间 | `"你好|你好"` | `"@开发者 你好|你好"` |
| 已有 @mention | `"@架构师 xxx"` | `"@架构师 xxx"` (不添加) |

## 设计方案

**修改位置**：`web/src/components/thread/ThreadInput.tsx` 第 117-139 行的 `useEffect`

### 核心逻辑

```typescript
useEffect(() => {
  if (prefilledMention && inputRef.current) {
    const agentExists = agentOptions.some(
      opt => opt.name === prefilledMention || opt.label.includes(prefilledMention)
    );

    if (!agentExists) return;

    const currentText = inputValue;

    // 判断是否以 @ 开头
    if (currentText.startsWith('@')) {
      // 已有 @mention，不添加
      onPrefillConsumed?.();
      return;
    }

    // 记录当前光标位置
    const cursorPos = inputRef.current.selectionStart || 0;

    // 构建新内容：前置 @mention
    const mention = `@${prefilledMention} `;
    const newText = mention + currentText;

    // 更新输入框
    setInputValue(newText);
    inputRef.current.focus();

    // 恢复光标相对偏移：原位置 + mention 长度
    const newCursorPos = cursorPos + mention.length;
    setTimeout(() => {
      inputRef.current?.setSelectionRange(newCursorPos, newCursorPos);
    }, 0);

    // 显示提示
    setShowPrefillHint(true);
    setTimeout(() => setShowPrefillHint(false), 3000);

    onPrefillConsumed?.();
  }
}, [prefilledMention, agentOptions, inputValue, onPrefillConsumed]);
```

### 技术细节

1. **光标位置恢复**：使用 `setTimeout` 确保 React 更新 DOM 后再设置光标
2. **已有 @ 检测**：使用 `startsWith('@')` 简单判断，无需复杂正则
3. **空内容处理**：`"" + "@开发者 "` 结果为 `"@开发者 "`，符合预期

## 影响范围

| 文件 | 改动 |
|------|------|
| `web/src/components/thread/ThreadInput.tsx` | 修改 useEffect 逻辑 |

**无需改动**：
- `ThreadView.tsx` — 触发逻辑不变
- Props 接口 — `prefilledMention` 语义不变

## 测试要点

1. Agent 完成后输入框有内容 → @mention 前置，光标在末尾
2. Agent 完成后输入框为空 → 直接填入 @mention
3. 输入框已有 @mention → 不添加
4. 光标在中间位置 → 添加后光标位置正确