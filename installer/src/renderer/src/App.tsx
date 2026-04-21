import { useState, useEffect } from 'react'
import { Button, Space, Spin, Card, Tag, Divider, Modal, message, Typography } from 'antd'
import { PlayCircleOutlined, StopOutlined, SettingOutlined, FileTextOutlined, FolderOutlined, DeleteOutlined, ReloadOutlined, RedoOutlined, CloudUploadOutlined, CloseOutlined, WarningOutlined } from '@ant-design/icons'
import { InstallConfig, InstalledVersion } from './types'
import Layout from './components/Layout'
import Welcome from './pages/Welcome'
import InviteVerification from './pages/InviteVerification'
import DirectorySelect from './pages/DirectorySelect'
import DependencyCheck from './pages/DependencyCheck'
import ModeSelect from './pages/ModeSelect'
import SystemConfig from './pages/SystemConfig'
import Installing from './pages/Installing'
import Complete from './pages/Complete'
import { LauncherDashboard } from './pages/LauncherDashboard'
import SelectAction from './pages/SelectAction'

const { Text, Title } = Typography

type AppMode = 'checking' | 'launcher' | 'select-action' | 'old-version-detected' | 'install' | 'installing' | 'complete'

const INSTALL_PAGES = {
  1: Welcome,
  2: InviteVerification,
  3: DirectorySelect,
  4: DependencyCheck,
  5: SystemConfig,
}

const STEP_LABELS = ['欢迎', '验证邀请码', '目录选择', '智能体检测', '系统配置']
const UPGRADE_STEP_LABELS = ['欢迎', '验证邀请码', '智能体检测', '系统配置']

export default function App() {
  const [mode, setMode] = useState<AppMode>('checking')
  const [currentStep, setCurrentStep] = useState(1)
  const [installedVersion, setInstalledVersion] = useState<InstalledVersion | undefined>(undefined)
  const [serviceStatus, setServiceStatus] = useState<'running' | 'stopped'>('stopped')
  const [config, setConfig] = useState<InstallConfig>({
    installDir: 'C:\\Program Files\\Colink',
    installMode: 'auto',
    dependencies: [],
    database: { type: 'sqlite', host: '', port: 3306, database: 'isdp', username: 'root', password: '' },
    serverPort: 8080,
    createShortcut: true,
    launchNow: true,
    keepData: true,
  })
  const [hasMissingDeps, setHasMissingDeps] = useState(false)
  const [dirValid, setDirValid] = useState(true)
  const [inviteVerified, setInviteVerified] = useState(false)
  const [oldISDPVersion, setOldISDPVersion] = useState<InstalledVersion | null>(null)
  const [uninstallingOld, setUninstallingOld] = useState(false)

  useEffect(() => {
    checkInstalledVersion()
  }, [])

  // 定期更新服务状态
  useEffect(() => {
    if (mode === 'launcher') {
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

      // Setup 模式，检测新版本安装状态
      const result = await window.electronAPI.checkInstalled()
      setInstalledVersion(result)

      if (result.installed && result.installDir) {
        setConfig(prev => ({ ...prev, installDir: result.installDir!, keepData: true }))
        setMode('select-action')  // 显示选择页面
        return
      }

      // 检测旧版 ISDP（品牌更名前）
      const oldResult = await window.electronAPI.checkOldISDP()
      if (oldResult.installed) {
        setOldISDPVersion(oldResult)
        setMode('old-version-detected')
        return
      }

      // 全新安装
      setMode('install')
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
      if (currentStep === 1) nextStep = 2  // Welcome -> InviteVerification
      else if (currentStep === 2) nextStep = 4  // InviteVerification -> DependencyCheck
      else if (currentStep === 4) nextStep = 5  // DependencyCheck -> SystemConfig
    }
    setCurrentStep(nextStep)
  }

  const handlePrev = () => {
    if (currentStep <= 1) return
    let prevStep = currentStep - 1
    if (isUpgrade) {
      if (currentStep === 5) prevStep = 4  // SystemConfig -> DependencyCheck
      else if (currentStep === 4) prevStep = 2  // DependencyCheck -> InviteVerification
      else if (currentStep === 2) prevStep = 1  // InviteVerification -> Welcome
    }
    setCurrentStep(prevStep)
  }

  const handleInstall = async () => {
    // 只切换到安装页面，让 Installing 组件自己管理安装流程
    setMode('installing')
  }

  const handleInstallComplete = () => {
    // 用户在 Installing 页面点击完成后，直接关闭窗口
    window.electronAPI.closeWindow()
  }

  const handleComplete = async () => {
    // 完成页面用户点击完成后，检测安装状态并跳转
    await checkInstalledVersion()
  }

  const handleUninstallOldISDP = async () => {
    setUninstallingOld(true)
    const result = await window.electronAPI.uninstallOldISDP()
    setUninstallingOld(false)

    if (result.success) {
      message.success('旧版本已卸载')
      setOldISDPVersion(null)
      setMode('install')
    } else {
      message.error(result.error || '卸载失败')
    }
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
      <Layout hideSteps title="Colink">
        <LauncherDashboard
          installDir={installedVersion?.installDir || ''}
          serviceStatus={serviceStatus}
          onStartService={handleStartService}
          onStopService={handleStopService}
        />
      </Layout>
    )
  }

  // 选择操作页面（已安装）
  if (mode === 'select-action') {
    return (
      <Layout hideSteps title="Colink Setup">
        <SelectAction
          installDir={installedVersion?.installDir || ''}
          onUpgrade={handleStartInstall}
          onUninstall={handleUninstall}
          onCancel={() => window.electronAPI.closeWindow()}
        />
      </Layout>
    )
  }

  // 检测到旧版本 ISDP
  if (mode === 'old-version-detected') {
    return (
      <Layout hideSteps title="Colink Setup">
        <div style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          padding: 24
        }}>
          <WarningOutlined style={{ fontSize: 64, color: '#faad14', marginBottom: 24 }} />
          <Title level={3} style={{ marginBottom: 8 }}>检测到旧版本 ISDP</Title>
          <Text type="secondary" style={{ marginBottom: 8, textAlign: 'center' }}>
            产品已更名为 Colink，需要先卸载旧版本才能继续安装。
          </Text>
          {oldISDPVersion?.installDir && (
            <Card size="small" style={{ marginBottom: 24, minWidth: 300 }}>
              <Text type="secondary">安装位置：</Text>
              <Text code>{oldISDPVersion.installDir}</Text>
              {oldISDPVersion.version && (
                <>
                  <br />
                  <Text type="secondary">版本：</Text>
                  <Text>{oldISDPVersion.version}</Text>
                </>
              )}
            </Card>
          )}
          <Space direction="vertical" style={{ width: '100%', maxWidth: 400 }} size="middle">
            <Button
              type="primary"
              size="large"
              block
              loading={uninstallingOld}
              onClick={handleUninstallOldISDP}
            >
              卸载旧版本并继续安装
            </Button>
            <Button
              size="large"
              block
              onClick={() => window.electronAPI.closeWindow()}
            >
              取消
            </Button>
          </Space>
          <Divider style={{ marginTop: 40, marginBottom: 16 }} />
          <Text type="secondary" style={{ fontSize: 12 }}>
            卸载时会尝试保留数据目录（config、logs等）
          </Text>
        </div>
      </Layout>
    )
  }

  // 安装中
  if (mode === 'installing') {
    return (
      <Layout hideSteps title="Colink Setup">
        <Installing config={config} onComplete={handleInstallComplete} isUpgrade={isUpgrade} />
      </Layout>
    )
  }

  // 安装完成
  if (mode === 'complete') {
    return (
      <Layout hideSteps title="Colink Setup">
        <Complete
          config={config}
          isUpgrade={isUpgrade}
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
    ? (currentStep === 1 ? 0 : currentStep === 2 ? 1 : currentStep === 4 ? 2 : 3)
    : currentStep - 1

  return (
    <Layout currentStep={stepIndex + 1} stepLabels={getStepLabels()} title="Colink Setup">
      <PageComponent
        config={config}
        onConfigUpdate={(updates) => setConfig(prev => ({ ...prev, ...updates }))}
        onDependenciesUpdate={(deps) => {
          setConfig(prev => ({ ...prev, dependencies: deps }))
          setHasMissingDeps(deps.some(d => !d.installed))
        }}
        onValidationChange={(valid) => {
          // Step 2: InviteVerification 验证状态
          if (currentStep === 2) setInviteVerified(valid)
          // Step 3: DirectorySelect 目录验证状态
          if (currentStep === 3) setDirValid(valid)
        }}
        installedVersion={installedVersion}
        isUpgrade={isUpgrade}
      />

      <div className="footer-buttons">
        <Space>
          {currentStep > 1 && !(isUpgrade && currentStep === 4) && !(isUpgrade && currentStep === 2) && (
            <Button onClick={handlePrev}>上一步</Button>
          )}
          {currentStep === 1 && (
            <Button type="primary" onClick={handleNext}>
              {isUpgrade ? '开始升级' : '开始安装'}
            </Button>
          )}
          {currentStep === 2 && inviteVerified && (
            <Button type="primary" onClick={handleNext}>下一步</Button>
          )}
          {currentStep > 2 && currentStep < 5 && (
            <Button type="primary" onClick={handleNext} disabled={currentStep === 3 && !dirValid}>下一步</Button>
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