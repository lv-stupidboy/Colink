// web/src/pages/Workflow/TeamGraphEditor/index.tsx
import React, { useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Spin, Button, Alert } from 'antd';
import { ArrowLeftOutlined } from '@ant-design/icons';
import { ReactFlowProvider } from '@xyflow/react';
import GraphCanvas from './GraphCanvas';
import SidePanel from './SidePanel';
import Toolbar from './Toolbar';
import { useGraphStore } from './useGraphStore';
import './TeamGraphEditor.css';

const TeamGraphEditor: React.FC = () => {
  const { teamId } = useParams<{ teamId: string }>();
  const navigate = useNavigate();
  const { loading, loadData, reset, setSelectedNode, setSelectedEdge, error, setError } = useGraphStore();

  useEffect(() => {
    if (teamId) {
      loadData(teamId);
    }
    return () => {
      reset();
    };
  }, [teamId, loadData, reset]);

  const handleBack = () => {
    navigate('/workflow');
  };

  const handleNodeClick = (nodeId: string) => {
    setSelectedNode(nodeId);
  };

  const handleEdgeClick = (edgeId: string) => {
    setSelectedEdge(edgeId);
  };

  const handleErrorClose = () => {
    setError(null);
  };

  if (loading) {
    return (
      <div className="team-graph-editor">
        <div className="graph-loading-overlay">
          <Spin size="large" />
        </div>
      </div>
    );
  }

  return (
    <ReactFlowProvider>
      <div className="team-graph-editor">
        {error && (
          <Alert
            message={error}
            type="error"
            closable
            onClose={handleErrorClose}
            style={{ margin: 16 }}
          />
        )}

        <div className="team-graph-editor-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <Button
              type="text"
              icon={<ArrowLeftOutlined />}
              onClick={handleBack}
            >
              返回
            </Button>
            <span className="team-graph-editor-title">
              团队关系图
            </span>
          </div>
          <Toolbar />
        </div>

        <div className="team-graph-editor-content">
          <div className="team-graph-editor-canvas">
            <GraphCanvas
              onNodeClick={handleNodeClick}
              onEdgeClick={handleEdgeClick}
            />
          </div>
          <SidePanel />
        </div>
      </div>
    </ReactFlowProvider>
  );
};

export default TeamGraphEditor;