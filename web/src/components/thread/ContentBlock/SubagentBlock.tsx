import React, { useState, useEffect, useRef, memo, useMemo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { ToolUseBlock } from '@/types';
import { highlightMentions } from '@/utils/mentionHighlight';
import './ContentBlock.css';

interface SubagentBlockProps {
  block: ToolUseBlock;
  defaultExpanded?: boolean;
}

/** 格式化执行时间 */
function formatDuration(ms?: number): string {
  if (!ms) return '';
  const totalSec = Math.round(ms / 1000);
  if (totalSec < 60) return `${totalSec}s`;
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  return `${m}m${s}s`;
}

/** 机器人图标 */
function RobotIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="7" width="18" height="13" rx="2" />
      <path d="M8 7V5a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
      <line x1="12" y1="12" x2="12" y2="12.01" />
      <line x1="9" y1="12" x2="9" y2="12.01" />
      <line x1="15" y1="12" x2="15" y2="12.01" />
      <path d="M8 16h8" />
    </svg>
  );
}

/** Chevron 图标 */
function ChevronIcon({ expanded }: { expanded: boolean }) {
  return (
    <svg
      width="14" height="14" viewBox="0 0 24 24" fill="none"
      stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
      style={{ transition: 'transform 0.2s', transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)' }}
    >
      <polyline points="9 18 15 12 9 6" />
    </svg>
  );
}

const SubagentBlock: React.FC<SubagentBlockProps> = memo(({ block, defaultExpanded = false }) => {
  const subagentName = (block.input?.subagentName as string) || (block.input?.subagentType as string) || 'agent';
  const description = (block.input?.description as string) || '';
  const prompt = (block.input?.prompt as string) || '';
  const output = block.output || '';
  const isStreaming = block.status === 'streaming';
  const isSuccess = block.status === 'success';
  const isFailed = block.status === 'failed';

  const [userToggled, setUserToggled] = useState(false);
  const [bodyOpen, setBodyOpen] = useState(defaultExpanded);
  const [promptOpen, setPromptOpen] = useState(false);
  const [outputExpanded, setOutputExpanded] = useState(false);
  const [runningTime, setRunningTime] = useState(block.duration || 0);

  // 实时计时器
  useEffect(() => {
    if (!isStreaming || !block.startedAt) return;
    const tick = () => setRunningTime(Date.now() - block.startedAt!);
    tick();
    const interval = setInterval(tick, 100);
    return () => clearInterval(interval);
  }, [isStreaming, block.startedAt]);

  // 完成后自动展开
  const prevStatusRef = useRef(block.status);
  useEffect(() => {
    if (prevStatusRef.current === 'streaming' && isSuccess && !userToggled) {
      setBodyOpen(true);
    }
    prevStatusRef.current = block.status;
  }, [block.status, isSuccess, userToggled]);

  const handleToggleBody = () => {
    setUserToggled(true);
    setBodyOpen(v => !v);
  };

  const displayDuration = isStreaming ? formatDuration(runningTime) : formatDuration(block.duration);

  // Markdown 自定义组件（处理 @mention）
  const markdownComponents = useMemo(() => ({
    p: ({ children }: { children?: React.ReactNode }) => {
      if (typeof children === 'string' && /@[^\s@]+/.test(children)) {
        return <p>{highlightMentions(children, [])}</p>;
      }
      if (Array.isArray(children)) {
        const processed = children.map((child, idx) => {
          if (typeof child === 'string' && /@[^\s@]+/.test(child)) {
            return <React.Fragment key={idx}>{highlightMentions(child, [])}</React.Fragment>;
          }
          return child;
        });
        return <p>{processed}</p>;
      }
      return <p>{children}</p>;
    },
    li: ({ children }: { children?: React.ReactNode }) => {
      if (typeof children === 'string' && /@[^\s@]+/.test(children)) {
        return <li>{highlightMentions(children, [])}</li>;
      }
      if (Array.isArray(children)) {
        const processed = children.map((child, idx) => {
          if (typeof child === 'string' && /@[^\s@]+/.test(child)) {
            return <React.Fragment key={idx}>{highlightMentions(child, [])}</React.Fragment>;
          }
          return child;
        });
        return <li>{processed}</li>;
      }
      return <li>{children}</li>;
    },
  }), []);

  const statusClass = isStreaming ? 'running' : isFailed ? 'failed' : 'success';
  const hasOutput = output.length > 0;
  const hasPrompt = prompt.length > 0;

  return (
    <div className={`subagent-block ${statusClass}`}>
      {/* 头部 */}
      <button type="button" className="subagent-block-header" onClick={handleToggleBody}>
        <span className="subagent-block-accent" />
        <span className="subagent-block-icon"><RobotIcon /></span>
        <span className="subagent-block-label">Subagent</span>
        <span className="subagent-block-name">@{subagentName}</span>
        {description && (
          <span className="subagent-block-desc" title={description}>
            {description.length > 40 ? `${description.slice(0, 40)}…` : description}
          </span>
        )}
        <span className={`subagent-block-status ${statusClass}`}>
          {isStreaming ? 'running' : isFailed ? 'failed' : 'done'}
        </span>
        {displayDuration && (
          <span className={`subagent-block-duration ${isStreaming ? 'live' : ''}`}>
            {displayDuration}
          </span>
        )}
        <span className="subagent-block-chevron">
          <ChevronIcon expanded={bodyOpen} />
        </span>
      </button>

      {/* 内容区 */}
      {bodyOpen && (
        <div className="subagent-block-body">
          {/* Prompt 区 */}
          {hasPrompt && (
            <div className="subagent-block-section">
              <button
                type="button"
                className="subagent-block-section-toggle"
                onClick={() => setPromptOpen(v => !v)}
              >
                <ChevronIcon expanded={promptOpen} />
                <span className="subagent-block-section-label">PROMPT</span>
              </button>
              {promptOpen && (
                <pre className="subagent-block-prompt">{prompt}</pre>
              )}
            </div>
          )}

          {/* Output 区 */}
          <div className="subagent-block-section">
            <div className="subagent-block-section-label">OUTPUT</div>
            {isStreaming && !hasOutput && (
              <div className="subagent-block-waiting">等待 subagent 返回结果...</div>
            )}
            {isFailed && hasOutput && (
              <pre className="subagent-block-error-output">{output}</pre>
            )}
            {isSuccess && hasOutput && (
              <div className={`subagent-block-output ${!outputExpanded ? 'capped' : ''}`}>
                <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
                  {output}
                </ReactMarkdown>
                {!outputExpanded && output.length > 2000 && (
                  <div className="subagent-block-output-fade" />
                )}
              </div>
            )}
            {isSuccess && !hasOutput && (
              <div className="subagent-block-empty">Subagent 已完成，无输出内容</div>
            )}
            {isSuccess && hasOutput && output.length > 2000 && !outputExpanded && (
              <button
                type="button"
                className="subagent-block-expander"
                onClick={() => setOutputExpanded(true)}
              >
                展开全部输出
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
});

SubagentBlock.displayName = 'SubagentBlock';

export default SubagentBlock;
