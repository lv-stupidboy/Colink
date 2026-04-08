// isdp/web/src/components/thread/TaskList.tsx
import React, { useState } from 'react';
import { List, Button, Empty, Popconfirm, message } from 'antd';
import { PlusOutlined, CheckCircleOutlined, ClockCircleOutlined, DeleteOutlined } from '@ant-design/icons';
import type { Thread } from '@/types';
import api from '@/api/client';

interface TaskListProps {
  tasks: Thread[];
  activeThreadId: string | null;
  onSelectTask: (task: Thread) => void;
  onCreateTask: () => void;
  onDeleteTask?: (taskId: string) => void;
  isRunning?: boolean;
  projectId?: string;
}

export const TaskList: React.FC<TaskListProps> = ({
  tasks,
  activeThreadId,
  onSelectTask,
  onCreateTask,
  onDeleteTask,
  isRunning,
  projectId: _projectId,
}) => {
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const handleDelete = async (taskId: string) => {
    if (!taskId) return;

    setDeletingId(taskId);
    try {
      await api.threads.delete(taskId);
      message.success('对话已删除');
      if (onDeleteTask) {
        onDeleteTask(taskId);
      }
    } catch (error) {
      console.error('Failed to delete thread:', error);
      message.error('删除失败');
    } finally {
      setDeletingId(null);
    }
  };

  return (
    <div className="task-list" style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <div style={{ padding: '12px', borderBottom: '1px solid var(--border-color)' }}>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={onCreateTask}
          block
          disabled={isRunning}
        >
          新建对话
        </Button>
      </div>
      <div style={{ flex: 1, overflow: 'auto' }}>
        {tasks.length === 0 ? (
          <Empty
            description="暂无对话"
            style={{ marginTop: '40px' }}
          />
        ) : (
          <List
            dataSource={tasks}
            renderItem={(task) => (
              <List.Item
                onClick={() => !isRunning && onSelectTask(task)}
                style={{
                  cursor: 'pointer',
                  backgroundColor: activeThreadId === task.id ? 'var(--bg-hover)' : undefined,
                  padding: '12px 16px',
                  display: 'flex',
                  alignItems: 'center',
                }}
              >
                <div style={{ flex: 1, display: 'flex', alignItems: 'center', minWidth: 0 }}>
                  {task.status === 'complete' ? (
                    <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 18, marginRight: 12 }} />
                  ) : (
                    <ClockCircleOutlined style={{ color: '#1890ff', fontSize: 18, marginRight: 12 }} />
                  )}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {task.name || '未命名对话'}
                    </div>
                    {task.createdAt && (
                      <div style={{ fontSize: 12, color: '#999' }}>
                        {new Date(task.createdAt).toLocaleString()}
                      </div>
                    )}
                  </div>
                </div>
                <Popconfirm
                  title="确定删除此对话？"
                  description="删除后无法恢复"
                  onConfirm={(e) => {
                    e?.stopPropagation();
                    handleDelete(task.id);
                  }}
                  onCancel={(e) => e?.stopPropagation()}
                  okText="删除"
                  cancelText="取消"
                  okButtonProps={{ danger: true, loading: deletingId === task.id }}
                >
                  <Button
                    type="text"
                    icon={<DeleteOutlined />}
                    size="small"
                    danger
                    loading={deletingId === task.id}
                    onClick={(e) => e.stopPropagation()}
                    style={{ opacity: 0.6, marginLeft: 8 }}
                  />
                </Popconfirm>
              </List.Item>
            )}
          />
        )}
      </div>
    </div>
  );
};