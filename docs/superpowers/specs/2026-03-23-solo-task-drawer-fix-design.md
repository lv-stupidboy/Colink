# Solo 模式任务抽屉修复设计

## 背景

Solo 模式是全栈工程师 Agent 的主要开发交互页面，左侧增加了任务列表组件用于快速新建任务和切换对话。当前存在以下问题：

1. **布局异常**：任务列表和消息区垂直排列而非水平排列，导致对话框展示异常
2. **切换功能缺失**：选择历史任务时，消息未加载、WebSocket 未连接，无法正常切换

## 目标

1. 修复 Solo 模式布局，实现任务抽屉与消息区的水平排列
2. 完善任务切换逻辑，确保选择历史任务时能正确加载消息和建立 WebSocket 连接
3. 添加抽屉展开/收起控制，提升用户体验

## 核心变更

### 1. 布局修复 (CSS)

**文件：** `isdp/web/src/pages/ThreadView.css`

**问题根因：**
```css
.thread-view-wrapper.solo-mode {
  display: flex;
  flex-direction: column;  /* 错误：导致任务列表和消息区垂直排列 */
}
```

**修复方案：**
```css
/* Solo 模式容器改为水平布局 */
.thread-view-wrapper.solo-mode {
  display: flex;
  flex-direction: row;  /* 改为水平排列 */
}

/* 任务抽屉样式 */
.solo-task-drawer {
  height: 100%;
  width: 240px;
  flex-shrink: 0;
  transition: width 0.3s ease, margin-left 0.3s ease;
  overflow: hidden;
}

.solo-task-drawer.collapsed {
  width: 0;
  padding: 0;
}

/* Solo 模式内容区自适应 */
.solo-mode-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
```

### 2. 抽屉展开/收起控制

**文件：** `isdp/web/src/pages/ThreadView.tsx`

新增状态：
```tsx
const [taskDrawerOpen, setTaskDrawerOpen] = useState(true);
```

在 `solo-mode-header` 左侧添加切换按钮：
```tsx
<Button
  type="text"
  className="solo-mode-tab"
  icon={<UnorderedListOutlined />}
  onClick={() => setTaskDrawerOpen(!taskDrawerOpen)}
>
  任务
</Button>
```

TaskList 组件添加 collapsed class：
```tsx
<TaskList
  className={`solo-task-drawer ${!taskDrawerOpen ? 'collapsed' : ''}`}
  // ...其他 props
/>
```

### 3. 任务切换逻辑修复

**文件：** `isdp/web/src/pages/ThreadView.tsx`

**问题：** `handleSelectSoloTask` 缺少消息加载和 WebSocket 连接逻辑

**修复方案：**

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
      const messages = await api.threads.getMessages(task.id);
      messages.forEach(msg => addDebugMessage(msg));
    } catch (error) {
      console.error('Failed to load messages:', error);
    }

    // 连接 WebSocket
    connectDebugWebSocket(task.id);
  }

  // 4. 更新 URL（不触发重新渲染）
  if (isDebugMode && agentId) {
    navigate(`/agents/${agentId}?threadId=${task.id}`, { replace: true });
  } else if (projectId) {
    navigate(`/projects/${projectId}/threads/${task.id}`, { replace: true });
  }
}, [isDebugMode, agentId, projectId, navigate, clearDebugAll, setDebugThreadId, addDebugMessage]);
```

### 4. API 检查

确认 `api.threads.getMessages(threadId)` 是否存在，如不存在需添加：

**文件：** `isdp/web/src/api/client.ts`

```tsx
threads: {
  // ... 现有方法
  getMessages: (threadId: string) =>
    request.get<Message[]>(`/threads/${threadId}/messages`),
}
```

## 修改文件列表

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `isdp/web/src/pages/ThreadView.css` | 修改 | 修复 Solo 模式布局，添加抽屉样式 |
| `isdp/web/src/pages/ThreadView.tsx` | 修改 | 添加抽屉状态控制，修复任务切换逻辑 |
| `isdp/web/src/api/client.ts` | 检查/新增 | 确认或添加 getMessages API |

## 验证方法

1. **布局验证**
   - 进入 Solo 模式，确认任务列表在左侧，消息区在右侧
   - 点击任务按钮，确认抽屉能正常展开/收起
   - 确认展开/收起动画流畅

2. **新建任务验证**
   - 点击"新建对话"按钮
   - 输入消息发送
   - 确认任务创建成功，消息正常显示

3. **切换任务验证**
   - 创建多个任务
   - 在任务列表中点击不同任务
   - 确认消息正确加载，WebSocket 连接正常
   - 确认能继续发送消息

## 影响范围

- Solo 模式（全栈工程师 Agent 调试模式）
- 不影响工作流模式和其他 Agent 调试模式