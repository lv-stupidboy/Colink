import React, { useState } from 'react';
import { CheckCircleOutlined, CloseCircleOutlined, RightOutlined } from '@ant-design/icons';
import type { AgentInvocation } from '@/types';
import type { TokenUsage } from '@/types/status';
import { DurationDisplay } from './DurationDisplay';

interface Props {
  completedAgents: AgentInvocation[];
  agentUsage: Record<string, TokenUsage>;
}

const formatTokens = (n: number): string => {
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return String(n);
};

export const AgentHistoryCard: React.FC<Props> = ({ completedAgents, agentUsage }) => {
  const [expanded, setExpanded] = useState(true);

  // 按时间倒序排列（最近的在上面）
  const sortedAgents = [...completedAgents].sort((a, b) => {
    const timeA = a.completedAt ? new Date(a.completedAt).getTime() : 0;
    const timeB = b.completedAt ? new Date(b.completedAt).getTime() : 0;
    return timeB - timeA;
  });

  const completed = sortedAgents.filter(a => a.status === 'completed');
  const failed = sortedAgents.filter(a => a.status === 'failed');

  return (
    <div className="status-section">
      <div
        className="section-collapse-header"
        onClick={() => setExpanded(!expanded)}
      >
        <span className={`section-collapse-icon ${expanded ? 'expanded' : ''}`}>
          <RightOutlined />
        </span>
        <span>历史参与</span>
        <span className="section-collapse-count">{completedAgents.length}</span>
      </div>

      {expanded && (
        <div className="history-list" style={{ marginTop: 8 }}>
          {completedAgents.length === 0 ? (
            <div className="idle-status">暂无历史调用</div>
          ) : (
            <>
              {completed.map(agent => (
                <div key={agent.id} className="history-item completed">
                  <div className="history-header">
                    <CheckCircleOutlined style={{ color: '#22c55e', fontSize: 14 }} />
                    <span className="history-name">{agent.agentName || agent.role || agent.id.slice(0, 8)}</span>
                    <DurationDisplay
                      startedAt={agent.startedAt}
                      completedAt={agent.completedAt}
                      compact
                    />
                  </div>
                  {agentUsage[agent.id] && (
                    <div className="history-usage">
                      <span>{formatTokens(agentUsage[agent.id].inputTokens || 0)}↓</span>
                      <span>{formatTokens(agentUsage[agent.id].outputTokens || 0)}↑</span>
                      {agentUsage[agent.id].costUsd !== undefined && agentUsage[agent.id].costUsd! > 0 && (
                        <span>${agentUsage[agent.id].costUsd!.toFixed(4)}</span>
                      )}
                    </div>
                  )}
                </div>
              ))}
              {failed.map(agent => (
                <div key={agent.id} className="history-item failed">
                  <div className="history-header">
                    <CloseCircleOutlined style={{ color: '#ef4444', fontSize: 14 }} />
                    <span className="history-name">{agent.agentName || agent.role || agent.id.slice(0, 8)}</span>
                    <span className="agent-status-badge failed">失败</span>
                  </div>
                </div>
              ))}
            </>
          )}
        </div>
      )}
    </div>
  );
};

export default AgentHistoryCard;