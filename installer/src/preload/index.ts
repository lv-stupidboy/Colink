import { contextBridge, ipcRenderer } from 'electron'

// 暴露安全的 API 给渲染进程
contextBridge.exposeInMainWorld('electronAPI', {
  // 窗口控制
  minimizeWindow: () => ipcRenderer.send('window-minimize'),
  closeWindow: () => ipcRenderer.send('window-close'),

  // 应用模式
  isLauncherMode: () => ipcRenderer.invoke('is-launcher-mode'),
  getStartupAction: () => ipcRenderer.invoke('get-startup-action'),

  // 路径获取
  getAppPath: () => ipcRenderer.invoke('get-app-path'),
  getResourcePath: () => ipcRenderer.invoke('get-resource-path'),

  // 安装相关
  selectDirectory: () => ipcRenderer.invoke('select-directory'),
  getDiskSpace: (path: string) => ipcRenderer.invoke('get-disk-space', path),
  checkDependency: (dep: string) => ipcRenderer.invoke('check-dependency', dep),
  installDependency: (dep: string) => ipcRenderer.invoke('install-dependency', dep),
  startInstallation: (config: object) => ipcRenderer.invoke('start-installation', config),
  generateConfigPreview: (params: {
    installDir?: string
    database: { type: 'sqlite' | 'mysql'; host?: string; port?: number; database?: string; username?: string; password?: string }
    serverPort?: number
  }) => ipcRenderer.invoke('generate-config-preview', params),
  testDatabaseConnection: (config: object) => ipcRenderer.invoke('test-database-connection', config),
  createShortcut: (path: string) => ipcRenderer.invoke('create-shortcut', path),

  // 邀请码验证
  verifyInviteCode: (request: { code: string; username: string }) =>
    ipcRenderer.invoke('verify-invite-code', request),

  // 获取系统用户名
  getSystemUsername: () => ipcRenderer.invoke('get-system-username'),

  // 安装状态
  checkInstalled: () => ipcRenderer.invoke('check-installed'),
  checkOldISDP: () => ipcRenderer.invoke('check-old-isdp'),
  uninstallOldISDP: () => ipcRenderer.invoke('uninstall-old-isdp'),
  readExistingConfig: (installDir: string) => ipcRenderer.invoke('read-existing-config', installDir),

  // 服务管理
  startService: () => ipcRenderer.invoke('start-service'),
  stopService: () => ipcRenderer.invoke('stop-service'),
  getServiceStatus: () => ipcRenderer.invoke('get-service-status'),

  // 快捷操作
  openLogs: () => ipcRenderer.invoke('open-logs'),
  openDataDir: () => ipcRenderer.invoke('open-data-dir'),
  openConfig: () => ipcRenderer.invoke('open-config'),
  openConsole: () => ipcRenderer.invoke('open-console'),

  // 卸载
  confirmUninstall: () => ipcRenderer.invoke('confirm-uninstall'),
  uninstall: (keepData: boolean) => ipcRenderer.invoke('uninstall', keepData),

  // 进度回调
  onInstallProgress: (callback: (progress: any) => void) => {
    ipcRenderer.on('install-progress', (_event, progress) => callback(progress))
  }
})