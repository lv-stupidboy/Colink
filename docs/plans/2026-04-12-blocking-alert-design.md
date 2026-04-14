# Agent 阻塞提醒弹框设计文档

> **日期**: 2026-04-12
> **状态**: 已确认，待实现

---

## 一、需求背景

Agent 在执行过程中存在多种需要用户介入的场景：
1. **Agent主动提问** - Agent 在执行中提出问题
2. **tool_use需要确认** - 敏感工具调用前需用户审批
3. **任务阻塞状态** - 有阻塞项需用户处理

当前这些场景仅通过消息展示，用户可能错过关键阻塞点，导致任务停滞。

---

## 二、核心约束

1. **弹框时机**：仅在所有 Agent 运行完成后才显示弹框提醒
2. **优先级排序**：按优先级只显示最高优先级的阻塞项
3. **全局开关**：通用设置中提供提醒开关，用户可屏蔽弹框提醒

---

## 三、优先级规则

优先级顺序（高→低）：

| 优先级 | 阻塞类型 | 说明 |
|--------|----------|------|
| 3（最高） | tool_confirm | 工具执行前必须确认，影响安全操作 |
| 2（中等） | agent_question | Agent 执行中需要信息才能继续 |
| 1（最低） | task_blocked | 外部阻塞项，可能不立即影响当前执行 |

---

## 四、交互方式

**分类多弹框**：不同类型的阻塞项用不同的弹框样式，但只显示当前最高优先级的那类。

弹框样式区分：
- 工具确认：橙色标签，显示工具名称和参数摘要
- Agent提问：蓝色标签，显示问题内容
- 任务阻塞：红色标签，显示阻塞原因

---

## 五、用户输入框优化

弹框确认后，输入框自动填入发起该阻塞项的 Agent 的 @mention：
- 来源：阻塞项的 `sourceAgentName`
- 可切换：用户可通过 @mention 下拉列表切换其他 Agent
- 提示：显示"已自动填入 @xxx，可切换其他 Agent"

---

## 六、技术方案

采用方案 A：**Zustand Store 扩展 + 阻塞状态管理**

### 6.1 数据结构

```typescript
// types/blocking.ts

/** 阻塞类型 */
export type BlockingType = 'tool_confirm' | 'agent_question' | 'task_blocked';

/** 阻塞优先级（数值越大优先级越高） */
export const BlockingPriority: Record<BlockingType, number> = {
  tool_confirm: 3,
  agent_question: 2,
  task_blocked: 1,
};

/** 阻塞项 */
export interface BlockingItem {
  id: string;
  type: BlockingType;
  priority: number;
  sourceAgentId: string;
  sourceAgentName: string;
  summary: string;
  details?: string[];
  timestamp: number;
  invocationId?: string;
  toolName?: string;
  toolInput?: Record<string, unknown>;
  question?: string;
}
```

### 6.2 Store 扩展

```typescript
// store/index.ts 扩展

interface AppState {
  // 新增
  blockingItems: BlockingItem[];
  blockingReminderEnabled: boolean;
}

interface AppActions {
  // 新增
  addBlockingItem: (item: BlockingItem) => void;
  removeBlockingItem: (id: string) => void;
  clearBlockingItems: () => void;
  setBlockingReminderEnabled: (enabled: boolean) => void;
}

// localStorage 持久化 key
const STORAGE_KEY_BLOCKING_REMINDER = 'isdp_blocking_reminder_enabled';
```

### 6.3 阻塞识别逻辑

```typescript
// utils/blockingDetector.ts

export class BlockingDetector {
  /** 检测 Agent 主动提问 */
  static detectAgentQuestion(
    content: string,
    invocationId: string,
    agentId: string,
    agentName: string
  ): BlockingItem | null;
  
  /** 检测工具确认需求 */
  static detectToolConfirm(
    block: MessageContentBlock,
    invocationId: string,
    agentId: string,
    agentName: string
  ): BlockingItem | null;
  
  /** 检测任务阻塞 */
  static detectTaskBlocked(
    content: string,
    invocationId: string,
    agentId: string,
    agentName: string
  ): BlockingItem | null;
}
```

识别规则：
- **Agent提问**：匹配 `请问|请确认|需要您|请您|是否|请选择|请回答`
- **工具确认**：识别敏感工具（Bash、Write、Edit 等）的 streaming 状态 tool_use
- **任务阻塞**：匹配 `阻塞|等待|暂停|需要处理|待办`

### 6.4 消息处理集成

在 `ThreadView.tsx` 的 `handleWsMessage` 中添加识别：

```typescript
case 'agent_output_chunk': {
  // 文本块：检测 Agent 提问和任务阻塞
  if (chunkType === 'text' && content) {
    const question = BlockingDetector.detectAgentQuestion(...);
    if (question) addBlockingItem(question);
    
    const blocked = BlockingDetector.detectTaskBlocked(...);
    if (blocked) addBlockingItem(blocked);
  }
  
  // tool_use 块：检测工具确认需求
  if (chunkType === 'tool_use') {
    const confirm = BlockingDetector.detectToolConfirm(...);
    if (confirm) addBlockingItem(confirm);
  }
}
```

### 6.5 弹框组件

```typescript
// components/BlockingAlert/BlockingAlertModal.tsx

interface BlockingAlertModalProps {
  visible: boolean;
  blockingItem: BlockingItem | null;
  onConfirm: (item: BlockingItem) => void;
  onReject: (item: BlockingItem) => void;
  onSkip: () => void;
}
```

弹框内容：
- 阻塞类型标签
- 来源 Agent 名称
- 一句话摘要
- 关键信息卡片（最多3条）

### 6.6 弹框触发控制

```typescript
// ThreadView.tsx

useEffect(() => {
  const noActiveAgents = activeAgents.length === 0;
  const shouldShow = blockingReminderEnabled && 
                     blockingItems.length > 0 && 
                     noActiveAgents;
  
  if (shouldShow && !blockingModalVisible) {
    // 按优先级排序，取最高优先级
    const sorted = [...blockingItems].sort((a, b) => b.priority - a.priority);
    setCurrentBlockingItem(sorted[0]);
    setBlockingModalVisible(true);
  }
}, [blockingItems, blockingReminderEnabled, activeAgents.length]);
```

### 6.7 ThreadInput 扩展

```typescript
interface ThreadInputProps {
  // 新增
  prefilledMention?: string;
  onPrefillConsumed?: () => void;
}

// 监听 prefilledMention 自动填入
useEffect(() => {
  if (prefilledMention && agentOptions.some(opt => opt.name === prefilledMention)) {
    setInputValue(`@${prefilledMention} `);
    inputRef.current.focus();
    onPrefillConsumed?.();
  }
}, [prefilledMention]);
```

### 6.8 通用设置开关

```typescript
// pages/Settings/GeneralSettings.tsx

// 阻塞提醒设置卡片
<Card title={<Space><AlertOutlined />阻塞提醒设置</Space>}>
  <Switch
    checked={reminderEnabled}
    onChange={handleReminderChange}
    checkedChildren="开启"
    unCheckedChildren="关闭"
  />
  <Text type="secondary">
    {reminderEnabled ? '阻塞时将弹框提醒' : '已关闭，阻塞项仅在消息区显示'}
  </Text>
</Card>
```

---

## 七、文件改动清单

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `web/src/types/blocking.ts` | 新增 | 阻塞类型和数据结构定义 |
| `web/src/utils/blockingDetector.ts` | 新增 | 阻塞识别规则实现 |
| `web/src/components/BlockingAlert/BlockingAlertModal.tsx` | 新增 | 阻塞提醒弹框组件 |
| `web/src/components/BlockingAlert/index.tsx` | 新增 | 导出入口 |
| `web/src/components/BlockingAlert/BlockingAlertModal.css` | 新增 | 弹框样式 |
| `web/src/store/index.ts` | 修改 | 扩展阻塞状态和 actions |
| `web/src/pages/ThreadView.tsx` | 修改 | 添加阻塞识别、弹框控制、预填入逻辑 |
| `web/src/components/thread/ThreadInput.tsx` | 修改 | 新增 prefilledMention props 和自动填入逻辑 |
| `web/src/pages/Settings/GeneralSettings.tsx` | 修改 | 新增阻塞提醒设置卡片 |

---

## 八、测试要点

1. **识别准确性**：验证三种场景的识别规则是否正确触发
2. **优先级排序**：多阻塞项同时存在时，只显示最高优先级
3. **弹框时机**：Agent 运行中不弹框，完成后才弹框
4. **开关生效**：关闭开关后不弹框，但阻塞项仍记录
5. **@mention 自动填入**：确认后输入框正确填入发起 Agent
6. **切换功能**：用户可通过下拉切换其他 Agent
7. **深色模式**：弹框样式在深色模式下正常显示

---

## 九、后续优化（可选）

- [ ] 阻塞项历史记录（可查看已处理的阻塞项）
- [ ] 批量处理模式（多阻塞项合并处理）
- [ ] 阻塞项声音提示（配合桌面通知）
- [ ] 后端推送优化（后端识别阻塞场景，减少前端计算）