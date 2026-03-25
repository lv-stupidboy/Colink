import { app, shell } from 'electron'
import { join } from 'path'
import { createTray } from './tray'
import { ServiceManager } from './service-manager'

const isDev = !app.isPackaged

// 启动器模式：不创建窗口，只显示托盘
async function runLauncher() {
  // 获取安装目录（启动器位于安装目录根目录）
  const installDir = isDev ? process.cwd() : join(app.getAppPath(), '../')

  // 创建服务管理器
  const serviceManager = new ServiceManager(installDir)

  // 创建托盘
  createTray(installDir, serviceManager)

  // 自动启动服务
  await serviceManager.start()

  // 打开浏览器
  shell.openExternal('http://localhost:8080')
}

// Electron 应用初始化
app.whenReady().then(() => {
  runLauncher()
})

app.on('window-all-closed', () => {
  // 启动器模式下，关闭窗口不应该退出应用
})

export { runLauncher }