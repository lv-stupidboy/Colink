import { basename } from 'path'

export type AppMode = 'setup' | 'launcher'

/**
 * 检测当前应用的运行模式
 * - setup: ISDP Setup 安装向导
 * - launcher: ISDP 启动器/控制面板
 */
export function detectAppMode(): AppMode {
  const exeName = basename(process.execPath, '.exe').toLowerCase()

  // 如果 exe 名为 "isdp setup" 或 "isdp-setup"，始终是 setup 模式
  if (exeName.includes('setup')) {
    return 'setup'
  }

  // 如果 exe 名为 "isdp"，检查资源路径
  if (exeName === 'isdp') {
    // 在打包后的应用中，launcher 的资源路径会包含 "launcher"
    // 因为它是从 release/launcher/win-unpacked/ 打包的
    if (process.resourcesPath) {
      // 开发模式下的判断
      if (process.resourcesPath.includes('launcher')) {
        return 'launcher'
      }
      // 生产模式下，检查是否在安装目录运行
      // launcher 运行时 resourcesPath 会是安装目录下的 resources/
      // setup 运行时 resourcesPath 会在 release/win-unpacked/resources/
      if (process.resourcesPath.includes('release') && !process.resourcesPath.includes('Program Files')) {
        return 'setup'
      }
    }
    // 默认为 launcher 模式（安装后的 ISDP.exe）
    return 'launcher'
  }

  // 其他情况默认为 setup 模式
  return 'setup'
}

/**
 * 判断当前是否为 Launcher 模式
 */
export function isLauncherMode(): boolean {
  return detectAppMode() === 'launcher'
}

/**
 * 判断当前是否为 Setup 模式
 */
export function isSetupMode(): boolean {
  return detectAppMode() === 'setup'
}