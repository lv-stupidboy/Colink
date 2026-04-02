import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { Thread, Message, AgentInvocation, AgentConfig, Phase, AgentRole, Project, WorkflowTemplate, SandboxServer } from '@/types';
import type { TokenUsage, TaskProgress } from '@/types/status';
import api from '@/api/client';

interface AppState {
  // 当前项目
  currentProjectId: string | null;

  // 当前Thread
  currentThread: Thread | null;

  // 消息列表
  messages: Message[];

  // 流式消息状态（简化：只追踪当前活跃的流式输出）
  isStreaming: boolean;
  streamingContent: string;
  streamingAgentId: string | null;
  streamingAgentName: string | null;
  streamingInvocationId: string | null;

  // 当前进度状态（简化：只追踪当前活跃 agent）
  progressStatus: 'thinking' | 'tool_use' | 'generating' | 'idle';
  progressToolName: string | null;
  progressToolInput: Record<string, unknown> | null;

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

  // 当前Agent团队（包含agentIds）
  currentWorkflowTemplate: WorkflowTemplate | null;

  // 项目上下文加载状态
  loadingProjectContext: boolean;

  // 调试模式状态
  isDebugMode: boolean;
  debugAgentId: string | null;
  debugAgentConfig: AgentConfig | null;
  debugProjectPath: string;

  // Solo 模式状态
  soloMode: boolean;

  // 沙箱状态
  sandboxServer: SandboxServer | null;
  sandboxLoading: boolean;
  dockerAvailable: boolean;
  sandboxDrawerVisible: boolean;

  // Agent Usage 和 TaskProgress
  agentUsage: Record<string, TokenUsage>;
  agentTaskProgress: Record<string, TaskProgress>;

  // 已完成的 Agent 历史
  completedAgents: AgentInvocation[];
}

interface AppActions {
  // 设置当前项目
  setCurrentProject: (projectId: string) => void;

  // 加载Thread
  loadThread: (threadId: string) => Promise<void>;

  // 设置当前Thread（用于Solo模式新建对话）
  setCurrentThread: (thread: Thread) => void;

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
  updateAgentStatus: (invocationId: string, status: string, agentName?: string) => void;

  // 设置WebSocket连接状态
  setWsConnected: (connected: boolean) => void;

  // 加载Agent配置
  loadAgentConfigs: () => Promise<void>;

  // 清除错误
  clearError: () => void;

  // 重置状态
  reset: () => void;

  // 清除当前会话消息（用于Solo模式新建对话）
  clearThreadMessages: () => void;

  // 加载项目上下文（项目和Agent团队）
  loadProjectContext: (projectId: string) => Promise<void>;

  // 加载Agent团队（直接根据templateId）
  loadWorkflowTemplate: (templateId: string) => Promise<void>;

  // 清除项目上下文
  clearProjectContext: () => void;

  // 获取过滤后的Agent列表（基于Agent团队）
  getFilteredAgents: () => AgentConfig[];

  // 更新流式消息（实时输出）
  updateStreamingMessage: (invocationId: string, chunk: string, agentId: string, agentName?: string) => void;

  // 完成流式消息（转为正式消息）
  finalizeStreamingMessage: (invocationId: string) => void;

  // 更新进度状态
  updateProgress: (invocationId: string, status: string, toolName?: string, toolInput?: Record<string, unknown>) => void;

  // 清除进度状态
  clearProgress: (invocationId: string) => void;

  // 调试模式 actions
  setDebugMode: (isDebug: boolean, agentId?: string) => void;
  setDebugAgentConfig: (config: AgentConfig | null) => void;
  setDebugProjectPath: (path: string) => void;

  // Solo 模式 actions
  setSoloMode: (soloMode: boolean) => void;

  // 沙箱 actions
  setSandboxServer: (server: SandboxServer | null) => void;
  setSandboxLoading: (loading: boolean) => void;
  setDockerAvailable: (available: boolean) => void;
  setSandboxDrawerVisible: (visible: boolean) => void;

  // Agent Usage 和 TaskProgress actions
  updateAgentUsage: (invocationId: string, usage: TokenUsage) => void;
  updateAgentTaskProgress: (invocationId: string, progress: TaskProgress) => void;
  clearAgentUsage: (invocationId: string) => void;
}

const initialState: AppState = {
  currentProjectId: null,
  currentThread: null,
  messages: [],
  // 简化的流式状态
  isStreaming: false,
  streamingContent: '',
  streamingAgentId: null,
  streamingAgentName: null,
  streamingInvocationId: null,
  // 简化的进度状态
  progressStatus: 'idle',
  progressToolName: null,
  progressToolInput: null,
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
  // Solo 模式状态
  soloMode: false,
  // 沙箱状态
  sandboxServer: null,
  sandboxLoading: false,
  dockerAvailable: false,
  sandboxDrawerVisible: false,
  // Agent Usage 和 TaskProgress
  agentUsage: {},
  agentTaskProgress: {},
  // 已完成的 Agent 历史
  completedAgents: [],
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
        // 重置流式状态
        isStreaming: false,
        streamingContent: '',
        streamingAgentId: null,
        streamingAgentName: null,
        streamingInvocationId: null,
        // 重置进度状态
        progressStatus: 'idle',
        progressToolName: null,
        progressToolInput: null,
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

    setCurrentThread: (thread) => {
      set({ currentThread: thread });
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
      set((state) => {
        // 去重检查：如果消息 ID 已存在，不重复添加
        const exists = state.messages.some(m => m.id === message.id);
        if (exists) {
          return state;
        }

        return {
          messages: [...state.messages, message],
        };
      });
    },

    updateAgentStatus: (invocationId, status, agentName?: string) => {
      set((state) => {
        if (status === 'completed' || status === 'failed' || status === 'cancelled') {
          // 找到完成的 agent 并移到历史列表
          const completedAgent = state.activeAgents.find((a) => a.id === invocationId);
          const newCompletedAgents = completedAgent
            ? [
                ...state.completedAgents.filter((a) => a.id !== invocationId),
                {
                  ...completedAgent,
                  status: status as 'completed' | 'failed' | 'cancelled',
                  completedAt: new Date().toISOString(),
                },
              ]
            : state.completedAgents;

          return {
            activeAgents: state.activeAgents.filter((a) => a.id !== invocationId),
            completedAgents: newCompletedAgents,
          };
        }
        // Agent 启动时添加到 activeAgents
        if (status === 'started' || status === 'running') {
          const exists = state.activeAgents.some((a) => a.id === invocationId);
          if (!exists) {
            return {
              activeAgents: [
                ...state.activeAgents,
                {
                  id: invocationId,
                  status: 'running',
                  agentName: agentName,
                  startedAt: new Date().toISOString(),
                } as AgentInvocation,
              ],
            };
          }
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

    clearThreadMessages: () => {
      set({
        messages: [],
        // 重置流式状态
        isStreaming: false,
        streamingContent: '',
        streamingAgentId: null,
        streamingAgentName: null,
        streamingInvocationId: null,
        // 重置进度状态
        progressStatus: 'idle',
        progressToolName: null,
        progressToolInput: null,
        currentThread: null,
        activeAgents: [],
      });
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
        // 追加流式内容
        const newContent = state.streamingContent + chunk;
        return {
          isStreaming: true,
          streamingContent: newContent,
          streamingAgentId: agentId,
          streamingAgentName: agentName || state.streamingAgentName,
          streamingInvocationId: invocationId,
        };
      });
    },

    finalizeStreamingMessage: (invocationId) => {
      set((state) => {
        // 如果没有流式内容，直接返回
        if (!state.streamingContent || state.streamingInvocationId !== invocationId) {
          return {
            isStreaming: false,
            streamingContent: '',
            streamingAgentId: null,
            streamingAgentName: null,
            streamingInvocationId: null,
            progressStatus: 'idle',
            progressToolName: null,
            progressToolInput: null,
          };
        }

        // 去重检查：如果消息已存在，只清理流式缓存
        const messageId = `agent-${invocationId}`;
        const exists = state.messages.some(m => m.id === messageId);
        if (exists) {
          return {
            isStreaming: false,
            streamingContent: '',
            streamingAgentId: null,
            streamingAgentName: null,
            streamingInvocationId: null,
            progressStatus: 'idle',
            progressToolName: null,
            progressToolInput: null,
          };
        }

        // Create the final message from streaming content
        const finalMessage: Message = {
          id: messageId,
          threadId: state.currentThread?.id || '',
          role: 'agent',
          agentId: state.streamingAgentId || '',
          content: state.streamingContent,
          messageType: 'text',
          metadata: {
            agentName: state.streamingAgentName,
          },
          createdAt: new Date().toISOString(),
        };

        return {
          isStreaming: false,
          streamingContent: '',
          streamingAgentId: null,
          streamingAgentName: null,
          streamingInvocationId: null,
          progressStatus: 'idle',
          progressToolName: null,
          progressToolInput: null,
          messages: [...state.messages, finalMessage],
        };
      });
    },

    updateProgress: (_invocationId, status, toolName, toolInput) => {
      set({
        progressStatus: status as 'thinking' | 'tool_use' | 'generating' | 'idle',
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

    // Solo 模式 actions
    setSoloMode: (soloMode) => {
      set({ soloMode });
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

    // Agent Usage 和 TaskProgress actions
    updateAgentUsage: (invocationId, usage) => {
      set((state) => ({
        agentUsage: {
          ...state.agentUsage,
          [invocationId]: {
            ...state.agentUsage[invocationId],
            ...usage,
          },
        },
      }));
    },

    updateAgentTaskProgress: (invocationId, progress) => {
      set((state) => ({
        agentTaskProgress: {
          ...state.agentTaskProgress,
          [invocationId]: progress,
        },
      }));
    },

    clearAgentUsage: (invocationId) => {
      set((state) => {
        const { [invocationId]: _, ...remainingUsage } = state.agentUsage;
        const { [invocationId]: __, ...remainingProgress } = state.agentTaskProgress;
        return {
          agentUsage: remainingUsage,
          agentTaskProgress: remainingProgress,
        };
      });
    },
  }))
);

export default useAppStore;
