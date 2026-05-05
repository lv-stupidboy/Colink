import { exec, spawn, execSync } from 'child_process'
import { promisify } from 'util'
import { createWriteStream, existsSync, unlinkSync, rmSync, readdirSync, mkdirSync, writeFileSync, copyFileSync, readFileSync, statSync, renameSync, Dirent } from 'fs'
import { join, dirname, basename } from 'path'
import { BrowserWindow } from 'electron'
import { https } from 'follow-redirects'
import YAML from 'yaml'

const execAsync = promisify(exec)

// 让主线程有机会处理 IPC 消息（解决 UI 未响应问题）
const yieldToMain = () => new Promise(resolve => setImmediate(resolve))

// 使用 Promise 包装 fs 同步函数，避免 fs/promises 在 Electron 打包时被替换为 original-fs/promises 导致错误
const fsMkdir = (path: string, options?: any) => Promise.resolve(mkdirSync(path, options))
const fsWriteFile = (path: string, data: string | Buffer, options?: any) => Promise.resolve(writeFileSync(path, data, options))
const fsCopyFile = (src: string, dest: string) => Promise.resolve(copyFileSync(src, dest))
const fsReadFile = (path: string, options?: any) => Promise.resolve(readFileSync(path, options))
const fsUnlink = (path: string) => Promise.resolve(unlinkSync(path))
const fsStat = (path: string) => Promise.resolve(statSync(path))
const fsRm = (path: string, options?: any) => Promise.resolve(rmSync(path, options))
const fsRename = (src: string, dest: string) => Promise.resolve(renameSync(src, dest))
const fsReaddir = (path: string, options?: any) => Promise.resolve(readdirSync(path, options))

// 手动递归复制目录（替代 cpSync，因为 original-fs 不支持 cpSync）
function copyDirRecursive(src: string, dest: string): void {
  if (existsSync(dest)) {
    rmSync(dest, { recursive: true, force: true })
  }
  mkdirSync(dest, { recursive: true })

  const entries = readdirSync(src, { withFileTypes: true })
  for (const entry of entries) {
    const srcPath = join(src, entry.name)
    const destPath = join(dest, entry.name)
    if (entry.isDirectory()) {
      copyDirRecursive(srcPath, destPath)
    } else {
      copyFileSync(srcPath, destPath)
    }
  }
}

// ==================== 依赖检测与安装 ====================

export interface DependencyCheckResult {
  installed: boolean
  version?: string
}

export async function checkDependency(key: string): Promise<DependencyCheckResult> {
  const commands: Record<string, string> = {
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
    // migrate.exe 位于 packages/runtime/tools 目录
    const migrateTool = join(process.resourcesPath, '..', 'packages', 'runtime', 'tools', 'migrate.exe')

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
    await fsMkdir(dirname(dbPath), { recursive: true })

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
  newVersion: string
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
        // SQLite 数据库子目录
        const dbTypePath = join(sqlChangeDir, versionDir, 'sqlite')
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

// ==================== 配置文件 ====================

// 读取配置文件原始内容
export async function readConfigFile(configPath: string): Promise<{ success: boolean; content?: string; error?: string }> {
  try {
    if (!existsSync(configPath)) {
      return { success: false, error: '配置文件不存在' }
    }
    const content = await fsReadFile(configPath, 'utf-8')
    return { success: true, content }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : '读取配置失败' }
  }
}

// 写入配置文件（所见即所得）
// 直接写入前端预览的 YAML 内容
export async function writeConfigFile(
  configPath: string,
  yamlContent: string
): Promise<{ success: boolean; error?: string }> {
  try {
    await fsMkdir(dirname(configPath), { recursive: true })
    await fsWriteFile(configPath, yamlContent, 'utf-8')
    console.log('[Config] Written successfully')
    return { success: true }
  } catch (error) {
    console.error('[Config] Write failed:', error)
    return { success: false, error: error instanceof Error ? error.message : '配置写入失败' }
  }
}

// 递归合并对象（以模板结构为准，用户值填充）
// 只保留模板中定义的字段，废弃字段自动丢弃
function mergeObjects(user: any, template: any): any {
  // 模板不是对象，直接返回模板值
  if (typeof template !== 'object' || template === null) {
    return template
  }

  // 用户配置不是对象，返回模板结构
  if (typeof user !== 'object' || user === null) {
    return template
  }

  const result = { ...template }

  // 只遍历模板中的字段（以模板结构为准）
  for (const key of Object.keys(template)) {
    if (user[key] !== undefined) {
      // 用户有值，检查是否需要递归合并
      if (typeof user[key] === 'object' && user[key] !== null &&
          typeof template[key] === 'object' && template[key] !== null &&
          !Array.isArray(template[key])) {
        // 递归合并嵌套对象（非数组）
        result[key] = mergeObjects(user[key], template[key])
      } else {
        // 用户值优先（包括数组，直接替换）
        result[key] = user[key]
      }
    }
    // 用户没有值的字段，保留模板默认值
  }

  return result
}

// ==================== 文件操作 ====================

// 强制结束所有相关进程（不包括当前进程）
export async function killAllProcesses(): Promise<void> {
  try {
    execSync('taskkill /f /im colink-server.exe 2>nul', { encoding: 'utf8' })
  } catch {}

  // 等待进程完全退出
  await new Promise(resolve => setTimeout(resolve, 1500))
}

export async function copyApplicationFiles(
  srcDir: string,
  destDir: string,
  onProgress?: (progress: number) => void
): Promise<{ success: boolean; error?: string }> {
  try {
    await fsMkdir(destDir, { recursive: true })

    const resourcesDir = process.resourcesPath

    if (!existsSync(resourcesDir)) {
      return { success: false, error: `资源目录不存在: ${resourcesDir}` }
    }

    // packages 目录与 resources 并列
    const packagesDir = join(resourcesDir, '..', 'packages')
    const runtimeDir = join(packagesDir, 'runtime')

    if (!existsSync(runtimeDir)) {
      return { success: false, error: `运行时目录不存在: ${runtimeDir}` }
    }

    // ========== 统一的原子替换策略 ==========
    // 原子替换单个文件：先删除目标（如果存在），复制到 .tmp，再 rename
    const atomicReplaceFile = (src: string, dest: string) => {
      if (existsSync(dest)) {
        rmSync(dest, { force: true })
      }
      const tmpPath = dest + '.tmp'
      copyFileSync(src, tmpPath)
      renameSync(tmpPath, dest)
    }

    // 原子替换目录：先删除目标（如果存在），递归复制每个文件
    const atomicReplaceDir = (src: string, dest: string) => {
      if (existsSync(dest)) {
        rmSync(dest, { recursive: true, force: true })
      }
      mkdirSync(dest, { recursive: true })
      const entries = readdirSync(src, { withFileTypes: true })
      for (const entry of entries) {
        const srcPath = join(src, entry.name)
        const destPath = join(dest, entry.name)
        if (entry.isDirectory()) {
          atomicReplaceDir(srcPath, destPath)
        } else {
          atomicReplaceFile(srcPath, destPath)
        }
      }
    }

    // 复制桌面版应用（已包含 server 和 web）
    // 桌面版应用的 resources/bin 和 resources/web 已包含所需文件

    // 同时复制 web 到根目录，供服务器直接访问
    // 服务器 cwd 是安装目录，需要在根目录找到 web/
    const webSrc = join(resourcesDir, '..', 'packages', 'desktop', 'resources', 'app.asar.unpacked', 'resources', 'web')
    const webDest = join(destDir, 'web')
    if (existsSync(webSrc)) {
      atomicReplaceDir(webSrc, webDest)
    } else {
      // Fallback: 从 runtime 目录复制（如果有的话）
      const runtimeWebSrc = join(runtimeDir, 'web')
      if (existsSync(runtimeWebSrc)) {
        atomicReplaceDir(runtimeWebSrc, webDest)
      } else {
        console.warn('[Copy] web/ not found')
      }
    }

    // 创建数据目录（包括 sqlite 目录）
    await fsMkdir(join(destDir, 'data', 'configs'), { recursive: true })
    await fsMkdir(join(destDir, 'data', 'logs'), { recursive: true })
    await fsMkdir(join(destDir, 'data', 'sqlite'), { recursive: true })
    await fsMkdir(join(destDir, 'data', 'agent-assets'), { recursive: true })
    await fsMkdir(join(destDir, 'data', 'agent-configs'), { recursive: true })
    await fsMkdir(join(destDir, 'data', 'repos'), { recursive: true })

    // 复制配置模板
    const templateSrc = join(runtimeDir, 'data', 'configs', 'config.yaml.example')
    const templateDest = join(destDir, 'data', 'configs', 'config.yaml.example')
    if (existsSync(templateSrc)) {
      atomicReplaceFile(templateSrc, templateDest)
      console.log('[Copy] Config template copied')
    }

    // 复制 icon.ico
    const iconSrc = join(resourcesDir, 'icon.ico')
    const iconDest = join(destDir, 'icon.ico')
    if (existsSync(iconSrc)) {
      atomicReplaceFile(iconSrc, iconDest)
    }

    onProgress?.(100)
    return { success: true }
  } catch (error) {
    console.error('[Copy] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '复制失败' }
  }
}

// 检查文件是否被锁定（Windows）
function isFileLocked(filePath: string): boolean {
  try {
    // 尝试以独占模式打开文件
    const fd = execSync(`powershell -Command "try { [System.IO.File]::Open('${filePath}', 'Open', 'ReadWrite', 'None').Close(); 'not_locked' } catch { 'locked' }"`, { encoding: 'utf8' })
    return fd.trim() === 'locked'
  } catch {
    return true // 出错时假设锁定
  }
}

// 杀掉所有Colink相关进程（安装时自动关闭）
// 重要：不杀死安装器自身（当前进程）
// 返回等待的总秒数，用于诊断
function killColinkProcesses(): { killed: boolean; error?: string; waitSeconds?: number } {
  try {
    // 获取当前进程 PID，避免杀死自己
    const currentPid = process.pid
    console.log('[Kill] Current installer PID:', currentPid)

    // 记录初始进程状态
    const initialLauncher = execSync(`tasklist /fi "imagename eq Colink.exe" /fo csv 2>nul`, { encoding: 'utf8' })
    const initialServer = execSync(`tasklist /fi "imagename eq colink-server.exe" /fo csv 2>nul`, { encoding: 'utf8' })
    console.log('[Kill] Initial process status:')
    console.log('[Kill] Colink.exe:', initialLauncher.trim())
    console.log('[Kill] colink-server.exe:', initialServer.trim())

    // 使用 /f 强制终止，多次尝试确保彻底杀死
    // 注意：不杀死 "Colink Setup.exe"，只杀死 Colink.exe 和 colink-server.exe
    for (let round = 0; round < 10; round++) {
      console.log(`[Kill] Round ${round + 1}: killing processes...`)
      execSync('taskkill /f /im Colink.exe 2>nul', { encoding: 'utf8', timeout: 5000 })
      execSync('taskkill /f /im colink-server.exe 2>nul', { encoding: 'utf8', timeout: 5000 })

      execSync('powershell -Command "Start-Sleep -Seconds 1"', { encoding: 'utf8' })

      const launcherOutput = execSync(`tasklist /fi "imagename eq Colink.exe" /fo csv 2>nul`, { encoding: 'utf8' })
      const serverOutput = execSync(`tasklist /fi "imagename eq colink-server.exe" /fo csv 2>nul`, { encoding: 'utf8' })
      const launcherLines = launcherOutput.trim().split('\n').filter(l => l.length > 0)
      const serverLines = serverOutput.trim().split('\n').filter(l => l.length > 0)

      console.log(`[Kill] After round ${round + 1}: Colink lines=${launcherLines.length}, server lines=${serverLines.length}`)

      if (launcherLines.length <= 1 && serverLines.length <= 1) {
        console.log('[Kill] All Colink and colink-server processes terminated')
        break
      }
    }

    // 等待文件句柄释放
    const waitSeconds = 15
    console.log(`[Kill] Waiting ${waitSeconds} seconds for file handles to release...`)
    execSync(`powershell -Command "Start-Sleep -Seconds ${waitSeconds}"`, { encoding: 'utf8' })

    // 最终检查
    const finalLauncher = execSync(`tasklist /fi "imagename eq Colink.exe" /fo csv 2>nul`, { encoding: 'utf8' })
    const finalServer = execSync(`tasklist /fi "imagename eq colink-server.exe" /fo csv 2>nul`, { encoding: 'utf8' })
    console.log('[Kill] Final status:')
    console.log('[Kill] Colink.exe:', finalLauncher.trim())
    console.log('[Kill] colink-server.exe:', finalServer.trim())

    const launcherLines = finalLauncher.trim().split('\n').filter(l => l.length > 0)
    const serverLines = finalServer.trim().split('\n').filter(l => l.length > 0)

    if (launcherLines.length > 1 || serverLines.length > 1) {
      return { killed: false, error: '进程无法关闭，请手动打开任务管理器结束所有 Colink.exe 和 colink-server.exe 后重试', waitSeconds }
    }

    console.log('[Kill] All target processes killed successfully (installer still running)')
    return { killed: true, waitSeconds }
  } catch (e) {
    console.error('[Kill] Error:', e)
    execSync('powershell -Command "Start-Sleep -Seconds 10"', { encoding: 'utf8' })
    return { killed: true, waitSeconds: 10 }
  }
}

// 卸载老版本（保留数据目录）
// 用于升级时先清理老版本程序文件，避免复制冲突
export async function uninstallOldVersion(
  installDir: string,
  mainWindow: BrowserWindow,
  onProgress?: (progress: number) => void
): Promise<{ success: boolean; error?: string }> {
  const sendProgress = (message: string, details?: string) => {
    console.log(`[UninstallOld] ${message}`)
    mainWindow.webContents.send('install-progress', {
      step: 'uninstall',
      status: 'running',
      message,
      details
    })
  }

  try {
    // 自动杀掉运行中的进程（安装时自动关闭）
    sendProgress('检查并关闭运行中的进程...')
    const killResult = killColinkProcesses()
    if (!killResult.killed) {
      const errorMsg = killResult.error || '无法关闭进程'
      sendProgress('进程关闭失败', errorMsg)
      return { success: false, error: errorMsg }
    }
    sendProgress('进程已关闭', 'Colink.exe 和 colink-server.exe 已停止')

    onProgress?.(10)
    await yieldToMain()

    // 删除快捷方式
    sendProgress('删除快捷方式...')
    const desktopPath = process.env.USERPROFILE + '\\Desktop\\Colink.lnk'
    const startMenuPath = process.env.USERPROFILE + '\\AppData\\Roaming\\Microsoft\\Windows\\Start Menu\\Programs\\Colink.lnk'
    try { if (existsSync(desktopPath)) await fsRm(desktopPath) } catch {}
    try { if (existsSync(startMenuPath)) await fsRm(startMenuPath) } catch {}
    onProgress?.(20)
    await yieldToMain()

    // 白名单模式：只保留 data 和 backup 目录
    // resources 目录跳过移动，由 copyLauncherFiles 直接原子替换
    // 原因：resources/app.asar 可能被锁定，移动会失败
    const whitelist = ['data', 'backup', 'resources']
    const entries = await fsReaddir(installDir, { withFileTypes: true }) as Dirent[]
    const entriesToMove = entries.filter(e => !whitelist.includes(e.name))
    await yieldToMain()

    // 特殊处理 resources：不移动，直接在 copyLauncherFiles 中原子替换
    // 但需要先清理旧的 resources/app.asar.unpacked（避免残留）
    const resourcesDir = join(installDir, 'resources')
    if (existsSync(resourcesDir)) {
      sendProgress('检查 resources 目录...')
      // 不移动整个 resources，只清理可能锁定的子目录
      // app.asar 和 app.asar.unpacked 由 copyLauncherFiles 替换
      console.log(`[UninstallOld] resources 目录将在 copyLauncherFiles 中原子替换`)
    }

    // 清理上次升级遗留的 backup 目录
    const backupDir = join(installDir, 'backup')
    if (existsSync(backupDir)) {
      sendProgress('清理 backup 目录...')

      // 统计 backup 目录大小，用于诊断
      try {
        const backupEntries = await fsReaddir(backupDir, { withFileTypes: true }) as Dirent[]
        console.log(`[UninstallOld] backup 目录包含 ${backupEntries.length} 个条目`)
        await yieldToMain()
      } catch {}

      const startTime = Date.now()
      try {
        await fsRm(backupDir, { recursive: true, force: true })
        console.log(`[UninstallOld] backup 清理耗时 ${Date.now() - startTime}ms`)
        await yieldToMain()
      } catch (e) {
        console.warn('[UninstallOld] Failed to clean backup:', e)
        // backup 清理失败不阻止升级，继续尝试
      }
    }

    // 如果没有需要移动的内容，直接返回成功
    if (entriesToMove.length === 0) {
      sendProgress('无需清理', '安装目录只有白名单内容')
      onProgress?.(100)
      return { success: true }
    }

    // 创建 backup 目录
    await fsMkdir(backupDir, { recursive: true })
    await yieldToMain()

    // 带重试的移动函数（对于锁定的文件，重试后强制删除）
    const moveWithRetry = async (src: string, dest: string, maxRetries: number = 5): Promise<{ success: boolean; error?: string }> => {
      for (let attempt = 0; attempt < maxRetries; attempt++) {
        try {
          // 尝试删除目标（如果存在）
          if (existsSync(dest)) {
            rmSync(dest, { recursive: true, force: true })
          }
          // 尝试移动
          renameSync(src, dest)
          return { success: true }
        } catch (e) {
          const errorMsg = e instanceof Error ? e.message : String(e)
          console.log(`[UninstallOld] Move attempt ${attempt + 1} failed for ${src}: ${errorMsg}`)

          if (attempt < maxRetries - 1) {
            // 等待 2 秒后重试
            execSync('powershell -Command "Start-Sleep -Seconds 2"', { encoding: 'utf8' })
          } else {
            // 最后一次尝试失败，尝试强制删除（既然要替换，没必要保留）
            console.log(`[UninstallOld] Force deleting ${src} after move failed`)
            try {
              rmSync(src, { recursive: true, force: true })
              console.log(`[UninstallOld] Force delete succeeded for ${src}`)
              return { success: true } // 删除也算成功，因为新文件会替换
            } catch (deleteErr) {
              console.error(`[UninstallOld] Force delete failed for ${src}:`, deleteErr)
              return { success: false, error: errorMsg }
            }
          }
        }
      }
      return { success: false, error: 'max retries exceeded' }
    }

    // 将非白名单内容移动到 backup 目录（带重试和强制删除）
    const totalEntries = entriesToMove.length
    let processedCount = 0
    const failedEntries: string[] = []

    for (const entry of entriesToMove) {
      const srcPath = join(installDir, entry.name)
      const destPath = join(backupDir, entry.name)
      sendProgress(`处理 ${entry.name}...`)

      console.log(`[UninstallOld] 开始处理: ${entry.name}`)
      const processStartTime = Date.now()

      const result = await moveWithRetry(srcPath, destPath, entry.name === 'resources' ? 7 : 5)

      if (result.success) {
        processedCount++
        console.log(`[UninstallOld] ${entry.name} 处理成功，耗时 ${Date.now() - processStartTime}ms`)
      } else {
        console.error(`[UninstallOld] Failed to process:`, srcPath, result.error)
        failedEntries.push(entry.name)
      }

      // 每处理一个文件后让主线程有机会处理 IPC
      await yieldToMain()
      onProgress?.(Math.round(20 + (processedCount / totalEntries) * 70))
    }

    // 如果有失败的，报错
    if (failedEntries.length > 0) {
      const errorMsg = `以下文件处理失败：${failedEntries.join(', ')}\n请手动关闭相关程序后重试`
      sendProgress('处理失败', errorMsg)
      return { success: false, error: errorMsg }
    }

    // 删除注册表
    sendProgress('清理注册表...')
    deleteRegistry()
    onProgress?.(95)
    await yieldToMain()

    sendProgress('老版本已清理', 'data 和 resources 目录已保留，旧文件已移至 backup')
    onProgress?.(100)

    return { success: true }
  } catch (error) {
    console.error('[UninstallOld] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '卸载老版本失败' }
  }
}

// 复制桌面版应用文件到目标目录
// 从 packages/desktop/ 目录复制完整的桌面版应用
// 采用原子替换策略：先复制到临时目录，再批量替换
export async function copyLauncherFiles(
  sourceDir: string,
  destDir: string,
  onProgress?: (progress: number) => void
): Promise<{ success: boolean; error?: string }> {
  try {
    // 不再前置检测锁定，直接在替换阶段处理
    // 原因：Windows释放文件句柄时间不可预测，前置检测会导致不必要的失败
    // 使用两步替换策略（resources.new）可以绕过锁定问题

    // packages 目录在安装包源目录下
    const desktopSrcDir = join(sourceDir, 'packages', 'desktop')

    if (!existsSync(desktopSrcDir)) {
      return { success: false, error: `桌面版应用目录不存在: ${desktopSrcDir}` }
    }

    const fs = require('original-fs')

    // ========== 统一的原子替换策略 ==========
    // Step 1: 在目标目录的父目录创建 staging（确保同盘，Windows rename 才是原子操作）
    // 例如：目标 D:\Colink，staging 在 D:\.colink-staging-xxx
    const destParent = dirname(destDir)
    const stagingDir = join(destParent, '.colink-staging-' + Date.now())

    // 原子复制单个文件：先复制到 .tmp，再 rename
    const atomicCopyFile = (src: string, dest: string) => {
      const tmpPath = dest + '.tmp'
      fs.copyFileSync(src, tmpPath)
      fs.renameSync(tmpPath, dest)
    }

    // 原子复制目录：递归处理每个文件
    const atomicCopyDir = (src: string, dest: string) => {
      fs.mkdirSync(dest, { recursive: true })
      const entries = fs.readdirSync(src, { withFileTypes: true })
      for (const entry of entries) {
        const srcPath = join(src, entry.name)
        const destPath = join(dest, entry.name)
        if (entry.isDirectory()) {
          atomicCopyDir(srcPath, destPath)
        } else {
          atomicCopyFile(srcPath, destPath)
        }
      }
    }

    // 原子替换单个文件到目标目录
    const atomicReplaceFile = (src: string, dest: string) => {
      if (fs.existsSync(dest)) {
        fs.rmSync(dest, { force: true })
      }
      fs.renameSync(src, dest)
    }

    // 原子替换目录到目标目录（带重试机制）
    // resources 目录特殊处理：不先删除，因为 app.asar 可能被锁定
    const atomicReplaceDir = (src: string, dest: string, maxRetries: number = 3, skipDelete: boolean = false) => {
      for (let attempt = 0; attempt < maxRetries; attempt++) {
        try {
          // 对于 skipDelete 模式，直接尝试 rename（Windows 会覆盖空目录，非空会失败）
          // 对于正常模式，先删除目标再 rename
          if (!skipDelete && fs.existsSync(dest)) {
            fs.rmSync(dest, { recursive: true, force: true })
          }
          fs.renameSync(src, dest)
          return // 成功则退出
        } catch (err) {
          const errorMsg = err instanceof Error ? err.message : String(err)
          console.log(`[Retry] atomicReplaceDir attempt ${attempt + 1} failed for ${dest}: ${errorMsg}`)

          if (attempt < maxRetries - 1) {
            // 等待 2 秒后重试
            execSync('powershell -Command "Start-Sleep -Seconds 2"', { encoding: 'utf8' })
          } else {
            // 最后一次尝试失败，如果目标存在且我们还没删除过，尝试删除后重试一次
            if (fs.existsSync(dest) && skipDelete) {
              console.log(`[Retry] Final attempt: trying to delete ${dest}`)
              try {
                fs.rmSync(dest, { recursive: true, force: true })
                fs.renameSync(src, dest)
                return
              } catch (finalErr) {
                console.error(`[Retry] Final delete+rename failed:`, finalErr)
              }
            }
            throw err // 抛出错误
          }
        }
      }
    }

    // Step 2: 将所有文件原子复制到 staging 目录
    const entries = fs.readdirSync(desktopSrcDir, { withFileTypes: true })
    if (entries.length === 0) {
      return { success: false, error: `桌面版应用目录为空: ${desktopSrcDir}` }
    }

    fs.mkdirSync(stagingDir, { recursive: true })

    let processedFiles = 0
    const totalFiles = entries.length

    for (const entry of entries) {
      const srcPath = join(desktopSrcDir, entry.name)
      const stagingPath = join(stagingDir, entry.name)

      try {
        if (entry.isDirectory()) {
          atomicCopyDir(srcPath, stagingPath)
        } else {
          atomicCopyFile(srcPath, stagingPath)
        }
        processedFiles++
        onProgress?.(Math.round((processedFiles / totalFiles) * 50))
      } catch (copyError) {
        console.error(`[Copy] Failed to copy ${entry.name}:`, copyError)
        // 清理 staging 目录
        try { fs.rmSync(stagingDir, { recursive: true, force: true }) } catch {}
        const errorMsg = copyError instanceof Error ? copyError.message : '未知错误'
        if (errorMsg.includes('EPERM') || errorMsg.includes('EACCES') || errorMsg.includes('being used')) {
          return { success: false, error: `${entry.name} 被锁定，请确保桌面版应用已完全关闭后重试` }
        }
        return { success: false, error: `${entry.name} 复制失败: ${errorMsg}` }
      }
    }

    // Step 3: 从 staging 原子替换到目标目录（带重试机制）
    // 特殊处理 resources 目录：使用两步替换策略避免锁定问题
    for (const entry of entries) {
      const stagingPath = join(stagingDir, entry.name)
      const destPath = join(destDir, entry.name)

      if (!fs.existsSync(stagingPath)) {
        continue  // 跳过未成功复制的文件
      }

      // 对于 resources 目录，使用特殊的两步替换策略
      if (entry.name === 'resources') {
        console.log('[Copy] Using two-step replacement for resources directory')
        const tempDestPath = destPath + '.new'

        try {
          // Step 3a: 先将 staging/resources 移动到 resources.new
          if (fs.existsSync(tempDestPath)) {
            fs.rmSync(tempDestPath, { recursive: true, force: true })
          }
          fs.renameSync(stagingPath, tempDestPath)
          console.log('[Copy] Moved staging/resources to resources.new')

          // Step 3b: 尝试删除旧的 resources（可能失败如果锁定）
          let deleted = false
          for (let attempt = 0; attempt < 10; attempt++) {
            if (!fs.existsSync(destPath)) {
              deleted = true
              break
            }
            try {
              fs.rmSync(destPath, { recursive: true, force: true })
              deleted = true
              console.log('[Copy] Deleted old resources directory')
              break
            } catch (delErr) {
              console.log(`[Copy] Delete attempt ${attempt + 1} failed, retrying...`)
              execSync('powershell -Command "Start-Sleep -Seconds 3"', { encoding: 'utf8' })
            }
          }

          if (!deleted && fs.existsSync(destPath)) {
            // 无法删除旧目录，保留新的在 resources.new，提示用户重启
            console.warn('[Copy] Could not delete old resources, leaving resources.new')
            // 不报错，让用户手动处理或下次重启自动处理
          } else {
            // Step 3c: 将 resources.new 重命名为 resources
            fs.renameSync(tempDestPath, destPath)
            console.log('[Copy] Renamed resources.new to resources')
          }

          processedFiles++
          onProgress?.(Math.round(50 + (processedFiles / totalFiles) * 50))
        } catch (replaceError) {
          console.error('[Copy] Failed to replace resources:', replaceError)
          try { fs.rmSync(stagingDir, { recursive: true, force: true }) } catch {}
          const errorMsg = replaceError instanceof Error ? replaceError.message : '未知错误'
          return { success: false, error: `resources 替换失败: ${errorMsg}\n请关闭 Colink 后重试，或手动删除 ${destPath}` }
        }
      } else {
        // 其他目录/文件使用标准替换
        const maxRetries = 3
        try {
          if (entry.isDirectory()) {
            atomicReplaceDir(stagingPath, destPath, maxRetries, false)
          } else {
            atomicReplaceFile(stagingPath, destPath)
          }
          processedFiles++
          onProgress?.(Math.round(50 + (processedFiles / totalFiles) * 50))
        } catch (replaceError) {
          console.error(`[Copy] Failed to replace ${entry.name}:`, replaceError)
          try { fs.rmSync(stagingDir, { recursive: true, force: true }) } catch {}
          const errorMsg = replaceError instanceof Error ? replaceError.message : '未知错误'
          if (errorMsg.includes('EPERM') || errorMsg.includes('EACCES') || errorMsg.includes('being used')) {
            return { success: false, error: `${entry.name} 替换失败（文件被锁定），请确保桌面版应用已完全关闭后重试` }
          }
          return { success: false, error: `${entry.name} 替换失败: ${errorMsg}` }
        }
      }
    }

    // Step 4: 清理 staging 目录
    try {
      if (fs.existsSync(stagingDir)) {
        fs.rmSync(stagingDir, { recursive: true, force: true })
      }
    } catch {}

    // Step 5: 复制 web 目录到顶层（供后端静态文件服务使用）
    // 后端期望 ./web/，而不是 ./resources/app.asar.unpacked/resources/web/
    const unpackedWebSrc = join(destDir, 'resources', 'app.asar.unpacked', 'resources', 'web')
    const topLevelWebDest = join(destDir, 'web')

    if (fs.existsSync(unpackedWebSrc)) {
      console.log('[Copy] Copying web from unpacked to top level:', unpackedWebSrc, '->', topLevelWebDest)
      try {
        // 先删除目标（如果存在）
        if (fs.existsSync(topLevelWebDest)) {
          fs.rmSync(topLevelWebDest, { recursive: true, force: true })
        }
        // 复制 web 目录
        const entries = fs.readdirSync(unpackedWebSrc, { withFileTypes: true })
        fs.mkdirSync(topLevelWebDest, { recursive: true })
        for (const entry of entries) {
          const srcPath = join(unpackedWebSrc, entry.name)
          const destPath = join(topLevelWebDest, entry.name)
          if (entry.isDirectory()) {
            atomicCopyDir(srcPath, destPath)
          } else {
            atomicCopyFile(srcPath, destPath)
          }
        }
        console.log('[Copy] Web copied to top level successfully')
      } catch (webCopyErr) {
        console.error('[Copy] Failed to copy web to top level:', webCopyErr)
        // 不阻止安装，但记录错误
      }
    } else {
      console.warn('[Copy] Unpacked web not found at:', unpackedWebSrc)
    }

    // Step 6: 复制 data 目录到顶层（供后端数据存储使用）
    // 配置文件中 data.base_path 默认是 ./data
    // 新结构下 data 在 packages/runtime/data，需要复制到顶层
    const runtimeDataSrc = join(sourceDir, 'packages', 'runtime', 'data')
    const topLevelDataDest = join(destDir, 'data')

    if (fs.existsSync(runtimeDataSrc)) {
      console.log('[Copy] Copying data from runtime to top level:', runtimeDataSrc, '->', topLevelDataDest)
      try {
        // data 目录可能已存在（copyApplicationFiles 创建了空目录结构）
        // 只复制必要的子目录，避免覆盖用户数据
        const dataSubDirs = ['configs', 'sql-change']
        for (const subDir of dataSubDirs) {
          const srcSubDir = join(runtimeDataSrc, subDir)
          const destSubDir = join(topLevelDataDest, subDir)
          if (fs.existsSync(srcSubDir)) {
            if (!fs.existsSync(destSubDir)) {
              fs.mkdirSync(destSubDir, { recursive: true })
            }
            const subEntries = fs.readdirSync(srcSubDir, { withFileTypes: true })
            for (const entry of subEntries) {
              const srcPath = join(srcSubDir, entry.name)
              const destPath = join(destSubDir, entry.name)
              if (entry.isDirectory()) {
                atomicCopyDir(srcPath, destPath)
              } else {
                // 只复制模板文件，不覆盖已有配置
                if (entry.name.includes('.example') || !fs.existsSync(destPath)) {
                  atomicCopyFile(srcPath, destPath)
                }
              }
            }
          }
        }
        console.log('[Copy] Data copied to top level successfully')
      } catch (dataCopyErr) {
        console.error('[Copy] Failed to copy data to top level:', dataCopyErr)
      }
    } else {
      console.warn('[Copy] Runtime data not found at:', runtimeDataSrc)
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
  database: { type: 'sqlite' }
  serverPort?: number
}): Promise<{ success: boolean; yaml?: string; error?: string }> {
  try {
    // 1. 读取模板作为基础
    const templatePath = findConfigTemplate()
    let baseYaml: any = {}

    if (templatePath) {
      const templateContent = await fsReadFile(templatePath, 'utf-8')
      baseYaml = YAML.parse(templateContent)
    }

    // 2. 如果有本地配置，合入（用户值覆盖模板）
    if (params.installDir) {
      const configPath = join(params.installDir, 'data', 'configs', 'config.yaml')
      if (existsSync(configPath)) {
        const userContent = await fsReadFile(configPath, 'utf-8')
        const userYaml = YAML.parse(userContent)
        baseYaml = mergeObjects(userYaml, baseYaml)
      }
    }

    // 3. 应用页面修改的参数（强制覆盖）
    if (baseYaml?.database) {
      baseYaml.database.type = 'sqlite'
      baseYaml.database.path = './data/sqlite/colink.db'
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
  // 打包后：从 packages/runtime/data/configs 目录读取
  const packagedPath = join(process.resourcesPath, '..', 'packages', 'runtime', 'data', 'configs', 'config.yaml.example')
  console.log('[ConfigTemplate] Checking packaged path:', packagedPath, 'exists:', existsSync(packagedPath))
  if (existsSync(packagedPath)) {
    return packagedPath
  }

  // 开发时：从项目根目录读取
  // __dirname 在开发模式下是 installer/out/main/
  const devPaths = [
    // installer/out/main/ -> isdp/configs/
    join(__dirname, '../../../configs/config.yaml.example'),
    // installer/out/main/ -> installer/packages/runtime/data/configs/
    join(__dirname, '../../packages/runtime/data/configs/config.yaml.example'),
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

// 读取已有配置
export async function readExistingConfig(installDir: string): Promise<{
  success: boolean
  config?: {
    database: { type: 'sqlite' }
    serverPort?: number
  }
  error?: string
}> {
  try {
    const configPath = join(installDir, 'data', 'configs', 'config.yaml')
    if (!existsSync(configPath)) {
      return { success: false, error: '配置文件不存在' }
    }

    const content = await fsReadFile(configPath, 'utf-8')
    const parsed = YAML.parse(content)

    // 从配置文件读取端口，如果没有则尝试从模板读取默认值
    let serverPort = parsed?.server?.port
    if (!serverPort) {
      const templatePath = findConfigTemplate()
      if (templatePath) {
        const templateContent = await fsReadFile(templatePath, 'utf-8')
        const templateYaml = YAML.parse(templateContent)
        serverPort = templateYaml?.server?.port
      }
    }

    return {
      success: true,
      config: {
        database: { type: 'sqlite' },
        serverPort: serverPort
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
    await fsWriteFile(vbsPath, vbsContent, 'utf-8')
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
    await fsWriteFile(vbsPath, vbsContent, 'utf-8')
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
    database: { type: 'sqlite' }
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
    // Step 0: 卸载老版本（如果已安装）
    // 采用卸载重装策略，避免文件锁定和复制冲突问题
    // 保留 data 目录，数据库 goose 版本信息不会丢失
    // 允许重复安装，不需要校验版本号
    if (existsSync(config.installDir)) {
      sendProgress('uninstall', 'running', 0, '卸载老版本...', `安装目录: ${config.installDir}\n保留数据目录`)
      const uninstallResult = await uninstallOldVersion(config.installDir, mainWindow, (p) => {
        sendProgress('uninstall', 'running', p, `卸载老版本 ${p}%...`)
      })
      if (!uninstallResult.success) {
        sendProgress('uninstall', 'failed', 0, uninstallResult.error, `卸载老版本失败: ${uninstallResult.error}`)
        return uninstallResult
      }
      sendProgress('uninstall', 'success', 100, '老版本已卸载', '数据目录已保留，数据库版本信息完整')
    } else {
      sendProgress('uninstall', 'success', 100, '跳过卸载', '首次安装，无需卸载老版本')
    }

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
      sendProgress('copy', 'running', 50 + Math.round(p * 0.5), `复制桌面版应用 ${50 + Math.round(p * 0.5)}%...`)
    })
    if (!launcherResult.success) {
      sendProgress('copy', 'failed', 0, launcherResult.error, `复制桌面版应用失败: ${launcherResult.error}`)
      return launcherResult
    }
    sendProgress('copy', 'success', 100, '文件复制完成', `已复制所有文件到 ${config.installDir}`)

    // sql-change 从 packages/runtime/data 目录读取，不复制到用户安装目录
    const sqlChangeDir = join(process.resourcesPath, '..', 'packages', 'runtime', 'data', 'sql-change')

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

    // Step 1.6: SQLite 数据库迁移
    const dbPath = join(config.installDir, 'data', 'sqlite', 'colink.db')

    if (config.currentVersion && config.newVersion) {
      // 升级场景：自动执行迁移
      const currentVer = config.currentVersion || '0.0.0'
      const targetVer = config.newVersion || '1.2.5'

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
      await fsMkdir(dirname(dbPath), { recursive: true })

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
    } else {
      // 新安装：按版本顺序执行所有迁移（从 v1.1.0 到最新版本）
      // 扫描所有版本目录
      const allVersionDirs = readdirSync(sqlChangeDir)
        .filter(f => f.startsWith('v') && existsSync(join(sqlChangeDir, f, 'sqlite')))
        .sort()

      if (allVersionDirs.length === 0) {
        sendProgress('migration', 'warning', 100, '无迁移脚本', 'sql-change 目录下未找到迁移脚本')
      } else {
        // 创建数据库目录
        await fsMkdir(dirname(dbPath), { recursive: true })

        const migrationDetails: string[] = []

        // 按版本顺序逐个执行
        for (let i = 0; i < allVersionDirs.length; i++) {
          const versionDir = allVersionDirs[i]
          const progress = Math.round(((i + 1) / allVersionDirs.length) * 100)

          sendProgress('migration', 'running', progress,
            `初始化数据库 ${versionDir}...`,
            `执行 sql-change/${versionDir}/sqlite/ 下的脚本（共 ${allVersionDirs.length} 个版本）`)

          const result = await runDatabaseMigration(
            dbPath,
            sqlChangeDir,
            versionDir.replace('v', ''),
            mainWindow
          )

          if (!result.success) {
            sendProgress('migration', 'failed', 0, '数据库初始化失败', result.error || '未知错误')
            return { success: false, error: `数据库初始化失败 (${versionDir}): ${result.error}` }
          }

          if (result.message) {
            migrationDetails.push(`${versionDir}: ${result.message}`)
          }
        }

        const detailStr = migrationDetails.length > 0
          ? migrationDetails.join('\n')
          : `所有版本已执行（${allVersionDirs.length} 个）`
        sendProgress('migration', 'success', 100, 'SQLite 数据库初始化完成', detailStr)
      }
    }

    // Step 2: 配置文件处理（所见即所得）
    sendProgress('config', 'running', 0, '写入配置文件...', `配置路径: ${config.installDir}/data/configs/config.yaml`)
    const configPath = join(config.installDir, 'data', 'configs', 'config.yaml')

    // 直接写入前端预览的 YAML 内容
    if (config.configYaml) {
      const writeResult = await writeConfigFile(configPath, config.configYaml)
      if (!writeResult.success) {
        console.warn('[Config] Write failed:', writeResult.error)
        sendProgress('config', 'warning', 100, '配置写入失败', writeResult.error)
      } else {
        sendProgress('config', 'success', 100, '配置已写入', '数据库: SQLite ./data/sqlite/colink.db')
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

    // Step 3: 创建快捷方式
    sendProgress('shortcut', 'running', 0, '创建快捷方式...', '桌面和开始菜单')
    if (config.createShortcut !== false) {
      await createDesktopShortcut(config.installDir)
      await createStartMenuShortcut(config.installDir)
      sendProgress('shortcut', 'success', 100, '快捷方式已创建', '桌面快捷方式和开始菜单已创建')
    } else {
      sendProgress('shortcut', 'success', 100, '跳过快捷方式', '用户选择不创建')
    }

    // Step 4: 写入注册表
    sendProgress('registry', 'running', 0, '写入注册表...', `版本: ${config.newVersion || '1.0.0'}`)
    await writeRegistry(config.installDir, config.newVersion || '1.0.0')
    sendProgress('registry', 'success', 100, '注册表已写入', `安装信息已注册到系统\n安装目录: ${config.installDir}`)

    return { success: true, dbChanges }
  } catch (error) {
    console.error('[Install] Error:', error)
    return { success: false, error: error instanceof Error ? error.message : '安装失败' }
  }
}