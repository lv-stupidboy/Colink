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

// 解析带纳秒的 ISO 时间格式（如 2026-04-13T15:44:34.3872777+08:00）
// JavaScript Date 只支持毫秒精度，需要截断纳秒
const parseISOTime = (isoString?: string): Date | null => {
  if (!isoString) return null;

  try {
    // 处理带纳秒的格式：截断为毫秒精度
    // 格式: 2026-04-13T15:44:34.3872777+08:00 或 2026-04-13T15:44:34.387Z
    let normalized = isoString;

    // 如果有小数点（纳秒），截断为毫秒（3位）
    const match = isoString.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})\.(\d+)(.*)$/);
    if (match) {
      const [, base, fractional, suffix] = match;
      // 截断为毫秒精度（取前3位），不足则补零
      const ms = (fractional.slice(0, 3) || '000').padEnd(3, '0');
      normalized = `${base}.${ms}${suffix}`;
    }

    const date = new Date(normalized);
    if (isNaN(date.getTime())) {
      console.warn('Failed to parse time:', isoString);
      return null;
    }
    return date;
  } catch (e) {
    console.warn('Error parsing time:', isoString, e);
    return null;
  }
};

// 格式化时间显示（只显示时分秒）
const formatStartTime = (isoString?: string): string => {
  const date = parseISOTime(isoString);
  if (!date) return '—';
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
};

// 获取开始时间（优先使用 startedAt，如果没有则使用 createdAt）
const getStartTime = (agent: AgentInvocation): string | undefined => {
  return agent.startedAt || agent.createdAt;
};

// 获取显示名称（带完整 fallback）
const getDisplayName = (agent: AgentInvocation): string => {
  // 优先使用 agentName
  if (agent.agentName && agent.agentName.trim()) {
    return agent.agentName.trim();
  }
  // 如果 role 是 custom，显示 'Agent'
  if (agent.role === 'custom') {
    return 'Agent';
  }
  // 否则显示 role（如 developer, architect 等）
  if (agent.role) {
    return agent.role;
  }
  // 最后使用 ID 前8位
  return agent.id.slice(0, 8);
};

export const AgentHistoryCard: React.FC<Props> = ({ completedAgents, agentUsage }) => {
  const [expanded, setExpanded] = useState(true);

  // 按完成时间倒序排列（最近的在上面）
  const sortedAgents = [...completedAgents].sort((a, b) => {
    const dateA = parseISOTime(a.completedAt);
    const dateB = parseISOTime(b.completedAt);
    const timeA = dateA ? dateA.getTime() : 0;
    const timeB = dateB ? dateB.getTime() : 0;
    return timeB - timeA;
  });

  // 实际显示的总数
  const displayCount = sortedAgents.length;

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
        <span className="section-collapse-count">{displayCount}</span>
      </div>

      {expanded && (
        <div className="history-list" style={{ marginTop: 8, maxHeight: 200, overflowY: 'auto' }}>
          {sortedAgents.length === 0 ? (
            <div className="idle-status">暂无历史调用</div>
          ) : (
            <>
              {sortedAgents.filter(a => a.status === 'completed').map(agent => (
                <div key={agent.id} className="history-item completed">
                  <div className="history-header">
                    <CheckCircleOutlined style={{ color: '#22c55e', fontSize: 14 }} />
                    <span className="history-name">{getDisplayName(agent)}</span>
                    <DurationDisplay
                      startedAt={getStartTime(agent)}
                      completedAt={agent.completedAt}
                      compact
                    />
                  </div>
                  <div className="history-time">
                    <span>开始: {formatStartTime(getStartTime(agent))}</span>
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
              {sortedAgents.filter(a => a.status === 'failed').map(agent => (
                <div key={agent.id} className="history-item failed">
                  <div className="history-header">
                    <CloseCircleOutlined style={{ color: '#ef4444', fontSize: 14 }} />
                    <span className="history-name">{getDisplayName(agent)}</span>
                    <DurationDisplay
                      startedAt={getStartTime(agent)}
                      completedAt={agent.completedAt}
                      compact
                    />
                  </div>
                  <div className="history-time">
                    <span>开始: {formatStartTime(getStartTime(agent))}</span>
                  </div>
                </div>
              ))}
              {sortedAgents.filter(a => a.status === 'interrupted' || a.status === 'cancelled').map(agent => (
                <div key={agent.id} className="history-item other-ended">
                  <div className="history-header">
                    <span className={`agent-status-badge ${agent.status}`}>{agent.status === 'interrupted' ? '中断' : '取消'}</span>
                    <span className="history-name">{getDisplayName(agent)}</span>
                    <DurationDisplay
                      startedAt={getStartTime(agent)}
                      completedAt={agent.completedAt}
                      compact
                    />
                  </div>
                  <div className="history-time">
                    <span>开始: {formatStartTime(getStartTime(agent))}</span>
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