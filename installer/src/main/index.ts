import { app, BrowserWindow, ipcMain } from 'electron'
import { join } from 'path'
import {
  checkDependency,
  installNpmPackage,
  copyApplicationFiles,
  generateConfigFile,
  createDesktopShortcut
} from './installer'
import mysql from 'mysql2/promise'

// 判断是否为开发模式
const isDev = !app.isPackaged

let mainWindow: BrowserWindow | null = null

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 900,
    height: 600,
    minWidth: 800,
    minHeight: 500,
    frame: false,  // 无边框窗口
    resizable: true,
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  })

  // 开发模式加载本地服务器，生产模式加载打包文件
  if (isDev) {
    mainWindow.loadURL('http://localhost:5173')
    mainWindow.webContents.openDevTools()
  } else {
    mainWindow.loadFile(join(__dirname, '../renderer/index.html'))
  }
}

app.whenReady().then(() => {
  createWindow()

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow()
    }
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

// IPC 处理：窗口控制
ipcMain.on('window-minimize', () => mainWindow?.minimize())
ipcMain.on('window-close', () => mainWindow?.close())

// IPC 处理：获取应用路径
ipcMain.handle('get-app-path', () => app.getAppPath())

// IPC 处理：获取资源路径
ipcMain.handle('get-resource-path', () => {
  return isDev
    ? join(__dirname, '../../resources')
    : process.resourcesPath
})

// IPC: 依赖检测
ipcMain.handle('check-dependency', async (_event, key: string) => {
  return checkDependency(key)
})

// IPC: 安装依赖
ipcMain.handle('install-dependency', async (_event, key: string) => {
  const packages: Record<string, string> = {
    claude: '@anthropic-ai/claude-cli',
    opencode: '@anthropic-ai/opencode',
  }
  if (packages[key]) {
    return installNpmPackage(packages[key])
  }
  return { success: false, error: '未知的依赖' }
})

// IPC: 复制文件
ipcMain.handle('copy-files', async (_event, src: string, dest: string) => {
  return copyApplicationFiles(src, dest)
})

// IPC: 生成配置
ipcMain.handle('generate-config', async (_event, data: { path: string; content: string }) => {
  return generateConfigFile(data.path, data.content)
})

// IPC: 数据库连接测试
ipcMain.handle('test-database-connection', async (_event, config: any) => {
  try {
    const connection = await mysql.createConnection({
      host: config.host,
      port: config.port,
      user: config.username,
      password: config.password,
      database: config.database,
      connectTimeout: 5000,
    })
    await connection.ping()
    await connection.end()
    return { success: true }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '连接失败',
    }
  }
})

// IPC: 创建快捷方式
ipcMain.handle('create-shortcut', async (_event, path: string) => {
  return { success: await createDesktopShortcut(path) }
})