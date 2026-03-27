import { useEffect, useState } from 'react'
import { Progress, Button, message } from 'antd'
import { CheckCircleOutlined, LoadingOutlined, CloseCircleOutlined } from '@ant-design/icons'
import { InstallConfig, InstallProgress, InstalledVersion } from '../types'

interface InstallingProps {
  config: InstallConfig
  onComplete: () => void
  installedVersion?: InstalledVersion
  isUpgrade?: boolean
}

interface StepProgress {
  step: string
  status: 'pending' | 'running' | 'success' | 'failed'
  progress: number
  error?: string
}

const INSTALL_STEPS = [
  { key: 'prepare', label: '准备工作' },
  { key: 'copy', label: '复制文件' },
  { key: 'claude', label: '安装 Claude CLI' },
  { key: 'opencode', label: '安装 OpenCode' },
  { key: 'config', label: '生成配置文件' },
  { key: 'shortcut', label: '创建快捷方式' },
  { key: 'registry', label: '写入注册表' },
]

export default function Installing({ config, onComplete, isUpgrade }: InstallingProps) {
  const [steps, setSteps] = useState<StepProgress[]>(
    INSTALL_STEPS.map(s => ({ step: s.key, status: 'pending', progress: 0 }))
  )
  const [installError, setInstallError] = useState<string | null>(null)
  const [installComplete, setInstallComplete] = useState(false)

  useEffect(() => {
    let isMounted = true

    // 监听安装进度
    window.electronAPI.onInstallProgress((progress: InstallProgress) => {
      if (!isMounted) return

      setSteps(prev => prev.map(s =>
        s.step === progress.step
          ? { ...s, status: progress.status, progress: progress.progress || 0, error: progress.message }
          : s
      ))

      if (progress.status === 'failed') {
        setInstallError(progress.message || `${progress.step} 失败`)
      }

      // 只有在没有错误且 registry 成功时才完成
      if (progress.status === 'success' && progress.step === 'registry' && !installError) {
        setInstallComplete(true)
        setTimeout(onComplete, 500)
      }
    })

    // 启动安装
    console.log('Starting installation with config:', config)
    window.electronAPI.startInstallation(config).then(result => {
      if (!isMounted) return
      console.log('Installation result:', result)
      if (!result.success) {
        setInstallError(result.error || '安装失败')
        message.error(result.error || '安装失败')
      }
    }).catch(err => {
      if (!isMounted) return
      console.error('Installation error:', err)
      setInstallError(err.message || '安装过程出错')
    })

    return () => {
      isMounted = false
    }
  }, [config, onComplete, installError])

  const getStepIcon = (status: string) => {
    switch (status) {
      case 'success': return <CheckCircleOutlined style={{ color: '#52c41a' }} />
      case 'running': return <LoadingOutlined style={{ color: '#10b981' }} />
      case 'failed': return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />
      default: return <span style={{ color: '#d9d9d9' }}>○</span>
    }
  }

  const handleClose = () => {
    window.electronAPI.closeWindow()
  }

  // 显示所有步骤
  const visibleSteps = steps

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>
        {installError ? (isUpgrade ? '升级失败' : '安装失败') : (isUpgrade ? '正在升级...' : '正在安装...')}
      </h2>
      <p style={{ color: '#666', marginBottom: 30 }}>
        {installError ? '安装过程中遇到错误，请检查后重试' : '请稍候，安装程序正在配置您的系统'}
      </p>

      {installError && (
        <div style={{
          background: '#fff2f0',
          border: '1px solid #ffccc7',
          borderRadius: 8,
          padding: 16,
          marginBottom: 20,
          color: '#cf1322'
        }}>
          <strong>安装失败：</strong>{installError}
        </div>
      )}

      <div>
        {visibleSteps.map((step, index) => (
          <div
            key={step.step}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 16,
              padding: '16px 0',
              borderBottom: index < visibleSteps.length - 1 ? '1px solid #f0f0f0' : 'none',
            }}
          >
            <div style={{ width: 24 }}>{getStepIcon(step.status)}</div>
            <div style={{ flex: 1 }}>{INSTALL_STEPS.find(s => s.key === step.step)?.label}</div>
            {step.status === 'running' && (
              <div style={{ width: 150 }}>
                <Progress percent={step.progress} size="small" />
              </div>
            )}
            {step.status === 'success' && (
              <span style={{ color: '#52c41a' }}>完成</span>
            )}
            {step.status === 'failed' && (
              <span style={{ color: '#ff4d4f' }}>失败 {step.error && `(${step.error})`}</span>
            )}
            {step.status === 'pending' && (
              <span style={{ color: '#999' }}>等待中</span>
            )}
          </div>
        ))}
      </div>

      {installError && (
        <div style={{ marginTop: 24, textAlign: 'right' }}>
          <Button type="primary" onClick={handleClose}>
            关闭
          </Button>
        </div>
      )}
    </div>
  )
}