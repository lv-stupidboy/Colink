import { useEffect, useState } from 'react'
import { Progress, message } from 'antd'
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

  useEffect(() => {
    // 监听安装进度
    window.electronAPI.onInstallProgress((progress: InstallProgress) => {
      setSteps(prev => prev.map(s =>
        s.step === progress.step
          ? { ...s, status: progress.status, progress: progress.progress || 0, error: progress.message }
          : s
      ))

      if (progress.status === 'failed') {
        setInstallError(progress.message || `${progress.step} 失败`)
      }

      if (progress.status === 'success' && progress.step === 'registry') {
        setTimeout(onComplete, 500)
      }
    })

    // 启动安装
    console.log('Starting installation with config:', config)
    window.electronAPI.startInstallation(config).then(result => {
      console.log('Installation result:', result)
      if (!result.success) {
        setInstallError(result.error || '安装失败')
        message.error(result.error || '安装失败')
      }
    }).catch(err => {
      console.error('Installation error:', err)
      setInstallError(err.message || '安装过程出错')
    })
  }, [config, onComplete])

  const getStepIcon = (status: string) => {
    switch (status) {
      case 'success': return <CheckCircleOutlined style={{ color: '#52c41a' }} />
      case 'running': return <LoadingOutlined style={{ color: '#10b981' }} />
      case 'failed': return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />
      default: return <span style={{ color: '#d9d9d9' }}>○</span>
    }
  }

  // 显示所有步骤
  const visibleSteps = steps

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>
        {isUpgrade ? '正在升级...' : '正在安装...'}
      </h2>
      <p style={{ color: '#666', marginBottom: 30 }}>请稍候，安装程序正在配置您的系统</p>

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
    </div>
  )
}