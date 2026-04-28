export const launcherApi = {
  openLogs: async (): Promise<void> => {
    const { invoke } = await import('@tauri-apps/api/core');
    return invoke('open_logs');
  },

  openDataDir: async (): Promise<void> => {
    const { invoke } = await import('@tauri-apps/api/core');
    return invoke('open_data_dir');
  },

  openConfig: async (): Promise<void> => {
    const { invoke } = await import('@tauri-apps/api/core');
    return invoke('open_config');
  },

  openConsole: async (): Promise<void> => {
    const { invoke } = await import('@tauri-apps/api/core');
    return invoke('open_console');
  },

  openInstallDir: async (installDir?: string): Promise<void> => {
    const { invoke } = await import('@tauri-apps/api/core');
    return invoke('open_install_dir', { installDir });
  },
};