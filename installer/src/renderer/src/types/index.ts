// 安装步骤
export type StepId = 1 | 2 | 3 | 4 | 5 | 6 | 7

// 依赖项
export interface Dependency {
  name: string           // 显示名称
  key: string            // 标识符: 'nodejs' | 'git' | 'claude' | 'opencode'
  required: boolean      // 是否必需
  version?: string       // 检测到的版本
  installed: boolean     // 是否已安装
}

// 安装模式
export type InstallMode = 'auto' | 'manual' | 'skip'

// 数据库配置
export interface DatabaseConfig {
  host: string
  port: number
  database: string
  username: string
  password: string
}

// 安装配置
export interface InstallConfig {
  installDir: string
  installMode: InstallMode
  dependencies: Dependency[]
  database: DatabaseConfig
  createShortcut: boolean
  launchNow: boolean
}

// 安装进度
export interface InstallProgress {
  step: string
  status: 'pending' | 'running' | 'success' | 'failed'
  progress?: number  // 0-100
  message?: string
}

// Electron API 类型声明
declare global {
  interface Window {
    electronAPI: {
      minimizeWindow: () => void
      closeWindow: () => void
      getAppPath: () => Promise<string>
      getResourcePath: () => Promise<string>
      selectDirectory: () => Promise<string | null>
      getDiskSpace: (path: string) => Promise<{ free: number; total: number }>
      checkDependency: (dep: string) => Promise<{ installed: boolean; version?: string }>
      installDependency: (dep: string) => Promise<{ success: boolean; error?: string }>
      startInstallation: (config: object) => Promise<{ success: boolean; error?: string }>
      copyFiles: (src: string, dest: string) => Promise<{ success: boolean; error?: string }>
      generateConfig: (config: object) => Promise<{ success: boolean; error?: string }>
      testDatabaseConnection: (config: object) => Promise<{ success: boolean; error?: string }>
      createShortcut: (path: string) => Promise<{ success: boolean }>
      launchService: (installDir: string) => Promise<{ success: boolean; error?: string }>
      onInstallProgress: (callback: (progress: InstallProgress) => void) => void
    }
  }
}

export {}