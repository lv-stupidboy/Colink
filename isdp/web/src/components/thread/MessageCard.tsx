// isdp/web/src/components/thread/MessageCard.tsx
import React from 'react';
import { Message } from '@/types';
import './MessageCard.css';

interface MessageCardProps {
  message: Message;
  isStreaming?: boolean;
  agentName?: string;
  agentRole?: string;
}

export const MessageCard: React.FC<MessageCardProps> = ({
  message,
  isStreaming,
  agentName,
  agentRole,
}) => {
  const isUser = message.role === 'user';

  return (
    <div className={`message-card ${isUser ? 'message-user' : 'message-agent'}`}>
      {!isUser && (
        <div className="message-header">
          <span className="agent-name">{agentName || 'Agent'}</span>
          {agentRole && <span className="agent-role">{agentRole}</span>}
        </div>
      )}
      <div className="message-content">
        {message.content}
        {isStreaming && <span className="streaming-cursor">▊</span>}
      </div>
    </div>
  );
};