// isdp/web/src/components/thread/ChatMessageList.tsx
import { forwardRef, useRef, useEffect, RefObject, useState, useCallback } from 'react';
import { useAppStore } from '@/store';
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
    } = props;

    // Use passed ref or create internal one
    const internalRef = useRef<HTMLDivElement>(null);
    const listRef = (ref as RefObject<HTMLDivElement>) || internalRef;

  // 使用自动滚动控制 hook
  const { isNearBottom, bottomAnchorRef } = useAutoScrollControl(listRef);

  // 订阅流式内容块变化（用于滚动控制）
  const streamingContentBlocks = useAppStore((s) => s.streamingContentBlocks);

  // 向上滚动加载历史的逻辑
  const [isLoadingFromTop, setIsLoadingFromTop] = useState(false);
  const prevScrollHeightRef = useRef(0);

  // 检测滚动到顶部
  const handleScroll = useCallback(() => {
    if (!listRef.current || !onLoadMore || loadingMore || !hasMoreHistory) return;

    const { scrollTop, scrollHeight } = listRef.current;
    // 当滚动到顶部附近（小于 100px）时触发加载
    if (scrollTop < 100) {
      // 记录当前滚动高度，用于加载后恢复位置
      prevScrollHeightRef.current = scrollHeight;
      setIsLoadingFromTop(true);
      onLoadMore();
    }
  }, [listRef, onLoadMore, loadingMore, hasMoreHistory]);

  // 加载完成后恢复滚动位置（避免跳动）
  useEffect(() => {
    if (!isLoadingFromTop || loadingMore) return;

    // 当 loadingMore 变为 false 时，表示加载完成
    if (!loadingMore && listRef.current) {
      const newScrollHeight = listRef.current.scrollHeight;
      const addedHeight = newScrollHeight - prevScrollHeightRef.current;
      // 保持滚动位置，使新加载的内容出现在上方
      listRef.current.scrollTop = addedHeight;
      setIsLoadingFromTop(false);
    }
  }, [loadingMore, isLoadingFromTop, listRef]);

  // 条件自动滚动：只有接近底部时才滚动
  // 监听已完成消息数量和流式内容变化
  useEffect(() => {
    if (autoScroll && isNearBottom && bottomAnchorRef.current) {
      bottomAnchorRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
    }
  }, [messages.length, autoScroll, isNearBottom]);

  // 流式内容变化时的滚动：只有接近底部时才滚动
  // 使用整个 streamingContentBlocks 数组作为依赖，这样内容更新也会触发滚动
  useEffect(() => {
    if (autoScroll && isNearBottom && streamingContentBlocks.length > 0 && bottomAnchorRef.current) {
      bottomAnchorRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
    }
  }, [streamingContentBlocks, autoScroll, isNearBottom]);

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