import { app, shell } from 'electron'
import { join, dirname } from 'path'
import { createTray } from './tray'
import { ServiceManager } from './service-manager'

const isDev = !app.isPackaged

// 启动器模式：不创建窗口，只显示托盘
async function runLauncher() {
  // 获取安装目录
  // 当启动器作为portable exe运行时，app.getAppPath()返回app.asar的路径
  // 需要向上两级才能到达安装目录根目录
  let installDir: string
  if (isDev) {
    installDir = process.cwd()
  } else {
    // app.getAppPath() = 安装目录/resources/app.asar
    // dirname两次得到安装目录
    installDir = dirname(dirname(app.getAppPath()))
  }

  console.log('ISDP Launcher starting...')
  console.log('Install directory:', installDir)

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