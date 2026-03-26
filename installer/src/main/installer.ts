import { exec, spawn } from 'child_process'
import { promisify } from 'util'
import { copyFile, mkdir, writeFile, readdir, stat } from 'fs/promises'
import { createWriteStream, existsSync } from 'fs'
import { join, dirname } from 'path'
import { app, BrowserWindow, shell } from 'electron'
import { tmpdir } from 'os'
import { https } from 'follow-redirects'

const execAsync = promisify(exec)

const DOWNLOAD_URLS = {
  nodejs: 'https://nodejs.org/dist/v20.11.0/node-v20.11.0-x64.msi',
  git: 'https://github.com/git-for-windows/git/releases/download/v2.43.0.windows.1/Git-2.43.0-64-bit.exe'
}

async function downloadFile(url: string, dest: string, onProgress?: (progress: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const file = createWriteStream(dest)
    https.get(url, (response) => {
      const totalSize = parseInt(response.headers['content-length'] || '0', 10)
      let downloaded = 0

      response.on('data', (chunk) => {
        downloaded += chunk.length
        if (totalSize > 0 && onProgress) {
          onProgress(Math.round((downloaded / totalSize) * 100))
        }
      })

      response.pipe(file)
      file.on('finish', () => {
        file.close()
        resolve()
      })
    }).on('error', (err) => {
      reject(err)
    })
  })
}

async function runInstaller(filePath: string, args: string[]): Promise<void> {
  return new Promise((resolve, reject) => {
    const proc = spawn(filePath, args, {
      detached: true,
      stdio: 'ignore',
    })
    proc.on('close', (code) => {
      if (code === 0) resolve()
      else reject(new Error(`Installer exited with code ${code}`))
    })
    proc.on('error', reject)
  })
}

export async function installNodejs(onProgress?: (progress: number) => void): Promise<{ success: boolean; error?: string }> {
  try {
    const check = await checkDependency('nodejs')
    if (check.installed) {
      return { success: true }
    }

    const destPath = join(tmpdir(), 'node-installer.msi')
    await downloadFile(DOWNLOAD_URLS.nodejs, destPath, onProgress)
    await runInstaller('msiexec.exe', ['/i', destPath, '/quiet', '/norestart'])

    return { success: true }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : '安装失败' }
  }
}

export async function installGit(onProgress?: (progress: number) => void): Promise<{ success: boolean; error?: string }> {
  try {
    const check = await checkDependency('git')
    if (check.installed) {
      return { success: true }
    }

    const destPath = join(tmpdir(), 'git-installer.exe')
    await downloadFile(DOWNLOAD_URLS.git, destPath, onProgress)
    await runInstaller(destPath, ['/VERYSILENT', '/NORESTART', '/NOCANCEL'])

    return { success: true }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : '安装失败' }
  }
}

export interface DependencyCheckResult {
  installed: boolean
  version?: string
}

// 检测依赖
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

// 安装 npm 包
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

// 需要排除的文件列表（开发调试文件，不需要打包）
const EXCLUDE_FILES = new Set([
  'debug.html',
  'style-preview.html',
  'theme-guide.md',
])

// 递归复制目录
async function copyDir(src: string, dest: string, onProgress?: (progress: number) => void): Promise<void> {
  await mkdir(dest, { recursive: true })
  const entries = await readdir(src, { withFileTypes: true })

  // 过滤掉需要排除的文件
  const filteredEntries = entries.filter(entry => !EXCLUDE_FILES.has(entry.name))

  let copied = 0
  const total = filteredEntries.length

  for (const entry of filteredEntries) {
    const srcPath = join(src, entry.name)
    const destPath = join(dest, entry.name)

    if (entry.isDirectory()) {
      await copyDir(srcPath, destPath)
    } else {
      await copyFile(srcPath, destPath)
    }

    copied++
    onProgress?.(Math.round((copied / total) * 100))
  }
}

// 复制应用文件
export async function copyApplicationFiles(
  srcDir: string,
  destDir: string,
  onProgress?: (progress: number) => void
): Promise<{ success: boolean; error?: string }> {
  try {
    await mkdir(destDir, { recursive: true })

    // 复制服务器可执行文件
    await copyFile(join(srcDir, 'isdp-server.exe'), join(destDir, 'isdp-server.exe'))

    // 复制前端静态文件
    const webSrc = join(srcDir, 'web')
    const webDest = join(destDir, 'web')
    await copyDir(webSrc, webDest, onProgress)

    // 创建日志目录
    await mkdir(join(destDir, 'logs'), { recursive: true })

    return { success: true }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '复制失败',
    }
  }
}

// 生成配置文件
export async function generateConfigFile(
  destPath: string,
  content: string
): Promise<{ success: boolean; error?: string }> {
  try {
    await mkdir(dirname(destPath), { recursive: true })
    await writeFile(destPath, content, 'utf-8')
    return { success: true }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '生成配置失败',
    }
  }
}

// 创建桌面快捷方式
export async function createDesktopShortcut(installDir: string): Promise<boolean> {
  try {
    const launcherPath = join(installDir, 'ISDP-Launcher.exe')

    const psScript = `
      $WshShell = New-Object -ComObject WScript.Shell
      $Shortcut = $WshShell.CreateShortcut("$env:USERPROFILE\\Desktop\\ISDP.lnk")
      $Shortcut.TargetPath = "${launcherPath.replace(/\\/g, '\\\\')}"
      $Shortcut.Arguments = "--launcher"
      $Shortcut.WorkingDirectory = "${installDir.replace(/\\/g, '\\\\')}"
      $Shortcut.Description = "ISDP 智能开发平台"
      $Shortcut.Save()
    `

    await execAsync(`powershell -Command "${psScript.replace(/"/g, '\\"').replace(/\n/g, ' ')}"`)

    return true
  } catch (error) {
    console.error('Failed to create shortcut:', error)
    return false
  }
}

// 生成配置文件内容
function generateConfigYaml(db: { host: string; port: number; database: string; username: string; password: string }): string {
  return `server:
  port: 8080
  mode: release

database:
  type: mysql
  mysql:
    host: ${db.host}
    port: ${db.port}
    database: ${db.database}
    username: ${db.username}
    password: ${db.password}
    charset: utf8mb4

claude:
  path: claude
  default_model: claude-sonnet-4-6
  timeout: 30m

logging:
  level: info
  format: json
`
}

// 运行安装流程
export async function runInstallation(
  config: {
    installDir: string
    installMode: string
    dependencies: Array<{ key: string; installed: boolean }>
    database: { host: string; port: number; database: string; username: string; password: string }
  },
  resourcePath: string,
  mainWindow: BrowserWindow
): Promise<{ success: boolean; error?: string }> {
  const sendProgress = (step: string, status: string, progress?: number) => {
    mainWindow.webContents.send('install-progress', { step, status, progress })
  }

  try {
    // Step 1: 复制文件
    sendProgress('copy', 'running', 0)
    const srcDir = join(resourcePath, 'app')
    const result = await copyApplicationFiles(srcDir, config.installDir, (p) => {
      sendProgress('copy', 'running', p)
    })
    if (!result.success) return result
    sendProgress('copy', 'success', 100)

    // Step 2: 安装 Claude CLI（如果选择自动安装）
    if (config.installMode === 'auto') {
      const claudeMissing = config.dependencies.find(d => d.key === 'claude' && !d.installed)
      if (claudeMissing) {
        sendProgress('claude', 'running', 0)
        const result = await installNpmPackage('@anthropic-ai/claude-cli')
        if (!result.success) {
          sendProgress('claude', 'failed', 0)
          // 可选依赖失败不阻止安装
        } else {
          sendProgress('claude', 'success', 100)
        }
      }
    }
    sendProgress('claude', 'success', 100)

    // Step 3: 安装 OpenCode（如果选择自动安装）
    if (config.installMode === 'auto') {
      const opencodeMissing = config.dependencies.find(d => d.key === 'opencode' && !d.installed)
      if (opencodeMissing) {
        sendProgress('opencode', 'running', 0)
        const result = await installNpmPackage('@anthropic-ai/opencode')
        if (!result.success) {
          sendProgress('opencode', 'failed', 0)
        } else {
          sendProgress('opencode', 'success', 100)
        }
      }
    }
    sendProgress('opencode', 'success', 100)

    // Step 4: 生成配置文件
    sendProgress('config', 'running', 0)
    const configContent = generateConfigYaml(config.database)
    const configResult = await generateConfigFile(
      join(config.installDir, 'config.yaml'),
      configContent
    )
    if (!configResult.success) return configResult
    sendProgress('config', 'success', 100)

    return { success: true }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : '安装失败',
    }
  }
}