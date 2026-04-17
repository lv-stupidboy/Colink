import React from 'react';
import { Card, Button, Space, Tag, Typography } from 'antd';
import { ClockCircleOutlined, UserOutlined, FileTextOutlined } from '@ant-design/icons';
import type { HumanTask } from '@/types';

const { Text, Paragraph } = Typography;

interface HumanTaskCardProps {
  task: HumanTask;
  onExecute?: () => void;
  onViewContext?: () => void;
  compact?: boolean; // 紧凑模式（用于任务中心列表）
}

const statusColors: Record<string, string> = {
  pending: 'orange',
  in_progress: 'blue',
  completed: 'green',
  rejected: 'red',
  failed: 'red',
};

const statusLabels: Record<string, string> = {
  pending: '待处理',
  in_progress: '进行中',
  completed: '已完成',
  rejected: '已拒绝',
  failed: '失败',
};

const HumanTaskCard: React.FC<HumanTaskCardProps> = ({
  task,
  onExecute,
  onViewContext,
  compact = false,
}) => {
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

  if (compact) {
    // 紧凑模式：任务中心列表项
    return (
      <Card
        size="small"
        style={{ marginBottom: 8 }}
        actions={task.status === 'pending' ? [
          <Button type="link" size="small" onClick={onExecute}>执行任务</Button>,
        ] : undefined}
      >
        <Space direction="vertical" size="small" style={{ width: '100%' }}>
          <Space>
            <FileTextOutlined />
            <Text strong>{task.roleName}: {task.taskContent.slice(0, 50)}...</Text>
          </Space>
          <Space split={<Text type="secondary">|</Text>}>
            <Text type="secondary">来源: @{task.sourceAgentName}</Text>
            <Text type="secondary"><ClockCircleOutlined /> {timeAgo()}</Text>
          </Space>
        </Space>
      </Card>
    );
  }

  // 完整模式：Thread 内卡片
  return (
    <Card
      title={
        <Space>
          <FileTextOutlined />
          <span>任务: {task.roleName}</span>
          <Tag color={statusColors[task.status]}>{statusLabels[task.status]}</Tag>
        </Space>
      }
      style={{ marginBottom: 16 }}
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Space>
          <UserOutlined />
          <Text type="secondary">来源: @{task.sourceAgentName}</Text>
        </Space>

        <div>
          <Text type="secondary">任务描述:</Text>
          <Paragraph
            style={{
              background: 'var(--bg-container)',
              padding: 12,
              borderRadius: 4,
              marginTop: 8,
            }}
          >
            {task.taskContent}
          </Paragraph>
        </div>

        <div>
          <Text type="secondary">期望交付物:</Text>
          <Paragraph type="secondary" style={{ marginTop: 4 }}>
            {task.expectedOutput}
          </Paragraph>
        </div>

        {task.status === 'pending' && (
          <Space>
            <Button type="primary" onClick={onExecute}>执行任务</Button>
            <Button onClick={onViewContext}>查看上下文</Button>
          </Space>
        )}

        {task.status === 'completed' && task.outputContent && (
          <div>
            <Text type="secondary">交付物:</Text>
            <Paragraph
              style={{
                background: 'var(--bg-container)',
                padding: 12,
                borderRadius: 4,
                marginTop: 8,
              }}
            >
              {task.outputContent}
            </Paragraph>
          </div>
        )}
      </Space>
    </Card>
  );
};

export default HumanTaskCard;