// isdp/web/src/components/thread/ChatMessageList.tsx
import React, { memo, useEffect, useRef } from 'react';
import type { AgentConfig, ToolEvent } from '@/types';
import type { FileChange } from '@/types/content';
import { ChatMessage } from './ChatMessage';
import { StreamingMessage } from './StreamingMessage';
import { useAutoScrollControl } from './useAutoScrollControl';
// Collapsible panels imported for future integration
// import ToolOutputPanel from '@/components/ToolOutputPanel';
// import ThinkingPanel from '@/components/ThinkingPanel';
import '@/components/CollapsiblePanels.css';

/**
 * 聊天消息列表组件
 * 支持自动滚动到底部、进度状态、终止操作、重试、代码预览、工具事件
 *
 * 性能优化：
 * - 流式消息使用独立的 StreamingMessage 组件隔离高频更新
 * - 已完成消息列表不会因流式更新而重渲染
 */

interface ChatMessageListProps {
  messages: Array<{
    id: string;
    threadId: string;
    role: string;
    agentId?: string;
    agentName?: string;
    content: string;
    messageType: string;
    metadata?: Record<string, unknown>;
    createdAt: string;
  }>;
  agentConfigs: AgentConfig[];
  projectPath?: string;
  toolEvents?: Record<string, ToolEvent[]>;
  onStopAgent?: (invocationId: string) => void;
  onRetryAgent?: (message: any) => void;
  onOpenCodePanel?: (files: FileChange[]) => void;
  loading?: boolean;
  autoScroll?: boolean;
  onQuestionSubmit?: (blockId: string, answers: Record<number, string | string[]>, invocationId: string) => void;
}

/**
 * 获取 Agent 配置（通过 ID 或名字）
 */
function getAgentConfig(
  agentId?: string,
  agentName?: string,
  agentConfigs: AgentConfig[] = []
): AgentConfig | undefined {
  if (agentId) {
    return agentConfigs.find((config) => config.id === agentId);
  }
  if (agentName) {
    return agentConfigs.find((config) => config.name === agentName);
  }
  return undefined;
}

/**
 * 消息列表组件
 */
export const ChatMessageList: React.FC<ChatMessageListProps> = memo(({
  messages,
  agentConfigs,
  projectPath,
  toolEvents = {},
  onStopAgent,
  onRetryAgent,
  onOpenCodePanel,
  loading = false,
  autoScroll = true,
  onQuestionSubmit,
}) => {
  const listRef = useRef<HTMLDivElement>(null);

  // 使用自动滚动控制 hook
  const { isNearBottom, bottomAnchorRef } = useAutoScrollControl(listRef);

  // 条件自动滚动：只有接近底部时才滚动
  useEffect(() => {
    if (autoScroll && isNearBottom && bottomAnchorRef.current) {
      bottomAnchorRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
    }
  }, [messages.length, autoScroll, isNearBottom]);

  // 空状态
  if (messages.length === 0) {
    return (
      <div
        className="chat-message-list-empty"
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100%',
          color: '#8c8c8c',
          fontSize: '14px',
        }}
      >
        {loading ? '加载中...' : '暂无消息'}
      </div>
    );
  }

  return (
    <div
      ref={listRef}
      className="chat-message-list"
      style={{
        height: '100%',
        overflowY: 'auto',
        padding: '16px',
      }}
    >
      {/* 已完成的消息 - 不会因流式消息更新而重渲染 */}
      {messages.map((message) => {
        const agentConfig = getAgentConfig(
          message.agentId,
          message.agentName,
          agentConfigs
        );

        // 获取该消息对应的工具事件
        const invocationId = message.id.startsWith('agent-')
          ? message.id.replace('agent-', '')
          : message.id;
        const messageToolEvents = toolEvents[invocationId] || [];

        return (
          <ChatMessage
            key={message.id}
            message={message as any}
            agentConfig={agentConfig}
            agentConfigs={agentConfigs}
            projectPath={projectPath}
            toolEvents={messageToolEvents}
            onRetry={onRetryAgent ? () => onRetryAgent(message) : undefined}
            onOpenCodePanel={onOpenCodePanel}
            onQuestionSubmit={onQuestionSubmit}
          />
        );
      })}

      {/* 流式消息 - 隔离组件，高频更新不影响已完成消息 */}
      <StreamingMessage
        agentConfigs={agentConfigs}
        projectPath={projectPath}
        toolEvents={toolEvents}
        onStop={onStopAgent}
        onQuestionSubmit={onQuestionSubmit}
      />

      {/* 底部锚点 - 用于 IntersectionObserver */}
      <div ref={bottomAnchorRef} style={{ height: '1px' }} />
    </div>
  );
});

ChatMessageList.displayName = 'ChatMessageList';