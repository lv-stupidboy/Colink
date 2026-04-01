// 文件路径: isdp/web/src/api/teamPackage.ts
import client from './client';

export interface TeamPackagePreview {
  workflow: { name: string; exists: boolean };
  roles: Array<{ name: string; exists: boolean; localId?: string }>;
  assets: {
    skills: Array<{ name: string; exists: boolean }>;
    commands: Array<{ name: string; exists: boolean }>;
    subagents: Array<{ name: string; exists: boolean }>;
    rules: Array<{ name: string; exists: boolean }>;
    settings: Array<{ name: string; exists: boolean }>;
  };
}

export interface ImportConfirm {
  mode: 'overwrite' | 'skip' | 'selective';
  workflowAction: 'overwrite' | 'skip';
  roleActions: Array<{ name: string; action: 'overwrite' | 'skip' }>;
  assetActions: Array<{ assetType: string; name: string; action: 'overwrite' | 'skip' }>;
}

export const teamPackageApi = {
  import: async (file: File): Promise<TeamPackagePreview> => {
    const formData = new FormData();
    formData.append('file', file);
    const response = await client.post('/api/team-packages/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return response.data;
  },

  importConfirm: async (file: File, confirm: ImportConfirm): Promise<any> => {
    const formData = new FormData();
    formData.append('file', file);
    const blob = new Blob([JSON.stringify(confirm)], { type: 'application/json' });
    formData.append('confirm', blob);
    const response = await client.post('/api/team-packages/import/confirm', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return response.data;
  },

  export: async (workflowId: string): Promise<Blob> => {
    const response = await client.post('/api/team-packages/export', { workflowId }, {
      responseType: 'blob',
    });
    return response.data;
  },

  getWorkflows: async (): Promise<any[]> => {
    const response = await client.get('/api/workflows');
    return response.data.data || [];
  },
};

export default teamPackageApi;