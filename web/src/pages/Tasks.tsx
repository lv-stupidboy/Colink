import React, { useEffect, useState, useCallback } from 'react';
import { Card, List, Button, Typography, Tag, Space, Empty, Spin } from 'antd';
import { RobotOutlined, ArrowRightOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '@/api/client';
import type { HumanTask } from '@/types';

const { Text, Title } = Typography;

const Tasks: React.FC = () => {
  const navigate = useNavigate();
  const [tasks, setTasks] = useState<HumanTask[]>([]);
  const [loading, setLoading] = useState(false);

  // 加载任务列表
  const loadTasks = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.humanTasks.list('pending');
      setTasks(data);
    } catch (error) {
      console.error('加载待办任务失败', error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadTasks();
  }, [loadTasks]);

  // WebSocket 实时更新（通过全局事件监听）
  useEffect(() => {
    const handleTaskCreated = (event: CustomEvent) => {
      const task = event.detail as HumanTask;
      setTasks(prev => [...prev, task]);
    };

    const handleTaskCompleted = (event: CustomEvent) => {
      const { invocationId } = event.detail;
      setTasks(prev => prev.filter(t => t.id !== invocationId));
    };

    const handleTaskCancelled = (event: CustomEvent) => {
      const { taskId } = event.detail;
      setTasks(prev => prev.filter(t => t.id !== taskId));
    };

    window.addEventListener('human_task_created', handleTaskCreated as EventListener);
    window.addEventListener('human_task_completed', handleTaskCompleted as EventListener);
    window.addEventListener('human_task_cancelled', handleTaskCancelled as EventListener);

    return () => {
      window.removeEventListener('human_task_created', handleTaskCreated as EventListener);
      window.removeEventListener('human_task_completed', handleTaskCompleted as EventListener);
      window.removeEventListener('human_task_cancelled', handleTaskCancelled as EventListener);
    };
  }, []);

  const handleEnterThread = (threadId: string) => {
    navigate(`/threads/${threadId}`);
  };

  const formatTime = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    const minutes = Math.floor(diff / 60000);

    if (minutes < 60) return `${minutes} 分钟前`;
    if (minutes < 24 * 60) return `${Math.floor(minutes / 60)} 小时前`;
    return date.toLocaleDateString();
  };

  // 获取任务类型标签
  const getTaskTypeTag = (taskType: string) => {
    const typeMap: Record<string, { color: string; label: string }> = {
      task_dispatch: { color: 'blue', label: '任务分发' },
      review: { color: 'orange', label: '评审' },
      confirm: { color: 'green', label: '确认' },
    };
    return typeMap[taskType] || { color: 'default', label: taskType };
  };

  return (
    <div className="tasks-page" style={{ padding: 24, maxWidth: 800, margin: '0 auto' }}>
      <Title level={3}>
        待办任务
        <Tag color="blue" style={{ marginLeft: 8 }}>{tasks.length}</Tag>
      </Title>

      {loading ? (
        <Spin size="large" style={{ display: 'block', margin: '40px auto' }} />
      ) : tasks.length === 0 ? (
        <Empty description="暂无待办任务" style={{ marginTop: 40 }} />
      ) : (
        <List
          dataSource={tasks}
          renderItem={(task) => {
            const typeTag = getTaskTypeTag(task.taskType);
            return (
              <Card
                style={{ marginBottom: 16 }}
                hoverable
                onClick={() => handleEnterThread(task.threadId)}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Space>
                    <RobotOutlined style={{ color: 'var(--color-primary)', fontSize: 20 }} />
                    <Text strong>{task.roleName}</Text>
                    <Tag color={typeTag.color}>{typeTag.label}</Tag>
                    <Tag color="processing">等待处理</Tag>
                  </Space>
                  <Text type="secondary">
                    {formatTime(task.createdAt)}
                  </Text>
                </div>
                <Text
                  type="secondary"
                  style={{
                    marginTop: 12,
                    display: 'block',
                    maxWidth: '100%',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap'
                  }}
                >
                  {task.taskContent || `${task.sourceAgentName} 分发的任务等待您的处理...`}
                </Text>
                <Button
                  type="primary"
                  icon={<ArrowRightOutlined />}
                  onClick={(e) => {
                    e.stopPropagation();
                    handleEnterThread(task.threadId);
                  }}
                  style={{ marginTop: 12 }}
                >
                  进入对话
                </Button>
              </Card>
            );
          }}
        />
      )}
    </div>
  );
};

export default Tasks;