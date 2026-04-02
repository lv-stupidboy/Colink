import React, { useState } from 'react';
import { useAppStore } from '@/store';
import { AgentStatusCard } from './AgentStatusCard';
import { TokenUsage } from './TokenUsage';
import { MessageStats } from './MessageStats';
import { AgentHistoryCard } from './AgentHistoryCard';
import { TaskProgressPanel } from './TaskProgressPanel';
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
      {/* Thread ID */}
      <div className="thread-id-section">
        <span className="thread-id-label">Thread ID</span>
        <span className="thread-id-value">{displayThreadId}</span>
        {threadId && (
          <span className="thread-id-copy" onClick={copyThreadId}>
            {copied ? <CheckOutlined /> : <CopyOutlined />}
          </span>
        )}
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

      {/* 消息统计 */}
      <MessageStats stats={messageStats} />

      {/* Token 统计 - 始终显示 */}
      <TokenUsage usage={agentUsage} totalUsage={totalUsage} />

      {/* 任务进度 - 仅在有任务时显示 */}
      {Object.keys(agentTaskProgress).length > 0 && (
        <TaskProgressPanel progress={agentTaskProgress} />
      )}
    </aside>
  );
};

export default StatusPanel;