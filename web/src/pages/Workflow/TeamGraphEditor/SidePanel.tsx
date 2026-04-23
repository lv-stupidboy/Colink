// web/src/pages/Workflow/TeamGraphEditor/SidePanel.tsx
import React from 'react';
import { Empty, Button } from 'antd';
import { CloseOutlined } from '@ant-design/icons';
import AgentDetailPanel from './AgentDetailPanel';
import EdgeDetailPanel from './EdgeDetailPanel';
import { useGraphStore } from './useGraphStore';

const SidePanel: React.FC = () => {
  const { selectedNode, selectedEdge, setSelectedNode, setSelectedEdge, mode } = useGraphStore();

  const handleClose = () => {
    setSelectedNode(null);
    setSelectedEdge(null);
  };

  const readOnly = mode === 'preview';

  const renderContent = () => {
    if (selectedNode) {
      return <AgentDetailPanel nodeId={selectedNode} readOnly={readOnly} />;
    }
    if (selectedEdge) {
      return <EdgeDetailPanel edgeId={selectedEdge} readOnly={readOnly} />;
    }
    return (
      <div style={{ padding: 32 }}>
        <Empty
          description="点击节点或连线查看详情"
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      </div>
    );
  };

  const title = selectedNode
    ? 'Agent 详情'
    : selectedEdge
      ? '连线详情'
      : '详情面板';

  return (
    <div className="team-graph-editor-panel">
      <div className="panel-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <span className="panel-title">{title}</span>
        {(selectedNode || selectedEdge) && (
          <Button
            type="text"
            icon={<CloseOutlined />}
            onClick={handleClose}
            size="small"
          />
        )}
      </div>
      {renderContent()}
    </div>
  );
};

export default SidePanel;