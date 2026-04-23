// web/src/pages/Workflow/TeamGraphEditor/useAutoLayout.ts
import dagre from '@dagrejs/dagre';
import { MarkerType } from '@xyflow/react';
import type { Node, Edge } from '@xyflow/react';

export interface LayoutResult {
  nodes: Node[];
  nodeLevels: Map<string, number>;
}

export interface LayoutConfig {
  direction: 'LR' | 'TB';
  nodeSep: number;
  rankSep: number;
}

const DEFAULT_CONFIG: LayoutConfig = {
  direction: 'LR',
  nodeSep: 50,
  rankSep: 100,
};

/**
 * 使用 dagre 算法计算层级布局
 * @param nodes 节点列表
 * @param edges 边列表
 * @param config 布局配置
 * @returns 布局后的节点位置和层级映射
 */
export function calculateLayout(
  nodes: Node[],
  edges: Edge[],
  config: LayoutConfig = DEFAULT_CONFIG
): LayoutResult {
  const dagreGraph = new dagre.graphlib.Graph();

  dagreGraph.setGraph({
    rankdir: config.direction,
    nodesep: config.nodeSep,
    ranksep: config.rankSep,
  });

  dagreGraph.setDefaultEdgeLabel(() => ({}));

  // 添加节点
  nodes.forEach((node) => {
    dagreGraph.setNode(node.id, { width: 200, height: 80 });
  });

  // 添加边
  edges.forEach((edge) => {
    dagreGraph.setEdge(edge.source, edge.target);
  });

  // 计算布局
  dagre.layout(dagreGraph);

  // 获取节点位置
  const layoutedNodes = nodes.map((node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    return {
      ...node,
      position: {
        x: nodeWithPosition.x - 100, // 居中对齐（节点宽度 200）
        y: nodeWithPosition.y - 40,  // 居中对齐（节点高度 80）
      },
    };
  });

  // 计算节点层级（用于检测回边）
  const nodeLevels = new Map<string, number>();
  layoutedNodes.forEach((node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    nodeLevels.set(node.id, nodeWithPosition.rank ?? 0);
  });

  return { nodes: layoutedNodes, nodeLevels };
}

/**
 * 检测边是否为回边（指向层级更低或相同的节点）
 */
export function isBackEdge(
  edge: Edge,
  nodeLevels: Map<string, number>
): boolean {
  const sourceLevel = nodeLevels.get(edge.source) ?? 0;
  const targetLevel = nodeLevels.get(edge.target) ?? 0;
  return targetLevel <= sourceLevel;
}

/**
 * 应用边样式，区分正向边和回边
 * @param edges 边列表
 * @param nodeLevels 节点层级映射
 * @returns 样式化的边列表
 */
export function applyEdgeStyles(
  edges: Edge[],
  nodeLevels: Map<string, number>
): Edge[] {
  return edges.map((edge) => {
    if (isBackEdge(edge, nodeLevels)) {
      return {
        ...edge,
        type: 'smoothstep',
        style: { stroke: '#fa8c16', strokeWidth: 2 },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          width: 20,
          height: 20,
          color: '#fa8c16',
        },
      };
    }
    return {
      ...edge,
      type: 'default',
      style: { stroke: 'var(--color-primary)', strokeWidth: 2 },
      markerEnd: {
        type: MarkerType.ArrowClosed,
        width: 20,
        height: 20,
      },
    };
  });
}

/**
 * Hook: 提供自动布局功能
 */
export function useAutoLayout() {
  return {
    calculateLayout,
    isBackEdge,
    applyEdgeStyles,
  };
}