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
    } catch (error) {
      console.error('Failed to save agent order:', error);
    } finally {
      setIsSaving(false);
      setDragState({ draggingIndex: null, dragOverIndex: null });
    }
  }, [dragState.draggingIndex, dragState.dragOverIndex, agents, onSave]);

  return {
    dragState,
    isSaving,
    handleDragStart,
    handleDragOver,
    handleDragEnd,
  };
};

export default useAgentDragSort;