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

// ISDP Logo SVG
const ISDPLogo = () => (
  <svg width="48" height="48" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
    <defs>
      <linearGradient id="bgGrad" x1="0%" y1="0%" x2="100%" y2="100%">
        <stop offset="0%" style={{ stopColor: '#10b981' }} />
        <stop offset="100%" style={{ stopColor: '#059669' }} />
      </linearGradient>
      <linearGradient id="flameGrad" x1="0%" y1="0%" x2="0%" y2="100%">
        <stop offset="0%" style={{ stopColor: '#fbbf24' }} />
        <stop offset="50%" style={{ stopColor: '#f59e0b' }} />
        <stop offset="100%" style={{ stopColor: '#ef4444' }} />
      </linearGradient>
    </defs>
    <rect x="2" y="2" width="28" height="28" rx="6" fill="url(#bgGrad)" />
    <path d="M16 5C16 5 11 9 11 15C11 17 11.5 19 12 20L13 22H19L20 20C20.5 19 21 17 21 15C21 9 16 5 16 5Z" fill="white" />
    <circle cx="16" cy="12" r="2" fill="#10b981" />
    <path d="M11 18L9 22L11 23L12.5 20.5Z" fill="#e5e7eb" />
    <path d="M21 18L23 22L21 23L19.5 20.5Z" fill="#e5e7eb" />
    <path d="M13.5 22L14.5 27L16 25L17.5 27L18.5 22Z" fill="url(#flameGrad)" />
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
          <Title level={3} style={{ margin: 0 }}>ISDP</Title>
          <Text type="secondary">智能软件开发平台</Text>
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

      <div style={{ marginTop: 24, textAlign: 'center' }}>
        <Text type="secondary">
          关闭窗口后服务将继续在后台运行
        </Text>
      </div>
    </div>
  )
}