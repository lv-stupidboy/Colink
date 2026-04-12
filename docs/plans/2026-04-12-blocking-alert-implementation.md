# Agent 阻塞提醒弹框实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现 Agent 阻塞提醒弹框，在 Agent 需要用户回答、工具需确认、任务阻塞时弹框提醒，输入框自动填入 @mention

**Architecture:** Zustand Store 扩展阻塞状态 + 前端消息处理智能识别 + 分类弹框按优先级显示 + ThreadInput 自动填入 @mention

**Tech Stack:** React + TypeScript + Zustand + Ant Design

---

## Task 1: 新增阻塞类型和数据结构定义

**Files:**
- Create: `web/src/types/blocking.ts`

### Step 1: 创建阻塞类型定义文件

在 `web/src/types/` 目录下创建 `blocking.ts`:

```typescript
// web/src/types/blocking.ts

/** 阻塞类型 */
export type BlockingType = 'tool_confirm' | 'agent_question' | 'task_blocked';

/** 阻塞优先级（数值越大优先级越高） */
export const BlockingPriority: Record<BlockingType, number> = {
  tool_confirm: 3,    // 最高：工具执行前必须确认
  agent_question: 2,  // 中等：Agent需要信息才能继续
  task_blocked: 1,    // 最低：外部阻塞项
};

/** 阻塞项 */
export interface BlockingItem {
  id: string;                    // 唯一标识
  type: BlockingType;            // 阻塞类型
  priority: number;              // 优先级数值
  sourceAgentId: string;         // 来源 Agent ID
  sourceAgentName: string;       // 来源 Agent 名称
  summary: string;               // 一句话摘要
  details?: string[];            // 关键信息列表（最多展示3条）
  timestamp: number;             // 阻塞发生时间
  invocationId?: string;         // 关联的 invocation ID（用于工具确认）
  toolName?: string;             // 工具名称（仅 tool_confirm）
  toolInput?: Record<string, unknown>;  // 工具参数（仅 tool_confirm）
  question?: string;             // Agent 提问内容（仅 agent_question）
}
```

### Step 2: 验证 TypeScript 编译

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run build
```

Expected: 编译成功，无 TypeScript 错误

### Step 3: Commit

```bash
cd D:/CoLinkProject/Colink-0412/isdp
git add web/src/types/blocking.ts
git commit -m "$(cat <<'EOF'
feat(types): 新增阻塞类型和数据结构定义

定义 BlockingType、BlockingPriority、BlockingItem 结构

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: 新增阻塞识别规则实现

**Files:**
- Create: `web/src/utils/blockingDetector.ts`

### Step 1: 创建阻塞识别工具类

在 `web/src/utils/` 目录下创建 `blockingDetector.ts`:

```typescript
// web/src/utils/blockingDetector.ts

import type { BlockingItem, BlockingType } from '@/types/blocking';
import { BlockingPriority } from '@/types/blocking';
import type { MessageContentBlock } from '@/types';

/** 需要确认的敏感工具列表 */
const SENSITIVE_TOOLS = [
  'Bash',
  'Write',
  'Edit',
  'execute_bash_command',
  'run_code',
];

/** 阻塞识别规则 */
export class BlockingDetector {
  
  /**
   * 检测 Agent 主动提问
   * 匹配输出中的问句关键词
   */
  static detectAgentQuestion(
    content: string,
    invocationId: string,
    agentId: string,
    agentName: string
  ): BlockingItem | null {
    // 问句关键词模式
    const questionPatterns = [
      /请问[：:\s]*(.{10,100})/,
      /请确认[：:\s]*(.{10,100})/,
      /需要您[：:\s]*(.{10,100})/,
      /请您[：:\s]*(.{10,100})/,
      /是否[？\s]*(.{10,100})/,
      /请选择[：:\s]*(.{10,100})/,
      /请回答[：:\s]*(.{10,100})/,
    ];
    
    for (const pattern of questionPatterns) {
      const match = content.match(pattern);
      if (match) {
        const question = match[1]?.trim() || match[0];
        return {
          id: `question-${invocationId}-${Date.now()}`,
          type: 'agent_question',
          priority: BlockingPriority.agent_question,
          sourceAgentId: agentId,
          sourceAgentName: agentName,
          summary: `Agent提问：${question.slice(0, 50)}${question.length > 50 ? '...' : ''}`,
          question: question,
          timestamp: Date.now(),
          invocationId,
        };
      }
    }
    return null;
  }
  
  /**
   * 检测工具确认需求
   * 识别敏感工具的 streaming 状态 tool_use
   */
  static detectToolConfirm(
    block: MessageContentBlock,
    invocationId: string,
    agentId: string,
    agentName: string
  ): BlockingItem | null {
    if (block.type !== 'tool_use') {
      return null;
    }
    
    // 检查是否是 streaming 状态（未完成）
    if ('status' in block && block.status !== 'streaming') {
      return null;
    }
    
    const toolName = block.toolName || '';
    
    // 检查是否是敏感工具
    const isSensitive = SENSITIVE_TOOLS.some(t => 
      toolName.toLowerCase().includes(t.toLowerCase())
    );
    
    if (!isSensitive) {
      return null;
    }
    
    // 提取关键参数摘要
    const inputSummary = this.extractToolInputSummary(
      'input' in block ? (block.input as Record<string, unknown>) || {} : {}
    );
    
    return {
      id: `tool-${block.toolId || Date.now()}`,
      type: 'tool_confirm',
      priority: BlockingPriority.tool_confirm,
      sourceAgentId: agentId,
      sourceAgentName: agentName,
      summary: `工具确认：${toolName}`,
      details: inputSummary,
      timestamp: Date.now(),
      invocationId,
      toolName,
      toolInput: 'input' in block ? block.input as Record<string, unknown> : undefined,
    };
  }
  
  /**
   * 检测任务阻塞
   * 匹配阻塞关键词
   */
  static detectTaskBlocked(
    content: string,
    invocationId: string,
    agentId: string,
    agentName: string
  ): BlockingItem | null {
    const blockedPatterns = [
      /阻塞[：:\s]*(.{10,100})/,
      /等待[：:\s]*(.{10,100})/,
      /暂停[：:\s]*(.{10,100})/,
      /需要处理[：:\s]*(.{10,100})/,
      /待办[：:\s]*(.{10,100})/,
    ];
    
    for (const pattern of blockedPatterns) {
      const match = content.match(pattern);
      if (match) {
        const reason = match[1]?.trim() || match[0];
        return {
          id: `blocked-${invocationId}-${Date.now()}`,
          type: 'task_blocked',
          priority: BlockingPriority.task_blocked,
          sourceAgentId: agentId,
          sourceAgentName: agentName,
          summary: `任务阻塞：${reason.slice(0, 50)}${reason.length > 50 ? '...' : ''}`,
          details: [reason],
          timestamp: Date.now(),
          invocationId,
        };
      }
    }
    return null;
  }
  
  /**
   * 提取工具输入参数摘要（最多3条关键信息）
   */
  private static extractToolInputSummary(input: Record<string, unknown>): string[] {
    const summary: string[] = [];
    const priorityKeys = ['file_path', 'path', 'command', 'code', 'content', 'url'];
    
    for (const key of priorityKeys) {
      if (input[key] !== undefined && input[key] !== null) {
        const value = String(input[key]);
        const truncated = value.length > 60 ? value.slice(0, 60) + '...' : value;
        summary.push(`${key}: ${truncated}`);
      }
    }
    
    return summary.slice(0, 3);
  }
}
```

### Step 2: 验证 TypeScript 编译

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run build
```

Expected: 编译成功

### Step 3: Commit

```bash
cd D:/CoLinkProject/Colink-0412/isdp
git add web/src/utils/blockingDetector.ts
git commit -m "$(cat <<'EOF'
feat(utils): 新增 BlockingDetector 阻塞识别规则实现

支持三种场景识别：Agent提问、工具确认、任务阻塞

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: 扩展 Zustand Store 阻塞状态

**Files:**
- Modify: `web/src/store/index.ts`

### Step 1: 添加阻塞相关 import

在文件顶部 import 区域添加：

```typescript
// 在现有 import 后添加
import type { BlockingItem } from '@/types/blocking';

// localStorage 持久化 key（常量）
const STORAGE_KEY_BLOCKING_REMINDER = 'isdp_blocking_reminder_enabled';
```

### Step 2: 扩展 AppState 接口

找到 `AppState` 接口定义（约第 7 行），添加阻塞状态字段：

```typescript
interface AppState {
  // ... 现有字段保持不变

  // 阻塞提醒相关（新增）
  blockingItems: BlockingItem[];
  blockingReminderEnabled: boolean;
}
```

### Step 3: 扩展 AppActions 接口

找到 `AppActions` 接口定义（约第 83 行），添加阻塞管理方法：

```typescript
interface AppActions {
  // ... 现有方法保持不变

  // 阻塞管理（新增）
  addBlockingItem: (item: BlockingItem) => void;
  removeBlockingItem: (id: string) => void;
  clearBlockingItems: () => void;
  setBlockingReminderEnabled: (enabled: boolean) => void;
}
```

### Step 4: 扩展 initialState

找到 `initialState` 定义（约第 191 行），添加阻塞初始状态：

```typescript
const initialState: AppState = {
  // ... 现有字段保持不变

  // 阻塞提醒相关（新增）
  blockingItems: [],
  blockingReminderEnabled: localStorage.getItem(STORAGE_KEY_BLOCKING_REMINDER) !== 'false',
};
```

### Step 5: 实现 blocking actions

在 `subscribeWithSelector` 内部，找到最后一个 action（约第 945 行 `setCollapsibleDefaults`），在其后添加：

```typescript
    // 阻塞管理 actions
    addBlockingItem: (item) => {
      set((state) => {
        // 去重检查：相同 invocationId + type 不重复添加
        const exists = state.blockingItems.some(
          (b) => b.invocationId === item.invocationId && b.type === item.type
        );
        if (exists) {
          return state;
        }
        return {
          blockingItems: [...state.blockingItems, item],
        };
      });
    },

    removeBlockingItem: (id) => {
      set((state) => ({
        blockingItems: state.blockingItems.filter((b) => b.id !== id),
      }));
    },

    clearBlockingItems: () => {
      set({ blockingItems: [] });
    },

    setBlockingReminderEnabled: (enabled) => {
      set({ blockingReminderEnabled: enabled });
      localStorage.setItem(STORAGE_KEY_BLOCKING_REMINDER, String(enabled));
    },
```

### Step 6: 验证 TypeScript 编译

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run build
```

Expected: 编译成功

### Step 7: Commit

```bash
cd D:/CoLinkProject/Colink-0412/isdp
git add web/src/store/index.ts
git commit -m "$(cat <<'EOF'
feat(store): 扩展 Zustand Store 阻塞状态管理

新增 blockingItems、blockingReminderEnabled 状态和相关 actions
支持 localStorage 持久化提醒开关

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: 新增阻塞提醒弹框组件

**Files:**
- Create: `web/src/components/BlockingAlert/BlockingAlertModal.tsx`
- Create: `web/src/components/BlockingAlert/index.tsx`
- Create: `web/src/components/BlockingAlert/BlockingAlertModal.css`

### Step 1: 创建弹框组件

创建 `web/src/components/BlockingAlert/BlockingAlertModal.tsx`:

```typescript
// web/src/components/BlockingAlert/BlockingAlertModal.tsx

import React from 'react';
import { Modal, Tag, Space, Typography, Card, List, Button, Alert } from 'antd';
import {
  QuestionCircleOutlined,
  ToolOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import type { BlockingItem, BlockingType } from '@/types/blocking';
import './BlockingAlertModal.css';

const { Text, Paragraph } = Typography;

interface BlockingAlertModalProps {
  visible: boolean;
  blockingItem: BlockingItem | null;
  onConfirm: (item: BlockingItem) => void;
  onReject: (item: BlockingItem) => void;
  onSkip: () => void;
}

/** 阻塞类型配置 */
const BlockingTypeConfig: Record<BlockingType, {
  icon: React.ReactNode;
  color: string;
  title: string;
  confirmText: string;
  rejectText: string;
}> = {
  tool_confirm: {
    icon: <ToolOutlined />,
    color: 'orange',
    title: '工具执行确认',
    confirmText: '确认执行',
    rejectText: '拒绝执行',
  },
  agent_question: {
    icon: <QuestionCircleOutlined />,
    color: 'blue',
    title: 'Agent 需要您的回答',
    confirmText: '去回答',
    rejectText: '稍后处理',
  },
  task_blocked: {
    icon: <WarningOutlined />,
    color: 'red',
    title: '任务阻塞提醒',
    confirmText: '去处理',
    rejectText: '暂不处理',
  },
};

export const BlockingAlertModal: React.FC<BlockingAlertModalProps> = ({
  visible,
  blockingItem,
  onConfirm,
  onReject,
  onSkip,
}) => {
  if (!blockingItem) return null;

  const config = BlockingTypeConfig[blockingItem.type];

  /** 渲染工具确认内容 */
  const renderToolConfirmContent = () => (
    <>
      <Alert
        type="warning"
        message={`Agent "${blockingItem.sourceAgentName}" 正在请求执行敏感操作`}
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Card size="small" title="工具信息" style={{ marginBottom: 12 }}>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Text strong>工具名称：{blockingItem.toolName}</Text>
          {blockingItem.details && blockingItem.details.length > 0 && (
            <>
              <Text type="secondary">参数摘要：</Text>
              <List
                size="small"
                dataSource={blockingItem.details}
                renderItem={(item) => (
                  <List.Item style={{ padding: '4px 0', border: 'none' }}>
                    <Text code style={{ fontSize: 12 }}>{item}</Text>
                  </List.Item>
                )}
              />
            </>
          )}
        </Space>
      </Card>

      <Text type="secondary" style={{ fontSize: 12 }}>
        确认后将执行此操作，拒绝将终止本次工具调用
      </Text>
    </>
  );

  /** 渲染 Agent 提问内容 */
  const renderAgentQuestionContent = () => (
    <>
      <Alert
        type="info"
        message={`Agent "${blockingItem.sourceAgentName}" 有问题需要您回答`}
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Card size="small" title="问题内容" style={{ marginBottom: 12 }}>
        <Paragraph style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
          {blockingItem.question || blockingItem.summary}
        </Paragraph>
      </Card>

      <Text type="secondary" style={{ fontSize: 12 }}>
        请在输入框中回复，输入框已自动填入 @{blockingItem.sourceAgentName}
      </Text>
    </>
  );

  /** 渲染任务阻塞内容 */
  const renderTaskBlockedContent = () => (
    <>
      <Alert
        type="error"
        message={`Agent "${blockingItem.sourceAgentName}" 遇到阻塞`}
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Card size="small" title="阻塞原因" style={{ marginBottom: 12 }}>
        <Paragraph style={{ margin: 0 }}>
          {blockingItem.summary}
        </Paragraph>
        {blockingItem.details && blockingItem.details.length > 0 && (
          <List
            size="small"
            dataSource={blockingItem.details}
            renderItem={(item) => (
              <List.Item style={{ padding: '4px 0', border: 'none' }}>
                <Text>{item}</Text>
              </List.Item>
            )}
            style={{ marginTop: 8 }}
          />
        )}
      </Card>

      <Text type="secondary" style={{ fontSize: 12 }}>
        请处理阻塞项后再继续任务
      </Text>
    </>
  );

  const renderContent = () => {
    switch (blockingItem.type) {
      case 'tool_confirm':
        return renderToolConfirmContent();
      case 'agent_question':
        return renderAgentQuestionContent();
      case 'task_blocked':
        return renderTaskBlockedContent();
      default:
        return null;
    }
  };

  return (
    <Modal
      title={
        <Space>
          <Tag color={config.color}>
            {config.icon}
          </Tag>
          <span>{config.title}</span>
        </Space>
      }
      open={visible}
      onCancel={onSkip}
      footer={
        <Space style={{ width: '100%', justifyContent: 'space-between' }}>
          <Button onClick={onSkip}>关闭</Button>
          <Space>
            <Button onClick={() => onReject(blockingItem)}>
              {config.rejectText}
            </Button>
            <Button type="primary" onClick={() => onConfirm(blockingItem)}>
              {config.confirmText}
            </Button>
          </Space>
        </Space>
      }
      width={500}
      centered
      className="blocking-alert-modal"
    >
      {renderContent()}
    </Modal>
  );
};

export default BlockingAlertModal;
```

### Step 2: 创建样式文件

创建 `web/src/components/BlockingAlert/BlockingAlertModal.css`:

```css
/* BlockingAlertModal.css */

.blocking-alert-modal .ant-modal-body {
  padding: 16px 24px;
}

.blocking-alert-modal .ant-card-body {
  padding: 12px;
}

.blocking-alert-modal .ant-list-item {
  padding: 4px 0;
}

/* 深色模式适配 */
[data-theme='dark'] .blocking-alert-modal .ant-card {
  background: rgba(255, 255, 255, 0.04);
}

[data-theme='dark'] .blocking-alert-modal .ant-card-head {
  background: transparent;
}

[data-theme='dark'] .blocking-alert-modal code {
  background: rgba(255, 255, 255, 0.08);
}
```

### Step 3: 创建导出入口

创建 `web/src/components/BlockingAlert/index.tsx`:

```typescript
// web/src/components/BlockingAlert/index.tsx

export { BlockingAlertModal } from './BlockingAlertModal';
export { default } from './BlockingAlertModal';
```

### Step 4: 验证编译

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run build
```

Expected: 编译成功

### Step 5: Commit

```bash
cd D:/CoLinkProject/Colink-0412/isdp
git add web/src/components/BlockingAlert/
git commit -m "$(cat <<'EOF'
feat(components): 新增 BlockingAlertModal 阻塞提醒弹框组件

支持三种阻塞类型的分类弹框样式
深色模式适配

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: ThreadInput 扩展自动填入 @mention

**Files:**
- Modify: `web/src/components/thread/ThreadInput.tsx`

### Step 1: 扩展 ThreadInputProps 接口

找到接口定义（约第 16 行），添加新 props：

```typescript
interface ThreadInputProps {
  placeholder: string;
  loadingContext: boolean;
  agentOptions: AgentOption[];
  onSend: (content: string) => void;
  disabled?: boolean;
  prefilledMention?: string;       // 新增：预填入的 @mention 名称
  onPrefillConsumed?: () => void;  // 新增：预填入被使用后的回调
}
```

### Step 2: 更新组件参数

找到组件定义（约第 28 行），更新参数接收：

```typescript
export const ThreadInput: React.FC<ThreadInputProps> = memo(({
  placeholder,
  loadingContext,
  agentOptions,
  onSend,
  disabled = false,
  prefilledMention,          // 新增
  onPrefillConsumed,         // 新增
}) => {
```

### Step 3: 添加预填入状态

在现有 useState 声明后（约第 37 行），添加预填入提示状态：

```typescript
  const [inputValue, setInputValue] = useState('');
  const [mentionListVisible, setMentionListVisible] = useState(false);
  const [mentionFilter, setMentionFilter] = useState('');
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const [showPrefillHint, setShowPrefillHint] = useState(false);  // 新增
```

### Step 4: 添加自动填入 useEffect

在现有 useEffect 之后（约第 102 行），添加预填入逻辑：

```typescript
  // 自动填入 @mention（阻塞确认后触发）
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

### Step 5: 更新渲染部分添加预填入提示

找到 return 部分（约第 143 行），在输入区域顶部添加提示：

```typescript
  return (
    <div className="thread-input" style={{ display: 'flex', gap: '12px', padding: '12px 16px' }}>
      <div style={{ position: 'relative', flex: 1 }}>
        {/* 预填入提示 */}
        {showPrefillHint && (
          <div
            className="prefill-hint"
            style={{
              position: 'absolute',
              top: -28,
              left: 0,
              padding: '4px 8px',
              background: 'var(--color-primary-opacity-10, rgba(24, 144, 255, 0.1))',
              borderRadius: 4,
              fontSize: 12,
              color: 'var(--color-primary, #1890ff)',
              whiteSpace: 'nowrap',
            }}
          >
            已自动填入 @{prefilledMention}，可切换其他 Agent
          </div>
        )}

        {/* 切换按钮 - 当已有 @mention 时显示 */}
        {inputValue.startsWith('@') && !mentionListVisible && (
          <Button
            size="small"
            type="text"
            className="mention-switch-btn"
            onClick={() => {
              setMentionListVisible(true);
              setMentionFilter('');
              setHighlightedIndex(0);
            }}
            style={{
              position: 'absolute',
              right: 8,
              top: 8,
              zIndex: 10,
              color: 'var(--text-secondary)',
            }}
          >
            切换
          </Button>
        )}

        {/* TextArea - 保持原有代码 */}
        <TextArea
          ref={inputRef}
          value={inputValue}
          onChange={handleInputChange}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          autoSize={{ minRows: 2, maxRows: 6 }}
          disabled={disabled}
        />
        
        {/* mention dropdown - 保持原有代码 */}
        {mentionListVisible && (
          // ... 保持原有实现不变
        )}
      </div>
      
      {/* 发送按钮 - 保持原有代码 */}
      <Space direction="vertical">
        <Button type="primary" icon={<SendOutlined />} onClick={handleSend} disabled={disabled || !inputValue.trim()}>
          发送
        </Button>
      </Space>
    </div>
  );
```

### Step 6: 验证编译

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run build
```

Expected: 编译成功

### Step 7: Commit

```bash
cd D:/CoLinkProject/Colink-0412/isdp
git add web/src/components/thread/ThreadInput.tsx
git commit -m "$(cat <<'EOF'
feat(ThreadInput): 扩展自动填入 @mention 功能

新增 prefilledMention、onPrefillConsumed props
添加预填入提示和切换按钮

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: ThreadView 集成阻塞识别和弹框控制

**Files:**
- Modify: `web/src/pages/ThreadView.tsx`

### Step 1: 添加 import

在文件顶部 import 区域（约第 1-30 行），添加：

```typescript
import { BlockingAlertModal } from '@/components/BlockingAlert';
import { BlockingDetector } from '@/utils/blockingDetector';
import type { BlockingItem } from '@/types/blocking';
import { useAppStore } from '@/store';
```

### Step 2: 添加阻塞弹框状态

找到组件内的 useState 声明区域（约第 80 行），添加：

```typescript
  // 阻塞提醒弹框状态
  const [blockingModalVisible, setBlockingModalVisible] = useState(false);
  const [currentBlockingItem, setCurrentBlockingItem] = useState<BlockingItem | null>(null);
  const [prefilledMention, setPrefilledMention] = useState<string | null>(null);
```

### Step 3: 从 Store 获取阻塞状态

找到现有 useAppStore 获取部分，添加阻塞状态：

```typescript
  // 现有 Store 状态获取
  const activeAgents = useAppStore((state) => state.activeAgents);
  
  // 新增阻塞状态获取
  const blockingItems = useAppStore((state) => state.blockingItems);
  const blockingReminderEnabled = useAppStore((state) => state.blockingReminderEnabled);
  const addBlockingItem = useAppStore((state) => state.addBlockingItem);
  const removeBlockingItem = useAppStore((state) => state.removeBlockingItem);
```

### Step 4: 添加阻塞弹框触发 useEffect

在现有 useEffect 区域后添加：

```typescript
  // 阻塞弹框触发控制：仅在所有 Agent 完成后显示
  useEffect(() => {
    const noActiveAgents = activeAgents.length === 0;
    const shouldShow = blockingReminderEnabled && 
                       blockingItems.length > 0 && 
                       noActiveAgents;

    if (shouldShow && !blockingModalVisible) {
      // 按优先级排序，取最高优先级的阻塞项
      const sorted = [...blockingItems].sort((a, b) => b.priority - a.priority);
      setCurrentBlockingItem(sorted[0]);
      setBlockingModalVisible(true);
    }
  }, [blockingItems, blockingReminderEnabled, activeAgents.length, blockingModalVisible]);
```

### Step 5: 添加阻塞处理函数

在组件内添加阻塞确认、拒绝、跳过处理函数：

```typescript
  // 处理阻塞确认：清除阻塞项，自动填入 @mention
  const handleBlockingConfirm = (item: BlockingItem) => {
    removeBlockingItem(item.id);
    setBlockingModalVisible(false);
    setPrefilledMention(item.sourceAgentName);
  };

  // 处理阻塞拒绝：清除阻塞项
  const handleBlockingReject = (item: BlockingItem) => {
    removeBlockingItem(item.id);
    setBlockingModalVisible(false);
  };

  // 处理阻塞跳过：关闭弹框但不清除阻塞项
  const handleBlockingSkip = () => {
    setBlockingModalVisible(false);
  };

  // 预填入被使用后清除
  const handlePrefillConsumed = () => {
    setPrefilledMention(null);
  };
```

### Step 6: 在 handleWsMessage 中添加阻塞识别

找到 `handleWsMessage` 函数中的 `agent_output_chunk` case（约第 558 行），在现有逻辑前添加阻塞识别：

```typescript
      case 'agent_output_chunk': {
        const chunkType = data.payload.chunkType as string || 'text';
        const invocId = data.payload.invocationId as string;
        const agentId = data.payload.agentId as string || '';
        const agentName = data.payload.agentName as string || '';
        const content = data.payload.chunk as string;

        // === 阻塞识别（新增）===
        // 文本块：检测 Agent 提问和任务阻塞
        if (chunkType === 'text' && content) {
          const question = BlockingDetector.detectAgentQuestion(content, invocId, agentId, agentName);
          if (question) addBlockingItem(question);

          const blocked = BlockingDetector.detectTaskBlocked(content, invocId, agentId, agentName);
          if (blocked) addBlockingItem(blocked);
        }

        // tool_use 块：检测工具确认需求
        if (chunkType === 'tool_use') {
          const toolBlock: MessageContentBlock = {
            id: data.payload.toolId as string || '',
            type: 'tool_use',
            toolName: data.payload.toolName as string,
            toolId: data.payload.toolId as string,
            input: data.payload.toolInput as Record<string, unknown>,
            status: 'streaming',
            timestamp: Date.now(),
          };
          const confirm = BlockingDetector.detectToolConfirm(toolBlock, invocId, agentId, agentName);
          if (confirm) addBlockingItem(confirm);
        }
        // === 阻塞识别结束 ===

        // === 现有处理逻辑保持不变 ===
        if (chunkType === 'thinking') {
          // ... 保持原有代码
        } else if (chunkType === 'tool_use') {
          // ... 保持原有代码
        } else if (chunkType === 'tool_result') {
          // ... 保持原有代码
        } else if (chunkType === 'text') {
          // ... 保持原有代码
        }
        break;
      }
```

### Step 7: 更新 ThreadInput 调用

找到 ThreadInput 组件的渲染位置（约第 1050 行），更新 props：

```typescript
        <ThreadInput
          placeholder={isDebugMode ? "输入消息..." : "输入消息，@ 可触发指定 Agent"}
          loadingContext={loadingProjectContext}
          agentOptions={filteredAgents.map(a => ({
            id: a.id,
            role: a.role,
            name: a.name,
            label: a.name,
          }))}
          onSend={handleSendMessage}
          disabled={isStreaming}
          prefilledMention={prefilledMention}
          onPrefillConsumed={handlePrefillConsumed}
        />
```

### Step 8: 添加 BlockingAlertModal 渲染

在组件 return 的最后（Modal 区域），添加阻塞弹框：

```typescript
      {/* 阻塞提醒弹框 */}
      <BlockingAlertModal
        visible={blockingModalVisible}
        blockingItem={currentBlockingItem}
        onConfirm={handleBlockingConfirm}
        onReject={handleBlockingReject}
        onSkip={handleBlockingSkip}
      />
```

### Step 9: 验证编译

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run build
```

Expected: 编译成功，无 TypeScript 错误

### Step 10: Commit

```bash
cd D:/CoLinkProject/Colink-0412/isdp
git add web/src/pages/ThreadView.tsx
git commit -m "$(cat <<'EOF'
feat(ThreadView): 集成阻塞识别和弹框控制

- handleWsMessage 中添加阻塞识别逻辑
- 添加阻塞弹框触发控制 useEffect
- ThreadInput 传入 prefilledMention props
- 渲染 BlockingAlertModal 组件

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: 通用设置新增阻塞提醒开关

**Files:**
- Modify: `web/src/pages/Settings/GeneralSettings.tsx`

### Step 1: 添加 import

在文件顶部 import 区域添加：

```typescript
import { AlertOutlined } from '@ant-design/icons';
import { useAppStore } from '@/store';
```

### Step 2: 添加阻塞提醒状态

找到组件内（约第 13 行），添加状态管理：

```typescript
const GeneralSettings: React.FC = () => {
  const [form] = Form.useForm();
  const [reminderEnabled, setReminderEnabled] = useState(true);

  // 从 Store 获取设置阻塞提醒的 action
  const setBlockingReminderEnabled = useAppStore((state) => state.setBlockingReminderEnabled);

  // 初始化时从 localStorage 读取
  useEffect(() => {
    const stored = localStorage.getItem('isdp_blocking_reminder_enabled');
    setReminderEnabled(stored !== 'false');  // 默认 true
  }, []);

  // 实时保存开关状态
  const handleReminderChange = (checked: boolean) => {
    setReminderEnabled(checked);
    localStorage.setItem('isdp_blocking_reminder_enabled', String(checked));
    setBlockingReminderEnabled(checked);
    message.success(checked ? '已开启阻塞提醒' : '已关闭阻塞提醒');
  };
```

### Step 3: 添加阻塞提醒设置卡片

在 API 配置卡片之前（约第 33 行），添加阻塞提醒卡片：

```typescript
      {/* 阻塞提醒设置卡片 */}
      <Card
        title={
          <Space>
            <AlertOutlined />
            阻塞提醒设置
          </Space>
        }
        style={{ marginBottom: 16 }}
      >
        <Form layout="vertical">
          <Form.Item
            label="阻塞提醒弹框"
            tooltip="当 Agent 需要用户回答、工具需要确认、或任务阻塞时，弹框提醒用户处理"
          >
            <Space direction="vertical">
              <Space>
                <Switch
                  checked={reminderEnabled}
                  onChange={handleReminderChange}
                  checkedChildren="开启"
                  unCheckedChildren="关闭"
                />
                <Text type="secondary">
                  {reminderEnabled ? '阻塞时将弹框提醒' : '已关闭，阻塞项仅在消息区显示'}
                </Text>
              </Space>
              <Text type="secondary" style={{ fontSize: 12, marginTop: 8 }}>
                提醒场景包括：Agent 主动提问、敏感工具执行前确认、任务遇到阻塞
              </Text>
            </Space>
          </Form.Item>
        </Form>
      </Card>

      {/* API 配置卡片 - 保持原有代码 */}
```

### Step 4: 验证编译

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run build
```

Expected: 编译成功

### Step 5: Commit

```bash
cd D:/CoLinkProject/Colink-0412/isdp
git add web/src/pages/Settings/GeneralSettings.tsx
git commit -m "$(cat <<'EOF'
feat(GeneralSettings): 新增阻塞提醒开关

支持全局开启/关闭阻塞弹框提醒
localStorage 持久化开关状态

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: 验证与测试

### Step 1: 启动前端开发服务器

```bash
cd D:/CoLinkProject/Colink-0412/isdp/web
npm run dev
```

Expected: 前端服务启动成功

### Step 2: 手动测试阻塞识别

1. 打开浏览器访问前端
2. 创建一个 Thread，触发 Agent 执行
3. 在 Agent 输出中模拟包含阻塞关键词（如"请确认是否继续"）
4. 等待 Agent 完成后，观察是否弹出阻塞提醒

### Step 3: 测试弹框优先级

测试场景：
- 同时有工具确认和 Agent 提问，应只显示工具确认弹框

### Step 4: 测试开关功能

1. 进入通用设置页面
2. 关闭阻塞提醒开关
3. 回到 Thread，触发阻塞场景
4. 验证不再弹框，但阻塞项仍在消息中显示

### Step 5: 测试 @mention 自动填入

1. 触发阻塞弹框
2. 点击"去回答"或"去处理"
3. 验证输入框已自动填入 `@Agent名称`
4. 测试切换按钮功能

### Step 6: 测试深色模式

1. 切换到深色主题
2. 触发阻塞弹框
3. 验证弹框样式在深色模式下正常显示

### Step 7: 最终 Commit（如有修复）

```bash
cd D:/CoLinkProject/Colink-0412/isdp
git add -A
git commit -m "$(cat <<'EOF'
fix: 阻塞提醒功能测试修复

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## 总结

改动文件清单：

| 文件 | 改动类型 |
|------|----------|
| `web/src/types/blocking.ts` | 新增 |
| `web/src/utils/blockingDetector.ts` | 新增 |
| `web/src/components/BlockingAlert/BlockingAlertModal.tsx` | 新增 |
| `web/src/components/BlockingAlert/index.tsx` | 新增 |
| `web/src/components/BlockingAlert/BlockingAlertModal.css` | 新增 |
| `web/src/store/index.ts` | 修改 |
| `web/src/pages/ThreadView.tsx` | 修改 |
| `web/src/components/thread/ThreadInput.tsx` | 修改 |
| `web/src/pages/Settings/GeneralSettings.tsx` | 修改 |

关键改动：
- 三种阻塞场景智能识别
- 优先级排序弹框显示
- 输入框自动填入 @mention
- 通用设置提醒开关（localStorage 持久化）