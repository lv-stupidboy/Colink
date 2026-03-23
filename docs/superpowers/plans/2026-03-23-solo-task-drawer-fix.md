# Solo 模式任务抽屉修复实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 Solo 模式布局、任务切换逻辑和新建任务体验

**Architecture:** 添加 `.solo-mode-body` 水平布局容器包裹任务抽屉、消息区和右侧面板；修复任务切换时加载消息和 WebSocket 连接；移除新建任务的页面跳转

**Tech Stack:** React, CSS, TypeScript

---

## 文件结构

| 文件 | 职责 |
|------|------|
| `isdp/web/src/pages/ThreadView.css` | Solo 模式布局样式 |
| `isdp/web/src/pages/ThreadView.tsx` | 组件状态、JSX 结构、事件处理 |

---

### Task 1: 添加 CSS 样式

**Files:**
- Modify: `isdp/web/src/pages/ThreadView.css`

- [ ] **Step 1: 添加 `.solo-mode-body` 样式**

在 `.solo-mode-content` 样式之前（约行 808）添加：

```css
/* Solo 模式内容容器 - 水平布局 */
.solo-mode-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
```

- [ ] **Step 2: 添加 `.solo-task-drawer` 样式**

在 `.solo-mode-body` 样式之后添加：

```css
/* 任务抽屉样式 */
.solo-task-drawer {
  height: 100%;
  width: 240px;
  flex-shrink: 0;
  transition: width 0.3s ease, opacity 0.3s ease;
  overflow: hidden;
}

.solo-task-drawer.collapsed {
  width: 0;
  opacity: 0;
}
```

- [ ] **Step 3: 修改 `.solo-mode-content` 样式**

找到现有的 `.solo-mode-content` 样式（约行 809-814），添加 `min-width: 0`：

```css
.solo-mode-content {
  display: flex;
  flex: 1;
  min-height: 0;
  min-width: 0;  /* 新增：防止内容区溢出 */
  overflow: hidden;
}
```

- [ ] **Step 4: 验证 CSS 语法**

在项目 `isdp/web` 目录下运行：
```bash
npm run lint
```

预期：无 CSS 相关错误

- [ ] **Step 5: 提交 CSS 更改**

```bash
git add isdp/web/src/pages/ThreadView.css
git commit -m "style: 添加 Solo 模式水平布局和任务抽屉样式"
```

---

### Task 2: 添加抽屉状态和切换按钮

**Files:**
- Modify: `isdp/web/src/pages/ThreadView.tsx`

- [ ] **Step 1: 添加 `taskDrawerOpen` 状态**

在 `soloMode` 状态附近（约行 147）添加：

```tsx
// Solo 模式状态
const [soloMode, setSoloMode] = useState(false);
const [taskDrawerOpen, setTaskDrawerOpen] = useState(true);
```

- [ ] **Step 2: 修改 `solo-mode-header` 添加任务按钮**

找到 `solo-mode-header` 的 JSX（约行 1388-1414），在 `solo-mode-tabs` 内添加任务按钮：

```tsx
<div className="solo-mode-header">
  <div className="solo-mode-tabs">
    {/* 新增：任务抽屉切换按钮 */}
    <Button
      type="text"
      className={`solo-mode-tab ${taskDrawerOpen ? 'active' : ''}`}
      icon={<UnorderedListOutlined />}
      onClick={() => setTaskDrawerOpen(!taskDrawerOpen)}
    >
      任务
    </Button>
    <Button
      type="text"
      className="solo-mode-tab"
      icon={<ApartmentOutlined />}
      onClick={() => setSoloMode(false)}
    >
      工作流模式
    </Button>
    <Button
      type="text"
      className="solo-mode-tab active"
      icon={<ThunderboltOutlined />}
    >
      Solo 模式
    </Button>
  </div>
  <Button
    className={`solo-mode-action-btn ${rightPanelVisible ? 'primary' : ''}`}
    icon={<DesktopOutlined />}
    onClick={() => setRightPanelVisible(!rightPanelVisible)}
  >
    面板
  </Button>
</div>
```

- [ ] **Step 3: 验证编译**

运行：
```bash
cd isdp/web && npm run build
```

预期：编译成功，无错误

- [ ] **Step 4: 提交状态和按钮更改**

```bash
git add isdp/web/src/pages/ThreadView.tsx
git commit -m "feat(SoloMode): 添加任务抽屉展开/收起控制"
```

---

### Task 3: 修改 JSX 布局结构

**Files:**
- Modify: `isdp/web/src/pages/ThreadView.tsx`

- [ ] **Step 1: 修改 Solo 模式 JSX 结构**

找到 Solo 模式的条件渲染部分（约行 1453-1575），将 Fragment 改为 `.solo-mode-body` 容器：

**修改前：**
```tsx
{soloMode ? (
  <>
    <TaskList
      projectId={projectId || ''}
      activeThreadId={soloActiveTask?.id || null}
      onSelectTask={handleSelectSoloTask}
      onCreateTask={handleCreateSoloTask}
      isRunning={debugStatus === 'running'}
    />
    <div className="solo-mode-content">
      ...
    </div>
    {rightPanelVisible && (
      <>
        <div className="resize-handle" .../>
        <RightPanel .../>
      </>
    )}
  </>
) : (
```

**修改后：**
```tsx
{soloMode ? (
  <div className="solo-mode-body">
    {/* 任务抽屉 */}
    <div className={`solo-task-drawer ${!taskDrawerOpen ? 'collapsed' : ''}`}>
      <TaskList
        projectId={projectId || ''}
        activeThreadId={soloActiveTask?.id || null}
        onSelectTask={handleSelectSoloTask}
        onCreateTask={handleCreateSoloTask}
        isRunning={debugStatus === 'running'}
      />
    </div>

    {/* 消息区 */}
    <div className="solo-mode-content">
      <div className="thread-view">
        {/* 消息区域 */}
        <div className="thread-messages">
          ...
        </div>

        {/* 底部输入区 */}
        <div className="thread-input">
          ...
        </div>
      </div>
    </div>

    {/* 右侧面板（可选） */}
    {rightPanelVisible && (
      <>
        <div className={`resize-handle ${isResizing ? 'resizing' : ''}`} onMouseDown={handleResizeStart} style={{ width: isResizing ? 3 : 6 }} />
        <div style={{ position: 'relative', display: 'flex' }}>
          {isResizing && <div className="resize-overlay" />}
          <RightPanel
            visible={rightPanelVisible}
            onClose={closeRightPanel}
            activeTab={rightPanelActiveTab}
            onTabChange={setRightPanelActiveTab}
            codeFiles={codeFiles}
            expandedFiles={expandedFiles}
            onToggleFile={toggleFileExpand}
            sandboxServer={currentSandboxServer}
            sandboxLoading={currentSandboxLoading}
            dockerAvailable={dockerAvailable}
            hasProjectPath={Boolean(getProjectPath())}
            isDebugMode={isDebugMode}
            onRunSandbox={handleRunSandbox}
            onStopSandbox={handleStopSandbox}
            width={rightPanelWidth}
          />
        </div>
      </>
    )}
  </div>
) : (
```

- [ ] **Step 2: 验证编译**

运行：
```bash
cd isdp/web && npm run build
```

预期：编译成功，无错误

- [ ] **Step 3: 提交 JSX 结构更改**

```bash
git add isdp/web/src/pages/ThreadView.tsx
git commit -m "fix(SoloMode): 修复布局结构，任务抽屉与消息区水平排列"
```

---

### Task 4: 修复任务切换逻辑

**Files:**
- Modify: `isdp/web/src/pages/ThreadView.tsx`

- [ ] **Step 1: 修改 `handleSelectSoloTask` 函数**

找到 `handleSelectSoloTask` 函数（约行 875-884），替换为：

```tsx
// 选择任务
const handleSelectSoloTask = useCallback(async (task: Thread) => {
  // 1. 清空当前消息
  if (isDebugMode) {
    clearDebugAll();
  }

  // 2. 设置活跃任务
  setSoloActiveTask(task);
  setSoloNewTaskPending(false);

  // 3. 调试模式：设置 threadId + 加载消息 + 连接 WebSocket
  if (isDebugMode) {
    setDebugThreadId(task.id);

    // 加载历史消息
    try {
      const messages = await api.messages.list(task.id);
      messages.forEach(msg => addDebugMessage(msg));
    } catch (error) {
      console.error('Failed to load messages:', error);
    }

    // 连接 WebSocket（函数内部会先关闭现有连接）
    connectDebugWebSocket(task.id);
  }

  // 4. 更新 URL（不触发重新渲染）
  if (isDebugMode && agentId) {
    navigate(`/agents/${agentId}?threadId=${task.id}`, { replace: true });
  } else if (projectId) {
    navigate(`/projects/${projectId}/threads/${task.id}`, { replace: true });
  }
}, [isDebugMode, agentId, projectId, navigate, clearDebugAll, setDebugThreadId, addDebugMessage, connectDebugWebSocket]);
```

- [ ] **Step 2: 验证编译**

运行：
```bash
cd isdp/web && npm run build
```

预期：编译成功，无错误

- [ ] **Step 3: 提交任务切换逻辑修复**

```bash
git add isdp/web/src/pages/ThreadView.tsx
git commit -m "fix(SoloMode): 修复任务切换逻辑，加载历史消息和连接 WebSocket"
```

---

### Task 5: 修复新建任务逻辑

**Files:**
- Modify: `isdp/web/src/pages/ThreadView.tsx`

- [ ] **Step 1: 修改 `handleCreateSoloTask` 函数**

找到 `handleCreateSoloTask` 函数（约行 887-900），替换为：

```tsx
// 新建任务
const handleCreateSoloTask = useCallback(() => {
  // 1. 清空当前消息和状态
  if (isDebugMode) {
    clearDebugAll();
  }

  // 2. 重置活跃任务状态，标记为新任务待创建
  setSoloActiveTask(null);
  setSoloNewTaskPending(true);

  // 3. 不再导航跳转，保持在当前页面
}, [isDebugMode, clearDebugAll]);
```

- [ ] **Step 2: 验证编译**

运行：
```bash
cd isdp/web && npm run build
```

预期：编译成功，无错误

- [ ] **Step 3: 提交新建任务逻辑修复**

```bash
git add isdp/web/src/pages/ThreadView.tsx
git commit -m "fix(SoloMode): 新建任务不跳转页面，直接开启新对话"
```

---

### Task 6: 手动验证

- [ ] **Step 1: 启动开发服务器**

```bash
cd isdp/web && npm run dev
cd isdp && go run ./cmd/server
```

- [ ] **Step 2: 验证布局**

1. 进入全栈工程师 Agent 调试模式，确认自动进入 Solo 模式
2. 确认任务列表在左侧，消息区在右侧（水平排列）
3. 点击"任务"按钮，确认抽屉能正常展开/收起
4. 确认动画流畅

- [ ] **Step 3: 验证新建任务**

1. 点击"新建对话"按钮
2. 确认不跳转页面
3. 输入消息发送
4. 确认任务创建成功，任务名使用第一条消息前 30 字符

- [ ] **Step 4: 验证任务切换**

1. 创建多个任务
2. 在任务列表中点击不同任务
3. 确认历史消息正确加载
4. 确认能继续发送消息

- [ ] **Step 5: 验证边界情况**

1. 打开右侧面板，收起/展开任务抽屉，确认无布局跳动
2. 测试长任务名，确认显示正确

---

### Task 7: 最终提交

- [ ] **Step 1: 更新 CHANGELOG**

在 `docs/CHANGELOG.md` 开头添加：

```markdown
## 2026-03-23 Solo 模式任务抽屉修复

### 背景
Solo 模式左侧任务列表存在布局异常、切换功能缺失、新建任务跳转页面等问题

### 目标
1. 修复 Solo 模式布局，实现任务抽屉与消息区的水平排列
2. 完善任务切换逻辑
3. 新建任务时不跳转页面

### 核心变更
- 添加 `.solo-mode-body` 水平布局容器
- 添加 `.solo-task-drawer` 抽屉样式
- 添加任务抽屉展开/收起控制按钮
- 修复 `handleSelectSoloTask` 加载消息和连接 WebSocket
- 修复 `handleCreateSoloTask` 不跳转页面

### 新增/修改文件列表
| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `isdp/web/src/pages/ThreadView.css` | 修改 | 添加布局和抽屉样式 |
| `isdp/web/src/pages/ThreadView.tsx` | 修改 | 状态、JSX 结构、事件处理 |

### 验证方法
- 布局验证：任务列表和消息区水平排列
- 新建任务：不跳转页面，以第一条消息命名
- 任务切换：历史消息正确加载

### 影响范围
Solo 模式（所有 Agent 调试模式可用）
```

- [ ] **Step 2: 提交 CHANGELOG**

```bash
git add docs/CHANGELOG.md
git commit -m "docs: 更新 CHANGELOG 记录 Solo 模式任务抽屉修复"
```