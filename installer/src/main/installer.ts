import { exec, spawn, execSync } from 'child_process'
import { promisify } from 'util'
import { copyFile, mkdir, writeFile, readFile, unlink, stat } from 'fs/promises'
import { createWriteStream, existsSync, unlinkSync, rmSync, cpSync } from 'fs'
import { join, dirname, basename } from 'path'
import { BrowserWindow } from 'electron'
import { tmpdir } from 'os'
import { https } from 'follow-redirects'

const execAsync = promisify(exec)

// ==================== 依赖检测与安装 ====================

export interface DependencyCheckResult {
  installed: boolean
  version?: string
}

export async function checkDependency(key: string): Promise<DependencyCheckResult> {
  const commands: Record<string, string> = {
    nodejs: 'node --version',
    git: 'git --version',
    claude: 'claude --version',
    opencode: 'opencode --version',
  }

  const cmd = commands[key]
  if (!cmd) return { installed: false }

  try {
    const { stdout } = await execAsync(cmd)
    const versionMatch = stdout.match(/(\d+\.\d+\.\d+)/)
    return {
      installed: true,
      version: versionMatch ? versionMatch[1] : stdout.trim(),
    }
  } catch {
    return { installed: false }
  }
}

export async function installNpmPackage(packageName: string): Promise<{ success: boolean; error?: string }> {
  try {
    await execAsync(`npm install -g ${packageName}`, { timeout: 120000 })
    return { success: true }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '安装失败',
    }
  }
}

// ==================== 文件操作 ====================

// 强制结束所有ISDP相关进程（不包括当前进程）
export async function killAllProcesses(): Promise<void> {
  console.log('[Process] Killing all ISDP processes...')

  // 结束 isdp-server.exe
  try {
    execSync('taskkill /f /im isdp-server.exe 2>nul', { encoding: 'utf8' })
    console.log('[Process] Killed isdp-server.exe')
  } catch {}

  console.log('[Process] Skip killing ISDP.exe to avoid self-termination')

  // 等待进程完全退出
  await new Promise(resolve => setTimeout(resolve, 1500))
}

// 带重试的强制删除
async function forceDelete(path: string, retries: number = 5): Promise<boolean> {
  for (let i = 0; i < retries; i++) {
    try {
      if (!existsSync(path)) return true
      rmSync(path, { recursive: true, force: true })
      console.log('[Delete] Successfully deleted:', path)
      return true
    } catch (e) {
      console.warn(`[Delete] Attempt ${i + 1} failed:`, e)
      await new Promise(resolve => setTimeout(resolve, 300 * (i + 1)))
    }
  }
  console.error('[Delete] All retries failed for:', path)
  return false
}

export async function copyApplicationFiles(
  srcDir: string,
  destDir: string,
  onProgress?: (progress: number) => void
): Promise<{ success: boolean; error?: string }> {
  try {
    console.log('[Copy] Copying application files')
    console.log('[Copy] Dest dir:', destDir)
    console.log('[Copy] Resources path:', process.resourcesPath)

    await mkdir(destDir, { recursive: true })

    const resourcesDir = process.resourcesPath
    console.log('[Copy] Using resources from:', resourcesDir)

    if (!existsSync(resourcesDir)) {
      return { success: false, error: `资源目录不存在: ${resourcesDir}` }
    }

    // 复制 isdp-server.exe
    const serverSrc = join(resourcesDir, 'isdp-server.exe')
    const serverDest = join(destDir, 'isdp-server.exe')

    if (existsSync(serverSrc)) {
      if (existsSync(serverDest)) {
        await forceDelete(serverDest)
      }
      await copyFile(serverSrc, serverDest)
      console.log('[Copy] isdp-server.exe copied')
    }

    // 复制 web/
    const webSrc = join(resourcesDir, 'web')
    const webDest = join(destDir, 'web')

    if (existsSync(webSrc)) {
      if (existsSync(webDest)) {
        await forceDelete(webDest)
      }
      cpSync(webSrc, webDest, { recursive: true })
      console.log('[Copy] web/ copied')
    }

    // 创建数据目录
    await mkdir(join(destDir, 'data', 'configs'), { recursive: true })
    await mkdir(join(destDir, 'data', 'logs'), { recursive: true })
    await mkdir(join(destDir, 'data', 'agent-assets'), { recursive: true })
    await mkdir(join(destDir, 'data', 'agent-configs'), { recursive: true })
    await mkdir(join(destDir, 'data', 'repos'), { recursive: true })

    onProgress?.(100)
    return { success: true }
  } catch (error) {
    console.error('[Copy] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '复制失败' }
  }
}

// 复制 Electron 运行时到目标目录
export async function copyLauncherFiles(
  sourceDir: string,
  destDir: string,
  onProgress?: (progress: number) => void
): Promise<{ success: boolean; error?: string }> {
  try {
    console.log('[Copy] Copying launcher files')
    console.log('[Copy] Dest:', destDir)

    const exeDir = dirname(process.execPath)
    console.log('[Copy] Current exe dir:', exeDir)

    // 复制 ISDP.exe
    const exeSrc = process.execPath
    const exeDest = join(destDir, 'ISDP.exe')

    if (exeSrc.toLowerCase() !== exeDest.toLowerCase()) {
      console.log('[Copy] Copying ISDP.exe...')
      if (existsSync(exeDest)) {
        await forceDelete(exeDest)
      }
      await copyFile(exeSrc, exeDest)
      console.log('[Copy] ISDP.exe copied')
    } else {
      console.log('[Copy] ISDP.exe is same location, skip copy')
    }

    // 复制 resources 目录
    const resourcesSrc = process.resourcesPath
    const resourcesDest = join(destDir, 'resources')
    console.log('[Copy] Resources src:', resourcesSrc)
    console.log('[Copy] Resources dest:', resourcesDest)

    // 删除目标 resources 目录
    if (existsSync(resourcesDest)) {
      console.log('[Copy] Removing existing resources directory...')
      rmSync(resourcesDest, { recursive: true, force: true })
      await new Promise(resolve => setTimeout(resolve, 500))
    }

    if (!existsSync(resourcesSrc)) {
      return { success: false, error: `源资源目录不存在: ${resourcesSrc}` }
    }

    console.log('[Copy] Copying resources directory...')

    // 手动复制 resources 目录内容，避免 cpSync 对 asar 文件的处理问题
    const fs = require('fs')
    await mkdir(resourcesDest, { recursive: true })

    const entries = fs.readdirSync(resourcesSrc, { withFileTypes: true })
    for (const entry of entries) {
      const srcPath = join(resourcesSrc, entry.name)
      const destPath = join(resourcesDest, entry.name)

      // 使用 stat 检查是否为目录（更可靠）
      const stat = fs.statSync(srcPath)
      if (stat.isDirectory()) {
        console.log(`[Copy] Copying directory: ${entry.name}`)
        cpSync(srcPath, destPath, { recursive: true })
      } else {
        console.log(`[Copy] Copying file: ${entry.name} (${stat.size} bytes)`)
        await copyFile(srcPath, destPath)
      }
    }
    console.log('[Copy] resources/ copied')

    // 验证关键文件
    const appAsarDest = join(resourcesDest, 'app.asar')
    const appAsarStat = fs.statSync(appAsarDest)
    if (!appAsarStat.isFile()) {
      return { success: false, error: 'app.asar 不是文件，复制失败' }
    }
    console.log('[Copy] app.asar verified as file, size:', appAsarStat.size)

    // 复制图标
    const iconSrc = join(exeDir, 'resources', 'icon.ico')
    const iconDest = join(destDir, 'icon.ico')
    if (existsSync(iconSrc) && iconSrc.toLowerCase() !== iconDest.toLowerCase()) {
      try {
        await copyFile(iconSrc, iconDest)
      } catch {}
    }

    // 复制 locales
    const localesSrc = join(exeDir, 'locales')
    const localesDest = join(destDir, 'locales')
    if (existsSync(localesSrc)) {
      if (existsSync(localesDest)) {
        await forceDelete(localesDest)
      }
      try {
        cpSync(localesSrc, localesDest, { recursive: true })
      } catch {}
    }

    // 复制 DLL 和其他文件
    const otherFiles = [
      'ffmpeg.dll', 'd3dcompiler_47.dll', 'libEGL.dll', 'libGLESv2.dll',
      'vk_swiftshader.dll', 'vulkan-1.dll', 'resources.pak',
      'chrome_100_percent.pak', 'chrome_200_percent.pak',
      'icudtl.dat', 'snapshot_blob.bin', 'v8_context_snapshot.bin',
      'LICENSE.electron.txt', 'LICENSES.chromium.html'
    ]

    for (const file of otherFiles) {
      const fileSrc = join(exeDir, file)
      const fileDest = join(destDir, file)
      if (existsSync(fileSrc) && fileSrc.toLowerCase() !== fileDest.toLowerCase()) {
        try {
          if (existsSync(fileDest)) {
            unlinkSync(fileDest)
          }
          await copyFile(fileSrc, fileDest)
        } catch {}
      }
    }

    onProgress?.(100)
    return { success: true }
  } catch (error) {
    console.error('[Copy] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '复制失败' }
  }
}

// ==================== 配置文件 ====================

export async function generateConfigFile(
  destPath: string,
  content: string
): Promise<{ success: boolean; error?: string }> {
  try {
    await mkdir(dirname(destPath), { recursive: true })
    await writeFile(destPath, content, 'utf-8')
    return { success: true }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : '生成配置失败' }
  }
}

export async function readExistingConfig(installDir: string): Promise<{
  success: boolean
  config?: { database: { host: string; port: number; database: string; username: string; password: string } }
  error?: string
}> {
  try {
    const configPath = join(installDir, 'data', 'configs', 'config.yaml')
    if (!existsSync(configPath)) {
      return { success: false, error: '配置文件不存在' }
    }

    const content = await readFile(configPath, 'utf-8')
    const config = {
      database: { host: '', port: 3306, database: 'isdp', username: 'root', password: '' }
    }

    const lines = content.split('\n')
    let inMysql = false

    for (const line of lines) {
      const trimmed = line.trim()
      if (trimmed === 'mysql:') { inMysql = true; continue }
      if (inMysql && !trimmed.startsWith('host') && !trimmed.startsWith('port') && !trimmed.startsWith('database') && !trimmed.startsWith('username') && !trimmed.startsWith('password') && !trimmed.startsWith('charset') && trimmed.includes(':')) {
        inMysql = false
      }
      if (inMysql) {
        if (trimmed.startsWith('host:')) config.database.host = trimmed.replace('host:', '').trim()
        else if (trimmed.startsWith('port:')) config.database.port = parseInt(trimmed.replace('port:', '').trim()) || 3306
        else if (trimmed.startsWith('database:')) config.database.database = trimmed.replace('database:', '').trim()
        else if (trimmed.startsWith('username:')) config.database.username = trimmed.replace('username:', '').trim()
        else if (trimmed.startsWith('password:')) config.database.password = trimmed.replace('password:', '').trim()
      }
    }

    return { success: true, config }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : '读取配置失败' }
  }
}

function generateConfigYaml(db: { host: string; port: number; database: string; username: string; password: string }): string {
  return `server:
  port: 8080
  mode: release

data:
  base_path: ./data

database:
  type: mysql
  mysql:
    host: ${db.host}
    port: ${db.port}
    database: ${db.database}
    username: ${db.username}
    password: ${db.password}
    charset: utf8mb4

sandbox:
  repos_dir: ./data/repos

claude:
  path: claude
  default_model: claude-sonnet-4-6
  timeout: 30m

logging:
  level: info
  format: json

agent_assets:
  base_path: ./data/agent-assets

agent_config:
  data_dir: ./data/agent-configs
`
}

// ==================== 快捷方式 ====================

export async function createDesktopShortcut(installDir: string): Promise<boolean> {
  try {
    const launcherPath = join(installDir, 'ISDP.exe')
    const desktopPath = process.env.USERPROFILE + '\\Desktop\\ISDP.lnk'

    const vbsContent = `Set WshShell = WScript.CreateObject("WScript.Shell")
Set oShellLink = WshShell.CreateShortcut("${desktopPath}")
oShellLink.TargetPath = "${launcherPath}"
oShellLink.WorkingDirectory = "${installDir}"
oShellLink.Description = "ISDP"
oShellLink.Save
WScript.Echo "OK"`

    const vbsPath = join(process.env.TEMP || '.', 'create_shortcut.vbs')
    await writeFile(vbsPath, vbsContent, 'utf-8')
    await execAsync(`cscript //nologo "${vbsPath}"`)
    try { unlinkSync(vbsPath) } catch {}

    return true
  } catch (error) {
    console.error('[Shortcut] Failed:', error)
    return false
  }
}

export async function createStartMenuShortcut(installDir: string): Promise<boolean> {
  try {
    const launcherPath = join(installDir, 'ISDP.exe')
    const startMenuPath = process.env.APPDATA + '\\Microsoft\\Windows\\Start Menu\\Programs\\ISDP.lnk'

    const vbsContent = `Set WshShell = WScript.CreateObject("WScript.Shell")
Set oShellLink = WshShell.CreateShortcut("${startMenuPath}")
oShellLink.TargetPath = "${launcherPath}"
oShellLink.WorkingDirectory = "${installDir}"
oShellLink.Description = "ISDP"
oShellLink.Save
WScript.Echo "OK"`

    const vbsPath = join(process.env.TEMP || '.', 'create_startmenu_shortcut.vbs')
    await writeFile(vbsPath, vbsContent, 'utf-8')
    await execAsync(`cscript //nologo "${vbsPath}"`)
    try { unlinkSync(vbsPath) } catch {}

    return true
  } catch (error) {
    console.error('[Shortcut] Failed:', error)
    return false
  }
}

// ==================== 注册表 ====================

export async function writeRegistry(installDir: string, version: string = '1.0.0'): Promise<boolean> {
  try {
    const launcherPath = join(installDir, 'ISDP.exe')

    const commands = [
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v "DisplayName" /t REG_SZ /d "ISDP" /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v "DisplayVersion" /t REG_SZ /d "${version}" /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v "Publisher" /t REG_SZ /d "ISDP Team" /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v "InstallLocation" /t REG_SZ /d "${installDir}" /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v "DisplayIcon" /t REG_SZ /d "${launcherPath}" /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v "NoModify" /t REG_DWORD /d 1 /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v "NoRepair" /t REG_DWORD /d 1 /f`,
    ]

    let success = true
    for (const cmd of commands) {
      try {
        await execAsync(cmd)
      } catch {
        success = false
      }
    }

    if (!success) {
      const hkcuCommands = commands.map(cmd => cmd.replace('HKLM', 'HKCU'))
      for (const cmd of hkcuCommands) {
        try { await execAsync(cmd) } catch {}
      }
    }

    return true
  } catch {
    return false
  }
}

export function deleteRegistry(): boolean {
  try {
    execSync('reg delete "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /f', { encoding: 'utf8' })
  } catch {}
  try {
    execSync('reg delete "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /f', { encoding: 'utf8' })
  } catch {}
  return true
}

// ==================== 安装流程 ====================

export async function runInstallation(
  config: {
    installDir: string
    installMode: string
    dependencies: Array<{ key: string; installed: boolean }>
    database: { host: string; port: number; database: string; username: string; password: string }
    keepData?: boolean
    createShortcut?: boolean
  },
  resourcePath: string,
  mainWindow: BrowserWindow,
  sourceDir: string
): Promise<{ success: boolean; error?: string }> {
  const sendProgress = (step: string, status: string, progress?: number, message?: string) => {
    console.log(`[Install] ${step}: ${status} ${progress || 0}%`)
    mainWindow.webContents.send('install-progress', { step, status, progress, message })
  }

  try {
    // Step 0: 停止所有进程
    sendProgress('prepare', 'running', 0, '停止服务...')
    await killAllProcesses()
    sendProgress('prepare', 'success', 100)

    // Step 1: 复制应用文件
    sendProgress('copy', 'running', 0)
    const appResult = await copyApplicationFiles(sourceDir, config.installDir, (p) => {
      sendProgress('copy', 'running', Math.round(p * 0.5))
    })
    if (!appResult.success) {
      sendProgress('copy', 'failed', 0, appResult.error)
      return appResult
    }

    const launcherResult = await copyLauncherFiles(sourceDir, config.installDir, (p) => {
      sendProgress('copy', 'running', 50 + Math.round(p * 0.5))
    })
    if (!launcherResult.success) {
      sendProgress('copy', 'failed', 0, launcherResult.error)
      return launcherResult
    }
    sendProgress('copy', 'success', 100)

    // Step 2-3: 安装依赖
    if (config.installMode === 'auto') {
      const claudeMissing = config.dependencies.find(d => d.key === 'claude' && !d.installed)
      if (claudeMissing) {
        sendProgress('claude', 'running', 0)
        const result = await installNpmPackage('@anthropic-ai/claude-cli')
        sendProgress('claude', result.success ? 'success' : 'failed', result.success ? 100 : 0)
      } else {
        sendProgress('claude', 'success', 100)
      }
    } else {
      sendProgress('claude', 'success', 100)
    }

    if (config.installMode === 'auto') {
      const opencodeMissing = config.dependencies.find(d => d.key === 'opencode' && !d.installed)
      if (opencodeMissing) {
        sendProgress('opencode', 'running', 0)
        const result = await installNpmPackage('@anthropic-ai/opencode')
        sendProgress('opencode', result.success ? 'success' : 'failed', result.success ? 100 : 0)
      } else {
        sendProgress('opencode', 'success', 100)
      }
    } else {
      sendProgress('opencode', 'success', 100)
    }

    // Step 4: 生成配置文件
    sendProgress('config', 'running', 0)
    const configContent = generateConfigYaml(config.database)
    const configPath = join(config.installDir, 'data', 'configs', 'config.yaml')
    const configResult = await generateConfigFile(configPath, configContent)
    if (!configResult.success) {
      sendProgress('config', 'failed', 0, configResult.error)
      return configResult
    }
    sendProgress('config', 'success', 100)

    // Step 5: 创建快捷方式
    sendProgress('shortcut', 'running', 0)
    if (config.createShortcut !== false) {
      await createDesktopShortcut(config.installDir)
      await createStartMenuShortcut(config.installDir)
    }
    sendProgress('shortcut', 'success', 100)

    // Step 6: 写入注册表
    sendProgress('registry', 'running', 0)
    await writeRegistry(config.installDir)
    sendProgress('registry', 'success', 100)

    return { success: true }
  } catch (error) {
    console.error('[Install] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '安装失败' }
  }
}