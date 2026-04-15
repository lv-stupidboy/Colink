// web/src/pages/Workflow/TeamRelationGraph.tsx
import React from 'react';
import { Empty } from 'antd';
import type { TeamAgent } from './useAgentDragSort';
import type { Transition } from '@/types';

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
    const midY = startY - 30;

    return `M ${startX} ${startY} Q ${midX} ${midY} ${endX} ${endY}`;
  };

  // 计算连线中点
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
        {/* 箭头定义 - 为每条连线动态创建 */}
        <defs>
          {transitions.map((transition, idx) => {
            const color = getLineColor(transition.type);
            return (
              <marker
                key={`arrow-${idx}`}
                id={`arrow-${idx}`}
                markerWidth="10"
                markerHeight="10"
                refX="9"
                refY="3"
                orient="auto"
                markerUnits="strokeWidth"
              >
                <path d="M0,0 L0,6 L9,3 z" fill={color} />
              </marker>
            );
          })}
        </defs>

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
              <path
                d={path}
                fill="none"
                stroke={color}
                strokeWidth="2"
                markerEnd={`url(#arrow-${idx})`}
              />
              {transition.triggerHint && (
                <text
                  x={midPoint.x}
                  y={midPoint.y}
                  textAnchor="middle"
                  fontSize="12"
                  fill="var(--text-secondary)"
                >
                  {transition.triggerHint.length > 20
                    ? transition.triggerHint.slice(0, 20) + '...'
                    : transition.triggerHint}
                </text>
              )}
            </g>
          );
        })}

        {/* 节点 */}
        {nodePositions.map(({ agent, x, y }) => (
          <g key={agent.config.id}>
            <circle
              cx={x + NODE_WIDTH / 2}
              cy={y + 30}
              r={30}
              fill="var(--bg-container)"
              stroke="var(--border-color)"
              strokeWidth="2"
            />
            <text
              x={x + NODE_WIDTH / 2}
              y={y + 30}
              textAnchor="middle"
              fontSize="14"
              fill={agent.config.isSystem ? 'var(--color-warning)' : 'var(--text-primary)'}
            >
              {agent.config.isSystem ? '👑' : '👤'}
            </text>
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