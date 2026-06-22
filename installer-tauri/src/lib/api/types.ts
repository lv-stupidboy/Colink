export interface InstallConfig {
  installDir: string;
  installMode: string;
  installType?: 'fresh' | 'upgrade' | 'reinstall';
  oldInstallDir?: string;
  dependencies?: DependencyStatus[];
  database: { type: string };
  serverPort?: number;
  webPort?: number;
  createShortcut: boolean;
  launchNow?: boolean;
  keepData?: boolean;
  configYaml?: string;
  currentVersion?: string;
  newVersion?: string;
}

export interface DependencyStatus {
  key: string;
  installed: boolean;
}

export interface InstallProgress {
  step: string;
  status: 'pending' | 'running' | 'success' | 'failed' | 'warning';
  progress?: number;
  message?: string;
  details?: string;
}

export interface InstallResult {
  success: boolean;
  error?: string;
  dbChanges?: DbChange[];
}

export interface DbChange {
  version: string;
  files: string[];
}

export interface InstalledVersion {
  installed: boolean;
  installDir?: string;
  version?: string;
  hasData?: boolean;
}

export interface DiskSpace {
  free: number;
  total: number;
}

export interface DependencyInfo {
  key: string;
  name: string;
  installed: boolean;
  version?: string;
  /** 检测失败时的诊断信息（PATH、exit code、stderr 摘要等）。安装成功/未安装都不会有。 */
  detectError?: string;
  /** 下载页面 URL（仅 code-agent 等只能手动下载的依赖）。
   *  非空时 UI 显示"前往下载"按钮，点击在外部浏览器打开。 */
  downloadUrl?: string;
}

export interface RunningAgentInstance {
  invocationId: string;
  agentName: string;
  projectName: string;
  threadTitle: string;
  startedAt: string;
  runningDurationSeconds: number;
}

export interface AppConfig {
  server: { port: number; host: string };
  database: {
    type: string;
    path?: string;
    host?: string;
    port?: number;
    name?: string;
    user?: string;
    password?: string;
  };
  log: { level: string; path?: string };
  auth?: { invite_code?: string };
}