import React, { useState, useEffect } from 'react';
import {
  Card, Table, Button, Space, Tag, message, Spin, Modal,
  Descriptions, Collapse, Typography, Divider,
  Alert, Popconfirm, Upload  // 用于冲突提示和确认、手动导入上传
} from 'antd';
import type { UploadProps } from 'antd';
import {
  CloudDownloadOutlined, ShopOutlined, CheckSquareOutlined,
  ReloadOutlined,  // 新增：用于刷新按钮
  WarningOutlined,  // 新增：用于冲突提示图标
  CloudUploadOutlined, FileZipOutlined  // 新增：用于手动导入
} from '@ant-design/icons';
import api from '@/api/client';
import type { MarketPackage, PackagePreviewResponse, ImportConfirm, ImportResult, ConfigGenResult } from '@/types';
import { getCachedPackages, setCachedPackages, clearCache } from '@/utils/teamPackageCache';

const { Title, Text } = Typography;

// 跳过规则说明文本（统一常量）
const SKIP_RULE_DESCRIPTION = "跳过规则：按资产粒度处理。选择「全部跳过」将保留已存在的 Workflow、Roles 和 Assets，仅导入新增项。";

// 资产类型标签颜色
const typeColors: Record<string, string> = {
  workflow: 'magenta',
  role: 'geekblue',
  skill: 'blue',
  command: 'green',
  subagent: 'purple',
  rule: 'orange',
  settings: 'cyan',
};

const TeamPackages: React.FC = () => {
  const [packages, setPackages] = useState<MarketPackage[]>([]);
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);  // 新增：刷新状态
  const [syncingPackage, setSyncingPackage] = useState<string | null>(null);
  const [previewingPackage, setPreviewingPackage] = useState<string | null>(null);
  const [previewData, setPreviewData] = useState<PackagePreviewResponse | null>(null);
  const [previewModalVisible, setPreviewModalVisible] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [batchProgress, setBatchProgress] = useState({ current: 0, total: 0, success: 0, failed: 0 });
  const [batchResults, setBatchResults] = useState<Array<{ name: string; status: 'success' | 'failed'; error?: string }>>([]);
  const [batchModalVisible, setBatchModalVisible] = useState(false);
  const [confirmModalVisible, setConfirmModalVisible] = useState(false);
  const [pendingImportPackages, setPendingImportPackages] = useState<MarketPackage[]>([]);
  // 批量导入预览状态
  const [batchPreviewData, setBatchPreviewData] = useState<Map<string, PackagePreviewResponse>>(new Map());
  const [loadingBatchPreview, setLoadingBatchPreview] = useState(false);
  const [batchConflictTotal, setBatchConflictTotal] = useState(0);
  // 批量导入确认按钮loading状态
  const [confirmingBatch, setConfirmingBatch] = useState(false);
  // 导入结果状态
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [importResultModalVisible, setImportResultModalVisible] = useState(false);

  // 手动导入状态
  const [manualImportModalVisible, setManualImportModalVisible] = useState(false);
  const [manualImportFile, setManualImportFile] = useState<File | null>(null);
  const [manualImportPreview, setManualImportPreview] = useState<TeamPackagePreview | null>(null);
  const [loadingManualPreview, setLoadingManualPreview] = useState(false);
  const [importingManual, setImportingManual] = useState(false);
  const [manualImportResult, setManualImportResult] = useState<ImportResult | null>(null);

  // TeamPackagePreview 类型定义（用于手动导入）
  interface TeamPackagePreview {
    workflow: { name: string; exists: boolean };
    roles: Array<{ name: string; exists: boolean; localId?: string }>;
    assets: {
      skills: Array<{ name: string; exists: boolean }>;
      commands: Array<{ name: string; exists: boolean }>;
      subagents: Array<{ name: string; exists: boolean }>;
      rules: Array<{ name: string; exists: boolean }>;
      settings: Array<{ name: string; exists: boolean }>;
    };
  }

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
      // 使用简化提示
      message.error(error.message || '加载团队包列表失败');
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

  // ========== 手动导入团队包功能 ==========
  const { Dragger } = Upload;

  // 处理文件上传预览
  const handleManualUploadPreview = async (file: File) => {
    setManualImportFile(file);
    setLoadingManualPreview(true);
    setManualImportPreview(null);
    setManualImportResult(null);
    try {
      const result = await api.teamPackages.import(file);
      setManualImportPreview(result);
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || error.message || '解析团队包失败';
      message.error(errorMsg);
      setManualImportFile(null);
    } finally {
      setLoadingManualPreview(false);
    }
    return false; // 阻止默认上传行为
  };

  // 上传配置
  const manualUploadProps: UploadProps = {
    accept: '.zip',
    showUploadList: false,
    beforeUpload: handleManualUploadPreview,
    disabled: loadingManualPreview || importingManual,
    capture: false,
  };

  // 生成手动导入确认参数
  const buildManualImportConfirm = (mode: 'overwrite' | 'skip'): ImportConfirm => {
    if (!manualImportPreview) return { mode, workflowAction: mode, roleActions: [], assetActions: [] };

    const roleActions = manualImportPreview.roles.map(r => ({ name: r.name, action: mode }));
    const assetActions: Array<{ assetType: string; name: string; action: 'overwrite' | 'skip' }> = [];

    // 收集所有资产
    Object.entries(manualImportPreview.assets).forEach(([assetType, items]) => {
      items.forEach(item => {
        assetActions.push({ assetType, name: item.name, action: mode });
      });
    });

    return {
      mode,
      workflowAction: manualImportPreview.workflow.exists ? mode : 'overwrite',
      roleActions,
      assetActions,
    };
  };

  // 执行确认手动导入
  const handleManualImportConfirm = async (mode: 'overwrite' | 'skip') => {
    if (!manualImportFile || !manualImportPreview) return;

    setImportingManual(true);
    try {
      const confirm = buildManualImportConfirm(mode);
      const result = await api.teamPackages.importConfirm(manualImportFile, confirm);
      setManualImportResult(result);

      // 统计导入结果
      const countByType = (type: string) =>
        result.details?.filter((d: any) => d.assetType === type && d.status === 'success').length || 0;
      const skillsCount = countByType('skill');
      const commandsCount = countByType('command');
      const subagentsCount = countByType('subagent');
      const rulesCount = countByType('rule');
      const settingsCount = countByType('settings');
      const rolesCount = countByType('role');
      const workflowName = result.details?.find((d: any) => d.assetType === 'workflow')?.name || '';

      // 统计配置生成结果
      const configGenCount = result.configGenResults?.length || 0;
      const configGenSuccess = result.configGenResults?.filter((c: any) => c.status === 'success').length || 0;

      let successMsg = `导入成功：团队 ${workflowName}，角色 ${rolesCount} 个，Skills ${skillsCount}、Commands ${commandsCount}、Subagents ${subagentsCount}、Rules ${rulesCount}、Settings ${settingsCount}`;
      if (configGenCount > 0) {
        successMsg += `，自动更新 ${configGenSuccess}/${configGenCount} 个角色配置`;
      }
      message.success(successMsg);
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || error.message || '导入失败';
      message.error(errorMsg);
    } finally {
      setImportingManual(false);
    }
  };

  // 计算手动导入冲突数量
  const getManualConflictCount = () => {
    if (!manualImportPreview) return 0;
    let count = 0;
    if (manualImportPreview.workflow.exists) count++;
    count += manualImportPreview.roles.filter(r => r.exists).length;
    count += manualImportPreview.assets.skills.filter(s => s.exists).length;
    count += manualImportPreview.assets.commands.filter(c => c.exists).length;
    count += manualImportPreview.assets.subagents.filter(s => s.exists).length;
    count += manualImportPreview.assets.rules.filter(r => r.exists).length;
    count += manualImportPreview.assets.settings.filter(s => s.exists).length;
    return count;
  };

  // 清除手动导入预览
  const handleClearManualPreview = () => {
    setManualImportFile(null);
    setManualImportPreview(null);
    setManualImportResult(null);
  };

  // 关闭手动导入弹窗
  const handleCloseManualImportModal = () => {
    setManualImportModalVisible(false);
    handleClearManualPreview();
  };

  // 渲染手动导入预览表格
  const renderManualPreviewTable = () => {
    if (!manualImportPreview) return null;

    const dataSource: any[] = [];

    // 工作流
    dataSource.push({
      key: 'workflow',
      type: 'Team',
      name: manualImportPreview.workflow.name,
      exists: manualImportPreview.workflow.exists,
      action: manualImportPreview.workflow.exists ? '待处理' : '新增',
    });

    // 角色
    manualImportPreview.roles.forEach((role, idx) => {
      dataSource.push({
        key: `role-${idx}`,
        type: 'Role',
        name: role.name,
        exists: role.exists,
        action: role.exists ? '待处理' : '新增',
      });
    });

    // 技能
    manualImportPreview.assets.skills.forEach((skill, idx) => {
      dataSource.push({
        key: `skill-${idx}`,
        type: 'Skill',
        name: skill.name,
        exists: skill.exists,
        action: skill.exists ? '待处理' : '新增',
      });
    });

    // 命令
    manualImportPreview.assets.commands.forEach((cmd, idx) => {
      dataSource.push({
        key: `command-${idx}`,
        type: 'Command',
        name: cmd.name,
        exists: cmd.exists,
        action: cmd.exists ? '待处理' : '新增',
      });
    });

    // 子代理
    manualImportPreview.assets.subagents.forEach((sub, idx) => {
      dataSource.push({
        key: `subagent-${idx}`,
        type: 'Subagent',
        name: sub.name,
        exists: sub.exists,
        action: sub.exists ? '待处理' : '新增',
      });
    });

    // 规则
    manualImportPreview.assets.rules.forEach((rule, idx) => {
      dataSource.push({
        key: `rule-${idx}`,
        type: 'Rule',
        name: rule.name,
        exists: rule.exists,
        action: rule.exists ? '待处理' : '新增',
      });
    });

    // 配置
    manualImportPreview.assets.settings.forEach((setting, idx) => {
      dataSource.push({
        key: `setting-${idx}`,
        type: 'Settings',
        name: setting.name,
        exists: setting.exists,
        action: setting.exists ? '待处理' : '新增',
      });
    });

    const columns = [
      {
        title: '类型',
        dataIndex: 'type',
        key: 'type',
        width: 100,
        render: (type: string) => <Tag color={typeColors[type.toLowerCase()] || 'default'}>{type}</Tag>,
      },
      {
        title: '名称',
        dataIndex: 'name',
        key: 'name',
      },
      {
        title: '状态',
        dataIndex: 'exists',
        key: 'exists',
        width: 100,
        render: (exists: boolean) => (
          <Tag color={exists ? 'warning' : 'success'}>
            {exists ? '已存在' : '新增'}
          </Tag>
        ),
      },
      {
        title: '操作',
        dataIndex: 'action',
        key: 'action',
        width: 100,
        render: (action: string) => (
          <Text type={action === '待处理' ? 'warning' : 'secondary'}>
            {action}
          </Text>
        ),
      },
    ];

    return (
      <Table
        dataSource={dataSource}
        columns={columns}
        pagination={false}
        size="small"
        scroll={{ y: 300 }}
      />
    );
  };

  // ========== 手动导入团队包功能结束 ==========

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
      message.error(error.message || '预览失败');
    } finally {
      setPreviewingPackage(null);
    }
  };

  const handleSync = async (pkg: MarketPackage, mode: 'overwrite' | 'skip') => {
    setSyncingPackage(pkg.name);
    try {
      const confirm = buildImportConfirm(previewData, mode);

      const result = await api.teamPackages.syncPackage(pkg.name, confirm, pkg.marketId);
      setImportResult(result);
      setImportResultModalVisible(true);

      // 统计导入结果
      const rolesCount = result.details?.filter((d: any) => d.assetType === 'role' && d.status === 'success').length || 0;
      const configGenCount = result.configGenResults?.length || 0;
      const configGenSuccess = result.configGenResults?.filter((c: any) => c.status === 'success').length || 0;

      let successMsg = `团队包 ${pkg.name} 导入成功`;
      if (rolesCount > 0) {
        successMsg += `，导入 ${rolesCount} 个角色`;
      }
      if (configGenCount > 0) {
        successMsg += `，自动更新 ${configGenSuccess}/${configGenCount} 个角色配置`;
      }
      message.success(successMsg);

      loadPackages(true);  // 强制刷新以更新最近导入时间
      setPreviewModalVisible(false);
    } catch (error: any) {
      message.error(error.message || '导入失败');
    } finally {
      setSyncingPackage(null);
    }
  };

  // 点击批量导入按钮 -> 立即显示确认弹框（预览loading）
  const handleBatchImportClick = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning('请先选择要导入的团队包');
      return;
    }

    const toImport = packages.filter(pkg =>
      selectedRowKeys.includes(`${pkg.marketId}-${pkg.name}`)
    );
    setPendingImportPackages(toImport);
    setBatchPreviewData(new Map());  // 清空预览数据
    setBatchConflictTotal(0);
    setLoadingBatchPreview(true);
    setConfirmModalVisible(true);  // 立即显示弹框，用户看到loading反馈

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
      message.error(error.message || '批量预览失败');
      setConfirmModalVisible(false);  // 预览失败关闭弹框
      setPendingImportPackages([]);
    } finally {
      setLoadingBatchPreview(false);
    }
  };

  // 导入进度状态（用于逐项导入实时进度展示）
  const [importProgressVisible, setImportProgressVisible] = useState(false);

  // 确认后执行批量导入（逐项执行以展示实时进度）
  const handleBatchImportConfirm = async (mode: 'overwrite' | 'skip') => {
    // 进度弹框独立显示，不再关闭确认弹框（由调用方处理）
    setBatchProgress({ current: 0, total: pendingImportPackages.length, success: 0, failed: 0 });
    setBatchResults([]);
    setImportProgressVisible(true);

    const results: Array<{ name: string; status: 'success' | 'failed'; error?: string }> = [];
    let successCount = 0;
    let failedCount = 0;

    try {
      // 逐项导入以展示实时进度
      for (let i = 0; i < pendingImportPackages.length; i++) {
        const pkg = pendingImportPackages[i];
        const preview = batchPreviewData.get(pkg.name);
        const confirm = buildImportConfirm(preview, mode);

        try {
          await api.teamPackages.syncPackage(pkg.name, confirm, pkg.marketId);
          results.push({ name: pkg.name, status: 'success' });
          successCount++;
        } catch (error: any) {
          // 使用简化提示
          results.push({ name: pkg.name, status: 'failed', error: error.message || '导入失败' });
          failedCount++;
        }

        // 更新进度
        setBatchResults([...results]);
        setBatchProgress({ current: i + 1, total: pendingImportPackages.length, success: successCount, failed: failedCount });
      }
    } catch (error: any) {
      // 不应该到达这里，但作为兜底
      message.error(error.message || '批量导入异常');
    }

    // 导入完成，进度弹框变为结果弹框
    setImportProgressVisible(false);
    setBatchModalVisible(true);
    setConfirmingBatch(false);
    setSelectedRowKeys([]);
    setPendingImportPackages([]);
    setBatchPreviewData(new Map());
    setBatchConflictTotal(0);
    loadPackages(true);  // 强制刷新以更新最近导入时间
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
              icon={<CloudUploadOutlined />}
              onClick={() => setManualImportModalVisible(true)}
            >
              手动导入
            </Button>
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
              disabled={selectedRowKeys.length === 0}
              loading={confirmingBatch || loadingBatchPreview}
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

      {/* 批量导入进度弹框 */}
      <Modal
        title="导入进度"
        open={importProgressVisible}
        footer={null}
        closable={false}
        width={500}
      >
        <div>
          <div style={{ marginBottom: 12 }}>
            <Text strong>正在导入...</Text>
            <Text type="secondary" style={{ marginLeft: 12 }}>
              {batchProgress.current} / {batchProgress.total}
            </Text>
          </div>
          <div style={{ marginBottom: 8 }}>
            <Text type="secondary">成功: {batchProgress.success} 个 | 失败: {batchProgress.failed} 个</Text>
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
      </Modal>

      {/* 批量导入结果弹框 */}
      <Modal
        title="批量导入结果"
        open={batchModalVisible}
        onCancel={() => {
          setBatchModalVisible(false);
          setBatchResults([]);
        }}
        footer={[
          <Button key="close" onClick={() => {
            setBatchModalVisible(false);
            setBatchResults([]);
          }}>
            关闭
          </Button>,
        ]}
        width={500}
      >
        <div>
          <div style={{ marginBottom: 12 }}>
            <Text strong>导入完成</Text>
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
              }} disabled={confirmingBatch}>
                取消
              </Button>
              {batchConflictTotal === 0 ? (
                <Button
                  type="primary"
                  icon={<CloudDownloadOutlined />}
                  loading={confirmingBatch}
                  onClick={() => handleBatchImportConfirm('overwrite')}
                >
                  确认导入
                </Button>
              ) : (
                <>
                  <Popconfirm
                    title="确定要覆盖所有冲突项吗？"
                    description="此操作将覆盖已存在的 Team、Roles 和 Assets。"
                    onConfirm={() => {
                      setConfirmModalVisible(false);  // 先关闭确认弹框
                      handleBatchImportConfirm('overwrite');
                    }}
                    okText="确定覆盖"
                    cancelText="取消"
                  >
                    <Button
                      type="primary"
                      icon={<CloudDownloadOutlined />}
                    >
                      全部覆盖
                    </Button>
                  </Popconfirm>
                  <Button
                    icon={<CloudDownloadOutlined />}
                    onClick={() => {
                      setConfirmModalVisible(false);  // 先关闭确认弹框
                      handleBatchImportConfirm('skip');
                    }}
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

      {/* 导入结果弹窗 */}
      <Modal
        title="导入结果"
        open={importResultModalVisible}
        onCancel={() => setImportResultModalVisible(false)}
        footer={<Button onClick={() => setImportResultModalVisible(false)}>关闭</Button>}
        width={700}
      >
        {importResult && (
          <div>
            <Alert
              type={importResult.failed > 0 ? 'warning' : 'success'}
              message={`导入完成：成功 ${importResult.success}，跳过 ${importResult.skipped}，失败 ${importResult.failed}`}
              showIcon
              style={{ marginBottom: 16 }}
            />
            {importResult.details && importResult.details.length > 0 && (
              <Table
                dataSource={importResult.details}
                rowKey={(item: any, index: number) => `${item.assetType}-${item.name}-${index}`}
                pagination={false}
                size="small"
                scroll={{ y: 300 }}
                columns={[
                  {
                    title: '类型',
                    dataIndex: 'assetType',
                    key: 'assetType',
                    width: 100,
                    render: (type: string) => (
                      <Tag color={typeColors[type] || 'default'}>{type}</Tag>
                    ),
                  },
                  { title: '名称', dataIndex: 'name', key: 'name' },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    key: 'status',
                    width: 80,
                    render: (status: string) => (
                      <Tag color={status === 'success' ? 'success' : status === 'skipped' ? 'warning' : 'error'}>
                        {status}
                      </Tag>
                    ),
                  },
                  { title: '信息', dataIndex: 'message', key: 'message', ellipsis: true },
                ]}
              />
            )}
            {/* 配置生成结果 */}
            {importResult.configGenResults && importResult.configGenResults.length > 0 && (
              <div style={{ marginTop: 16 }}>
                <Alert
                  type="info"
                  message={`自动更新了 ${importResult.configGenResults.length} 个角色的配置`}
                  showIcon
                  style={{ marginBottom: 8 }}
                />
                <Table
                  dataSource={importResult.configGenResults}
                  rowKey={(item: ConfigGenResult) => item.agentId}
                  pagination={false}
                  size="small"
                  scroll={{ y: 200 }}
                  columns={[
                    { title: '角色名称', dataIndex: 'agentName', key: 'agentName' },
                    {
                      title: '状态',
                      dataIndex: 'status',
                      key: 'status',
                      width: 80,
                      render: (status: string) => (
                        <Tag color={status === 'success' ? 'success' : 'error'}>
                          {status === 'success' ? '成功' : '失败'}
                        </Tag>
                      ),
                    },
                    { title: '信息', dataIndex: 'message', key: 'message', ellipsis: true },
                  ]}
                />
              </div>
            )}
          </div>
        )}
      </Modal>

      {/* 手动导入团队包弹窗 */}
      <Modal
        title="手动导入团队包"
        open={manualImportModalVisible}
        onCancel={handleCloseManualImportModal}
        footer={null}
        width={600}
        destroyOnClose
      >
        <Spin spinning={loadingManualPreview || importingManual}>
          {!manualImportPreview && !manualImportResult ? (
            <Dragger {...manualUploadProps}>
              <p className="ant-upload-drag-icon">
                <FileZipOutlined style={{ fontSize: 48, color: '#1890ff' }} />
              </p>
              <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
              <p className="ant-upload-hint">支持 .zip 格式的团队包文件</p>
            </Dragger>
          ) : manualImportResult ? (
            <div>
              <Alert
                type={manualImportResult.failed > 0 ? 'warning' : 'success'}
                message={`导入完成：成功 ${manualImportResult.success}，跳过 ${manualImportResult.skipped}，失败 ${manualImportResult.failed}`}
                showIcon
                style={{ marginBottom: 16 }}
              />
              {manualImportResult.details && manualImportResult.details.length > 0 && (
                <Table
                  dataSource={manualImportResult.details}
                  rowKey={(item: any, index: number) => `${item.assetType}-${item.name}-${index}`}
                  pagination={false}
                  size="small"
                  scroll={{ y: 300 }}
                  columns={[
                    {
                      title: '类型',
                      dataIndex: 'assetType',
                      key: 'assetType',
                      width: 100,
                      render: (type: string) => (
                        <Tag color={typeColors[type] || 'default'}>{type}</Tag>
                      ),
                    },
                    { title: '名称', dataIndex: 'name', key: 'name' },
                    {
                      title: '状态',
                      dataIndex: 'status',
                      key: 'status',
                      width: 80,
                      render: (status: string) => (
                        <Tag color={status === 'success' ? 'success' : status === 'skipped' ? 'warning' : 'error'}>
                          {status}
                        </Tag>
                      ),
                    },
                    { title: '信息', dataIndex: 'message', key: 'message', ellipsis: true },
                  ]}
                />
              )}
              {/* 配置生成结果 */}
              {manualImportResult.configGenResults && manualImportResult.configGenResults.length > 0 && (
                <div style={{ marginTop: 16 }}>
                  <Alert
                    type="info"
                    message={`自动更新了 ${manualImportResult.configGenResults.length} 个角色的配置`}
                    showIcon
                    style={{ marginBottom: 8 }}
                  />
                  <Table
                    dataSource={manualImportResult.configGenResults}
                    rowKey={(item: ConfigGenResult) => item.agentId}
                    pagination={false}
                    size="small"
                    scroll={{ y: 200 }}
                    columns={[
                      { title: '角色名称', dataIndex: 'agentName', key: 'agentName' },
                      {
                        title: '状态',
                        dataIndex: 'status',
                        key: 'status',
                        width: 80,
                        render: (status: string) => (
                          <Tag color={status === 'success' ? 'success' : 'error'}>
                            {status === 'success' ? '成功' : '失败'}
                          </Tag>
                        ),
                      },
                      { title: '信息', dataIndex: 'message', key: 'message', ellipsis: true },
                    ]}
                  />
                </div>
              )}
              <div style={{ marginTop: 16, textAlign: 'right' }}>
                <Button onClick={handleCloseManualImportModal}>关闭</Button>
              </div>
            </div>
          ) : (
            <div>
              {/* 冲突提示 */}
              {getManualConflictCount() > 0 && (
                <Alert
                  type="warning"
                  icon={<WarningOutlined />}
                  message={`检测到 ${getManualConflictCount()} 个冲突项，请选择处理方式`}
                  showIcon
                  style={{ marginBottom: 16 }}
                />
              )}

              {/* 预览表格 */}
              {renderManualPreviewTable()}

              {/* 操作按钮 */}
              <div style={{ marginTop: 16, textAlign: 'right' }}>
                <Space>
                  {getManualConflictCount() === 0 ? (
                    <Button
                      type="primary"
                      icon={<CloudUploadOutlined />}
                      loading={importingManual}
                      onClick={() => handleManualImportConfirm('overwrite')}
                    >
                      确认导入
                    </Button>
                  ) : (
                    <>
                      <Popconfirm
                        title="确定要覆盖所有冲突项吗？"
                        onConfirm={() => handleManualImportConfirm('overwrite')}
                        okText="确定"
                        cancelText="取消"
                      >
                        <Button
                          type="primary"
                          icon={<CloudUploadOutlined />}
                          loading={importingManual}
                        >
                          全部覆盖
                        </Button>
                      </Popconfirm>
                      <Button
                        icon={<CloudUploadOutlined />}
                        loading={importingManual}
                        onClick={() => handleManualImportConfirm('skip')}
                      >
                        全部跳过
                      </Button>
                    </>
                  )}
                </Space>
              </div>
            </div>
          )}
        </Spin>
      </Modal>
    </div>
  );
};

export default TeamPackages;