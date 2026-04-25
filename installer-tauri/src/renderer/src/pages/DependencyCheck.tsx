import React, { useState, useEffect } from 'react';
import { Button, Table, Typography, Tag, Spin, message } from 'antd';
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
  config,
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
      render: (installed: boolean) => (
        <Tag color={installed ? 'green' : 'red'}>
          {installed ? '已安装' : '未安装'}
        </Tag>
      ),
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      render: (version?: string) => version || '-',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: DependencyInfo) =>
        record.installed ? (
          <Tag color="green">✓</Tag>
        ) : (
          <Button
            size="small"
            onClick={() => handleInstall(record.key)}
            loading={installing === record.key}
          >
            安装
          </Button>
        ),
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