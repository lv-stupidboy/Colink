// isdp/web/src/components/thread/ChatMessageList.tsx
import { forwardRef, useRef, useEffect, RefObject, useState, useCallback } from 'react';
import { useAppStore } from '@/store';
import type { AgentConfig, ToolEvent } from '@/types';
import type { FileChange } from '@/types/content';
import { ChatMessage } from './ChatMessage';
import { StreamingMessage } from './StreamingMessage';
// Collapsible panels imported for future integration
// import ToolOutputPanel from '@/components/ToolOutputPanel';
// import ThinkingPanel from '@/components/ThinkingPanel';
import '@/components/CollapsiblePanels.css';

/**
 * 聊天消息列表组件
 * 支持自动滚动到底部、进度状态、终止操作、重试、代码预览、工具事件
 * 支持向上滚动加载历史消息（类似微信）
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
  agentTypes?: { type: string }[];
  projectPath?: string;
  toolEvents?: Record<string, ToolEvent[]>;
  onStopAgent?: (invocationId: string) => void;
  onRetryAgent?: (message: any) => void;
  onOpenCodePanel?: (files: FileChange[]) => void;
  loading?: boolean;
  autoScroll?: boolean;
  onQuestionSubmit?: (blockId: string, answers: Record<number, string | string[]>, invocationId: string) => void;
  hasMoreHistory?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
  onAgentClick?: (agentName: string) => void;  // 点击 Agent 头像/名称的回调
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
export const ChatMessageList = forwardRef<HTMLDivElement, ChatMessageListProps>(
  (props, ref) => {
    const {
      messages,
      agentConfigs,
      agentTypes = [],
      projectPath,
      toolEvents = {},
      onStopAgent,
      onRetryAgent,
      onOpenCodePanel,
      loading = false,
      autoScroll = true,
      onQuestionSubmit,
      hasMoreHistory = false,
      loadingMore = false,
      onLoadMore,
      onAgentClick,
    } = props;

    const internalRef = useRef<HTMLDivElement>(null);
    const listRef = (ref as RefObject<HTMLDivElement>) || internalRef;

    const [isNearBottom, setIsNearBottom] = useState(true);
    const bottomAnchorRef = useRef<HTMLDivElement>(null);
    const streamingContentBlocks = useAppStore((s) => s.streamingContentBlocks);

    // 滚动到底部
    const scrollToBottom = useCallback(() => {
      if (bottomAnchorRef.current && messages.length > 0) {
        bottomAnchorRef.current.scrollIntoView({ behavior: 'instant', block: 'end' });
      }
    }, [messages.length]);

    // 条件自动滚动：只有接近底部时才滚动
    useEffect(() => {
      if (autoScroll && isNearBottom) {
        scrollToBottom();
      }
    }, [messages.length, autoScroll, isNearBottom, scrollToBottom]);

    // 流式内容变化时的滚动
    useEffect(() => {
      if (!autoScroll || streamingContentBlocks.length === 0 || !isNearBottom) return;
      scrollToBottom();
    }, [streamingContentBlocks, autoScroll, isNearBottom, scrollToBottom]);

    // 监听滚动位置，判断是否接近底部
    const handleScroll = useCallback(() => {
      if (!listRef.current) return;

      const { scrollTop, scrollHeight, clientHeight } = listRef.current;
      const nearBottom = scrollHeight - scrollTop - clientHeight < 50;
      setIsNearBottom(nearBottom);

      // 向上滚动加载更多历史（距离顶部小于 100px 时触发）
      if (onLoadMore && !loadingMore && hasMoreHistory && scrollTop < 100) {
        onLoadMore();
      }
    }, [listRef, onLoadMore, loadingMore, hasMoreHistory]);

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
        onScroll={handleScroll}
      >
        {/* 加载更多历史指示器 */}
        {hasMoreHistory && (
          <div
            style={{
              textAlign: 'center',
              padding: '8px 0',
              color: 'var(--text-secondary)',
              fontSize: '13px',
            }}
          >
            {loadingMore ? (
              <span>正在加载历史消息...</span>
            ) : (
              <span style={{ opacity: 0.6 }}>↑ 向上滚动加载更多</span>
            )}
          </div>
        )}

        {/* 已完成的消息列表 */}
        {messages.map((message) => {
          const agentConfig = getAgentConfig(
            message.agentId,
            message.agentName,
            agentConfigs
          );

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
              agentTypes={agentTypes}
              projectPath={projectPath}
              toolEvents={messageToolEvents}
              onRetry={onRetryAgent ? () => onRetryAgent(message) : undefined}
              onOpenCodePanel={onOpenCodePanel}
              onQuestionSubmit={onQuestionSubmit}
              onAgentClick={onAgentClick}
            />
          );
        })}

        {/* 流式消息 - 始终独立渲染，不在消息列表中 */}
        <StreamingMessage
          agentConfigs={agentConfigs}
          projectPath={projectPath}
          toolEvents={toolEvents}
          onStop={onStopAgent}
          onQuestionSubmit={onQuestionSubmit}
        />

        {/* 底部锚点，用于滚动定位 */}
        <div ref={bottomAnchorRef} style={{ height: '1px' }} />
      </div>
    );
  });

ChatMessageList.displayName = 'ChatMessageList';