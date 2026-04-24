import React, { useState, useCallback } from 'react';
import { RightOutlined, CopyOutlined, CheckOutlined } from '@ant-design/icons';
import { AgentStatusBadge, TimeDisplay } from './shared';
import { DurationDisplay } from './DurationDisplay';
import type { AgentInvocation } from '@/types';

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
 * 布局：状态 + 名称 + 调用时间 + 运行时长 + [展开按钮] [复制按钮]
 */
export const TimelineItem = React.memo(function TimelineItem({ inv, onViewDetail }: TimelineItemProps) {
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);

  const hasFullPrompt = inv.fullPrompt && inv.fullPrompt.length > 0;
  const usage = inv.inputTokens !== undefined || inv.outputTokens !== undefined ? inv : null;

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
    <div className="timeline-item" onClick={handleViewDetail}>
      {/* 主信息行：状态 + 名称 + 时间 + 时长 + 操作按钮 */}
      <div className="timeline-main-row">
        <AgentStatusBadge status={inv.status as any} />
        <span className="timeline-agent-name">{getDisplayName(inv)}</span>
        <TimeDisplay isoString={inv.startedAt} />
        <DurationDisplay startedAt={inv.startedAt} completedAt={inv.completedAt} compact />

        {/* 操作按钮区域 */}
        {hasFullPrompt && (
          <div className="timeline-actions-inline">
            <span
              className={`expand-btn ${expanded ? 'expanded' : ''}`}
              onClick={toggleExpand}
              title={expanded ? '收起提示词' : '展开提示词'}
            >
              <RightOutlined />
            </span>
            <span
              className={`copy-btn ${copied ? 'copied' : ''}`}
              onClick={handleCopy}
              title={copied ? '已复制' : '复制提示词'}
            >
              {copied ? <CheckOutlined /> : <CopyOutlined />}
            </span>
          </div>
        )}
      </div>

      {/* Token 使用（可选） */}
      {usage && (
        <div className="timeline-usage">
          <span>{formatTokens(usage.inputTokens || 0)}↓</span>
          <span>{formatTokens(usage.outputTokens || 0)}↑</span>
          {usage.costUsd !== undefined && usage.costUsd > 0 && (
            <span>${usage.costUsd.toFixed(4)}</span>
          )}
        </div>
      )}

      {/* 提示词区域（展开时显示） */}
      {hasFullPrompt && expanded && (
        <div className="timeline-prompt-expanded">
          <pre className="prompt-content">
            {inv.fullPrompt}
          </pre>
        </div>
      )}
    </div>
  );
});

export default TimelineItem;