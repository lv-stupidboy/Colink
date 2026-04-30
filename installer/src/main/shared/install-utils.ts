import { execFileSync } from 'child_process'
import { join } from 'path'
import { existsSync, readdirSync, rmSync } from 'fs'

/**
 * 安全执行 reg query 命令
 */
function safeRegQuery(key: string, value: string): string | null {
  try {
    const output = execFileSync('reg', ['query', key, '/v', value], {
      encoding: 'utf8',
      timeout: 5000
    })
    const match = output.match(new RegExp(`${value}\\s+REG_SZ\\s+(.+)`, 'i'))
    return match ? match[1].trim() : null
  } catch {
    return null
  }
}

/**
 * 安全执行 reg delete 命令
 */
function safeRegDelete(key: string): boolean {
  try {
    execFileSync('reg', ['delete', key, '/f'], { timeout: 5000 })
    return true
  } catch {
    return false
  }
}

/**
 * 检测已安装的 Colink 版本
 */
export function getInstalledVersion(): { installed: boolean; installDir?: string; version?: string; hasData?: boolean } {
  try {
    let installDir: string | null = null

    // 优先使用 reg query（安全方式）
    installDir = safeRegQuery('HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink', 'InstallLocation')
    if (!installDir) {
      installDir = safeRegQuery('HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink', 'InstallLocation')
    }

    if (installDir && existsSync(installDir)) {
      const dataDir = join(installDir, 'data')
      const hasData = existsSync(dataDir) && readdirSync(dataDir).length > 0

      // 从注册表读取已安装版本
      let version: string | undefined
      version = safeRegQuery('HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink', 'DisplayVersion')
      if (!version) {
        version = safeRegQuery('HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink', 'DisplayVersion')
      }

      console.log('[InstallUtils] Found installation:', installDir, 'version:', version)
      return { installed: true, installDir, version, hasData }
    }
  } catch (e) {
    console.warn('[InstallUtils] Detection failed:', e)
  }
  return { installed: false }
}

/**
 * 检测旧版 ISDP 安装（品牌更名前的版本）
 * 用于提示用户卸载旧版本
 */
export function getOldISDPVersion(): { installed: boolean; installDir?: string; version?: string } {
  const installDir = safeRegQuery('HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP', 'InstallLocation')
    || safeRegQuery('HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP', 'InstallLocation')

  if (installDir) {
    const version = safeRegQuery('HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP', 'DisplayVersion')
      || safeRegQuery('HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP', 'DisplayVersion')
    return { installed: true, installDir, version }
  }
  return { installed: false }
}

/**
 * 卸载旧版 ISDP（静默卸载）
 * 删除程序文件但保留数据目录
 */
export function uninstallOldISDP(): { success: boolean; error?: string } {
  try {
    const oldVersion = getOldISDPVersion()
    if (!oldVersion.installed || !oldVersion.installDir) {
      return { success: true }
    }

    const dir = oldVersion.installDir

    // 尝试运行旧版的卸载程序
    const uninstallerPath = join(dir, 'uninstall.exe')
    if (existsSync(uninstallerPath)) {
      try {
        // 静默卸载（使用 execFileSync，无 shell 避免 injection）
        execFileSync(uninstallerPath, ['/S'], { timeout: 60000 })
      } catch {
        // 卸载程序可能失败，继续手动清理
      }
    }

    // 清理注册表（使用安全方式）
    safeRegDelete('HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP')
    safeRegDelete('HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP')

    // 删除旧版快捷方式
    const desktopPath = join(process.env.USERPROFILE || '', 'Desktop', 'ISDP.lnk')
    const startMenuPath = join(process.env.APPDATA || '', 'Microsoft', 'Windows', 'Start Menu', 'Programs', 'ISDP.lnk')
    try { if (existsSync(desktopPath)) rmSync(desktopPath) } catch {}
    try { if (existsSync(startMenuPath)) rmSync(startMenuPath) } catch {}

    // 删除老安装目录中的程序文件（保留 data 目录）
    const entriesToDelete = [
      // 可执行文件
      'Colink.exe', 'colink-server.exe',
      'uninstall.exe',
      // DLL 文件
      'ffmpeg.dll', 'd3dcompiler_47.dll', 'libEGL.dll', 'libGLESv2.dll',
      'vk_swiftshader.dll', 'vulkan-1.dll',
      // 其他文件
      'resources.pak', 'chrome_100_percent.pak', 'chrome_200_percent.pak',
      'icudtl.dat', 'snapshot_blob.bin', 'v8_context_snapshot.bin',
      'vk_swiftshader_icd.json', 'icon.ico',
      'LICENSE.electron.txt', 'LICENSES.chromium.html',
      // 目录
      'locales', 'resources', 'web'
    ]

    for (const entry of entriesToDelete) {
      const path = join(dir, entry)
      if (existsSync(path)) {
        try {
          rmSync(path, { recursive: true, force: true })
          console.log('[UninstallOld] Removed:', entry)
        } catch (e) {
          console.warn('[UninstallOld] Failed to remove:', path, e)
        }
      }
    }

    return { success: true }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : '卸载失败' }
  }
}