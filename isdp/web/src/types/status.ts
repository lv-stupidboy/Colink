// Status panel types for ThreadView right-side status display

export interface TokenUsage {
  inputTokens?: number;
  outputTokens?: number;
  cacheReadTokens?: number;
  cacheCreationTokens?: number;
  costUsd?: number;
  durationApiMs?: number;
  durationMs?: number;
  numTurns?: number;
}

export interface TaskProgress {
  snapshotStatus: 'completed' | 'interrupted' | 'running';
  tasks: TaskItem[];
}

export interface TaskItem {
  id: string;
  title: string;
  status: 'completed' | 'in_progress' | 'pending';
}

export interface MessageStats {
  total: number;
  agent: number;
  system: number;
  user: number;
}