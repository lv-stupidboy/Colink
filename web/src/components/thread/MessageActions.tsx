// isdp/web/src/components/thread/MessageActions.tsx
import React, { memo, useState, useCallback } from 'react';
import { Modal, message as antdMessage } from 'antd';
import { DeleteOutlined, ForkOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';
import type { Message } from '@/types';

/**
 * 消息操作工具栏
 * 悬停时显示删除/分支按钮
 */

interface MessageActionsProps {
  message: Message;
  threadId: string;
  children: React.ReactNode;
  onDelete?: (messageId: string) => void;
}

export const MessageActions: React.FC<MessageActionsProps> = memo(({
  message,
  threadId,
  children,
  onDelete,
}) => {
  const navigate = useNavigate();
  const [branching, setBranching] = useState(false);

  const isUser = message.role === 'user';
  const canAct = !message.agentId; // 用户消息可以操作

  const handleDelete = useCallback(() => {
    Modal.confirm({
      title: '删除消息',
      content: '确认删除此消息？删除后可恢复。',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await axios.delete(`/api/v1/messages/${message.id}`);
          onDelete?.(message.id);
          antdMessage.success('消息已删除');
        } catch {
          antdMessage.error('删除失败');
        }
      },
    });
  }, [message.id, onDelete]);

  const handleBranch = useCallback(async () => {
    Modal.confirm({
      title: '从此消息分支',
      content: '将从此消息创建一个新的对话分支，复制到这条消息为止的所有历史。原对话保留不变。',
      okText: '创建分支',
      cancelText: '取消',
      onOk: async () => {
        setBranching(true);
        try {
          const response = await axios.post(`/api/v1/threads/${threadId}/branch`, {
            fromMessageId: message.id,
          });
          const data = response.data as { threadId?: string };
          if (data.threadId) {
            navigate(`/thread/${data.threadId}`);
            antdMessage.success('分支创建成功');
          }
        } catch {
          antdMessage.error('分支创建失败，请检查后端 API 是否支持');
        } finally {
          setBranching(false);
        }
      },
    });
  }, [message.id, threadId, navigate]);

  return (
    <div className="message-actions-wrapper group" style={{ position: 'relative' }}>
      {children}

      {/* 悬停工具栏 */}
      {canAct && (
        <div
          className="message-actions-toolbar"
          style={{
            position: 'absolute',
            top: isUser ? 32 : 4,
            right: 4,
            display: 'flex',
            gap: 4,
            opacity: 0,
            transition: 'opacity 0.2s',
            background: 'rgba(255, 255, 255, 0.9)',
            borderRadius: 6,
            padding: '2px 4px',
            boxShadow: '0 1px 3px rgba(0,0,0,0.1)',
          }}
        >
          <button
            onClick={handleDelete}
            style={{
              padding: 4,
              border: 'none',
              background: 'transparent',
              cursor: 'pointer',
              color: '#8c8c8c',
            }}
            title="删除"
          >
            <DeleteOutlined style={{ fontSize: 14 }} />
          </button>
          <button
            onClick={handleBranch}
            disabled={branching}
            style={{
              padding: 4,
              border: 'none',
              background: 'transparent',
              cursor: branching ? 'wait' : 'pointer',
              color: '#8c8c8c',
            }}
            title="从此消息分支"
          >
            <ForkOutlined style={{ fontSize: 14 }} />
          </button>
        </div>
      )}

      {/* CSS: group-hover 显示工具栏 */}
      <style>{`
        .message-actions-wrapper:hover .message-actions-toolbar {
          opacity: 1;
        }
      `}</style>
    </div>
  );
});

MessageActions.displayName = 'MessageActions';