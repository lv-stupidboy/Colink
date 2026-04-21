import { Radio, Space } from 'antd'
import { InstallConfig, Dependency, InstallMode, InstalledVersion } from '../types'

interface ModeSelectProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
  installedVersion?: InstalledVersion
  isUpgrade?: boolean
}

export default function ModeSelect({ config, onConfigUpdate }: ModeSelectProps) {
  const missingDeps = config.dependencies.filter(d => !d.installed)

  const handleModeChange = (mode: InstallMode) => {
    onConfigUpdate({ installMode: mode })
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>智能体安装方式</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>
        检测到以下智能体未安装：{missingDeps.map(d => d.name).join('、')}
      </p>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div
          onClick={() => handleModeChange('launcher-install')}
          style={{
            padding: 20,
            border: `2px solid ${config.installMode === 'launcher-install' ? '#10b981' : '#e8e8e8'}`,
            borderRadius: 8,
            cursor: 'pointer',
            background: config.installMode === 'launcher-install' ? '#d1fae5' : '#fff',
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: 8 }}>
            <Radio checked={config.installMode === 'launcher-install'} />
            <span style={{ marginLeft: 8 }}>稍后在启动器安装（推荐）</span>
          </div>
          <p style={{ color: '#666', fontSize: 13, marginLeft: 30 }}>
            安装完成后，在启动器的「依赖管理」中自助安装 Claude CLI 和 OpenCode
          </p>
        </div>

        <div
          onClick={() => handleModeChange('skip')}
          style={{
            padding: 20,
            border: `2px solid ${config.installMode === 'skip' ? '#10b981' : '#e8e8e8'}`,
            borderRadius: 8,
            cursor: 'pointer',
            background: config.installMode === 'skip' ? '#d1fae5' : '#fff',
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: 8 }}>
            <Radio checked={config.installMode === 'skip'} />
            <span style={{ marginLeft: 8 }}>跳过安装</span>
          </div>
          <p style={{ color: '#666', fontSize: 13, marginLeft: 30 }}>
            暂不安装这些智能体，后续在平台中配置其他 Agent 类型
          </p>
        </div>
      </div>
    </div>
  )
}