// isdp/web/src/store/debugThread.ts
import { create } from 'zustand';
import { Message, AgentConfig } from '@/types';

interface DebugThreadState {
  threadId: string | null;
  status: 'idle' | 'running' | 'completed' | 'error';
  messages: Message[];
  currentOutput: string;
  invocationId: string | null;
  agentConfig: AgentConfig | null;
  sandboxUrl: string | null;
  projectPath: string;

  // Actions
  setThreadId: (id: string | null) => void;
  setStatus: (status: 'idle' | 'running' | 'completed' | 'error') => void;
  addMessage: (message: Message) => void;
  setMessages: (messages: Message[]) => void;
  appendOutput: (chunk: string) => void;
  clearOutput: () => void;
  setInvocationId: (id: string | null) => void;
  setAgentConfig: (config: AgentConfig | null) => void;
  setSandboxUrl: (url: string | null) => void;
  setProjectPath: (path: string) => void;
  reset: () => void;
}

const initialState = {
  threadId: null,
  status: 'idle' as const,
  messages: [],
  currentOutput: '',
  invocationId: null,
  agentConfig: null,
  sandboxUrl: null,
  projectPath: '',
};

export const useDebugThreadStore = create<DebugThreadState>((set) => ({
  ...initialState,

  setThreadId: (id) => set({ threadId: id }),
  setStatus: (status) => set({ status }),
  addMessage: (message) => set((state) => ({
    messages: [...state.messages, message]
  })),
  setMessages: (messages) => set({ messages }),
  appendOutput: (chunk) => set((state) => ({
    currentOutput: state.currentOutput + chunk
  })),
  clearOutput: () => set({ currentOutput: '' }),
  setInvocationId: (id) => set({ invocationId: id }),
  setAgentConfig: (config) => set({ agentConfig: config }),
  setSandboxUrl: (url) => set({ sandboxUrl: url }),
  setProjectPath: (path) => set({ projectPath: path }),
  reset: () => set(initialState),
}));