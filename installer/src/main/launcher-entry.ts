import { app, BrowserWindow, ipcMain, dialog, shell, Menu } from 'electron'
import { join } from 'path'
import { existsSync, readFileSync } from 'fs'
import http from 'http'
import { exec } from 'child_process'
import { ServiceManager } from './service-manager'
import { getInstalledVersion } from './shared/install-utils'
import { showCloseConfirm } from './shared/window-utils'
import { checkDependency, installNpmPackage, generateConfigPreview, writeConfigFile, readExistingConfig, readConfigFile } from './installer'

const isDev = !app.isPackaged

let mainWindow: BrowserWindow | null = null
let serviceManager: ServiceManager | null = null
let installDir: string = ''

// ==================== IPC 处理器 ====================

ipcMain.on('window-minimize', () => mainWindow?.minimize())
ipcMain.on('window-maximize', () => {
  if (mainWindow?.isMaximized()) {
    mainWindow.unmaximize()
  } else {
    mainWindow?.maximize()
  }
})
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

ipcMain.handle('get-running-agents', async () => {
  // 端口获取：优先从配置文件读取，否则使用默认端口26305
  let port = 26305
  const dir = installDir

  if (dir) {
    try {
      const configPath = join(dir, 'data', 'configs', 'config.yaml')
      if (existsSync(configPath)) {
        const content = readFileSync(configPath, 'utf-8')
        const portMatch = content.match(/port:\s*(\d+)/)
        if (portMatch) {
          port = parseInt(portMatch[1])
        }
      }
    } catch {
      // ignore
    }
  }

  // 调用 Go API
  return new Promise((resolve) => {
    const req = http.request({
      hostname: 'localhost',
      port: port,
      path: '/api/v1/invocations/running',
      method: 'GET',
      timeout: 5000
    }, (res) => {
      let data = ''
      res.on('data', chunk => data += chunk)
      res.on('end', () => {
        try {
          const result = JSON.parse(data)
          resolve(result)
        } catch {
          resolve({ instances: [] })
        }
      })
    })
    req.on('error', () => {
      resolve({ instances: [] })
    })
    req.on('timeout', () => {
      req.destroy()
      resolve({ instances: [] })
    })
    req.end()
  })
})

ipcMain.handle('open-logs', async () => {
  if (installDir) {
    const logsDir = join(installDir, 'data', 'logs')
    const logFile = join(logsDir, 'colink.log')

    // 使用 PowerShell 的 Get-Content -Wait 实现 tail -f 功能
    // 启动一个新的 PowerShell 窗口，显示日志最后 100 行并自动跟随新内容
    if (existsSync(logFile)) {
      const { spawn } = require('child_process')
      spawn('powershell', [
        '-Command',
        `Get-Content -Path '${logFile}' -Wait -Tail 100`
      ], {
        detached: true,
        stdio: 'ignore'
      })
    } else {
      // 日志文件不存在，打开日志目录
      shell.openPath(logsDir)
    }
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
  let port = 8080

  // 尝试从配置文件读取端口
  if (installDir) {
    try {
      const configPath = join(installDir, 'data', 'configs', 'config.yaml')
      if (existsSync(configPath)) {
        const content = readFileSync(configPath, 'utf-8')
        const portMatch = content.match(/port:\s*(\d+)/)
        if (portMatch) {
          port = parseInt(portMatch[1])
        }
      }
    } catch (e) {
      console.warn('[Launcher] Failed to read config port:', e)
    }
  }

  const url = `http://localhost:${port}`
  // 使用系统命令打开浏览器，绕过 Electron shell API 的 chrome_elf.dll 问题
  if (process.platform === 'win32') {
    exec(`start "" "${url}"`)
  } else if (process.platform === 'darwin') {
    exec(`open "${url}"`)
  } else {
    exec(`xdg-open "${url}"`)
  }
})

// ==================== 依赖管理 ====================

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

ipcMain.handle('check-all-dependencies', async () => {
  const deps = ['claude', 'opencode']
  const results = []
  for (const dep of deps) {
    const result = await checkDependency(dep)
    results.push({
      key: dep,
      name: dep === 'claude' ? 'Claude CLI' : 'OpenCode',
      installed: result.installed,
      version: result.version
    })
  }
  return results
})

// ==================== 配置编辑 ====================

ipcMain.handle('read-config-file', async () => {
  const configPath = join(installDir, 'data', 'configs', 'config.yaml')
  return readConfigFile(configPath)
})

ipcMain.handle('get-config-preview', async () => {
  // 读取现有配置，生成预览
  const existingResult = await readExistingConfig(installDir)
  if (existingResult.success && existingResult.config) {
    return generateConfigPreview({
      installDir,
      database: existingResult.config.database,
      serverPort: existingResult.config.serverPort
    })
  }
  return { success: false, error: '读取配置失败' }
})

ipcMain.handle('save-config', async (_event, yaml: string) => {
  const configPath = join(installDir, 'data', 'configs', 'config.yaml')
  return writeConfigFile(configPath, yaml)
})

ipcMain.handle('get-existing-config', async () => {
  return readExistingConfig(installDir)
})

// ==================== 创建窗口 ====================

function createLauncherWindow(): BrowserWindow {
  const window = new BrowserWindow({
    width: 900,
    height: 650,
    minWidth: 800,
    minHeight: 550,
    frame: false,
    resizable: true,
    show: false,
    icon: isDev ? undefined : join(process.resourcesPath, 'icon.ico'),
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  })

  if (isDev) {
    window.loadURL('http://localhost:5173')
    window.webContents.openDevTools()
  } else {
    const rendererPath = join(__dirname, '../renderer/index.html')
    window.loadFile(rendererPath).catch(err => {
      console.error('[Launcher] Failed to load file:', err)
      dialog.showErrorBox('加载失败', `无法加载界面：${err.message}`)
    })
  }

  // 页面加载完成后显示窗口
  window.webContents.on('did-finish-load', () => {
    window.show()
    window.focus()
  })

  // 按 F12 打开开发者工具（用于调试）
  window.webContents.on('before-input-event', (event, input) => {
    if (input.key === 'F12') {
      window.webContents.toggleDevTools()
    }
  })

  // 右键菜单支持（粘贴、复制、剪切、全选）
  window.webContents.on('context-menu', (event, params) => {
    const menu = Menu.buildFromTemplate([
      { label: '粘贴', role: 'paste', enabled: params.editFlags.canPaste },
      { label: '复制', role: 'copy', enabled: params.editFlags.canCopy },
      { label: '剪切', role: 'cut', enabled: params.editFlags.canCut },
      { type: 'separator' },
      { label: '全选', role: 'selectAll' }
    ])
    menu.popup(window)
  })

  return window
}

// ==================== 应用启动 ====================

// 设置应用名称，确保与 Setup 的单实例锁独立
app.setName('Colink')

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
      dialog.showErrorBox('错误', 'Colink 未安装，请先运行安装程序')
      app.quit()
      return
    }

    installDir = installed.installDir
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
        app.isQuitting = true  // 设置标志，避免再次触发 close 事件
        app.quit()  // 退出应用，而非只销毁窗口
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