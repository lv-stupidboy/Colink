import { contextBridge, ipcRenderer } from 'electron'

// 暴露安全的 API 给渲染进程
contextBridge.exposeInMainWorld('electronAPI', {
  // 窗口控制
  minimizeWindow: () => ipcRenderer.send('window-minimize'),
  closeWindow: () => ipcRenderer.send('window-close'),

  // 路径获取
  getAppPath: () => ipcRenderer.invoke('get-app-path'),
  getResourcePath: () => ipcRenderer.invoke('get-resource-path'),

  // 安装相关
  selectDirectory: () => ipcRenderer.invoke('select-directory'),
  getDiskSpace: (path: string) => ipcRenderer.invoke('get-disk-space', path),
  checkDependency: (dep: string) => ipcRenderer.invoke('check-dependency', dep),
  installDependency: (dep: string) => ipcRenderer.invoke('install-dependency', dep),
  startInstallation: (config: object) => ipcRenderer.invoke('start-installation', config),
  copyFiles: (src: string, dest: string) => ipcRenderer.invoke('copy-files', src, dest),
  generateConfig: (config: object) => ipcRenderer.invoke('generate-config', config),
  testDatabaseConnection: (config: object) => ipcRenderer.invoke('test-database-connection', config),
  createShortcut: (path: string) => ipcRenderer.invoke('create-shortcut', path),

  // 进度回调
  onInstallProgress: (callback: (progress: any) => void) => {
    ipcRenderer.on('install-progress', (_event, progress) => callback(progress))
  }
})