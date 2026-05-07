import { useEffect } from 'react'
import { Button, Input } from 'antd'
import { FolderOpenOutlined } from '@ant-design/icons'
import { InstallConfig, InstalledVersion } from '../types'

interface DirectorySelectProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
  installedVersion?: InstalledVersion
  isUpgrade?: boolean
  onValidationChange?: (valid: boolean) => void
}

export default function DirectorySelect({ config, onConfigUpdate, onValidationChange }: DirectorySelectProps) {
  // 始终允许继续，无需验证磁盘空间
  useEffect(() => {
    onValidationChange?.(true)
  }, [onValidationChange])

  const handleBrowse = async () => {
    const result = await window.electronAPI.selectDirectory()
    if (result) {
      onConfigUpdate({ installDir: result })
    }
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>选择安装位置</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>请选择 Colink 的安装目录</p>

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
        <div style={{ color: '#999', fontSize: 12, marginTop: 8 }}>
          目录不存在时将自动创建
        </div>
      </div>
    </div>
  )
}