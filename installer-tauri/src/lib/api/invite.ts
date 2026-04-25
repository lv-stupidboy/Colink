import { invoke } from '@tauri-apps/api/core';
import type { InviteVerificationResponse } from './types';

export const inviteApi = {
  verify: async (
    code: string,
    username: string
  ): Promise<InviteVerificationResponse> => {
    return invoke('verify_invite_code', { code, username });
  },

  save: async (
    inviteCode: string,
    installDir?: string
  ): Promise<{ success: boolean; message?: string }> => {
    return invoke('save_invite_code', { inviteCode, installDir });
  },

  load: async (
    installDir?: string
  ): Promise<{ success: boolean; inviteCode?: string; message?: string }> => {
    return invoke('load_invite_code', { installDir });
  },

  getSystemUsername: async (): Promise<string> => {
    return invoke('get_system_username');
  },
};