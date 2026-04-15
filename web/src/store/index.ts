import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { Thread, Message, AgentInvocation, AgentConfig, Phase, AgentRole, Project, WorkflowTemplate, SandboxServer, MessageContentBlock } from '@/types';
import type { TokenUsage, TaskProgress } from '@/types/status';
import type { BlockingItem } from '@/types/blocking';
import api from '@/api/client';

// localStorage 持久化 key
const STORAGE_KEY_BLOCKING_REMINDER = 'isdp_blocking_reminder_enabled';

interface AppState {
  // 当前项目
  currentProjectId: string | null;

  // 当前Thread
  currentThread: Thread | null;

  // 消息列表
  messages: Message[];

  // 流式消息状态
  isStreaming: boolean;
  streamingAgentId: string | null;
  streamingAgentName: string | null;
  streamingInvocationId: string | null;
  // 流式内容块（按返回顺序追加，thinking/text 智能累积）
  streamingContentBlocks: MessageContentBlock[];

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

  // 可收缩面板默认状态
  collapsibleDefaults: {
    toolOutput: 'expanded' | 'collapsed';
    thinking: 'expanded' | 'collapsed';
  };

  // 阻塞提醒相关
  blockingItems: BlockingItem[];
  blockingReminderEnabled: boolean;

  // 已提交的 question block IDs（用于过滤历史消息中的重复渲染）
  submittedQuestionBlockIds: Set<string>;
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
  updateAgentStatus: (invocationId: string, status: string, agentName?: string, input?: string) => void;

  // 更新 invocation 的完整 prompt
  updateInvocationFullPrompt: (invocationId: string, fullPrompt: string) => void;

  // 恢复 invocation 状态（后台执行支持）
  recoverInvocationState: (invocationId: string, contentBlocks: MessageContentBlock[], status: string, agentId?: string, agentName?: string) => void;

  // 更新 invocation 状态（后台执行完成状态同步）
  updateInvocationStatus: (invocationId: string, status: string, agentId?: string, agentName?: string) => void;

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

  // 更新流式消息（实时输出）- 保留用于文本累积
  updateStreamingMessage: (invocationId: string, chunk: string, agentId: string, agentName?: string) => void;

  // 完成流式消息（转为正式消息）
  finalizeStreamingMessage: (invocationId: string) => void;

  // 追加内容块（思考/工具调用/文本，按顺序）
  appendContentBlock: (invocationId: string, block: MessageContentBlock) => void;

  // 更新内容块（如工具调用状态变化）
  updateContentBlock: (invocationId: string, blockId: string, update: Partial<MessageContentBlock>) => void;

  // 标记 question block 已提交（用于过滤历史消息中的重复渲染）
  markQuestionSubmitted: (blockId: string) => void;

  // 用真实消息替换临时消息（完整替换，包括 contentBlocks）
  replaceMessageId: (tempId: string, realId: string, realContentBlocks?: MessageContentBlock[]) => void;

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

  // 可收缩面板默认状态 actions
  setCollapsibleDefaults: (type: 'toolOutput' | 'thinking', state: 'expanded' | 'collapsed') => void;

  // 阻塞管理 actions
  addBlockingItem: (item: BlockingItem) => void;
  removeBlockingItem: (id: string) => void;
  clearBlockingItems: () => void;
  setBlockingReminderEnabled: (enabled: boolean) => void;
}

const initialState: AppState = {
  currentProjectId: null,
  currentThread: null,
  messages: [],
  // 流式状态
  isStreaming: false,
  streamingAgentId: null,
  streamingAgentName: null,
  streamingInvocationId: null,
  streamingContentBlocks: [],
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
  // 可收缩面板默认状态
  collapsibleDefaults: {
    toolOutput: 'collapsed',
    thinking: 'collapsed',
  },
  // 阻塞提醒相关
  blockingItems: [],
  blockingReminderEnabled: localStorage.getItem(STORAGE_KEY_BLOCKING_REMINDER) !== 'false',
  // 已提交的 question block IDs
  submittedQuestionBlockIds: new Set<string>(),
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
        streamingAgentId: null,
        streamingAgentName: null,
        streamingInvocationId: null,
        streamingContentBlocks: [],
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

        // 构建已完成的 invocation ID 集合
        const completedInvocationIds = new Set(
          (invocations || [])
            .filter((i: AgentInvocation) => i.status === 'completed' || i.status === 'failed' || i.status === 'interrupted')
            .map((i: AgentInvocation) => i.id)
        );

        // 更新消息中已完成的 content blocks 状态
        const updatedMessages = (messages || []).map((msg: Message) => {
          if (msg.contentBlocks && msg.contentBlocks.length > 0) {
            // 检查消息关联的 invocation 是否已完成
            // 通过 agentId 或消息中的 metadata 判断
            const msgInvocationId = msg.metadata?.invocationId as string | undefined;
            const isCompleted = msgInvocationId && completedInvocationIds.has(msgInvocationId);

            if (isCompleted) {
              // 更新 content blocks 的状态（只有 thinking 和 tool_use 有 status）
              const updatedBlocks = msg.contentBlocks.map(block => {
                if ((block.type === 'thinking' || block.type === 'tool_use') && 'status' in block && block.status === 'streaming') {
                  return {
                    ...block,
                    status: 'success' as const,
                  };
                }
                return block;
              });
              return {
                ...msg,
                contentBlocks: updatedBlocks,
              };
            }
          }
          return msg;
        });

        set({
          currentThread: thread,
          messages: updatedMessages,
          activeAgents: (invocations || []).filter((i: AgentInvocation) => i.status === 'running'),
          // 恢复已完成的 Agent 历史
          completedAgents: (invocations || [])
            .filter((i: AgentInvocation) =>
              i.status === 'completed' ||
              i.status === 'failed' ||
              i.status === 'interrupted' ||
              i.status === 'cancelled'
            )
            .map((i: AgentInvocation) => ({
              ...i,
              // 确保 agentName 存在，如果没有则使用 role
              agentName: i.agentName || (i.role === 'custom' ? 'Agent' : i.role),
            })),
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
      const { currentThread, blockingItems } = get();
      if (!currentThread) return;

      // 用户发送新消息时，清除之前的阻塞项（开始新的交互）
      if (blockingItems.length > 0) {
        set({ blockingItems: [] });
      }

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
      const { currentThread, blockingItems } = get();
      if (!currentThread) return;

      // 新 Agent 启动时，清除之前的阻塞项（因为用户已经开始新的任务了）
      if (blockingItems.length > 0) {
        set({ blockingItems: [] });
      }

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

    updateAgentStatus: (invocationId, status, agentName?: string, input?: string) => {
      set((state) => {
        if (status === 'completed' || status === 'failed' || status === 'cancelled') {
          // 找到完成的 agent 并移到历史列表
          const completedAgent = state.activeAgents.find((a) => a.id === invocationId);

          // 对于 cancelled 状态，保留已输出的内容（转成临时消息）
          // 只有 completed/failed 才清理流式状态
          const isCancelled = status === 'cancelled';
          const isCurrentStreaming = state.streamingInvocationId === invocationId;

          // 如果取消且有流式内容，创建一个临时消息保留这些内容
          let newMessages = state.messages;
          if (isCancelled && isCurrentStreaming && state.streamingContentBlocks.length > 0) {
            const tempMessage: Message = {
              id: `agent-${invocationId}`,
              threadId: state.currentThread?.id || '',
              role: 'agent',
              agentId: state.streamingAgentId || '',
              content: state.streamingContentBlocks
                .filter(b => b.type === 'text')
                .map(b => b.type === 'text' ? b.content : '')
                .join(''),
              contentBlocks: state.streamingContentBlocks,
              messageType: 'text',
              metadata: {
                agentName: state.streamingAgentName,
                cancelled: true,
              },
              createdAt: new Date().toISOString(),
            };
            // 检查是否已存在
            const exists = state.messages.some(m => m.id === tempMessage.id);
            if (!exists) {
              newMessages = [...state.messages, tempMessage];
            }
          }

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

          // 重置流式状态（但 cancelled 不清空 contentBlocks，因为已转为消息）
          return {
            messages: newMessages,
            activeAgents: state.activeAgents.filter((a) => a.id !== invocationId),
            completedAgents: newCompletedAgents,
            // 重置流式状态
            isStreaming: isCurrentStreaming ? false : state.isStreaming,
            streamingInvocationId: isCurrentStreaming ? null : state.streamingInvocationId,
            streamingAgentId: isCurrentStreaming ? null : state.streamingAgentId,
            streamingAgentName: isCurrentStreaming ? null : state.streamingAgentName,
            streamingContentBlocks: isCurrentStreaming ? [] : state.streamingContentBlocks,
            // 重置进度状态
            progressStatus: isCurrentStreaming ? 'idle' : state.progressStatus,
            progressToolName: isCurrentStreaming ? null : state.progressToolName,
            progressToolInput: isCurrentStreaming ? null : state.progressToolInput,
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
                  input: input, // 保存输入内容
                  startedAt: new Date().toISOString(),
                } as AgentInvocation,
              ],
            };
          }
        }
        return state;
      });
    },

    recoverInvocationState: (invocationId, contentBlocks, status, agentId?: string, agentName?: string) => {
      set((state) => {
        if (status === 'running') {
          // 去重：合并现有块和恢复块，按 id 去重
          const existingIds = new Set(state.streamingContentBlocks.map((b) => b.id));
          const newBlocks = contentBlocks.filter((b) => !existingIds.has(b.id));
          const mergedBlocks = [...state.streamingContentBlocks, ...newBlocks];

          // 确保该 invocation 在 activeAgents 中
          const exists = state.activeAgents.some((a) => a.id === invocationId);
          const newActiveAgents = exists
            ? state.activeAgents
            : [
                ...state.activeAgents,
                {
                  id: invocationId,
                  status: 'running',
                  agentName: agentName,
                  startedAt: new Date().toISOString(),
                } as AgentInvocation,
              ];

          return {
            isStreaming: true,
            streamingInvocationId: invocationId,
            streamingAgentId: agentId || null,
            streamingAgentName: agentName || null,
            streamingContentBlocks: mergedBlocks,
            activeAgents: newActiveAgents,
          };
        }
        return state;
      });
    },

    updateInvocationStatus: (invocationId, status, _agentId?: string, _agentName?: string) => {
      set((state) => {
        // 找到完成的 agent
        const completedAgent = state.activeAgents.find((a) => a.id === invocationId);

        // 如果是当前正在流式输出的 invocation，停止流式输出
        const isCurrentStreaming = state.streamingInvocationId === invocationId;

        // 如果 activeAgents 中没有这个 agent，说明页面可能刚刷新或数据已从 API 加载
        // 不应该用假数据覆盖，直接忽略即可
        if (!completedAgent) {
          return {
            isStreaming: isCurrentStreaming ? false : state.isStreaming,
            streamingInvocationId: isCurrentStreaming ? null : state.streamingInvocationId,
            streamingAgentId: isCurrentStreaming ? null : state.streamingAgentId,
            streamingAgentName: isCurrentStreaming ? null : state.streamingAgentName,
          };
        }

        // 将完成的 agent 移到 completedAgents（保留原始数据）
        const newCompletedAgents = [
          ...state.completedAgents.filter((a) => a.id !== invocationId),
          {
            ...completedAgent,
            status: status as 'completed' | 'failed' | 'interrupted' | 'cancelled',
            completedAt: new Date().toISOString(),
          },
        ];

        return {
          activeAgents: state.activeAgents.filter((a) => a.id !== invocationId),
          completedAgents: newCompletedAgents,
          isStreaming: isCurrentStreaming ? false : state.isStreaming,
          streamingInvocationId: isCurrentStreaming ? null : state.streamingInvocationId,
          streamingAgentId: isCurrentStreaming ? null : state.streamingAgentId,
          streamingAgentName: isCurrentStreaming ? null : state.streamingAgentName,
        };
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
        streamingAgentId: null,
        streamingAgentName: null,
        streamingInvocationId: null,
        streamingContentBlocks: [],
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
        // 只处理当前 invocation
        if (state.streamingInvocationId && state.streamingInvocationId !== invocationId) {
          return {};
        }

        // 智能累积：查找最后一个 text 块，如果存在且状态正常则追加
        const blocks = state.streamingContentBlocks;
        const lastBlock = blocks.length > 0 ? blocks[blocks.length - 1] : null;

        if (lastBlock && lastBlock.type === 'text') {
          // 追加到最后一个 text 块
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

        // 没有可追加的 text 块，创建新的
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
      set((state) => {
        // 如果不是当前 invocation，直接返回
        if (state.streamingInvocationId !== invocationId) {
          return {};
        }

        // 从 contentBlocks 提取文本内容
        const textBlocks = state.streamingContentBlocks.filter(b => b.type === 'text');
        const content = textBlocks.map(b => b.type === 'text' ? b.content : '').join('');

        // 只将已提交或失败的 question blocks 的 ID 加入 submittedQuestionBlockIds
        // waiting_user_input 状态的 question blocks 应保留在历史消息中渲染（等待用户响应）
        const submittedQuestionBlockIds = state.streamingContentBlocks
          .filter(b => b.type === 'question' && (b.status === 'success' || b.status === 'failed'))
          .map(b => b.id);
        const newSubmittedIds = new Set(state.submittedQuestionBlockIds);
        submittedQuestionBlockIds.forEach(id => newSubmittedIds.add(id));

        // 去重检查：如果消息已存在，只清理流式缓存
        const messageId = `agent-${invocationId}`;
        const exists = state.messages.some(m => m.id === messageId);
        if (exists) {
          return {
            isStreaming: false,
            streamingAgentId: null,
            streamingAgentName: null,
            streamingInvocationId: null,
            streamingContentBlocks: [],
            progressStatus: 'idle',
            progressToolName: null,
            progressToolInput: null,
            submittedQuestionBlockIds: newSubmittedIds,
          };
        }

        // 创建最终消息
        const finalMessage: Message = {
          id: messageId,
          threadId: state.currentThread?.id || '',
          role: 'agent',
          agentId: state.streamingAgentId || '',
          content,
          messageType: 'text',
          metadata: {
            agentName: state.streamingAgentName,
          },
          createdAt: new Date().toISOString(),
          contentBlocks: state.streamingContentBlocks.length > 0 ? state.streamingContentBlocks : undefined,
        };

        return {
          isStreaming: false,
          streamingAgentId: null,
          streamingAgentName: null,
          streamingInvocationId: null,
          streamingContentBlocks: [],
          progressStatus: 'idle',
          progressToolName: null,
          progressToolInput: null,
          messages: [...state.messages, finalMessage],
          submittedQuestionBlockIds: newSubmittedIds,
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

    replaceMessageId: (tempId, realId, realContentBlocks) => {
      set((state) => {
        const messages = state.messages.map((m) =>
          m.id === tempId ? {
            ...m,
            id: realId,
            // 如果提供了真实的 contentBlocks，用真实的替换（避免重复渲染 question blocks）
            contentBlocks: realContentBlocks || m.contentBlocks,
          } : m
        );
        return { messages };
      });
    },

    // 追加内容块（思考/工具调用/文本，按顺序，thinking 智能累积）
    appendContentBlock: (invocationId, block) => {
      set((state) => {
        // 问题块类型不受 invocationId 限制（允许跨 invocation 添加）
        // 因为问题块需要等待用户输入，可能跨多个 invocation 存在
        const isQuestionBlock = block.type === 'question';

        // 非 question 类型：只处理当前 invocation 的内容块
        if (!isQuestionBlock && state.streamingInvocationId && state.streamingInvocationId !== invocationId) {
          return {};
        }

        const blocks = state.streamingContentBlocks;

        // thinking 块智能累积：查找同 ID 的现有 thinking 块，追加内容
        // 注意：不再依赖 status === 'streaming' 判断，因为可能在 done=true 后仍有内容
        if (block.type === 'thinking') {
          const existingThinkingIndex = blocks.findIndex(
            (b) => b.id === block.id && b.type === 'thinking'
          );
          if (existingThinkingIndex >= 0) {
            // 已存在同 ID 的 thinking 块，累积内容
            const existingBlock = blocks[existingThinkingIndex] as typeof block;
            const updatedBlocks = [...blocks];
            updatedBlocks[existingThinkingIndex] = {
              ...existingBlock,
              content: existingBlock.content + block.content,
              // 如果新块状态是 success，更新状态
              status: block.status === 'success' ? 'success' : existingBlock.status,
            } as MessageContentBlock;
            return {
              isStreaming: true,
              streamingInvocationId: invocationId,
              streamingContentBlocks: updatedBlocks,
            };
          }
        }

        // 去重检查：如果已存在相同 id 的块，更新而非追加（用于 tool_result 和 question 等）
        const existingIndex = blocks.findIndex((b) => b.id === block.id);
        if (existingIndex >= 0) {
          // 已存在，更新该块（合并属性）
          const updatedBlocks = [...blocks];
          updatedBlocks[existingIndex] = { ...updatedBlocks[existingIndex], ...block } as MessageContentBlock;
          return {
            isStreaming: true,
            streamingInvocationId: invocationId,
            streamingContentBlocks: updatedBlocks,
          };
        }

        // 默认：追加新块
        return {
          isStreaming: true,
          streamingInvocationId: invocationId,
          streamingContentBlocks: [...blocks, block],
        };
      });
    },

    // 更新内容块（如工具调用状态变化）
    updateContentBlock: (invocationId, blockId, update) => {
      set((state) => {
        // 问题块类型的更新不受 invocationId 限制（允许跨 invocation 更新）
        // 因为问题块可能来自不同的 invocation，用户可能在多个问题之间来回操作
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

    // 标记 question block 已提交（用于过滤历史消息中的重复渲染）
    markQuestionSubmitted: (blockId) => {
      set((state) => {
        const newSet = new Set(state.submittedQuestionBlockIds);
        newSet.add(blockId);
        return { submittedQuestionBlockIds: newSet };
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

    // 更新 invocation 的完整 prompt
    updateInvocationFullPrompt: (invocationId, fullPrompt) => {
      set((state) => {
        // 更新 activeAgents
        const updatedActiveAgents = state.activeAgents.map((a) =>
          a.id === invocationId ? { ...a, fullPrompt } : a
        );

        // 更新 completedAgents
        const updatedCompletedAgents = state.completedAgents.map((a) =>
          a.id === invocationId ? { ...a, fullPrompt } : a
        );

        return {
          activeAgents: updatedActiveAgents,
          completedAgents: updatedCompletedAgents,
        };
      });
    },

    // 可收缩面板默认状态 actions
    setCollapsibleDefaults: (type, state) => {
      set((prev) => ({
        collapsibleDefaults: {
          ...prev.collapsibleDefaults,
          [type]: state,
        },
      }));
    },

    // 阻塞管理 actions
    addBlockingItem: (item) => {
      set((state) => {
        // 去重检查：相同 invocationId + type 不重复添加
        const exists = state.blockingItems.some(
          (b) => b.invocationId === item.invocationId && b.type === item.type
        );
        if (exists) {
          return state;
        }
        return {
          blockingItems: [...state.blockingItems, item],
        };
      });
    },

    removeBlockingItem: (id) => {
      set((state) => ({
        blockingItems: state.blockingItems.filter((b) => b.id !== id),
      }));
    },

    clearBlockingItems: () => {
      set({ blockingItems: [] });
    },

    setBlockingReminderEnabled: (enabled) => {
      set({ blockingReminderEnabled: enabled });
      localStorage.setItem(STORAGE_KEY_BLOCKING_REMINDER, String(enabled));
    },
  }))
);

export default useAppStore;
