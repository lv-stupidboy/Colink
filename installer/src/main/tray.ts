import { Tray, Menu, nativeImage, app, shell } from 'electron'
import { join } from 'path'
import { ServiceManager } from './service-manager'

export function createTray(installDir: string, serviceManager: ServiceManager): Tray {
  const iconPath = join(__dirname, '../../resources/icon.ico')
  const icon = nativeImage.createFromPath(iconPath)

  const tray = new Tray(icon)
  tray.setToolTip('ISDP 智能开发平台')

  const updateMenu = () => {
    const status = serviceManager.getStatus()
    const contextMenu = Menu.buildFromTemplate([
      {
        label: '打开控制台',
        click: () => shell.openExternal('http://localhost:8080')
      },
      {
        label: `服务状态: ${status === 'running' ? '运行中' : '已停止'}`,
        enabled: false
      },
      { type: 'separator' },
      {
        label: '查看日志',
        click: () => shell.openPath(join(installDir, 'logs'))
      },
      {
        label: '重启服务',
        click: async () => {
          await serviceManager.restart()
          updateMenu()
        }
      },
      { type: 'separator' },
      {
        label: '退出',
        click: async () => {
          await serviceManager.stop()
          app.quit()
        }
      }
    ])
    tray.setContextMenu(contextMenu)
  }

  updateMenu()
  return tray
}