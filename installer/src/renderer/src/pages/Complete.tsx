import { useEffect, useState } from 'react'
import { Checkbox, Button, Spin } from 'antd'
import { CheckCircleOutlined } from '@ant-design/icons'
import { InstallConfig, InstalledVersion } from '../types'

interface CompleteProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
  installedVersion?: InstalledVersion
  isUpgrade?: boolean
}

export default function Complete({ config, isUpgrade }: CompleteProps) {
  const [launching, setLaunching] = useState(false)
  const [serviceLaunched, setServiceLaunched] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    // 如果选择立即启动，启动服务
    if (config.launchNow) {
      const launchService = async () => {
        setLaunching(true)
        try {
          const result = await window.electronAPI.launchService(config.installDir)
          if (result.success) {
            setServiceLaunched(true)
          } else {
            setError(result.error || '启动失败')
          }
        } catch (err) {
          setError(err instanceof Error ? err.message : '启动失败')
        }
        setLaunching(false)
      }

      // 延迟启动，让用户看到完成页面
      setTimeout(launchService, 500)
    }
  }, [config])

  const handleClose = () => {
    window.electronAPI.closeWindow()
  }

  return (
    <div style={{
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      textAlign: 'center'
    }}>
      <div style={{
        width: 80,
        height: 80,
        background: '#52c41a',
        borderRadius: '50%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: 40,
        color: '#fff',
        marginBottom: 24
      }}>
        ✓
      </div>

      <h2 style={{ fontSize: 22, marginBottom: 16, color: '#333' }}>
        {isUpgrade ? '升级完成！' : '安装完成！'}
      </h2>

      <div style={{
        background: '#f5f5f5',
        padding: '12px 20px',
        borderRadius: 6,
        marginBottom: 24,
        fontFamily: 'monospace',
        color: '#666'
      }}>
        安装位置：{config.installDir}
      </div>

      <div style={{ marginBottom: 30, textAlign: 'left', minWidth: 280 }}>
        <div style={{ marginBottom: 10, display: 'flex', alignItems: 'center', gap: 8 }}>
          <CheckCircleOutlined style={{ color: '#52c41a' }} />
          <span>桌面快捷方式已创建</span>
        </div>
        <div style={{ marginBottom: 10, display: 'flex', alignItems: 'center', gap: 8 }}>
          <CheckCircleOutlined style={{ color: '#52c41a' }} />
          <span>开始菜单快捷方式已创建</span>
        </div>
        {config.launchNow && (
          <div style={{ marginBottom: 10, display: 'flex', alignItems: 'center', gap: 8 }}>
            {launching ? (
              <Spin size="small" />
            ) : serviceLaunched ? (
              <CheckCircleOutlined style={{ color: '#52c41a' }} />
            ) : error ? (
              <span style={{ color: '#ff4d4f' }}>✗</span>
            ) : (
              <CheckCircleOutlined style={{ color: '#52c41a' }} />
            )}
            <span>启动 ISDP 服务</span>
            {error && <span style={{ color: '#ff4d4f', fontSize: 12 }}>({error})</span>}
          </div>
        )}
      </div>

      <Button type="primary" size="large" onClick={handleClose}>
        关闭
      </Button>
    </div>
  )
}