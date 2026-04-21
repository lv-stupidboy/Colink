import { useState, useEffect } from 'react'
import { Modal, Button, Table, Tag, Spin, message, Space, Progress } from 'antd'
import { ToolOutlined, CheckCircleOutlined, CloseCircleOutlined, ReloadOutlined } from '@ant-design/icons'

interface DependencyItem {
  key: string
  name: string
  installed: boolean
  version?: string
}

interface DependencyManagerProps {
  visible: boolean
  onClose: () => void
}

export default function DependencyManager({ visible, onClose }: DependencyManagerProps) {
  const [loading, setLoading] = useState(false)
  const [dependencies, setDependencies] = useState<DependencyItem[]>([])
  const [installing, setInstalling] = useState<string | null>(null)
  const [installProgress, setInstallProgress] = useState(0)

  const loadDependencies = async () => {
    setLoading(true)
    try {
      const results = await window.electronAPI.checkAllDependencies()
      setDependencies(results)
    } catch (e) {
      message.error('检测智能体失败')
    }
    setLoading(false)
  }

  useEffect(() => {
    if (visible) {
      loadDependencies()
    }
  }, [visible])

  const handleInstall = async (key: string) => {
    setInstalling(key)
    setInstallProgress(0)

    try {
      const result = await window.electronAPI.installDependency(key)
      setInstallProgress(100)

      if (result.success) {
        message.success(`${key === 'claude' ? 'Claude CLI' : 'OpenCode'} 安装成功`)
        // 刷新列表
        await loadDependencies()
      } else {
        message.error(result.error || '安装失败')
      }
    } catch (e) {
      message.error('安装失败')
    }

    setInstalling(null)
    setInstallProgress(0)
  }

  const columns = [
    {
      title: '智能体名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '状态',
      dataIndex: 'installed',
      key: 'installed',
      render: (installed: boolean, record: DependencyItem) => (
        <Tag color={installed ? 'success' : 'warning'}>
          {installed ? `已安装 ${record.version}` : '未安装'}
        </Tag>
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: DependencyItem) => (
        record.installed ? (
          <span style={{ color: '#52c41a' }}>
            <CheckCircleOutlined style={{ marginRight: 4 }} />
            已就绪
          </span>
        ) : (
          <Button
            type="primary"
            size="small"
            loading={installing === record.key}
            onClick={() => handleInstall(record.key)}
          >
            安装
          </Button>
        )
      ),
    },
  ]

  return (
    <Modal
      title={
        <Space>
          <ToolOutlined />
          智能体管理
        </Space>
      }
      open={visible}
      onCancel={onClose}
      footer={null}
      width={500}
    >
      <div style={{ marginBottom: 16 }}>
        <p style={{ color: '#666', fontSize: 13 }}>
          检测并安装智能体 CLI 工具。安装完成后，即可在平台中使用对应的 Agent 类型。
        </p>
      </div>

      {loading ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin tip="检测智能体状态..." />
        </div>
      ) : (
        <>
          <Table
            dataSource={dependencies}
            columns={columns}
            rowKey="key"
            pagination={false}
            size="small"
          />

          {installing && (
            <div style={{ marginTop: 16 }}>
              <Progress
                percent={installProgress}
                status="active"
                format={() => `正在安装 ${installing === 'claude' ? 'Claude CLI' : 'OpenCode'}...`}
              />
            </div>
          )}

          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'space-between' }}>
            <Button
              icon={<ReloadOutlined />}
              onClick={loadDependencies}
              loading={loading}
            >
              重新检测
            </Button>
            <Button onClick={onClose}>
              关闭
            </Button>
          </div>
        </>
      )}
    </Modal>
  )
}