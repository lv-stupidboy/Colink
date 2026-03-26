import { app, BrowserWindow, ipcMain, dialog, shell } from 'electron'
import { join, dirname } from 'path'
import { execSync, spawn, exec } from 'child_process'
import { existsSync, readdirSync, rmSync } from 'fs'
import { promisify } from 'util'
import {
  checkDependency,
  installNpmPackage,
  generateConfigFile,
  createDesktopShortcut,
  runInstallation,
  readExistingConfig,
  copyApplicationFiles,
  writeRegistry,
  deleteRegistry
} from './installer'
import { ServiceManager } from './service-manager'
import mysql from 'mysql2/promise'

const execAsync = promisify(exec)

const isDev = !app.isPackaged

let mainWindow: BrowserWindow | null = null
let serviceManager: ServiceManager | null = null
let installDir: string = ''

// 检测已安装的ISDP版本
export function getInstalledVersion(): { installed: boolean; installDir?: string; version?: string; hasData?: boolean } {
  try {
    let regQuery: string
    try {
      regQuery = execSync(
        'reg query "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v InstallLocation 2>nul',
        { encoding: 'utf8' }
      )
    } catch {
      regQuery = execSync(
        'reg query "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v InstallLocation 2>nul',
        { encoding: 'utf8' }
      )
    }
    const match = regQuery.match(/InstallLocation\s+REG_SZ\s+(.+)/)
    if (match) {
      const dir = match[1].trim()
      const dataDir = join(dir, 'data')
      const hasData = existsSync(dataDir) && readdirSync(dataDir).length > 0
      return { installed: true, installDir: dir, hasData }
    }
  } catch {
    // 未安装
  }
  return { installed: false }
}

// 获取当前exe所在目录
function getExeDir(): string {
  return dirname(process.execPath)
}

// 创建主窗口
function createWindow() {
  // 如果窗口已存在，显示并聚焦
  if (mainWindow) {
    mainWindow.show()
    mainWindow.focus()
    return
  }

  mainWindow = new BrowserWindow({
    width: 900,
    height: 650,
    minWidth: 800,
    minHeight: 550,
    frame: false,
    resizable: true,
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  })

  if (isDev) {
    mainWindow.loadURL('http://localhost:5173')
    mainWindow.webContents.openDevTools()
  } else {
    mainWindow.loadFile(join(__dirname, '../renderer/index.html'))
  }

  // 关闭窗口时弹出确认对话框
  mainWindow.on('close', (event) => {
    // 如果应用正在退出，直接关闭窗口
    if (app.isQuitting) {
      return
    }

    const choice = dialog.showMessageBoxSync(mainWindow!, {
      type: 'question',
      buttons: ['取消', '确认关闭'],
      defaultId: 1,
      cancelId: 0,
      title: '关闭 ISDP',
      message: '关闭 ISDP 控制面板？',
      detail: '后端服务将继续在后台运行。您可以通过桌面快捷方式重新打开控制面板。'
    })

    if (choice === 0) {
      event.preventDefault()
    }
  })
}

// 初始化服务管理器
function initServiceManager() {
  const installed = getInstalledVersion()
  if (installed.installed && installed.installDir) {
    installDir = installed.installDir
    serviceManager = new ServiceManager(installDir)
  }
}

// 停止所有ISDP相关进程
async function stopAllProcesses(): Promise<void> {
  // 停止服务管理器
  if (serviceManager) {
    await serviceManager.stop()
    serviceManager = null
  }

  // 强制结束isdp-server.exe进程
  try {
    execSync('taskkill /f /im isdp-server.exe 2>nul', { encoding: 'utf8' })
  } catch {
    // 忽略错误
  }
}

// ==================== IPC 处理 ====================

ipcMain.on('window-minimize', () => mainWindow?.minimize())
ipcMain.on('window-close', () => mainWindow?.close())

ipcMain.handle('get-app-path', () => app.getAppPath())

ipcMain.handle('get-resource-path', () => {
  return isDev ? join(__dirname, '../../resources') : process.resourcesPath
})

ipcMain.handle('check-installed', async () => {
  return getInstalledVersion()
})

ipcMain.handle('check-dependency', async (_event, key: string) => {
  return checkDependency(key)
})

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

ipcMain.handle('select-directory', async () => {
  const result = await dialog.showOpenDialog(mainWindow!, {
    properties: ['openDirectory'],
    defaultPath: 'C:\\Program Files',
  })
  return result.canceled ? null : result.filePaths[0]
})

ipcMain.handle('get-disk-space', async (_event, path: string) => {
  try {
    const drive = path.substring(0, 2)
    const output = execSync(`wmic logicaldisk where "DeviceID='${drive}'" get FreeSpace,Size /format:csv`, { encoding: 'utf8' })
    const lines = output.trim().split('\n')
    const data = lines[1].split(',')
    return { free: parseInt(data[1]) || 0, total: parseInt(data[2]) || 0 }
  } catch {
    return { free: 0, total: 0 }
  }
})

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
    return { success: false, error: error instanceof Error ? error.message : '连接失败' }
  }
})

ipcMain.handle('read-existing-config', async (_event, dir: string) => {
  return readExistingConfig(dir)
})

ipcMain.handle('start-installation', async (_event, config) => {
  // 安装前停止所有进程
  await stopAllProcesses()

  const sourceDir = getExeDir()
  const resourcePath = isDev ? join(__dirname, '../../resources') : process.resourcesPath
  return runInstallation(config, resourcePath, mainWindow!, sourceDir)
})

ipcMain.handle('create-shortcut', async (_event, path: string) => {
  return { success: await createDesktopShortcut(path) }
})

ipcMain.handle('start-service', async () => {
  const installed = getInstalledVersion()
  if (!installed.installed || !installed.installDir) {
    return { success: false, error: '未安装' }
  }

  if (!serviceManager) {
    serviceManager = new ServiceManager(installed.installDir)
  }

  await serviceManager.start()
  return { success: true }
})

ipcMain.handle('stop-service', async () => {
  await stopAllProcesses()
  return { success: true }
})

ipcMain.handle('get-service-status', async () => {
  return { status: serviceManager?.getStatus() || 'stopped' }
})

// 卸载前确认
ipcMain.handle('confirm-uninstall', async () => {
  const installed = getInstalledVersion()
  if (!installed.installed || !installed.installDir) {
    return { confirmed: false }
  }

  const result = await dialog.showMessageBox(mainWindow!, {
    type: 'warning',
    buttons: ['取消', '卸载'],
    defaultId: 0,
    cancelId: 0,
    title: '卸载 ISDP',
    message: '确定要卸载 ISDP 吗？',
    detail: installed.hasData
      ? '检测到数据目录，卸载后可以选择保留或删除。'
      : '卸载后将无法使用本软件。',
    checkboxLabel: installed.hasData ? '保留数据目录' : undefined,
    checkboxChecked: true,
  })

  return {
    confirmed: result.response === 1,
    keepData: installed.hasData ? result.checkboxChecked : false
  }
})

ipcMain.handle('uninstall', async (_event, keepData: boolean) => {
  const installed = getInstalledVersion()
  if (!installed.installed || !installed.installDir) {
    return { success: false, error: '未安装' }
  }

  try {
    // 停止所有进程
    await stopAllProcesses()

    // 删除注册表
    deleteRegistry()

    // 删除快捷方式
    const desktopPath = process.env.USERPROFILE + '\\Desktop\\ISDP.lnk'
    const startMenuPath = process.env.APPDATA + '\\Microsoft\\Windows\\Start Menu\\Programs\\ISDP.lnk'
    try { if (existsSync(desktopPath)) rmSync(desktopPath) } catch {}
    try { if (existsSync(startMenuPath)) rmSync(startMenuPath) } catch {}

    // 删除文件
    const dir = installed.installDir
    const entries = ['ISDP.exe', 'isdp-server.exe', 'web', 'resources']
    for (const entry of entries) {
      const path = join(dir, entry)
      if (existsSync(path)) {
        try {
          rmSync(path, { recursive: true, force: true })
        } catch (e) {
          console.error('[Uninstall] Failed to remove:', path, e)
        }
      }
    }

    // 删除数据目录
    if (!keepData) {
      const dataDir = join(dir, 'data')
      if (existsSync(dataDir)) {
        try {
          rmSync(dataDir, { recursive: true, force: true })
        } catch (e) {
          console.error('[Uninstall] Failed to remove data:', dataDir, e)
        }
      }
    }

    installDir = ''
    serviceManager = null

    // 显示卸载完成提示
    dialog.showMessageBox(mainWindow!, {
      type: 'info',
      title: '卸载完成',
      message: 'ISDP 已卸载',
      detail: keepData
        ? `数据目录已保留：${dir}\\data\n\n请手动删除安装目录：${dir}`
        : `请手动删除安装目录：${dir}`,
    })

    return { success: true }
  } catch (error) {
    console.error('[Uninstall] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '卸载失败' }
  }
})

ipcMain.handle('open-logs', async () => {
  const installed = getInstalledVersion()
  if (installed.installDir) {
    shell.openPath(join(installed.installDir, 'data', 'logs'))
  }
})

ipcMain.handle('open-data-dir', async () => {
  const installed = getInstalledVersion()
  if (installed.installDir) {
    shell.openPath(join(installed.installDir, 'data'))
  }
})

ipcMain.handle('open-config', async () => {
  const installed = getInstalledVersion()
  if (installed.installDir) {
    shell.openPath(join(installed.installDir, 'data', 'configs'))
  }
})

// ==================== 应用启动 ====================

// 单实例锁定：如果已有实例运行，激活它并退出
const gotTheLock = app.requestSingleInstanceLock()

if (!gotTheLock) {
  // 已有实例运行，退出
  app.quit()
} else {
  app.on('second-instance', () => {
    // 有人尝试运行第二个实例，激活当前实例
    mainWindow?.show()
    mainWindow?.focus()
  })

  app.whenReady().then(() => {
    createWindow()
    initServiceManager()

    app.on('activate', () => {
      if (BrowserWindow.getAllWindows().length === 0) {
        createWindow()
      }
    })
  })

  app.on('before-quit', () => {
    app.isQuitting = true
    serviceManager?.stop()
  })
}

// 扩展 app 类型
declare module 'electron' {
  interface App {
    isQuitting?: boolean
  }
}