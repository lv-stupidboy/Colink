import React, { useState, useEffect } from 'react';
import {
  Card, Table, Button, Space, Tag, message, Spin, Modal,
  Descriptions, Collapse, Typography, Divider, Progress
} from 'antd';
import {
  CloudDownloadOutlined, ShopOutlined, CheckSquareOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { MarketPackage, PackagePreviewResponse } from '@/types';

const { Title, Text } = Typography;

const TeamPackages: React.FC = () => {
  const [packages, setPackages] = useState<MarketPackage[]>([]);
  const [loading, setLoading] = useState(false);
  const [syncingPackage, setSyncingPackage] = useState<string | null>(null);
  const [previewingPackage, setPreviewingPackage] = useState<string | null>(null);
  const [previewData, setPreviewData] = useState<PackagePreviewResponse | null>(null);
  const [previewModalVisible, setPreviewModalVisible] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [batchImporting, setBatchImporting] = useState(false);
  const [batchProgress, setBatchProgress] = useState({ current: 0, total: 0, success: 0, failed: 0 });
  const [batchResults, setBatchResults] = useState<Array<{ name: string; status: 'success' | 'failed'; error?: string }>>([]);
  const [batchModalVisible, setBatchModalVisible] = useState(false);
  const [confirmModalVisible, setConfirmModalVisible] = useState(false);
  const [pendingImportPackages, setPendingImportPackages] = useState<MarketPackage[]>([]);

  useEffect(() => {
    loadPackages();
  }, []);

  const loadPackages = async () => {
    setLoading(true);
    try {
      const result = await api.markets.getTeamPackages();
      setPackages(result.data);
      setSelectedRowKeys([]);
    } catch (error: any) {
      message.error(error.response?.data?.error || '加载团队包列表失败');
    } finally {
      setLoading(false);
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

  // 点击批量导入按钮 -> 显示确认弹框
  const handleBatchImportClick = () => {
    if (selectedRowKeys.length === 0) {
      message.warning('请先选择要导入的团队包');
      return;
    }

    const toImport = packages.filter(pkg =>
      selectedRowKeys.includes(`${pkg.marketId}-${pkg.name}`)
    );
    setPendingImportPackages(toImport);
    setConfirmModalVisible(true);
  };

  // 确认后执行批量导入
  const handleBatchImportConfirm = async () => {
    setConfirmModalVisible(false);
    setBatchImporting(true);
    setBatchProgress({ current: 0, total: pendingImportPackages.length, success: 0, failed: 0 });
    setBatchResults([]);
    setBatchModalVisible(true);

    const results: Array<{ name: string; status: 'success' | 'failed'; error?: string }> = [];

    for (let i = 0; i < pendingImportPackages.length; i++) {
      const pkg = pendingImportPackages[i];
      setBatchProgress(prev => ({ ...prev, current: i + 1 }));

      try {
        await api.teamPackages.syncPackage(pkg.name, undefined, pkg.marketId);
        results.push({ name: pkg.name, status: 'success' });
        setBatchProgress(prev => ({ ...prev, success: prev.success + 1 }));
      } catch (error: any) {
        const errorMsg = error.response?.data?.error || '导入失败';
        results.push({ name: pkg.name, status: 'failed', error: errorMsg });
        setBatchProgress(prev => ({ ...prev, failed: prev.failed + 1 }));
      }
    }

    setBatchResults(results);
    setBatchImporting(false);
    setSelectedRowKeys([]);
    setPendingImportPackages([]);

    loadPackages();
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
      title: '最近导入',
      dataIndex: 'lastImportedAt',
      key: 'lastImportedAt',
      width: 150,
      render: (v?: string) => v ? new Date(v).toLocaleString() : '-',
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
            <span>团队包</span>
          </Space>
        }
        extra={
          <Space>
            <Text type="secondary">
              已选 {selectedRowKeys.length} 项
            </Text>
            <Button
              type="primary"
              icon={<CheckSquareOutlined />}
              onClick={handleBatchImportClick}
              disabled={selectedRowKeys.length === 0 || batchImporting}
              loading={batchImporting}
            >
              批量导入
            </Button>
          </Space>
        }
      >
        <Spin spinning={loading}>
          <Table
            dataSource={packages}
            columns={columns}
            rowKey={(record) => `${record.marketId}-${record.name}`}
            pagination={{ pageSize: 20 }}
            rowSelection={{
              selectedRowKeys,
              onChange: setSelectedRowKeys,
            }}
          />
        </Spin>
      </Card>

      {/* 批量导入进度/结果弹框 */}
      <Modal
        title="批量导入"
        open={batchModalVisible}
        onCancel={() => {
          if (!batchImporting) {
            setBatchModalVisible(false);
            setBatchResults([]);
          }
        }}
        footer={batchImporting ? null : [
          <Button key="close" onClick={() => {
            setBatchModalVisible(false);
            setBatchResults([]);
          }}>
            关闭
          </Button>,
        ]}
        width={500}
      >
        {batchImporting ? (
          <div>
            <Progress
              percent={Math.round(batchProgress.current / batchProgress.total * 100)}
              status="active"
              format={() => `${batchProgress.current}/${batchProgress.total}`}
            />
            <Text type="secondary" style={{ marginTop: 8 }}>
              成功: {batchProgress.success} | 失败: {batchProgress.failed}
            </Text>
          </div>
        ) : (
          <div>
            <div style={{ marginBottom: 12 }}>
              <Text strong>导入结果</Text>
              <Text type="secondary" style={{ marginLeft: 12 }}>
                成功 {batchProgress.success} 个，失败 {batchProgress.failed} 个
              </Text>
            </div>
            <Space direction="vertical" size="small" style={{ width: '100%' }}>
              {batchResults.map((result, idx) => (
                <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Tag color={result.status === 'success' ? 'green' : 'red'}>
                    {result.status === 'success' ? '成功' : '失败'}
                  </Tag>
                  <Text>{result.name}</Text>
                  {result.error && (
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      ({result.error})
                    </Text>
                  )}
                </div>
              ))}
            </Space>
          </div>
        )}
      </Modal>

      {/* 批量导入确认弹框 */}
      <Modal
        title="确认批量导入"
        open={confirmModalVisible}
        onCancel={() => setConfirmModalVisible(false)}
        onOk={handleBatchImportConfirm}
        okText="确认导入"
        cancelText="取消"
        width={500}
      >
        <Text>将导入以下 {pendingImportPackages.length} 个团队包：</Text>
        <div style={{ marginTop: 12, maxHeight: 200, overflow: 'auto' }}>
          {pendingImportPackages.map((pkg, idx) => (
            <div key={idx} style={{ padding: '4px 0' }}>
              <Text>{pkg.name}</Text>
              <Tag color="blue" style={{ marginLeft: 8 }}>{pkg.version}</Tag>
              <Tag color={pkg.localStatus === 'new' ? 'blue' : pkg.localStatus === 'update' ? 'orange' : 'green'} style={{ marginLeft: 4 }}>
                {pkg.localStatus === 'new' ? '未导入' : pkg.localStatus === 'update' ? '待更新' : '已导入'}
              </Tag>
            </div>
          ))}
        </div>
      </Modal>

      {renderPreviewModal()}
    </div>
  );
};

export default TeamPackages;