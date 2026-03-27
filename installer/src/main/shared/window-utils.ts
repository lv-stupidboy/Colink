import { BrowserWindow, dialog } from 'electron'

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
      await options.stopService()
      return true
    }
    return false
  }

  return true
}