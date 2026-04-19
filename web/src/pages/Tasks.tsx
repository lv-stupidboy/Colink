import React, { useEffect, useState, useCallback } from 'react';
import { Card, List, Typography, Tag, Space, Empty, Spin, Tabs, Button, Tooltip, message } from 'antd';
import { RobotOutlined, ArrowRightOutlined, CloseOutlined, CheckOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '@/api/client';
import type { HumanTask, HumanTaskStatus } from '@/types';

const { Text, Title } = Typography;

const Tasks: React.FC = () => {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<HumanTaskStatus>('pending');
  const [tasks, setTasks] = useState<HumanTask[]>([]);
  const [loading, setLoading] = useState(false);

  // 加载任务列表
  const loadTasks = useCallback(async (status?: HumanTaskStatus) => {
    setLoading(true);
    try {
      const data = await api.humanTasks.list(status || activeTab);
      setTasks(data);
    } catch (error) {
      console.error('加载待办任务失败', error);
    } finally {
      setLoading(false);
    }
  }, [activeTab]);

  useEffect(() => {
    loadTasks();
  }, [loadTasks]);

  // Tab 切换时重新加载
  const handleTabChange = (key: string) => {
    const status = key as HumanTaskStatus;
    setActiveTab(status);
    loadTasks(status);
  };

  // 手动完成任务
  const handleCompleteTask = async (taskId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await api.humanTasks.complete(taskId);
      message.success('任务已完成');
      setTasks(prev => prev.filter(t => t.id !== taskId));
    } catch (error) {
      message.error('完成任务失败');
    }
  };

  // 取消任务
  const handleCancelTask = async (taskId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await api.humanTasks.cancel(taskId);
      message.success('任务已取消');
      setTasks(prev => prev.filter(t => t.id !== taskId));
    } catch (error) {
      message.error('取消任务失败');
    }
  };

  // WebSocket 实时更新（通过全局事件监听）
  useEffect(() => {
    const handleTaskCreated = (event: CustomEvent) => {
      const task = event.detail as HumanTask;
      if (activeTab === 'pending') {
        setTasks(prev => [...prev, task]);
      }
    };

    const handleTaskCompleted = (event: CustomEvent) => {
      const { invocationId } = event.detail;
      setTasks(prev => prev.filter(t => t.invocationId !== invocationId));
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
  }, [activeTab]);

  const handleEnterThread = (threadId: string) => {
    navigate(`/threads/${threadId}`);
  };

  // 时间格式化：直接显示具体时间（精确到秒）
  const formatTime = (dateStr: string) => {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    if (isNaN(date.getTime())) return dateStr;
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  };

  // 状态标签颜色
  const getStatusTag = (status: HumanTaskStatus) => {
    switch (status) {
      case 'pending':
        return <Tag color="orange" style={{ fontSize: 11, padding: '0 4px' }}>等待处理</Tag>;
      case 'completed':
        return <Tag color="green" style={{ fontSize: 11, padding: '0 4px' }}>已完成</Tag>;
      case 'cancelled':
        return <Tag color="default" style={{ fontSize: 11, padding: '0 4px' }}>已取消</Tag>;
      default:
        return null;
    }
  };

  return (
    <div className="tasks-page" style={{ padding: 24, maxWidth: 800, margin: '0 auto' }}>
      <Title level={3} style={{ marginBottom: 16 }}>待办任务</Title>

      <Tabs
        activeKey={activeTab}
        onChange={handleTabChange}
        items={[
          { key: 'pending', label: `待处理 (${tasks.filter(t => t.status === 'pending').length})` },
          { key: 'completed', label: '已完成' },
          { key: 'cancelled', label: '已取消' },
        ]}
      />

      {loading ? (
        <Spin size="large" style={{ display: 'block', margin: '40px auto' }} />
      ) : tasks.length === 0 ? (
        <Empty description={activeTab === 'pending' ? '暂无待办任务' : '暂无记录'} style={{ marginTop: 40 }} />
      ) : (
        <List
          dataSource={tasks}
          renderItem={(task) => (
              <Card
                size="small"
                style={{ marginBottom: 8, cursor: 'pointer' }}
                hoverable
                onClick={() => handleEnterThread(task.threadId)}
                bodyStyle={{ padding: '12px 16px' }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Space size="small">
                    <RobotOutlined style={{ color: 'var(--color-primary)', fontSize: 16 }} />
                    <Text strong style={{ fontSize: 14 }}>{task.agentName}</Text>
                    {getStatusTag(task.status)}
                  </Space>
                  <Space size="small">
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {formatTime(task.createdAt)}
                    </Text>
                    {task.status === 'pending' && (
                      <Space size={0}>
                        <Tooltip title="标记任务完成">
                          <Button
                            type="text"
                            size="small"
                            icon={<CheckOutlined />}
                            onClick={(e) => handleCompleteTask(task.id, e)}
                            style={{ padding: '0 4px', color: '#52c41a' }}
                          />
                        </Tooltip>
                        <Tooltip title="标记任务取消">
                          <Button
                            type="text"
                            size="small"
                            danger
                            icon={<CloseOutlined />}
                            onClick={(e) => handleCancelTask(task.id, e)}
                            style={{ padding: '0 4px' }}
                          />
                        </Tooltip>
                      </Space>
                    )}
                    <ArrowRightOutlined style={{ color: 'var(--color-primary)', fontSize: 14 }} />
                  </Space>
                </div>
                {/* 项目和任务信息 */}
                <Space style={{ marginTop: 6 }} size={4}>
                  {task.projectName && (
                    <Tag style={{ fontSize: 11, padding: '0 4px', margin: 0 }}>{task.projectName}</Tag>
                  )}
                  {task.threadName && (
                    <Tag color="blue" style={{ fontSize: 11, padding: '0 4px', margin: 0 }}>{task.threadName}</Tag>
                  )}
                </Space>
                {/* 等待原因 */}
                {task.waitReason && (
                  <Text
                    type="secondary"
                    style={{
                      fontSize: 12,
                      marginTop: 6,
                      display: 'block',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap'
                    }}
                  >
                    {task.waitReason}
                  </Text>
                )}
              </Card>
            )}
        />
      )}
    </div>
  );
};

export default Tasks;