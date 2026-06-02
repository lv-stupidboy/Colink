# 输入框 @mention 智能前置实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Agent 完成后智能前置 @mention，不清空已有内容，保持光标相对偏移

**Architecture:** 修改 ThreadInput.tsx 的 prefilledMention useEffect，实现前置逻辑而非替换逻辑

**Tech Stack:** React hooks (useEffect, useRef), TypeScript, Ant Design TextArea

---

## Task 1: 修改 prefilledMention useEffect 逻辑

**Files:**
- Modify: `web/src/components/thread/ThreadInput.tsx:117-139`

- [ ] **Step 1: 读取当前代码**

确认修改位置：第 117-139 行的 useEffect

```typescript
// 当前代码（需要替换）
useEffect(() => {
  if (prefilledMention && inputRef.current) {
    // 检查是否在 agentOptions 中
    const agentExists = agentOptions.some(
      opt => opt.name === prefilledMention || opt.label.includes(prefilledMention)
    );

    if (agentExists) {
      // 自动填入 @mention
      setInputValue(`@${prefilledMention} `);
      inputRef.current.focus();
      setShowPrefillHint(true);

      // 3秒后隐藏提示
      setTimeout(() => setShowPrefillHint(false), 3000);

      // 通知父组件预填入已使用
      if (onPrefillConsumed) {
        onPrefillConsumed();
      }
    }
  }
}, [prefilledMention, agentOptions, onPrefillConsumed]);
```

- [ ] **Step 2: 替换为新的前置逻辑**

替换第 117-139 行的整个 useEffect：

```typescript
// 自动填入 @mention（阻塞确认后触发）- 智能前置逻辑
useEffect(() => {
  if (prefilledMention && inputRef.current) {
    // 检查是否在 agentOptions 中
    const agentExists = agentOptions.some(
      opt => opt.name === prefilledMention || opt.label.includes(prefilledMention)
    );

    if (!agentExists) return;

    const currentText = inputValue;

    // 判断是否以 @ 开头（已有 @mention 时不添加）
    if (currentText.startsWith('@')) {
      // 通知父组件预填入已使用（跳过添加）
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

    // 通知父组件预填入已使用
    onPrefillConsumed?.();
  }
}, [prefilledMention, agentOptions, inputValue, onPrefillConsumed]);
```

**关键改动：**
1. 添加 `inputValue` 到依赖数组
2. 检查 `currentText.startsWith('@')` 判断是否已有 @mention
3. 使用 `mention + currentText` 前置而非替换
4. 记录并恢复光标相对偏移

- [ ] **Step 3: 提交改动**

```bash
git add web/src/components/thread/ThreadInput.tsx
git commit -m "fix(thread): prefilledMention 前置而非清空，保持光标相对偏移

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 2: 手动验证功能

**Files:**
- 无文件改动，仅运行验证

- [ ] **Step 1: 启动开发服务器**

```bash
cd web && npm run dev
```

Expected: 前端开发服务器启动在端口 26306

- [ ] **Step 2: 启动后端服务**

```bash
go run ./cmd/server
```

Expected: 后端服务启动在端口 26305

- [ ] **Step 3: 验证场景 1 - 空输入框**

操作：
1. 进入对话页面
2. 发送一条消息触发 Agent
3. 等待 Agent 完成回复
4. 观察输入框

Expected: 输入框自动填入 `@AgentName `，光标在末尾

- [ ] **Step 4: 验证场景 2 - 有内容时添加**

操作：
1. 在输入框输入 "你好你好"
2. 等待 Agent 完成（不要发送消息）
3. 观察输入框变化

Expected: 输入框变为 `@AgentName 你好你好`，光标在 "你好你好" 后面（末尾）

- [ ] **Step 5: 验证场景 3 - 已有 @mention**

操作：
1. 在输入框输入 "@架构师 请帮我"
2. 等待 Agent 完成
3. 观察输入框变化

Expected: 输入框内容不变，仍为 `@架构师 请帮我`（不添加新的 @mention）

- [ ] **Step 6: 验证场景 4 - 光标在中间**

操作：
1. 在输入框输入 "你好你好"
2. 将光标移动到第一个 "你好" 后面（位置 2）
3. 等待 Agent 完成
4. 观察光标位置

Expected: 输入框变为 `@AgentName 你好你好`，光标在 "你好你好" 的位置 2（即 `@AgentName 你好|你好`）

---

## Task 3: TypeScript 类型检查

**Files:**
- 无文件改动

- [ ] **Step 1: 运行 TypeScript 检查**

```bash
cd web && npm run build
```

Expected: 构建成功，无 TypeScript 类型错误

**注意：** `npm run build` 会先执行 `tsc && vite build`，如果类型有错误会在此步骤失败

---

## 完成标志

- [ ] 所有手动验证场景通过
- [ ] TypeScript 类型检查通过
- [ ] Git commit 已提交