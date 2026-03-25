import { Radio, Space } from 'antd'
import { InstallConfig, Dependency, InstallMode } from '../types'

interface ModeSelectProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
}

export default function ModeSelect({ config, onConfigUpdate }: ModeSelectProps) {
  const missingDeps = config.dependencies.filter(d => !d.installed)

  const handleModeChange = (mode: InstallMode) => {
    onConfigUpdate({ installMode: mode })
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>选择安装方式</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>
        检测到以下依赖未安装：{missingDeps.map(d => d.name).join('、')}
      </p>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div
          onClick={() => handleModeChange('auto')}
          style={{
            padding: 20,
            border: `2px solid ${config.installMode === 'auto' ? '#10b981' : '#e8e8e8'}`,
            borderRadius: 8,
            cursor: 'pointer',
            background: config.installMode === 'auto' ? '#d1fae5' : '#fff',
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: 8 }}>
            <Radio checked={config.installMode === 'auto'} />
            <span style={{ marginLeft: 8 }}>自动安装（推荐）</span>
          </div>
          <p style={{ color: '#666', fontSize: 13, marginLeft: 30 }}>
            安装器将自动下载并安装缺失的依赖项
          </p>
        </div>

        <div
          onClick={() => handleModeChange('manual')}
          style={{
            padding: 20,
            border: `2px solid ${config.installMode === 'manual' ? '#10b981' : '#e8e8e8'}`,
            borderRadius: 8,
            cursor: 'pointer',
            background: config.installMode === 'manual' ? '#d1fae5' : '#fff',
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: 8 }}>
            <Radio checked={config.installMode === 'manual'} />
            <span style={{ marginLeft: 8 }}>手动安装</span>
          </div>
          <p style={{ color: '#666', fontSize: 13, marginLeft: 30 }}>
            我将自行安装依赖，完成后继续
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
            暂不安装这些依赖，后续在平台中配置其他 Agent 类型
          </p>
        </div>
      </div>
    </div>
  )
}