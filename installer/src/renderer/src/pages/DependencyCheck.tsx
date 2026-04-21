import { useEffect, useState } from 'react'
import { Spin, Tag, Alert } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import { Dependency, InstallConfig, InstalledVersion } from '../types'
import { checkAllDependencies } from '../services/dependency-checker'

interface DependencyCheckProps {
  config: InstallConfig
  onDependenciesUpdate: (deps: Dependency[]) => void
  installedVersion?: InstalledVersion
  isUpgrade?: boolean
}

export default function DependencyCheck({ onDependenciesUpdate }: DependencyCheckProps) {
  const [loading, setLoading] = useState(true)
  const [dependencies, setDependencies] = useState<Dependency[]>([])

  useEffect(() => {
    checkAllDependencies().then(deps => {
      setDependencies(deps)
      onDependenciesUpdate(deps)
      setLoading(false)
    })
  }, [onDependenciesUpdate])

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>智能体检测</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>
        {loading ? '正在检测系统智能体环境...' : '检测结果如下'}
      </p>

      {loading ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin size="large" />
        </div>
      ) : (
        <>
          <div style={{
            background: '#fafafa',
            borderRadius: 8,
            padding: 20,
            marginBottom: 20
          }}>
            {dependencies.map(dep => (
              <div
                key={dep.key}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '12px 16px',
                  borderBottom: '1px solid #e8e8e8',
                }}
              >
                <span style={{ fontWeight: 500 }}>{dep.name}</span>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Tag color={dep.installed ? 'success' : 'warning'}>
                    {dep.installed ? `已安装 ${dep.version}` : '未安装'}
                  </Tag>
                </div>
              </div>
            ))}
          </div>

          <Alert
            type="info"
            showIcon
            style={{ marginTop: 16 }}
            message="智能体说明"
            description={
              <div>
                <p style={{ marginBottom: 8 }}>
                  Colink 平台当前支持 Claude CLI 和 OpenCode 等智能体，安装后即可使用对应的 Agent 类型。
                </p>
                <p style={{ marginBottom: 0 }}>
                  您可以在安装 Colink 后自行安装需要的智能体，也可以提前安装，无先后顺序要求。
                </p>
              </div>
            }
          />
        </>
      )}
    </div>
  )
}