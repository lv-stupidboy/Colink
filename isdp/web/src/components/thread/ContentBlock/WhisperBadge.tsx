// isdp/web/src/components/thread/ContentBlock/WhisperBadge.tsx
import React, { memo } from 'react';
import { Tag } from 'antd';
import { LockOutlined, UnlockOutlined } from '@ant-design/icons';

interface WhisperBadgeProps {
  revealedAt?: string;  // 解密时间
  isRevealed?: boolean; // 是否已揭秘
}

/**
 * 悄悄话标签组件
 * 显示消息可见性状态
 */
export const WhisperBadge: React.FC<WhisperBadgeProps> = memo(({
  revealedAt,
  isRevealed = false,
}) => {
  if (isRevealed) {
    return (
      <Tag
        className="whisper-badge whisper-badge-revealed"
        icon={<UnlockOutlined />}
        color="default"
      >
        已揭秘
      </Tag>
    );
  }

  return (
    <Tag
      className="whisper-badge whisper-badge-locked"
      icon={<LockOutlined />}
      color="warning"
    >
      悄悄话
      {revealedAt && (
        <span className="whisper-badge-timer">
          {formatRevealTime(revealedAt)}
        </span>
      )}
    </Tag>
  );
});

/**
 * 格式化揭秘时间
 */
function formatRevealTime(revealedAt: string): string {
  const revealDate = new Date(revealedAt);
  const now = new Date();
  const diffMs = revealDate.getTime() - now.getTime();

  if (diffMs <= 0) {
    return '即将揭秘';
  }

  const diffMins = Math.floor(diffMs / 60000);
  if (diffMins < 60) {
    return `${diffMins}分钟后揭秘`;
  }

  const diffHours = Math.floor(diffMins / 60);
  if (diffHours < 24) {
    return `${diffHours}小时后揭秘`;
  }

  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays}天后揭秘`;
}

WhisperBadge.displayName = 'WhisperBadge';