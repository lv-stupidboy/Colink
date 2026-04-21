// 安装步骤
export type StepId = 1 | 2 | 3 | 4 | 5 | 6

// 依赖项
export interface Dependency {
  name: string
  key: string
  required: boolean
  version?: string
  installed: boolean
}

// 安装模式
export type InstallMode = 'launcher-install' | 'skip'

// 数据库配置
export interface DatabaseConfig {
  type: 'sqlite' | 'mysql'  // 数据库类型，默认 sqlite
  host?: string             // MySQL 主机
  port?: number             // MySQL 端口
  database?: string         // MySQL 数据库名
  username?: string         // MySQL 用户名
  password?: string         // MySQL 密码
}

// 验证请求
export interface InviteVerificationRequest {
  code: string
  username: string
}

// 验证响应
export interface InviteVerificationResponse {
  success: boolean
  message: string
  token?: string
  user?: { id?: string; username?: string }
}

// 验证状态（内存中的验证结果）
export interface VerificationState {
  verified: boolean
  token?: string
  username?: string
  verifiedAt?: number
}

// 持久化的邀请码信息（文件保存）
export interface SavedInviteCode {
  inviteCode: string
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
  verification?: VerificationState
  configYaml?: string  // 预览的完整配置 YAML（所见即所得）
}

// 安装进度
export interface InstallProgress {
  step: string
  status: 'pending' | 'running' | 'success' | 'failed' | 'warning'
  progress?: number
  message?: string
  details?: string  // 步骤详情
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

// 运行中的Agent实例
export interface RunningAgentInstance {
  invocationId: string
  agentName: string
  projectName: string
  threadTitle: string
  startedAt: string
  runningDurationSeconds: number
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
      startInstallation: (config: object) => Promise<{ success: boolean; error?: string; dbChanges?: Array<{ version: string; files: string[] }> }>
      testDatabaseConnection: (config: object) => Promise<{ success: boolean; error?: string }>
      createShortcut: (path: string) => Promise<{ success: boolean }>

      // 邀请码验证
      verifyInviteCode: (request: InviteVerificationRequest) => Promise<InviteVerificationResponse>
      saveInviteCode: (inviteCode: string, installDir?: string) => Promise<{ success: boolean; message?: string }>
      loadInviteCode: (installDir?: string) => Promise<{ success: boolean; inviteCode?: string; message?: string }>

      // 获取系统用户名
      getSystemUsername: () => Promise<string>

      // 安装状态
      checkInstalled: () => Promise<InstalledVersion>
      checkOldISDP: () => Promise<InstalledVersion>
      uninstallOldISDP: () => Promise<{ success: boolean; error?: string }>
      readExistingConfig: (installDir: string) => Promise<{ success: boolean; config?: ExistingConfig; error?: string }>
      generateConfigPreview: (params: {
        installDir?: string
        database: DatabaseConfig
        serverPort?: number
      }) => Promise<{ success: boolean; yaml?: string; error?: string }>

      // 服务管理
      startService: () => Promise<{ success: boolean; error?: string }>
      stopService: () => Promise<{ success: boolean }>
      getServiceStatus: () => Promise<{ status: 'running' | 'stopped' }>
      getRunningAgents: () => Promise<{ instances: RunningAgentInstance[] }>

      // 依赖管理（启动器）
      checkAllDependencies: () => Promise<Array<{ key: string; name: string; installed: boolean; version?: string }>>

      // 配置编辑（启动器）
      readConfigFile: () => Promise<{ success: boolean; content?: string; error?: string }>
      getConfigPreview: () => Promise<{ success: boolean; yaml?: string; error?: string }>
      saveConfig: (yaml: string) => Promise<{ success: boolean; error?: string }>
      getExistingConfig: () => Promise<{ success: boolean; config?: ExistingConfig; error?: string }>

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