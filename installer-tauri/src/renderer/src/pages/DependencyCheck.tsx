import React, { useState, useEffect } from 'react';
import { Button, Table, Typography, Tag, Spin, message, Tooltip } from 'antd';
import { openUrl } from '@tauri-apps/plugin-opener';
import { dependencyApi } from '../../../lib/api';
import type { DependencyInfo } from '../../../lib/api/types';

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

interface DependencyCheckProps {
  config: InstallConfig;
  onConfigUpdate: (updates: Partial<InstallConfig>) => void;
  installedVersion?: InstalledVersion;
  isUpgrade?: boolean;
  onValidationChange?: (valid: boolean) => void;
}

const DependencyCheck: React.FC<DependencyCheckProps> = ({
  onConfigUpdate
}) => {
  const [dependencies, setDependencies] = useState<DependencyInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [installing, setInstalling] = useState<string | null>(null);

  useEffect(() => {
    checkDependencies();
  }, []);

  const checkDependencies = async () => {
    setLoading(true);
    try {
      const deps = await dependencyApi.checkAll();
      setDependencies(deps);
      onConfigUpdate({ dependencies: deps });
    } catch (err) {
      console.error('Failed to check dependencies:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleInstall = async (key: string) => {
    setInstalling(key);
    try {
      await dependencyApi.install(key);
      message.success(`${key} 安装成功`);
      await checkDependencies();
    } catch (err) {
      message.error(`${key} 安装失败`);
    } finally {
      setInstalling(null);
    }
  };

  const handleOpenDownload = async (url: string) => {
    try {
      await openUrl(url);
    } catch (err) {
      console.error('open download url failed:', err);
      message.error('打开下载页失败，请手动复制链接到浏览器');
    }
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '状态',
      dataIndex: 'installed',
      key: 'installed',
      render: (_: boolean, record: DependencyInfo) => {
        if (record.installed) {
          return <Tag color="green">已安装</Tag>;
        }
        // 有 detectError 说明尝试了所有兜底策略仍失败，显示"检测失败"
        if (record.detectError) {
          return (
            <Tooltip title={record.detectError} color="orange">
              <Tag color="orange">检测失败</Tag>
            </Tooltip>
          );
        }
        return <Tag color="red">未安装</Tag>;
      },
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      render: (version: string | undefined, record: DependencyInfo) => {
        if (!version && record.detectError) {
          return (
            <Tooltip title={record.detectError} color="orange">
              <span style={{ color: '#fa8c16', cursor: 'help' }}>检测失败</span>
            </Tooltip>
          );
        }
        return version || '-';
      },
    },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: DependencyInfo) => {
        if (record.installed) {
          return <Tag color="green">✓</Tag>;
        }
        // code-agent 等只能手动下载的依赖：有 download_url 显示"前往下载"，否则提示去 config.yaml 配置
        if (record.key === 'code-agent') {
          if (record.downloadUrl) {
            return (
              <Tooltip title={record.downloadUrl}>
                <Button size="small" onClick={() => handleOpenDownload(record.downloadUrl!)}>
                  前往下载
                </Button>
              </Tooltip>
            );
          }
          return (
            <Tooltip title="请在 config.yaml 的 code_agent.download_url 中配置下载页地址">
              <Tag color="default">未配置下载地址</Tag>
            </Tooltip>
          );
        }
        // claude / opencode 走 npm 自动安装
        return (
          <Button
            size="small"
            onClick={() => handleInstall(record.key)}
            loading={installing === record.key}
          >
            安装
          </Button>
        );
      },
    },
  ];

  return (
    <div style={{ flex: 1 }}>
      <Title level={3} style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>智能体检测</Title>
      <Text style={{ color: '#666', marginBottom: 24 }}>检查运行所需的依赖环境</Text>

      <Spin spinning={loading}>
        <Table
          columns={columns}
          dataSource={dependencies}
          rowKey="key"
          pagination={false}
          style={{ marginBottom: 24 }}
        />
      </Spin>

      <Button onClick={checkDependencies} style={{ marginBottom: 16 }}>
        重新检查
      </Button>

      <div style={{ color: '#999', fontSize: 12 }}>
        部分依赖为可选组件，即使未安装也可继续
      </div>
    </div>
  );
};

export default DependencyCheck;