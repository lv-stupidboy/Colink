// isdp/web/src/components/thread/TaskList.tsx
import React from 'react';
import { List, Button, Empty } from 'antd';
import { PlusOutlined, CheckCircleOutlined, ClockCircleOutlined } from '@ant-design/icons';
import type { Thread } from '@/types';

interface TaskListProps {
  tasks: Thread[];
  activeThreadId: string | null;
  onSelectTask: (task: Thread) => void;
  onCreateTask: () => void;
  isRunning?: boolean;
}

export const TaskList: React.FC<TaskListProps> = ({
  tasks,
  activeThreadId,
  onSelectTask,
  onCreateTask,
  isRunning,
}) => {
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
          新建任务
        </Button>
      </div>
      <div style={{ flex: 1, overflow: 'auto' }}>
        {tasks.length === 0 ? (
          <Empty
            description="暂无任务"
            style={{ marginTop: '40px' }}
          />
        ) : (
          <List
            dataSource={tasks}
            renderItem={(task) => (
              <List.Item
                onClick={() => onSelectTask(task)}
                style={{
                  cursor: 'pointer',
                  backgroundColor: activeThreadId === task.id ? 'var(--bg-hover)' : undefined,
                  padding: '12px 16px',
                }}
              >
                <List.Item.Meta
                  avatar={
                    task.status === 'complete' ? (
                      <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 18 }} />
                    ) : (
                      <ClockCircleOutlined style={{ color: '#1890ff', fontSize: 18 }} />
                    )
                  }
                  title={task.name || '未命名任务'}
                  description={task.createdAt ? new Date(task.createdAt).toLocaleString() : undefined}
                />
              </List.Item>
            )}
          />
        )}
      </div>
    </div>
  );
};