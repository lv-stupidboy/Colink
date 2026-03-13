import axios, { AxiosInstance } from 'axios';
import type {
  Project,
  Thread,
  Message,
  AgentConfig,
  AgentInvocation,
  Artifact,
  MergeCheckResult
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
          config.data = camelToSnake(config.data);
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
    create: (threadId: string, content: string): Promise<Message> =>
      this.request(`/messages/thread/${threadId}`, 'POST', { content }),
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
  };

  // Agent 调用 API
  invocations = {
    list: (threadId: string): Promise<AgentInvocation[]> =>
      this.request(`/threads/${threadId}/invocations`, 'GET'),
    get: (id: string): Promise<AgentInvocation> => this.request(`/invocations/${id}`, 'GET'),
    spawn: (threadId: string, role: string, input: string, configId?: string): Promise<AgentInvocation> =>
      this.request(`/threads/${threadId}/invocations`, 'POST', { role, input, configId }),
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
  };
}

export const api = new APIClient();
export default api;
