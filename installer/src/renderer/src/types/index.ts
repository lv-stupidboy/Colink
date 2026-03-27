// 安装步骤
export type StepId = 1 | 2 | 3 | 4 | 5

// 依赖项
export interface Dependency {
  name: string
  key: string
  required: boolean
  version?: string
  installed: boolean
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
  serverPort?: number
  createShortcut: boolean
  launchNow: boolean
  keepData?: boolean
}

// 安装进度
export interface InstallProgress {
  step: string
  status: 'pending' | 'running' | 'success' | 'failed'
  progress?: number
  message?: string
}

// 已安装版本信息
export interface InstalledVersion {
  installed: boolean
  installDir?: string
  version?: string
  hasData?: boolean
}

// 已有配置信息
export interface ExistingConfig {
  database: DatabaseConfig
  serverPort?: number
}

// Electron API 类型声明
declare global {
  interface Window {
    electronAPI: {
      // 窗口控制
      minimizeWindow: () => void
      closeWindow: () => void

      // 应用模式
      isLauncherMode: () => Promise<boolean>
      getStartupAction: () => Promise<'install' | 'upgrade' | 'uninstall'>

      // 路径获取
      getAppPath: () => Promise<string>
      getResourcePath: () => Promise<string>

      // 安装相关
      selectDirectory: () => Promise<string | null>
      getDiskSpace: (path: string) => Promise<{ free: number; total: number }>
      checkDependency: (dep: string) => Promise<{ installed: boolean; version?: string }>
      installDependency: (dep: string) => Promise<{ success: boolean; error?: string }>
      startInstallation: (config: object) => Promise<{ success: boolean; error?: string }>
      generateConfig: (config: object) => Promise<{ success: boolean; error?: string }>
      testDatabaseConnection: (config: object) => Promise<{ success: boolean; error?: string }>
      createShortcut: (path: string) => Promise<{ success: boolean }>

      // 安装状态
      checkInstalled: () => Promise<InstalledVersion>
      readExistingConfig: (installDir: string) => Promise<{ success: boolean; config?: ExistingConfig; error?: string }>

      // 服务管理
      startService: () => Promise<{ success: boolean; error?: string }>
      stopService: () => Promise<{ success: boolean }>
      getServiceStatus: () => Promise<{ status: 'running' | 'stopped' }>

      // 快捷操作
      openLogs: () => Promise<void>
      openDataDir: () => Promise<void>
      openConfig: () => Promise<void>
      openConsole: () => Promise<void>

      // 卸载
      confirmUninstall: () => Promise<{ confirmed: boolean; keepData: boolean }>
      uninstall: (keepData: boolean) => Promise<{ success: boolean; error?: string }>

      // 进度回调
      onInstallProgress: (callback: (progress: InstallProgress) => void) => void
    }
  }
}

export {}