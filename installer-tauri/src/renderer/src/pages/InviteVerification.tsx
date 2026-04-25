import React, { useState, useEffect } from 'react';
import { Button, Input, Typography, Space, message, Alert, Spin } from 'antd';
import { inviteApi } from '../../../lib/api';

const { Title, Text } = Typography;

interface InstallConfig {
  installDir: string;
  createShortcut: boolean;
  launchNow: boolean;
  keepData: boolean;
  dependencies: any[];
}

interface InstalledVersion {
  installed: boolean;
  version?: string;
  installDir?: string;
}

interface InviteVerificationProps {
  config: InstallConfig;
  onConfigUpdate: (updates: Partial<InstallConfig>) => void;
  installedVersion?: InstalledVersion;
  isUpgrade?: boolean;
  onValidationChange?: (valid: boolean) => void;
}

const InviteVerification: React.FC<InviteVerificationProps> = ({
  config,
  onValidationChange
}) => {
  const [inviteCode, setInviteCode] = useState('');
  const [username, setUsername] = useState('');
  const [loadingUsername, setLoadingUsername] = useState(true);
  const [loading, setLoading] = useState(false);
  const [verified, setVerified] = useState(false);

  useEffect(() => {
    loadInitialData();
  }, []);

  useEffect(() => {
    onValidationChange?.(verified);
  }, [verified]);

  const loadInitialData = async () => {
    setLoadingUsername(true);
    try {
      const name = await inviteApi.getSystemUsername();
      setUsername(name);
    } catch (err) {
      console.error('Failed to get system username:', err);
      // Fallback: Try to detect from browser environment
      // In web debug mode, Tauri commands don't work, so we need a fallback
      message.warning('无法获取系统用户名，请手动输入');
    } finally {
      setLoadingUsername(false);
    }

    // Try to load existing invite code
    try {
      const loaded = await inviteApi.load(config.installDir);
      if (loaded.success && loaded.inviteCode) {
        setInviteCode(loaded.inviteCode);
      }
    } catch (err) {
      console.error('Failed to load invite code:', err);
    }
  };

  const handleVerify = async () => {
    if (!inviteCode) {
      message.warning('请输入邀请码');
      return;
    }
    if (!username) {
      message.warning('请输入用户名');
      return;
    }

    setLoading(true);
    try {
      const result = await inviteApi.verify(inviteCode, username);
      if (result.success) {
        setVerified(true);
        message.success('验证成功');
      } else {
        message.error(result.message || '验证失败');
      }
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : '验证请求失败';
      message.error(errorMsg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ maxWidth: 500, margin: '0 auto' }}>
      <Title level={3} style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>邀请码验证</Title>
      <Text style={{ color: '#666', marginBottom: 24 }}>请输入您的邀请码以完成安装验证</Text>

      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <div>
          <Text style={{ marginBottom: 4, display: 'block' }}>用户名:</Text>
          {loadingUsername ? (
            <Spin size="small" />
          ) : (
            <Input
              value={username}
              onChange={(e) => {
                setUsername(e.target.value);
                setVerified(false);
              }}
              placeholder="请输入系统用户名"
              size="large"
              style={{ marginTop: 4 }}
            />
          )}
          <Text style={{ fontSize: 12, color: '#999', marginTop: 4 }}>
            用户名应与系统用户目录名称一致（如 Windows 用户名）
          </Text>
        </div>

        <Input
          placeholder="输入邀请码"
          value={inviteCode}
          onChange={(e) => {
            setInviteCode(e.target.value);
            setVerified(false);
          }}
          size="large"
        />

        <Button
          type="primary"
          onClick={handleVerify}
          loading={loading}
          disabled={!username || !inviteCode}
          size="large"
          block
        >
          验证邀请码
        </Button>

        {verified && (
          <Alert message="邀请码验证成功" type="success" showIcon />
        )}
      </Space>
    </div>
  );
};

export default InviteVerification;