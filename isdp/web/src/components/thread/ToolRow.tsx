// isdp/web/src/components/thread/ToolRow.tsx
import React, { memo, useState } from 'react';
import { CheckOutlined, CloseOutlined, LoadingOutlined } from '@ant-design/icons';
import type { CliEvent } from '@/utils/toCliEvents';

/**
 * 工具行组件
 * 显示单个工具调用的状态、名称、参数、耗时
 * 参考 clowder-ai 的 ToolRow 设计
 */

/** 加深颜色（用于浅色背景） */
function darken(hex: string, ratio: number): string {
  const r = Number.parseInt(hex.slice(1, 3), 16);
  const g = Number.parseInt(hex.slice(3, 5), 16);
  const b = Number.parseInt(hex.slice(5, 7), 16);
  const dr = Math.round(r * (1 - ratio));
  const dg = Math.round(g * (1 - ratio));
  const db = Math.round(b * (1 - ratio));
  return `rgb(${dr}, ${dg}, ${db})`;
}

/** hex 转 rgba */
function hexToRgba(hex: string, opacity: number): string {
  const r = Number.parseInt(hex.slice(1, 3), 16);
  const g = Number.parseInt(hex.slice(3, 5), 16);
  const b = Number.parseInt(hex.slice(5, 7), 16);
  return `rgba(${r}, ${g}, ${b}, ${opacity})`;
}

/** 扳手图标 */
function WrenchIcon({ color }: { color?: string }) {
  return (
    <svg
      aria-hidden="true"
      width="11"
      height="11"
      viewBox="0 0 24 24"
      fill="none"
      stroke={color || '#6B7280'}
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{ flexShrink: 0 }}
    >
      <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
    </svg>
  );
}

interface ToolRowProps {
  event: CliEvent;
  isActive?: boolean;
  accentColor?: string;
  onUserInteract?: () => void;
}

function formatDuration(ms?: number): string {
  if (!ms) return '';
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rem = s % 60;
  return rem > 0 ? `${m}m${rem}s` : `${m}m`;
}

/** Chevron 图标 */
function ChevronIcon({ expanded }: { expanded: boolean }) {
  return (
    <svg
      aria-hidden="true"
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{
        flexShrink: 0,
        transition: 'transform 0.15s',
        transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)',
      }}
    >
      <polyline points="9 18 15 12 9 6" />
    </svg>
  );
}

export const ToolRow: React.FC<ToolRowProps> = memo(({
  event,
  isActive = false,
  accentColor = '#7C3AED',
  onUserInteract,
}) => {
  const [expanded, setExpanded] = useState(false);
  const hasDetail = event.detail != null;

  // 活跃状态的颜色变体（深色用于浅色背景）
  const accentDark = darken(accentColor, 0.3);

  const handleClick = () => {
    setExpanded(v => !v);
    onUserInteract?.();
  };

  // 解析 label：工具名称 + 参数
  const labelParts = event.label?.split(' ') || [];
  const toolName = labelParts[0] || 'Tool';
  const toolArgs = labelParts.slice(1).join(' ');

  return (
    <button
      type="button"
      data-testid={`tool-row-${event.id}`}
      className="w-full text-left cursor-pointer rounded font-mono text-[11px]"
      style={{
        padding: '5px 8px',
        borderRadius: 4,
        backgroundColor: isActive ? hexToRgba(accentColor, 0.2) : undefined,
        borderLeft: isActive ? `2px solid ${accentColor}` : undefined,
      }}
      onClick={handleClick}
    >
      <div className="flex items-center gap-2 min-w-0 flex-1">
        {/* 状态图标 */}
        {event.status === 'running' ? (
          <LoadingOutlined style={{ color: accentColor }} />
        ) : event.status === 'success' ? (
          <CheckOutlined style={{ color: '#10B981' }} />
        ) : event.status === 'failed' ? (
          <CloseOutlined style={{ color: '#EF4444' }} />
        ) : null}

        {/* 扳手图标 */}
        <WrenchIcon color={isActive ? accentDark : '#6B7280'} />

        {/* 工具名称 */}
        <span className="truncate" style={{ color: '#374151' }}>
          <span style={{ fontWeight: 500 }}>{toolName}</span>
          {toolArgs && (
            <span style={{ color: '#6B7280' }}> {toolArgs}</span>
          )}
        </span>

        {/* 耗时 */}
        {event.duration && (
          <span style={{ color: '#9CA3AF', marginLeft: 'auto', fontSize: 10 }}>
            {formatDuration(event.duration)}
          </span>
        )}

        {/* 展开指示器 */}
        {hasDetail && !expanded && (
          <ChevronIcon expanded={false} />
        )}
      </div>

      {/* 展开的详情 */}
      {expanded && hasDetail && event.detail && (
        <div
          className="w-full mt-1 whitespace-pre-wrap"
          style={{
            paddingLeft: 24,
            fontSize: 10,
            color: '#6B7280',
            wordBreak: 'break-all',
          }}
        >
          <code style={{ display: 'block', padding: 4, background: '#f5f5f5', borderRadius: 4 }}>
            {event.detail}
          </code>
        </div>
      )}
    </button>
  );
});

ToolRow.displayName = 'ToolRow';