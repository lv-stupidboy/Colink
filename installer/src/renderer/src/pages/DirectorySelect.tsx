import { Button, Input } from 'antd'
import { FolderOpenOutlined } from '@ant-design/icons'
import { InstallConfig } from '../types'

interface DirectorySelectProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
}

export default function DirectorySelect({ config, onConfigUpdate }: DirectorySelectProps) {
  const handleBrowse = async () => {
    // TODO: 调用主进程打开目录选择对话框
    // const result = await window.electronAPI.selectDirectory()
    // if (result) onConfigUpdate({ installDir: result })
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>选择安装位置</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>请选择 ISDP 的安装目录</p>

      <div style={{ marginBottom: 20 }}>
        <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 8 }}>
          安装目录
        </label>
        <div style={{ display: 'flex', gap: 12 }}>
          <Input
            value={config.installDir}
            onChange={(e) => onConfigUpdate({ installDir: e.target.value })}
            style={{ flex: 1 }}
          />
          <Button icon={<FolderOpenOutlined />} onClick={handleBrowse}>
            浏览...
          </Button>
        </div>
      </div>

      <div style={{ display: 'flex', gap: 40, color: '#666', fontSize: 14 }}>
        <span>所需空间：约 500 MB</span>
        <span>可用空间：120 GB</span>
      </div>
    </div>
  )
}