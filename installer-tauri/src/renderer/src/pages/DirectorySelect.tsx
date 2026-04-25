import React, { useState, useEffect } from 'react';
import { Button, Input, Typography, Checkbox, Alert } from 'antd';
import { FolderOpenOutlined, InfoCircleOutlined } from '@ant-design/icons';
import { installApi } from '../../../lib/api';

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

interface DirectorySelectProps {
  config: InstallConfig;
  onConfigUpdate: (updates: Partial<InstallConfig>) => void;
  installedVersion?: InstalledVersion;
  installType?: 'fresh' | 'upgrade' | 'reinstall';
  onValidationChange?: (valid: boolean) => void;
}

const DirectorySelect: React.FC<DirectorySelectProps> = ({
  config,
  onConfigUpdate,
  installedVersion,
  installType = 'fresh',
  onValidationChange
}) => {
  const [freeSpace, setFreeSpace] = useState<number>(0);
  const [dirChanged, setDirChanged] = useState<boolean>(false);

  const isReinstall = installType === 'reinstall';
  const originalDir = installedVersion?.installDir || '';

  // 如果之前安装过，自动填入已安装的路径
  useEffect(() => {
    if (installedVersion?.installDir) {
      onConfigUpdate({ installDir: installedVersion.installDir });
      setDirChanged(false);
    }
  }, [installedVersion?.installDir]);

  // 检测目录是否改变
  useEffect(() => {
    if (isReinstall && originalDir) {
      const changed = config.installDir !== originalDir;
      setDirChanged(changed);
    }
  }, [config.installDir, originalDir, isReinstall]);

  const checkDiskSpace = async (path: string) => {
    if (!path || path.trim() === '') {
      setFreeSpace(0);
      onValidationChange?.(true);
      return;
    }

    const normalizedPath = path.replace(/\//g, '\\');
    const windowsPathRegex = /^[A-Za-z]:\\/;
    if (!windowsPathRegex.test(normalizedPath)) {
      setFreeSpace(0);
      onValidationChange?.(true);
      return;
    }

    const drive = normalizedPath.substring(0, 2).toUpperCase();

    try {
      const result = await installApi.getDiskSpace(drive);
      setFreeSpace(result.free);
    } catch {
      setFreeSpace(0);
    }

    onValidationChange?.(true);
  };

  useEffect(() => {
    checkDiskSpace(config.installDir);
  }, [config.installDir]);

  const handleBrowse = async () => {
    try {
      const result = await installApi.selectDirectory(config.installDir);
      if (result) {
        onConfigUpdate({ installDir: result });
      }
    } catch (err) {
      console.error('Failed to select directory:', err);
    }
  };

  const formatSize = (bytes: number) => {
    if (bytes === 0) return '未知';
    const gb = bytes / (1024 * 1024 * 1024);
    return `${gb.toFixed(1)} GB`;
  };

  return (
    <div style={{ flex: 1 }}>
      <Title level={3} style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>选择安装位置</Title>
      <Text style={{ color: '#666', marginBottom: 30 }}>请选择 Colink 的安装目录</Text>

      {/* 重新安装时显示原安装路径 */}
      {isReinstall && originalDir && (
        <Alert
          type="info"
          showIcon
          icon={<InfoCircleOutlined />}
          style={{ marginBottom: 20 }}
          message={`原安装路径：${originalDir}`}
          description={dirChanged
            ? '您选择了新的安装目录，原目录的程序文件将被删除。'
            : '保持原目录将覆盖安装程序文件。'
          }
        />
      )}

      <div style={{ marginBottom: 20 }}>
        <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 8 }}>
          安装目录
        </label>
        <div style={{ display: 'flex', gap: 12 }}>
          <Input
            value={config.installDir}
            onChange={(e) => onConfigUpdate({ installDir: e.target.value })}
            style={{ flex: 1 }}
          />
          <Button icon={<FolderOpenOutlined />} onClick={handleBrowse}>
            浏览...
          </Button>
        </div>
        <Text style={{ color: '#999', fontSize: 12, marginTop: 8 }}>
          目录不存在时将自动创建
        </Text>
      </div>

      {/* 重新安装时显示保留数据选项 */}
      {isReinstall && (
        <div style={{ marginBottom: 20 }}>
          <Checkbox
            checked={config.keepData}
            onChange={(e) => onConfigUpdate({ keepData: e.target.checked })}
          >
            保留用户数据（数据库、配置、日志等）
          </Checkbox>
          <div style={{ marginLeft: 24, marginTop: 4, color: '#999', fontSize: 12 }}>
            {dirChanged
              ? '数据将从原目录迁移到新目录'
              : '将保留现有 data 目录中的所有用户数据'
            }
          </div>
        </div>
      )}

      <div style={{ display: 'flex', gap: 40, color: '#666', fontSize: 14 }}>
        <span>所需空间：约 500 MB</span>
        <span>可用空间：{formatSize(freeSpace)}</span>
      </div>
    </div>
  );
};

export default DirectorySelect;