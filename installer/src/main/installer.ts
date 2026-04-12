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

// 数据库迁移结果
interface MigrationResult {
  success: boolean
  currentVersion?: number
  targetVersion?: number
  migrations?: number      // 执行的迁移数量
  backupPath?: string      // 备份路径
  error?: string
  message?: string
}

// 执行数据库迁移（使用 migrate.exe）
// 统一使用 up 命令：首次安装或升级都执行 goose 迁移
export async function runDatabaseMigration(
  dbPath: string,
  sqlChangeDir: string,
  targetVersion?: string,  // 目标版本号，如 "1.1.0"
  mainWindow?: BrowserWindow
): Promise<MigrationResult> {
  try {
    // migrate.exe 位于安装包资源目录的 tools 目录
    const migrateTool = join(process.resourcesPath, 'runtime', 'tools', 'migrate.exe')

    // 如果工具不存在，跳过迁移
    if (!existsSync(migrateTool)) {
      console.warn('[Migration] migrate.exe not found at:', migrateTool)
      return { success: true, message: 'migrate.exe not found' }
    }

    // 需要指定目标版本
    if (!targetVersion) {
      console.warn('[Migration] No target version specified, skipping')
      return { success: true, message: 'no target version' }
    }

    // 构建迁移目录路径：sql-change/v{version}/sqlite/
    const versionDir = targetVersion.startsWith('v') ? targetVersion : `v${targetVersion}`
    const migrationsDir = join(sqlChangeDir, versionDir, 'sqlite')

    // 检查迁移目录是否存在
    if (!existsSync(migrationsDir)) {
      console.warn('[Migration] Migrations dir not found:', migrationsDir)
      return { success: true, message: 'no migrations for this version' }
    }

    // 创建数据库目录（首次安装时需要）
    await mkdir(dirname(dbPath), { recursive: true })

    // 执行 migrate up（首次安装和升级都使用 up 命令）
    // migrate 工具会自动处理数据库不存在的情况
    console.log('[Migration] Executing up:', migrationsDir)

    const { stdout, stderr } = await execAsync(
      `"${migrateTool}" up --db "${dbPath}" --dir "${migrationsDir}" --backup --json`,
      { timeout: 60000 }
    )

    // 解析结果
    try {
      const result = JSON.parse(stdout.trim())
      if (result.success) {
        console.log('[Migration] Success:', result.message)
        return {
          success: true,
          currentVersion: result.currentVersion,
          targetVersion: result.targetVersion,
          migrations: result.migrations,
          backupPath: result.backupPath,
          message: result.message
        }
      } else {
        console.error('[Migration] Failed:', result.error)
        return { success: false, error: result.error }
      }
    } catch {
      // JSON 解析失败，检查 stderr
      if (stderr && stderr.includes('error')) {
        return { success: false, error: stderr }
      }
      return { success: true, message: 'migration completed' }
    }
  } catch (error) {
    console.error('[Migration] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '迁移执行失败' }
  }
}

// 检测数据库变更（根据版本范围和数据库类型筛选）
// 路径格式：sql-change/v{version}/{dbType}/*.sql
export async function checkDatabaseChanges(
  sqlChangeDir: string,
  currentVersion: string,
  newVersion: string,
  dbType: 'sqlite' | 'mysql' = 'sqlite'
): Promise<{ hasChanges: boolean; changes: Array<{ version: string; files: string[] }> }> {
  try {
    // 如果目录不存在，返回无变更
    if (!existsSync(sqlChangeDir)) {
      return { hasChanges: false, changes: [] }
    }

    // 扫描版本目录（v1.0.0, v1.1.0 等）
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
        // 检查对应数据库类型的子目录
        const dbTypePath = join(sqlChangeDir, versionDir, dbType)
        if (existsSync(dbTypePath)) {
          const sqlFiles = readdirSync(dbTypePath)
            .filter(f => f.endsWith('.sql'))
            .sort()

          if (sqlFiles.length > 0) {
            // 返回完整路径：包含 dbType 目录
            changes.push({
              version: versionDir,
              files: sqlFiles.map(f => `${dbType}/${f}`)
            })
          }
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

// ==================== 配置写入 ====================

// 写入配置文件（所见即所得）
// 直接写入前端预览的 YAML 内容
export async function writeConfigFile(
  configPath: string,
  yamlContent: string
): Promise<{ success: boolean; error?: string }> {
  try {
    await mkdir(dirname(configPath), { recursive: true })
    await writeFile(configPath, yamlContent, 'utf-8')
    console.log('[Config] Written successfully')
    return { success: true }
  } catch (error) {
    console.error('[Config] Write failed:', error)
    return { success: false, error: error instanceof Error ? error.message : '配置写入失败' }
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

// 强制结束所有相关进程（不包括当前进程）
export async function killAllProcesses(): Promise<void> {
  try {
    execSync('taskkill /f /im colink-server.exe 2>nul', { encoding: 'utf8' })
  } catch {}
  // 同时结束旧版进程名（兼容升级）
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

    // 复制 colink-server.exe
    const serverSrc = join(runtimeDir, 'colink-server.exe')
    const serverDest = join(destDir, 'colink-server.exe')

    if (existsSync(serverSrc)) {
      if (existsSync(serverDest)) {
        await forceDelete(serverDest)
      }
      await copyFile(serverSrc, serverDest)
    } else {
      console.warn('[Copy] colink-server.exe not found')
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
    const exeDest = join(destDir, 'Colink.exe')
    if (!fs.existsSync(exeDest)) {
      return { success: false, error: '启动器可执行文件复制失败: Colink.exe 不存在' }
    }

    onProgress?.(100)
    return { success: true }
  } catch (error) {
    console.error('[Copy] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '复制失败' }
  }
}

// ==================== 配置文件 ====================

// 生成配置预览（所见即所得）
// 合并顺序：模板 → 用户本地配置 → 页面修改参数
export async function generateConfigPreview(params: {
  installDir?: string  // 安装目录（用于读取本地配置）
  database: {
    type: 'sqlite' | 'mysql'
    host?: string
    port?: number
    database?: string
    username?: string
    password?: string
  }
  serverPort?: number
}): Promise<{ success: boolean; yaml?: string; error?: string }> {
  try {
    // 1. 读取模板作为基础
    const templatePath = findConfigTemplate()
    let baseYaml: any = {}

    if (templatePath) {
      const templateContent = await readFile(templatePath, 'utf-8')
      baseYaml = YAML.parse(templateContent)
    }

    // 2. 如果有本地配置，合入（用户值覆盖模板）
    if (params.installDir) {
      const configPath = join(params.installDir, 'data', 'configs', 'config.yaml')
      if (existsSync(configPath)) {
        const userContent = await readFile(configPath, 'utf-8')
        const userYaml = YAML.parse(userContent)
        baseYaml = mergeObjects(userYaml, baseYaml)
      }
    }

    // 3. 应用页面修改的参数（强制覆盖）
    if (baseYaml?.database) {
      baseYaml.database.type = params.database.type

      // SQLite 模式：强制设置默认路径（确保不会被旧配置覆盖）
      if (params.database.type === 'sqlite') {
        baseYaml.database.path = './data/sqlite/colink.db'
        // 清理 MySQL 相关配置（避免干扰）
        if (baseYaml.database.mysql) {
          // 保留 MySQL 配置供用户手动切换回来时使用，但不影响 SQLite 运行
        }
      }
    }

    if (params.database.type === 'mysql' && baseYaml?.database?.mysql) {
      if (params.database.host) baseYaml.database.mysql.host = params.database.host
      if (params.database.port) baseYaml.database.mysql.port = params.database.port
      if (params.database.database) baseYaml.database.mysql.database = params.database.database
      if (params.database.username) baseYaml.database.mysql.username = params.database.username
      if (params.database.password) baseYaml.database.mysql.password = params.database.password
    }

    if (params.serverPort && baseYaml?.server) {
      baseYaml.server.port = params.serverPort
    }

    return { success: true, yaml: YAML.stringify(baseYaml) }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : '生成配置预览失败' }
  }
}

// 查找配置模板文件（支持开发模式和打包模式）
function findConfigTemplate(): string | null {
  // 打包后：从 resources 目录读取
  const packagedPath = join(process.resourcesPath, 'runtime', 'data', 'configs', 'config.yaml.example')
  console.log('[ConfigTemplate] Checking packaged path:', packagedPath, 'exists:', existsSync(packagedPath))
  if (existsSync(packagedPath)) {
    return packagedPath
  }

  // 开发时：从项目根目录读取
  // __dirname 在开发模式下是 installer/out/main/
  const devPaths = [
    // installer/out/main/ -> isdp/configs/
    join(__dirname, '../../../configs/config.yaml.example'),
    // installer/out/main/ -> installer/resources/runtime/data/configs/
    join(__dirname, '../../resources/runtime/data/configs/config.yaml.example'),
  ]

  for (const p of devPaths) {
    console.log('[ConfigTemplate] Checking dev path:', p, 'exists:', existsSync(p))
    if (existsSync(p)) {
      return p
    }
  }

  // 最后尝试使用 app.isPackaged 判断
  try {
    const { app } = require('electron')
    if (!app.isPackaged) {
      // 开发模式，尝试从工作目录查找
      const cwd = process.cwd()
      const cwdPath = join(cwd, '../configs/config.yaml.example')
      console.log('[ConfigTemplate] Checking cwd path:', cwdPath, 'exists:', existsSync(cwdPath))
      if (existsSync(cwdPath)) {
        return cwdPath
      }
    }
  } catch {}

  console.error('[ConfigTemplate] Template not found in any path')
  return null
}

// 读取已有配置（用于切换 MySQL 时恢复参数）
export async function readExistingConfig(installDir: string): Promise<{
  success: boolean
  config?: {
    database: {
      type: 'sqlite' | 'mysql'
      host?: string
      port?: number
      database?: string
      username?: string
      password?: string
    }
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

    // 保留 MySQL 配置信息供切换回来时使用
    const mysqlConfig = parsed?.database?.mysql || {}

    return {
      success: true,
      config: {
        database: {
          type: parsed?.database?.type || 'sqlite',
          host: mysqlConfig.host || '',
          port: mysqlConfig.port || 3306,
          database: mysqlConfig.database || 'isdp',
          username: mysqlConfig.username || 'root',
          password: mysqlConfig.password || ''
        },
        serverPort: parsed?.server?.port || 8080
      }
    }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : '读取配置失败' }
  }
}

// ==================== 快捷方式 ====================

export async function createDesktopShortcut(installDir: string): Promise<boolean> {
  try {
    const launcherPath = join(installDir, 'Colink.exe')
    const desktopPath = process.env.USERPROFILE + '\\Desktop\\Colink.lnk'

    // 不设置 IconLocation，让快捷方式自动使用 exe 内嵌的图标
    const vbsContent = `Set WshShell = WScript.CreateObject("WScript.Shell")
Set oShellLink = WshShell.CreateShortcut("${desktopPath}")
oShellLink.TargetPath = "${launcherPath}"
oShellLink.WorkingDirectory = "${installDir}"
oShellLink.Description = "Colink"
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
    const launcherPath = join(installDir, 'Colink.exe')
    const startMenuPath = process.env.APPDATA + '\\Microsoft\\Windows\\Start Menu\\Programs\\Colink.lnk'

    // 不设置 IconLocation，让快捷方式自动使用 exe 内嵌的图标
    const vbsContent = `Set WshShell = WScript.CreateObject("WScript.Shell")
Set oShellLink = WshShell.CreateShortcut("${startMenuPath}")
oShellLink.TargetPath = "${launcherPath}"
oShellLink.WorkingDirectory = "${installDir}"
oShellLink.Description = "Colink"
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
    const launcherPath = join(installDir, 'Colink.exe')

    const commands = [
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink" /v "DisplayName" /t REG_SZ /d "Colink" /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink" /v "DisplayVersion" /t REG_SZ /d "${version}" /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink" /v "Publisher" /t REG_SZ /d "Colink Team" /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink" /v "InstallLocation" /t REG_SZ /d "${installDir}" /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink" /v "DisplayIcon" /t REG_SZ /d "${launcherPath}" /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink" /v "NoModify" /t REG_DWORD /d 1 /f`,
      `reg add "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink" /v "NoRepair" /t REG_DWORD /d 1 /f`,
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
    execSync('reg delete "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink" /f', { encoding: 'utf8' })
  } catch {}
  try {
    execSync('reg delete "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink" /f', { encoding: 'utf8' })
  } catch {}
  return true
}

// ==================== 安装流程 ====================

export async function runInstallation(
  config: {
    installDir: string
    installMode: string
    dependencies: Array<{ key: string; installed: boolean }>
    database: {
      type: 'sqlite' | 'mysql'
      host?: string
      port?: number
      database?: string
      username?: string
      password?: string
    }
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
  const sendProgress = (step: string, status: string, progress?: number, message?: string, details?: string) => {
    console.log(`[Install] ${step}: ${status} ${progress || 0}%`)
    mainWindow.webContents.send('install-progress', { step, status, progress, message, details })
  }

  try {
    // Step 0: 停止所有进程
    sendProgress('prepare', 'running', 0, '停止服务...', '正在停止所有相关进程...')
    await killAllProcesses()
    sendProgress('prepare', 'success', 100, '服务已停止', '所有进程已停止')

    // Step 1: 复制应用文件
    sendProgress('copy', 'running', 0, '复制应用文件...', `源目录: ${sourceDir}\n目标目录: ${config.installDir}`)
    const appResult = await copyApplicationFiles(sourceDir, config.installDir, (p) => {
      sendProgress('copy', 'running', Math.round(p * 0.5), `复制应用文件 ${Math.round(p * 0.5)}%...`)
    })
    if (!appResult.success) {
      sendProgress('copy', 'failed', 0, appResult.error, `复制应用文件失败: ${appResult.error}`)
      return appResult
    }

    const launcherResult = await copyLauncherFiles(sourceDir, config.installDir, (p) => {
      sendProgress('copy', 'running', 50 + Math.round(p * 0.5), `复制启动器文件 ${50 + Math.round(p * 0.5)}%...`)
    })
    if (!launcherResult.success) {
      sendProgress('copy', 'failed', 0, launcherResult.error, `复制启动器文件失败: ${launcherResult.error}`)
      return launcherResult
    }
    sendProgress('copy', 'success', 100, '文件复制完成', `已复制所有文件到 ${config.installDir}`)

    // sql-change 从安装包资源目录读取，不复制到用户安装目录
    const sqlChangeDir = join(process.resourcesPath, 'runtime', 'data', 'sql-change')

    // Step 1.5: 检测数据库变更（仅升级安装，根据数据库类型检测对应目录）
    let dbChanges: Array<{ version: string; files: string[] }> = []
    const dbType = config.database.type || 'sqlite'

    if (config.currentVersion && config.newVersion) {
      sendProgress('dbcheck', 'running', 0, '检查数据库变更...', `检查版本范围: ${config.currentVersion} -> ${config.newVersion} (${dbType})`)
      const dbResult = await checkDatabaseChanges(
        sqlChangeDir,
        config.currentVersion,
        config.newVersion,
        dbType
      )
      if (dbResult.hasChanges) {
        dbChanges = dbResult.changes
        const details = dbChanges.map(c => `${c.version}:\n  ${c.files.join('\n  ')}`).join('\n')
        const actionHint = dbType === 'mysql'
          ? 'MySQL 需手动执行迁移'
          : 'SQLite 将自动执行迁移'
        sendProgress('dbcheck', 'success', 100, `检测到 ${dbChanges.length} 个版本的 ${dbType} 数据库变更`, `${actionHint}\n${details}`)
      } else {
        sendProgress('dbcheck', 'success', 100, '无数据库变更', `${dbType} 无需执行数据库迁移`)
      }
    } else {
      sendProgress('dbcheck', 'success', 100, '跳过检测', '新安装不需要检查数据库变更')
    }

    // Step 1.6: 数据库迁移（SQLite 自动执行，MySQL 提示手动执行）
    const dbPath = join(config.installDir, 'data', 'sqlite', 'colink.db')

    // MySQL 场景：提示用户手动执行
    if (dbType === 'mysql') {
      if (dbChanges.length > 0) {
        // 每个文件单独拼接完整路径
        const manualHint = dbChanges.map(c => {
          const filesList = c.files.map(f => `  sql-change/${c.version}/${f}`).join('\n')
          return `${c.version}:\n${filesList}`
        }).join('\n')
        sendProgress('migration', 'warning', 100, 'MySQL 数据库需手动迁移',
          `请手动执行以下 SQL 脚本：\n${manualHint}`)
      } else {
        sendProgress('migration', 'success', 100, '无需迁移', 'MySQL 模式，无数据库变更')
      }
    } else {
      // SQLite 场景：自动执行迁移
      const currentVer = config.currentVersion || '0.0.0'
      const targetVer = config.newVersion || '1.1.0'

      // 扫描所有版本目录，筛选需要执行的
      const versionDirs = readdirSync(sqlChangeDir)
        .filter(f => f.startsWith('v'))
        .sort()

      const versionsToRun: string[] = []
      const current = parseVersion(currentVer)
      const target = parseVersion(targetVer)

      for (const versionDir of versionDirs) {
        const version = parseVersion(versionDir.replace('v', ''))
        // 版本在范围内：大于当前版本，小于或等于目标版本
        if (compareVersions(version, current) > 0 && compareVersions(version, target) <= 0) {
          const sqlitePath = join(sqlChangeDir, versionDir, 'sqlite')
          if (existsSync(sqlitePath)) {
            versionsToRun.push(versionDir)
          }
        }
      }

      // 创建数据库目录
      await mkdir(dirname(dbPath), { recursive: true })

      if (versionsToRun.length === 0) {
        sendProgress('migration', 'success', 100, '无需迁移', 'SQLite 数据库已是最新')
      } else {
        // 按版本顺序逐个执行迁移
        const migrationDetails: string[] = []

        for (let i = 0; i < versionsToRun.length; i++) {
          const versionDir = versionsToRun[i]
          const progress = Math.round(((i + 1) / versionsToRun.length) * 100)

          sendProgress('migration', 'running', progress,
            `迁移 SQLite 数据库到 ${versionDir}...`,
            `执行 sql-change/${versionDir}/sqlite/ 下的脚本`)

          const result = await runDatabaseMigration(
            dbPath,
            sqlChangeDir,
            versionDir.replace('v', ''),
            mainWindow
          )

          if (!result.success) {
            sendProgress('migration', 'failed', 0, '数据库迁移失败', result.error || '未知错误')
            return { success: false, error: `数据库迁移失败: ${result.error}` }
          }

          // 记录每个版本的迁移结果
          if (result.message) {
            migrationDetails.push(`${versionDir}: ${result.message}`)
          }
        }

        // 显示详细的迁移结果
        const detailStr = migrationDetails.length > 0
          ? migrationDetails.join('\n')
          : `${versionsToRun.length} 个版本已处理`
        sendProgress('migration', 'success', 100, 'SQLite 数据库迁移完成', detailStr)
      }
    }

    // Step 2-3: 安装依赖
    if (config.installMode === 'auto') {
      const claudeMissing = config.dependencies.find(d => d.key === 'claude' && !d.installed)
      if (claudeMissing) {
        sendProgress('claude', 'running', 0, '安装 Claude CLI...', '正在执行: npm install -g @anthropic-ai/claude-cli')
        const result = await installNpmPackage('@anthropic-ai/claude-cli')
        sendProgress('claude', result.success ? 'success' : 'failed', result.success ? 100 : 0,
          result.success ? 'Claude CLI 安装成功' : 'Claude CLI 安装失败',
          result.success ? '已安装到全局 npm 目录' : result.error)
      } else {
        sendProgress('claude', 'success', 100, 'Claude CLI 已安装', '检测到已安装，跳过')
      }
    } else {
      sendProgress('claude', 'success', 100, '跳过自动安装', '手动模式')
    }

    if (config.installMode === 'auto') {
      const opencodeMissing = config.dependencies.find(d => d.key === 'opencode' && !d.installed)
      if (opencodeMissing) {
        sendProgress('opencode', 'running', 0, '安装 OpenCode...', '正在执行: npm install -g @anthropic-ai/opencode')
        const result = await installNpmPackage('@anthropic-ai/opencode')
        sendProgress('opencode', result.success ? 'success' : 'failed', result.success ? 100 : 0,
          result.success ? 'OpenCode 安装成功' : 'OpenCode 安装失败',
          result.success ? '已安装到全局 npm 目录' : result.error)
      } else {
        sendProgress('opencode', 'success', 100, 'OpenCode 已安装', '检测到已安装，跳过')
      }
    } else {
      sendProgress('opencode', 'success', 100, '跳过自动安装', '手动模式')
    }

    // Step 4: 配置文件处理（所见即所得）
    sendProgress('config', 'running', 0, '写入配置文件...', `配置路径: ${config.installDir}/data/configs/config.yaml`)
    const configPath = join(config.installDir, 'data', 'configs', 'config.yaml')

    // 直接写入前端预览的 YAML 内容
    if (config.configYaml) {
      const writeResult = await writeConfigFile(configPath, config.configYaml)
      if (!writeResult.success) {
        console.warn('[Config] Write failed:', writeResult.error)
        sendProgress('config', 'warning', 100, '配置写入失败', writeResult.error)
      } else {
        const dbInfo = config.database.type === 'sqlite'
          ? 'SQLite: ./data/sqlite/colink.db'
          : `MySQL: ${config.database.host}:${config.database.port}/${config.database.database}`
        sendProgress('config', 'success', 100, '配置已写入', `数据库类型: ${config.database.type}\n${dbInfo}`)
      }
    } else {
      // 没有 configYaml，使用默认配置
      const defaultResult = await generateConfigPreview({
        installDir: config.installDir,
        database: config.database,
        serverPort: config.serverPort
      })
      if (defaultResult.success && defaultResult.yaml) {
        await writeConfigFile(configPath, defaultResult.yaml)
        sendProgress('config', 'success', 100, '配置已写入', '使用默认配置')
      } else {
        sendProgress('config', 'warning', 100, '配置写入失败', '缺少配置内容')
      }
    }

    // Step 5: 创建快捷方式
    sendProgress('shortcut', 'running', 0, '创建快捷方式...', '桌面和开始菜单')
    if (config.createShortcut !== false) {
      await createDesktopShortcut(config.installDir)
      await createStartMenuShortcut(config.installDir)
      sendProgress('shortcut', 'success', 100, '快捷方式已创建', '桌面快捷方式和开始菜单已创建')
    } else {
      sendProgress('shortcut', 'success', 100, '跳过快捷方式', '用户选择不创建')
    }

    // Step 6: 写入注册表
    sendProgress('registry', 'running', 0, '写入注册表...', `版本: ${config.newVersion || '1.0.0'}`)
    await writeRegistry(config.installDir, config.newVersion || '1.0.0')
    sendProgress('registry', 'success', 100, '注册表已写入', `安装信息已注册到系统\n安装目录: ${config.installDir}`)

    return { success: true, dbChanges }
  } catch (error) {
    console.error('[Install] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '安装失败' }
  }
}