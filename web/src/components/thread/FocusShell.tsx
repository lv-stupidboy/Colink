import React, { useEffect } from 'react';
import './FocusShell.css';

interface FocusShellProps {
  children: React.ReactNode;
  onExit: () => void;
  title?: string;
}

/**
 * 专注模式容器 - 全屏覆盖，提供退出按钮和 ESC 快捷键
 */
export const FocusShell: React.FC<FocusShellProps> = ({ children, onExit, title }) => {
  // ESC 键退出
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onExit();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onExit]);

  return (
    <div className="focus-shell">
      {/* 退出按钮 */}
      <button
        className="focus-shell-exit"
        onClick={onExit}
        title="退出专注模式 (ESC)"
      >
        <svg width="8" height="8" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M1 1l8 8M9 1l-8 8" />
        </svg>
        <span>退出专注</span>
      </button>

      {/* 标题（可选） */}
      {title && <div className="focus-shell-title">{title}</div>}

      {/* 内容区域 */}
      <div className="focus-shell-content">
        {children}
      </div>
    </div>
  );
};

export default FocusShell;