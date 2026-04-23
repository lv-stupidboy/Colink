// web/src/pages/Workflow/TeamCard.tsx
import React, { useState, useCallback } from 'react';
import {
  Button,
  Tag,
  Popconfirm,
  Popover,
  Input,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  TeamOutlined,
  EditOutlined,
  AppstoreOutlined,
  ShareAltOutlined,
} from '@ant-design/icons';
import AgentTypeIcon from '@/components/AgentTypeIcon';
import type { AgentConfig, Transition } from '@/types';
import AgentAvatar from './AgentAvatar';
import TeamRelationGraph from './TeamRelationGraph';
import useAgentDragSort, { type TeamAgent } from './useAgentDragSort';
import './Workflow.css';

// Agent 触发配置
interface AgentTrigger {
  toAgentId: string;
  triggerHint: string;
}

// 团队视图
interface TeamView {
  id: string;
  name: string;
  description?: string;
  agents: TeamAgent[];
  isDefault: boolean;
  isSystem: boolean;
}

// AgentTrigger 视图转换为 Transition 数组
function teamViewToTransitions(teamAgents: TeamAgent[]): Transition[] {
  const transitions: Transition[] = [];

  teamAgents.forEach(agent => {
    agent.triggers.forEach(trigger => {
      transitions.push({
        fromAgentId: agent.config.id,
        toAgentId: trigger.toAgentId,
        type: 'sequence', // 默认顺序执行
        triggerHint: trigger.triggerHint,
      });
    });
  });

  return transitions;
}

// 视图切换类型
type ViewMode = 'card' | 'graph';

// TeamCard Props
interface TeamCardProps {
  team: TeamView;
  allAgents: AgentConfig[];
  onUpdateName: (name: string, description?: string) => void;
  onDelete: () => void;
  onAddAgent: (agentId: string) => void;
  onRemoveAgent: (index: number) => void;
  onOpenTriggerModal: (agent: TeamAgent, index: number) => void;
  onSaveAgentOrder: (agentIds: string[]) => Promise<void>;
  onOpenGraphEditor?: () => void;
}

// 团队名称行内编辑组件
const TeamNameEditor: React.FC<{
  name: string;
  description?: string;
  onSave: (name: string, description?: string) => void;
  disabled?: boolean;
}> = ({ name, description, onSave, disabled }) => {
  const [editing, setEditing] = useState(false);
  const [editName, setEditName] = useState(name);
  const [editDesc, setEditDesc] = useState(description || '');

  const handleSave = () => {
    if (editName.trim()) {
      onSave(editName.trim(), editDesc.trim() || undefined);
      setEditing(false);
    }
  };

  if (editing) {
    return (
      <div className="workflow-team-name-editor">
        <Input
          value={editName}
          onChange={e => setEditName(e.target.value)}
          placeholder="团队名称"
          style={{ width: 160, marginRight: 8 }}
          autoFocus
        />
        <Input
          value={editDesc}
          onChange={e => setEditDesc(e.target.value)}
          placeholder="描述（可选）"
          style={{ width: 200, marginRight: 8 }}
        />
        <Button type="primary" size="small" onClick={handleSave}>保存</Button>
        <Button size="small" onClick={() => setEditing(false)} style={{ marginLeft: 4 }}>取消</Button>
      </div>
    );
  }

  return (
    <div
      className={`workflow-team-name ${disabled ? '' : 'editable'}`}
      onClick={() => !disabled && setEditing(true)}
    >
      <TeamOutlined style={{ marginRight: 8 }} />
      <span>{name}</span>
      {!disabled && <EditOutlined style={{ marginLeft: 8, fontSize: 12, opacity: 0.5 }} />}
    </div>
  );
};

const TeamCard: React.FC<TeamCardProps> = ({
  team,
  allAgents,
  onUpdateName,
  onDelete,
  onAddAgent,
  onRemoveAgent,
  onOpenTriggerModal,
  onSaveAgentOrder,
  onOpenGraphEditor,
}) => {
  // 视图模式切换
  const [viewMode, setViewMode] = useState<ViewMode>('card');

  // 拖拽排序
  const { dragState, isSaving, handleDragStart, handleDragOver, handleDragEnd } = useAgentDragSort(
    team.id,
    team.agents,
    onSaveAgentOrder
  );

  // 获取可添加的 Agent 列表
  const getAvailableAgents = useCallback(() => {
    const existingIds = new Set(team.agents.map(a => a.config.id));
    return allAgents.filter(a => !existingIds.has(a.id));
  }, [team.agents, allAgents]);

  const availableAgents = getAvailableAgents();

  // 计算 transitions 用于关系图
  const transitions = teamViewToTransitions(team.agents);

  return (
    <div className="workflow-team-card">
      {/* 团队头部 */}
      <div className="workflow-team-header">
        <div className="workflow-team-title-wrapper">
          <TeamNameEditor
            name={team.name}
            description={team.description}
            onSave={onUpdateName}
            disabled={team.isSystem}
          />
          {team.isDefault && <Tag color="gold" style={{ marginLeft: 8 }}>默认</Tag>}
          {team.isSystem && <Tag color="purple" style={{ marginLeft: 4 }}>系统</Tag>}
        </div>

        <div className="workflow-team-header-actions">
          {/* 视图切换按钮 */}
          <Button
            type="text"
            icon={viewMode === 'card' ? <ShareAltOutlined /> : <AppstoreOutlined />}
            size="small"
            onClick={() => setViewMode(viewMode === 'card' ? 'graph' : 'card')}
            title={viewMode === 'card' ? '切换到关系图视图' : '切换到卡片视图'}
          />

          {/* 打开关系图编辑器按钮 */}
          {onOpenGraphEditor && (
            <Button
              type="text"
              icon={<EditOutlined />}
              size="small"
              onClick={onOpenGraphEditor}
              title="打开关系图编辑器"
            />
          )}

          {/* 删除按钮 */}
          {!team.isSystem && (
            <Popconfirm
              title="确定删除此团队？"
              onConfirm={onDelete}
              okText="确定"
              cancelText="取消"
            >
              <Button type="text" danger icon={<DeleteOutlined />} size="small" />
            </Popconfirm>
          )}
        </div>
      </div>

      {/* Agent 内容区域 */}
      {viewMode === 'card' ? (
        // 卡片视图 - Agent 列表
        <div className="workflow-team-agents">
          {team.agents.map((agent, index) => (
            <AgentAvatar
              key={agent.config.id}
              agent={agent}
              index={index}
              onRemove={onRemoveAgent}
              onClick={onOpenTriggerModal}
              onDragStart={handleDragStart}
              onDragOver={handleDragOver}
              onDragEnd={handleDragEnd}
              isDragging={dragState.draggingIndex === index}
              isDragOver={dragState.dragOverIndex === index}
              disabled={team.isSystem || isSaving}
            />
          ))}

          {/* 添加 Agent 按钮 */}
          {availableAgents.length > 0 && !team.isSystem && (
            <Popover
              trigger="click"
              placement="bottom"
              content={
                <div className="workflow-add-agent-popover">
                  <div className="workflow-add-agent-title">选择 Agent</div>
                  <div className="workflow-add-agent-list">
                    {availableAgents.map(agent => (
                      <div
                        key={agent.id}
                        className="workflow-add-agent-item"
                        onClick={() => onAddAgent(agent.id)}
                      >
                        <AgentTypeIcon
                          requiresHuman={agent.requiresHuman}
                          isSystem={agent.isSystem}
                          size={16}
                          style={{ marginRight: 8 }}
                        />
                        <span>{agent.name}</span>
                        </div>
                    ))}
                  </div>
                </div>
              }
            >
              <div className="workflow-agent-add">
                <PlusOutlined />
              </div>
            </Popover>
          )}
        </div>
      ) : (
        // 关系图视图
        <TeamRelationGraph
          agents={team.agents}
          transitions={transitions}
        />
      )}
    </div>
  );
};

export type { TeamAgent, TeamView, AgentTrigger };
export { teamViewToTransitions };
export default TeamCard;