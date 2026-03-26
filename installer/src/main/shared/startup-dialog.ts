import { dialog } from 'electron'

export type StartupAction = 'upgrade' | 'uninstall' | 'cancel'

export interface InstalledInfo {
  installDir?: string
  version?: string
}

/**
 * 显示启动操作选择对话框
 * 当检测到已安装时，让用户选择：升级、卸载、或取消
 */
export async function showStartupActionDialog(
  installed: InstalledInfo
): Promise<StartupAction> {
  const choice = await dialog.showMessageBox({
    type: 'question',
    buttons: ['升级', '卸载', '取消'],
    defaultId: 0,
    cancelId: 2,
    title: 'ISDP Setup',
    message: '检测到已安装的 ISDP',
    detail: `安装位置: ${installed.installDir || '未知'}\n\n请选择要执行的操作：`
  })

  const actions: StartupAction[] = ['upgrade', 'uninstall', 'cancel']
  return actions[choice.response]
}