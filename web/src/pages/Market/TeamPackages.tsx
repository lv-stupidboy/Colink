import React, { useState, useEffect } from 'react';
import {
  Card, Table, Button, Space, Tag, message, Spin, Modal,
  Descriptions, Collapse, Typography, Divider, Progress,
  Alert, Popconfirm  // 新增：用于冲突提示和确认
} from 'antd';
import {
  CloudDownloadOutlined, ShopOutlined, CheckSquareOutlined,
  ReloadOutlined,  // 新增：用于刷新按钮
  WarningOutlined  // 新增：用于冲突提示图标
} from '@ant-design/icons';
import api from '@/api/client';
import type { MarketPackage, PackagePreviewResponse, ImportConfirm } from '@/types';
import { getCachedPackages, setCachedPackages, clearCache } from '@/utils/teamPackageCache';

const { Title, Text } = Typography;

// 跳过规则说明文本（统一常量）
const SKIP_RULE_DESCRIPTION = "跳过规则：按资产粒度处理。选择「全部跳过」将保留已存在的 Workflow、Roles 和 Assets，仅导入新增项。";

const TeamPackages: React.FC = () => {
  const [packages, setPackages] = useState<MarketPackage[]>([]);
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);  // 新增：刷新状态
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
  // 批量导入预览状态（新增）
  const [batchPreviewData, setBatchPreviewData] = useState<Map<string, PackagePreviewResponse>>(new Map());
  const [loadingBatchPreview, setLoadingBatchPreview] = useState(false);
  const [batchConflictTotal, setBatchConflictTotal] = useState(0);

  useEffect(() => {
    loadPackages();
  }, []);

  // 修改：支持缓存和强制刷新
  const loadPackages = async (forceRefresh = false) => {
    // 非强制刷新时，先尝试读取缓存
    if (!forceRefresh) {
      const cached = getCachedPackages();
      if (cached && cached.length > 0) {
        setPackages(cached);
        setLoading(false);
        return;
      }
    }

    setLoading(true);
    try {
      const result = await api.markets.getTeamPackages(forceRefresh);
      setPackages(result.data);
      setCachedPackages(result.data);  // 写入缓存
      setSelectedRowKeys([]);
    } catch (error: any) {
      message.error(error.response?.data?.error || '加载团队包列表失败');
    } finally {
      setLoading(false);
    }
  };

  // 新增：手动刷新处理函数
  const handleRefresh = async () => {
    setRefreshing(true);
    clearCache();
    await loadPackages(true);
    setRefreshing(false);
  };

  // 构建 ImportConfirm 参数的公共函数
  const buildImportConfirm = (preview: PackagePreviewResponse | null | undefined, mode: 'overwrite' | 'skip'): ImportConfirm => {
    return {
      mode,
      workflowAction: preview?.workflow?.exists ? mode : 'overwrite',
      roleActions: preview?.roles?.map(r => ({
        name: r.name,
        action: r.exists ? mode : 'overwrite',
      })) || [],
      assetActions: [
        ...(preview?.assets?.skills?.map(s => ({
          assetType: 'skill',
          name: s.name,
          action: s.exists ? mode : 'overwrite',
        })) || []),
        ...(preview?.assets?.commands?.map(c => ({
          assetType: 'command',
          name: c.name,
          action: c.exists ? mode : 'overwrite',
        })) || []),
        ...(preview?.assets?.subagents?.map(s => ({
          assetType: 'subagent',
          name: s.name,
          action: s.exists ? mode : 'overwrite',
        })) || []),
        ...(preview?.assets?.rules?.map(r => ({
          assetType: 'rule',
          name: r.name,
          action: r.exists ? mode : 'overwrite',
        })) || []),
        ...(preview?.assets?.settings?.map(s => ({
          assetType: 'settings',
          name: s.name,
          action: s.exists ? mode : 'overwrite',
        })) || []),
      ],
    };
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

  const handleSync = async (pkg: MarketPackage, mode: 'overwrite' | 'skip') => {
    setSyncingPackage(pkg.name);
    try {
      const confirm = buildImportConfirm(previewData, mode);

      await api.teamPackages.syncPackage(pkg.name, confirm, pkg.marketId);
      message.success(`团队包 ${pkg.name} 导入成功`);
      loadPackages();
      setPreviewModalVisible(false);
    } catch (error: any) {
      message.error(error.response?.data?.error || '导入失败');
    } finally {
      setSyncingPackage(null);
    }
  };

  // 点击批量导入按钮 -> 使用批量预览API（并行处理）
  const handleBatchImportClick = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning('请先选择要导入的团队包');
      return;
    }

    const toImport = packages.filter(pkg =>
      selectedRowKeys.includes(`${pkg.marketId}-${pkg.name}`)
    );
    setPendingImportPackages(toImport);
    setLoadingBatchPreview(true);

    try {
      // 使用批量预览API（并行处理）
      const result = await api.teamPackages.previewPackagesBatch(
        toImport.map(pkg => ({ name: pkg.name, marketId: pkg.marketId }))
      );

      // 构建预览Map
      const previewMap = new Map<string, PackagePreviewResponse>();
      result.previews.forEach(p => {
        if (p.data) {
          previewMap.set(p.name, p.data);
        } else if (p.error) {
          // 预览失败的包也记录
          const pkg = toImport.find(pkg => pkg.name === p.name);
          previewMap.set(p.name, {
            packageName: p.name,
            version: pkg?.version || '',
            description: pkg?.description || '',
            conflictCount: 0,
            previewFailed: true,
            workflow: { name: '', description: '', exists: false },
            roles: [],
            assets: { skills: [], commands: [], subagents: [], rules: [], settings: [] },
          } as PackagePreviewResponse);
        }
      });

      setBatchPreviewData(previewMap);
      setBatchConflictTotal(result.totalConflicts);
    } catch (error: any) {
      message.error(error.response?.data?.error || '批量预览失败');
      setPendingImportPackages([]);
    } finally {
      setLoadingBatchPreview(false);
      setConfirmModalVisible(true);
    }
  };

  // 确认后执行批量导入（使用批量API并行执行）
  const handleBatchImportConfirm = async (mode: 'overwrite' | 'skip') => {
    setConfirmModalVisible(false);
    setBatchImporting(true);
    setBatchProgress({ current: 0, total: pendingImportPackages.length, success: 0, failed: 0 });
    setBatchResults([]);
    setBatchModalVisible(true);

    try {
      // 构建批量导入请求
      const batchRequests = pendingImportPackages.map(pkg => {
        const preview = batchPreviewData.get(pkg.name);
        const confirm = buildImportConfirm(preview, mode);
        return {
          name: pkg.name,
          marketId: pkg.marketId,
          confirm,
        };
      });

      // 使用批量API并行导入
      const batchResult = await api.teamPackages.syncPackagesBatch(batchRequests);

      // 处理结果
      const results: Array<{ name: string; status: 'success' | 'failed'; error?: string }> =
        batchResult.results.map(r => ({
          name: r.name,
          status: r.error ? 'failed' : 'success',
          error: r.error,
        }));

      setBatchResults(results);
      setBatchProgress({
        current: pendingImportPackages.length,
        total: pendingImportPackages.length,
        success: batchResult.successCount,
        failed: batchResult.failedCount,
      });
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || '批量导入失败';
      message.error(errorMsg);
      // 全部标记为失败
      const results = pendingImportPackages.map(pkg => ({
        name: pkg.name,
        status: 'failed' as const,
        error: errorMsg,
      }));
      setBatchResults(results);
      setBatchProgress({
        current: pendingImportPackages.length,
        total: pendingImportPackages.length,
        success: 0,
        failed: pendingImportPackages.length,
      });
    }

    setBatchImporting(false);
    setSelectedRowKeys([]);
    setPendingImportPackages([]);
    setBatchPreviewData(new Map());
    setBatchConflictTotal(0);

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

  // 渲染资产折叠面板标题（显示数量和冲突数）
  const renderAssetCollapseLabel = (type: string, assets: Array<{ name: string; exists: boolean }>) => {
    const count = assets?.length || 0;
    const conflictCount = assets?.filter(a => a.exists)?.length || 0;
    return (
      <span>
        {type} ({count})
        {conflictCount > 0 && (
          <Tag color="warning" style={{ marginLeft: 8 }}>{conflictCount} 个已存在</Tag>
        )}
      </span>
    );
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
        title: '状态',  // 新增：状态列
        dataIndex: 'exists',
        key: 'exists',
        width: 80,
        render: (exists: boolean) => (
          <Tag color={exists ? 'warning' : 'success'}>
            {exists ? '已存在' : '新增'}
          </Tag>
        ),
      },
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
      {
        title: '状态',  // 新增：状态列
        dataIndex: 'exists',
        key: 'exists',
        width: 80,
        render: (exists: boolean) => (
          <Tag color={exists ? 'warning' : 'success'}>
            {exists ? '已存在' : '新增'}
          </Tag>
        ),
      },
    ];

    return (
      <Modal
        title={`导入预览 - ${previewData.packageName}`}
        open={previewModalVisible}
        onCancel={() => setPreviewModalVisible(false)}
        width={700}
        styles={{ body: { maxHeight: '60vh', overflow: 'auto' } }}
        footer={previewData.conflictCount === 0 ? [
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
              if (pkg) handleSync(pkg, 'overwrite');
            }}
          >
            确认导入
          </Button>,
        ] : [
          <Button key="cancel" onClick={() => setPreviewModalVisible(false)}>
            取消
          </Button>,
          <Popconfirm
            key="overwrite"
            title="确定要覆盖所有冲突项吗？"
            onConfirm={() => {
              const pkg = packages.find(p => p.name === previewData.packageName);
              if (pkg) handleSync(pkg, 'overwrite');
            }}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="primary"
              icon={<CloudDownloadOutlined />}
              loading={syncingPackage === previewData.packageName}
            >
              全部覆盖
            </Button>
          </Popconfirm>,
          <Button
            key="skip"
            icon={<CloudDownloadOutlined />}
            loading={syncingPackage === previewData.packageName}
            onClick={() => {
              const pkg = packages.find(p => p.name === previewData.packageName);
              if (pkg) handleSync(pkg, 'skip');
            }}
          >
            全部跳过
          </Button>,
        ]}
      >
        {/* 冲突提示 Alert */}
        {previewData.conflictCount > 0 && (
          <Alert
            type="warning"
            icon={<WarningOutlined />}
            message={`检测到 ${previewData.conflictCount} 个冲突项`}
            description={SKIP_RULE_DESCRIPTION}
            showIcon
            style={{ marginBottom: 16 }}
          />
        )}

        <Descriptions bordered column={2} size="small">
          <Descriptions.Item label="团队包名称">{previewData.packageName}</Descriptions.Item>
          <Descriptions.Item label="版本">{previewData.version}</Descriptions.Item>
          <Descriptions.Item label="描述" span={2}>{previewData.description}</Descriptions.Item>
        </Descriptions>

        <Divider style={{ margin: '12px 0' }} />

        <Title level={5} style={{ marginBottom: 8 }}>
          团队信息
          {previewData.workflow.exists && <Tag color="warning" style={{ marginLeft: 8 }}>已存在</Tag>}
        </Title>
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="团队名称">{previewData.workflow.name}</Descriptions.Item>
          <Descriptions.Item label="团队描述">{previewData.workflow.description}</Descriptions.Item>
        </Descriptions>

        <Divider style={{ margin: '12px 0' }} />

        <Title level={5} style={{ marginBottom: 8 }}>
          角色 Agent ({previewData.roles.length} 个)
          {previewData.roles.some(r => r.exists) && (
            <Tag color="warning" style={{ marginLeft: 8 }}>
              {previewData.roles.filter(r => r.exists).length} 个已存在
            </Tag>
          )}
        </Title>
        <Table
          dataSource={previewData.roles}
          columns={roleColumns}
          rowKey="name"
          pagination={false}
          size="small"
        />

        <Divider style={{ margin: '12px 0' }} />

        <Collapse
          items={[
            {
              key: 'skills',
              label: renderAssetCollapseLabel('Skills', previewData.assets.skills),
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
              label: renderAssetCollapseLabel('Commands', previewData.assets.commands),
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
              label: renderAssetCollapseLabel('Subagents', previewData.assets.subagents),
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
              label: renderAssetCollapseLabel('Rules', previewData.assets.rules),
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
              label: renderAssetCollapseLabel('Settings', previewData.assets.settings),
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
            <Button
              icon={<ReloadOutlined />}
              loading={refreshing}
              onClick={handleRefresh}
            >
              刷新
            </Button>
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
              percent={batchProgress.total > 0 ? Math.round(batchProgress.current / batchProgress.total * 100) : 0}
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

      {/* 批量导入确认弹框（改为显示冲突汇总） */}
      <Modal
        title="批量导入确认"
        open={confirmModalVisible}
        onCancel={() => {
          setConfirmModalVisible(false);
          setBatchPreviewData(new Map());
          setBatchConflictTotal(0);
        }}
        footer={null}  // 自定义 footer
        width={700}
      >
        <Spin spinning={loadingBatchPreview}>
          <div style={{ marginBottom: 16 }}>
            <Text>将导入以下 {pendingImportPackages.length} 个团队包：</Text>
            {batchConflictTotal > 0 && (
              <Alert
                type="warning"
                message={`共检测到 ${batchConflictTotal} 个冲突项`}
                description={SKIP_RULE_DESCRIPTION}
                showIcon
                style={{ marginTop: 12 }}
              />
            )}
          </div>

          {/* 包列表和冲突明细 */}
          <Collapse
            style={{ marginBottom: 16 }}
            items={pendingImportPackages.map((pkg, idx) => {
              const preview = batchPreviewData.get(pkg.name);
              const hasError = preview?.previewFailed === true;

              // 构建冲突明细内容
              let conflictDetails: React.ReactNode = null;
              if (preview && !hasError && preview.conflictCount > 0) {
                const conflicts: string[] = [];
                if (preview.workflow?.exists) conflicts.push('Team');
                (preview.roles || []).forEach(r => { if (r.exists) conflicts.push(`Role: ${r.name}`); });
                (preview.assets?.skills || []).forEach(s => { if (s.exists) conflicts.push(`Skill: ${s.name}`); });
                (preview.assets?.commands || []).forEach(c => { if (c.exists) conflicts.push(`Command: ${c.name}`); });
                (preview.assets?.subagents || []).forEach(s => { if (s.exists) conflicts.push(`Subagent: ${s.name}`); });
                (preview.assets?.rules || []).forEach(r => { if (r.exists) conflicts.push(`Rule: ${r.name}`); });
                (preview.assets?.settings || []).forEach(s => { if (s.exists) conflicts.push(`Settings: ${s.name}`); });

                conflictDetails = (
                  <div style={{ marginTop: 8 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      冲突明细（{conflicts.length} 个）：
                    </Text>
                    <div style={{ marginTop: 4 }}>
                      {conflicts.map((c, i) => (
                        <Tag key={i} color="warning" style={{ marginBottom: 4 }}>
                          {c}
                        </Tag>
                      ))}
                    </div>
                  </div>
                );
              }

              return {
                key: idx.toString(),
                label: (
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <Text strong>{pkg.name}</Text>
                    <Tag color="blue">{pkg.version}</Tag>
                    <Tag color={pkg.localStatus === 'new' ? 'blue' : pkg.localStatus === 'update' ? 'orange' : 'green'}>
                      {pkg.localStatus === 'new' ? '未导入' : pkg.localStatus === 'update' ? '待更新' : '已导入'}
                    </Tag>
                    {preview && !hasError && preview.conflictCount > 0 && (
                      <Tag color="warning">{preview.conflictCount} 个冲突</Tag>
                    )}
                    {hasError && (
                      <Tag color="red">预览失败</Tag>
                    )}
                  </div>
                ),
                children: hasError ? (
                  <Text type="danger">预览失败，导入时可能报错</Text>
                ) : preview ? (
                  <div>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      Team: {preview.workflow.name} ({preview.workflow.exists ? '已存在' : '新增'})
                      {' | '} Roles: {preview.roles.length} 个
                      {' | '} Skills: {preview.assets.skills.length} 个
                      {' | '} Commands: {preview.assets.commands.length} 个
                      {' | '} Subagents: {preview.assets.subagents.length} 个
                      {' | '} Rules: {preview.assets.rules.length} 个
                      {' | '} Settings: {preview.assets.settings.length} 个
                    </Text>
                    {conflictDetails}
                  </div>
                ) : (
                  <Text type="secondary">加载中...</Text>
                ),
              };
            })}
          />

          {/* 处理方式按钮 */}
          <div style={{ textAlign: 'right', borderTop: '1px solid var(--border-color)', paddingTop: 16 }}>
            <Space>
              <Button onClick={() => {
                setConfirmModalVisible(false);
                setBatchPreviewData(new Map());
                setBatchConflictTotal(0);
              }}>
                取消
              </Button>
              {batchConflictTotal === 0 ? (
                <Button
                  type="primary"
                  icon={<CloudDownloadOutlined />}
                  onClick={() => handleBatchImportConfirm('overwrite')}
                >
                  确认导入
                </Button>
              ) : (
                <>
                  <Popconfirm
                    title="确定要覆盖所有冲突项吗？"
                    onConfirm={() => handleBatchImportConfirm('overwrite')}
                    okText="确定"
                    cancelText="取消"
                  >
                    <Button type="primary" icon={<CloudDownloadOutlined />}>
                      全部覆盖
                    </Button>
                  </Popconfirm>
                  <Button
                    icon={<CloudDownloadOutlined />}
                    onClick={() => handleBatchImportConfirm('skip')}
                  >
                    全部跳过
                  </Button>
                </>
              )}
            </Space>
          </div>
        </Spin>
      </Modal>

      {renderPreviewModal()}
    </div>
  );
};

export default TeamPackages;