import { useEffect, useState } from 'react'
import { Progress } from 'antd'
import { CheckCircleOutlined, LoadingOutlined } from '@ant-design/icons'
import { InstallConfig, InstallProgress } from '../types'

interface InstallingProps {
  config: InstallConfig
  onComplete: () => void
}

interface StepProgress {
  step: string
  status: 'pending' | 'running' | 'success' | 'failed'
  progress: number
}

const INSTALL_STEPS = [
  { key: 'copy', label: '复制文件' },
  { key: 'claude', label: '安装 Claude CLI' },
  { key: 'opencode', label: '安装 OpenCode' },
  { key: 'config', label: '生成配置文件' },
]

export default function Installing({ config, onComplete }: InstallingProps) {
  const [steps, setSteps] = useState<StepProgress[]>(
    INSTALL_STEPS.map(s => ({ step: s.key, status: 'pending', progress: 0 }))
  )

  useEffect(() => {
    // 监听安装进度
    window.electronAPI.onInstallProgress((progress: InstallProgress) => {
      setSteps(prev => prev.map(s =>
        s.step === progress.step
          ? { ...s, status: progress.status, progress: progress.progress || 0 }
          : s
      ))

      if (progress.status === 'success' && progress.step === 'config') {
        setTimeout(onComplete, 500)
      }
    })

    // 启动安装
    window.electronAPI.startInstallation(config).then(result => {
      if (!result.success) {
        console.error('Installation failed:', result.error)
      }
    })
  }, [config, onComplete])

  const getStepIcon = (status: string) => {
    switch (status) {
      case 'success': return <CheckCircleOutlined style={{ color: '#52c41a' }} />
      case 'running': return <LoadingOutlined style={{ color: '#10b981' }} />
      default: return <span style={{ color: '#d9d9d9' }}>○</span>
    }
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>正在安装...</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>请稍候，安装程序正在配置您的系统</p>

      <div>
        {steps.map((step, index) => (
          <div
            key={step.step}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 16,
              padding: '16px 0',
              borderBottom: index < steps.length - 1 ? '1px solid #f0f0f0' : 'none',
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
            {step.status === 'pending' && (
              <span style={{ color: '#999' }}>等待中</span>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}