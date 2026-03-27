import { useState, useEffect } from 'react'
import { Button, Input, Row, Col, message, Spin } from 'antd'
import { CheckCircleOutlined } from '@ant-design/icons'
import ConfigSection from '../components/ConfigSection'
import { InstallConfig, InstalledVersion } from '../types'
import { testConnection } from '../services/database-connector'

interface SystemConfigProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
  installedVersion?: InstalledVersion
  isUpgrade?: boolean
}

export default function SystemConfig({ config, onConfigUpdate, installedVersion, isUpgrade }: SystemConfigProps) {
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)
  const [loadingConfig, setLoadingConfig] = useState(false)

  // 升级模式或安装目录已有配置时读取已有配置
  useEffect(() => {
    const loadExistingConfig = async () => {
      // 优先从安装目录读取配置（可能是卸载保留的数据）
      const targetDir = config.installDir || installedVersion?.installDir

      if (!targetDir) return

      setLoadingConfig(true)
      try {
        const result = await window.electronAPI.readExistingConfig(targetDir)
        if (result.success && result.config) {
          console.log('[SystemConfig] Loaded existing config from:', targetDir)
          onConfigUpdate({
            database: result.config.database,
            serverPort: result.config.serverPort || 8080
          })
          if (!isUpgrade) {
            message.info('已加载该目录下的现有配置')
          }
        }
      } catch (e) {
        console.warn('[SystemConfig] Failed to load existing config:', e)
      }
      setLoadingConfig(false)
    }

    // 升级模式：从已安装目录读取
    if (isUpgrade && installedVersion?.installDir) {
      loadExistingConfig()
    }
    // 安装模式：检查目标目录是否有配置（卸载保留的数据）
    else if (!isUpgrade && config.installDir) {
      loadExistingConfig()
    }
  }, [isUpgrade, installedVersion, config.installDir])

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

  if (loadingConfig) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" tip="加载已有配置..." />
      </div>
    )
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>系统配置</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>
        {isUpgrade ? '已加载现有配置，如需修改请直接调整' : '请配置 Lights-Out 运行所需的参数'}
      </p>

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

      <ConfigSection title="服务配置">
        <Row gutter={20}>
          <Col span={12}>
            <div style={{ marginBottom: 16 }}>
              <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 6 }}>
                服务端口
              </label>
              <Input
                type="number"
                value={config.serverPort || 8080}
                onChange={(e) => onConfigUpdate({ serverPort: parseInt(e.target.value) || 8080 })}
                placeholder="8080"
              />
              <span style={{ fontSize: 12, color: '#999' }}>Web 控制台将在此端口运行</span>
            </div>
          </Col>
        </Row>
      </ConfigSection>
    </div>
  )
}