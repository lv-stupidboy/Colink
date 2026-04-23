// web/src/pages/Workflow/TeamGraphEditor/useGraphStore.ts
import { create } from 'zustand';
import type { Node, Edge } from '@xyflow/react';
import type { AgentConfig, Transition } from '@/types';
import api from '@/api/client';
import { calculateLayout, applyEdgeStyles } from './useAutoLayout';

interface GraphState {
  // Mode
  mode: 'preview' | 'edit';

  // Data
  teamId: string;
  teamName: string;
  nodes: Node[];
  edges: Edge[];
  allAgents: AgentConfig[];

  // Selection
  selectedNode: string | null;
  selectedEdge: string | null;

  // Status tracking
  hasChanges: boolean;
  loading: boolean;
  saving: boolean;
  error: string | null;
}

interface GraphActions {
  setMode: (mode: 'preview' | 'edit') => void;
  setSelectedNode: (nodeId: string | null) => void;
  setSelectedEdge: (edgeId: string | null) => void;
  addNode: (agent: AgentConfig, position?: { x: number; y: number }) => void;
  removeNode: (nodeId: string) => void;
  addEdge: (sourceId: string, targetId: string) => void;
  removeEdge: (edgeId: string) => void;
  updateEdgeTriggerHint: (edgeId: string, triggerHint: string) => void;
  setNodes: (nodes: Node[] | ((nodes: Node[]) => Node[])) => void;
  setEdges: (edges: Edge[] | ((edges: Edge[]) => Edge[])) => void;
  setHasChanges: (hasChanges: boolean) => void;
  setError: (error: string | null) => void;
  loadData: (teamId: string) => Promise<void>;
  saveChanges: () => Promise<void>;
  refreshAgent: (agentId: string) => Promise<void>;
  relayout: () => void;
  reset: () => void;
}

type GraphStoreState = GraphState & GraphActions;

const initialState: GraphState = {
  mode: 'preview',
  teamId: '',
  teamName: '',
  nodes: [],
  edges: [],
  allAgents: [],
  selectedNode: null,
  selectedEdge: null,
  hasChanges: false,
  loading: false,
  saving: false,
  error: null,
};

export const useGraphStore = create<GraphStoreState>((set, get) => ({
  ...initialState,

  setMode: (mode) => set({ mode }),

  setSelectedNode: (nodeId) => set({ selectedNode: nodeId, selectedEdge: null }),
  setSelectedEdge: (edgeId) => set({ selectedEdge: edgeId, selectedNode: null }),

  setError: (error) => set({ error }),

  addNode: (agent, position) => {
    const nodes = get().nodes;
    const newNode: Node = {
      id: agent.id,
      type: 'agentNode',
      position: position || { x: 100 + nodes.length * 150, y: 100 },
      data: { agent },
    };
    set({ nodes: [...nodes, newNode], hasChanges: true, error: null });
  },

  removeNode: (nodeId) => {
    const nodes = get().nodes.filter(n => n.id !== nodeId);
    const edges = get().edges.filter(e => e.source !== nodeId && e.target !== nodeId);
    set({ nodes, edges, hasChanges: true, selectedNode: null, error: null });
  },

  addEdge: (sourceId, targetId) => {
    const edges = get().edges;
    const existingEdge = edges.find(
      e => (e.source === sourceId && e.target === targetId) ||
           (e.source === targetId && e.target === sourceId)
    );

    if (existingEdge) {
      set({ error: '该 Agent 之间已存在连线，无需重复添加' });
      return;
    }

    const newEdge: Edge = {
      id: `${sourceId}-${targetId}`,
      source: sourceId,
      target: targetId,
      data: { triggerHint: '' },
    };

    // 添加边后触发 relayout 以应用正确的边样式
    set({ edges: [...edges, newEdge], hasChanges: true, error: null });
    get().relayout();
  },

  removeEdge: (edgeId) => {
    const edges = get().edges.filter(e => e.id !== edgeId);
    set({ edges, hasChanges: true, selectedEdge: null, error: null });
  },

  updateEdgeTriggerHint: (edgeId, triggerHint) => {
    const edges = get().edges.map(e =>
      e.id === edgeId ? { ...e, data: { ...e.data, triggerHint } } : e
    );
    set({ edges, hasChanges: true, error: null });
  },

  setNodes: (nodes) => set((state) => ({
    nodes: typeof nodes === 'function' ? nodes(state.nodes) : nodes
  })),
  setEdges: (edges) => set((state) => ({
    edges: typeof edges === 'function' ? edges(state.edges) : edges
  })),
  setHasChanges: (hasChanges) => set({ hasChanges }),

  relayout: () => {
    const { nodes, edges } = get();
    if (nodes.length === 0) return;

    const { nodes: layoutedNodes, nodeLevels } = calculateLayout(nodes, edges);
    const styledEdges = applyEdgeStyles(edges, nodeLevels);

    set({ nodes: layoutedNodes, edges: styledEdges });
  },

  loadData: async (teamId) => {
    set({ loading: true, teamId, error: null });
    try {
      const [workflow, agents] = await Promise.all([
        api.workflows.get(teamId),
        api.agents.list(),
      ]);

      // 先设置原始数据
      const rawNodes = (workflow.agentIds || []).map((agentId: string, index: number) => {
        const agent = agents.find((a: AgentConfig) => a.id === agentId);
        return {
          id: agentId,
          type: 'agentNode',
          position: { x: 100 + index * 150, y: 100 },
          data: { agent: agent || { id: agentId, name: 'Unknown' } },
        };
      });

      const rawEdges = (workflow.transitions || []).map((t: Transition) => ({
        id: `${t.fromAgentId}-${t.toAgentId}`,
        source: t.fromAgentId,
        target: t.toAgentId,
        data: { triggerHint: t.triggerHint || '' },
      }));

      // 计算布局并应用边样式
      const { nodes: layoutedNodes, nodeLevels } = calculateLayout(rawNodes, rawEdges);
      const styledEdges = applyEdgeStyles(rawEdges, nodeLevels);

      set({
        teamName: workflow.name || '',
        nodes: layoutedNodes,
        edges: styledEdges,
        allAgents: agents || [],
        hasChanges: false,
        error: null,
      });
    } catch (error) {
      const errorMsg = error instanceof Error ? error.message : '加载团队数据失败';
      set({ error: errorMsg });
      console.error('Failed to load data:', error);
    } finally {
      set({ loading: false });
    }
  },

  saveChanges: async () => {
    const { teamId, nodes, edges } = get();
    set({ saving: true, error: null });
    try {
      const agentIds = nodes.map(n => n.id);
      const transitions: Transition[] = edges.map(e => ({
        fromAgentId: e.source,
        toAgentId: e.target,
        type: 'sequence' as const,
        triggerHint: (e.data?.triggerHint as string) || '',
      }));

      await api.workflows.update(teamId, { agentIds, transitions });

      set({ hasChanges: false, error: null });
    } catch (error) {
      const errorMsg = error instanceof Error ? error.message : '保存失败';
      set({ error: errorMsg });
      throw error;
    } finally {
      set({ saving: false });
    }
  },

  refreshAgent: async (agentId) => {
    try {
      const updatedAgent = await api.agents.get(agentId);
      const { nodes, allAgents } = get();

      // Update node data
      const updatedNodes = nodes.map(n =>
        n.id === agentId
          ? { ...n, data: { agent: updatedAgent } }
          : n
      );

      // Update allAgents list
      const updatedAllAgents = allAgents.map(a =>
        a.id === agentId ? updatedAgent : a
      );

      set({
        nodes: updatedNodes,
        allAgents: updatedAllAgents,
        error: null,
      });
    } catch (error) {
      console.error('Failed to refresh agent:', error);
    }
  },

  reset: () => set(initialState),
}));