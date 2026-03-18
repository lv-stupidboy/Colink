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
  camelToSnake,
} from './transform';

class APIClient {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: '/api/v1',
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // 请求拦截器 - 转换 camelCase 为 snake_case
    this.client.interceptors.request.use(
      (config) => {
        // 添加认证 token
        const token = localStorage.getItem('token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        // 转换请求数据
        if (config.data && typeof config.data === 'object') {
          const originalData = config.data;
          config.data = camelToSnake(config.data);
          console.log('[DEBUG] Request interceptor - original:', originalData, 'transformed:', config.data);
        }
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
        if (url.includes('/projects')) {
          result = Array.isArray(result) ? transformProjects(result) : transformProject(result);
        } else if (url.includes('/threads')) {
          result = Array.isArray(result) ? transformThreads(result) : transformThread(result);
        } else if (url.includes('/messages')) {
          result = Array.isArray(result) ? transformMessages(result) : transformMessage(result);
        } else if (url.includes('/agents')) {
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
      console.error('[DEBUG] API error:', error);
      console.error('[DEBUG] Error response:', error.response?.data);
      console.error('[DEBUG] Error status:', error.response?.status);
      if (error.response?.status === 401) {
        localStorage.removeItem('token');
        window.location.href = '/login';
      }
      throw error;
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

  // Thread API
  threads = {
    list: (projectId: string): Promise<Thread[]> => this.request(`/threads/project/${projectId}`, 'GET'),
    get: (id: string): Promise<Thread> => this.request(`/threads/${id}`, 'GET'),
    create: (projectId: string): Promise<Thread> => this.request(`/threads/project/${projectId}`, 'POST'),
    updateStatus: (id: string, status: string): Promise<Thread> =>
      this.request(`/threads/${id}/status`, 'PUT', { status }),
    setPhase: (id: string, phase: string, agent: string): Promise<Thread> =>
      this.request(`/threads/${id}/phase`, 'PUT', { phase, agent }),
  };

  // 消息 API
  messages = {
    list: (threadId: string, limit = 50): Promise<Message[]> =>
      this.request(`/messages/thread/${threadId}`, 'GET', undefined, { params: { limit } }),
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
    // 预创建调试Thread，前端先调用此方法获取threadId，建立WebSocket连接
    createDebugThread: (projectPath?: string): Promise<{ threadId: string }> =>
      this.request('/agents/debug/thread', 'POST', { projectPath }),
    // 调试Agent，threadId可选（如果已预创建则传入）
    debug: (id: string, input: string, projectPath?: string, threadId?: string): Promise<{ invocationId: string; threadId: string; output: string; sandboxUrl?: string }> =>
      this.request(`/agents/${id}/debug`, 'POST', { input, projectPath, threadId }),
    continueDebug: (threadId: string, message: string): Promise<{ status: string }> =>
      this.request(`/agents/debug/${threadId}/continue`, 'POST', { message }),
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
  };

  // Agent 调用 API
  invocations = {
    list: (threadId: string): Promise<AgentInvocation[]> =>
      this.request(`/threads/${threadId}/invocations`, 'GET'),
    get: (id: string): Promise<AgentInvocation> => this.request(`/invocations/${id}`, 'GET'),
    spawn: (threadId: string, role: string, input: string, configId?: string): Promise<AgentInvocation> => {
      const payload = { role, input, configId };
      console.log('[DEBUG] spawn request payload:', payload);
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

  // 工作流模板 API
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
}

export const api = new APIClient();
export default api;
