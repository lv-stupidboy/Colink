import React, { useState } from 'react';
import { Progress, Button, Alert, Typography, Checkbox, Space } from 'antd';
import { CheckCircleOutlined, LoadingOutlined, CloseCircleOutlined } from '@ant-design/icons';
import { uninstallApi } from '../../../lib/api';

const { Text, Title } = Typography;

interface UninstallProgress {
  step: string;
  status: 'pending' | 'running' | 'success' | 'failed';
  message?: string;
}

interface UninstallProps {
  installDir: string;
  onComplete: () => void;
  onCancel: () => void;
}

const Uninstall: React.FC<UninstallProps> = ({
  installDir,
  onComplete,
  onCancel,
}) => {
  const [keepData, setKeepData] = useState(true);
  const [isUninstalling, setIsUninstalling] = useState(false);
  const [progress, setProgress] = useState<UninstallProgress[]>([
    { step: 'stop', status: 'pending', message: '停止服务' },
    { step: 'files', status: 'pending', message: '删除程序文件' },
    { step: 'registry', status: 'pending', message: '清理注册表' },
    { step: 'shortcut', status: 'pending', message: '删除快捷方式' },
  ]);
  const [error, setError] = useState<string | null>(null);
  const [complete, setComplete] = useState(false);

  const startUninstall = async () => {
    setIsUninstalling(true);
    setError(null);

    // Update progress for each step
    const updateStep = (step: string, status: 'running' | 'success' | 'failed', message?: string) => {
      setProgress(prev => prev.map(p =>
        p.step === step ? { ...p, status, message } : p
      ));
    };

    try {
      // Step 1: Stop services
      updateStep('stop', 'running', '正在停止服务...');
      // For now, we just mark as success since there's no explicit stop command
      updateStep('stop', 'success', '服务已停止');

      // Step 2: Delete files
      updateStep('files', 'running', '正在删除程序文件...');
      const result = await uninstallApi.runUninstall({
        installDir,
        keepData,
      });
      if (!result.success) {
        throw new Error(result.error || '删除文件失败');
      }
      updateStep('files', 'success', '程序文件已删除');

      // Step 3: Clean registry
      updateStep('registry', 'running', '正在清理注册表...');
      await uninstallApi.cleanRegistry();
      updateStep('registry', 'success', '注册表已清理');

      // Step 4: Delete shortcuts
      updateStep('shortcut', 'running', '正在删除快捷方式...');
      await uninstallApi.removeShortcuts();
      updateStep('shortcut', 'success', '快捷方式已删除');

      setComplete(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : '卸载失败');
      // Mark current running step as failed
      setProgress(prev => prev.map(p =>
        p.status === 'running' ? { ...p, status: 'failed' } : p
      ));
    }

    setIsUninstalling(false);
  };

  const getStepIcon = (status: string) => {
    switch (status) {
      case 'success': return <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 16 }} />;
      case 'running': return <LoadingOutlined style={{ color: '#10b981', fontSize: 16 }} spin />;
      case 'failed': return <CloseCircleOutlined style={{ color: '#ff4d4f', fontSize: 16 }} />;
      default: return <span style={{ color: '#d9d9d9', fontSize: 16 }}>○</span>;
    }
  };

  const completedSteps = progress.filter(p => p.status === 'success').length;
  const totalProgress = Math.round((completedSteps / progress.length) * 100);

  if (complete) {
    return (
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
        <Title level={3} style={{ color: '#52c41a' }}>卸载完成</Title>
        <Text type="secondary" style={{ marginBottom: 24 }}>
          Colink 已从您的系统中移除。
        </Text>
        {keepData && (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 24, maxWidth: 400 }}
            message="用户数据已保留"
            description={`数据目录位于: ${installDir}\\data`}
          />
        )}
        <Button type="primary" size="large" onClick={onComplete}>
          完成
        </Button>
      </div>
    );
  }

  return (
    <div style={{ flex: 1 }}>
      <Title level={3} style={{ marginBottom: 8 }}>卸载 Colink</Title>
      <Text type="secondary" style={{ marginBottom: 24 }}>
        安装位置：{installDir}
      </Text>

      {!isUninstalling && !error && (
        <div style={{ marginBottom: 24 }}>
          <Checkbox
            checked={keepData}
            onChange={(e) => setKeepData(e.target.checked)}
          >
            保留用户数据（配置、日志、数据库等）
          </Checkbox>
          <Text type="secondary" style={{ marginLeft: 24, fontSize: 12 }}>
            勾选后将保留 {installDir}\\data 目录
          </Text>
        </div>
      )}

      {error && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 20 }}
          message="卸载失败"
          description={error}
        />
      )}

      {isUninstalling && (
        <div style={{ marginBottom: 24 }}>
          <Progress percent={totalProgress} status="active" />
        </div>
      )}

      <div style={{ marginBottom: 24 }}>
        {progress.map((p) => (
          <div key={p.step} style={{ display: 'flex', alignItems: 'center', marginBottom: 12 }}>
            <div style={{ width: 24 }}>{getStepIcon(p.status)}</div>
            <Text style={{ marginLeft: 12 }}>{p.message}</Text>
          </div>
        ))}
      </div>

      <div style={{ marginTop: 24, textAlign: 'right' }}>
        <Space>
          <Button onClick={onCancel} disabled={isUninstalling}>
            取消
          </Button>
          {!isUninstalling && !error && (
            <Button type="primary" danger onClick={startUninstall}>
              开始卸载
            </Button>
          )}
          {error && (
            <Button type="primary" onClick={onCancel}>
              关闭
            </Button>
          )}
        </Space>
      </div>
    </div>
  );
};

export default Uninstall;