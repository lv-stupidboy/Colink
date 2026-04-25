import { invoke } from '@tauri-apps/api/core';
import type { DependencyInfo } from './types';

export const dependencyApi = {
  check: async (key: string): Promise<DependencyInfo> => {
    return invoke('check_dependency', { key });
  },

  install: async (
    key: string
  ): Promise<{ success: boolean; error?: string }> => {
    return invoke('install_dependency', { key });
  },

  checkAll: async (): Promise<DependencyInfo[]> => {
    return invoke('check_all_dependencies');
  },
};