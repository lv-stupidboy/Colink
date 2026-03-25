import { Checkbox, Button, message } from 'antd'
import { InstallConfig } from '../types'

interface CompleteProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
}

export default function Complete({ config, onConfigUpdate }: CompleteProps) {
  const handleFinish = async () => {
    // 创建快捷方式
    if (config.createShortcut) {
      await window.electronAPI.createShortcut(config.installDir)
    }

    message.success('安装完成！')

    // 如果选择立即启动，启动服务
    if (config.launchNow) {
      await window.electronAPI.launchService(config.installDir)
    }

    // 关闭安装器
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

      <h2 style={{ fontSize: 22, marginBottom: 16, color: '#333' }}>安装完成！</h2>

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

      <div style={{ marginBottom: 30 }}>
        <div style={{ marginBottom: 10 }}>
          <Checkbox
            checked={config.createShortcut}
            onChange={(e) => onConfigUpdate({ createShortcut: e.target.checked })}
          >
            创建桌面快捷方式
          </Checkbox>
        </div>
        <div>
          <Checkbox
            checked={config.launchNow}
            onChange={(e) => onConfigUpdate({ launchNow: e.target.checked })}
          >
            立即启动 ISDP
          </Checkbox>
        </div>
      </div>

      <Button type="primary" size="large" onClick={handleFinish}>
        完成
      </Button>
    </div>
  )
}