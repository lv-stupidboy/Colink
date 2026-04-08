import React, { useState, useLayoutEffect, useRef, useEffect } from 'react';
import type { ThinkingBlock as ThinkingBlockType } from '@/types';
import './ContentBlock.css';

/**
 * 思考过程块组件
 * 参考 Clowder AI 设计：
 * - 60字符预览 + 脑图标
 * - streaming 时自动展开，完成后自动折叠（尊重用户交互）
 */

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

/** 脑图标 - Lucide brain */
function BrainIcon() {
  return (
    <svg
      aria-hidden="true"
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="#6B7280"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{ flexShrink: 0 }}
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
}

interface ThinkingBlockProps {
  block: ThinkingBlockType;
  defaultExpanded?: boolean;
  breedColor?: string;
}

const ThinkingBlock: React.FC<ThinkingBlockProps> = ({
  block,
  defaultExpanded = false,
  breedColor = '#7C3AED',
}) => {
  // streaming 时自动展开
  const isStreaming = block.status === 'streaming';
  const forceExpanded = isStreaming;

  const [expanded, setExpanded] = useState(forceExpanded || defaultExpanded);
  const userInteracted = useRef(false);
  const hasMounted = useRef(false);

  // streaming 时自动展开
  useEffect(() => {
    if (forceExpanded && !expanded) {
      setExpanded(true);
    }
  }, [forceExpanded, expanded]);

  // 完成后自动折叠（但尊重用户交互）
  const prevStatusRef = useRef(block.status);
  useEffect(() => {
    if (prevStatusRef.current === 'streaming' && block.status !== 'streaming') {
      if (!userInteracted.current) {
        setExpanded(false);
      }
    }
    prevStatusRef.current = block.status;
  }, [block.status]);

  // 布局变化通知
  useLayoutEffect(() => {
    if (!hasMounted.current) {
      hasMounted.current = true;
      return;
    }
    window.dispatchEvent(new Event('isdp:chat-layout-changed'));
  }, [expanded]);

  const handleToggle = () => {
    userInteracted.current = true;
    setExpanded(v => !v);
  };

  // 预览：前60字符
  const previewLength = 60;
  const content = block.content || '';
  const preview = content.length > previewLength
    ? `${content.slice(0, previewLength)}…`
    : content;

  // 耗时
  const durationStr = block.duration
    ? `${(block.duration / 1000).toFixed(1)}s`
    : '';

  return (
    <div className="thinking-block-wrapper">
      {/* Header - 可点击展开/收起 */}
      <button
        type="button"
        onClick={handleToggle}
        className="thinking-block-header"
      >
        <ChevronIcon expanded={expanded} color={breedColor} />
        <BrainIcon />
        <span className="thinking-block-label">Thinking</span>
        {durationStr && (
          <span className="thinking-block-duration">{durationStr}</span>
        )}
        {/* 折叠时显示预览 */}
        {!expanded && (
          <span className="thinking-block-preview">{preview}</span>
        )}
      </button>

      {/* Body - 展开后显示 */}
      {expanded && (
        <div className="thinking-block-body">
          <div className="thinking-block-divider" />
          <div className="thinking-block-content">
            {content}
          </div>
        </div>
      )}
    </div>
  );
};

export default React.memo(ThinkingBlock);