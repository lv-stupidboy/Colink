import { useState, useEffect } from 'react'
import { Button, Space, Spin, Card, Tag, Divider, Modal, message } from 'antd'
import { PlayCircleOutlined, StopOutlined, SettingOutlined, FileTextOutlined, FolderOutlined, DeleteOutlined, ReloadOutlined, RedoOutlined } from '@ant-design/icons'
import { InstallConfig, InstalledVersion } from './types'
import Layout from './components/Layout'
import Welcome from './pages/Welcome'
import DirectorySelect from './pages/DirectorySelect'
import DependencyCheck from './pages/DependencyCheck'
import ModeSelect from './pages/ModeSelect'
import SystemConfig from './pages/SystemConfig'
import Installing from './pages/Installing'
import Complete from './pages/Complete'
import Dashboard from './pages/Dashboard'
import { LauncherDashboard } from './pages/LauncherDashboard'

type AppMode = 'checking' | 'launcher' | 'dashboard' | 'install' | 'installing' | 'complete'

const INSTALL_PAGES = {
  1: Welcome,
  2: DirectorySelect,
  3: DependencyCheck,
  4: ModeSelect,
  5: SystemConfig,
}

const STEP_LABELS = ['欢迎', '目录选择', '依赖检测', '模式选择', '系统配置']
const UPGRADE_STEP_LABELS = ['欢迎', '依赖检测', '系统配置']

export default function App() {
  const [mode, setMode] = useState<AppMode>('checking')
  const [currentStep, setCurrentStep] = useState(1)
  const [installedVersion, setInstalledVersion] = useState<InstalledVersion | null>(null)
  const [serviceStatus, setServiceStatus] = useState<'running' | 'stopped'>('stopped')
  const [config, setConfig] = useState<InstallConfig>({
    installDir: 'C:\\Program Files\\ISDP',
    installMode: 'auto',
    dependencies: [],
    database: { host: '', port: 3306, database: 'isdp', username: 'root', password: '' },
    createShortcut: true,
    launchNow: true,
    keepData: true,
  })
  const [hasMissingDeps, setHasMissingDeps] = useState(false)
  const [dirValid, setDirValid] = useState(true)

  useEffect(() => {
    checkInstalledVersion()
  }, [])

  // 定期更新服务状态
  useEffect(() => {
    if (mode === 'dashboard' || mode === 'launcher') {
      updateServiceStatus()
      const timer = setInterval(updateServiceStatus, 5000)
      return () => clearInterval(timer)
    }
  }, [mode])

  const checkInstalledVersion = async () => {
    try {
      // 首先检测是否是 Launcher 模式
      const isLauncher = await window.electronAPI.isLauncherMode()
      if (isLauncher) {
        // Launcher 模式，显示简化控制面板
        const result = await window.electronAPI.checkInstalled()
        setInstalledVersion(result)
        setMode('launcher')
        updateServiceStatus()
        return
      }

      // Setup 模式，检测安装状态
      const result = await window.electronAPI.checkInstalled()
      setInstalledVersion(result)
      if (result.installed && result.installDir) {
        setConfig(prev => ({ ...prev, installDir: result.installDir!, keepData: true }))
        setMode('dashboard')
        updateServiceStatus()
      } else {
        setMode('install')
      }
    } catch {
      setMode('install')
    }
  }

  const updateServiceStatus = async () => {
    try {
      const result = await window.electronAPI.getServiceStatus()
      setServiceStatus(result.status)
    } catch {
      setServiceStatus('stopped')
    }
  }

  const handleStartInstall = () => {
    setMode('install')
    setCurrentStep(1)
  }

  const handleStartService = async () => {
    const result = await window.electronAPI.startService()
    if (result.success) {
      setServiceStatus('running')
      message.success('服务已启动')
    } else {
      message.error(result.error || '启动失败')
    }
  }

  const handleStopService = async () => {
    await window.electronAPI.stopService()
    setServiceStatus('stopped')
    message.success('服务已停止')
  }

  const handleUninstall = async () => {
    // 先调用后端确认对话框
    const confirmResult = await window.electronAPI.confirmUninstall()
    if (!confirmResult.confirmed) return

    // 执行卸载
    const result = await window.electronAPI.uninstall(confirmResult.keepData)
    if (result.success) {
      message.success('卸载成功')
      checkInstalledVersion()
    } else {
      message.error(result.error || '卸载失败')
    }
  }

  const isUpgrade = installedVersion?.installed === true

  const getStepLabels = () => isUpgrade ? UPGRADE_STEP_LABELS : STEP_LABELS

  const handleNext = () => {
    if (currentStep >= 5) return
    let nextStep = currentStep + 1
    if (isUpgrade) {
      if (currentStep === 1) nextStep = 3
      else if (currentStep === 3 && !hasMissingDeps) nextStep = 5
    } else {
      if (currentStep === 3 && !hasMissingDeps) nextStep = 5
    }
    setCurrentStep(nextStep)
  }

  const handlePrev = () => {
    if (currentStep <= 1) return
    let prevStep = currentStep - 1
    if (isUpgrade) {
      if (currentStep === 5) prevStep = 3
      else if (currentStep === 3) prevStep = 1
    } else {
      if (currentStep === 5 && !hasMissingDeps) prevStep = 3
    }
    setCurrentStep(prevStep)
  }

  const handleInstall = async () => {
    setMode('installing')
    const result = await window.electronAPI.startInstallation(config)
    if (result.success) {
      setMode('complete')
    } else {
      message.error(result.error || '安装失败')
      setMode('install')
    }
  }

  const handleComplete = async () => {
    await checkInstalledVersion()
  }

  // 检测中
  if (mode === 'checking') {
    return (
      <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" tip="检测安装环境..." />
      </div>
    )
  }

  // Launcher 模式（简化控制面板）
  if (mode === 'launcher') {
    return (
      <Layout hideSteps>
        <LauncherDashboard
          installDir={installedVersion?.installDir || ''}
          serviceStatus={serviceStatus}
          onStartService={handleStartService}
          onStopService={handleStopService}
        />
      </Layout>
    )
  }

  // 控制面板
  if (mode === 'dashboard') {
    return (
      <Layout hideSteps>
        <Dashboard
          installDir={installedVersion?.installDir || ''}
          serviceStatus={serviceStatus}
          onStartService={handleStartService}
          onStopService={handleStopService}
          onReinstall={handleStartInstall}
          onUninstall={handleUninstall}
        />
      </Layout>
    )
  }

  // 安装中
  if (mode === 'installing') {
    return (
      <Layout hideSteps>
        <Installing config={config} onComplete={handleComplete} isUpgrade={isUpgrade} />
      </Layout>
    )
  }

  // 安装完成
  if (mode === 'complete') {
    return (
      <Layout hideSteps>
        <Complete
          config={config}
          onComplete={async () => {
            await checkInstalledVersion()
          }}
        />
      </Layout>
    )
  }

  // 安装向导
  const PageComponent = INSTALL_PAGES[currentStep as keyof typeof INSTALL_PAGES]
  const stepIndex = isUpgrade
    ? (currentStep === 1 ? 0 : currentStep === 3 ? 1 : 2)
    : currentStep - 1

  return (
    <Layout currentStep={stepIndex + 1} stepLabels={getStepLabels()}>
      <PageComponent
        config={config}
        onConfigUpdate={(updates) => setConfig(prev => ({ ...prev, ...updates }))}
        onDependenciesUpdate={(deps) => {
          setConfig(prev => ({ ...prev, dependencies: deps }))
          setHasMissingDeps(deps.some(d => !d.installed))
        }}
        onValidationChange={(valid) => setDirValid(valid)}
        installedVersion={installedVersion}
        isUpgrade={isUpgrade}
      />

      <div className="footer-buttons">
        <Space>
          {currentStep > 1 && !(isUpgrade && currentStep === 3) && (
            <Button onClick={handlePrev}>上一步</Button>
          )}
          {currentStep === 1 && (
            <Button type="primary" onClick={handleNext}>
              {isUpgrade ? '开始升级' : '开始安装'}
            </Button>
          )}
          {currentStep > 1 && currentStep < 5 && (
            <Button type="primary" onClick={handleNext} disabled={currentStep === 2 && !dirValid}>下一步</Button>
          )}
          {currentStep === 5 && (
            <Button type="primary" onClick={handleInstall}>
              {isUpgrade ? '升级' : '安装'}
            </Button>
          )}
        </Space>
      </div>
    </Layout>
  )
}