import { app, BrowserWindow, ipcMain, dialog, shell, Menu } from 'electron'
import { join, dirname } from 'path'
import { execSync } from 'child_process'
import { existsSync, readdirSync, rmSync } from 'fs'
import { promises as fsPromises } from 'fs'
import https from 'https'
import http from 'http'
const readFile = fsPromises.readFile
import {
  checkDependency,
  installNpmPackage,
  generateConfigPreview,
  writeConfigFile,
  createDesktopShortcut,
  runInstallation,
  readExistingConfig,
  copyApplicationFiles,
  writeRegistry,
  deleteRegistry
} from './installer'
import { ServiceManager } from './service-manager'
import { showCloseConfirm } from './shared/window-utils'
import { getInstalledVersion, getOldISDPVersion, uninstallOldISDP } from './shared/install-utils'
import mysql from 'mysql2/promise'

const isDev = !app.isPackaged

// 安装器配置
interface InstallerConfig {
  verificationApiUrl?: string
}

let installerConfig: InstallerConfig = {}

// 读取安装器配置文件
function loadInstallerConfig(): InstallerConfig {
  try {
    const configPath = isDev
      ? join(__dirname, '../../resources/installer-config.json')
      : join(process.resourcesPath, 'installer-config.json')

    if (existsSync(configPath)) {
      const content = require('fs').readFileSync(configPath, 'utf-8')
      return JSON.parse(content)
    }
  } catch (e) {
    console.warn('[Installer] Failed to load installer config:', e)
  }
  return {}
}

let mainWindow: BrowserWindow | null = null
let serviceManager: ServiceManager | null = null
let installDir: string = ''
let startupAction: 'install' | 'upgrade' | 'uninstall' = 'install'

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
    const rendererPath = join(__dirname, '../renderer/index.html')
    mainWindow.loadFile(rendererPath).catch(err => {
      console.error('[Setup] Failed to load renderer:', err)
      const { dialog } = require('electron')
      dialog.showErrorBox('加载失败', `无法加载界面：${err.message}`)
    })
  }

  // 按 F12 打开开发者工具（用于调试）
  mainWindow.webContents.on('before-input-event', (event, input) => {
    if (input.key === 'F12') {
      mainWindow?.webContents.toggleDevTools()
    }
  })

  // 右键菜单支持（粘贴、复制、剪切、全选）
  mainWindow.webContents.on('context-menu', (event, params) => {
    const menu = Menu.buildFromTemplate([
      { label: '粘贴', role: 'paste', enabled: params.editFlags.canPaste },
      { label: '复制', role: 'copy', enabled: params.editFlags.canCopy },
      { label: '剪切', role: 'cut', enabled: params.editFlags.canCut },
      { type: 'separator' },
      { label: '全选', role: 'selectAll' }
    ])
    menu.popup(mainWindow!)
  })

  // 关闭窗口时弹出确认对话框
  mainWindow.on('close', async (event) => {
    // 如果应用正在退出，直接关闭窗口
    if (app.isQuitting) {
      return
    }

    event.preventDefault()
    const canClose = await showCloseConfirm(mainWindow!, {
      checkServiceRunning: () => serviceManager?.getStatus() === 'running',
      stopService: async () => { await stopServiceProcess() }
    })

    if (canClose) {
      mainWindow?.destroy()
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

// 停止后端服务进程（用于启动器页面"停止服务"按钮）
async function stopServiceProcess(): Promise<void> {
  // 停止服务管理器
  if (serviceManager) {
    await serviceManager.stop()
    serviceManager = null
  }

  // 强制结束服务进程
  try {
    execSync('taskkill /f /im colink-server.exe 2>nul', { encoding: 'utf8' })
  } catch {
    // 忽略错误
  }
}

// 停止所有相关进程（用于卸载流程）
async function stopAllProcessesForUninstall(): Promise<void> {
  // 停止服务管理器
  if (serviceManager) {
    await serviceManager.stop()
    serviceManager = null
  }

  // 强制结束所有相关进程
  const processesToKill = ['Colink.exe', 'colink-server.exe']
  for (const proc of processesToKill) {
    try {
      execSync(`taskkill /f /im ${proc} 2>nul`, { encoding: 'utf8' })
    } catch {
      // 忽略错误
    }
  }

  // 等待进程退出
  await new Promise(resolve => setTimeout(resolve, 2000))
}

// 检查进程是否还在运行
function checkProcessRunning(processName: string): boolean {
  try {
    const output = execSync(`tasklist /fi "imagename eq ${processName}" /fo csv`, { encoding: 'utf8' })
    // CSV格式: "Image Name","PID","Session Name","Session#","Mem Usage"
    // 如果进程不存在，只返回一行标题
    const lines = output.trim().split('\n')
    return lines.length > 1
  } catch {
    return false
  }
}

// 检查所有相关进程是否都已退出
function checkAllProcessesStopped(): string[] {
  const processesToCheck = ['Colink.exe', 'colink-server.exe']
  const stillRunning: string[] = []
  for (const proc of processesToCheck) {
    if (checkProcessRunning(proc)) {
      stillRunning.push(proc)
    }
  }
  return stillRunning
}

// ==================== IPC 处理 ====================

ipcMain.on('window-minimize', () => mainWindow?.minimize())
ipcMain.on('window-maximize', () => {
  if (mainWindow?.isMaximized()) {
    mainWindow.unmaximize()
  } else {
    mainWindow?.maximize()
  }
})
ipcMain.on('window-close', () => mainWindow?.close())

ipcMain.handle('is-launcher-mode', () => false)

ipcMain.handle('get-startup-action', () => startupAction)

ipcMain.handle('get-app-path', () => app.getAppPath())

ipcMain.handle('get-resource-path', () => {
  return isDev ? join(__dirname, '../../resources') : process.resourcesPath
})

ipcMain.handle('check-installed', async () => {
  return getInstalledVersion()
})

ipcMain.handle('check-old-isdp', async () => {
  return getOldISDPVersion()
})

ipcMain.handle('uninstall-old-isdp', async () => {
  // 先停止所有进程
  await stopAllProcessesForUninstall()
  return uninstallOldISDP()
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
    // 提取驱动器字母（支持 D: 或 D:\ 格式）
    const drive = path.substring(0, 2).toUpperCase()
    if (!drive.match(/^[A-Z]:$/)) {
      return { free: 0, total: 0 }
    }

    // 方法1: 使用 PowerShell (推荐，Windows 10+)
    try {
      const output = execSync(
        `powershell -Command "(Get-PSDrive -Name '${drive.charAt(0)}').Free;(Get-PSDrive -Name '${drive.charAt(0)}').Used"`,
        { encoding: 'utf8', timeout: 5000 }
      )
      const lines = output.trim().split('\n')
      const free = parseInt(lines[0].trim()) || 0
      const used = parseInt(lines[1].trim()) || 0
      if (free > 0 || used > 0) {
        return { free, total: free + used }
      }
    } catch {
      // PowerShell 失败，尝试 wmic
    }

    // 方法2: 回退到 wmic (兼容旧系统)
    const output = execSync(
      `wmic logicaldisk where "DeviceID='${drive}'" get FreeSpace,Size /format:csv`,
      { encoding: 'utf8', timeout: 5000 }
    )
    const lines = output.trim().split('\n')

    // CSV 格式: Node,FreeSpace,Size
    for (let i = 1; i < lines.length; i++) {
      const line = lines[i].trim()
      if (line) {
        const data = line.split(',')
        if (data.length >= 3) {
          const free = parseInt(data[1]) || 0
          const total = parseInt(data[2]) || 0
          if (free > 0 || total > 0) {
            return { free, total }
          }
        }
      }
    }

    return { free: 0, total: 0 }
  } catch {
    return { free: 0, total: 0 }
  }
})

ipcMain.handle('test-database-connection', async (_event, config: any) => {
  try {
    // SQLite 模式不需要测试连接
    if (config.type === 'sqlite') {
      return { success: true, message: 'SQLite 无需测试连接' }
    }

    // MySQL 模式：测试连接
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

// 验证邀请码
// 验证 API URL 从配置文件读取
function getVerificationApiUrl(): string {
  if (!installerConfig.verificationApiUrl) {
    throw new Error('验证服务地址未配置，请在 installer-config.json 中设置 verificationApiUrl')
  }
  return installerConfig.verificationApiUrl
}

// 获取系统用户名
ipcMain.handle('get-system-username', () => {
  return process.env.USERNAME || process.env.USER || 'unknown'
})

ipcMain.handle('verify-invite-code', async (_event, request: { code: string; username: string }) => {
  return new Promise((resolve) => {
    try {
      const postData = JSON.stringify({
        code: request.code,
        username: request.username
      })

      const apiUrl = getVerificationApiUrl()
      console.log('[VerifyInviteCode] Using API URL:', apiUrl)
      const urlObj = new URL(apiUrl)
      const isHttps = urlObj.protocol === 'https:'
      const client = isHttps ? https : http

      const options = {
        hostname: urlObj.hostname,
        port: urlObj.port || (isHttps ? 443 : 80),
        path: urlObj.pathname,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Content-Length': Buffer.byteLength(postData)
        },
        timeout: 15000 // 15秒超时
      }

      const req = client.request(options, (res) => {
        let data = ''
        res.on('data', chunk => data += chunk)
        res.on('end', () => {
          try {
            const result = JSON.parse(data)
            resolve(result)
          } catch (e) {
            resolve({
              success: false,
              message: '响应解析失败，请稍后重试'
            })
          }
        })
      })

      req.on('error', (e) => {
        console.error('[VerifyInviteCode] Request error:', e)
        resolve({
          success: false,
          message: `网络错误: ${e.message}`
        })
      })

      req.on('timeout', () => {
        req.destroy()
        resolve({
          success: false,
          message: '请求超时，请检查网络连接'
        })
      })

      req.write(postData)
      req.end()
    } catch (e) {
      resolve({
        success: false,
        message: e instanceof Error ? e.message : '验证服务配置错误'
      })
    }
  })
})

ipcMain.handle('read-existing-config', async (_event, dir: string) => {
  return readExistingConfig(dir)
})

ipcMain.handle('generate-config-preview', async (_event, params: {
  installDir?: string
  database: { type: 'sqlite' | 'mysql'; host?: string; port?: number; database?: string; username?: string; password?: string }
  serverPort?: number
}) => {
  return generateConfigPreview(params)
})

ipcMain.handle('start-installation', async (_event, config) => {
  const sourceDir = getExeDir()
  const resourcePath = isDev ? join(__dirname, '../../resources') : process.resourcesPath

  // 获取当前版本和新版本
  const installed = getInstalledVersion()
  const currentVersion = installed.version || '0.0.0'
  const newVersion = getPackageVersion()

  // 传递版本信息
  const configWithVersion = {
    ...config,
    currentVersion,
    newVersion
  }

  const result = await runInstallation(configWithVersion, resourcePath, mainWindow!, sourceDir)

  // 数据库变更提示已通过进度信息发送，无需额外弹框
  return result
})

// 获取当前包版本
function getPackageVersion(): string {
  try {
    // 优先从 VERSION 文件读取
    const versionPath = isDev
      ? join(__dirname, '../../VERSION')
      : join(process.resourcesPath, 'runtime/VERSION')

    if (existsSync(versionPath)) {
      const content = require('fs').readFileSync(versionPath, 'utf-8')
      return content.trim() || '1.0.0'
    }

    // fallback: 从 package.json 读取
    const packagePath = isDev
      ? join(__dirname, '../../package.json')
      : join(process.resourcesPath, 'app.asar', 'package.json')

    if (existsSync(packagePath)) {
      const content = require('fs').readFileSync(packagePath, 'utf-8')
      const pkg = JSON.parse(content)
      return pkg.version || '1.0.0'
    }
  } catch {}
  return '1.0.0'
}

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

  return serviceManager.start()
})

ipcMain.handle('stop-service', async () => {
  await stopServiceProcess()
  return { success: true }
})

ipcMain.handle('get-service-status', async () => {
  return { status: serviceManager?.getStatus() || 'stopped' }
})

ipcMain.handle('get-running-agents', async () => {
  const installed = getInstalledVersion()

  // 端口获取：优先从配置文件读取，否则使用默认端口
  let port = 26305
  const dir = installed.installDir || installDir

  if (dir) {
    try {
      const configPath = join(dir, 'data', 'configs', 'config.yaml')
      if (existsSync(configPath)) {
        const content = await readFile(configPath, 'utf-8')
        const portMatch = content.match(/port:\s*(\d+)/)
        if (portMatch) {
          port = parseInt(portMatch[1])
        }
      }
    } catch (e) {
      // ignore
    }
  }

  // 使用http模块调用API
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

// 卸载前确认
ipcMain.handle('confirm-uninstall', async () => {
  const installed = getInstalledVersion()
  if (!installed.installed || !installed.installDir) {
    return { confirmed: false }
  }

  // 只有当 hasData 为 true 时才显示复选框
  if (installed.hasData) {
    const result = await dialog.showMessageBox(mainWindow!, {
      type: 'warning',
      buttons: ['取消', '卸载'],
      defaultId: 0,
      cancelId: 0,
      title: '卸载 Colink',
      message: '确定要卸载 Colink 吗？',
      detail: '检测到数据目录，卸载后可以选择保留或删除。',
      checkboxLabel: '保留数据目录',
      checkboxChecked: true,
    })

    return {
      confirmed: result.response === 1,
      keepData: result.checkboxChecked || false
    }
  } else {
    // 没有数据目录，显示简单确认
    const result = await dialog.showMessageBox(mainWindow!, {
      type: 'warning',
      buttons: ['取消', '卸载'],
      defaultId: 0,
      cancelId: 0,
      title: '卸载 Colink',
      message: '确定要卸载 Colink 吗？',
      detail: '卸载后将无法使用本软件。',
    })

    return {
      confirmed: result.response === 1,
      keepData: false
    }
  }
})

ipcMain.handle('uninstall', async (_event, keepData: boolean) => {
  const installed = getInstalledVersion()
  if (!installed.installed || !installed.installDir) {
    return { success: false, error: '未安装' }
  }

  try {
    // 停止所有进程
    await stopAllProcessesForUninstall()

    // 检查进程是否真的退出
    const stillRunning = checkAllProcessesStopped()
    if (stillRunning.length > 0) {
      const errorMsg = `以下进程仍在运行，无法卸载：\n${stillRunning.map(p => `- ${p}`).join('\n')}\n\n请手动关闭这些进程后重试。`
      dialog.showMessageBox(mainWindow!, {
        type: 'error',
        title: '卸载失败',
        message: '进程仍在运行',
        detail: errorMsg,
      })
      return { success: false, error: errorMsg }
    }

    // 删除注册表
    deleteRegistry()

    // 删除快捷方式
    const desktopPath = process.env.USERPROFILE + '\\Desktop\\Colink.lnk'
    const startMenuPath = process.env.APPDATA + '\\Microsoft\\Windows\\Start Menu\\Programs\\Colink.lnk'
    try { if (existsSync(desktopPath)) rmSync(desktopPath) } catch {}
    try { if (existsSync(startMenuPath)) rmSync(startMenuPath) } catch {}

    // 同时删除旧版 ISDP 的快捷方式（兼容升级）
    const oldDesktopPath = process.env.USERPROFILE + '\\Desktop\\ISDP.lnk'
    const oldStartMenuPath = process.env.APPDATA + '\\Microsoft\\Windows\\Start Menu\\Programs\\ISDP.lnk'
    try { if (existsSync(oldDesktopPath)) rmSync(oldDesktopPath) } catch {}
    try { if (existsSync(oldStartMenuPath)) rmSync(oldStartMenuPath) } catch {}

    // 白名单模式删除目录（只保留 data）
    const dir = installed.installDir
    const whitelist = keepData ? ['data'] : []
    const entries = readdirSync(dir, { withFileTypes: true })
    const entriesToDelete = entries.filter(e => !whitelist.includes(e.name))

    const failedEntries: string[] = []
    for (const entry of entriesToDelete) {
      const path = join(dir, entry.name)
      try {
        rmSync(path, { recursive: true, force: true })
        console.log('[Uninstall] Removed:', entry.name)
      } catch (e) {
        console.error('[Uninstall] Failed to remove:', path, e)
        failedEntries.push(entry.name)
      }
    }

    // 如果有删除失败的，报错
    if (failedEntries.length > 0) {
      const errorMsg = `以下文件删除失败：${failedEntries.join(', ')}\n请手动关闭相关程序后重试`
      dialog.showMessageBox(mainWindow!, {
        type: 'error',
        title: '卸载失败',
        message: '部分文件无法删除',
        detail: errorMsg,
      })
      return { success: false, error: errorMsg }
    }

    installDir = ''
    serviceManager = null

    // 显示卸载完成提示
    dialog.showMessageBox(mainWindow!, {
      type: 'info',
      title: '卸载完成',
      message: 'Colink 已卸载',
      detail: keepData
        ? `数据目录已保留：${dir}\\data`
        : '所有文件已删除',
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

ipcMain.handle('open-console', async () => {
  const installed = getInstalledVersion()
  let port = 8080

  // 尝试从配置文件读取端口
  if (installed.installDir) {
    try {
      const configPath = join(installed.installDir, 'data', 'configs', 'config.yaml')
      if (existsSync(configPath)) {
        const content = await readFile(configPath, 'utf-8')
        const portMatch = content.match(/port:\s*(\d+)/)
        if (portMatch) {
          port = parseInt(portMatch[1])
        }
      }
    } catch (e) {
      console.warn('[OpenConsole] Failed to read config:', e)
    }
  }

  shell.openExternal(`http://localhost:${port}`)
})

// ==================== 应用启动 ====================

// 设置应用名称，确保与 Launcher 的单实例锁独立
app.setName('Colink Setup')

// 单实例锁定：Setup 只允许一个实例运行
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

  app.whenReady().then(async () => {
    // 加载安装器配置
    installerConfig = loadInstallerConfig()
    console.log('[Installer] Loaded config:', installerConfig)

    // 检测是否已安装（前端会处理选项展示）
    const installed = getInstalledVersion()
    startupAction = installed.installed ? 'upgrade' : 'install'

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