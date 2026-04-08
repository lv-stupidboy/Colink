import { execSync } from 'child_process'
import { join } from 'path'
import { existsSync, readdirSync } from 'fs'

/**
 * 检测已安装的ISDP版本
 */
export function getInstalledVersion(): { installed: boolean; installDir?: string; version?: string; hasData?: boolean } {
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
      const dataDir = join(dir, 'data')
      const hasData = existsSync(dataDir) && readdirSync(dataDir).length > 0

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

      return { installed: true, installDir: dir, version, hasData }
    }
  } catch {
    // 未安装
  }
  return { installed: false }
}