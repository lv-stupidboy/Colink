import React, { useState, useEffect, useCallback } from 'react';
import { Tabs, Card, Empty, Spin, Badge, message } from 'antd';
import { FileTextOutlined } from '@ant-design/icons';
import HumanTaskCard from '@/components/HumanTaskCard';
import TaskExecuteModal from '@/components/HumanTaskCard/TaskExecuteModal';
import { api } from '@/api/client';
import type { HumanTask, HumanTaskStatus } from '@/types';

const MyTasks: React.FC = () => {
  const [tasks, setTasks] = useState<HumanTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<HumanTaskStatus>('pending');
  const [executeTask, setExecuteTask] = useState<HumanTask | null>(null);

  const loadTasks = useCallback(async (status?: HumanTaskStatus) => {
    setLoading(true);
    try {
      const data = await api.humanTasks.list(status);
      setTasks(data);
    } catch (err) {
      console.error('Failed to load tasks:', err);
      message.error('加载任务失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadTasks(activeTab);
  }, [activeTab, loadTasks]);

  const handleExecute = (task: HumanTask) => {
    setExecuteTask(task);
  };

  const handleExecuteSuccess = () => {
    loadTasks(activeTab);
    setExecuteTask(null);
  };

  const tabItems = [
    {
      key: 'pending',
      label: (
        <Badge count={tasks.filter((t) => t.status === 'pending').length} offset={[10, 0]}>
          <span>待处理</span>
        </Badge>
      ),
    },
    {
      key: 'completed',
      label: '已完成',
    },
    {
      key: 'cancelled',
      label: '已取消',
    },
  ];

  return (
    <Card
      title={
        <span>
          <FileTextOutlined style={{ marginRight: 8 }} />
          我的任务
        </span>
      }
    >
      <Tabs
        activeKey={activeTab}
        onChange={(key) => setActiveTab(key as HumanTaskStatus)}
        items={tabItems}
      />

      {loading ? (
        <Spin style={{ display: 'block', margin: '20px auto' }} />
      ) : tasks.length === 0 ? (
        <Empty description="暂无任务" />
      ) : (
        <div>
          {tasks.map((task) => (
            <HumanTaskCard
              key={task.id}
              task={task}
              compact
              onExecute={() => handleExecute(task)}
              onViewContext={() => {
                // TODO: 跳转到 Thread 页面
              }}
            />
          ))}
        </div>
      )}

      {executeTask && (
        <TaskExecuteModal
          task={executeTask}
          visible={!!executeTask}
          onClose={() => setExecuteTask(null)}
          onSuccess={handleExecuteSuccess}
        />
      )}
    </Card>
  );
};

export default MyTasks;