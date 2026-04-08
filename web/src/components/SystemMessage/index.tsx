import React from 'react';
import { Alert, Tag, Space, Typography } from 'antd';
import {
  InfoCircleOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  WarningOutlined,
  BellOutlined,
} from '@ant-design/icons';

const { Text } = Typography;

interface SystemMessageProps {
  type?: 'info' | 'success' | 'warning' | 'error' | 'notification';
  title?: string;
  content: string;
  timestamp?: string;
  tags?: string[];
}

/**
 * 系统消息组件
 * 用于显示系统通知、阶段变更、Agent 状态等
 */
export const SystemMessage: React.FC<SystemMessageProps> = ({
  type = 'info',
  title,
  content,
  timestamp,
  tags,
}) => {
  const getIcon = () => {
    switch (type) {
      case 'success':
        return <CheckCircleOutlined />;
      case 'warning':
        return <WarningOutlined />;
      case 'error':
        return <CloseCircleOutlined />;
      case 'notification':
        return <BellOutlined />;
      default:
        return <InfoCircleOutlined />;
    }
  };

  const getConfig = () => {
    switch (type) {
      case 'success':
        return { type: 'success' as const, color: '#52c41a' };
      case 'warning':
        return { type: 'warning' as const, color: '#faad14' };
      case 'error':
        return { type: 'error' as const, color: '#ff4d4f' };
      default:
        return { type: 'info' as const, color: '#1890ff' };
    }
  };

  const config = getConfig();

  return (
    <div className="system-message" style={{ margin: '8px 0' }}>
      <Alert
        type={config.type}
        showIcon
        icon={getIcon()}
        message={
          <Space>
            {title && <Text strong>{title}</Text>}
            {tags &&
              tags.map((tag, i) => (
                <Tag key={i} color={config.color}>
                  {tag}
                </Tag>
              ))}
            {timestamp && (
              <Text type="secondary" style={{ fontSize: 12 }}>
                {timestamp}
              </Text>
            )}
          </Space>
        }
        description={content}
        style={{ margin: 0 }}
      />
    </div>
  );
};

export default SystemMessage;
