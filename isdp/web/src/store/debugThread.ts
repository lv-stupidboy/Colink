// isdp/web/src/store/debugThread.ts
import { create } from 'zustand';
import { Message, AgentConfig, SandboxServer } from '@/types';
import type { FileChange } from '@/types/content';

interface DebugThreadState {
  threadId: string | null;
  status: 'idle' | 'running' | 'completed' | 'error';
  messages: Message[];
  currentOutput: string;
  streamingContent: string;
  invocationId: string | null;
  agentConfig: AgentConfig | null;
  sandboxUrl: string | null;
  projectPath: string;
  // 沙箱状态（调试模式独立）
  sandboxServer: SandboxServer | null;
  sandboxLoading: boolean;

  // 代码面板状态
  codePanelOpen: boolean;
  codePanelCollapsed: boolean;
  expandedFiles: Set<string>;
  codeFiles: FileChange[];

  // Actions
  setThreadId: (id: string | null) => void;
  setStatus: (status: 'idle' | 'running' | 'completed' | 'error') => void;
  addMessage: (message: Message) => void;
  setMessages: (messages: Message[]) => void;
  appendOutput: (chunk: string) => void;
  clearOutput: () => void;
  appendStreamChunk: (chunk: string) => void;
  clearStreamContent: () => void;
  setInvocationId: (id: string | null) => void;
  setAgentConfig: (config: AgentConfig | null) => void;
  setSandboxUrl: (url: string | null) => void;
  setProjectPath: (path: string) => void;
  // 沙箱 Actions
  setSandboxServer: (server: SandboxServer | null) => void;
  setSandboxLoading: (loading: boolean) => void;
  // 代码面板 Actions
  openCodePanel: (files: FileChange[]) => void;
  closeCodePanel: () => void;
  toggleCodePanelCollapse: () => void;
  toggleFileExpand: (fileId: string) => void;
  clearCodeFiles: () => void;
  reset: () => void;
  clearAll: () => void;
}

const initialState = {
  threadId: null,
  status: 'idle' as const,
  messages: [],
  currentOutput: '',
  streamingContent: '',
  invocationId: null,
  agentConfig: null,
  sandboxUrl: null,
  projectPath: '',
  // 沙箱状态
  sandboxServer: null,
  sandboxLoading: false,
  // 代码面板状态
  codePanelOpen: false,
  codePanelCollapsed: false,
  expandedFiles: new Set<string>(),
  codeFiles: [] as FileChange[],
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
  appendStreamChunk: (chunk) => set((state) => ({
    streamingContent: state.streamingContent + chunk
  })),
  clearStreamContent: () => set({ streamingContent: '' }),
  setInvocationId: (id) => set({ invocationId: id }),
  setAgentConfig: (config) => set({ agentConfig: config }),
  setSandboxUrl: (url) => set({ sandboxUrl: url }),
  setProjectPath: (path) => set({ projectPath: path }),
  // 沙箱 Actions
  setSandboxServer: (server) => set({ sandboxServer: server }),
  setSandboxLoading: (loading) => set({ sandboxLoading: loading }),
  // 代码面板 Actions
  openCodePanel: (files) => set({
    codePanelOpen: true,
    codePanelCollapsed: false,
    codeFiles: files,
  }),
  closeCodePanel: () => set({
    codePanelOpen: false,
  }),
  toggleCodePanelCollapse: () => set((state) => ({
    codePanelCollapsed: !state.codePanelCollapsed,
  })),
  toggleFileExpand: (fileId) => set((state) => {
    const newExpanded = new Set(state.expandedFiles);
    if (newExpanded.has(fileId)) {
      newExpanded.delete(fileId);
    } else {
      newExpanded.add(fileId);
    }
    return { expandedFiles: newExpanded };
  }),
  clearCodeFiles: () => set({
    codeFiles: [],
    expandedFiles: new Set(),
  }),
  reset: () => set(initialState),
  clearAll: () => set(initialState),
}));