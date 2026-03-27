import { useEffect, useState } from 'react'
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
  const [freeSpace, setFreeSpace] = useState<number>(0)

  // 获取磁盘空间（不阻止用户继续）
  const checkDiskSpace = async (path: string) => {
    if (!path || path.trim() === '') {
      setFreeSpace(0)
      onValidationChange?.(true)
      return
    }

    // 提取驱动器字母
    const normalizedPath = path.replace(/\//g, '\\')
    const windowsPathRegex = /^[A-Za-z]:\\/
    if (!windowsPathRegex.test(normalizedPath)) {
      setFreeSpace(0)
      onValidationChange?.(true)
      return
    }

    const drive = normalizedPath.substring(0, 2).toUpperCase()

    try {
      const result = await window.electronAPI.getDiskSpace(drive)
      setFreeSpace(result.free)
    } catch {
      setFreeSpace(0)
    }

    // 始终允许继续
    onValidationChange?.(true)
  }

  useEffect(() => {
    checkDiskSpace(config.installDir)
  }, [config.installDir])

  const handleBrowse = async () => {
    const result = await window.electronAPI.selectDirectory()
    if (result) {
      onConfigUpdate({ installDir: result })
    }
  }

  const formatSize = (bytes: number) => {
    if (bytes === 0) return '未知'
    const gb = bytes / (1024 * 1024 * 1024)
    return `${gb.toFixed(1)} GB`
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>选择安装位置</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>请选择 Lights-Out 的安装目录</p>

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

      <div style={{ display: 'flex', gap: 40, color: '#666', fontSize: 14 }}>
        <span>所需空间：约 500 MB</span>
        <span>可用空间：{formatSize(freeSpace)}</span>
      </div>
    </div>
  )
}