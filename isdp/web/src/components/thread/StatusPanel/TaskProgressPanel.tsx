import React from 'react';
import { CheckCircleOutlined, ClockCircleOutlined, MinusCircleOutlined } from '@ant-design/icons';
import { Progress } from 'antd';
import type { TaskProgress, TaskItem } from '@/types/status';

interface Props {
  progress: Record<string, TaskProgress>;
}

const TaskStatusIcon: React.FC<{ status: string }> = ({ status }) => {
  if (status === 'completed') return <CheckCircleOutlined style={{ color: '#22c55e', fontSize: 12 }} />;
  if (status === 'in_progress') return <ClockCircleOutlined style={{ color: '#3b82f6', fontSize: 12 }} />;
  return <MinusCircleOutlined style={{ color: '#d1d5db', fontSize: 12 }} />;
};

export const TaskProgressPanel: React.FC<Props> = ({ progress }) => {
  const hasTasks = Object.keys(progress).length > 0;

  if (!hasTasks) {
    return null;
  }

  return (
    <div className="status-section">
      <div className="status-section-title">任务进度</div>
      {Object.entries(progress).map(([agentId, tp]) => {
        const completed = tp.tasks.filter(t => t.status === 'completed').length;
        const total = tp.tasks.length;

        return (
          <div key={agentId} className="task-group">
            <div className="task-header">
              <span>Agent: {agentId.slice(0, 8)}</span>
              <span className="task-count">{completed}/{total}</span>
            </div>
            <Progress
              percent={total > 0 ? Math.round((completed / total) * 100) : 0}
              size="small"
              showInfo={false}
              strokeColor={tp.snapshotStatus === 'running' ? '#3b82f6' : '#22c55e'}
              trailColor="#e5e7eb"
            />
            <div className="task-list">
              {tp.tasks.slice(0, 5).map((task: TaskItem, idx: number) => (
                <div key={idx} className="task-item">
                  <TaskStatusIcon status={task.status} />
                  <span className="task-title">{task.title}</span>
                </div>
              ))}
              {tp.tasks.length > 5 && (
                <div className="task-item" style={{ color: '#6b7280' }}>
                  +{tp.tasks.length - 5} 更多任务
                </div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
};

export default TaskProgressPanel;