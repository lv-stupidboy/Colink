// web/src/pages/Workflow/AgentAvatar.tsx
import React from 'react';
import { Button, Tooltip } from 'antd';
import { DeleteOutlined, HolderOutlined } from '@ant-design/icons';
import AgentTypeIcon from '@/components/AgentTypeIcon';
import type { TeamAgent } from './useAgentDragSort';

interface AgentAvatarProps {
  agent: TeamAgent;
  index: number;
  onRemove: (index: number) => void;
  onClick: (agent: TeamAgent, index: number) => void;
  onDragStart: (index: number) => void;
  onDragOver: (index: number) => void;
  onDragEnd: () => void;
  isDragging: boolean;
  isDragOver: boolean;
  disabled?: boolean;
}

const AgentAvatar: React.FC<AgentAvatarProps> = ({
  agent,
  index,
  onRemove,
  onClick,
  onDragStart,
  onDragOver,
  onDragEnd,
  isDragging,
  isDragOver,
  disabled = false,
}) => {
  const tooltipContent = agent.config.name;

  const avatarClassName = 'workflow-agent-avatar agent';

  const handleDragStart = (e: React.DragEvent) => {
    e.dataTransfer.effectAllowed = 'move';
    onDragStart(index);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
    onDragOver(index);
  };

  const handleDragEnd = () => {
    onDragEnd();
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    onDragEnd();
  };

  return (
    <div
      className={`workflow-agent-avatar-wrapper ${isDragging ? 'dragging' : ''} ${isDragOver ? 'drag-over' : ''}`}
      draggable={!disabled}
      onDragStart={handleDragStart}
      onDragOver={handleDragOver}
      onDragEnd={handleDragEnd}
      onDrop={handleDrop}
    >
      {/* 拖拽手柄 */}
      {!disabled && (
        <div className="workflow-agent-drag-handle">
          <HolderOutlined />
        </div>
      )}

      {/* Agent 头像 */}
      <Tooltip title={tooltipContent} placement="top">
        <div className={avatarClassName} onClick={() => onClick(agent, index)}>
          <AgentTypeIcon
            requiresHuman={agent.config.requiresHuman}
            isSystem={agent.config.isSystem}
            size={20}
            iconColor="#fff"
            className="workflow-agent-icon"
          />
        </div>
      </Tooltip>

      {/* Agent 名称 */}
      <Tooltip title={tooltipContent} placement="bottom">
        <div className="workflow-agent-name">{agent.config.name}</div>
      </Tooltip>

      {/* 删除按钮 */}
      {!disabled && (
        <Button
          type="text"
          danger
          size="small"
          icon={<DeleteOutlined />}
          className="workflow-agent-remove"
          onClick={() => onRemove(index)}
        />
      )}
    </div>
  );
};

export default AgentAvatar;