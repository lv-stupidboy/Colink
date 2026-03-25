import { useEffect, useState } from 'react'
import { Spin, Tag } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import { Dependency, InstallConfig } from '../types'
import { checkAllDependencies } from '../services/dependency-checker'

interface DependencyCheckProps {
  config: InstallConfig
  onDependenciesUpdate: (deps: Dependency[]) => void
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
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>依赖检测</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>
        {loading ? '正在检测系统运行环境...' : '检测结果如下'}
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
            <p style={{ color: '#fa8c16', fontWeight: 500 }}>
              检测到 <strong>{missingCount}</strong> 个依赖项缺失
            </p>
          )}
        </>
      )}
    </div>
  )
}