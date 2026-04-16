// isdp/web/src/components/thread/StreamingMessage.tsx
import React, { memo, useRef } from 'react';
import { useAppStore } from '@/store';
import { ChatMessage } from './ChatMessage';
import type { AgentConfig, ToolEvent, MessageContentBlock } from '@/types';
import type { ProgressInfo } from './ChatMessage';

interface StreamingMessageProps {
  agentConfigs: AgentConfig[];
  projectPath?: string;
  toolEvents: Record<string, ToolEvent[]>;
  onStop?: (invocationId: string) => void;
  onQuestionSubmit?: (blockId: string, answers: Record<number, string | string[]>, invocationId: string) => void;
}

/**
 * 流式消息组件 - 隔离高频更新
 *
 * 直接订阅 store 的流式状态，不会触发父组件重渲染
 * 使用 selector 订阅，只有流式状态变化时才更新
 */
export const StreamingMessage: React.FC<StreamingMessageProps> = memo(({
  agentConfigs,
  projectPath,
  toolEvents,
  onStop,
  onQuestionSubmit,
}) => {
  // 订阅流式状态
  const isStreaming = useAppStore((s) => s.isStreaming);
  const streamingContentBlocks = useAppStore((s) => s.streamingContentBlocks);
  const streamingAgentId = useAppStore((s) => s.streamingAgentId);
  const streamingAgentName = useAppStore((s) => s.streamingAgentName);
  const streamingInvocationId = useAppStore((s) => s.streamingInvocationId);

  // 进度状态
  const progressStatus = useAppStore((s) => s.progressStatus);
  const progressToolName = useAppStore((s) => s.progressToolName);
  const progressToolInput = useAppStore((s) => s.progressToolInput);

  // 自动滚动 ref
  const containerRef = useRef<HTMLDivElement>(null);

  // 过滤 streamingContentBlocks：
  // - question 类型：只保留 waiting_user_input 状态的
  //   - waiting_user_input：等待用户输入，需要渲染选项（由 StreamingMessage 渲染）
  //   - success/failed：已提交或失败，由历史消息渲染（filterWaitingQuestions=true 不会过滤这些）
  // - 其他类型：全部保留
  const filteredContentBlocks = streamingContentBlocks.filter((block: MessageContentBlock) => {
    if (block.type === 'question') {
      // 只保留 waiting_user_input 状态的（需要用户交互）
      return block.status === 'waiting_user_input';
    }
    return true;
  });

  // 注意：滚动控制由 ChatMessageList 通过 useAutoScrollControl hook 统一管理
  // 本组件不再直接调用 scrollIntoView

  // 检查是否有需要渲染的 question blocks（只有 waiting_user_input 状态的）
  const hasQuestionToRender = filteredContentBlocks.some(b => b.type === 'question' && b.status === 'waiting_user_input');

  // 没有流式内容时不渲染
  // 但如果有需要渲染的 question blocks，即使 isStreaming 为 false 也需要渲染
  if (!isStreaming && !hasQuestionToRender) {
    return null;
  }
  if (filteredContentBlocks.length === 0) {
    return null;
  }

  // 从 contentBlocks 提取文本内容（用于 content 字段）
  const textContent = filteredContentBlocks
    .filter(b => b.type === 'text')
    .map(b => b.type === 'text' ? b.content : '')
    .join('');

  // 创建临时消息对象
  const tempMessage = {
    id: `streaming-${streamingInvocationId || 'current'}`,
    threadId: '',
    role: 'agent' as const,
    agentId: streamingAgentId || '',
    agentName: streamingAgentName || undefined,
    content: textContent,
    messageType: 'text' as const,
    createdAt: new Date().toISOString(),
    contentBlocks: filteredContentBlocks,
  };

  // 获取 Agent 配置
  const agentConfig = agentConfigs.find(
    (c) => c.id === streamingAgentId || c.name === streamingAgentName
  );

  // 进度状态
  const progress: ProgressInfo | undefined = streamingInvocationId ? {
    status: progressStatus,
    toolName: progressToolName || undefined,
    toolInput: progressToolInput || undefined,
  } : undefined;

  // 工具事件（旧版兼容，用于 ChatMessage 旧版路径）
  const messageToolEvents = streamingInvocationId ? (toolEvents[streamingInvocationId] || []) : [];

  return (
    <div ref={containerRef}>
      <ChatMessage
        message={tempMessage}
        agentConfig={agentConfig}
        agentConfigs={agentConfigs}
        projectPath={projectPath}
        isStreaming={true}
        progress={progress}
        toolEvents={messageToolEvents}
        onStop={onStop && streamingInvocationId ? () => onStop(streamingInvocationId) : undefined}
        onQuestionSubmit={onQuestionSubmit}
      />
    </div>
  );
});

StreamingMessage.displayName = 'StreamingMessage';