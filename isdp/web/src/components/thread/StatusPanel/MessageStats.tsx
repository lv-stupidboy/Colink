import React from 'react';
import { MessageOutlined } from '@ant-design/icons';

interface Props {
  stats: {
    total: number;
    agent: number;
    system: number;
    user: number;
  };
}

export const MessageStats: React.FC<Props> = ({ stats }) => {
  return (
    <div className="status-section">
      <div className="status-section-title">
        <MessageOutlined />
        消息统计
      </div>
      <div className="message-grid">
        <div className="message-item">
          <span className="message-count">{stats.total}</span>
          <span className="message-label">总数</span>
        </div>
        <div className="message-item">
          <span className="message-count">{stats.user}</span>
          <span className="message-label">用户</span>
        </div>
        <div className="message-item">
          <span className="message-count">{stats.agent}</span>
          <span className="message-label">Agent</span>
        </div>
        <div className="message-item">
          <span className="message-count">{stats.system}</span>
          <span className="message-label">系统</span>
        </div>
      </div>
    </div>
  );
};

export default MessageStats;