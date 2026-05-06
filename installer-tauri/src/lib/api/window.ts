import { invoke } from '@tauri-apps/api/core';

export type CloseResult = 'allowClose' | 'blockClose';

export const windowApi = {
  minimize: async (): Promise<void> => {
    await invoke('window_minimize');
  },

  maximize: async (): Promise<void> => {
    await invoke('window_maximize');
  },

  close: async (): Promise<void> => {
    const result = await invoke<CloseResult>('window_close_with_confirm');
    if (result === 'allowClose') {
      await invoke('window_close');
    }
  },
};