// Settings API client wrapper
import api from './client';
import type {
  Settings,
  SettingsListQuery,
  SettingsListResponse,
  AgentSettingsResponse,
} from '@/types';

// Settings API - 使用 api client 的 settings 方法
export const settingsApi = {
  // 获取Settings列表
  list: (query?: SettingsListQuery): Promise<SettingsListResponse> =>
    api.settings.list(query),

  // 获取单个Settings详情
  get: (id: string): Promise<Settings> =>
    api.settings.get(id),

  // 创建Settings（目录上传）
  create: (formData: FormData): Promise<Settings> =>
    api.settings.create(formData),

  // 删除Settings
  delete: (id: string): Promise<void> =>
    api.settings.delete(id),

  // 绑定Settings到Agent角色
  bindToAgent: (agentId: string, settingsIds: string[]): Promise<void> =>
    api.settings.bindToAgent(agentId, settingsIds),

  // 解绑Settings
  unbindFromAgent: (agentId: string, settingsId: string): Promise<void> =>
    api.settings.unbindFromAgent(agentId, settingsId),

  // 获取Agent绑定的Settings
  getAgentSettings: (agentId: string): Promise<AgentSettingsResponse> =>
    api.settings.getAgentSettings(agentId),

  // 获取Settings绑定的Agents
  getBoundAgents: (id: string): Promise<any[]> =>
    api.settings.getBoundAgents(id),

  // 读取Settings目录内容
  readDirectory: (id: string, subPath?: string): Promise<any> =>
    api.settings.readDirectory(id, subPath),

  // 读取Settings目录中的文件内容
  readFile: (id: string, filePath: string): Promise<string> =>
    api.settings.readFile(id, filePath),
};

export default settingsApi;