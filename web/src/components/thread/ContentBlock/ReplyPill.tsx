// isdp/web/src/components/thread/ContentBlock/ReplyPill.tsx
import React, { memo } from 'react';
import { Tag } from 'antd';
import { MessageOutlined } from '@ant-design/icons';
import './ReplyPill.css';

interface ReplyPillProps {
  replyToAgentName?: string;
  replyPreview: string;
  onClick?: () => void;
}

/**
 * 回复引用标签组件
 * 显示被引用消息的预览，点击可跳转到原消息
 */
export const ReplyPill: React.FC<ReplyPillProps> = memo(({
  replyToAgentName,
  replyPreview,
  onClick,
}) => {
  // 截断预览文本（最多60字符）
  const truncatedPreview = replyPreview.length > 60
    ? `${replyPreview.slice(0, 60)}...`
    : replyPreview;

  return (
    <div
      className={`reply-pill ${onClick ? 'reply-pill-clickable' : ''}`}
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
    >
      <MessageOutlined className="reply-pill-icon" />
      <span className="reply-pill-content">
        {replyToAgentName && (
          <Tag className="reply-pill-agent" color="default">
            {replyToAgentName}
          </Tag>
        )}
        <span className="reply-pill-preview">
          {truncatedPreview}
        </span>
      </span>
    </div>
  );
});

ReplyPill.displayName = 'ReplyPill';