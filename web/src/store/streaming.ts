import { create } from 'zustand';
import type { MessageContentBlock } from '@/types';

export type ProgressStatus = 'thinking' | 'tool_use' | 'generating' | 'idle';

export interface StreamingState {
  isStreaming: boolean;
  streamingInvocationId: string | null;
  streamingAgentId: string | null;
  streamingAgentName: string | null;
  streamingContentBlocks: MessageContentBlock[];
  progressStatus: ProgressStatus;
  progressToolName: string | null;
  progressToolInput: Record<string, unknown> | null;
}

export interface StreamingActions {
  startStreaming: (invocationId: string, agentId: string, agentName: string) => void;
  stopStreaming: () => void;
  updateStreamingMessage: (invocationId: string, chunk: string, agentId: string, agentName?: string) => void;
  finalizeStreamingMessage: (invocationId: string) => MessageContentBlock[] | null;
  appendContentBlock: (invocationId: string, block: MessageContentBlock) => void;
  updateContentBlock: (invocationId: string, blockId: string, update: Partial<MessageContentBlock>) => void;
  updateProgress: (invocationId: string, status: string, toolName?: string, toolInput?: Record<string, unknown>) => void;
  clearProgress: (invocationId: string) => void;
  recoverInvocationState: (invocationId: string, contentBlocks: MessageContentBlock[], status: string, agentId?: string, agentName?: string) => void;
  clearStreaming: () => void;
}

const initialState: StreamingState = {
  isStreaming: false,
  streamingInvocationId: null,
  streamingAgentId: null,
  streamingAgentName: null,
  streamingContentBlocks: [],
  progressStatus: 'idle',
  progressToolName: null,
  progressToolInput: null,
};

export const useStreamingStore = create<StreamingState & StreamingActions>((set, get) => ({
  ...initialState,

  startStreaming: (invocationId, agentId, agentName) => {
    set({
      isStreaming: true,
      streamingInvocationId: invocationId,
      streamingAgentId: agentId,
      streamingAgentName: agentName,
      streamingContentBlocks: [],
      progressStatus: 'generating',
    });
  },

  stopStreaming: () => {
    set({
      isStreaming: false,
      progressStatus: 'idle',
      progressToolName: null,
      progressToolInput: null,
    });
  },

  updateStreamingMessage: (invocationId, chunk, agentId, agentName) => {
    set((state) => {
      if (state.streamingInvocationId && state.streamingInvocationId !== invocationId) {
        return {};
      }

      const blocks = state.streamingContentBlocks;
      const lastBlock = blocks.length > 0 ? blocks[blocks.length - 1] : null;

      if (lastBlock && lastBlock.type === 'text') {
        const updatedBlocks = [...blocks];
        updatedBlocks[updatedBlocks.length - 1] = {
          ...lastBlock,
          content: lastBlock.content + chunk,
        };
        return {
          isStreaming: true,
          streamingAgentId: agentId,
          streamingAgentName: agentName || state.streamingAgentName,
          streamingInvocationId: invocationId,
          streamingContentBlocks: updatedBlocks,
        };
      }

      return {
        isStreaming: true,
        streamingAgentId: agentId,
        streamingAgentName: agentName || state.streamingAgentName,
        streamingInvocationId: invocationId,
        streamingContentBlocks: [...blocks, {
          id: `text-${invocationId}-${Date.now()}`,
          type: 'text',
          content: chunk,
          timestamp: Date.now(),
        }],
      };
    });
  },

  finalizeStreamingMessage: (invocationId) => {
    const state = get();
    if (state.streamingInvocationId !== invocationId) {
      return null;
    }

    const contentBlocks = [...state.streamingContentBlocks];

    set({
      isStreaming: false,
      streamingAgentId: null,
      streamingAgentName: null,
      streamingInvocationId: null,
      streamingContentBlocks: [],
      progressStatus: 'idle',
      progressToolName: null,
      progressToolInput: null,
    });

    return contentBlocks;
  },

  appendContentBlock: (invocationId, block) => {
    set((state) => {
      const isQuestionBlock = block.type === 'question';

      if (!isQuestionBlock && state.streamingInvocationId && state.streamingInvocationId !== invocationId) {
        return {};
      }

      const blocks = state.streamingContentBlocks;

      if (block.type === 'thinking') {
        const existingThinkingIndex = blocks.findIndex(
          (b) => b.id === block.id && b.type === 'thinking'
        );
        if (existingThinkingIndex >= 0) {
          const existingBlock = blocks[existingThinkingIndex] as typeof block;
          const updatedBlocks = [...blocks];
          updatedBlocks[existingThinkingIndex] = {
            ...existingBlock,
            content: existingBlock.content + block.content,
            status: block.status === 'success' ? 'success' : existingBlock.status,
          } as MessageContentBlock;
          return {
            isStreaming: true,
            streamingInvocationId: invocationId,
            streamingContentBlocks: updatedBlocks,
          };
        }
      }

      const existingIndex = blocks.findIndex((b) => b.id === block.id);
      if (existingIndex >= 0) {
        const existingBlock = blocks[existingIndex];
        const existingStatus = (existingBlock as any).status;
        const newStatus = (block as any).status;
        const shouldPreserveStatus = existingStatus === 'success' || existingStatus === 'failed';
        const finalStatus = shouldPreserveStatus ? existingStatus : newStatus;

        const updatedBlocks = [...blocks];
        updatedBlocks[existingIndex] = {
          ...existingBlock,
          ...block,
          status: finalStatus,
        } as MessageContentBlock;
        return {
          isStreaming: true,
          streamingInvocationId: invocationId,
          streamingContentBlocks: updatedBlocks,
        };
      }

      return {
        isStreaming: true,
        streamingInvocationId: invocationId,
        streamingContentBlocks: [...blocks, block],
      };
    });
  },

  updateContentBlock: (invocationId, blockId, update) => {
    set((state) => {
      const targetBlock = state.streamingContentBlocks.find(b => b.id === blockId);
      const isQuestionBlock = targetBlock && targetBlock.type === 'question';

      if (!isQuestionBlock && state.streamingInvocationId && state.streamingInvocationId !== invocationId) {
        return {};
      }

      const updatedBlocks = state.streamingContentBlocks.map((block) =>
        block.id === blockId ? { ...block, ...update } as MessageContentBlock : block
      );
      return { streamingContentBlocks: updatedBlocks };
    });
  },

  updateProgress: (_invocationId, status, toolName, toolInput) => {
    set({
      progressStatus: status as ProgressStatus,
      progressToolName: toolName || null,
      progressToolInput: toolInput || null,
    });
  },

  clearProgress: (_invocationId) => {
    set({
      progressStatus: 'idle',
      progressToolName: null,
      progressToolInput: null,
    });
  },

  recoverInvocationState: (invocationId, contentBlocks, status, agentId, agentName) => {
    set((state) => {
      if (status === 'running') {
        const existingIds = new Set(state.streamingContentBlocks.map((b) => b.id));
        const newBlocks = contentBlocks.filter((b) => !existingIds.has(b.id));
        const mergedBlocks = [...state.streamingContentBlocks, ...newBlocks];

        return {
          isStreaming: true,
          streamingInvocationId: invocationId,
          streamingAgentId: agentId || null,
          streamingAgentName: agentName || null,
          streamingContentBlocks: mergedBlocks,
        };
      }
      return state;
    });
  },

  clearStreaming: () => {
    set(initialState);
  },
}));
