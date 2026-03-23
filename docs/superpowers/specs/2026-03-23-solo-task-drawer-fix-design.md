# Solo 模式任务抽屉修复设计

## 背景

Solo 模式是全栈工程师 Agent 的主要开发交互页面，左侧增加了任务列表组件用于快速新建任务和切换对话。当前存在以下问题：

1. **布局异常**：任务列表和消息区垂直排列而非水平排列，导致对话框展示异常
2. **切换功能缺失**：选择历史任务时，消息未加载、WebSocket 未连接，无法正常切换
3. **新建任务跳转页面**：点击新建任务会跳转到项目页面，用户体验不连贯

## 目标

1. 修复 Solo 模式布局，实现任务抽屉与消息区的水平排列
2. 完善任务切换逻辑，确保选择历史任务时能正确加载消息和建立 WebSocket 连接
3. 新建任务时不跳转页面，直接开启新对话，以用户第一条消息作为任务名
4. 添加抽屉展开/收起控制，提升用户体验

## 核心变更

### 1. 布局修复 (CSS + JSX 结构)

**问题根因分析：**

当前 JSX 结构：
```tsx
// ThreadView.tsx 行 1453-1575
<div className={`thread-view-wrapper ${soloMode ? 'solo-mode' : ''}`}>
  {soloMode && <div className="solo-mode-header">...</div>}

  {soloMode ? (
    <>
      <TaskList ... />
      <div className="solo-mode-content">
        <div className="thread-view">...</div>
      </div>
      {rightPanelVisible && (
        <>
          <div className="resize-handle" .../>
          <RightPanel .../>
        </>
      )}
    </>
  ) : (
    ...
  )}
</div>
```

当前 CSS：
```css
.thread-view-wrapper.solo-mode {
  display: flex;
  flex-direction: column;  /* header 上，内容下 */
}
```

问题：`TaskList`、`.solo-mode-content`、右侧面板在 Fragment 中是兄弟元素，但在 `flex-direction: column` 容器中垂直排列。

**修复方案：**

添加 `.solo-mode-body` 水平布局容器，将 TaskList、消息区、右侧面板包裹在一起：

```tsx
// 修改后的结构
<div className={`thread-view-wrapper ${soloMode ? 'solo-mode' : ''}`}>
  {soloMode && <div className="solo-mode-header">...</div>}

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
        <div className="thread-view">...</div>
      </div>

      {/* 右侧面板（可选） */}
      {rightPanelVisible && (
        <>
          <div className="resize-handle" .../>
          <RightPanel .../>
        </>
      )}
    </div>
  ) : (
    ...
  )}
</div>
```

**文件：** `isdp/web/src/pages/ThreadView.css`

**新增样式（追加到现有 CSS）：**

```css
/* 新增：Solo 模式内容容器 - 水平布局 */
.solo-mode-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* 新增：任务抽屉样式 */
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

**修改现有样式：**

将现有的 `.solo-mode-content` 样式（行 809-827）添加 `min-width: 0`：

```css
.solo-mode-content {
  display: flex;
  flex: 1;
  min-height: 0;
  min-width: 0;  /* 新增：防止内容区溢出 */
  overflow: hidden;
}
```

### 2. 抽屉展开/收起控制

**文件：** `isdp/web/src/pages/ThreadView.tsx`

新增状态（在其他 useState 附近添加）：
```tsx
const [taskDrawerOpen, setTaskDrawerOpen] = useState(true);
```

在 `solo-mode-header` 左侧添加切换按钮（修改行 1388-1414）：
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
  {/* ... */}
</div>
```

### 3. 任务切换逻辑修复

**文件：** `isdp/web/src/pages/ThreadView.tsx`

#### 修复 `handleSelectSoloTask`：

```tsx
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

**说明：** Solo 模式下不加载 artifacts 和 review data，这些是工作流模式特有的功能。Solo 模式专注于单 Agent 的开发交互。

**注意：** `connectDebugWebSocket` 函数（ThreadView.tsx 行 220-261）已实现关闭现有连接的逻辑，无需额外处理。

#### 修复 `handleCreateSoloTask` - 不跳转页面：

```tsx
const handleCreateSoloTask = useCallback(() => {
  // 1. 清空当前消息和状态
  if (isDebugMode) {
    clearDebugAll();
  }

  // 2. 重置活跃任务状态，标记为新任务待创建
  setSoloActiveTask(null);
  setSoloNewTaskPending(true);

  // 3. 不再导航跳转，保持在当前页面
  // 删除原有的 navigate 调用
}, [isDebugMode, clearDebugAll]);
```

### 4. 新建任务逻辑优化

`handleSoloSend` 已正确实现：以用户第一条消息作为任务名。

当前代码（ThreadView.tsx 行 903-939）逻辑正确，无需修改：
```tsx
const handleSoloSend = useCallback(async (content: string) => {
  if (soloNewTaskPending && projectId) {
    try {
      // 用第一条消息的前 30 个字符作为任务名
      const taskName = content.slice(0, 30) + (content.length > 30 ? '...' : '');
      const newThread = await api.threads.create(projectId, taskName);
      // ... 设置状态、连接 WebSocket、更新 URL
    } catch (error) {
      // ...
    }
  }
  // ... 发送消息
}, [...]);
```

确认 URL 更新使用 `replace: true`（已存在，无需修改）：
```tsx
navigate(`/agents/${agentId}?threadId=${newThread.id}`, { replace: true });
```

## 修改文件列表

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `isdp/web/src/pages/ThreadView.css` | 修改 | 添加 `.solo-mode-body`、`.solo-task-drawer` 样式，修改 `.solo-mode-content` |
| `isdp/web/src/pages/ThreadView.tsx` | 修改 | 添加抽屉状态、修改 JSX 结构、修复切换逻辑、添加任务按钮 |

## 验证方法

### 1. 布局验证
- 进入 Solo 模式，确认任务列表在左侧，消息区在右侧
- 点击"任务"按钮，确认抽屉能正常展开/收起
- 确认展开/收起动画流畅
- 展开右侧面板，确认布局正确：任务抽屉 | 消息区 | 右侧面板
- 收起任务抽屉时，确认右侧面板布局不受影响

### 2. 新建任务验证
- 点击"新建对话"按钮
- 确认**不跳转页面**，保持在当前界面
- 输入消息发送
- 确认任务创建成功，任务名使用第一条消息的前 30 字符
- 确认消息正常显示，WebSocket 连接正常

### 3. 切换任务验证
- 创建多个任务
- 在任务列表中点击不同任务
- 确认消息正确加载（历史消息可见）
- 确认 WebSocket 连接正常，能继续发送消息

### 4. 边界情况验证
- 测试长任务名，确认任务列表显示正确（ellipsis 生效）
- 右侧面板打开时，收起/展开任务抽屉，确认无布局跳动
- 任务列表较长时，确认滚动正常

## 影响范围

- Solo 模式（全栈工程师 Agent 调试模式）
- 不影响工作流模式和其他 Agent 调试模式