// web/src/pages/Workflow/TeamGraphEditor/AgentDetailPanel.tsx
import React from 'react';
import { Descriptions, Button, Tag, Divider } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';
import AgentTypeIcon from '@/components/AgentTypeIcon';
import { useGraphStore } from './useGraphStore';
import type { AgentConfig } from '@/types';

interface AgentDetailPanelProps {
  nodeId: string;
  readOnly?: boolean;
}

const AgentDetailPanel: React.FC<AgentDetailPanelProps> = ({ nodeId, readOnly = false }) => {
  const { nodes, removeNode, mode } = useGraphStore();

  const node = nodes.find(n => n.id === nodeId);
  const agent: AgentConfig | undefined = node?.data?.agent as AgentConfig | undefined;

  if (!agent) {
    return (
      <div className="panel-content">
        <div className="panel-section">
          <p style={{ color: 'var(--text-secondary)' }}>Agent 信息不可用</p>
        </div>
      </div>
    );
  }

  const handleRemove = () => {
    removeNode(nodeId);
  };

  return (
    <div className="panel-content">
      <div className="panel-header" style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <AgentTypeIcon
          requiresHuman={agent.requiresHuman}
          isSystem={agent.isSystem}
          size={24}
        />
        <span className="panel-title">{agent.name}</span>
      </div>

      <Divider />

      <Descriptions column={1} size="small">
        <Descriptions.Item label="角色">{agent.role}</Descriptions.Item>
        <Descriptions.Item label="基础 Agent">{agent.baseAgent?.name || '-'}</Descriptions.Item>
        <Descriptions.Item label="需要人工">
          {agent.requiresHuman ? <Tag color="blue">是</Tag> : <Tag>否</Tag>}
        </Descriptions.Item>
        {agent.isSystem && (
          <Descriptions.Item label="系统角色">
            <Tag color="purple">系统预置</Tag>
          </Descriptions.Item>
        )}
        <Descriptions.Item label="描述">
          {agent.description || '-'}
        </Descriptions.Item>
      </Descriptions>

      {!readOnly && mode === 'edit' && !agent.isSystem && (
        <>
          <Divider />
          <div className="panel-section">
            <Button
              danger
              icon={<DeleteOutlined />}
              onClick={handleRemove}
              block
            >
              从团队移除
            </Button>
          </div>
        </>
      )}
    </div>
  );
};

export default AgentDetailPanel;