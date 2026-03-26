import { app, BrowserWindow, ipcMain, dialog, shell } from 'electron'
import { join } from 'path'
import { ServiceManager } from './service-manager'
import { getInstalledVersion } from './shared/install-utils'
import { createMainWindow, showCloseConfirm } from './shared/window-utils'

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

  await serviceManager.start()
  return { success: true }
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
    const installed = getInstalledVersion()

    if (!installed.installed || !installed.installDir) {
      dialog.showErrorBox('错误', 'ISDP 未安装，请先运行安装程序')
      app.quit()
      return
    }

    installDir = installed.installDir
    serviceManager = new ServiceManager(installDir)

    mainWindow = createMainWindow(isDev)

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
        mainWindow = createMainWindow(isDev)
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