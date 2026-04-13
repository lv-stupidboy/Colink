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

// 格式化时间显示（只显示时分秒）
const formatStartTime = (isoString?: string): string => {
  if (!isoString) return '—';
  const date = new Date(isoString);
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
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
        <div className="history-list" style={{ marginTop: 8, maxHeight: 200, overflowY: 'auto' }}>
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
                  <div className="history-time">
                    <span>开始: {formatStartTime(agent.startedAt)}</span>
                  </div>
                  {(() => {
                    // 优先使用 invocation 自带的 usage 数据，其次使用 agentUsage
                    const usage = agent.inputTokens !== undefined || agent.outputTokens !== undefined
                      ? agent
                      : agentUsage[agent.id];
                    if (!usage) return null;
                    return (
                      <div className="history-usage">
                        <span>{formatTokens(usage.inputTokens || 0)}↓</span>
                        <span>{formatTokens(usage.outputTokens || 0)}↑</span>
                        {usage.costUsd !== undefined && usage.costUsd > 0 && (
                          <span>${usage.costUsd.toFixed(4)}</span>
                        )}
                      </div>
                    );
                  })()}
                </div>
              ))}
              {failed.map(agent => (
                <div key={agent.id} className="history-item failed">
                  <div className="history-header">
                    <CloseCircleOutlined style={{ color: '#ef4444', fontSize: 14 }} />
                    <span className="history-name">{agent.agentName || agent.role || agent.id.slice(0, 8)}</span>
                    <DurationDisplay
                      startedAt={agent.startedAt}
                      completedAt={agent.completedAt}
                      compact
                    />
                  </div>
                  <div className="history-time">
                    <span>开始: {formatStartTime(agent.startedAt)}</span>
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