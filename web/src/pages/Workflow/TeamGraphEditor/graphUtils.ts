// web/src/pages/Workflow/TeamGraphEditor/graphUtils.ts
import type { Node, Edge } from '@xyflow/react';
import type { AgentConfig, Transition, WorkflowTemplate } from '@/types';

// 布局配置
export const LAYOUT_CONFIG = {
  direction: 'LR' as const,   // Left-to-Right
  nodeSep: 50,                // 节点间距
  rankSep: 100,               // 层级间距
};

// Convert WorkflowTemplate to React Flow data
export function toFlowData(
  workflow: WorkflowTemplate,
  agents: AgentConfig[]
): { nodes: Node[]; edges: Edge[] } {
  const agentMap = new Map(agents.map(a => [a.id, a]));

  const nodes: Node[] = workflow.agentIds.map((agentId, index) => {
    const agent = agentMap.get(agentId);
    return {
      id: agentId,
      type: 'agentNode',
      position: { x: 100 + index * 150, y: 100 },
      data: {
        agent: agent || { id: agentId, name: 'Unknown Agent', role: 'agent' } as AgentConfig
      },
    };
  });

  const edges: Edge[] = workflow.transitions.map(t => ({
    id: `${t.fromAgentId}-${t.toAgentId}`,
    source: t.fromAgentId,
    target: t.toAgentId,
    type: 'default',
    animated: false,
    data: { triggerHint: t.triggerHint || '' },
  }));

  return { nodes, edges };
}

// Convert React Flow data to WorkflowTemplate format
export function toWorkflowData(
  nodes: Node[],
  edges: Edge[]
): { agentIds: string[]; transitions: Transition[] } {
  const agentIds = nodes.map(n => n.id);

  const transitions: Transition[] = edges.map(e => ({
    fromAgentId: e.source,
    toAgentId: e.target,
    type: 'sequence' as const,
    triggerHint: (e.data?.triggerHint as string) || '',
  }));

  return { agentIds, transitions };
}