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

  const missingCount = dependencies.filter(d => !d.installed).length

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

          {missingCount > 0 && (
            <Alert
              type="info"
              showIcon
              style={{ marginTop: 16 }}
              message="智能体安装提示"
              description={
                <div>
                  <p style={{ marginBottom: 8 }}>
                    Claude CLI 和 OpenCode 是 Colink 平台的核心智能体，需要安装后才能使用相应功能。
                  </p>
                  <p style={{ marginBottom: 0 }}>
                    您可以在安装 Colink 后，通过启动器的「智能体管理」自助安装；也可以提前安装，无先后顺序要求。
                  </p>
                </div>
              }
            />
          )}

          {missingCount === 0 && (
            <Alert
              type="success"
              showIcon
              style={{ marginTop: 16 }}
              message="所有智能体已就绪"
              description="Claude CLI 和 OpenCode 已安装，Colink 平台可以正常使用全部功能。"
            />
          )}
        </>
      )}
    </div>
  )
}