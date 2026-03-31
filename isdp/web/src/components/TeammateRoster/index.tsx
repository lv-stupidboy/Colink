import React from 'react';
import { Card, Avatar, List, Tag, Space, Typography, Tooltip } from 'antd';
import { RobotOutlined, UserOutlined } from '@ant-design/icons';
import type { AgentConfig } from '@/types';
import { AgentRoleLabels } from '@/types';

const { Text } = Typography;

export interface TeammateRosterProps {
  agents: AgentConfig[];
  loading?: boolean;
  onAgentClick?: (agent: AgentConfig) => void;
  currentAgentId?: string;
}

/**
 * TeammateRoster 组件
 * 显示当前团队中可 @ 的队友列表及其擅长领域
 * 用于自由协作模式
 */
const TeammateRoster: React.FC<TeammateRosterProps> = ({
  agents,
  loading = false,
  onAgentClick,
  currentAgentId,
}) => {
  if (agents.length === 0 && !loading) {
    return null;
  }

  return (
    <Card
      size="small"
      title={
        <Space>
          <UserOutlined />
          <span>团队成员</span>
          <Tag color="blue">{agents.length}</Tag>
        </Space>
      }
      style={{ marginBottom: 16 }}
      loading={loading}
      bodyStyle={{ padding: '8px 12px' }}
    >
      <List
        size="small"
        dataSource={agents}
        renderItem={(agent) => {
          const isCurrentAgent = agent.id === currentAgentId;
          return (
            <List.Item
              style={{
                padding: '8px 4px',
                cursor: onAgentClick ? 'pointer' : 'default',
                backgroundColor: isCurrentAgent ? '#f0f5ff' : 'transparent',
                borderRadius: 4,
              }}
              onClick={() => onAgentClick?.(agent)}
            >
              <Space style={{ width: '100%' }}>
                <Avatar
                  size="small"
                  icon={<RobotOutlined />}
                  style={{
                    backgroundColor: isCurrentAgent ? '#1890ff' : '#87d068',
                  }}
                />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <Text strong style={{ fontSize: 13 }}>
                      {agent.name}
                    </Text>
                    {isCurrentAgent && (
                      <Tag color="blue" style={{ fontSize: 10, padding: '0 4px', margin: 0 }}>
                        当前
                      </Tag>
                    )}
                  </div>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {AgentRoleLabels[agent.role as keyof typeof AgentRoleLabels] || agent.role}
                  </Text>
                </div>
                {agent.mentionPatterns && agent.mentionPatterns.length > 0 && (
                  <Tooltip title={`@${agent.mentionPatterns[0]} 触发`}>
                    <Tag color="green" style={{ fontSize: 10, padding: '0 4px' }}>
                      @{agent.mentionPatterns[0]}
                    </Tag>
                  </Tooltip>
                )}
              </Space>
            </List.Item>
          );
        }}
      />
    </Card>
  );
};

export default TeammateRoster;