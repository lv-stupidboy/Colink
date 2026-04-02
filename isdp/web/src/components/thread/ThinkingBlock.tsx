// isdp/web/src/components/thread/ThinkingBlock.tsx
import React, { useState, memo, useLayoutEffect, useRef } from 'react';

/**
 * 思考块组件
 * 可折叠面板，显示 Agent 的思考过程
 * 参考 clowder-ai 的 ThinkingContent 设计
 */

/** 混合颜色到暗色背景 */
function tintedDark(hex: string, ratio = 0.25, base = '#1A1625'): string {
  const parse = (h: string) => [
    Number.parseInt(h.slice(1, 3), 16),
    Number.parseInt(h.slice(3, 5), 16),
    Number.parseInt(h.slice(5, 7), 16),
  ];
  const [r1, g1, b1] = parse(hex);
  const [r2, g2, b2] = parse(base);
  return `rgb(${Math.round(r2 + (r1 - r2) * ratio)}, ${Math.round(g2 + (g1 - g2) * ratio)}, ${Math.round(b2 + (b1 - b2) * ratio)})`;
}

const DIVIDER = '#334155';

interface ThinkingBlockProps {
  content: string;
  label?: string;
  defaultExpanded?: boolean;
  className?: string;
  breedColor?: string;
  expandInExport?: boolean;
}

/**
 * 截断预览文本（60字符）
 */
function truncatePreview(text: string, maxLength: number = 60): string {
  if (text.length <= maxLength) return text;
  return text.slice(0, maxLength).trim() + '…';
}

/**
 * Brain 图标组件（Lucide SVG）
 */
export const BrainIcon: React.FC<{ className?: string; style?: React.CSSProperties }> = ({ className, style }) => (
  <svg
    className={className}
    style={style}
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    width="14"
    height="14"
  >
    <path d="M12 5a3 3 0 1 0-5.997.125 4 4 0 0 0-2.526 5.77 4 4 0 0 0 .556 6.588A4 4 0 1 0 12 18Z" />
    <path d="M12 5a3 3 0 1 1 5.997.125 4 4 0 0 1 2.526 5.77 4 4 0 0 1-.556 6.588A4 4 0 1 1 12 18Z" />
    <path d="M15 13a4.5 4.5 0 0 1-3-4 4.5 4.5 0 0 1-3 4" />
    <path d="M17.599 6.5a3 3 0 0 0 .399-1.375" />
    <path d="M6.003 5.125A3 3 0 0 0 6.401 6.5" />
    <path d="M3.477 10.896a4 4 0 0 1 .585-.396" />
    <path d="M19.938 10.5a4 4 0 0 1 .585.396" />
    <path d="M6 18a4 4 0 0 1-1.967-.516" />
    <path d="M19.967 17.484A4 4 0 0 1 18 18" />
  </svg>
);

/**
 * Chevron 图标（展开/折叠指示）
 */
const ChevronIcon: React.FC<{ expanded: boolean; color?: string }> = ({ expanded, color }) => (
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

/**
 * 思考块组件
 */
export const ThinkingBlock: React.FC<ThinkingBlockProps> = memo(({
  content,
  label = 'Thinking',
  defaultExpanded = false,
  className = '',
  breedColor = '#7C3AED',
  expandInExport = true,
}) => {
  // 导出模式检测
  const isExport = typeof window !== 'undefined' &&
    new URLSearchParams(window.location.search).get('export') === 'true';
  const shouldExpand = (isExport && expandInExport) || defaultExpanded;

  const [expanded, setExpanded] = useState(shouldExpand);
  const hasMounted = useRef(false);

  // 布局变化通知（用于滚动调整）
  useLayoutEffect(() => {
    if (!hasMounted.current) {
      hasMounted.current = true;
      return;
    }
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new Event('catcafe:chat-layout-changed'));
    }
  }, [expanded]);

  const preview = truncatePreview(content);

  // 品种色调的深色背景
  const surface = tintedDark(breedColor, 0.25);
  const surfaceInner = tintedDark(breedColor, 0.18);

  return (
    <div className={`thinking-block mt-2 mb-1 overflow-hidden ${className}`} style={{ backgroundColor: surface, borderRadius: 10 }}>
      <button
        type="button"
        onClick={() => setExpanded(v => !v)}
        className="w-full flex items-center gap-2 text-[11px] font-mono transition-colors"
        style={{ padding: '8px 12px', backgroundColor: surface, color: '#94A3B8' }}
      >
        <span style={{ color: breedColor }}>
          <ChevronIcon expanded={expanded} color={breedColor} />
        </span>
        <BrainIcon style={{ color: '#94A3B8' }} />
        <span className="font-medium">{label}</span>
        {!expanded && (
          <span className="truncate max-w-[240px]" style={{ color: '#64748B' }}>
            {preview}
          </span>
        )}
      </button>

      {expanded && (
        <div style={{ backgroundColor: surfaceInner }}>
          <div style={{ height: 1, backgroundColor: DIVIDER }} />
          <div
            style={{
              padding: '8px 12px 10px 12px',
              color: '#CBD5E1',
              fontSize: 12,
              lineHeight: 1.6,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}
            className="font-mono text-xs leading-relaxed cli-output-md"
          >
            {content}
          </div>
        </div>
      )}
    </div>
  );
});

ThinkingBlock.displayName = 'ThinkingBlock';

/**
 * 检查消息是否包含思考内容
 * 从 metadata 中提取 thinking 字段
 */
export function hasThinkingContent(message: { metadata?: Record<string, unknown> }): boolean {
  return !!message.metadata?.thinking;
}

/**
 * 获取思考内容
 */
export function getThinkingContent(message: { metadata?: Record<string, unknown> }): string | undefined {
  const thinking = message.metadata?.thinking;
  return typeof thinking === 'string' ? thinking : undefined;
}