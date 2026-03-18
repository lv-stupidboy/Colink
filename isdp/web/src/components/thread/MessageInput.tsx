// isdp/web/src/components/thread/MessageInput.tsx
import React, { useState, useRef } from 'react';
import { AgentConfig } from '@/types';
import './MessageInput.css';

interface MessageInputProps {
  onSend: (message: string) => void;
  disabled?: boolean;
  placeholder?: string;
  agents?: AgentConfig[];
}

export const MessageInput: React.FC<MessageInputProps> = ({
  onSend,
  disabled,
  placeholder = '输入消息...',
  agents = [],
}) => {
  const [input, setInput] = useState('');
  const [showMentions, setShowMentions] = useState(false);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const handleSend = () => {
    if (input.trim() && !disabled) {
      onSend(input.trim());
      setInput('');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleMentionSelect = (agent: AgentConfig) => {
    setInput(prev => prev + `@${agent.name} `);
    setShowMentions(false);
    inputRef.current?.focus();
  };

  return (
    <div className="message-input-container">
      <div className="input-wrapper">
        <textarea
          ref={inputRef}
          value={input}
          onChange={(e) => {
            setInput(e.target.value);
            if (e.target.value.endsWith('@')) {
              setShowMentions(true);
            }
          }}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={disabled}
          rows={1}
        />
        {showMentions && agents.length > 0 && (
          <div className="mention-dropdown">
            {agents.map(agent => (
              <div
                key={agent.id}
                className="mention-item"
                onClick={() => handleMentionSelect(agent)}
              >
                <span className="mention-name">{agent.name}</span>
                <span className="mention-role">{agent.role}</span>
              </div>
            ))}
          </div>
        )}
      </div>
      <button onClick={handleSend} disabled={disabled || !input.trim()}>
        发送
      </button>
    </div>
  );
};