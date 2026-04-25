import { invoke } from '@tauri-apps/api/core';

export const uninstallApi = {
  confirm: async (): Promise<{ confirmed: boolean; keepData: boolean }> => {
    return invoke('confirm_uninstall');
  },

  runUninstall: async (params: {
    installDir: string;
    keepData: boolean;
  }): Promise<{ success: boolean; error?: string }> => {
    return invoke('run_uninstall', params);
  },

  cleanRegistry: async (): Promise<{ success: boolean }> => {
    return invoke('clean_registry');
  },

  removeShortcuts: async (): Promise<{ success: boolean }> => {
    return invoke('remove_shortcuts');
  },

  execute: async (
    keepData: boolean
  ): Promise<{ success: boolean; error?: string }> => {
    return invoke('uninstall', { keepData });
  },
};