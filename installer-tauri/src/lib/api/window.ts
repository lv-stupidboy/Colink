import { invoke } from '@tauri-apps/api/core';

export const windowApi = {
  minimize: async (): Promise<void> => {
    await invoke('window_minimize');
  },

  maximize: async (): Promise<void> => {
    await invoke('window_maximize');
  },

  close: async (): Promise<void> => {
    await invoke('window_close');
  },
};