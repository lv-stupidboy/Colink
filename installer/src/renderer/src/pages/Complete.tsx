import { Button } from 'antd'
import { CheckCircleOutlined, DesktopOutlined } from '@ant-design/icons'
import { InstallConfig } from '../types'

interface CompleteProps {
  config: InstallConfig
  isUpgrade?: boolean
  onComplete: () => void
}

export default function Complete({ config, isUpgrade }: CompleteProps) {
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

      <div style={{
        background: '#e6f7ff',
        border: '1px solid #91d5ff',
        borderRadius: 8,
        padding: 16,
        marginBottom: 24,
        maxWidth: 400
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <DesktopOutlined style={{ color: '#1890ff', fontSize: 18 }} />
          <span style={{ fontWeight: 500, color: '#1890ff' }}>启动方式</span>
        </div>
        <p style={{ margin: 0, color: '#666', fontSize: 14 }}>
          请双击桌面上的快捷方式启动程序
        </p>
      </div>

      <Button type="primary" size="large" onClick={handleClose}>
        关闭
      </Button>
    </div>
  )
}