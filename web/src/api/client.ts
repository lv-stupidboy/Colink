import axios, { AxiosInstance } from 'axios';
import type {
  Project,
  Thread,
  Message,
  AgentConfig,
  BaseAgent,
  BaseAgentTypeInfo,
  AgentInvocation,
  Artifact,
  MergeCheckResult,
  WorkflowTemplate,
  ListFilesResponse,
  Skill,
  SkillUpdateResponse,
  AssetAgentsResponse,
  CreateSkillRequest,
  UpdateSkillRequest,
  SkillListQuery,
  SkillListResponse,
  AgentSkillsResponse,
  SkillAgentsResponse,
  SkillRegistry,
  CreateRegistryRequest,
  UpdateRegistryRequest,
  RegistryListQuery,
  RegistryListResponse,
  SyncResult,
  KnowledgeBase,
  CreateKnowledgeBaseRequest,
  UpdateKnowledgeBaseRequest,
  KnowledgeBaseListQuery,
  KnowledgeBaseListResponse,
  KnowledgeQueryRequest,
  KnowledgeQueryResult,
  BuiltInTagCategory,
  Subagent,
  SubagentUpdateResponse,
  CreateSubagentRequest,
  UpdateSubagentRequest,
  SubagentListQuery,
  SubagentListResponse,
  Command,
  CommandUpdateResponse,
  CreateCommandRequest,
  UpdateCommandRequest,
  CommandListQuery,
  CommandListResponse,
  Rule,
  RuleUpdateResponse,
  CreateRuleRequest,
  UpdateRuleRequest,
  RuleListQuery,
  RuleListResponse,
  CommandSkillsResponse,
  AgentRulesResponse,
  ExportAssetPackageRequest,
  ImportResult,
  Settings,
  SettingsListQuery,
  SettingsListResponse,
  AgentSettingsResponse,
  HumanTask,
  HumanTaskStatus,
  SubmitHumanTaskRequest,
  SubmitHumanTaskResponse,
  BatchGenerateResult,
  BatchUpdateResult,
  RemotePackageList,
  UpdateCheckResult,
  TeamPackageVersion,
  ImportConfirm,
  Market,
  MarketPackage,
  AddMarketRequest,
  UpdateMarketRequest,
  PackagePreviewResponse,
  ScanResult,
  BatchImportRequest,
  BatchImportResult,
  SyncPreviewResult,
  SyncConfirmRequest,
  SyncConfirmResult,
} from '@/types';
import {
  transformProjects,
  transformProject,
  transformThreads,
  transformThread,
  transformMessages,
  transformMessage,
  transformAgentConfigs,
  transformAgentConfig,
  transformAgentInvocations,
  transformAgentInvocation,
  transformArtifacts,
  transformArtifact,
  transformWorkflowTemplates,
  transformWorkflowTemplate,
} from './transform';

class APIClient {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: '/api/v1',
      timeout: 120000, // 2分钟默认超时
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // 请求拦截器
    this.client.interceptors.request.use(
      (config) => {
        // 添加认证 token
        const token = localStorage.getItem('token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        // 后端 JSON 字段使用 camelCase，无需转换
        return config;
      },
      (error) => Promise.reject(error)
    );
  }

  // 辅助方法：发送请求并转换响应
  private async request<T>(url: string, method: string, data?: any, config?: any): Promise<T> {
    try {
      const response = await this.client.request({
        url,
        method,
        data,
        ...config,
      });

      let result = response.data;

      // 转换 snake_case 为 camelCase
      if (result && typeof result === 'object') {
        if (url.includes('/base-agents')) {
          // 基础Agent - 使用通用转换
          const snakeToCamel = (obj: any): any => {
            if (obj === null || obj === undefined) return obj;
            if (Array.isArray(obj)) return obj.map(snakeToCamel);
            if (typeof obj !== 'object') return obj;
            const res: any = {};
            for (const key in obj) {
              if (Object.prototype.hasOwnProperty.call(obj, key)) {
                const camelKey = key.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
                res[camelKey] = snakeToCamel(obj[key]);
              }
            }
            return res;
          };
          result = snakeToCamel(result);
        } else if (url.includes('/messages')) {
          // 消息 API - 必须在 /threads 检查之前，因为 URL 包含 /messages/thread/
          result = Array.isArray(result) ? transformMessages(result) : transformMessage(result);
        } else if (url.includes('/projects')) {
          result = Array.isArray(result) ? transformProjects(result) : transformProject(result);
        } else if (url.includes('/threads')) {
          result = Array.isArray(result) ? transformThreads(result) : transformThread(result);
        } else if (url.includes('/agents') && !url.includes('/config/') && !url.includes('/batch-')) {
          result = Array.isArray(result) ? transformAgentConfigs(result) : transformAgentConfig(result);
        } else if (url.includes('/invocations')) {
          result = Array.isArray(result) ? transformAgentInvocations(result) : transformAgentInvocation(result);
        } else if (url.includes('/artifacts')) {
          result = Array.isArray(result) ? transformArtifacts(result) : transformArtifact(result);
        } else if (url.includes('/workflows')) {
          result = Array.isArray(result) ? transformWorkflowTemplates(result) : transformWorkflowTemplate(result);
        } else {
          // 通用转换
          const snakeToCamel = (obj: any): any => {
            if (obj === null || obj === undefined) return obj;
            if (Array.isArray(obj)) return obj.map(snakeToCamel);
            if (typeof obj !== 'object') return obj;
            const res: any = {};
            for (const key in obj) {
              if (Object.prototype.hasOwnProperty.call(obj, key)) {
                const camelKey = key.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
                res[camelKey] = snakeToCamel(obj[key]);
              }
            }
            return res;
          };
          result = snakeToCamel(result);
        }
      }

      return result as T;
    } catch (error: any) {
      if (error.response?.status === 401) {
        localStorage.removeItem('token');
        window.location.href = '/login';
      }

      // 统一错误处理：优先使用结构化错误
      const apiError = error.response?.data;
      if (apiError?.code && apiError?.message) {
        // 新格式：结构化错误
        throw { code: apiError.code, message: apiError.message };
      }
      // 旧格式兜底
      throw { code: 'UNKNOWN', message: apiError?.error || '操作失败' };
    }
  }

  // 项目 API
  projects = {
    list: (): Promise<Project[]> => this.request('/projects', 'GET'),
    get: (id: string): Promise<Project> => this.request(`/projects/${id}`, 'GET'),
    create: (data: Partial<Project>): Promise<Project> => this.request('/projects', 'POST', data),
    update: (id: string, data: Partial<Project>): Promise<Project> => this.request(`/projects/${id}`, 'PUT', data),
    delete: (id: string): Promise<void> => this.request(`/projects/${id}`, 'DELETE'),
    listFiles: (id: string, path?: string): Promise<ListFilesResponse> => {
      const url = path ? `/projects/${id}/files?path=${encodeURIComponent(path)}` : `/projects/${id}/files`;
      return this.request(url, 'GET');
    },
    // 根据路径浏览文件（调试模式，不需要项目ID）
    browseFiles: (basePath: string, path?: string): Promise<ListFilesResponse> => {
      const url = path
        ? `/files/browse?basePath=${encodeURIComponent(basePath)}&path=${encodeURIComponent(path)}`
        : `/files/browse?basePath=${encodeURIComponent(basePath)}`;
      return this.request(url, 'GET');
    },
  };

  // 文件浏览 API（用于路径选择）
  files = {
    // 浏览文件系统路径，返回目录列表
    browse: (path?: string): Promise<{
      currentPath: string;
      parentPath: string;
      entries: Array<{ name: string; path: string; isDir: boolean }>;
      drives?: string[];
      isValid: boolean;
      error?: string;
    }> => {
      const url = path ? `/files/browse?path=${encodeURIComponent(path)}` : '/files/browse';
      return this.request(url, 'GET');
    },
    // 验证路径是否可用于项目
    validate: (path: string): Promise<{
      isValid: boolean;
      exists: boolean;
      isDir: boolean;
      writable: boolean;
      error?: string;
      canCreate: boolean;
    }> => {
      return this.request(`/files/validate?path=${encodeURIComponent(path)}`, 'GET');
    },
    // 创建文件夹
    createFolder: (parentPath: string, name: string): Promise<{ success: boolean }> => {
      return this.request('/files/folder', 'POST', { path: parentPath, name });
    },
    // 获取文件内容（用于文件预览）
    getContent: (basePath: string, path: string): Promise<{
      content: string;
      size: number;
      truncated: boolean;
      path: string;
      isBinary: boolean;
    }> => {
      const url = `/files/content?basePath=${encodeURIComponent(basePath)}&path=${encodeURIComponent(path)}`;
      return this.request(url, 'GET');
    },
  };

  // Thread API
  threads = {
    list: (projectId: string): Promise<Thread[]> => this.request(`/threads/project/${projectId}`, 'GET'),
    get: (id: string): Promise<Thread> => this.request(`/threads/${id}`, 'GET'),
    create: (projectId: string, name?: string, workflowTemplateId?: string): Promise<Thread> =>
      this.request(`/threads/project/${projectId}`, 'POST', { name: name || '新任务', workflowTemplateId }),
    update: (id: string, data: { workflowTemplateId?: string }): Promise<Thread> =>
      this.request(`/threads/${id}`, 'PUT', data),
    delete: (id: string): Promise<void> => this.request(`/threads/${id}`, 'DELETE'),
    updateStatus: (id: string, status: string): Promise<Thread> =>
      this.request(`/threads/${id}/status`, 'PUT', { status }),
    setPhase: (id: string, phase: string, agent: string): Promise<Thread> =>
      this.request(`/threads/${id}/phase`, 'PUT', { phase, agent }),
  };

  // 消息 API
  messages = {
    list: (threadId: string, limit = 10): Promise<{ messages: Message[]; total: number; hasMore: boolean }> =>
      this.request(`/messages/thread/${threadId}`, 'GET', undefined, { params: { limit } }),
    // 加载历史消息（向上滚动加载）
    listHistory: (threadId: string, cursor: string, limit = 10): Promise<{ messages: Message[]; hasMore: boolean }> =>
      this.request(`/messages/thread/${threadId}/history`, 'GET', undefined, { params: { cursor, limit } }),
    create: (threadId: string, content: string, skipAgentTrigger?: boolean): Promise<Message> =>
      this.request(`/messages/thread/${threadId}`, 'POST', { content, skipAgentTrigger }),
  };

  // Agent 配置 API
  agents = {
    list: (): Promise<AgentConfig[]> => this.request('/agents', 'GET'),
    get: (id: string): Promise<AgentConfig> => this.request(`/agents/${id}`, 'GET'),
    create: (data: Partial<AgentConfig>): Promise<AgentConfig> => this.request('/agents', 'POST', data),
    update: (id: string, data: Partial<AgentConfig>): Promise<AgentConfig> =>
      this.request(`/agents/${id}`, 'PUT', data),
    delete: (id: string): Promise<void> => this.request(`/agents/${id}`, 'DELETE'),
    getByRole: (role: string): Promise<AgentConfig[]> => this.request(`/agents/role/${role}`, 'GET'),
    copy: (id: string): Promise<AgentConfig> => this.request(`/agents/${id}/copy`, 'POST'),
    checkReferences: (id: string): Promise<{ referenced: boolean; referenceCount: number; referenceNames: string[] }> =>
      this.request(`/agents/${id}/refs`, 'POST'),
    // 预创建调试Thread，前端先调用此方法获取threadId，建立WebSocket连接
    createDebugThread: (projectPath?: string): Promise<{ threadId: string }> =>
      this.request('/agents/debug/thread', 'POST', { projectPath }),
    // 调试Agent，threadId可选（如果已预创建则传入）
    debug: (id: string, input: string, projectPath?: string, threadId?: string): Promise<{ invocationId: string; threadId: string; output: string; sandboxUrl?: string }> =>
      this.request(`/agents/${id}/debug`, 'POST', { input, projectPath, threadId }),
    continueDebug: (threadId: string, message: string): Promise<{ status: string }> =>
      this.request(`/agents/debug/${threadId}/continue`, 'POST', { message }),
    // Skill 相关
    getSkills: (agentId: string): Promise<AgentSkillsResponse> =>
      this.request(`/agent-skills/${agentId}`, 'GET'),
    bindSkills: (agentId: string, skillIds: string[]): Promise<{ message: string }> =>
      this.request(`/agent-skills/${agentId}`, 'POST', { skillIds: skillIds }),
    unbindSkill: (agentId: string, skillId: string): Promise<{ message: string }> =>
      this.request(`/agent-skills/${agentId}/${skillId}`, 'DELETE'),
    // 配置生成相关
    previewConfig: (id: string): Promise<{
      agentId: string;
      agentName: string;
      skills: Array<{ id: string; name: string; description: string }>;
      commands: Array<{ id: string; name: string; description: string }>;
      subagents: Array<{ id: string; name: string; description: string }>;
      rules: Array<{ id: string; name: string; description: string }>;
      settings: Array<{ id: string; name: string; description: string }>;
      skillsCount: number;
      commandsCount: number;
      subagentsCount: number;
      rulesCount: number;
      settingsCount: number;
    }> => this.request(`/agents/${id}/config/preview`, 'GET'),
    generateConfig: (id: string, baseAgentType: string, cleanExisting?: boolean): Promise<{
      message: string;
      agentId: string;
      configPath: string;
      skillsCount: number;
      subagentsCount: number;
      commandsCount: number;
      rulesCount: number;
      settingsCount: number;
      generatedAt: string;
    }> => this.request(`/agents/${id}/config/generate`, 'POST', {
      baseAgentType: baseAgentType,
      cleanExisting: cleanExisting !== false, // 默认清空
    }),
    // 刷新配置（自动检测类型，无需传递 baseAgentType）
    refreshConfig: (id: string): Promise<{
      message: string;
      agentId: string;
      agentName: string;
      configPath: string;
    }> => this.request(`/agents/${id}/refresh`, 'POST'),
    getSubagents: (id: string): Promise<{ subagents: Subagent[] }> =>
      this.request(`/agents/${id}/subagents`, 'GET'),
    bindSubagents: (id: string, subagentIds: string[]): Promise<{ message: string }> =>
      this.request(`/agents/${id}/subagents`, 'POST', { subagentIds: subagentIds }),
    unbindSubagent: (id: string, subagentId: string): Promise<{ message: string }> =>
      this.request(`/agents/${id}/subagents/${subagentId}`, 'DELETE'),
    // AskUserQuestion 答案提交（通过 stdin 发送给 CLI）
    submitQuestionAnswer: (threadId: string, toolCallId: string, answer: string): Promise<{ status: string }> =>
      this.request(`/agents/question/${threadId}/answer`, 'POST', { toolCallId, answer }),
    // 批量删除
    batchDelete: (ids: string[]): Promise<void> =>
      this.request('/agents/batch-delete', 'POST', { ids }),
    // 批量生成配置
    batchGenerateConfig: (agentIds: string[], cliType?: string): Promise<BatchGenerateResult> =>
      this.request('/agents/batch-generate-config', 'POST', {
        agentIds,
        cliType: cliType || 'claude_code',
      }),
    // 批量修改基础Agent
    batchUpdateBaseAgent: (agentIds: string[], baseAgentId: string): Promise<BatchUpdateResult> =>
      this.request('/agents/batch-update-base-agent', 'POST', {
        agentIds,
        baseAgentId,
      }),
  };

  // HumanTask API（待办任务）
  humanTasks = {
    list: (status?: HumanTaskStatus): Promise<HumanTask[]> =>
      this.request(`/human-tasks${status ? `?status=${status}` : ''}`, 'GET'),
    get: (id: string): Promise<HumanTask> =>
      this.request(`/human-tasks/${id}`, 'GET'),
    complete: (id: string): Promise<{ status: string }> =>
      this.request(`/human-tasks/${id}/complete`, 'PUT'),
    cancel: (id: string): Promise<{ status: string }> =>
      this.request(`/human-tasks/${id}/cancel`, 'PUT'),
    // 根据 invocationId 关闭待办任务（用于 AskUserQuestion 回答后）
    completeByInvocation: (invocationId: string): Promise<{ status: string }> =>
      this.request(`/human-tasks/invocation/${invocationId}/complete`, 'PUT'),
    // 提交任务执行结果（用于 TaskExecuteModal）
    submit: (id: string, data: SubmitHumanTaskRequest): Promise<SubmitHumanTaskResponse> =>
      this.request(`/human-tasks/${id}/submit`, 'POST', data),
  };

  // 基础Agent API
  baseAgents = {
    list: (): Promise<BaseAgent[]> => this.request('/base-agents', 'GET'),
    get: (id: string): Promise<BaseAgent> => this.request(`/base-agents/${id}`, 'GET'),
    getTypes: (): Promise<BaseAgentTypeInfo[]> => this.request('/base-agents/types', 'GET'),
    create: (data: Partial<BaseAgent>): Promise<BaseAgent> => this.request('/base-agents', 'POST', data),
    update: (id: string, data: Partial<BaseAgent>): Promise<BaseAgent> =>
      this.request(`/base-agents/${id}`, 'PUT', data),
    delete: (id: string): Promise<void> => this.request(`/base-agents/${id}`, 'DELETE'),
    test: (id: string): Promise<{ success: boolean; message: string }> =>
      this.request(`/base-agents/${id}/test`, 'POST'),
    setDefault: (id: string): Promise<BaseAgent[]> =>
      this.request(`/base-agents/${id}/default`, 'PUT'),
    clearDefault: (id: string): Promise<BaseAgent[]> =>
      this.request(`/base-agents/${id}/default`, 'DELETE'),
  };

  // Agent 调用 API
  invocations = {
    list: (threadId: string): Promise<AgentInvocation[]> =>
      this.request(`/threads/${threadId}/invocations`, 'GET'),
    get: (id: string): Promise<AgentInvocation> => this.request(`/invocations/${id}`, 'GET'),
    spawn: (threadId: string, role: string, input: string, configId?: string): Promise<AgentInvocation> => {
      const payload = { role, input, configId };
      return this.request(`/threads/${threadId}/invocations`, 'POST', payload);
    },
    cancel: (id: string): Promise<void> => this.request(`/invocations/${id}/cancel`, 'POST'),
  };

  // 工作产物 API
  artifacts = {
    list: (threadId: string): Promise<Artifact[]> =>
      this.request(`/threads/${threadId}/artifacts`, 'GET'),
    get: (id: string): Promise<Artifact> => this.request(`/artifacts/${id}`, 'GET'),
    create: (threadId: string, data: Partial<Artifact>): Promise<Artifact> =>
      this.request(`/threads/${threadId}/artifacts`, 'POST', data),
  };

  // 合并门禁 API
  merge = {
    check: (threadId: string): Promise<MergeCheckResult> =>
      this.request(`/threads/${threadId}/merge/check`, 'GET'),
    approve: (threadId: string): Promise<void> =>
      this.request(`/threads/${threadId}/merge/approve`, 'POST'),
    handover: (threadId: string): Promise<any> =>
      this.request(`/threads/${threadId}/merge/handover`, 'GET'),
  };

  // 沙箱 API
  sandbox = {
    run: (threadId: string, data: { image?: string; command: string[] }): Promise<any> =>
      this.request(`/threads/${threadId}/sandbox/run`, 'POST', data),
    status: (runId: string): Promise<any> => this.request(`/sandbox/runs/${runId}`, 'GET'),
    // 新增项目运行API
    runProject: (threadId?: string, projectPath?: string, mode?: string): Promise<any> =>
      this.request('/sandbox/run', 'POST', { threadId: threadId || undefined, projectPath, mode }),
    getServer: (id: string): Promise<any> => this.request(`/sandbox/${id}`, 'GET'),
    stopServer: (id: string): Promise<void> => this.request(`/sandbox/${id}/stop`, 'POST'),
    getPreview: (threadId: string): Promise<any> => this.request(`/sandbox/preview/${threadId}`, 'GET'),
    listServers: (): Promise<any[]> => this.request('/sandbox', 'GET'),
    // 新增方法
    getLogs: (id: string): Promise<{ logs: string }> =>
      this.request(`/sandbox/${id}/logs`, 'GET'),
    checkDocker: (): Promise<{ available: boolean }> =>
      this.request('/sandbox/docker/status', 'GET'),
  };

  // Agent团队 API
  workflows = {
    list: (): Promise<WorkflowTemplate[]> => this.request('/workflows', 'GET'),
    get: (id: string): Promise<WorkflowTemplate> => this.request(`/workflows/${id}`, 'GET'),
    create: (data: Partial<WorkflowTemplate>): Promise<WorkflowTemplate> =>
      this.request('/workflows', 'POST', data),
    update: (id: string, data: Partial<WorkflowTemplate>): Promise<WorkflowTemplate> =>
      this.request(`/workflows/${id}`, 'PUT', data),
    delete: (id: string): Promise<void> => this.request(`/workflows/${id}`, 'DELETE'),
    setDefault: (id: string): Promise<WorkflowTemplate> =>
      this.request(`/workflows/${id}/default`, 'PUT'),
  };

  // Skill API
  skills = {
    list: (query?: SkillListQuery): Promise<SkillListResponse> => {
      const params = new URLSearchParams();
      if (query?.tag) params.append('tag', query.tag);
      if (query?.sourceType) params.append('source_type', query.sourceType);
      if (query?.agentType) params.append('agent_type', query.agentType);
      if (query?.search) params.append('search', query.search);
      if (query?.page) params.append('page', query.page.toString());
      if (query?.pageSize) params.append('page_size', query.pageSize.toString());

      const url = params.toString() ? `/skills?${params.toString()}` : '/skills';
      return this.request(url, 'GET');
    },
    get: (id: string): Promise<Skill> =>
      this.request(`/skills/${id}`, 'GET'),
    create: (data: CreateSkillRequest): Promise<Skill> =>
      this.request('/skills', 'POST', data),
    update: (id: string, data: UpdateSkillRequest): Promise<SkillUpdateResponse> =>
      this.request(`/skills/${id}`, 'PUT', data),
    delete: (id: string): Promise<void> =>
      this.request(`/skills/${id}`, 'DELETE'),
    getBoundAgents: (id: string): Promise<AssetAgentsResponse> =>
      this.request(`/skills/${id}/agents`, 'GET'),
    getTags: (): Promise<string[]> =>
      this.request('/skills/tags', 'GET'),
    getBuiltInTags: (): Promise<BuiltInTagCategory[]> =>
      this.request('/skills/tags/builtin', 'GET'),
    // 从仓库导入
    importRepo: (repoUrl: string): Promise<Skill> =>
      this.request('/skills/import/repo', 'POST', { repoUrl: repoUrl }),
    // 从联邦源导入
    importFederated: (registryId: string, skillName?: string): Promise<Skill | { skills: any[] }> => {
      const body: any = { registryId: registryId };
      if (skillName) body.skillName = skillName;
      return this.request('/skills/import/federated', 'POST', body);
    },
    // 扫描联邦源 Skill 列表（使用更长的超时时间）
    scanFederatedSkills: async (registryId: string): Promise<ScanResult> => {
      const response = await this.client.post('/skills/import/federated/scan', { registryId }, {
        timeout: 180000, // 3分钟超时（git clone 可能需要较长时间）
      });
      return response.data;
    },
    // 批量导入联邦源 Skill（使用更长的超时时间）
    batchImportFederated: async (req: BatchImportRequest): Promise<BatchImportResult> => {
      const response = await this.client.post('/skills/import/federated/batch', req, {
        timeout: 300000, // 5分钟超时（批量导入可能需要较长时间）
      });
      return response.data;
    },
  };

  // Registry API
  registries = {
    list: (query?: RegistryListQuery): Promise<RegistryListResponse> => {
      const params = new URLSearchParams();
      if (query?.type) params.append('type', query.type);
      if (query?.status) params.append('status', query.status);
      if (query?.search) params.append('search', query.search);
      if (query?.page) params.append('page', query.page.toString());
      if (query?.size) params.append('size', query.size.toString());

      const url = params.toString() ? `/registries?${params.toString()}` : '/registries';
      return this.request(url, 'GET');
    },
    get: (id: string): Promise<SkillRegistry> =>
      this.request(`/registries/${id}`, 'GET'),
    create: (data: CreateRegistryRequest): Promise<SkillRegistry> =>
      this.request('/registries', 'POST', data),
    update: (id: string, data: UpdateRegistryRequest): Promise<SkillRegistry> =>
      this.request(`/registries/${id}`, 'PUT', data),
    delete: (id: string): Promise<void> =>
      this.request(`/registries/${id}`, 'DELETE'),
    sync: (id: string): Promise<SyncResult> =>
      this.request(`/registries/${id}/sync`, 'POST'),
    syncPreview: (id: string): Promise<SyncPreviewResult> =>
      this.request(`/registries/${id}/sync-preview`, 'POST'),
    syncConfirm: (id: string, request: SyncConfirmRequest): Promise<SyncConfirmResult> =>
      this.request(`/registries/${id}/sync-confirm`, 'POST', request),
    syncAll: (): Promise<{ message: string; results: SyncResult[] }> =>
      this.request('/registries/sync', 'POST'),
  };

  // Knowledge API
  knowledge = {
    list: (query?: KnowledgeBaseListQuery): Promise<KnowledgeBaseListResponse> => {
      const params = new URLSearchParams();
      if (query?.type) params.append('type', query.type);
      if (query?.status) params.append('status', query.status);
      if (query?.search) params.append('search', query.search);
      if (query?.page) params.append('page', query.page.toString());
      if (query?.size) params.append('size', query.size.toString());

      const url = params.toString() ? `/knowledge?${params.toString()}` : '/knowledge';
      return this.request(url, 'GET');
    },
    get: (id: string): Promise<KnowledgeBase> =>
      this.request(`/knowledge/${id}`, 'GET'),
    create: (data: CreateKnowledgeBaseRequest): Promise<KnowledgeBase> =>
      this.request('/knowledge', 'POST', data),
    update: (id: string, data: UpdateKnowledgeBaseRequest): Promise<KnowledgeBase> =>
      this.request(`/knowledge/${id}`, 'PUT', data),
    delete: (id: string): Promise<void> =>
      this.request(`/knowledge/${id}`, 'DELETE'),
    query: (id: string, request: KnowledgeQueryRequest): Promise<KnowledgeQueryResult> =>
      this.request(`/knowledge/${id}/query`, 'POST', request),
    queryAll: (request: KnowledgeQueryRequest): Promise<{ query: string; results: KnowledgeQueryResult[] }> =>
      this.request('/knowledge/query', 'POST', request),
  };

  // Subagent API
  subagents = {
    list: (query?: SubagentListQuery): Promise<SubagentListResponse> => {
      const params = new URLSearchParams();
      if (query?.search) params.append('search', query.search);
      if (query?.page) params.append('page', query.page.toString());
      if (query?.pageSize) params.append('page_size', query.pageSize.toString());

      const url = params.toString() ? `/subagents?${params.toString()}` : '/subagents';
      return this.request(url, 'GET');
    },
    get: (id: string): Promise<Subagent> =>
      this.request(`/subagents/${id}`, 'GET'),
    create: (data: CreateSubagentRequest): Promise<Subagent> =>
      this.request('/subagents', 'POST', data),
    update: (id: string, data: UpdateSubagentRequest): Promise<SubagentUpdateResponse> =>
      this.request(`/subagents/${id}`, 'PUT', data),
    delete: (id: string): Promise<void> =>
      this.request(`/subagents/${id}`, 'DELETE'),
    // 获取绑定的角色
    getBoundAgents: (id: string): Promise<AssetAgentsResponse> =>
      this.request(`/subagents/${id}/agents`, 'GET'),
    // Subagent-Skill 绑定
    getSkills: (id: string): Promise<{ skills: Skill[]; count: number }> =>
      this.request(`/subagents/${id}/skills`, 'GET'),
    bindSkills: (id: string, skillIds: string[]): Promise<{ message: string }> =>
      this.request(`/subagents/${id}/skills`, 'POST', { skillIds: skillIds }),
    unbindSkill: (id: string, skillId: string): Promise<{ message: string }> =>
      this.request(`/subagents/${id}/skills/${skillId}`, 'DELETE'),
  };

  // Command API
  commands = {
    list: (query?: CommandListQuery): Promise<CommandListResponse> => {
      const params = new URLSearchParams();
      if (query?.search) params.append('search', query.search);
      if (query?.page) params.append('page', query.page.toString());
      if (query?.pageSize) params.append('page_size', query.pageSize.toString());

      const url = params.toString() ? `/commands?${params.toString()}` : '/commands';
      return this.request(url, 'GET');
    },
    get: (id: string): Promise<Command> =>
      this.request(`/commands/${id}`, 'GET'),
    create: (data: CreateCommandRequest): Promise<Command> =>
      this.request('/commands', 'POST', data),
    update: (id: string, data: UpdateCommandRequest): Promise<CommandUpdateResponse> =>
      this.request(`/commands/${id}`, 'PUT', data),
    delete: (id: string): Promise<void> =>
      this.request(`/commands/${id}`, 'DELETE'),
    // 获取绑定的角色
    getBoundAgents: (id: string): Promise<AssetAgentsResponse> =>
      this.request(`/commands/${id}/agents`, 'GET'),
    uploadFile: (formData: FormData): Promise<{ message: string; file_path: string }> => {
      return this.request('/commands/upload', 'POST', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
    },
    // Command 绑定的 Skills
    getSkills: (id: string): Promise<CommandSkillsResponse> =>
      this.request(`/commands/${id}/skills`, 'GET'),
    bindSkills: (id: string, skillIds: string[]): Promise<{ message: string }> =>
      this.request(`/commands/${id}/skills`, 'POST', { skillIds: skillIds }),
    unbindSkill: (id: string, skillId: string): Promise<{ message: string }> =>
      this.request(`/commands/${id}/skills/${skillId}`, 'DELETE'),
    // Agent 绑定的 Commands
    getAgentCommands: (agentId: string): Promise<{ commands: Command[]; count: number }> =>
      this.request(`/agents/${agentId}/commands`, 'GET'),
    bindCommandsToAgent: (agentId: string, commandIds: string[]): Promise<{ message: string }> =>
      this.request(`/agents/${agentId}/commands`, 'POST', { commandIds: commandIds }),
    unbindCommandFromAgent: (agentId: string, commandId: string): Promise<{ message: string }> =>
      this.request(`/agents/${agentId}/commands/${commandId}`, 'DELETE'),
  };

  // Rule API
  rules = {
    list: (query?: RuleListQuery): Promise<RuleListResponse> => {
      const params = new URLSearchParams();
      if (query?.search) params.append('search', query.search);
      if (query?.page) params.append('page', query.page.toString());
      if (query?.pageSize) params.append('page_size', query.pageSize.toString());

      const url = params.toString() ? `/rules?${params.toString()}` : '/rules';
      return this.request(url, 'GET');
    },
    get: (id: string): Promise<Rule> =>
      this.request(`/rules/${id}`, 'GET'),
    create: (data: CreateRuleRequest): Promise<Rule> =>
      this.request('/rules', 'POST', data),
    update: (id: string, data: UpdateRuleRequest): Promise<RuleUpdateResponse> =>
      this.request(`/rules/${id}`, 'PUT', data),
    delete: (id: string): Promise<void> =>
      this.request(`/rules/${id}`, 'DELETE'),
    // 获取绑定的角色
    getBoundAgents: (id: string): Promise<AssetAgentsResponse> =>
      this.request(`/rules/${id}/agents`, 'GET'),
    uploadFile: (formData: FormData): Promise<{ message: string; file_path: string }> => {
      return this.request('/rules/upload', 'POST', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
    },
    // Agent 绑定的 Rules
    getAgentRules: (agentId: string): Promise<AgentRulesResponse> =>
      this.request(`/agents/${agentId}/rules`, 'GET'),
    bindRulesToAgent: (agentId: string, ruleIds: string[]): Promise<{ message: string }> =>
      this.request(`/agents/${agentId}/rules`, 'POST', { ruleIds: ruleIds }),
    unbindRuleFromAgent: (agentId: string, ruleId: string): Promise<{ message: string }> =>
      this.request(`/agents/${agentId}/rules/${ruleId}`, 'DELETE'),
  };

  // Asset Package API (只保留导入和导出)
  assetPackages = {
    import: async (file: File): Promise<any> => {
      const formData = new FormData();
      formData.append('file', file);
      const response = await this.client.post('/asset-packages/import', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
        timeout: 300000, // 5分钟超时
      });
      return response.data;
    },
    importConfirm: async (file: File, confirm: any): Promise<ImportResult> => {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('confirm', new Blob([JSON.stringify(confirm)], { type: 'application/json' }));
      const response = await this.client.post('/asset-packages/import/confirm', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
        timeout: 300000, // 5分钟超时，导入操作耗时较长
      });
      return response.data;
    },
    export: async (data: ExportAssetPackageRequest): Promise<Blob> => {
      const response = await this.client.post('/asset-packages/export', data, {
        responseType: 'blob',
        timeout: 300000, // 5分钟超时，导出操作耗时较长
      });
      return response.data;
    },
  };

  // Team Package API
  teamPackages = {
    import: async (file: File): Promise<any> => {
      const formData = new FormData();
      formData.append('file', file);
      const response = await this.client.post('/team-packages/import', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      return response.data;
    },
    importConfirm: async (file: File, confirm: any): Promise<any> => {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('confirm', new Blob([JSON.stringify(confirm)], { type: 'application/json' }));
      const response = await this.client.post('/team-packages/import/confirm', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
        timeout: 300000, // 5分钟超时，导入操作耗时较长
      });
      return response.data;
    },
    export: async (workflowId: string): Promise<Blob> => {
      const response = await this.client.post('/team-packages/export', { workflowId }, {
        responseType: 'blob',
        timeout: 300000, // 5分钟超时，导出操作耗时较长
      });
      return response.data;
    },
    // Team Package Sync methods
    getRemotePackages: (): Promise<RemotePackageList> =>
      this.request('/team-package-sync/remote', 'GET'),
    checkUpdates: (): Promise<UpdateCheckResult> =>
      this.request('/team-package-sync/check-update', 'GET'),
    previewPackage: (packageName: string, marketId?: string): Promise<PackagePreviewResponse> =>
      this.request('/team-package-sync/preview', 'POST', { packageName, marketId }),
    previewPackagesBatch: (packages: Array<{ name: string; marketId: string }>): Promise<{
      previews: Array<{ name: string; data: PackagePreviewResponse; error?: string }>;
      totalConflicts: number;
      successCount: number;
      failedCount: number;
    }> =>
      this.request('/team-package-sync/preview-batch', 'POST', { packages }),
    syncPackage: (packageName: string, confirm?: ImportConfirm, marketId?: string): Promise<ImportResult> =>
      this.request('/team-package-sync/sync', 'POST', { packageName, confirm, marketId }),
    syncPackagesBatch: (packages: Array<{
      name: string;
      marketId: string;
      confirm?: ImportConfirm;
    }>): Promise<{
      results: Array<{ name: string; data: ImportResult; error?: string }>;
      successCount: number;
      failedCount: number;
    }> =>
      this.request('/team-package-sync/sync-batch', 'POST', { packages }),
    listLocalVersions: (): Promise<{ data: TeamPackageVersion[]; total: number }> =>
      this.request('/team-package-sync/local-versions', 'GET'),
  };

  // Market API
  markets = {
    list: (): Promise<{ data: Market[]; total: number }> =>
      this.request('/markets', 'GET'),
    getDefaultConfig: (): Promise<{ name: string; url: string; branch: string }> =>
      this.request('/markets/default', 'GET'),
    addDefaultMarket: (): Promise<Market> =>
      this.request('/markets/default', 'POST'),
    add: (req: AddMarketRequest): Promise<Market> =>
      this.request('/markets', 'POST', req),
    update: (id: string, req: UpdateMarketRequest): Promise<Market> =>
      this.request(`/markets/${id}`, 'PUT', req),
    delete: (id: string): Promise<{ message: string }> =>
      this.request(`/markets/${id}`, 'DELETE'),
    refresh: (id: string): Promise<{ message: string; plugins: number }> =>
      this.request(`/markets/${id}/refresh`, 'POST'),
    getTeamPackages: (forceRefresh?: boolean): Promise<{ data: MarketPackage[]; total: number }> => {
      const url = forceRefresh
        ? '/markets/packages?forceRefresh=true'
        : '/markets/packages';
      return this.request(url, 'GET');
    },
    refreshPackages: (): Promise<{ message: string }> =>
      this.request('/markets/packages/refresh', 'POST'),
  };

  // Settings API
  settings = {
    list: (query?: SettingsListQuery): Promise<SettingsListResponse> => {
      const params = new URLSearchParams();
      if (query?.search) params.append('search', query.search);
      if (query?.page) params.append('page', query.page.toString());
      if (query?.pageSize) params.append('page_size', query.pageSize.toString());

      const url = params.toString() ? `/settings?${params.toString()}` : '/settings';
      return this.request(url, 'GET');
    },
    get: (id: string): Promise<Settings> =>
      this.request(`/settings/${id}`, 'GET'),
    create: async (formData: FormData): Promise<Settings> => {
      const response = await this.client.post('/settings', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      return response.data;
    },
    delete: (id: string): Promise<void> =>
      this.request(`/settings/${id}`, 'DELETE'),
    getBoundAgents: (id: string): Promise<AgentConfig[]> =>
      this.request(`/settings/${id}/agents`, 'GET'),
    readDirectory: (id: string, subPath?: string): Promise<any> => {
      const url = subPath
        ? `/settings/${id}/directory?path=${encodeURIComponent(subPath)}`
        : `/settings/${id}/directory`;
      return this.request(url, 'GET');
    },
    readFile: async (id: string, filePath: string): Promise<string> => {
      const response = await this.client.get(`/settings/${id}/file`, {
        params: { path: filePath },
      });
      return response.data;
    },
    // Agent-Settings 绑定
    getAgentSettings: (agentId: string): Promise<AgentSettingsResponse> =>
      this.request(`/agent-roles/${agentId}/settings`, 'GET'),
    bindToAgent: (agentId: string, settingsIds: string[]): Promise<void> =>
      this.request(`/agent-roles/${agentId}/settings`, 'POST', { settingsIds }),
    unbindFromAgent: (agentId: string, settingsId: string): Promise<void> =>
      this.request(`/agent-roles/${agentId}/settings/${settingsId}`, 'DELETE'),
  };

  // System API
  system = {
    // 获取版本信息（动态获取，避免浏览器缓存问题）
    getVersion: async (): Promise<{
      version: string;
      baseVersion: string;
    }> => {
      const response = await this.client.get('/system/version');
      return response.data;
    },
  };

  // Governance API（治理规则热更新）
  governance = {
    getDigest: (): Promise<{ content: string; status: Record<string, unknown> }> =>
      this.request('/governance/digest', 'GET'),
    updateDigest: (content: string): Promise<{ message: string; status: Record<string, unknown> }> =>
      this.request('/governance/digest', 'PUT', { content }),
    getStatus: (): Promise<Record<string, unknown>> =>
      this.request('/governance/digest/status', 'GET'),
  };

  // Dashboard API（首页统计）
  dashboard = {
    getStats: (): Promise<{
      totalProjects: number;
      activeThreads: number;
      workflowTeams: number;
      agentRoles: number;
      todayAgentInteractions: Array<{
        agentId: string;
        agentName: string;
        count: number;
        isDefault: boolean;
      }>;
      todayWorkflowUsage: Array<{
        workflowId: string;
        workflowName: string;
        count: number;
        isSystem: boolean;
      }>;
      totalSkills: number;
      totalCommands: number;
      totalSubagents: number;
      totalRules: number;
    }> => this.request('/dashboard/stats', 'GET'),
    getWorkflowsWithAssets: (): Promise<Array<{
      id: string;
      name: string;
      description: string;
      isSystem: boolean;
      agentCount: number;
      agents: Array<{
        id: string;
        name: string;
        role: string;
        skillsCount: number;
        commandsCount: number;
        subagentsCount: number;
        rulesCount: number;
      }>;
      skills: number;
      commands: number;
      subagents: number;
      rules: number;
      totalAssets: number;
      isActive: boolean;
    }>> => this.request('/dashboard/workflows-with-assets', 'GET'),
    getActiveThreads: (): Promise<Array<{
      id: string;
      projectId: string;
      name: string;
      status: string;
      currentPhase: string;
      currentAgentNames: string[];
      workflowTemplateId: string;
      createdAt: string;
      updatedAt: string;
      projectName: string;
      workflowName: string;
    }>> => this.request('/dashboard/active-threads', 'GET'),
    getRecentThreads: (): Promise<Array<{
      id: string;
      projectId: string;
      name: string;
      status: string;
      currentPhase: string;
      workflowTemplateId: string;
      updatedAt: string;
      projectName: string;
      teamName: string;
    }>> => this.request('/dashboard/recent-threads', 'GET'),
  };
}

export const api = new APIClient();
export default api;
