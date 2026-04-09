import { useState } from 'react'
import { Button, Input, Alert, Typography } from 'antd'
import { CheckCircleOutlined, WarningOutlined } from '@ant-design/icons'
import { InstallConfig, InstalledVersion } from '../types'

const { Text } = Typography

interface InviteVerificationProps {
  config: InstallConfig
  onConfigUpdate: (updates: Partial<InstallConfig>) => void
  installedVersion?: InstalledVersion
  isUpgrade?: boolean
  onValidationChange?: (valid: boolean) => void
}

export default function InviteVerification({
  config,
  onConfigUpdate,
  onValidationChange
}: InviteVerificationProps) {
  const [code, setCode] = useState('')
  const [username, setUsername] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [verified, setVerified] = useState(false)

  const handleVerify = async () => {
    if (!code.trim() || !username.trim()) {
      setError('请填写邀请码和用户名')
      return
    }

    setLoading(true)
    setError(null)

    try {
      const response = await window.electronAPI.verifyInviteCode({
        code: code.trim(),
        username: username.trim()
      })

      if (response.success) {
        setVerified(true)
        // 保存验证状态到 config
        onConfigUpdate({
          verification: {
            verified: true,
            token: response.token,
            username: response.user?.username || username.trim(),
            verifiedAt: Date.now()
          }
        })
        onValidationChange?.(true)
      } else {
        setError(response.message || '验证失败，请检查邀请码和用户名')
        onValidationChange?.(false)
      }
    } catch (e) {
      setError('网络请求失败，请稍后重试')
      onValidationChange?.(false)
    } finally {
      setLoading(false)
    }
  }

  // 已验证状态显示
  if (verified) {
    return (
      <div style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center'
      }}>
        <CheckCircleOutlined style={{ fontSize: 64, color: '#10b981', marginBottom: 24 }} />
        <h2 style={{ fontSize: 22, marginBottom: 12, color: '#333' }}>验证成功</h2>
        <p style={{ color: '#666', marginBottom: 30 }}>
          用户名：{' '}
          <Text code style={{ background: '#f5f5f5', padding: '2px 8px', borderRadius: 4 }}>
            {config.verification?.username}
          </Text>
        </p>
      </div>
    )
  }

  // 输入表单
  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>验证邀请码</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>
        请输入您的邀请码和用户名以继续安装 Colink
      </p>

      <div style={{ maxWidth: 400 }}>
        <div style={{ marginBottom: 20 }}>
          <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 8 }}>
            邀请码
          </label>
          <Input
            placeholder="请输入邀请码"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            disabled={loading}
            size="large"
            onPressEnter={handleVerify}
          />
        </div>

        <div style={{ marginBottom: 20 }}>
          <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 8 }}>
            用户名
          </label>
          <Input
            placeholder="请输入用户名"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            disabled={loading}
            size="large"
            onPressEnter={handleVerify}
          />
        </div>

        {error && (
          <Alert
            type="error"
            message={error}
            icon={<WarningOutlined />}
            showIcon
            style={{ marginBottom: 20 }}
          />
        )}

        <Button
          type="primary"
          size="large"
          block
          loading={loading}
          onClick={handleVerify}
          disabled={!code.trim() || !username.trim()}
        >
          验证
        </Button>
      </div>
    </div>
  )
}