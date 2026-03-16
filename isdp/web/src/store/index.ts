import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { Thread, Message, AgentInvocation, AgentConfig, Phase, AgentRole, Project, WorkflowTemplate } from '@/types';
import api from '@/api/client';

interface AppState {
  // 当前项目
  currentProjectId: string | null;

  // 当前Thread
  currentThread: Thread | null;

  // 消息列表
  messages: Message[];

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
}

interface AppActions {
  // 设置当前项目
  setCurrentProject: (projectId: string) => void;

  // 加载Thread
  loadThread: (threadId: string) => Promise<void>;

  // 发送消息
  sendMessage: (content: string) => Promise<void>;

  // 触发Agent
  spawnAgent: (role: AgentRole, input: string) => Promise<void>;

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

  // 清除项目上下文
  clearProjectContext: () => void;

  // 获取过滤后的Agent列表（基于工作流模板）
  getFilteredAgents: () => AgentConfig[];
}

const initialState: AppState = {
  currentProjectId: null,
  currentThread: null,
  messages: [],
  activeAgents: [],
  agentConfigs: [],
  wsConnected: false,
  loading: false,
  error: null,
  currentProject: null,
  currentWorkflowTemplate: null,
  loadingProjectContext: false,
};

export const useAppStore = create<AppState & AppActions>()(
  subscribeWithSelector((set, get) => ({
    ...initialState,

    setCurrentProject: (projectId) => {
      set({ currentProjectId: projectId });
    },

    loadThread: async (threadId) => {
      set({ loading: true, error: null });
      try {
        const [thread, messages, invocations] = await Promise.all([
          api.threads.get(threadId),
          api.messages.list(threadId),
          api.invocations.list(threadId),
        ]);

        let initialMessages: Message[] = messages || [];

        // 如果是新创建的 Thread（没有消息），自动触发需求分析师
        if (!messages || messages.length === 0) {
          // 自动创建一条欢迎消息
          const welcomeMessage: Message = {
            id: `sys-${Date.now()}`,
            threadId,
            role: 'system',
            content: '欢迎使用开发工作台！需求分析师已启动，请描述您的需求。',
            messageType: 'command',
            createdAt: new Date().toISOString(),
          };
          initialMessages = [welcomeMessage];

          // 自动触发需求分析师 Agent
          try {
            api.invocations.spawn(threadId, 'requirement', '用户已创建新任务，请主动询问并收集用户的需求。请用友好的语气打招呼，并引导用户描述需求。')
              .catch(err => console.error('Failed to spawn agent:', err));
          } catch (spawnError) {
            console.error('Failed to auto-spawn requirement agent:', spawnError);
          }
        }

        set({
          currentThread: thread,
          messages: initialMessages,
          activeAgents: (invocations || []).filter((i: AgentInvocation) => i.status === 'running'),
        });
      } catch (error) {
        set({ error: (error as Error).message });
      } finally {
        set({ loading: false });
      }
    },

    sendMessage: async (content) => {
      const { currentThread, messages } = get();
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

      // 立即更新本地消息列表
      set({ messages: [...messages, userMessage] });

      try {
        await api.messages.create(currentThread.id, content);
      } catch (error) {
        set({ error: (error as Error).message });
      }
    },

    spawnAgent: async (role, input) => {
      const { currentThread } = get();
      if (!currentThread) return;

      try {
        await api.invocations.spawn(currentThread.id, role, input);
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
  }))
);

export default useAppStore;
