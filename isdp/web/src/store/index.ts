import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { Thread, Message, AgentInvocation, AgentConfig, Phase, AgentRole, Project, WorkflowTemplate, SandboxServer } from '@/types';
import api from '@/api/client';

interface AppState {
  // 当前项目
  currentProjectId: string | null;

  // 当前Thread
  currentThread: Thread | null;

  // 消息列表
  messages: Message[];

  // 流式消息缓存（key: invocationId, value: 正在生成的消息）
  streamingMessages: Record<string, { content: string; agentId: string; agentName?: string }>;

  // 运行中的Agent
  activeAgents: AgentInvocation[];

  // Agent配置列表
  agentConfigs: AgentConfig[];

  // WebSocket连接状态
  wsConnected: boolean;

  // 加载状态
  loading: boolean;

  // 错误信息
  error: string | null;

  // 当前项目（包含workflowTemplateId）
  currentProject: Project | null;

  // 当前工作流模板（包含agentIds）
  currentWorkflowTemplate: WorkflowTemplate | null;

  // 项目上下文加载状态
  loadingProjectContext: boolean;

  // 调试模式状态
  isDebugMode: boolean;
  debugAgentId: string | null;
  debugAgentConfig: AgentConfig | null;
  debugProjectPath: string;

  // 沙箱状态
  sandboxServer: SandboxServer | null;
  sandboxLoading: boolean;
  dockerAvailable: boolean;
  sandboxDrawerVisible: boolean;
}

interface AppActions {
  // 设置当前项目
  setCurrentProject: (projectId: string) => void;

  // 加载Thread
  loadThread: (threadId: string) => Promise<void>;

  // 发送消息
  sendMessage: (content: string, skipAgentTrigger?: boolean) => Promise<void>;

  // 触发Agent
  spawnAgent: (role: AgentRole, input: string, configId?: string) => Promise<void>;

  // 取消Agent
  cancelAgent: (invocationId: string) => Promise<void>;

  // 更新Thread阶段
  updatePhase: (phase: Phase) => Promise<void>;

  // 添加消息（WebSocket推送）
  addMessage: (message: Message) => void;

  // 更新Agent状态
  updateAgentStatus: (invocationId: string, status: string) => void;

  // 设置WebSocket连接状态
  setWsConnected: (connected: boolean) => void;

  // 加载Agent配置
  loadAgentConfigs: () => Promise<void>;

  // 清除错误
  clearError: () => void;

  // 重置状态
  reset: () => void;

  // 加载项目上下文（项目和工作流模板）
  loadProjectContext: (projectId: string) => Promise<void>;

  // 加载工作流模板（直接根据templateId）
  loadWorkflowTemplate: (templateId: string) => Promise<void>;

  // 清除项目上下文
  clearProjectContext: () => void;

  // 获取过滤后的Agent列表（基于工作流模板）
  getFilteredAgents: () => AgentConfig[];

  // 更新流式消息（实时输出）
  updateStreamingMessage: (invocationId: string, chunk: string, agentId: string, agentName?: string) => void;

  // 完成流式消息（转为正式消息）
  finalizeStreamingMessage: (invocationId: string) => void;

  // 调试模式 actions
  setDebugMode: (isDebug: boolean, agentId?: string) => void;
  setDebugAgentConfig: (config: AgentConfig | null) => void;
  setDebugProjectPath: (path: string) => void;

  // 沙箱 actions
  setSandboxServer: (server: SandboxServer | null) => void;
  setSandboxLoading: (loading: boolean) => void;
  setDockerAvailable: (available: boolean) => void;
  setSandboxDrawerVisible: (visible: boolean) => void;
}

const initialState: AppState = {
  currentProjectId: null,
  currentThread: null,
  messages: [],
  streamingMessages: {},
  activeAgents: [],
  agentConfigs: [],
  wsConnected: false,
  loading: false,
  error: null,
  currentProject: null,
  currentWorkflowTemplate: null,
  loadingProjectContext: false,
  // 调试模式状态
  isDebugMode: false,
  debugAgentId: null,
  debugAgentConfig: null,
  debugProjectPath: '',
  // 沙箱状态
  sandboxServer: null,
  sandboxLoading: false,
  dockerAvailable: false,
  sandboxDrawerVisible: false,
};

export const useAppStore = create<AppState & AppActions>()(
  subscribeWithSelector((set, get) => ({
    ...initialState,

    setCurrentProject: (projectId) => {
      set({ currentProjectId: projectId });
    },

    loadThread: async (threadId) => {
      // 先清空旧状态，防止显示其他Thread的消息
      set({
        loading: true,
        error: null,
        messages: [],
        streamingMessages: {},
        currentThread: null,
        activeAgents: [],
      });

      try {
        const [thread, messages, invocations] = await Promise.all([
          api.threads.get(threadId),
          api.messages.list(threadId),
          api.invocations.list(threadId),
        ]);

        set({
          currentThread: thread,
          messages: messages || [],
          activeAgents: (invocations || []).filter((i: AgentInvocation) => i.status === 'running'),
        });
      } catch (error) {
        set({ error: (error as Error).message });
      } finally {
        set({ loading: false });
      }
    },

    sendMessage: async (content, skipAgentTrigger = false) => {
      const { currentThread } = get();
      if (!currentThread) return;

      // 先创建本地消息（乐观更新）
      const userMessage: Message = {
        id: `user-${Date.now()}`,
        threadId: currentThread.id,
        role: 'user',
        content,
        messageType: 'text',
        createdAt: new Date().toISOString(),
      };

      // 使用函数式更新，避免竞态条件
      set((state) => ({
        messages: [...state.messages, userMessage]
      }));

      try {
        await api.messages.create(currentThread.id, content, skipAgentTrigger);
      } catch (error) {
        set({ error: (error as Error).message });
      }
    },

    spawnAgent: async (role, input, configId) => {
      const { currentThread } = get();
      if (!currentThread) return;

      try {
        await api.invocations.spawn(currentThread.id, role, input, configId);
      } catch (error) {
        set({ error: (error as Error).message });
      }
    },

    cancelAgent: async (invocationId) => {
      try {
        await api.invocations.cancel(invocationId);
        set((state) => ({
          activeAgents: state.activeAgents.filter((a) => a.id !== invocationId),
        }));
      } catch (error) {
        set({ error: (error as Error).message });
      }
    },

    updatePhase: async (phase) => {
      const { currentThread } = get();
      if (!currentThread) return;

      try {
        const updated = await api.threads.setPhase(currentThread.id, phase, '');
        set({ currentThread: updated });
      } catch (error) {
        set({ error: (error as Error).message });
      }
    },

    addMessage: (message) => {
      set((state) => ({
        messages: [...state.messages, message],
      }));
    },

    updateAgentStatus: (invocationId, status) => {
      set((state) => {
        if (status === 'completed' || status === 'failed' || status === 'cancelled') {
          return {
            activeAgents: state.activeAgents.filter((a) => a.id !== invocationId),
          };
        }
        return state;
      });
    },

    setWsConnected: (connected) => {
      set({ wsConnected: connected });
    },

    loadAgentConfigs: async () => {
      try {
        const configs = await api.agents.list();
        set({ agentConfigs: configs });
      } catch (error) {
        console.error('Failed to load agent configs:', error);
      }
    },

    clearError: () => {
      set({ error: null });
    },

    reset: () => {
      set(initialState);
    },

    loadProjectContext: async (projectId: string) => {
      set({ loadingProjectContext: true });
      try {
        // Load project to get workflowTemplateId
        const project = await api.projects.get(projectId);

        // Load workflow template if project has one bound
        let workflowTemplate: WorkflowTemplate | null = null;
        if ((project as unknown as Project).workflowTemplateId) {
          workflowTemplate = await api.workflows.get((project as unknown as Project).workflowTemplateId!);
        }

        set({
          currentProject: project as unknown as Project,
          currentWorkflowTemplate: workflowTemplate,
          loadingProjectContext: false,
        });
      } catch (error) {
        console.error('Failed to load project context:', error);
        set({
          loadingProjectContext: false,
          currentProject: null,
          currentWorkflowTemplate: null,
        });
      }
    },

    loadWorkflowTemplate: async (templateId: string) => {
      set({ loadingProjectContext: true });
      try {
        const workflowTemplate = await api.workflows.get(templateId);
        set({
          currentWorkflowTemplate: workflowTemplate,
          loadingProjectContext: false,
        });
      } catch (error) {
        console.error('Failed to load workflow template:', error);
        set({
          loadingProjectContext: false,
          currentWorkflowTemplate: null,
        });
      }
    },

    clearProjectContext: () => {
      set({
        currentProject: null,
        currentWorkflowTemplate: null,
      });
    },

    getFilteredAgents: () => {
      const { currentWorkflowTemplate, agentConfigs } = get();

      // If no workflow template or no agentIds, return all agents
      if (!currentWorkflowTemplate || !currentWorkflowTemplate.agentIds?.length) {
        return agentConfigs;
      }

      // Filter agents that are in the workflow's agentIds
      return agentConfigs.filter(agent =>
        currentWorkflowTemplate.agentIds.includes(agent.id)
      );
    },

    updateStreamingMessage: (invocationId, chunk, agentId, agentName) => {
      set((state) => {
        const existing = state.streamingMessages[invocationId];
        return {
          streamingMessages: {
            ...state.streamingMessages,
            [invocationId]: {
              content: (existing?.content || '') + chunk,
              agentId,
              agentName: agentName || existing?.agentName,
            },
          },
        };
      });
    },

    finalizeStreamingMessage: (invocationId) => {
      set((state) => {
        const streamingMsg = state.streamingMessages[invocationId];
        if (!streamingMsg) return state;

        // Create the final message from streaming content
        const finalMessage: Message = {
          id: `agent-${invocationId}`,
          threadId: state.currentThread?.id || '',
          role: 'agent',
          agentId: streamingMsg.agentId,
          content: streamingMsg.content,
          messageType: 'text',
          metadata: {
            agentName: streamingMsg.agentName,
          },
          createdAt: new Date().toISOString(),
        };

        // Remove from streaming and add to messages
        const { [invocationId]: _, ...remainingStreaming } = state.streamingMessages;
        return {
          streamingMessages: remainingStreaming,
          messages: [...state.messages, finalMessage],
        };
      });
    },

    // 调试模式 actions
    setDebugMode: (isDebug, agentId) => {
      set({
        isDebugMode: isDebug,
        debugAgentId: agentId || null,
        debugAgentConfig: null,
        debugProjectPath: '', // 每次进入调试模式时清空工作目录
      });
    },

    setDebugAgentConfig: (config) => {
      set({ debugAgentConfig: config });
    },

    setDebugProjectPath: (path) => {
      set({ debugProjectPath: path });
    },

    // 沙箱 actions
    setSandboxServer: (server) => {
      set({ sandboxServer: server });
    },

    setSandboxLoading: (loading) => {
      set({ sandboxLoading: loading });
    },

    setDockerAvailable: (available) => {
      set({ dockerAvailable: available });
    },

    setSandboxDrawerVisible: (visible) => {
      set({ sandboxDrawerVisible: visible });
    },
  }))
);

export default useAppStore;
