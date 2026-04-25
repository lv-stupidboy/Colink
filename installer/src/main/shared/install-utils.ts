import { execSync } from 'child_process'
import { join } from 'path'
import { existsSync, readdirSync } from 'fs'

/**
 * 检测已安装的 Colink 版本
 */
export function getInstalledVersion(): { installed: boolean; installDir?: string; version?: string; hasData?: boolean } {
  try {
    // 使用 PowerShell 方式读取注册表，更可靠
    const psCommand = `
      try {
        $key = Get-ItemProperty -Path 'HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink' -ErrorAction SilentlyContinue
        if ($key) { $key.InstallLocation }
      } catch {}
      try {
        $key = Get-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink' -ErrorAction SilentlyContinue
        if ($key) { $key.InstallLocation }
      } catch {}
    `

    let installDir: string | null = null

    try {
      const output = execSync(
        `powershell -NoProfile -Command "${psCommand}"`,
        { encoding: 'utf8', timeout: 5000 }
      )

      // PowerShell 输出可能是多行，找到非空的行
      const lines = output.trim().split('\n').filter(l => l.trim())
      if (lines.length > 0 && lines[0].trim()) {
        installDir = lines[0].trim()
      }
    } catch {
      // PowerShell 失败，回退到 reg query
      try {
        let regQuery: string
        try {
          regQuery = execSync(
            'reg query "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink" /v InstallLocation 2>nul',
            { encoding: 'utf8' }
          )
        } catch {
          regQuery = execSync(
            'reg query "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink" /v InstallLocation 2>nul',
            { encoding: 'utf8' }
          )
        }

        // 更宽松的正则匹配，支持不同格式的输出
        const match = regQuery.match(/InstallLocation\s+REG_SZ\s+(.+)/i)
        if (match) {
          installDir = match[1].trim()
        }
      } catch {
        // reg query 也失败
      }
    }

    if (installDir && existsSync(installDir)) {
      const dataDir = join(installDir, 'data')
      const hasData = existsSync(dataDir) && readdirSync(dataDir).length > 0

      // 从注册表读取已安装版本
      let version: string | undefined
      try {
        const versionPsCommand = `
          try {
            $key = Get-ItemProperty -Path 'HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink' -ErrorAction SilentlyContinue
            if ($key) { $key.DisplayVersion }
          } catch {}
          try {
            $key = Get-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink' -ErrorAction SilentlyContinue
            if ($key) { $key.DisplayVersion }
          } catch {}
        `
        const versionOutput = execSync(
          `powershell -NoProfile -Command "${versionPsCommand}"`,
          { encoding: 'utf8', timeout: 5000 }
        )
        const versionLines = versionOutput.trim().split('\n').filter(l => l.trim())
        if (versionLines.length > 0) {
          version = versionLines[0].trim()
        }
      } catch {
        // 忽略读取错误
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
  try {
    let regQuery: string
    try {
      regQuery = execSync(
        'reg query "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v InstallLocation 2>nul',
        { encoding: 'utf8' }
      )
    } catch {
      regQuery = execSync(
        'reg query "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v InstallLocation 2>nul',
        { encoding: 'utf8' }
      )
    }
    const match = regQuery.match(/InstallLocation\s+REG_SZ\s+(.+)/)
    if (match) {
      const dir = match[1].trim()

      // 从注册表读取已安装版本
      let version: string | undefined
      try {
        let versionQuery: string
        try {
          versionQuery = execSync(
            'reg query "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v DisplayVersion 2>nul',
            { encoding: 'utf8' }
          )
        } catch {
          versionQuery = execSync(
            'reg query "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /v DisplayVersion 2>nul',
            { encoding: 'utf8' }
          )
        }
        const versionMatch = versionQuery.match(/DisplayVersion\s+REG_SZ\s+(.+)/)
        if (versionMatch) {
          version = versionMatch[1].trim()
        }
      } catch {
        // 忽略读取错误
      }

      return { installed: true, installDir: dir, version }
    }
  } catch {
    // 未安装
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
    const fs = require('fs')

    // 尝试运行旧版的卸载程序
    const uninstallerPath = join(dir, 'uninstall.exe')
    if (existsSync(uninstallerPath)) {
      try {
        // 静默卸载
        execSync(`"${uninstallerPath}" /S`, { timeout: 60000 })
      } catch {
        // 卸载程序可能失败，继续手动清理
      }
    }

    // 清理注册表
    try {
      execSync('reg delete "HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /f 2>nul')
    } catch {}
    try {
      execSync('reg delete "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP" /f 2>nul')
    } catch {}

    // 删除旧版快捷方式
    const desktopPath = process.env.USERPROFILE + '\\Desktop\\ISDP.lnk'
    const startMenuPath = process.env.APPDATA + '\\Microsoft\\Windows\\Start Menu\\Programs\\ISDP.lnk'
    try { if (existsSync(desktopPath)) fs.rmSync(desktopPath) } catch {}
    try { if (existsSync(startMenuPath)) fs.rmSync(startMenuPath) } catch {}

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
          fs.rmSync(path, { recursive: true, force: true })
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