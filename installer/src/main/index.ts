import { app, BrowserWindow, ipcMain, dialog, shell } from 'electron'
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
  deleteRegistry,
  checkProcessesRunning
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

  // 关闭窗口时弹出确认对话框
  mainWindow.on('close', async (event) => {
    // 如果应用正在退出，直接关闭窗口
    if (app.isQuitting) {
      return
    }

    event.preventDefault()
    const canClose = await showCloseConfirm(mainWindow!, {
      checkServiceRunning: () => serviceManager?.getStatus() === 'running',
      stopService: async () => { await stopAllProcesses() }
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

// 停止所有相关进程
async function stopAllProcesses(): Promise<void> {
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
  // 同时结束旧版进程名（兼容升级）
  try {
    execSync('taskkill /f /im isdp-server.exe 2>nul', { encoding: 'utf8' })
  } catch {
    // 忽略错误
  }
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
  await stopAllProcesses()
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
  // 升级前检测进程是否运行，若运行则报错提示用户手动停止
  const runningProcesses = checkProcessesRunning()
  if (runningProcesses.length > 0) {
    // 弹窗提示用户
    const processList = runningProcesses.map(p => `- ${p}`).join('\n')
    dialog.showMessageBox(mainWindow!, {
      type: 'error',
      title: '无法升级',
      message: '检测到以下进程正在运行，请先手动停止后再升级：',
      detail: `${processList}\n\n请在启动器中停止服务，或手动关闭相关程序后重试。`,
      buttons: ['确定'],
    })
    return { success: false, error: '进程正在运行，请先手动停止' }
  }

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
  await stopAllProcesses()
  return { success: true }
})

ipcMain.handle('get-service-status', async () => {
  return { status: serviceManager?.getStatus() || 'stopped' }
})

ipcMain.handle('get-running-agents', async () => {
  const installed = getInstalledVersion()
  if (!installed.installed || !installed.installDir) {
    return { instances: [] }
  }

  // 尝试从配置文件读取端口
  let port = 26305
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
    console.warn('[GetRunningAgents] Failed to read config:', e)
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
    req.on('error', () => resolve({ instances: [] }))
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
    await stopAllProcesses()

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

    // 删除文件（除了data目录）
    const dir = installed.installDir

    // 先强制删除 resources 目录（包含 launcher 和 runtime）
    const resourcesDir = join(dir, 'resources')
    if (existsSync(resourcesDir)) {
      try {
        // 使用最大力度的删除参数
        rmSync(resourcesDir, { recursive: true, force: true, maxRetries: 3, retryDelay: 100 })
        console.log('[Uninstall] Removed resources directory')
      } catch (e) {
        console.error('[Uninstall] Failed to remove resources:', resourcesDir, e)
        // 尝试逐个删除子目录
        try {
          const subEntries = readdirSync(resourcesDir)
          for (const subEntry of subEntries) {
            const subPath = join(resourcesDir, subEntry)
            try {
              rmSync(subPath, { recursive: true, force: true, maxRetries: 3 })
            } catch {}
          }
          // 最后删除空目录
          rmSync(resourcesDir, { force: true })
        } catch {}
      }
    }

    const entriesToDelete = [
      'Colink.exe', 'colink-server.exe', 'isdp-server.exe', 'web',
      // DLL 文件
      'ffmpeg.dll', 'd3dcompiler_47.dll', 'libEGL.dll', 'libGLESv2.dll',
      'vk_swiftshader.dll', 'vulkan-1.dll',
      // 其他文件
      'resources.pak', 'chrome_100_percent.pak', 'chrome_200_percent.pak',
      'icudtl.dat', 'snapshot_blob.bin', 'v8_context_snapshot.bin',
      'vk_swiftshader_icd.json', 'icon.ico',
      'LICENSE.electron.txt', 'LICENSES.chromium.html',
      // 目录
      'locales'
    ]

    for (const entry of entriesToDelete) {
      const path = join(dir, entry)
      if (existsSync(path)) {
        try {
          rmSync(path, { recursive: true, force: true })
          console.log('[Uninstall] Removed:', entry)
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
      message: 'Colink 已卸载',
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