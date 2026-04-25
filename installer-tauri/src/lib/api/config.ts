import { invoke } from '@tauri-apps/api/core';
import type { AppConfig } from './types';

export const configApi = {
  readConfigFile: async (): Promise<{
    success: boolean;
    content?: string;
    error?: string;
  }> => {
    return invoke('read_config_file');
  },

  saveConfig: async (
    yaml: string
  ): Promise<{ success: boolean; error?: string }> => {
    return invoke('save_config', { yaml });
  },

  getExistingConfig: async (): Promise<{
    success: boolean;
    config?: AppConfig;
    error?: string;
  }> => {
    return invoke('get_existing_config');
  },
};