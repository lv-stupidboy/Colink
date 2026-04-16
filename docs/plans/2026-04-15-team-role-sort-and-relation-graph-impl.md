# Team Role Sort and Relation Graph Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现团队内 Agent 拖拽排序和基于 transitions 的关系图展示功能

**Architecture:** 使用 CSS 原生拖拽实现排序，SVG 绘制关系图节点和连线，复用现有 Workflow API

**Tech Stack:** React, Ant Design, SVG, CSS Drag Events

---

## Phase 1: 角色拖拽排序

### Task 1: 创建拖拽排序 Hook

**Files:**
- Create: `web/src/pages/Workflow/useAgentDragSort.ts`

**Step 1: 创建 hook 文件骨架**

```typescript
// web/src/pages/Workflow/useAgentDragSort.ts
import { useState, useCallback } from 'react';

interface TeamAgent {
  config: { id: string; name: string; isSystem?: boolean };
  triggers: Array<{ toAgentId: string; triggerHint: string }>;
}

interface DragState {
  draggingIndex: number | null;
  dragOverIndex: number | null;
}

export const useAgentDragSort = (
  teamId: string,
  agents: TeamAgent[],
  onSave: (agentIds: string[]) => Promise<void>
) => {
  const [dragState, setDragState] = useState<DragState>({
    draggingIndex: null,
    dragOverIndex: null,
  });
  const [isSaving, setIsSaving] = useState(false);

  const handleDragStart = useCallback((index: number) => {
    setDragState({ draggingIndex: index, dragOverIndex: null });
  }, []);

  const handleDragOver = useCallback((index: number) => {
    if (dragState.draggingIndex !== null && dragState.draggingIndex !== index) {
      setDragState(prev => ({ ...prev, dragOverIndex: index }));
    }
  }, [dragState.draggingIndex]);

  const handleDragEnd = useCallback(async () => {
    const { draggingIndex, dragOverIndex } = dragState;
    
    if (draggingIndex === null || dragOverIndex === null || draggingIndex === dragOverIndex) {
      setDragState({ draggingIndex: null, dragOverIndex: null });
      return;
    }

    // 计算新顺序
    const newAgents = [...agents];
    const [removed] = newAgents.splice(draggingIndex, 1);
    newAgents.splice(dragOverIndex, 0, removed);
    
    const newAgentIds = newAgents.map(a => a.config.id);

    setIsSaving(true);
    try {
      await onSave(newAgentIds);
    } finally {
      setIsSaving(false);
      setDragState({ draggingIndex: null, dragOverIndex: null });
    }
  }, [dragState, agents, onSave]);

  return {
    dragState,
    isSaving,
    handleDragStart,
    handleDragOver,
    handleDragEnd,
  };
};
```

**Step 2: 验证 hook 导出正确**

在文件末尾确保导出:
```typescript
export default useAgentDragSort;
```

**Step 3: 提交**

```bash
git add web/src/pages/Workflow/useAgentDragSort.ts
git commit -m "feat(workflow): add drag sort hook for team agents"
```

---

### Task 2: 创建 AgentAvatar 组件

**Files:**
- Create: `web/src/pages/Workflow/AgentAvatar.tsx`

**Step 1: 创建 AgentAvatar 组件**

```typescript
// web/src/pages/Workflow/AgentAvatar.tsx
import React from 'react';
import { Button, Tooltip } from 'antd';
import { UserOutlined, CrownOutlined, DeleteOutlined, HolderOutlined } from '@ant-design/icons';
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
      <div className="workflow-agent-avatar" onClick={() => onClick(agent, index)}>
        {agent.config.isSystem ? (
          <CrownOutlined className="workflow-agent-icon system" />
        ) : (
          <UserOutlined className="workflow-agent-icon" />
        )}
      </div>
      
      {/* Agent 名称 */}
      <div className="workflow-agent-name">{agent.config.name}</div>
      
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
```

**Step 2: 提交**

```bash
git add web/src/pages/Workflow/AgentAvatar.tsx
git commit -m "feat(workflow): add AgentAvatar component with drag support"
```

---

### Task 3: 抽取 TeamCard 组件

**Files:**
- Create: `web/src/pages/Workflow/TeamCard.tsx`
- Modify: `web/src/pages/Workflow/index.tsx:323-425`

**Step 1: 创建 TeamCard 组件**

```typescript
// web/src/pages/Workflow/TeamCard.tsx
import React, { useState, useCallback } from 'react';
import { Button, Popconfirm, Tag, Popover, Input, message } from 'antd';
import { DeleteOutlined, PlusOutlined, TeamOutlined, EditOutlined, CrownOutlined, UserOutlined, SwitcherOutlined } from '@ant-design/icons';
import type { TeamAgent, TeamView, AgentConfig, Transition } from '@/types';
import AgentAvatar from './AgentAvatar';
import TeamRelationGraph from './TeamRelationGraph';
import useAgentDragSort from './useAgentDragSort';
import { AgentRoleLabels } from '@/types';
import './Workflow.css';

interface TeamCardProps {
  team: TeamView;
  agents: AgentConfig[];
  onUpdateName: (teamId: string, name: string, description?: string) => void;
  onDelete: (teamId: string) => void;
  onAddAgent: (teamId: string, agentId: string) => void;
  onRemoveAgent: (teamId: string, agentIndex: number) => void;
  onOpenTriggerModal: (teamId: string, agent: TeamAgent, index: number) => void;
  onUpdateAgentOrder: (teamId: string, agentIds: string[], transitions: Transition[]) => Promise<void>;
}

const TeamCard: React.FC<TeamCardProps> = ({
  team,
  agents,
  onUpdateName,
  onDelete,
  onAddAgent,
  onRemoveAgent,
  onOpenTriggerModal,
  onUpdateAgentOrder,
}) => {
  const [viewMode, setViewMode] = useState<'card' | 'graph'>('card');
  const [editingName, setEditingName] = useState(false);
  const [editName, setEditName] = useState(team.name);
  const [editDesc, setEditDesc] = useState(team.description || '');

  // 拖拽排序 hook
  const { dragState, isSaving, handleDragStart, handleDragOver, handleDragEnd } = useAgentDragSort(
    team.id,
    team.agents,
    async (agentIds) => {
      // 保持 transitions 不变
      const transitions = teamViewToTransitions(team.agents);
      await onUpdateAgentOrder(team.id, agentIds, transitions);
    }
  );

  // 获取可添加的 Agent
  const availableAgents = useCallback(() => {
    const existingIds = new Set(team.agents.map(a => a.config.id));
    return agents.filter(a => !existingIds.has(a.id));
  }, [team.agents, agents]);

  const handleSaveName = () => {
    if (editName.trim()) {
      onUpdateName(team.id, editName.trim(), editDesc.trim() || undefined);
      setEditingName(false);
    }
  };

  // TeamAgent 转 Transition
  const teamViewToTransitions = (teamAgents: TeamAgent[]): Transition[] => {
    const transitions: Transition[] = [];
    teamAgents.forEach(agent => {
      agent.triggers.forEach(trigger => {
        transitions.push({
          fromAgentId: agent.config.id,
          toAgentId: trigger.toAgentId,
          type: 'sequence',
          triggerHint: trigger.triggerHint,
        });
      });
    });
    return transitions;
  };

  return (
    <div className="workflow-team-card">
      {/* 团队头部 */}
      <div className="workflow-team-header">
        <div className="workflow-team-title-wrapper">
          {editingName ? (
            <div className="workflow-team-name-editor">
              <Input
                value={editName}
                onChange={e => setEditName(e.target.value)}
                style={{ width: 160, marginRight: 8 }}
                autoFocus
              />
              <Input
                value={editDesc}
                onChange={e => setEditDesc(e.target.value)}
                placeholder="描述"
                style={{ width: 200, marginRight: 8 }}
              />
              <Button type="primary" size="small" onClick={handleSaveName}>保存</Button>
              <Button size="small" onClick={() => setEditingName(false)}>取消</Button>
            </div>
          ) : (
            <div
              className={`workflow-team-name ${team.isSystem ? '' : 'editable'}`}
              onClick={() => !team.isSystem && setEditingName(true)}
            >
              <TeamOutlined style={{ marginRight: 8 }} />
              <span>{team.name}</span>
              {!team.isSystem && <EditOutlined style={{ marginLeft: 8, fontSize: 12, opacity: 0.5 }} />}
            </div>
          )}
          {team.isDefault && <Tag color="gold" style={{ marginLeft: 8 }}>默认</Tag>}
          {team.isSystem && <Tag color="purple" style={{ marginLeft: 4 }}>系统</Tag>}
        </div>
        
        <div className="workflow-team-header-actions">
          {/* 视图切换 */}
          <Button
            type="text"
            icon={<SwitcherOutlined />}
            size="small"
            onClick={() => setViewMode(viewMode === 'card' ? 'graph' : 'card')}
            title={viewMode === 'card' ? '切换到关系图' : '切换到卡片'}
          />
          
          {/* 删除按钮 */}
          {!team.isSystem && (
            <Popconfirm
              title="确定删除此团队？"
              onConfirm={() => onDelete(team.id)}
              okText="确定"
              cancelText="取消"
            >
              <Button type="text" danger icon={<DeleteOutlined />} size="small" />
            </Popconfirm>
          )}
        </div>
      </div>

      {/* 内容区域 */}
      {viewMode === 'card' ? (
        <div className="workflow-team-agents">
          {team.agents.map((agent, index) => (
            <AgentAvatar
              key={agent.config.id}
              agent={agent}
              index={index}
              onRemove={onRemoveAgent.bind(null, team.id)}
              onClick={onOpenTriggerModal.bind(null, team.id)}
              onDragStart={handleDragStart}
              onDragOver={handleDragOver}
              onDragEnd={handleDragEnd}
              isDragging={dragState.draggingIndex === index}
              isDragOver={dragState.dragOverIndex === index}
              disabled={team.isSystem}
            />
          ))}

          {/* 添加 Agent */}
          {availableAgents().length > 0 && (
            <Popover
              trigger="click"
              placement="bottom"
              content={
                <div className="workflow-add-agent-popover">
                  <div className="workflow-add-agent-list">
                    {availableAgents().map(agent => (
                      <div
                        key={agent.id}
                        className="workflow-add-agent-item"
                        onClick={() => onAddAgent(team.id, agent.id)}
                      >
                        {agent.isSystem ? (
                          <CrownOutlined style={{ color: '#faad14', marginRight: 8 }} />
                        ) : (
                          <UserOutlined style={{ marginRight: 8 }} />
                        )}
                        <span>{agent.name}</span>
                        <span style={{ marginLeft: 8, fontSize: 12, color: '#999' }}>
                          {AgentRoleLabels[agent.role] || agent.role}
                        </span>
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
        <TeamRelationGraph agents={team.agents} transitions={teamViewToTransitions(team.agents)} />
      )}
    </div>
  );
};

export default TeamCard;
```

**Step 2: 提交**

```bash
git add web/src/pages/Workflow/TeamCard.tsx
git commit -m "feat(workflow): extract TeamCard component with drag sort and view switch"
```

---

### Task 4: 更新 Workflow.css 添加拖拽样式

**Files:**
- Modify: `web/src/pages/Workflow/Workflow.css`

**Step 1: 添加拖拽相关样式**

```css
/* web/src/pages/Workflow/Workflow.css - 新增内容 */

/* 拖拽手柄 */
.workflow-agent-drag-handle {
  position: absolute;
  left: -8px;
  top: 50%;
  transform: translateY(-50%);
  width: 16px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: grab;
  color: var(--text-secondary);
  opacity: 0;
  transition: opacity 0.2s;
}

.workflow-agent-avatar-wrapper:hover .workflow-agent-drag-handle {
  opacity: 1;
}

.workflow-agent-drag-handle:hover {
  color: var(--color-primary);
}

/* 拖拽状态 */
.workflow-agent-avatar-wrapper.dragging {
  opacity: 0.5;
  background: var(--bg-container-hover);
}

.workflow-agent-avatar-wrapper.drag-over {
  border: 2px dashed var(--color-primary);
  background: var(--bg-container-hover);
}

/* 视图切换按钮 */
.workflow-team-header-actions {
  display: flex;
  gap: 8px;
}

/* 关系图容器 */
.workflow-relation-graph {
  height: 200px;
  overflow-x: auto;
  overflow-y: hidden;
  padding: 16px;
  background: var(--bg-container);
}

.workflow-relation-graph-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-secondary);
}
```

**Step 2: 提交**

```bash
git add web/src/pages/Workflow/Workflow.css
git commit -m "feat(workflow): add drag and relation graph styles"
```

---

## Phase 2: 关系图视图

### Task 5: 创建 TeamRelationGraph 组件

**Files:**
- Create: `web/src/pages/Workflow/TeamRelationGraph.tsx`

**Step 1: 创建关系图组件**

```typescript
// web/src/pages/Workflow/TeamRelationGraph.tsx
import React from 'react';
import { Tooltip, Empty } from 'antd';
import { CrownOutlined, UserOutlined } from '@ant-design/icons';
import type { TeamAgent, Transition } from '@/types';

interface TeamRelationGraphProps {
  agents: TeamAgent[];
  transitions: Transition[];
}

const NODE_WIDTH = 80;
const NODE_HEIGHT = 100;
const GAP = 40;
const START_X = 60;
const START_Y = 60;

const TeamRelationGraph: React.FC<TeamRelationGraphProps> = ({ agents, transitions }) => {
  if (agents.length === 0) {
    return (
      <div className="workflow-relation-graph">
        <Empty description="暂无 Agent" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      </div>
    );
  }

  if (transitions.length === 0) {
    return (
      <div className="workflow-relation-graph">
        <div className="workflow-relation-graph-empty">暂无流转关系配置</div>
      </div>
    );
  }

  // 计算节点位置
  const nodePositions = agents.map((agent, index) => ({
    agent,
    x: START_X + index * (NODE_WIDTH + GAP),
    y: START_Y,
  }));

  // 创建 agentId -> position 映射
  const positionMap = new Map(
    nodePositions.map(p => [p.agent.config.id, p])
  );

  // 计算连线颜色
  const getLineColor = (type: string) => {
    switch (type) {
      case 'sequence': return '#1890ff';
      case 'parallel': return '#52c41a';
      case 'merge': return '#fa8c16';
      default: return '#1890ff';
    }
  };

  // 计算连线路径
  const calculatePath = (from: { x: number; y: number }, to: { x: number; y: number }) => {
    const startX = from.x + NODE_WIDTH / 2;
    const startY = from.y + NODE_HEIGHT / 2;
    const endX = to.x + NODE_WIDTH / 2;
    const endY = to.y + NODE_HEIGHT / 2;
    
    // 贝塞尔曲线，中点偏移避免直线
    const midX = (startX + endX) / 2;
    const midY = startY - 30; // 向上偏移
    
    return `M ${startX} ${startY} Q ${midX} ${midY} ${endX} ${endY}`;
  };

  // 计算连线中点（用于显示触发提示）
  const calculateMidPoint = (from: { x: number; y: number }, to: { x: number; y: number }) => {
    const startX = from.x + NODE_WIDTH / 2;
    const endX = to.x + NODE_WIDTH / 2;
    return { x: (startX + endX) / 2, y: START_Y - 20 };
  };

  // 计算 SVG 宽度
  const svgWidth = Math.max(
    nodePositions.length * (NODE_WIDTH + GAP) + START_X * 2,
    400
  );

  return (
    <div className="workflow-relation-graph">
      <svg width={svgWidth} height={NODE_HEIGHT + 80} style={{ overflow: 'visible' }}>
        {/* 连线 */}
        {transitions.map((transition, idx) => {
          const fromPos = positionMap.get(transition.fromAgentId);
          const toPos = positionMap.get(transition.toAgentId);
          
          if (!fromPos || !toPos) return null;
          
          const path = calculatePath(fromPos, toPos);
          const midPoint = calculateMidPoint(fromPos, toPos);
          const color = getLineColor(transition.type);
          
          return (
            <g key={`transition-${idx}`}>
              {/* 连线路径 */}
              <path
                d={path}
                fill="none"
                stroke={color}
                strokeWidth="2"
                markerEnd="url(#arrow)"
              />
              
              {/* 触发提示 */}
              {transition.triggerHint && (
                <Tooltip title={transition.triggerHint}>
                  <text
                    x={midPoint.x}
                    y={midPoint.y}
                    textAnchor="middle"
                    fontSize="12"
                    fill="var(--text-secondary)"
                    style={{ cursor: 'pointer' }}
                  >
                    {transition.triggerHint.length > 20
                      ? transition.triggerHint.slice(0, 20) + '...'
                      : transition.triggerHint}
                  </text>
                </Tooltip>
              )}
            </g>
          );
        })}
        
        {/* 箭头定义 */}
        <defs>
          <marker
            id="arrow"
            markerWidth="10"
            markerHeight="10"
            refX="9"
            refY="3"
            orient="auto"
            markerUnits="strokeWidth"
          >
            <path d="M0,0 L0,6 L9,3 z" fill="#1890ff" />
          </marker>
        </defs>
        
        {/* 节点 */}
        {nodePositions.map(({ agent, x, y }) => (
          <g key={agent.config.id}>
            {/* 节点背景 */}
            <circle
              cx={x + NODE_WIDTH / 2}
              cy={y + 30}
              r={30}
              fill="var(--bg-container)"
              stroke="var(--border-color)"
              strokeWidth="2"
            />
            
            {/* 节点图标 */}
            {agent.config.isSystem ? (
              <CrownOutlined
                style={{
                  position: 'absolute',
                  left: x + NODE_WIDTH / 2 - 8,
                  top: y + 22,
                  color: '#faad14',
                }}
              />
            ) : (
              <UserOutlined
                style={{
                  position: 'absolute',
                  left: x + NODE_WIDTH / 2 - 8,
                  top: y + 22,
                  color: 'var(--text-primary)',
                }}
              />
            )}
            
            {/* 节点名称 */}
            <text
              x={x + NODE_WIDTH / 2}
              y={y + NODE_HEIGHT - 10}
              textAnchor="middle"
              fontSize="12"
              fill="var(--text-primary)"
            >
              {agent.config.name.length > 8
                ? agent.config.name.slice(0, 8) + '...'
                : agent.config.name}
            </text>
          </g>
        ))}
      </svg>
    </div>
  );
};

export default TeamRelationGraph;
```

**Step 2: 提交**

```bash
git add web/src/pages/Workflow/TeamRelationGraph.tsx
git commit -m "feat(workflow): add TeamRelationGraph component with SVG rendering"
```

---

## Phase 3: 整合与优化

### Task 6: 更新 index.tsx 使用 TeamCard

**Files:**
- Modify: `web/src/pages/Workflow/index.tsx`

**Step 1: 引入 TeamCard 替换内联渲染**

修改 index.tsx，引入 TeamCard:

```typescript
// 在文件顶部添加 import
import TeamCard from './TeamCard';

// 删除 renderTeamCard 函数（约第 323-425 行）
// 替换 teams.map(renderTeamCard) 为:
teams.map(team => (
  <TeamCard
    key={team.id}
    team={team}
    agents={agents}
    onUpdateName={handleUpdateTeamName}
    onDelete={handleDeleteTeam}
    onAddAgent={handleAddAgent}
    onRemoveAgent={handleRemoveAgent}
    onOpenTriggerModal={handleOpenTriggerModal}
    onUpdateAgentOrder={handleUpdateTeam}
  />
))
```

**Step 2: 确保 handleUpdateTeam 支持 transitions 参数**

检查现有 handleUpdateTeam 函数，确保能接收 agentIds 和 transitions:

```typescript
// 如果需要修改，添加此函数:
const handleUpdateTeam = async (teamId: string, agentIds: string[], transitions: Transition[]) => {
  try {
    await api.workflows.update(teamId, { agentIds, transitions });
    setTeams(teams.map(t =>
      t.id === teamId ? { 
        ...t, 
        agents: transitionsToTeamView(agentIds, transitions, agents) 
      } : t
    ));
    message.success('更新成功');
  } catch (error: any) {
    message.error(error?.response?.data?.error || '更新失败');
  }
};
```

**Step 3: 提交**

```bash
git add web/src/pages/Workflow/index.tsx
git commit -m "feat(workflow): integrate TeamCard with drag sort and relation graph"
```

---

### Task 7: 深色模式适配

**Files:**
- Modify: `web/src/pages/Workflow/Workflow.css`

**Step 1: 添加深色模式变量**

确保 CSS 使用 CSS 变量而非硬编码颜色:

```css
/* 深色模式适配 - 检查并修正 */
[data-theme='dark'] .workflow-relation-graph {
  background: var(--bg-container);
}

[data-theme='dark'] .workflow-agent-avatar-wrapper {
  background: var(--bg-container);
  border-color: var(--border-color);
}

[data-theme='dark'] .workflow-agent-avatar {
  background: var(--bg-container-hover);
}

[data-theme='dark'] .workflow-agent-icon {
  color: var(--text-primary);
}

[data-theme='dark'] .workflow-agent-icon.system {
  color: #faad14;
}
```

**Step 2: 提交**

```bash
git add web/src/pages/Workflow/Workflow.css
git commit -m "feat(workflow): add dark mode support for relation graph"
```

---

### Task 8: 测试验证

**Step 1: 启动前后端服务**

```bash
# 后端
go run ./cmd/server

# 前端
cd web && npm run dev
```

**Step 2: 手动测试排序功能**

- 打开团队页面 `http://localhost:3000/workflow`
- 拖拽 Agent 到新位置，验证顺序保存
- 测试边界情况：第一个、最后一个位置
- 系统团队不允许拖拽

**Step 3: 手动测试关系图功能**

- 点击视图切换按钮
- 验证节点显示正确
- 验证连线绘制正确
- 验证触发提示显示
- 切换深色模式验证样式

**Step 4: 修复发现的问题**

如有问题，创建修复 commit:

```bash
git add <修改的文件>
git commit -m "fix(workflow): <问题描述>"
```

---

### Task 9: 最终提交

**Step 1: 确保所有更改已提交**

```bash
git status
git log --oneline -10
```

**Step 2: 推送到远程**

```bash
git push origin cc
```

---

## 完成标志

- ✅ Agent 可拖拽排序
- ✅ 排序后 API 调用保存正确
- ✅ 关系图视图切换正常
- ✅ SVG 节点和连线渲染正确
- ✅ 深色模式样式正确
- ✅ 系统团队禁止排序
- ✅ 空团队/单 Agent 团队显示正常