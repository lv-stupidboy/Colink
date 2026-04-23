// web/src/pages/Workflow/TeamGraphEditor/EdgeDetailPanel.tsx
import React, { useState, useEffect } from 'react';
import { Input, Button, Divider, Typography } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';
import { useGraphStore } from './useGraphStore';
import type { AgentConfig } from '@/types';

const { Text } = Typography;

interface EdgeDetailPanelProps {
  edgeId: string;
  readOnly?: boolean;
}

const EdgeDetailPanel: React.FC<EdgeDetailPanelProps> = ({ edgeId, readOnly = false }) => {
  const { edges, nodes, updateEdgeTriggerHint, removeEdge, mode } = useGraphStore();

  const edge = edges.find(e => e.id === edgeId);
  const sourceNode = nodes.find(n => n.id === edge?.source);
  const targetNode = nodes.find(n => n.id === edge?.target);

  const [triggerHint, setTriggerHint] = useState('');

  useEffect(() => {
    if (edge?.data?.triggerHint) {
      setTriggerHint(edge.data.triggerHint as string);
    } else {
      setTriggerHint('');
    }
  }, [edge]);

  if (!edge) {
    return (
      <div className="panel-content">
        <p style={{ color: 'var(--text-secondary)' }}>连线信息不可用</p>
      </div>
    );
  }

  const handleSave = () => {
    updateEdgeTriggerHint(edgeId, triggerHint);
  };

  const handleRemove = () => {
    removeEdge(edgeId);
  };

  const sourceAgent = sourceNode?.data?.agent as AgentConfig | undefined;
  const targetAgent = targetNode?.data?.agent as AgentConfig | undefined;
  const sourceName = sourceAgent?.name || edge.source;
  const targetName = targetAgent?.name || edge.target;

  return (
    <div className="panel-content">
      <div className="panel-header">
        <span className="panel-title">触发关系</span>
      </div>

      <Divider />

      <div className="panel-section">
        <Text type="secondary">源 Agent → 目标 Agent</Text>
        <div style={{ marginTop: 8, display: 'flex', alignItems: 'center', gap: 8 }}>
          <Text strong>{sourceName}</Text>
          <Text>→</Text>
          <Text strong>{targetName}</Text>
        </div>
      </div>

      <Divider />

      <div className="panel-section">
        <div className="panel-section-title">触发条件</div>
        <Input.TextArea
          value={triggerHint}
          onChange={(e) => setTriggerHint(e.target.value)}
          placeholder="例如：当需要前端实现时"
          rows={3}
          disabled={readOnly}
        />
      </div>

      {!readOnly && mode === 'edit' && (
        <>
          <Divider />
          <div className="panel-section">
            <Button
              type="primary"
              onClick={handleSave}
              style={{ marginBottom: 8 }}
              block
            >
              保存触发条件
            </Button>
            <Button
              danger
              icon={<DeleteOutlined />}
              onClick={handleRemove}
              block
            >
              删除连线
            </Button>
          </div>
        </>
      )}
    </div>
  );
};

export default EdgeDetailPanel;