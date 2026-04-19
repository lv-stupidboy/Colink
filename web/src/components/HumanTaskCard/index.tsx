import React from 'react';
import { Card, Button, Space, Tag, Typography } from 'antd';
import { ClockCircleOutlined, UserOutlined, RightOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import type { HumanTask } from '@/types';

const { Text, Paragraph } = Typography;

interface HumanTaskCardProps {
  task: HumanTask;
  onExecute?: () => void;
  compact?: boolean; // 紧凑模式（用于任务中心列表）
}

const statusColors: Record<string, string> = {
  pending: 'orange',
  completed: 'green',
  cancelled: 'default',
};

const statusLabels: Record<string, string> = {
  pending: '待处理',
  completed: '已完成',
  cancelled: '已取消',
};

const HumanTaskCard: React.FC<HumanTaskCardProps> = ({
  task,
  onExecute,
  compact = false,
}) => {
  const navigate = useNavigate();

  const timeAgo = () => {
    const created = new Date(task.createdAt);
    const now = new Date();
    const diffMs = now.getTime() - created.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 60) return `${diffMin}分钟前`;
    const diffHour = Math.floor(diffMin / 60);
    if (diffHour < 24) return `${diffHour}小时前`;
    return `${Math.floor(diffHour / 24)}天前`;
  };

  const handleNavigate = () => {
    navigate(`/thread/${task.threadId}`);
  };

  if (compact) {
    // 紧凑模式：任务中心列表项
    return (
      <Card
        size="small"
        style={{ marginBottom: 8, cursor: 'pointer' }}
        hoverable
        onClick={handleNavigate}
      >
        <Space direction="vertical" size="small" style={{ width: '100%' }}>
          <Space style={{ width: '100%', justifyContent: 'space-between' }}>
            <Space>
              <UserOutlined />
              <Text strong>{task.agentName}</Text>
              <Tag color={statusColors[task.status]}>{statusLabels[task.status]}</Tag>
            </Space>
            <RightOutlined style={{ color: 'var(--text-secondary)' }} />
          </Space>
          <Paragraph
            ellipsis={{ rows: 2 }}
            style={{ margin: 0, color: 'var(--text-secondary)' }}
          >
            {task.waitReason}
          </Paragraph>
          <Space split={<Text type="secondary">|</Text>}>
            <Text type="secondary"><ClockCircleOutlined /> {timeAgo()}</Text>
          </Space>
        </Space>
      </Card>
    );
  }

  // 完整模式：Thread 内卡片
  return (
    <Card
      size="small"
      style={{ marginBottom: 16 }}
    >
      <Space direction="vertical" size="small" style={{ width: '100%' }}>
        <Space style={{ width: '100%', justifyContent: 'space-between' }}>
          <Space>
            <UserOutlined />
            <Text strong>{task.agentName}</Text>
            <Tag color={statusColors[task.status]}>{statusLabels[task.status]}</Tag>
          </Space>
          <Text type="secondary"><ClockCircleOutlined /> {timeAgo()}</Text>
        </Space>

        <div>
          <Text type="secondary">等待原因:</Text>
          <Paragraph
            style={{
              background: 'var(--bg-container)',
              padding: 12,
              borderRadius: 4,
              marginTop: 8,
              marginBottom: 0,
            }}
          >
            {task.waitReason}
          </Paragraph>
        </div>

        {task.status === 'pending' && (
          <Space>
            <Button type="primary" size="small" onClick={onExecute}>
              前往处理
            </Button>
            <Button size="small" onClick={handleNavigate}>
              查看对话
            </Button>
          </Space>
        )}

        {task.status === 'completed' && task.completedAt && (
          <Text type="secondary">
            完成于 {new Date(task.completedAt).toLocaleString()}
          </Text>
        )}
      </Space>
    </Card>
  );
};

export default HumanTaskCard;