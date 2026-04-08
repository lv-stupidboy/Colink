import { Card, Button, Space, Typography, Divider } from 'antd'
import { CloudUploadOutlined, DeleteOutlined, CloseOutlined } from '@ant-design/icons'

const { Title, Text } = Typography

interface SelectActionProps {
  installDir: string
  onUpgrade: () => void
  onUninstall: () => void
  onCancel: () => void
}

export default function SelectAction({ installDir, onUpgrade, onUninstall, onCancel }: SelectActionProps) {
  return (
    <div style={{
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      padding: 24
    }}>
      <Title level={3} style={{ marginBottom: 8 }}>检测到已安装的 Colink</Title>
      <Text type="secondary" style={{ marginBottom: 30 }}>请选择要执行的操作</Text>

      <Card size="small" style={{ marginBottom: 30, minWidth: 300 }}>
        <Text type="secondary">安装位置：</Text>
        <Text code>{installDir}</Text>
      </Card>

      <Space direction="vertical" style={{ width: '100%', maxWidth: 400 }} size="middle">
        <Button
          type="primary"
          icon={<CloudUploadOutlined />}
          size="large"
          block
          onClick={onUpgrade}
        >
          升级 Colink
        </Button>

        <Button
          icon={<DeleteOutlined />}
          danger
          size="large"
          block
          onClick={onUninstall}
        >
          卸载
        </Button>

        <Button
          icon={<CloseOutlined />}
          size="large"
          block
          onClick={onCancel}
        >
          取消
        </Button>
      </Space>

      <Divider style={{ marginTop: 40, marginBottom: 16 }} />

      <Text type="secondary" style={{ fontSize: 12 }}>
        升级将保留现有配置和数据
      </Text>
    </div>
  )
}