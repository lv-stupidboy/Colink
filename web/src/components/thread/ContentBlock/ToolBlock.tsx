import React, { useState, useEffect, memo } from 'react';
import type { ToolUseBlock as ToolUseBlockType } from '@/types';
import './ContentBlock.css';

/**
 * 工具调用行组件
 * 只显示单行工具信息：状态图标 + 工具名 + 参数 + 耗时
 */

/** 格式化执行时间 */
function formatDuration(ms?: number): string {
  if (!ms) return '';
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rem = s % 60;
  return rem > 0 ? `${m}m${rem}s` : `${m}m`;
}

/** 状态图标 */
function StatusIcon({ status, color }: { status: 'streaming' | 'success' | 'failed'; color?: string }) {
  if (status === 'streaming') {
    return (
      <svg
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke={color || '#1890ff'}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        style={{ animation: 'spin 1s linear infinite' }}
      >
        <path d="M21 12a9 9 0 1 1-6.219-8.56" />
      </svg>
    );
  }
  if (status === 'success') {
    return (
      <svg
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke="#52c41a"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <polyline points="20 6 9 17 4 12" />
      </svg>
    );
  }
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="#ff4d4f"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="10" />
      <line x1="15" y1="9" x2="9" y2="15" />
      <line x1="9" y1="9" x2="15" y2="15" />
    </svg>
  );
}

/** 扳手图标 */
function WrenchIcon({ color }: { color?: string }) {
  return (
    <svg
      width="11"
      height="11"
      viewBox="0 0 24 24"
      fill="none"
      stroke={color || '#8c8c8c'}
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
    </svg>
  );
}

/** Chevron 图标 */
function ChevronIcon({ expanded, color }: { expanded: boolean; color?: string }) {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke={color || '#8c8c8c'}
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{
        transition: 'transform 0.15s',
        transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)',
      }}
    >
      <polyline points="9 18 15 12 9 6" />
    </svg>
  );
}

/** 工具行属性 */
interface ToolCallRowProps {
  toolName: string;
  input?: Record<string, unknown>;
  output?: string;
  status: 'streaming' | 'success' | 'failed';
  duration?: number;
  startedAt?: number;
  defaultExpanded?: boolean;
  accentColor?: string;
}

/** 单行工具调用组件 */
export const ToolCallRow: React.FC<ToolCallRowProps> = memo(({
  toolName,
  input,
  output,
  status,
  duration,
  startedAt,
  defaultExpanded = false,
  accentColor = '#7C3AED',
}) => {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const [runningTime, setRunningTime] = useState(duration || 0);

  // 实时计算运行时间
  useEffect(() => {
    if (status !== 'streaming' || !startedAt) return;

    const updateTimer = () => {
      setRunningTime(Date.now() - startedAt);
    };

    updateTimer();
    const interval = setInterval(updateTimer, 100);
    return () => clearInterval(interval);
  }, [status, startedAt]);

  const displayDuration = status === 'streaming'
    ? formatDuration(runningTime)
    : formatDuration(duration);

  // 提取主要参数
  const primaryArgKeys = ['file_path', 'command', 'pattern', 'url', 'query', 'path', 'content'];
  let primaryArg = '';
  for (const key of primaryArgKeys) {
    const val = input?.[key];
    if (typeof val === 'string' && val.length > 0) {
      primaryArg = val.length > 60 ? `${val.slice(0, 60)}...` : val;
      break;
    }
  }

  const hasDetail = (input && Object.keys(input).length > 0) || output;

  // streaming 时自动展开
  useEffect(() => {
    if (status === 'streaming' && !expanded) {
      setExpanded(true);
    }
  }, [status]);

  return (
    <div className="tool-call-row-container">
      <button
        type="button"
        className={`tool-call-row ${status}`}
        onClick={() => hasDetail && setExpanded(v => !v)}
        style={{ cursor: hasDetail ? 'pointer' : 'default' }}
      >
        {/* 状态图标 */}
        <span className="tool-call-icon">
          <StatusIcon status={status} color={accentColor} />
        </span>

        {/* 扳手图标 */}
        <WrenchIcon color={status === 'streaming' ? accentColor : undefined} />

        {/* 工具名称 */}
        <span
          className="tool-call-name"
          style={{ color: status === 'streaming' ? accentColor : undefined }}
        >
          {toolName}
        </span>

        {/* 主要参数 */}
        {primaryArg && (
          <span className="tool-call-input">
            {primaryArg}
          </span>
        )}

        {/* 耗时 */}
        {displayDuration && (
          <span className="tool-call-duration">
            {status === 'streaming' && (
              <span className="tool-call-timer">{displayDuration}</span>
            )}
            {status !== 'streaming' && displayDuration}
          </span>
        )}

        {/* 展开指示器 */}
        {hasDetail && <ChevronIcon expanded={expanded} />}
      </button>

      {/* 展开的详情 */}
      {expanded && hasDetail && (
        <div className="tool-call-detail">
          {input && Object.keys(input).length > 0 && (
            <div className="tool-call-detail-section">
              <div className="tool-call-detail-label">Input:</div>
              <pre>{JSON.stringify(input, null, 2)}</pre>
            </div>
          )}
          {output && (
            <div className="tool-call-detail-section">
              <div className="tool-call-detail-label">Output:</div>
              <pre>{output.length > 2000 ? `${output.slice(0, 2000)}...` : output}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
});

ToolCallRow.displayName = 'ToolCallRow';

/** 工具块属性 */
interface ToolBlockProps {
  block: ToolUseBlockType;
  defaultExpanded?: boolean;
}

/**
 * 单个工具块组件（包装器）
 * 用于单个工具调用场景，提供外壳
 */
const ToolBlock: React.FC<ToolBlockProps> = memo(({ block, defaultExpanded = false }) => {
  return (
    <div className="tool-block-single">
      <ToolCallRow
        toolName={block.toolName}
        input={block.input}
        output={block.output}
        status={block.status}
        duration={block.duration}
        startedAt={block.startedAt}
        defaultExpanded={defaultExpanded}
      />
    </div>
  );
});

ToolBlock.displayName = 'ToolBlock';

export default ToolBlock;