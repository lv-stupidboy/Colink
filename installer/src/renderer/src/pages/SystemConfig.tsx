import { useState } from 'react'
import { Button, Input, Row, Col, message } from 'antd'
import { CheckCircleOutlined } from '@ant-design/icons'
import ConfigSection from '../components/ConfigSection'
import { InstallConfig } from '../types'
import { testConnection } from '../services/database-connector'

interface SystemConfigProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
}

export default function SystemConfig({ config, onConfigUpdate }: SystemConfigProps) {
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)

  const handleDbChange = (field: string, value: string | number) => {
    onConfigUpdate({
      database: { ...config.database, [field]: value }
    })
  }

  const handleTestConnection = async () => {
    setTesting(true)
    setTestResult(null)

    const result = await testConnection(config.database)
    setTesting(false)

    if (result.success) {
      setTestResult({ success: true, message: '连接成功' })
      message.success('数据库连接成功')
    } else {
      setTestResult({ success: false, message: result.error || '连接失败' })
      message.error(result.error || '数据库连接失败')
    }
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>系统配置</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>请配置 ISDP 运行所需的参数</p>

      <ConfigSection title="数据库配置">
        <Row gutter={20}>
          <Col span={16}>
            <div style={{ marginBottom: 16 }}>
              <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
                数据库地址
              </label>
              <Input
                value={config.database.host}
                onChange={(e) => handleDbChange('host', e.target.value)}
                placeholder="rm-xxx.mysql.rds.aliyuncs.com"
              />
            </div>
          </Col>
          <Col span={8}>
            <div style={{ marginBottom: 16 }}>
              <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
                端口
              </label>
              <Input
                type="number"
                value={config.database.port}
                onChange={(e) => handleDbChange('port', parseInt(e.target.value) || 3306)}
              />
            </div>
          </Col>
        </Row>

        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
            数据库名
          </label>
          <Input
            value={config.database.database}
            onChange={(e) => handleDbChange('database', e.target.value)}
          />
        </div>

        <Row gutter={20}>
          <Col span={12}>
            <div style={{ marginBottom: 16 }}>
              <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
                用户名
              </label>
              <Input
                value={config.database.username}
                onChange={(e) => handleDbChange('username', e.target.value)}
              />
            </div>
          </Col>
          <Col span={12}>
            <div style={{ marginBottom: 16 }}>
              <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
                密码
              </label>
              <Input.Password
                value={config.database.password}
                onChange={(e) => handleDbChange('password', e.target.value)}
              />
            </div>
          </Col>
        </Row>

        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Button onClick={handleTestConnection} loading={testing}>
            测试连接
          </Button>
          {testResult && (
            <span style={{ color: testResult.success ? '#52c41a' : '#ff4d4f' }}>
              {testResult.success && <CheckCircleOutlined style={{ marginRight: 4 }} />}
              {testResult.message}
            </span>
          )}
        </div>
      </ConfigSection>

      <ConfigSection title="高级设置（可选）" defaultCollapsed>
        <div style={{ color: '#999', textAlign: 'center', padding: 20 }}>
          预留扩展空间
        </div>
      </ConfigSection>
    </div>
  )
}