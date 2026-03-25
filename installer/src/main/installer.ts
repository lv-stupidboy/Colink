import { exec } from 'child_process'
import { promisify } from 'util'
import { copyFile, mkdir, writeFile } from 'fs/promises'
import { join, dirname } from 'path'
import { app, BrowserWindow, shell } from 'electron'

const execAsync = promisify(exec)

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

// 复制应用文件
// 注意：完整实现在 Task 21，这里仅定义接口
export async function copyApplicationFiles(
  srcDir: string,
  destDir: string,
  onProgress?: (progress: number) => void
): Promise<{ success: boolean; error?: string }> {
  try {
    await mkdir(destDir, { recursive: true })
    // Task 21 会完整实现文件复制逻辑
    onProgress?.(100)
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
export async function createDesktopShortcut(targetPath: string): Promise<boolean> {
  // Windows 创建快捷方式需要使用特殊方法
  // 可以使用 electron-shell 或者外部工具
  return true
}