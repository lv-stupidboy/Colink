import { useState, useEffect } from 'react'
import { Modal, Button, Input, Spin, message, Space, Alert } from 'antd'
import { SettingOutlined, EditOutlined, SaveOutlined, ReloadOutlined } from '@ant-design/icons'

const { TextArea } = Input

interface ConfigEditorProps {
  visible: boolean
  onClose: () => void
  onSaved?: () => void
}

export default function ConfigEditor({ visible, onClose, onSaved }: ConfigEditorProps) {
  const [loading, setLoading] = useState(false)
  const [yamlContent, setYamlContent] = useState<string>('')
  const [saving, setSaving] = useState(false)

  const loadConfig = async () => {
    setLoading(true)
    try {
      const result = await window.electronAPI.getConfigPreview()
      if (result.success && result.yaml) {
        setYamlContent(result.yaml)
      } else {
        message.error(result.error || '读取配置失败')
      }
    } catch (e) {
      message.error('读取配置失败')
    }
    setLoading(false)
  }

  useEffect(() => {
    if (visible) {
      loadConfig()
    }
  }, [visible])

  const handleSave = async () => {
    if (!yamlContent.trim()) {
      message.warning('配置内容不能为空')
      return
    }

    setSaving(true)
    try {
      const result = await window.electronAPI.saveConfig(yamlContent)
      if (result.success) {
        message.success('配置已保存')
        onSaved?.()
        onClose()
      } else {
        message.error(result.error || '保存失败')
      }
    } catch (e) {
      message.error('保存失败')
    }
    setSaving(false)
  }

  return (
    <Modal
      title={
        <Space>
          <SettingOutlined />
          系统配置
        </Space>
      }
      open={visible}
      onCancel={onClose}
      footer={null}
      width={700}
    >
      <div style={{ marginBottom: 16 }}>
        <Alert
          type="info"
          showIcon
          message="配置修改后需重启服务生效"
          description="保存配置后，请在启动器中重启服务以应用新的配置。"
        />
      </div>

      {loading ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin tip="加载配置..." />
        </div>
      ) : (
        <>
          <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ color: '#666', fontSize: 13 }}>
              配置文件：data/configs/config.yaml
            </span>
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={loadConfig}
              loading={loading}
            >
              刷新
            </Button>
          </div>

          <div style={{
            background: '#fafafa',
            border: '1px solid #e8e8e8',
            borderRadius: 6,
            overflow: 'hidden'
          }}>
            <TextArea
              value={yamlContent}
              onChange={(e) => setYamlContent(e.target.value)}
              autoSize={{ minRows: 15, maxRows: 25 }}
              style={{
                fontFamily: 'Consolas, Monaco, monospace',
                fontSize: 12,
                lineHeight: 1.5,
                border: 'none',
                background: 'transparent'
              }}
            />
          </div>

          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
            <Button onClick={onClose}>
              取消
            </Button>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              loading={saving}
              onClick={handleSave}
            >
              保存
            </Button>
          </div>
        </>
      )}
    </Modal>
  )
}