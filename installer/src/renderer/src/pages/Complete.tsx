import { useState } from 'react'
import { Button, message } from 'antd'
import { CheckCircleOutlined } from '@ant-design/icons'
import { InstallConfig } from '../types'

interface CompleteProps {
  config: InstallConfig
  isUpgrade?: boolean
  onComplete: () => void
}

export default function Complete({ config, isUpgrade }: CompleteProps) {
  const [launching, setLaunching] = useState(false)

  const handleLaunch = async () => {
    setLaunching(true)
    try {
      // 启动 ISDP.exe（桌面快捷方式指向的程序）
      const result = await window.electronAPI.launchISDP()
      if (result.success) {
        // 延迟关闭窗口
        setTimeout(() => {
          window.electronAPI.closeWindow()
        }, 1000)
      } else {
        message.error(result.error || '启动失败')
        setLaunching(false)
      }
    } catch (err) {
      message.error('启动失败')
      setLaunching(false)
    }
  }

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
          <span>已创建桌面快捷方式</span>
        </div>
        <div style={{ marginBottom: 10, display: 'flex', alignItems: 'center', gap: 8 }}>
          <CheckCircleOutlined style={{ color: '#52c41a' }} />
          <span>已创建开始菜单快捷方式</span>
        </div>
      </div>

      <div style={{ display: 'flex', gap: 16 }}>
        <Button size="large" onClick={handleClose}>
          关闭
        </Button>
        <Button type="primary" size="large" onClick={handleLaunch} loading={launching}>
          启动 ISDP
        </Button>
      </div>
    </div>
  )
}