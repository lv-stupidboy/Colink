import { app, BrowserWindow, ipcMain, dialog, shell } from 'electron'
import { join } from 'path'
import { ServiceManager } from './service-manager'
import { getInstalledVersion } from './shared/install-utils'
import { showCloseConfirm } from './shared/window-utils'

const isDev = !app.isPackaged

let mainWindow: BrowserWindow | null = null
let serviceManager: ServiceManager | null = null
let installDir: string = ''

// ==================== IPC 处理器 ====================

ipcMain.on('window-minimize', () => mainWindow?.minimize())
ipcMain.on('window-close', () => mainWindow?.close())

ipcMain.handle('is-launcher-mode', () => true)

ipcMain.handle('get-app-path', () => app.getAppPath())

ipcMain.handle('get-resource-path', () => {
  return isDev ? join(__dirname, '../../resources') : process.resourcesPath
})

ipcMain.handle('check-installed', async () => {
  return getInstalledVersion()
})

ipcMain.handle('start-service', async () => {
  if (!serviceManager) {
    return { success: false, error: '服务管理器未初始化' }
  }

  return serviceManager.start()
})

ipcMain.handle('stop-service', async () => {
  if (serviceManager) {
    await serviceManager.stop()
  }
  return { success: true }
})

ipcMain.handle('get-service-status', async () => {
  return { status: serviceManager?.getStatus() || 'stopped' }
})

ipcMain.handle('open-logs', async () => {
  if (installDir) {
    shell.openPath(join(installDir, 'data', 'logs'))
  }
})

ipcMain.handle('open-data-dir', async () => {
  if (installDir) {
    shell.openPath(join(installDir, 'data'))
  }
})

ipcMain.handle('open-config', async () => {
  if (installDir) {
    shell.openPath(join(installDir, 'data', 'configs'))
  }
})

ipcMain.handle('open-console', async () => {
  shell.openExternal('http://localhost:8080')
})

// ==================== 创建窗口 ====================

function createLauncherWindow(): BrowserWindow {
  console.log('[Launcher] Creating window')
  console.log('[Launcher] isDev:', isDev)
  console.log('[Launcher] __dirname:', __dirname)
  console.log('[Launcher] resourcesPath:', process.resourcesPath)

  const window = new BrowserWindow({
    width: 900,
    height: 650,
    minWidth: 800,
    minHeight: 550,
    frame: false,
    resizable: true,
    icon: isDev ? undefined : join(process.resourcesPath, 'icon.ico'),
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  })

  if (isDev) {
    console.log('[Launcher] Loading dev URL')
    window.loadURL('http://localhost:5173')
    window.webContents.openDevTools()
  } else {
    const rendererPath = join(__dirname, '../renderer/index.html')
    console.log('[Launcher] Loading file:', rendererPath)

    window.loadFile(rendererPath).catch(err => {
      console.error('[Launcher] Failed to load file:', err)
      dialog.showErrorBox('加载失败', `无法加载界面：${err.message}`)
    })
  }

  window.webContents.on('did-finish-load', () => {
    console.log('[Launcher] Page loaded successfully')
  })

  window.webContents.on('did-fail-load', (event, errorCode, errorDescription) => {
    console.error('[Launcher] Page load failed:', errorCode, errorDescription)
  })

  return window

  // 开发模式下打开开发者工具
  if (isDev) {
    window.webContents.openDevTools()
  }

  return window
}

// ==================== 应用启动 ====================

// 单实例锁定
const gotTheLock = app.requestSingleInstanceLock()

if (!gotTheLock) {
  app.quit()
} else {
  app.on('second-instance', () => {
    mainWindow?.show()
    mainWindow?.focus()
  })

  app.whenReady().then(async () => {
    console.log('[Launcher] App ready')

    const installed = getInstalledVersion()
    console.log('[Launcher] Installed version:', installed)

    if (!installed.installed || !installed.installDir) {
      dialog.showErrorBox('错误', 'ISDP 未安装，请先运行安装程序')
      app.quit()
      return
    }

    installDir = installed.installDir
    console.log('[Launcher] Install dir:', installDir)

    serviceManager = new ServiceManager(installDir)

    mainWindow = createLauncherWindow()

    // 关闭确认
    mainWindow.on('close', async (event) => {
      if (app.isQuitting) return

      event.preventDefault()
      const canClose = await showCloseConfirm(mainWindow!, {
        checkServiceRunning: () => serviceManager?.getStatus() === 'running',
        stopService: async () => { await serviceManager?.stop() }
      })

      if (canClose) {
        mainWindow?.destroy()
      }
    })

    app.on('activate', () => {
      if (BrowserWindow.getAllWindows().length === 0) {
        mainWindow = createLauncherWindow()
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