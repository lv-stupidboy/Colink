// web/src/pages/Workflow/TeamGraphEditor/AgentNode.tsx
import React, { memo } from 'react';
import { Handle, Position, type NodeProps } from '@xyflow/react';
import AgentTypeIcon from '@/components/AgentTypeIcon';
import type { AgentConfig } from '@/types';
import './TeamGraphEditor.css';

interface AgentNodeData {
  agent: AgentConfig;
}

const AgentNode: React.FC<NodeProps<AgentNodeData>> = ({ data, selected }) => {
  const { agent } = data;

  return (
    <div className={`agent-node ${selected ? 'selected' : ''}`}>
      <Handle type="target" position={Position.Left} className="agent-node-handle" />

      <div className="agent-node-icon">
        <AgentTypeIcon
          requiresHuman={agent.requiresHuman}
          isSystem={agent.isSystem}
          size={24}
        />
      </div>
      <div className="agent-node-name">
        {agent.name.length > 12 ? agent.name.slice(0, 12) + '...' : agent.name}
      </div>
      {agent.isSystem && (
        <div className="agent-node-badge">系统</div>
      )}

      <Handle type="source" position={Position.Right} className="agent-node-handle" />
    </div>
  );
};

export default memo(AgentNode);