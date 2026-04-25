import { invoke } from '@tauri-apps/api/core';

export const modeApi = {
  isLauncherMode: async (): Promise<boolean> => {
    return invoke('is_launcher_mode');
  },

  getStartupAction: async (): Promise<'install' | 'upgrade' | 'uninstall'> => {
    return invoke('get_startup_action');
  },

  getAppPath: async (): Promise<string> => {
    return invoke('get_app_path');
  },

  getInstallDir: async (): Promise<string | null> => {
    return invoke('get_install_dir');
  },

  getResourcePath: async (): Promise<string> => {
    return invoke('get_resource_path');
  },

  getVersion: async (): Promise<string> => {
    return invoke('get_version');
  },
};