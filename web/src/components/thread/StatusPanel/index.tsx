import React, { useState } from 'react';
import { useAppStore } from '@/store';
import { AgentStatusCard } from './AgentStatusCard';
import { TokenUsage } from './TokenUsage';
import { AgentHistoryCard } from './AgentHistoryCard';
import { TaskProgressPanel } from './TaskProgressPanel';
import { AgentInvocationLogPanel } from './AgentInvocationLogPanel';
import { CopyOutlined, CheckOutlined } from '@ant-design/icons';
import './StatusPanel.css';

interface StatusPanelProps {
  width?: number;
  threadId?: string;
}

export const StatusPanel: React.FC<StatusPanelProps> = ({ width = 320, threadId }) => {
  const { activeAgents, agentUsage, messages, completedAgents, agentTaskProgress } = useAppStore();
  const [copied, setCopied] = useState(false);

  // 计算消息统计
  const messageStats = {
    total: messages.length,
    agent: messages.filter(m => m.role === 'agent').length,
    system: messages.filter(m => m.role === 'system').length,
    user: messages.filter(m => m.role === 'user').length,
  };

  // 计算 Token 总计
  const totalUsage = Object.values(agentUsage).reduce(
    (acc, u) => ({
      input: acc.input + (u.inputTokens || 0),
      output: acc.output + (u.outputTokens || 0),
      cache: acc.cache + (u.cacheReadTokens || 0),
      cost: acc.cost + (u.costUsd || 0),
    }),
    { input: 0, output: 0, cache: 0, cost: 0 }
  );

  const copyThreadId = () => {
    if (threadId) {
      navigator.clipboard.writeText(threadId);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const displayThreadId = threadId ? threadId.slice(0, 8) : '—';

  return (
    <aside className="status-panel" style={{ width }}>
      {/* Thread ID + 消息统计 合并区块 */}
      <div className="status-section thread-info-section">
        <div className="thread-id-row">
          <span className="thread-id-label">Thread ID</span>
          <span className="thread-id-value">{displayThreadId}</span>
          {threadId && (
            <span className="thread-id-copy" onClick={copyThreadId}>
              {copied ? <CheckOutlined /> : <CopyOutlined />}
            </span>
          )}
        </div>
        <div className="message-grid compact">
          <div className="message-item">
            <span className="message-count">{messageStats.total}</span>
            <span className="message-label">消息</span>
          </div>
          <div className="message-item">
            <span className="message-count">{messageStats.user}</span>
            <span className="message-label">用户</span>
          </div>
          <div className="message-item">
            <span className="message-count">{messageStats.agent}</span>
            <span className="message-label">Agent</span>
          </div>
          <div className="message-item">
            <span className="message-count">{messageStats.system}</span>
            <span className="message-label">系统</span>
          </div>
        </div>
      </div>

      {/* Agent 状态 */}
      <AgentStatusCard
        activeAgents={activeAgents}
        agentUsage={agentUsage}
      />

      {/* 历史参与 */}
      <AgentHistoryCard
        completedAgents={completedAgents}
        agentUsage={agentUsage}
      />

      {/* Agent 调用日志 */}
      <AgentInvocationLogPanel />

      {/* 任务进度 - 仅在有任务时显示 */}
      {Object.keys(agentTaskProgress).length > 0 && (
        <TaskProgressPanel progress={agentTaskProgress} />
      )}

      {/* Token 统计 - 默认收起 */}
      <TokenUsage usage={agentUsage} totalUsage={totalUsage} defaultCollapsed />
    </aside>
  );
};

export default StatusPanel;