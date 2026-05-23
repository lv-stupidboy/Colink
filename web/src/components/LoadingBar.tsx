import React from 'react';
import './LoadingBar.css';

interface LoadingBarProps {
  visible: boolean;
}

/**
 * 轻量顶部加载条
 *
 * 用于切屏返回等场景：
 * - 有数据时只显示此加载条，不阻塞 UI
 * - 避免全屏 loading 导致的白屏感
 */
export const LoadingBar: React.FC<LoadingBarProps> = ({ visible }) => {
  if (!visible) return null;

  return (
    <div className="loading-bar">
      <div className="loading-bar-progress" />
    </div>
  );
};
