// isdp/web/src/components/thread/CliOutputBlock.tsx
import React, { memo, useEffect, useRef, useState } from 'react';
import { ToolRow } from './ToolRow';
import type { CliEvent } from '@/utils/toCliEvents';
import { buildCliSummary } from '@/utils/toCliEvents';
import './CliOutputBlock.css';

/**
 * CLI 输出块组件
 * 可折叠的工具调用列表，streaming 结束后自动收起
 * 参考 clowder-ai 的 CliOutputBlock 设计
 */

/** 浅色背景下的色调（白色消息气泡） */
function tintedLight(hex: string, ratio = 0.08): string {
  const r = Number.parseInt(hex.slice(1, 3), 16);
  const g = Number.parseInt(hex.slice(3, 5), 16);
  const b = Number.parseInt(hex.slice(5, 7), 16);
  return `rgb(${Math.round(r * ratio)}, ${Math.round(g * ratio)}, ${Math.round(b * ratio)})`;
}

const DIVIDER = '#E5E7EB';

/** Chevron 图标 */
function ChevronIcon({ expanded, color }: { expanded: boolean; color?: string }) {
  return (
    <svg
      aria-hidden="true"
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke={color || '#6B7280'}
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="flex-shrink-0 transition-transform duration-150"
      style={{ transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)' }}
    >
      <polyline points="9 18 15 12 9 6" />
    </svg>
  );
}

/** 爪印图标 */
function PawIcon() {
  return (
    <svg
      aria-hidden="true"
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="#94A3B8"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="flex-shrink-0"
    >
      <circle cx="11" cy="4" r="2" />
      <circle cx="18" cy="8" r="2" />
      <circle cx="20" cy="16" r="2" />
      <path d="M9 10a5 5 0 0 1 5 5v3.5a3.5 3.5 0 0 1-6.84 1.045Q6.52 17.48 4.46 16.84A3.5 3.5 0 0 1 5.5 10Z" />
    </svg>
  );
}

interface CliOutputBlockProps {
  events: CliEvent[];
  status: 'streaming' | 'done' | 'failed' | 'idle';
  defaultExpanded?: boolean;
  breedColor?: string;
}

export const CliOutputBlock: React.FC<CliOutputBlockProps> = memo(({
  events,
  status,
  defaultExpanded = false,
  breedColor = '#7C3AED',
}) => {
  // streaming 或导出时强制展开
  const isExport = typeof window !== 'undefined' &&
    new URLSearchParams(window.location.search).get('export') === 'true';
  const forceExpanded = status === 'streaming' || isExport;

  const [expanded, setExpanded] = useState(forceExpanded || defaultExpanded);
  const userInteracted = useRef(false);
  const prevStatusRef = useRef(status);

  // 强制展开时更新状态
  if (forceExpanded && !expanded) {
    setExpanded(true);
  }

  // 自动收起逻辑：streaming 结束后，如果用户没有交互，则自动折叠
  useEffect(() => {
    if (prevStatusRef.current === 'streaming' && status !== 'streaming' && !userInteracted.current) {
      setExpanded(false);
    }
    prevStatusRef.current = status;
  }, [status]);

  const handleToggle = () => {
    userInteracted.current = true;
    setExpanded(!expanded);
  };

  if (events.length === 0) {
    return null;
  }

  // 构建摘要
  const summary = buildCliSummary(events, status as 'streaming' | 'done' | 'failed');

  // 分离工具调用和结果
  const toolUses = events.filter(e => e.kind === 'tool_use');
  const toolResults = events.filter(e => e.kind === 'tool_result');

  // 当前活跃的工具（streaming 时）
  const lastToolId = status === 'streaming'
    ? [...events].reverse().find(e => e.kind === 'tool_use')?.id
    : undefined;

  // 品种色调的浅色背景（用于白色消息气泡）
  const surface = tintedLight(breedColor, 0.06);
  const surfaceInner = tintedLight(breedColor, 0.03);

  return (
    <div className="cli-output-block mt-2 mb-1 overflow-hidden" style={{ backgroundColor: surface, borderRadius: 10 }}>
      {/* Header */}
      <button
        type="button"
        onClick={handleToggle}
        className="w-full flex items-center gap-2 text-[11px] font-mono transition-colors"
        style={{ padding: '8px 12px', color: '#374151', backgroundColor: surface }}
      >
        <span style={{ color: breedColor }}>
          <ChevronIcon expanded={expanded} color={breedColor} />
        </span>
        <span className="font-medium" style={{ color: '#374151' }}>{summary}</span>
        <span className="ml-auto flex items-center gap-1" style={{ color: '#6B7280', fontSize: 10 }}>
          <PawIcon />
          <span>{toolUses.length} tool{toolUses.length > 1 ? 's' : ''}</span>
        </span>
      </button>

      {/* Body */}
      {expanded && (
        <div data-testid="cli-output-body" style={{ backgroundColor: surfaceInner }}>
          <div style={{ height: 1, backgroundColor: DIVIDER }} />
          <div className="cli-output-tools space-y-0.5" style={{ padding: '4px 12px' }}>
            {toolUses.map((toolEvent, i) => {
              // 合并工具结果（如果有）
              const result = toolResults[i];
              const mergedEvent: CliEvent = {
                ...toolEvent,
                detail: result?.detail ?? toolEvent.detail,
                status: result?.status ?? toolEvent.status,
              };
              return (
                <ToolRow
                  key={toolEvent.id}
                  event={mergedEvent}
                  isActive={toolEvent.id === lastToolId}
                  accentColor={breedColor}
                  onUserInteract={() => { userInteracted.current = true; }}
                />
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
});

CliOutputBlock.displayName = 'CliOutputBlock';