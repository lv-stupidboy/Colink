import { BrowserWindow, dialog, app } from 'electron'
import { join } from 'path'

export interface CloseConfirmOptions {
  checkServiceRunning: () => boolean
  stopService: () => Promise<void>
}

/**
 * 显示关闭确认对话框
 * - 服务运行时：三选项（取消/停止并关闭/保持运行并关闭）
 * - 服务停止时：二选项（取消/确认关闭）
 */
export async function showCloseConfirm(
  mainWindow: BrowserWindow,
  options: CloseConfirmOptions
): Promise<boolean> {
  const isRunning = options.checkServiceRunning()

  if (isRunning) {
    const choice = await dialog.showMessageBox(mainWindow, {
      type: 'question',
      buttons: ['取消', '停止服务并关闭', '保持运行并关闭'],
      defaultId: 2,
      cancelId: 0,
      title: '关闭 ISDP',
      message: '服务正在运行',
      detail: '请选择关闭方式：'
    })

    if (choice.response === 0) {
      return false // 取消关闭
    }
    if (choice.response === 1) {
      // 停止服务并关闭
      await options.stopService()
    }
    // response === 2: 保持运行并关闭
    return true
  } else {
    // 服务未运行时，简单确认
    const choice = dialog.showMessageBoxSync(mainWindow, {
      type: 'question',
      buttons: ['取消', '确认关闭'],
      defaultId: 1,
      cancelId: 0,
      title: '关闭 ISDP',
      message: '关闭 ISDP 控制面板？',
      detail: '后端服务将继续在后台运行。'
    })

    return choice !== 0
  }
}

/**
 * 创建主窗口
 */
export function createMainWindow(isDev: boolean): BrowserWindow {
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
    mainWindow.loadURL('http://localhost:5173')
    mainWindow.webContents.openDevTools()
  } else {
    mainWindow.loadFile(join(__dirname, '../renderer/index.html'))
  }

  return mainWindow
}