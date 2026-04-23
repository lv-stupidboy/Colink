// web/src/pages/Workflow/TeamGraphEditor/GraphCanvas.tsx
import React, { useCallback, useEffect } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  type Node,
  type Edge,
  type Connection,
  type OnConnect,
  type NodeChange,
  type EdgeChange,
} from '@xyflow/react';
import { Empty, Button } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import AgentNode from './AgentNode';
import { useGraphStore } from './useGraphStore';
import './TeamGraphEditor.css';

// Define node types with proper typing
const nodeTypes = {
  agentNode: AgentNode,
};

interface GraphCanvasProps {
  onNodeClick?: (nodeId: string) => void;
  onEdgeClick?: (edgeId: string) => void;
}

const GraphCanvas: React.FC<GraphCanvasProps> = ({ onNodeClick, onEdgeClick }) => {
  const {
    mode,
    nodes: storeNodes,
    edges: storeEdges,
    setNodes,
    setEdges,
    setHasChanges,
    addEdge: addEdgeToStore,
    addNode,
    allAgents,
    setMode,
  } = useGraphStore();

  // Update store when nodes/edges change from store
  useEffect(() => {
    setNodes(storeNodes);
  }, [storeNodes, setNodes]);

  useEffect(() => {
    setEdges(storeEdges);
  }, [storeEdges, setEdges]);

  const onConnect: OnConnect = useCallback((connection: Connection) => {
    if (mode === 'edit' && connection.source && connection.target) {
      addEdgeToStore(connection.source, connection.target);
      setHasChanges(true);
    }
  }, [mode, addEdgeToStore, setHasChanges]);

  const handleNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    onNodeClick?.(node.id);
  }, [onNodeClick]);

  const handleEdgeClick = useCallback((_: React.MouseEvent, edge: Edge) => {
    onEdgeClick?.(edge.id);
  }, [onEdgeClick]);

  const handleAddFirstAgent = () => {
    if (allAgents.length > 0) {
      setMode('edit');
      addNode(allAgents[0]);
    }
  };

  const handleNodesChange = useCallback((_changes: NodeChange[]) => {
    if (mode === 'edit') {
      setHasChanges(true);
    }
  }, [mode, setHasChanges]);

  const handleEdgesChange = useCallback((_changes: EdgeChange[]) => {
    if (mode === 'edit') {
      setHasChanges(true);
    }
  }, [mode, setHasChanges]);

  if (storeNodes.length === 0) {
    return (
      <div className="graph-empty-state">
        <Empty
          description="团队暂无 Agent"
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        >
          {mode === 'edit' && allAgents.length > 0 ? (
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={handleAddFirstAgent}
            >
              添加第一个 Agent
            </Button>
          ) : (
            <Button onClick={() => setMode('edit')}>
              切换到编辑模式
            </Button>
          )}
        </Empty>
      </div>
    );
  }

  return (
    <ReactFlow
      nodes={storeNodes}
      edges={storeEdges}
      onNodesChange={mode === 'edit' ? handleNodesChange : undefined}
      onEdgesChange={mode === 'edit' ? handleEdgesChange : undefined}
      onConnect={onConnect}
      onNodeClick={handleNodeClick}
      onEdgeClick={handleEdgeClick}
      nodeTypes={nodeTypes}
      nodesDraggable={mode === 'edit'}
      nodesConnectable={mode === 'edit'}
      elementsSelectable={true}
      fitView
      fitViewOptions={{ padding: 0.2 }}
    >
      <Background gap={16} size={1} />
      <Controls showInteractive={mode === 'edit'} />
      <MiniMap
        nodeColor={(node) => {
          const data = node.data as { agent?: { isSystem?: boolean } };
          return data.agent?.isSystem ? 'var(--color-warning)' : 'var(--color-primary)';
        }}
        maskColor="rgba(0, 0, 0, 0.1)"
      />
    </ReactFlow>
  );
};

export default GraphCanvas;