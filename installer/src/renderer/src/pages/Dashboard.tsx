import { Card, Button, Space, Tag, Divider, Typography } from 'antd'
import {
  PlayCircleOutlined,
  StopOutlined,
  SettingOutlined,
  FileTextOutlined,
  FolderOutlined,
  DeleteOutlined,
  RedoOutlined,
  GlobalOutlined
} from '@ant-design/icons'

const { Title, Text } = Typography

interface DashboardProps {
  installDir: string
  serviceStatus: 'running' | 'stopped'
  onStartService: () => void
  onStopService: () => void
  onReinstall: () => void
  onUninstall: () => void
}

export default function Dashboard({
  installDir,
  serviceStatus,
  onStartService,
  onStopService,
  onReinstall,
  onUninstall
}: DashboardProps) {
  const isRunning = serviceStatus === 'running'

  const handleOpenConsole = () => {
    window.open('http://localhost:8080', '_blank')
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
      <Title level={3} style={{ marginBottom: 8 }}>ISDP 控制面板</Title>
      <Text type="secondary">智能软件开发平台</Text>

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
      <Card title="安装信息" size="small" style={{ marginBottom: 16 }}>
        <div style={{ marginBottom: 8 }}>
          <Text type="secondary">安装目录：</Text>
          <Text code>{installDir}</Text>
        </div>
      </Card>

      <Divider />

      {/* 维护操作 */}
      <Card title="维护" size="small">
        <Space>
          <Button
            icon={<RedoOutlined />}
            onClick={onReinstall}
          >
            重新安装
          </Button>
          <Button
            icon={<DeleteOutlined />}
            danger
            onClick={onUninstall}
          >
            卸载
          </Button>
        </Space>
      </Card>

      <div style={{ marginTop: 24, textAlign: 'center' }}>
        <Text type="secondary">
          关闭窗口后服务将继续在后台运行
        </Text>
      </div>
    </div>
  )
}