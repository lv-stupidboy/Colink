export type RepoStatus = 'pending' | 'ready' | 'syncing' | 'error';

export interface LocalRepo {
  id: string;
  name: string;
  gitUrl: string;
  localPath: string;
  branch?: string;
  lastCommit?: string;
  status: RepoStatus;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
}

export interface RemoteBranch {
  name: string;
  type: string;
}

export interface BrowsePathEntry {
  name: string;
  path: string;
  isDir: boolean;
}

export interface BrowsePathResponse {
  currentPath: string;
  parentPath: string;
  isValid: boolean;
  error?: string;
  entries: BrowsePathEntry[];
  drives?: string[];
}

export interface CloneRepoRequest {
  gitUrl: string;
  branch: string;
  name?: string;
  targetPath: string;
}

export interface GitConfigRequest {
  gitUrl: string;
  branch: string;
}
