import { invoke } from '@tauri-apps/api/core';
import type { RunningAgentInstance } from './types';

export const serviceApi = {
  start: async (): Promise<{ success: boolean; error?: string }> => {
    return invoke('start_service');
  },

  stop: async (): Promise<{ success: boolean }> => {
    return invoke('stop_service');
  },

  getStatus: async (): Promise<{ status: 'running' | 'stopped' }> => {
    return invoke('get_service_status');
  },

  getRunningAgents: async (): Promise<{
    instances: RunningAgentInstance[];
  }> => {
    return invoke('get_running_agents');
  },
};