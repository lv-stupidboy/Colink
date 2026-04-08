import { useState, useEffect } from 'react'
import { Button, Input, Row, Col, message, Spin, Typography } from 'antd'
import { CheckCircleOutlined, EditOutlined } from '@ant-design/icons'
import ConfigSection from '../components/ConfigSection'
import { InstallConfig, InstalledVersion } from '../types'
import { testConnection } from '../services/database-connector'

const { Text } = Typography
const { TextArea } = Input

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
  const [mergedConfigYaml, setMergedConfigYaml] = useState<string>('')
  const [editingYaml, setEditingYaml] = useState(false)
  const [yamlError, setYamlError] = useState<string | null>(null)
  const [configMerged, setConfigMerged] = useState(false)

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
          setConfigMerged(true)
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

  // 加载完整配置预览
  useEffect(() => {
    const loadFullConfig = async () => {
      const targetDir = config.installDir || installedVersion?.installDir
      if (!targetDir) {
        // 生成默认配置预览
        await generateDefaultConfig()
        return
      }

      try {
        const yaml = await window.electronAPI.readFullConfig(targetDir)
        if (yaml) {
          setMergedConfigYaml(yaml)
        } else {
          await generateDefaultConfig()
        }
      } catch (e) {
        console.warn('[SystemConfig] Failed to load full config:', e)
        await generateDefaultConfig()
      }
    }

    // 延迟加载，避免阻塞渲染
    const timer = setTimeout(() => {
      loadFullConfig().catch(e => console.error('[SystemConfig] loadFullConfig error:', e))
    }, 100)

    return () => clearTimeout(timer)
  }, [config.installDir, installedVersion])

  // 生成默认配置预览
  const generateDefaultConfig = async () => {
    try {
      const result = await window.electronAPI.generateConfig({
        database: config.database,
        serverPort: config.serverPort || 8080
      })
      if (result.success && result.yaml) {
        setMergedConfigYaml(result.yaml)
      }
    } catch (e) {
      console.warn('[SystemConfig] Failed to generate config:', e)
      setMergedConfigYaml('# 配置加载失败')
    }
  }

  // 当数据库配置变化时更新预览
  useEffect(() => {
    if (!editingYaml && !mergedConfigYaml) {
      generateDefaultConfig().catch(e => console.error(e))
    }
  }, [config.database, config.serverPort, editingYaml, mergedConfigYaml])

  const handleDbChange = (field: string, value: string | number) => {
    onConfigUpdate({
      database: { ...config.database, [field]: value }
    })
    setConfigMerged(false)
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

  // YAML 编辑处理
  const handleYamlChange = (value: string) => {
    setMergedConfigYaml(value)
    setYamlError(null)
  }

  const handleSaveYaml = () => {
    // 简单校验 YAML 格式
    try {
      // 这里只是简单检查，实际保存时后端会完整校验
      if (mergedConfigYaml.trim().length === 0) {
        setYamlError('配置不能为空')
        return
      }
      setEditingYaml(false)
      setYamlError(null)
      message.success('配置已更新')
    } catch (e) {
      setYamlError('配置格式错误')
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
    <div style={{ flex: 1, overflow: 'auto' }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>系统配置</h2>
      <p style={{ color: '#666', marginBottom: 20 }}>
        {isUpgrade ? '已加载现有配置，如需修改请直接调整' : '请配置 Lights-Out 运行所需的参数'}
        {configMerged && isUpgrade && (
          <span style={{ color: '#52c41a', marginLeft: 8 }}>
            <CheckCircleOutlined /> 配置已合并
          </span>
        )}
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
                onChange={(e) => {
                  onConfigUpdate({ serverPort: parseInt(e.target.value) || 8080 })
                  setConfigMerged(false)
                }}
                placeholder="8080"
              />
              <span style={{ fontSize: 12, color: '#999' }}>Web 控制台将在此端口运行</span>
            </div>
          </Col>
        </Row>
      </ConfigSection>

      {/* 完整配置预览 */}
      <ConfigSection title="完整配置预览">
        <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Text type="secondary">
            {isUpgrade && configMerged
              ? '升级时将自动合并您的配置与新模板（您的值优先）'
              : '以下是将生成的完整配置文件内容'
            }
          </Text>
          <div style={{ display: 'flex', gap: 8 }}>
            {editingYaml ? (
              <>
                <Button size="small" onClick={() => {
                  setEditingYaml(false)
                  loadFullConfig()
                  setYamlError(null)
                }}>
                  取消
                </Button>
                <Button size="small" type="primary" onClick={handleSaveYaml}>
                  保存
                </Button>
              </>
            ) : (
              <Button size="small" icon={<EditOutlined />} onClick={() => setEditingYaml(true)}>
                编辑
              </Button>
            )}
          </div>
        </div>

        {yamlError && (
          <div style={{
            background: '#fff2f0',
            border: '1px solid #ffccc7',
            borderRadius: 4,
            padding: 8,
            marginBottom: 12,
            color: '#cf1322'
          }}>
            {yamlError}
          </div>
        )}

        <div style={{
          background: '#fafafa',
          border: '1px solid #e8e8e8',
          borderRadius: 6,
          overflow: 'hidden'
        }}>
          {editingYaml ? (
            <TextArea
              value={mergedConfigYaml}
              onChange={(e) => handleYamlChange(e.target.value)}
              autoSize={{ minRows: 15, maxRows: 25 }}
              style={{
                fontFamily: 'Consolas, Monaco, monospace',
                fontSize: 12,
                lineHeight: 1.5,
                border: 'none',
                background: 'transparent'
              }}
            />
          ) : (
            <pre style={{
              margin: 0,
              padding: 12,
              maxHeight: 400,
              overflow: 'auto',
              fontFamily: 'Consolas, Monaco, monospace',
              fontSize: 12,
              lineHeight: 1.5,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word'
            }}>
              {mergedConfigYaml || '加载中...'}
            </pre>
          )}
        </div>
      </ConfigSection>
    </div>
  )
}