import { Card, Button, Space, Divider, Typography } from 'antd'
import {
  DeleteOutlined,
  RedoOutlined
} from '@ant-design/icons'

const { Title, Text } = Typography

interface DashboardProps {
  installDir: string
  onReinstall: () => void
  onUninstall: () => void
}

export default function Dashboard({
  installDir,
  onReinstall,
  onUninstall
}: DashboardProps) {
  return (
    <div style={{ padding: 24 }}>
      <Title level={3} style={{ marginBottom: 8 }}>ISDP Setup</Title>
      <Text type="secondary">安装管理</Text>

      <Divider />

      {/* 安装信息 */}
      <Card title="安装信息" size="small" style={{ marginBottom: 16 }}>
        <div>
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
    </div>
  )
}