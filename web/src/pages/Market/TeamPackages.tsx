import React, { useState, useEffect } from 'react';
import {
  Card, Table, Button, Space, Tag, message, Spin, Modal,
  Descriptions, Collapse, Typography, Divider
} from 'antd';
import {
  CloudDownloadOutlined, SyncOutlined, ShopOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { MarketPackage, PackagePreviewResponse } from '@/types';

const { Title, Text } = Typography;

const TeamPackages: React.FC = () => {
  const [packages, setPackages] = useState<MarketPackage[]>([]);
  const [loading, setLoading] = useState(false);
  const [refreshingAll, setRefreshingAll] = useState(false);
  const [syncingPackage, setSyncingPackage] = useState<string | null>(null);
  const [previewingPackage, setPreviewingPackage] = useState<string | null>(null);
  const [previewData, setPreviewData] = useState<PackagePreviewResponse | null>(null);
  const [previewModalVisible, setPreviewModalVisible] = useState(false);

  useEffect(() => {
    loadPackages();
  }, []);

  const loadPackages = async () => {
    setLoading(true);
    try {
      const result = await api.markets.getTeamPackages();
      setPackages(result.data);
    } catch (error: any) {
      message.error(error.response?.data?.error || '加载团队包列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleRefreshAll = async () => {
    setRefreshingAll(true);
    try {
      const markets = await api.markets.list();
      for (const market of markets.data) {
        if (market.enabled) {
          await api.markets.refresh(market.id);
        }
      }
      message.success('所有市场已刷新');
      loadPackages();
    } catch (error: any) {
      message.error('刷新失败');
    } finally {
      setRefreshingAll(false);
    }
  };

  const handlePreview = async (pkg: MarketPackage) => {
    setPreviewingPackage(pkg.name);
    try {
      const result = await api.teamPackages.previewPackage(pkg.name, pkg.marketId);
      setPreviewData(result);
      setPreviewModalVisible(true);
    } catch (error: any) {
      message.error(error.response?.data?.error || '预览失败');
    } finally {
      setPreviewingPackage(null);
    }
  };

  const handleSync = async (pkg: MarketPackage) => {
    setSyncingPackage(pkg.name);
    try {
      await api.teamPackages.syncPackage(pkg.name, undefined, pkg.marketId);
      message.success(`团队包 ${pkg.name} 导入成功`);
      loadPackages();
      setPreviewModalVisible(false);
    } catch (error: any) {
      message.error(error.response?.data?.error || '导入失败');
    } finally {
      setSyncingPackage(null);
    }
  };

  const getStatusTag = (status: string) => {
    const colors: Record<string, string> = {
      new: 'blue',
      update: 'orange',
      latest: 'green',
    };
    const labels: Record<string, string> = {
      new: '未导入',
      update: '待更新',
      latest: '已导入',
    };
    return <Tag color={colors[status]}>{labels[status]}</Tag>;
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 120,
    },
    {
      title: '来源市场',
      dataIndex: 'marketName',
      key: 'marketName',
      width: 150,
    },
    {
      title: '本地版本',
      dataIndex: 'localVersion',
      key: 'localVersion',
      width: 120,
      render: (v?: string) => v || '-',
    },
    {
      title: '状态',
      dataIndex: 'localStatus',
      key: 'localStatus',
      width: 100,
      render: getStatusTag,
    },
    {
      title: '操作',
      key: 'action',
      width: 180,
      render: (_: any, record: MarketPackage) => {
        const isPreviewing = previewingPackage === record.name;
        const buttonText = record.localStatus === 'new' ? '导入' :
                           record.localStatus === 'update' ? '更新' : '重新导入';
        return (
          <Button
            type={record.localStatus === 'new' ? 'primary' : 'default'}
            size="small"
            icon={<CloudDownloadOutlined />}
            loading={isPreviewing}
            onClick={() => handlePreview(record)}
          >
            {buttonText}
          </Button>
        );
      },
    },
  ];

  // 渲染预览模态框
  const renderPreviewModal = () => {
    if (!previewData) return null;

    const roleColumns = [
      { title: '角色名称', dataIndex: 'name', key: 'name', width: 150 },
      { title: '角色类型', dataIndex: 'role', key: 'role', width: 100 },
      { title: '描述', dataIndex: 'description', key: 'description' },
      {
        title: '绑定资产',
        dataIndex: 'assets',
        key: 'assets',
        render: (assets: string[]) => (
          <Space direction="vertical" size="small">
            {assets.map((asset, idx) => (
              <Tag key={idx} color={
                asset.startsWith('Skill:') ? 'blue' :
                asset.startsWith('Command:') ? 'green' :
                asset.startsWith('Subagent:') ? 'purple' :
                asset.startsWith('Rule:') ? 'orange' :
                asset.startsWith('Settings:') ? 'cyan' : 'default'
              }>
                {asset}
              </Tag>
            ))}
          </Space>
        ),
      },
    ];

    const assetColumns = [
      { title: '名称', dataIndex: 'name', key: 'name', width: 200 },
      { title: '描述', dataIndex: 'description', key: 'description' },
    ];

    return (
      <Modal
        title={`导入预览 - ${previewData.packageName}`}
        open={previewModalVisible}
        onCancel={() => setPreviewModalVisible(false)}
        width={800}
        footer={[
          <Button key="cancel" onClick={() => setPreviewModalVisible(false)}>
            取消
          </Button>,
          <Button
            key="import"
            type="primary"
            icon={<CloudDownloadOutlined />}
            loading={syncingPackage === previewData.packageName}
            onClick={() => {
              const pkg = packages.find(p => p.name === previewData.packageName);
              if (pkg) handleSync(pkg);
            }}
          >
            确认导入
          </Button>,
        ]}
      >
        <Descriptions bordered column={2} size="small">
          <Descriptions.Item label="团队包名称">{previewData.packageName}</Descriptions.Item>
          <Descriptions.Item label="版本">{previewData.version}</Descriptions.Item>
          <Descriptions.Item label="描述" span={2}>{previewData.description}</Descriptions.Item>
        </Descriptions>

        <Divider />

        <Title level={5}>团队信息</Title>
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="团队名称">{previewData.workflow.name}</Descriptions.Item>
          <Descriptions.Item label="团队描述">{previewData.workflow.description}</Descriptions.Item>
        </Descriptions>

        <Divider />

        <Title level={5}>角色 Agent ({previewData.roles.length} 个)</Title>
        <Table
          dataSource={previewData.roles}
          columns={roleColumns}
          rowKey="name"
          pagination={false}
          size="small"
        />

        <Divider />

        <Collapse
          items={[
            {
              key: 'skills',
              label: `Skills (${previewData.assets.skills?.length || 0})`,
              children: (
                <Table
                  dataSource={previewData.assets.skills || []}
                  columns={assetColumns}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              ),
            },
            {
              key: 'commands',
              label: `Commands (${previewData.assets.commands?.length || 0})`,
              children: (
                <Table
                  dataSource={previewData.assets.commands || []}
                  columns={assetColumns}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              ),
            },
            {
              key: 'subagents',
              label: `Subagents (${previewData.assets.subagents?.length || 0})`,
              children: (
                <Table
                  dataSource={previewData.assets.subagents || []}
                  columns={assetColumns}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              ),
            },
            {
              key: 'rules',
              label: `Rules (${previewData.assets.rules?.length || 0})`,
              children: (
                <Table
                  dataSource={previewData.assets.rules || []}
                  columns={assetColumns}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              ),
            },
            {
              key: 'settings',
              label: `Settings (${previewData.assets.settings?.length || 0})`,
              children: (
                <Table
                  dataSource={previewData.assets.settings || []}
                  columns={assetColumns}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              ),
            },
          ]}
        />
      </Modal>
    );
  };

  return (
    <div className="team-packages">
      <Card
        title={
          <Space>
            <ShopOutlined />
            <span>远程团队包</span>
          </Space>
        }
        extra={
          <Button icon={<SyncOutlined />} onClick={handleRefreshAll} loading={refreshingAll}>
            刷新全部市场
          </Button>
        }
      >
        <Spin spinning={loading}>
          <Table
            dataSource={packages}
            columns={columns}
            rowKey={(record) => `${record.marketId}-${record.name}`}
            pagination={{ pageSize: 20 }}
          />
        </Spin>
      </Card>

      {renderPreviewModal()}
    </div>
  );
};

export default TeamPackages;