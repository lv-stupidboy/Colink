// isdp/web/src/components/thread/StreamingMessage.tsx
import React, { memo, useEffect, useRef } from 'react';
import { useAppStore } from '@/store';
import { ChatMessage } from './ChatMessage';
import type { AgentConfig, ToolEvent } from '@/types';
import type { ProgressInfo } from './ChatMessage';

interface StreamingMessageProps {
  agentConfigs: AgentConfig[];
  projectPath?: string;
  toolEvents: Record<string, ToolEvent[]>;
  onStop?: (invocationId: string) => void;
}

/**
 * 流式消息组件 - 隔离高频更新
 *
 * 这个组件直接订阅 store 的流式状态，不会触发父组件重渲染
 * 关键优化：使用 selector 订阅，只有流式状态变化时才更新
 */
export const StreamingMessage: React.FC<StreamingMessageProps> = memo(({
  agentConfigs,
  projectPath,
  toolEvents,
  onStop,
}) => {
  // 使用 selector 订阅流式状态
  const isStreaming = useAppStore((s) => s.isStreaming);
  const streamingContent = useAppStore((s) => s.streamingContent);
  const streamingAgentId = useAppStore((s) => s.streamingAgentId);
  const streamingAgentName = useAppStore((s) => s.streamingAgentName);
  const streamingInvocationId = useAppStore((s) => s.streamingInvocationId);

  // 进度状态
  const progressStatus = useAppStore((s) => s.progressStatus);
  const progressToolName = useAppStore((s) => s.progressToolName);
  const progressToolInput = useAppStore((s) => s.progressToolInput);

  // 自动滚动 ref
  const containerRef = useRef<HTMLDivElement>(null);

  // 流式内容变化时自动滚动
  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
    }
  }, [streamingContent]);

  // 没有流式内容时不渲染
  if (!isStreaming || !streamingContent) {
    return null;
  }

  // 创建临时消息对象
  const tempMessage = {
    id: `streaming-${streamingInvocationId || 'current'}`,
    threadId: '',
    role: 'agent' as const,
    agentId: streamingAgentId || '',
    agentName: streamingAgentName || undefined,
    content: streamingContent,
    messageType: 'text' as const,
    createdAt: new Date().toISOString(),
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

  // 工具事件
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
      />
    </div>
  );
});

StreamingMessage.displayName = 'StreamingMessage';