import { invoke } from '@tauri-apps/api/core';
import { listen, type UnlistenFn } from '@tauri-apps/api/event';
import type {
  InstallConfig,
  InstallProgress,
  InstallResult,
  InstalledVersion,
  DiskSpace,
} from './types';

export const installApi = {
  checkInstalled: async (): Promise<InstalledVersion> => {
    return invoke('check_installed');
  },

  checkOldISDP: async (): Promise<InstalledVersion> => {
    return invoke('check_old_isdp');
  },

  uninstallOldISDP: async (): Promise<{ success: boolean; error?: string }> => {
    return invoke('uninstall_old_isdp');
  },

  selectDirectory: async (defaultPath?: string): Promise<string | null> => {
    return invoke('select_directory', { defaultPath });
  },

  getDiskSpace: async (path: string): Promise<DiskSpace> => {
    return invoke('get_disk_space', { path });
  },

  generateConfigPreview: async (params: {
    installDir?: string;
    database?: { type: string };
    serverPort?: number;
  }): Promise<{ success: boolean; yaml?: string; error?: string }> => {
    return invoke('generate_config_preview', {
      installDir: params.installDir,
      database: params.database,
      serverPort: params.serverPort,
    });
  },

  readExistingConfig: async (
    installDir: string
  ): Promise<{ success: boolean; config?: unknown; error?: string }> => {
    return invoke('read_existing_config', { installDir });
  },

  startInstallation: async (config: InstallConfig): Promise<InstallResult> => {
    return invoke('start_installation', { config });
  },

  createShortcut: async (path: string): Promise<{ success: boolean }> => {
    return invoke('create_shortcut', { path });
  },

  onInstallProgress: (
    callback: (progress: InstallProgress) => void
  ): Promise<UnlistenFn> => {
    return listen<InstallProgress>('install-progress', (event) => {
      callback(event.payload);
    });
  },
};