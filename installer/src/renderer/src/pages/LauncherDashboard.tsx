import { Card, Button, Space, Tag, Divider, Typography } from 'antd'
import {
  PlayCircleOutlined,
  StopOutlined,
  SettingOutlined,
  FileTextOutlined,
  FolderOutlined,
  GlobalOutlined
} from '@ant-design/icons'

const { Title, Text } = Typography

interface LauncherDashboardProps {
  installDir: string
  serviceStatus: 'running' | 'stopped'
  onStartService: () => void
  onStopService: () => void
}

// ISDP Logo SVG - 熄灯工厂主题
const ISDPLogo = () => (
  <svg width="48" height="48" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
    <defs>
      <linearGradient id="bgGrad" x1="0%" y1="0%" x2="100%" y2="100%">
        <stop offset="0%" style={{ stopColor: '#374151' }} />
        <stop offset="100%" style={{ stopColor: '#1f2937' }} />
      </linearGradient>
      <linearGradient id="bulbGrad" x1="0%" y1="0%" x2="0%" y2="100%">
        <stop offset="0%" style={{ stopColor: '#fbbf24' }} />
        <stop offset="100%" style={{ stopColor: '#f59e0b' }} />
      </linearGradient>
    </defs>
    <rect x="2" y="2" width="28" height="28" rx="6" fill="url(#bgGrad)" />
    <ellipse cx="16" cy="12" rx="6" ry="7" fill="url(#bulbGrad)" />
    <rect x="13" y="18" width="6" height="2" rx="0.5" fill="#9ca3af" />
    <rect x="13.5" y="20.5" width="5" height="1.5" rx="0.5" fill="#6b7280" />
    <rect x="14" y="22.5" width="4" height="1.5" rx="0.5" fill="#4b5563" />
    <line x1="13" y1="19" x2="19" y2="19" stroke="#6b7280" strokeWidth="0.5" />
    <line x1="13.5" y1="21.25" x2="18.5" y2="21.25" stroke="#4b5563" strokeWidth="0.5" />
    <path d="M14 14 Q16 11 18 14" stroke="#fcd34d" strokeWidth="1.5" fill="none" strokeLinecap="round" />
  </svg>
)

export function LauncherDashboard({
  installDir,
  serviceStatus,
  onStartService,
  onStopService
}: LauncherDashboardProps) {
  const isRunning = serviceStatus === 'running'

  const handleOpenConsole = async () => {
    await window.electronAPI.openConsole()
  }

  const handleOpenLogs = async () => {
    await window.electronAPI.openLogs()
  }

  const handleOpenDataDir = async () => {
    await window.electronAPI.openDataDir()
  }

  const handleOpenConfig = async () => {
    await window.electronAPI.openConfig()
  }

  return (
    <div style={{ padding: 24 }}>
      {/* Logo 和标题 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 16 }}>
        <ISDPLogo />
        <div>
          <Title level={3} style={{ margin: 0 }}>Lights-Out</Title>
          <Text type="secondary">熄灯工厂</Text>
        </div>
      </div>

      <Divider />

      {/* 服务状态 */}
      <Card size="small" style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div>
            <Text strong>服务状态：</Text>
            <Tag color={isRunning ? 'green' : 'default'}>
              {isRunning ? '运行中' : '已停止'}
            </Tag>
          </div>
          <Space>
            {isRunning ? (
              <Button
                icon={<StopOutlined />}
                onClick={onStopService}
                danger
              >
                停止服务
              </Button>
            ) : (
              <Button
                type="primary"
                icon={<PlayCircleOutlined />}
                onClick={onStartService}
              >
                启动服务
              </Button>
            )}
          </Space>
        </div>
      </Card>

      {/* 快捷操作 */}
      <Card title="快捷操作" size="small" style={{ marginBottom: 16 }}>
        <Space wrap>
          <Button
            icon={<GlobalOutlined />}
            onClick={handleOpenConsole}
            disabled={!isRunning}
          >
            打开控制台
          </Button>
          <Button
            icon={<SettingOutlined />}
            onClick={handleOpenConfig}
          >
            系统配置
          </Button>
          <Button
            icon={<FileTextOutlined />}
            onClick={handleOpenLogs}
          >
            查看日志
          </Button>
          <Button
            icon={<FolderOutlined />}
            onClick={handleOpenDataDir}
          >
            数据目录
          </Button>
        </Space>
      </Card>

      {/* 安装信息 */}
      <Card title="安装信息" size="small">
        <div>
          <Text type="secondary">安装目录：</Text>
          <Text code>{installDir}</Text>
        </div>
      </Card>
    </div>
  )
}