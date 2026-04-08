import { exec, spawn, execSync } from 'child_process'
import { promisify } from 'util'
import { copyFile, mkdir, writeFile, readFile, unlink, stat } from 'fs/promises'
import { createWriteStream, existsSync, unlinkSync, rmSync, cpSync, readdirSync } from 'fs'
import { join, dirname, basename } from 'path'
import { BrowserWindow } from 'electron'
import { tmpdir } from 'os'
import { https } from 'follow-redirects'
import YAML from 'yaml'

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

// ==================== 数据库变更检测 ====================

// 检测数据库变更（根据版本范围筛选）
export async function checkDatabaseChanges(
  installDir: string,
  currentVersion: string,
  newVersion: string
): Promise<{ hasChanges: boolean; changes: Array<{ version: string; files: string[] }> }> {
  try {
    const sqlChangeDir = join(installDir, 'data', 'sql-change', 'migrations')

    // 如果目录不存在，返回无变更
    if (!existsSync(sqlChangeDir)) {
      return { hasChanges: false, changes: [] }
    }

    // 扫描版本目录
    const versions = readdirSync(sqlChangeDir)
      .filter(f => {
        const fullPath = join(sqlChangeDir, f)
        return existsSync(fullPath) && f.startsWith('v')
      })
      .sort()

    if (versions.length === 0) {
      return { hasChanges: false, changes: [] }
    }

    // 解析版本号
    const current = parseVersion(currentVersion)
    const newVer = parseVersion(newVersion)

    const changes: Array<{ version: string; files: string[] }> = []

    for (const versionDir of versions) {
      const version = parseVersion(versionDir.replace('v', ''))

      // 版本在范围内：大于当前版本，小于或等于新版本
      // 新安装时 currentVersion 为 "0.0.0"，会提示所有版本
      if (compareVersions(version, current) > 0 && compareVersions(version, newVer) <= 0) {
        const versionPath = join(sqlChangeDir, versionDir)
        const sqlFiles = readdirSync(versionPath)
          .filter(f => f.endsWith('.sql'))
          .sort()

        if (sqlFiles.length > 0) {
          changes.push({ version: versionDir, files: sqlFiles })
        }
      }
    }

    return { hasChanges: changes.length > 0, changes }
  } catch (error) {
    console.error('[DBChanges] Check failed:', error)
    return { hasChanges: false, changes: [] }
  }
}

// 解析版本号为数字数组
function parseVersion(version: string): [number, number, number] {
  const match = version.match(/(\d+)\.(\d+)\.(\d+)/)
  if (match) {
    return [parseInt(match[1]), parseInt(match[2]), parseInt(match[3])]
  }
  return [0, 0, 0]
}

// 比较版本号：返回 -1, 0, 1
function compareVersions(a: [number, number, number], b: [number, number, number]): number {
  for (let i = 0; i < 3; i++) {
    if (a[i] < b[i]) return -1
    if (a[i] > b[i]) return 1
  }
  return 0
}

// ==================== 配置合并 ====================

// 合并用户配置与模板配置（用户值优先）
export async function mergeConfigFiles(
  userConfigPath: string,
  templateConfigPath: string
): Promise<{ success: boolean; error?: string }> {
  try {
    // 如果用户配置不存在，直接复制模板
    if (!existsSync(userConfigPath)) {
      const templateContent = await readFile(templateConfigPath, 'utf-8')
      await writeFile(userConfigPath, templateContent, 'utf-8')
      return { success: true }
    }

    // 读取模板配置
    if (!existsSync(templateConfigPath)) {
      return { success: true } // 模板不存在，跳过合并
    }

    const templateContent = await readFile(templateConfigPath, 'utf-8')
    const userContent = await readFile(userConfigPath, 'utf-8')

    // 解析 YAML
    const templateYaml = YAML.parse(templateContent)
    const userYaml = YAML.parse(userContent)

    // 合并配置：用户值优先，模板补充缺失字段
    const mergedYaml = mergeObjects(userYaml, templateYaml)

    // 写回配置文件
    const mergedContent = YAML.stringify(mergedYaml)
    await writeFile(userConfigPath, mergedContent, 'utf-8')

    console.log('[Config] Merged with template')
    return { success: true }
  } catch (error) {
    console.error('[Config] Merge failed:', error)
    return { success: false, error: error instanceof Error ? error.message : '配置合并失败' }
  }
}

// 递归合并对象（用户值优先）
function mergeObjects(user: any, template: any): any {
  if (typeof template !== 'object' || template === null) {
    return user !== undefined ? user : template
  }

  if (typeof user !== 'object' || user === null) {
    return template
  }

  const result = { ...template }

  for (const key of Object.keys(user)) {
    if (typeof user[key] === 'object' && user[key] !== null &&
        typeof template[key] === 'object' && template[key] !== null) {
      // 递归合并嵌套对象
      result[key] = mergeObjects(user[key], template[key])
    } else {
      // 用户值优先
      result[key] = user[key]
    }
  }

  return result
}

// ==================== 文件操作 ====================

// 强制结束所有ISDP相关进程（不包括当前进程）
export async function killAllProcesses(): Promise<void> {
  try {
    execSync('taskkill /f /im isdp-server.exe 2>nul', { encoding: 'utf8' })
  } catch {}

  // 等待进程完全退出
  await new Promise(resolve => setTimeout(resolve, 1500))
}

// 带重试的强制删除
async function forceDelete(path: string, retries: number = 5): Promise<boolean> {
  for (let i = 0; i < retries; i++) {
    try {
      if (!existsSync(path)) return true
      rmSync(path, { recursive: true, force: true })
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
    await mkdir(destDir, { recursive: true })

    const resourcesDir = process.resourcesPath

    if (!existsSync(resourcesDir)) {
      return { success: false, error: `资源目录不存在: ${resourcesDir}` }
    }

    const runtimeDir = join(resourcesDir, 'runtime')

    if (!existsSync(runtimeDir)) {
      return { success: false, error: `运行时目录不存在: ${runtimeDir}` }
    }

    // 复制 isdp-server.exe
    const serverSrc = join(runtimeDir, 'isdp-server.exe')
    const serverDest = join(destDir, 'isdp-server.exe')

    if (existsSync(serverSrc)) {
      if (existsSync(serverDest)) {
        await forceDelete(serverDest)
      }
      await copyFile(serverSrc, serverDest)
    } else {
      console.warn('[Copy] isdp-server.exe not found')
    }

    // 复制 web/
    const webSrc = join(runtimeDir, 'web')
    const webDest = join(destDir, 'web')

    if (existsSync(webSrc)) {
      if (existsSync(webDest)) {
        await forceDelete(webDest)
      }
      cpSync(webSrc, webDest, { recursive: true })
    } else {
      console.warn('[Copy] web/ not found')
    }

    // 创建数据目录
    await mkdir(join(destDir, 'data', 'configs'), { recursive: true })
    await mkdir(join(destDir, 'data', 'logs'), { recursive: true })
    await mkdir(join(destDir, 'data', 'agent-assets'), { recursive: true })
    await mkdir(join(destDir, 'data', 'agent-configs'), { recursive: true })
    await mkdir(join(destDir, 'data', 'repos'), { recursive: true })

    // 复制数据库变更目录（如果存在）
    const sqlChangeSrc = join(runtimeDir, 'data', 'sql-change')
    if (existsSync(sqlChangeSrc)) {
      const sqlChangeDest = join(destDir, 'data', 'sql-change')
      await mkdir(sqlChangeDest, { recursive: true })
      cpSync(sqlChangeSrc, sqlChangeDest, { recursive: true })
      console.log('[Copy] SQL change directory copied')
    }

    // 复制配置模板（用于升级时配置合并）
    const templateSrc = join(runtimeDir, 'data', 'configs', 'config.yaml.example')
    const templateDest = join(destDir, 'data', 'configs', 'config.yaml.example')
    if (existsSync(templateSrc)) {
      await copyFile(templateSrc, templateDest)
      console.log('[Copy] Config template copied')
    }

    // 复制 icon.ico 到安装目录（用于快捷方式和注册表）
    const iconSrc = join(resourcesDir, 'icon.ico')
    const iconDest = join(destDir, 'icon.ico')
    if (existsSync(iconSrc)) {
      await copyFile(iconSrc, iconDest)
    }

    onProgress?.(100)
    return { success: true }
  } catch (error) {
    console.error('[Copy] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '复制失败' }
  }
}

// 复制启动器文件到目标目录
// 从 resources/launcher/ 目录复制完整的启动器
export async function copyLauncherFiles(
  sourceDir: string,
  destDir: string,
  onProgress?: (progress: number) => void
): Promise<{ success: boolean; error?: string }> {
  try {
    const resourcesDir = process.resourcesPath
    const launcherSrcDir = join(resourcesDir, 'launcher')

    if (!existsSync(launcherSrcDir)) {
      return { success: false, error: `启动器目录不存在: ${launcherSrcDir}` }
    }

    const fs = require('original-fs')

    const entries = fs.readdirSync(launcherSrcDir, { withFileTypes: true })
    if (entries.length === 0) {
      return { success: false, error: `启动器目录为空: ${launcherSrcDir}` }
    }

    let copiedFiles = 0
    const totalFiles = entries.length

    for (const entry of entries) {
      const srcPath = join(launcherSrcDir, entry.name)
      const destPath = join(destDir, entry.name)

      try {
        const fileStat = fs.statSync(srcPath)

        // 特殊处理 resources 目录，因为包含 asar 文件
        if (fileStat.isDirectory() && entry.name === 'resources') {
          fs.mkdirSync(destPath, { recursive: true })

          const resourcesEntries = fs.readdirSync(srcPath, { withFileTypes: true })
          for (const resEntry of resourcesEntries) {
            const resSrcPath = join(srcPath, resEntry.name)
            const resDestPath = join(destPath, resEntry.name)

            if (resEntry.name.endsWith('.asar')) {
              if (fs.existsSync(resDestPath)) {
                fs.rmSync(resDestPath, { force: true })
              }
              fs.copyFileSync(resSrcPath, resDestPath)
            } else if (fs.statSync(resSrcPath).isDirectory()) {
              fs.cpSync(resSrcPath, resDestPath, { recursive: true })
            } else {
              fs.copyFileSync(resSrcPath, resDestPath)
            }
          }
        } else if (fileStat.isDirectory()) {
          if (fs.existsSync(destPath)) {
            fs.rmSync(destPath, { recursive: true, force: true })
          }
          fs.cpSync(srcPath, destPath, { recursive: true })
        } else {
          if (fs.existsSync(destPath)) {
            fs.rmSync(destPath, { force: true })
          }
          fs.copyFileSync(srcPath, destPath)
        }
        copiedFiles++
        onProgress?.(Math.round((copiedFiles / totalFiles) * 100))
      } catch (copyError) {
        console.error(`[Copy] Failed to copy ${entry.name}:`, copyError)
        // 继续复制其他文件
      }
    }

    // 验证关键文件
    const exeDest = join(destDir, 'ISDP.exe')
    if (!fs.existsSync(exeDest)) {
      return { success: false, error: '启动器可执行文件复制失败: ISDP.exe 不存在' }
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
  config?: {
    database: { host: string; port: number; database: string; username: string; password: string }
    serverPort?: number
  }
  error?: string
}> {
  try {
    const configPath = join(installDir, 'data', 'configs', 'config.yaml')
    if (!existsSync(configPath)) {
      return { success: false, error: '配置文件不存在' }
    }

    const content = await readFile(configPath, 'utf-8')
    const parsed = YAML.parse(content)

    // 使用 YAML 库正确解析，自动处理引号等特殊字符
    const dbConfig = parsed?.database?.mysql || {}

    return {
      success: true,
      config: {
        database: {
          host: dbConfig.host || '',
          port: dbConfig.port || 3306,
          database: dbConfig.database || 'isdp',
          username: dbConfig.username || 'root',
          password: dbConfig.password || ''
        },
        serverPort: parsed?.server?.port || 8080
      }
    }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : '读取配置失败' }
  }
}

function generateConfigYaml(db: { host: string; port: number; database: string; username: string; password: string }, serverPort: number = 8080): string {
  const config = {
    server: {
      port: serverPort,
      mode: 'release'
    },
    data: {
      base_path: './data'
    },
    database: {
      type: 'mysql',
      mysql: {
        host: db.host,
        port: db.port,
        database: db.database,
        username: db.username,
        password: db.password,
        charset: 'utf8mb4'
      }
    },
    sandbox: {
      repos_dir: './data/repos'
    },
    claude: {
      path: 'claude',
      default_model: 'claude-sonnet-4-6',
      timeout: '30m'
    },
    logging: {
      level: 'info',
      format: 'json'
    },
    agent_assets: {
      base_path: './data/agent-assets'
    },
    agent_config: {
      data_dir: './data/agent-configs'
    }
  }

  return YAML.stringify(config)
}

// ==================== 快捷方式 ====================

export async function createDesktopShortcut(installDir: string): Promise<boolean> {
  try {
    const launcherPath = join(installDir, 'ISDP.exe')
    const iconPath = join(installDir, 'icon.ico')
    const desktopPath = process.env.USERPROFILE + '\\Desktop\\ISDP.lnk'

    const vbsContent = `Set WshShell = WScript.CreateObject("WScript.Shell")
Set oShellLink = WshShell.CreateShortcut("${desktopPath}")
oShellLink.TargetPath = "${launcherPath}"
oShellLink.WorkingDirectory = "${installDir}"
oShellLink.Description = "ISDP"
oShellLink.IconLocation = "${iconPath},0"
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
    const iconPath = join(installDir, 'icon.ico')
    const startMenuPath = process.env.APPDATA + '\\Microsoft\\Windows\\Start Menu\\Programs\\ISDP.lnk'

    const vbsContent = `Set WshShell = WScript.CreateObject("WScript.Shell")
Set oShellLink = WshShell.CreateShortcut("${startMenuPath}")
oShellLink.TargetPath = "${launcherPath}"
oShellLink.WorkingDirectory = "${installDir}"
oShellLink.Description = "ISDP"
oShellLink.IconLocation = "${iconPath},0"
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
    serverPort?: number
    keepData?: boolean
    createShortcut?: boolean
    currentVersion?: string  // 当前已安装版本（升级时使用）
    newVersion?: string      // 新安装版本
  },
  resourcePath: string,
  mainWindow: BrowserWindow,
  sourceDir: string
): Promise<{ success: boolean; error?: string; dbChanges?: Array<{ version: string; files: string[] }> }> {
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

    // Step 1.5: 检测数据库变更（仅升级安装）
    let dbChanges: Array<{ version: string; files: string[] }> = []
    if (config.currentVersion && config.newVersion) {
      sendProgress('dbcheck', 'running', 0, '检查数据库变更...')
      const dbResult = await checkDatabaseChanges(
        config.installDir,
        config.currentVersion,
        config.newVersion
      )
      if (dbResult.hasChanges) {
        dbChanges = dbResult.changes
        sendProgress('dbcheck', 'warning', 100, `检测到 ${dbChanges.length} 个版本的数据库变更`)
      } else {
        sendProgress('dbcheck', 'success', 100, '无数据库变更')
      }
    }

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

    // Step 4: 配置文件处理
    sendProgress('config', 'running', 0)
    const configPath = join(config.installDir, 'data', 'configs', 'config.yaml')
    const templatePath = join(config.installDir, 'data', 'configs', 'config.yaml.example')

    if (existsSync(configPath)) {
      // 已有配置文件，执行合并
      const mergeResult = await mergeConfigFiles(configPath, templatePath)
      if (!mergeResult.success) {
        console.warn('[Config] Merge failed, keeping existing config:', mergeResult.error)
      }
      sendProgress('config', 'success', 100, '配置已合并')
    } else {
      // 新安装，生成配置文件
      const configContent = generateConfigYaml(config.database, config.serverPort || 8080)
      const configResult = await generateConfigFile(configPath, configContent)
      if (!configResult.success) {
        sendProgress('config', 'failed', 0, configResult.error)
        return { success: false, error: configResult.error }
      }
      sendProgress('config', 'success', 100)
    }

    // Step 5: 创建快捷方式
    sendProgress('shortcut', 'running', 0)
    if (config.createShortcut !== false) {
      await createDesktopShortcut(config.installDir)
      await createStartMenuShortcut(config.installDir)
    }
    sendProgress('shortcut', 'success', 100)

    // Step 6: 写入注册表
    sendProgress('registry', 'running', 0)
    await writeRegistry(config.installDir, config.newVersion || '1.0.0')
    sendProgress('registry', 'success', 100)

    return { success: true, dbChanges }
  } catch (error) {
    console.error('[Install] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '安装失败' }
  }
}