import React, { useState, useEffect } from 'react';
import {
  Card, Button, Upload, Table, Tag, Space, Typography, message,
  Divider, Alert, Spin, Empty, Form, Input, Transfer, Popconfirm
} from 'antd';
import {
  CloudDownloadOutlined,
  CloudUploadOutlined,
  FileZipOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import type { UploadProps } from 'antd';
import api from '@/api/client';
import type {
  Skill,
  Command,
  Subagent,
  Rule,
  Settings,
  ImportResult,
  ImportDetail,
} from '@/types';

const { Title, Text } = Typography;
const { Dragger } = Upload;

// 资产类型标签颜色
const assetTypeColors: Record<string, string> = {
  skill: 'blue',
  command: 'green',
  subagent: 'purple',
  rule: 'orange',
  settings: 'cyan',
};

// 资产类型选项
const assetTypeOptions = [
  { key: 'skills', title: 'Skills' },
  { key: 'commands', title: 'Commands' },
  { key: 'subagents', title: 'Subagents' },
  { key: 'rules', title: 'Rules' },
  { key: 'settings', title: 'Settings' },
];

// AssetPackagePreview 类型定义
interface AssetPackagePreviewAsset {
  name: string;
  exists: boolean;
}

interface AssetPackagePreviewAssets {
  skills: AssetPackagePreviewAsset[];
  commands: AssetPackagePreviewAsset[];
  subagents: AssetPackagePreviewAsset[];
  rules: AssetPackagePreviewAsset[];
  settings: AssetPackagePreviewAsset[];
}

interface AssetPackagePreview {
  assets: AssetPackagePreviewAssets;
}

const AssetPackageManagement: React.FC = () => {
  // 导入状态
  const [importFile, setImportFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<AssetPackagePreview | null>(null);
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);

  // 导出状态
  const [exporting, setExporting] = useState(false);
  const [exportForm] = Form.useForm();

  // 资产选择状态
  const [skills, setSkills] = useState<Skill[]>([]);
  const [commands, setCommands] = useState<Command[]>([]);
  const [subagents, setSubagents] = useState<Subagent[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [settings, setSettings] = useState<Settings[]>([]);
  const [selectedAssetType, setSelectedAssetType] = useState<string>('skills');
  const [loadingAssets, setLoadingAssets] = useState(false);

  // 选中的资产ID
  const [selectedSkillIds, setSelectedSkillIds] = useState<string[]>([]);
  const [selectedCommandIds, setSelectedCommandIds] = useState<string[]>([]);
  const [selectedSubagentIds, setSelectedSubagentIds] = useState<string[]>([]);
  const [selectedRuleIds, setSelectedRuleIds] = useState<string[]>([]);
  const [selectedSettingsIds, setSelectedSettingsIds] = useState<string[]>([]);

  // 加载各类资产列表（用于导出选择）
  const loadAssets = async () => {
    setLoadingAssets(true);
    try {
      const [skillsRes, commandsRes, subagentsRes, rulesRes, settingsRes] = await Promise.all([
        api.skills.list({ pageSize: 100 }),
        api.commands.list({ pageSize: 100 }),
        api.subagents.list({ pageSize: 100 }),
        api.rules.list({ pageSize: 100 }),
        api.settings.list({ pageSize: 100 }),
      ]);
      setSkills(skillsRes.data || []);
      setCommands(commandsRes.data || []);
      setSubagents(subagentsRes.data || []);
      setRules(rulesRes.data || []);
      setSettings(settingsRes.data || []);
    } catch (error) {
      // 忽略错误
    } finally {
      setLoadingAssets(false);
    }
  };

  // 初始化加载资产
  useEffect(() => {
    loadAssets();
  }, []);

  // 处理文件上传预览
  const handleUploadPreview = async (file: File) => {
    setImportFile(file);
    setLoadingPreview(true);
    setPreview(null);
    setImportResult(null);
    try {
      const result = await api.assetPackages.import(file);
      setPreview(result);
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || error.message || '解析资产包失败';
      message.error(errorMsg);
      setImportFile(null);
    } finally {
      setLoadingPreview(false);
    }
    return false; // 阻止默认上传行为
  };

  // 上传配置
  const uploadProps: UploadProps = {
    accept: '.zip',
    showUploadList: false,
    beforeUpload: handleUploadPreview,
    disabled: loadingPreview || importing,
  };

  // 生成导入确认参数
  const buildImportConfirm = (mode: 'overwrite' | 'skip') => {
    if (!preview) return { mode, assetActions: [] };

    const assetActions: Array<{ assetType: string; name: string; action: string }> = [];

    // 收集所有资产
    Object.entries(preview.assets).forEach(([assetType, items]) => {
      items.forEach((item: AssetPackagePreviewAsset) => {
        assetActions.push({ assetType, name: item.name, action: mode });
      });
    });

    return {
      mode,
      assetActions,
    };
  };

  // 执行确认导入
  const handleImportConfirm = async (mode: 'overwrite' | 'skip') => {
    if (!importFile || !preview) return;

    setImporting(true);
    try {
      const confirm = buildImportConfirm(mode);
      const result = await api.assetPackages.importConfirm(importFile, confirm);
      setImportResult(result);
      // 从 details 中统计各类型的成功数量
      const countByType = (type: string) =>
        result.details?.filter((d: ImportDetail) => d.assetType === type && d.status === 'success').length || 0;
      const skillsCount = countByType('skill');
      const commandsCount = countByType('command');
      const subagentsCount = countByType('subagent');
      const rulesCount = countByType('rule');
      const settingsCount = countByType('settings');
      message.success(`导入成功：Skills ${skillsCount}、Commands ${commandsCount}、Subagents ${subagentsCount}、Rules ${rulesCount}、Settings ${settingsCount}`);
      // 清理状态
      setImportFile(null);
      setPreview(null);
      // 刷新资产列表
      loadAssets();
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || error.message || '导入失败';
      message.error(errorMsg);
    } finally {
      setImporting(false);
    }
  };

  // 清除预览
  const handleClearPreview = () => {
    setImportFile(null);
    setPreview(null);
    setImportResult(null);
  };

  // 执行导出
  const handleExport = async (values: any) => {
    setExporting(true);
    try {
      const blob = await api.assetPackages.export({
        name: values.name,
        skillIds: selectedSkillIds,
        commandIds: selectedCommandIds,
        subagentIds: selectedSubagentIds,
        ruleIds: selectedRuleIds,
        settingsIds: selectedSettingsIds,
      });

      // 创建下载链接
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${values.name}.zip`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);

      message.success('导出成功');
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || error.message || '导出失败';
      message.error(errorMsg);
    } finally {
      setExporting(false);
    }
  };

  // 获取指定资产类型的数量
  const getAssetCount = (type: string) => {
    switch (type) {
      case 'skills':
        return skills.length;
      case 'commands':
        return commands.length;
      case 'subagents':
        return subagents.length;
      case 'rules':
        return rules.length;
      case 'settings':
        return settings.length;
      default:
        return 0;
    }
  };

  // 获取当前选中资产类型的列表
  const getCurrentAssetList = () => {
    switch (selectedAssetType) {
      case 'skills':
        return skills.map(s => ({ key: s.id, title: s.name }));
      case 'commands':
        return commands.map(c => ({ key: c.id, title: c.name }));
      case 'subagents':
        return subagents.map(s => ({ key: s.id, title: s.name }));
      case 'rules':
        return rules.map(r => ({ key: r.id, title: r.name }));
      case 'settings':
        return settings.map(s => ({ key: s.id, title: s.name }));
      default:
        return [];
    }
  };

  // 获取当前选中的资产ID列表
  const getCurrentSelectedIds = () => {
    switch (selectedAssetType) {
      case 'skills':
        return selectedSkillIds;
      case 'commands':
        return selectedCommandIds;
      case 'subagents':
        return selectedSubagentIds;
      case 'rules':
        return selectedRuleIds;
      case 'settings':
        return selectedSettingsIds;
      default:
        return [];
    }
  };

  // 更新当前选中的资产ID列表
  const setCurrentSelectedIds = (ids: string[]) => {
    switch (selectedAssetType) {
      case 'skills':
        setSelectedSkillIds(ids);
        break;
      case 'commands':
        setSelectedCommandIds(ids);
        break;
      case 'subagents':
        setSelectedSubagentIds(ids);
        break;
      case 'rules':
        setSelectedRuleIds(ids);
        break;
      case 'settings':
        setSelectedSettingsIds(ids);
        break;
    }
  };

  // 渲染预览表格
  const renderPreviewTable = () => {
    if (!preview) return null;

    // 构建表格数据
    const dataSource: any[] = [];

    // 技能
    preview.assets.skills.forEach((item, idx) => {
      dataSource.push({
        key: `skill-${idx}`,
        type: 'Skill',
        name: item.name,
        exists: item.exists,
        action: item.exists ? '待处理' : '新增',
      });
    });

    // 命令
    preview.assets.commands.forEach((item, idx) => {
      dataSource.push({
        key: `command-${idx}`,
        type: 'Command',
        name: item.name,
        exists: item.exists,
        action: item.exists ? '待处理' : '新增',
      });
    });

    // 子代理
    preview.assets.subagents.forEach((item, idx) => {
      dataSource.push({
        key: `subagent-${idx}`,
        type: 'Subagent',
        name: item.name,
        exists: item.exists,
        action: item.exists ? '待处理' : '新增',
      });
    });

    // 规则
    preview.assets.rules.forEach((item, idx) => {
      dataSource.push({
        key: `rule-${idx}`,
        type: 'Rule',
        name: item.name,
        exists: item.exists,
        action: item.exists ? '待处理' : '新增',
      });
    });

    // 配置
    preview.assets.settings.forEach((item, idx) => {
      dataSource.push({
        key: `setting-${idx}`,
        type: 'Settings',
        name: item.name,
        exists: item.exists,
        action: item.exists ? '待处理' : '新增',
      });
    });

    const columns = [
      {
        title: '类型',
        dataIndex: 'type',
        key: 'type',
        width: 100,
        render: (type: string) => <Tag color={assetTypeColors[type.toLowerCase()] || 'default'}>{type}</Tag>,
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

  // 计算冲突数量
  const getConflictCount = () => {
    if (!preview) return 0;
    let count = 0;
    count += preview.assets.skills.filter(s => s.exists).length;
    count += preview.assets.commands.filter(c => c.exists).length;
    count += preview.assets.subagents.filter(s => s.exists).length;
    count += preview.assets.rules.filter(r => r.exists).length;
    count += preview.assets.settings.filter(s => s.exists).length;
    return count;
  };

  return (
    <div style={{ padding: 24 }}>
      <Title level={2}>资产包管理</Title>
      <Text type="secondary">导入导出资产包，便于团队协作和资产共享</Text>

      <Divider />

      <div style={{ display: 'flex', gap: 24 }}>
        {/* 左侧：导入区域 */}
        <Card
          title="导入资产包"
          style={{ flex: 1, minHeight: 400 }}
          extra={(preview || importResult) && (
            <Button size="small" onClick={handleClearPreview}>
              清除
            </Button>
          )}
        >
          {!preview && !importResult ? (
            <Spin spinning={loadingPreview}>
              <Dragger {...uploadProps}>
                <p className="ant-upload-drag-icon">
                  <FileZipOutlined style={{ fontSize: 48, color: '#1890ff' }} />
                </p>
                <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
                <p className="ant-upload-hint">支持 .zip 格式的资产包文件</p>
              </Dragger>
            </Spin>
          ) : importResult ? (
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
                  rowKey={(item, index) => `${item.assetType}-${item.name}-${index}`}
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
                        <Tag color={assetTypeColors[type] || 'default'}>{type}</Tag>
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
            </div>
          ) : (
            <div>
              {/* 冲突提示 */}
              {getConflictCount() > 0 && (
                <Alert
                  type="warning"
                  icon={<ExclamationCircleOutlined />}
                  message={`检测到 ${getConflictCount()} 个冲突项，请选择处理方式`}
                  showIcon
                  style={{ marginBottom: 16 }}
                />
              )}

              {/* 预览表格 */}
              {renderPreviewTable()}

              {/* 操作按钮 */}
              <div style={{ marginTop: 16, textAlign: 'right' }}>
                <Space>
                  <Popconfirm
                    title="确定要覆盖所有冲突项吗？"
                    onConfirm={() => handleImportConfirm('overwrite')}
                    okText="确定"
                    cancelText="取消"
                  >
                    <Button
                      type="primary"
                      icon={<CloudUploadOutlined />}
                      loading={importing}
                      disabled={getConflictCount() === 0}
                    >
                      全部覆盖
                    </Button>
                  </Popconfirm>
                  <Button
                    icon={<CloudUploadOutlined />}
                    loading={importing}
                    onClick={() => handleImportConfirm('skip')}
                  >
                    全部跳过
                  </Button>
                </Space>
              </div>
            </div>
          )}
        </Card>

        {/* 右侧：导出区域 */}
        <Card
          title="导出资产包"
          style={{ flex: 1, minHeight: 400 }}
        >
          <Spin spinning={exporting || loadingAssets}>
            <Form
              form={exportForm}
              layout="vertical"
              onFinish={handleExport}
            >
              <Form.Item
                name="name"
                label="包名称"
                rules={[{ required: true, message: '请输入包名称' }]}
              >
                <Input placeholder="如：my-assets" />
              </Form.Item>
            </Form>

            <Divider>选择资产</Divider>

            {/* 资产类型选择 */}
            <Space style={{ marginBottom: 12 }}>
              {assetTypeOptions.map(opt => (
                <Tag
                  key={opt.key}
                  color={selectedAssetType === opt.key ? assetTypeColors[opt.key.slice(0, -1)] : 'default'}
                  style={{ cursor: 'pointer' }}
                  onClick={() => setSelectedAssetType(opt.key)}
                >
                  {opt.title} ({getAssetCount(opt.key)})
                </Tag>
              ))}
            </Space>

            {/* 已选资产统计 */}
            <div style={{ marginBottom: 12, color: '#666' }}>
              已选：Skills {selectedSkillIds.length} / Commands {selectedCommandIds.length} /
              Subagents {selectedSubagentIds.length} / Rules {selectedRuleIds.length} /
              Settings {selectedSettingsIds.length}
            </div>

            {/* Transfer 选择器 */}
            <Transfer
              dataSource={getCurrentAssetList()}
              titles={['可选', '已选']}
              targetKeys={getCurrentSelectedIds()}
              onChange={(targetKeys) => setCurrentSelectedIds(targetKeys as string[])}
              render={(item) => item.title}
              listStyle={{ flex: 1, height: 250 }}
              style={{ width: '100%' }}
              showSearch
              filterOption={(input, option) =>
                (option.title as string).toLowerCase().includes(input.toLowerCase())
              }
            />

            {getCurrentAssetList().length === 0 && !loadingAssets && (
              <Empty description="暂无可用资产" style={{ marginTop: 24 }} />
            )}

            <div style={{ marginTop: 16, textAlign: 'center' }}>
              <Button
                type="primary"
                icon={<CloudDownloadOutlined />}
                size="large"
                onClick={() => exportForm.submit()}
                loading={exporting}
                disabled={
                  selectedSkillIds.length === 0 &&
                  selectedCommandIds.length === 0 &&
                  selectedSubagentIds.length === 0 &&
                  selectedRuleIds.length === 0 &&
                  selectedSettingsIds.length === 0
                }
              >
                导出资产包
              </Button>
            </div>
          </Spin>
        </Card>
      </div>
    </div>
  );
};

export default AssetPackageManagement;