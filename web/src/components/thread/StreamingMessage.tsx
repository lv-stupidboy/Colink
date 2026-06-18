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
  onInteractiveAction?: (blockId: string, action: string, value?: string | string[]) => void;
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
  onInteractiveAction,
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

  // streaming 阶段对 question block 不再做状态过滤，直接全量保留：
  //   - waiting_user_input：用户交互中，渲染选项卡片
  //   - success / failed：用户已提交答案，渲染"已回答"卡片显示用户选择
  //
  // 注意：原先这里只保留 waiting_user_input、把 success/failed 让"历史消息渲染"，是按
  // native CLI 模式假设——提交答案 == invocation 收尾 == streamingContentBlocks 被
  // finalizeStreamingMessage 移到 messages。但 ACP elicitation 模式下用户提交后
  // invocation 仍 running（同一 prompt turn 内异步等答案），question block 仍在
  // streamingContentBlocks 中，过滤掉的话用户提交完会"卡片直接消失"，看不到自己的
  // 答案。
  // 已提交的 question block 不会被重复渲染——agent_message 事件触发的
  // finalizeStreamingMessage 会把它的 ID 加进 submittedQuestionBlockIds，历史消息
  // 渲染时再用 submittedQuestionBlockIds 跳过。
  const filteredContentBlocks = streamingContentBlocks.filter((block: MessageContentBlock) => {
    if (block.type === 'question') {
      return true;
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
        onInteractiveAction={onInteractiveAction}
      />
    </div>
  );
});

StreamingMessage.displayName = 'StreamingMessage';