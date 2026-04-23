// web/src/pages/Workflow/TeamGraphEditor/Toolbar.tsx
import React from 'react';
import { Button, Segmented, Dropdown, message } from 'antd';
import {
  PlusOutlined,
  SaveOutlined,
  EyeOutlined,
  EditOutlined,
} from '@ant-design/icons';
import { useGraphStore } from './useGraphStore';
import type { MenuProps } from 'antd';

const Toolbar: React.FC = () => {
  const {
    mode,
    setMode,
    allAgents,
    nodes,
    addNode,
    hasChanges,
    saveChanges,
    saving,
  } = useGraphStore();

  const availableAgents = allAgents.filter(
    (agent) => !nodes.some((n) => n.id === agent.id)
  );

  const handleAddAgent = (agentId: string) => {
    const agent = allAgents.find((a) => a.id === agentId);
    if (agent) {
      addNode(agent);
      message.success(`已添加 ${agent.name}`);
    }
  };

  const handleSave = async () => {
    try {
      await saveChanges();
      message.success('保存成功');
    } catch {
      message.error('保存失败');
    }
  };

  const agentMenuItems: MenuProps['items'] = availableAgents.map((agent) => ({
    key: agent.id,
    label: (
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <span>{agent.name}</span>
        {agent.isSystem && <span style={{ color: 'var(--color-warning)', fontSize: 12 }}>系统</span>}
      </div>
    ),
    onClick: () => handleAddAgent(agent.id),
  }));

  return (
    <div className="team-graph-toolbar">
      <Segmented
        value={mode}
        onChange={(value) => setMode(value as 'preview' | 'edit')}
        options={[
          { value: 'preview', label: '预览', icon: <EyeOutlined /> },
          { value: 'edit', label: '编辑', icon: <EditOutlined /> },
        ]}
      />

      {mode === 'edit' && (
        <>
          <Dropdown
            menu={{ items: agentMenuItems }}
            disabled={availableAgents.length === 0}
          >
            <Button icon={<PlusOutlined />}>
              添加 Agent
            </Button>
          </Dropdown>

          {hasChanges && (
            <Button
              type="primary"
              icon={<SaveOutlined />}
              onClick={handleSave}
              loading={saving}
            >
              保存
            </Button>
          )}
        </>
      )}
    </div>
  );
};

export default Toolbar;