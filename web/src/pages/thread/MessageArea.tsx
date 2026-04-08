// isdp/web/src/pages/thread/MessageArea.tsx
import React, { memo, useRef, useEffect } from 'react';
import { Typography } from 'antd';
import { RobotOutlined } from '@ant-design/icons';
import { ChatMessageList } from '@/components/thread/ChatMessageList';
import type { Message, AgentConfig, ToolEvent } from '@/types';
import type { FileChange } from '@/types/content';

const { Title, Text } = Typography;

interface MessageAreaProps {
  messages: Message[];
  toolEvents: Record<string, ToolEvent[]>;
  agentConfigs: AgentConfig[];
  projectPath: string;
  onStopAgent: (invocationId: string) => void;
  onRetryAgent: (message: Message) => void;
  onOpenCodePanel: (files: FileChange[]) => void;
  autoScroll?: boolean;
}

/**
 * 消息区域组件
 * 流式消息由 StreamingMessage 组件独立处理
 */
export const MessageArea: React.FC<MessageAreaProps> = memo(({
  messages,
  toolEvents,
  agentConfigs,
  projectPath,
  onStopAgent,
  onRetryAgent,
  onOpenCodePanel,
  autoScroll = true,
}) => {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // 自动滚动到底部（只在消息数量变化时触发）
  useEffect(() => {
    if (autoScroll && messagesEndRef.current) {
      messagesEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [messages.length, autoScroll]);

  const hasMessages = messages.length > 0;

  if (!hasMessages) {
    return (
      <div style={{
        textAlign: 'center',
        padding: '60px 20px',
        color: '#999',
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
      }}>
        <RobotOutlined style={{ fontSize: 48, marginBottom: 16 }} />
        <Title level={4} type="secondary">开始您的开发任务</Title>
        <Text type="secondary">
          在下方输入您的需求，或使用 @需求分析师、@架构师、@开发者 等 Agent 协助开发
        </Text>
      </div>
    );
  }

  return (
    <div className="thread-messages" style={{ height: '100%', overflowY: 'auto' }}>
      <ChatMessageList
        messages={messages}
        agentConfigs={agentConfigs}
        projectPath={projectPath}
        toolEvents={toolEvents}
        onStopAgent={onStopAgent}
        onRetryAgent={onRetryAgent}
        onOpenCodePanel={onOpenCodePanel}
        autoScroll={autoScroll}
      />
      <div ref={messagesEndRef} />
    </div>
  );
});

MessageArea.displayName = 'MessageArea';