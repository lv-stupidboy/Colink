// isdp/web/src/components/thread/TaskList/TaskList.tsx
import React, { useState, useEffect, useCallback, memo } from 'react';
import { Button, List, Spin, Empty, Typography } from 'antd';
import {
  PlusOutlined,
  MessageOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import type { Thread } from '@/types';
import api from '@/api/client';
import './TaskList.css';

const { Text } = Typography;

interface TaskListProps {
  // 项目ID（用于获取任务列表，当 tasks 未提供时使用）
  projectId?: string;
  // 外部传入的任务列表（优先使用）
  tasks?: Thread[];
  // 当前选中的任务ID
  activeThreadId: string | null;
  // 选择任务回调
  onSelectTask: (thread: Thread) => void;
  // 新建任务回调
  onCreateTask: () => void;
  // 当前是否正在运行
  isRunning?: boolean;
}

/**
 * 任务列表组件
 * 用于 Solo 模式下的任务管理
 */
export const TaskList: React.FC<TaskListProps> = memo(({
  projectId,
  tasks: externalTasks,
  activeThreadId,
  onSelectTask,
  onCreateTask,
  isRunning = false,
}) => {
  const [internalTasks, setInternalTasks] = useState<Thread[]>([]);
  const [loading, setLoading] = useState(true);

  // 加载任务列表（仅当没有外部传入时）
  const loadTasks = useCallback(async () => {
    // 如果外部传入了任务列表，不加载
    if (externalTasks !== undefined) {
      setLoading(false);
      return;
    }

    if (!projectId) {
      setInternalTasks([]);
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      const data = await api.threads.list(projectId);
      // 按更新时间倒序排列
      const sorted = (data || []).sort((a, b) =>
        new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
      );
      setInternalTasks(sorted);
    } catch (error) {
      console.error('Failed to load tasks:', error);
      setInternalTasks([]);
    } finally {
      setLoading(false);
    }
  }, [projectId, externalTasks]);

  useEffect(() => {
    loadTasks();
  }, [loadTasks]);

  // 使用外部传入的任务列表或内部加载的任务列表
  const tasks = externalTasks !== undefined ? externalTasks : internalTasks;

  // 获取状态图标
  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'complete':
        return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
      case 'running':
        return <ClockCircleOutlined spin style={{ color: '#1890ff' }} />;
      case 'failed':
        return <ExclamationCircleOutlined style={{ color: '#ff4d4f' }} />;
      default:
        return <ClockCircleOutlined style={{ color: '#8c8c8c' }} />;
    }
  };

  return (
    <div className="task-list">
      {/* 新建任务按钮 */}
      <div className="task-list-header">
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={onCreateTask}
          block
          className="task-list-new-btn"
          disabled={isRunning}
        >
          新建对话
        </Button>
      </div>

      {/* 任务列表 */}
      <div className="task-list-content">
        {loading && externalTasks === undefined ? (
          <div className="task-list-loading">
            <Spin size="small" />
          </div>
        ) : tasks.length === 0 ? (
          <div className="task-list-empty">
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="暂无对话记录"
            />
          </div>
        ) : (
          <List
            dataSource={tasks}
            renderItem={(task) => (
              <List.Item
                key={task.id}
                className={`task-item ${task.id === activeThreadId ? 'active' : ''}`}
                onClick={() => !isRunning && onSelectTask(task)}
              >
                <div className="task-item-content">
                  <MessageOutlined className="task-item-icon" />
                  <div className="task-item-info">
                    <Text className="task-item-name" ellipsis title={task.name}>
                      {task.name || '未命名对话'}
                    </Text>
                    <Text type="secondary" className="task-item-time">
                      {new Date(task.updatedAt).toLocaleDateString()}
                    </Text>
                  </div>
                  <div className="task-item-status">
                    {getStatusIcon(task.status)}
                  </div>
                </div>
              </List.Item>
            )}
          />
        )}
      </div>
    </div>
  );
});

TaskList.displayName = 'TaskList';