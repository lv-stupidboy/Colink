import { BrowserWindow, dialog } from 'electron'
import { join } from 'path'

export interface CloseConfirmOptions {
  checkServiceRunning: () => boolean
  stopService: () => Promise<void>
}

/**
 * 显示关闭确认对话框
 * - 服务运行时：必须先停止服务才能关闭
 * - 服务停止时：直接关闭
 */
export async function showCloseConfirm(
  mainWindow: BrowserWindow,
  options: CloseConfirmOptions
): Promise<boolean> {
  const isRunning = options.checkServiceRunning()

  if (isRunning) {
    // 服务运行时，必须先停止
    const choice = await dialog.showMessageBox(mainWindow, {
      type: 'warning',
      buttons: ['停止服务并关闭', '取消'],
      defaultId: 0,
      cancelId: 1,
      title: '无法关闭',
      message: '服务正在运行',
      detail: '请先停止服务后再关闭窗口。'
    })

    if (choice.response === 0) {
      // 停止服务并关闭
      await options.stopService()
      return true
    }
    return false
  }

  return true
}

/**
 * 创建主窗口
 */
export function createMainWindow(isDev: boolean): BrowserWindow {
  console.log('[Window] Creating main window')
  console.log('[Window] isDev:', isDev)
  console.log('[Window] __dirname:', __dirname)
  console.log('[Window] process.resourcesPath:', process.resourcesPath)

  const mainWindow = new BrowserWindow({
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
    console.log('[Window] Loading dev URL: http://localhost:5173')
    mainWindow.loadURL('http://localhost:5173')
    mainWindow.webContents.openDevTools()
  } else {
    const rendererPath = join(__dirname, '../renderer/index.html')
    console.log('[Window] Loading file:', rendererPath)
    mainWindow.loadFile(rendererPath).catch(err => {
      console.error('[Window] Failed to load:', err)
    })
  }

  mainWindow.webContents.on('did-finish-load', () => {
    console.log('[Window] Page loaded successfully')
  })

  mainWindow.webContents.on('did-fail-load', (event, errorCode, errorDescription) => {
    console.error('[Window] Page load failed:', errorCode, errorDescription)
  })

  return mainWindow
}