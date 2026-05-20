import React from 'react';
import './ThreadSkeleton.css';

/**
 * 对话页骨架屏
 *
 * 用于首次加载（无数据）场景：
 * - 比全屏 spinner 更有进度感
 * - 接近真实 UI 布局，减少感知等待时间
 */
export const ThreadSkeleton: React.FC = () => {
  return (
    <div className="thread-skeleton">
      {/* 模拟 3 条消息骨架 */}
      {[1, 2, 3].map((i) => (
        <div key={i} className="skeleton-message">
          <div className="skeleton-message-avatar" />
          <div className="skeleton-message-content">
            <div className="skeleton-line skeleton-line-short" />
            <div className="skeleton-line skeleton-line-medium" />
            <div className="skeleton-line skeleton-line-long" />
          </div>
        </div>
      ))}
    </div>
  );
};
