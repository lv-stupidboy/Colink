import React, { useState, useCallback, useMemo } from 'react';
import { ExpandOutlined, CompressOutlined, CopyOutlined, CheckOutlined, SwapOutlined } from '@ant-design/icons';
import { AgentStatusBadge, TimeDisplay } from './shared';
import { DurationDisplay } from './DurationDisplay';
import type { AgentInvocation } from '@/types';

/**
 * 解析 a2a-handoff 交接块
 */
const parseA2AHandoff = (content: string) => {
  const handoffMatch = content.match(/<a2a-handoff>([\s\S]*?)<\/a2a-handoff>/);
  if (!handoffMatch) return null;

  const handoffContent = handoffMatch[1];

  const extractPart = (header: string): string => {
    const idx = handoffContent.indexOf(header);
    if (idx === -1) return '';
    const start = idx + header.length;
    const nextPart = handoffContent.slice(start).indexOf('### ');
    if (nextPart !== -1) {
      return handoffContent.slice(start, start + nextPart).trim();
    }
    return handoffContent.slice(start).trim();
  };

  return {
    hasHandoff: true,
    what: extractPart('### What'),
    why: extractPart('### Why'),
    tradeoff: extractPart('### Tradeoff'),
    openQuestions: extractPart('### Open Questions'),
    nextAction: extractPart('### Next Action'),
  };
};

/**
 * 格式化 Token 数量
 */
const formatTokens = (n: number): string => {
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return String(n);
};

/**
 * 获取显示名称
 */
const getDisplayName = (inv: AgentInvocation): string => {
  if (inv.agentName && inv.agentName.trim()) return inv.agentName.trim();
  if (inv.role === 'agent') return 'Agent';
  if (inv.role) return inv.role;
  return inv.id.slice(0, 8);
};

interface TimelineItemProps {
  inv: AgentInvocation;
  onViewDetail: (inv: AgentInvocation) => void;
}

/**
 * 单条时间线记录组件（使用 React.memo 优化）
 */
export const TimelineItem = React.memo(function TimelineItem({ inv, onViewDetail }: TimelineItemProps) {
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);

  const hasFullPrompt = inv.fullPrompt && inv.fullPrompt.length > 0;
  const usage = inv.inputTokens !== undefined || inv.outputTokens !== undefined ? inv : null;

  // 使用 useMemo 缓存解析结果
  const handoffInfo = useMemo(() =>
    inv.output ? parseA2AHandoff(inv.output) : null,
    [inv.output]
  );

  // 复制处理
  const handleCopy = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await navigator.clipboard.writeText(inv.fullPrompt || '');
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('复制失败:', err);
    }
  }, [inv.fullPrompt]);

  // 展开/收起处理
  const toggleExpand = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setExpanded(prev => !prev);
  }, []);

  // 查看详情处理
  const handleViewDetail = useCallback(() => {
    onViewDetail(inv);
  }, [inv, onViewDetail]);

  return (
    <div className="timeline-item">
      {/* 状态行 */}
      <div className="timeline-status-row">
        <AgentStatusBadge status={inv.status as any} />
        <span className="timeline-agent-name">{getDisplayName(inv)}</span>
        <TimeDisplay isoString={inv.startedAt} />
        <DurationDisplay startedAt={inv.startedAt} completedAt={inv.completedAt} compact />
      </div>

      {/* Token 使用 */}
      {usage && (
        <div className="timeline-usage">
          <span>{formatTokens(usage.inputTokens || 0)}↓</span>
          <span>{formatTokens(usage.outputTokens || 0)}↑</span>
          {usage.costUsd !== undefined && usage.costUsd > 0 && (
            <span>${usage.costUsd.toFixed(4)}</span>
          )}
        </div>
      )}

      {/* A2A Handoff */}
      {handoffInfo && (
        <div className="handoff-card mini">
          <div className="handoff-card-header">
            <SwapOutlined style={{ marginRight: 6 }} />
            <span>A2A 交接</span>
          </div>
          <div className="handoff-card-content compact">
            {handoffInfo.what && (
              <div className="handoff-part">
                <span className="handoff-label">What:</span>
                <span className="handoff-value">{handoffInfo.what}</span>
              </div>
            )}
            {handoffInfo.nextAction && (
              <div className="handoff-part">
                <span className="handoff-label">Next:</span>
                <span className="handoff-value">{handoffInfo.nextAction}</span>
              </div>
            )}
          </div>
        </div>
      )}

      {/* 提示词区域 */}
      {hasFullPrompt && (
        <div className="timeline-prompt">
          <div className="timeline-prompt-header">
            <span className="timeline-prompt-label">提示词</span>
            <div className="timeline-prompt-actions">
              <span
                className="prompt-action"
                onClick={toggleExpand}
                title={expanded ? '收起' : '展开'}
              >
                {expanded ? <CompressOutlined /> : <ExpandOutlined />}
              </span>
              <span
                className={`prompt-action ${copied ? 'copied' : ''}`}
                onClick={handleCopy}
                title={copied ? '已复制' : '复制'}
              >
                {copied ? <CheckOutlined /> : <CopyOutlined />}
              </span>
            </div>
          </div>
          <pre className={expanded ? 'expanded' : 'collapsed'}>
            {expanded ? inv.fullPrompt : inv.fullPrompt?.slice(0, 200) + (inv.fullPrompt && inv.fullPrompt.length > 200 ? '...' : '')}
          </pre>
        </div>
      )}

      {/* 操作按钮 */}
      <div className="timeline-actions">
        <span className="detail-btn" onClick={handleViewDetail}>
          查看详情
        </span>
      </div>
    </div>
  );
});

export default TimelineItem;