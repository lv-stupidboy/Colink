import { useEffect, useState } from 'react'
import { Button, Input, Radio, Spin } from 'antd'
import { FolderOpenOutlined, InfoCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
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
  const [isValid, setIsValid] = useState<boolean>(true)
  const [parentPath, setParentPath] = useState<string>('')

  // 验证目录路径
  const validatePath = async (path: string) => {
    if (!path || path.trim() === '') {
      setIsValid(false)
      setFreeSpace(0)
      onValidationChange?.(false)
      return
    }

    // 获取父目录路径
    const parts = path.replace(/\\/g, '/').split('/')
    parts.pop() // 移除最后一级目录名
    const parent = parts.join('/') || parts[0] + '/'

    setParentPath(parent)

    // 检查父目录是否存在并获取磁盘空间
    try {
      const result = await window.electronAPI.getDiskSpace(parent)
      if (result.free === 0 && result.total === 0) {
        // 父目录不存在
        setIsValid(false)
        setFreeSpace(0)
        onValidationChange?.(false)
      } else {
        setIsValid(true)
        setFreeSpace(result.free)
        onValidationChange?.(true)
      }
    } catch {
      setIsValid(false)
      setFreeSpace(0)
      onValidationChange?.(false)
    }
  }

  useEffect(() => {
    // 获取磁盘空间和验证目录
    validatePath(config.installDir)
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
            status={!isValid ? 'error' : undefined}
          />
          <Button icon={<FolderOpenOutlined />} onClick={handleBrowse}>
            浏览...
          </Button>
        </div>
        {!isValid && (
          <div style={{ color: '#ff4d4f', fontSize: 12, marginTop: 8, display: 'flex', alignItems: 'center', gap: 4 }}>
            <CloseCircleOutlined />
            目录不存在，请选择有效的安装位置
          </div>
        )}
      </div>

      <div style={{ display: 'flex', gap: 40, color: '#666', fontSize: 14 }}>
        <span>所需空间：约 500 MB</span>
        <span>可用空间：{isValid ? formatSize(freeSpace) : '未知'}</span>
      </div>
    </div>
  )
}