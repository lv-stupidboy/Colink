import { useEffect, useState } from 'react'
import { Progress, Button, Tag, Alert, message } from 'antd'
import { CheckCircleOutlined, LoadingOutlined, CloseCircleOutlined, RightOutlined, WarningOutlined, DatabaseOutlined } from '@ant-design/icons'
import { InstallConfig, InstallProgress, InstalledVersion } from '../types'

interface InstallingProps {
  config: InstallConfig
  onComplete: () => void
  installedVersion?: InstalledVersion
  isUpgrade?: boolean
}

interface StepProgress {
  step: string
  label: string
  status: 'pending' | 'running' | 'success' | 'failed' | 'warning'
  progress: number
  message?: string
  details?: string
  startTime?: number
  endTime?: number
}

// 完整的安装步骤定义
const INSTALL_STEPS = [
  { key: 'prepare', label: '准备工作', description: '初始化安装环境' },
  { key: 'copy', label: '复制文件', description: '复制应用程序文件到安装目录' },
  { key: 'dbcheck', label: '检测数据库变更', description: '检查是否需要执行数据库迁移' },
  { key: 'migration', label: '数据库迁移', description: '执行 SQLite 数据库迁移脚本' },
  { key: 'claude', label: '安装 Claude CLI', description: '安装 Anthropic Claude CLI 工具' },
  { key: 'opencode', label: '安装 OpenCode', description: '安装 OpenCode 工具' },
  { key: 'config', label: '生成配置文件', description: '合并用户配置与模板配置' },
  { key: 'shortcut', label: '创建快捷方式', description: '创建桌面和开始菜单快捷方式' },
  { key: 'registry', label: '写入注册表', description: '注册安装信息到系统' },
]

export default function Installing({ config, onComplete, isUpgrade }: InstallingProps) {
  const [steps, setSteps] = useState<StepProgress[]>(
    INSTALL_STEPS.map(s => ({
      step: s.key,
      label: s.label,
      status: 'pending',
      progress: 0
    }))
  )
  const [installError, setInstallError] = useState<string | null>(null)
  const [installComplete, setInstallComplete] = useState(false)
  const [dbChanges, setDbChanges] = useState<Array<{ version: string; files: string[] }> | null>(null)
  const [expandedSteps, setExpandedSteps] = useState<string[]>([])

  useEffect(() => {
    let isMounted = true

    // 监听安装进度
    window.electronAPI.onInstallProgress((progress: InstallProgress) => {
      if (!isMounted) return

      console.log('[Install Progress]', progress)

      setSteps(prev => prev.map(s => {
        if (s.step === progress.step) {
          return {
            ...s,
            status: progress.status,
            progress: progress.progress || 0,
            message: progress.message,
            details: progress.details,
            endTime: progress.status !== 'running' ? Date.now() : undefined,
            startTime: s.startTime || (progress.status === 'running' ? Date.now() : undefined)
          }
        }
        return s
      }))

      if (progress.status === 'failed') {
        setInstallError(progress.message || `${progress.step} 失败`)
      }

      // 检测数据库变更
      if (progress.step === 'dbcheck' && progress.status === 'warning') {
        // 从message中解析数据库变更信息（如果后端传递了）
        setDbChanges([]) // 实际数据会从完成结果中获取
      }

      // 只有在没有错误且 registry 成功时才完成
      if (progress.status === 'success' && progress.step === 'registry' && !installError) {
        setInstallComplete(true)
        // 不自动跳转，让用户点击完成
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
      } else if (result.dbChanges && result.dbChanges.length > 0) {
        setDbChanges(result.dbChanges)
      }
    }).catch(err => {
      if (!isMounted) return
      console.error('Installation error:', err)
      setInstallError(err.message || '安装过程出错')
    })

    return () => {
      isMounted = false
    }
  }, [config, installError])

  const getStepIcon = (status: string) => {
    switch (status) {
      case 'success': return <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 18 }} />
      case 'running': return <LoadingOutlined style={{ color: '#10b981', fontSize: 18 }} spin />
      case 'failed': return <CloseCircleOutlined style={{ color: '#ff4d4f', fontSize: 18 }} />
      case 'warning': return <WarningOutlined style={{ color: '#faad14', fontSize: 18 }} />
      default: return <span style={{ color: '#d9d9d9', fontSize: 18 }}>○</span>
    }
  }

  const getStepTag = (status: string) => {
    switch (status) {
      case 'success': return <Tag color="success">完成</Tag>
      case 'running': return <Tag color="processing">进行中</Tag>
      case 'failed': return <Tag color="error">失败</Tag>
      case 'warning': return <Tag color="warning">注意</Tag>
      default: return <Tag color="default">等待中</Tag>
    }
  }

  const toggleStepExpand = (stepKey: string) => {
    setExpandedSteps(prev =>
      prev.includes(stepKey)
        ? prev.filter(s => s !== stepKey)
        : [...prev, stepKey]
    )
  }

  const handleClose = () => {
    window.electronAPI.closeWindow()
  }

  const handleComplete = () => {
    onComplete()
  }

  // 计算总体进度
  const completedSteps = steps.filter(s => s.status === 'success' || s.status === 'warning').length
  const totalProgress = Math.round((completedSteps / steps.length) * 100)

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>
        {installError ? (isUpgrade ? '升级失败' : '安装失败') :
         installComplete ? (isUpgrade ? '升级完成' : '安装完成') :
         (isUpgrade ? '正在升级...' : '正在安装...')}
      </h2>
      <p style={{ color: '#666', marginBottom: 20 }}>
        {installError ? '安装过程中遇到错误，请检查后重试' :
         installComplete ? '所有步骤已完成，请点击完成按钮继续' :
         '请稍候，安装程序正在配置您的系统'}
      </p>

      {/* 总体进度条 */}
      {!installError && (
        <div style={{ marginBottom: 24 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <span>总体进度</span>
            <span>{completedSteps} / {steps.length} 步骤</span>
          </div>
          <Progress
            percent={totalProgress}
            status={installComplete ? 'success' : 'active'}
            strokeColor={{
              '0%': '#10b981',
              '100%': '#059669',
            }}
          />
        </div>
      )}

      {/* 数据库变更提示（自动执行后显示结果） */}
      {dbChanges && dbChanges.length > 0 && (
        <Alert
          type="info"
          showIcon
          icon={<DatabaseOutlined />}
          style={{ marginBottom: 20 }}
          message="数据库迁移已完成"
          description={
            <div>
              <p style={{ marginBottom: 8 }}>已自动执行以下版本迁移：</p>
              {dbChanges.map(change => (
                <div key={change.version} style={{ marginBottom: 8 }}>
                  <strong>{change.version}：</strong>
                  <ul style={{ margin: '4px 0', paddingLeft: 20 }}>
                    {change.files.map(file => (
                      <li key={file}>{file}</li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          }
        />
      )}

      {/* 安装成功提示 */}
      {installComplete && !installError && (
        <Alert
          type="success"
          showIcon
          style={{ marginBottom: 20 }}
          message={isUpgrade ? '升级成功' : '安装成功'}
          description={
            <div>
              <p style={{ marginBottom: 8 }}>安装目录：{config.installDir}</p>
              <p style={{ marginBottom: 0 }}>请点击"完成"关闭安装程序，然后通过桌面快捷方式启动 Colink。</p>
            </div>
          }
        />
      )}

      {/* 安装错误提示 */}
      {installError && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 20 }}
          message="安装失败"
          description={installError}
        />
      )}

      {/* 步骤列表 */}
      <div style={{ flex: 1, overflow: 'auto' }}>
        {steps.map((step, index) => {
          const stepDef = INSTALL_STEPS.find(s => s.key === step.step)
          const isExpanded = expandedSteps.includes(step.step)
          const hasDetails = step.details || step.message || step.status === 'failed'

          return (
            <div
              key={step.step}
              style={{
                border: '1px solid #f0f0f0',
                borderRadius: 8,
                marginBottom: 8,
                overflow: 'hidden',
                background: step.status === 'running' ? '#f6ffed' : '#fff',
              }}
            >
              {/* 步骤头部 */}
              <div
                onClick={() => hasDetails && toggleStepExpand(step.step)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 12,
                  padding: '12px 16px',
                  cursor: hasDetails ? 'pointer' : 'default',
                  transition: 'background 0.2s',
                }}
              >
                <div style={{ width: 24 }}>{getStepIcon(step.status)}</div>
                <div style={{ flex: 1 }}>
                  <div style={{ fontWeight: 500 }}>{step.label}</div>
                  {step.status === 'running' && step.message && (
                    <div style={{ fontSize: 12, color: '#666', marginTop: 2 }}>{step.message}</div>
                  )}
                </div>
                {step.status === 'running' && (
                  <div style={{ width: 120 }}>
                    <Progress percent={step.progress} size="small" />
                  </div>
                )}
                {getStepTag(step.status)}
                {hasDetails && (
                  <RightOutlined
                    style={{
                      fontSize: 12,
                      color: '#999',
                      transform: isExpanded ? 'rotate(90deg)' : 'rotate(0deg)',
                      transition: 'transform 0.2s'
                    }}
                  />
                )}
              </div>

              {/* 展开的详情 */}
              {isExpanded && hasDetails && (
                <div style={{
                  padding: '12px 16px',
                  background: '#fafafa',
                  borderTop: '1px solid #f0f0f0',
                }}>
                  {stepDef?.description && (
                    <div style={{ marginBottom: 8, color: '#666', fontSize: 13 }}>
                      {stepDef.description}
                    </div>
                  )}
                  {step.details && (
                    <pre style={{
                      margin: 0,
                      padding: 8,
                      background: '#fff',
                      borderRadius: 4,
                      fontSize: 12,
                      fontFamily: 'Consolas, Monaco, monospace',
                      whiteSpace: 'pre-wrap',
                      wordBreak: 'break-word',
                      border: '1px solid #e8e8e8',
                    }}>
                      {step.details}
                    </pre>
                  )}
                  {step.status === 'failed' && step.message && (
                    <div style={{ color: '#ff4d4f', fontSize: 13, marginTop: 8 }}>
                      错误信息：{step.message}
                    </div>
                  )}
                  {step.endTime && step.startTime && (
                    <div style={{ color: '#999', fontSize: 12, marginTop: 8 }}>
                      耗时：{((step.endTime - step.startTime) / 1000).toFixed(1)}s
                    </div>
                  )}
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* 底部按钮 */}
      <div style={{ marginTop: 24, textAlign: 'right' }}>
        {installError ? (
          <Button type="primary" onClick={handleClose}>
            关闭
          </Button>
        ) : installComplete ? (
          <Button type="primary" size="large" onClick={handleComplete}>
            完成
          </Button>
        ) : null}
      </div>
    </div>
  )
}