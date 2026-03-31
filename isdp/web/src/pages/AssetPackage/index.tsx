import React, { useEffect, useState, useCallback } from 'react';
import {
  Card, Button, Modal, Form, Input, message, Space, Tag, Typography,
  Popconfirm, Empty, Spin, Table, Divider, Upload, Transfer, Descriptions,
  Pagination, Alert
} from 'antd';
import {
  CloudUploadOutlined,
  CloudDownloadOutlined,
  DeleteOutlined,
  InboxOutlined,
  FileZipOutlined,
  EyeOutlined,
} from '@ant-design/icons';
import type { UploadProps } from 'antd';
import api from '@/api/client';
import type {
  AssetPackage,
  ImportResult,
  Skill,
  Command,
  Subagent,
  Rule,
  Settings,
} from '@/types';

const { Title, Text } = Typography;

// 资产类型选项
const assetTypeOptions = [
  { key: 'skills', title: '技能' },
  { key: 'commands', title: '命令' },
  { key: 'subagents', title: '子代理' },
  { key: 'rules', title: '规则' },
  { key: 'settings', title: '配置' },
];

// 资产类型标签颜色
const assetTypeColors: Record<string, string> = {
  skills: 'blue',
  commands: 'green',
  subagents: 'purple',
  rules: 'orange',
  settings: 'cyan',
};

// 导入状态标签颜色
const statusColors: Record<string, string> = {
  success: 'success',
  skipped: 'warning',
  failed: 'error',
};

const AssetPackageManagement: React.FC = () => {
  // 资产包列表状态
  const [packages, setPackages] = useState<AssetPackage[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [searchText, setSearchText] = useState('');

  // 导入模态框状态
  const [importModalVisible, setImportModalVisible] = useState(false);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [importing, setImporting] = useState(false);

  // 导出模态框状态
  const [exportModalVisible, setExportModalVisible] = useState(false);
  const [exportForm] = Form.useForm();
  const [exporting, setExporting] = useState(false);

  // 资产选择状态
  const [skills, setSkills] = useState<Skill[]>([]);
  const [commands, setCommands] = useState<Command[]>([]);
  const [subagents, setSubagents] = useState<Subagent[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [settings, setSettings] = useState<Settings[]>([]);
  const [selectedAssetType, setSelectedAssetType] = useState<string>('skills');

  // 选中的资产ID
  const [selectedSkillIds, setSelectedSkillIds] = useState<string[]>([]);
  const [selectedCommandIds, setSelectedCommandIds] = useState<string[]>([]);
  const [selectedSubagentIds, setSelectedSubagentIds] = useState<string[]>([]);
  const [selectedRuleIds, setSelectedRuleIds] = useState<string[]>([]);
  const [selectedSettingsIds, setSelectedSettingsIds] = useState<string[]>([]);

  // 详情模态框状态
  const [detailModalVisible, setDetailModalVisible] = useState(false);
  const [selectedPackage, setSelectedPackage] = useState<AssetPackage | null>(null);

  // 加载资产包列表
  const loadPackages = useCallback(async () => {
    setLoading(true);
    try {
      const result = await api.assetPackages.list({
        page,
        pageSize,
        search: searchText,
      });
      setPackages(result.data || []);
      setTotal(result.total);
    } catch (error) {
      message.error('加载资产包列表失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText]);

  // 加载各类资产列表（用于导出选择）
  const loadAssets = useCallback(async () => {
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
    }
  }, []);

  useEffect(() => {
    loadPackages();
  }, [loadPackages]);

  // 打开导入模态框
  const handleImportOpen = () => {
    setImportResult(null);
    setImportModalVisible(true);
  };

  // 处理文件导入
  const handleImport = async (file: File) => {
    setImporting(true);
    try {
      const result = await api.assetPackages.import(file);
      setImportResult(result);
      message.success('导入完成');
      loadPackages();
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || error.message || '导入失败';
      message.error(errorMsg);
    } finally {
      setImporting(false);
    }
    return false; // 阻止默认上传行为
  };

  // 上传配置
  const uploadProps: UploadProps = {
    accept: '.zip',
    showUploadList: false,
    beforeUpload: handleImport,
    disabled: importing,
  };

  // 打开导出模态框
  const handleExportOpen = () => {
    exportForm.resetFields();
    setSelectedSkillIds([]);
    setSelectedCommandIds([]);
    setSelectedSubagentIds([]);
    setSelectedRuleIds([]);
    setSelectedSettingsIds([]);
    setSelectedAssetType('skills');
    loadAssets();
    setExportModalVisible(true);
  };

  // 执行导出
  const handleExport = async (values: any) => {
    setExporting(true);
    try {
      const blob = await api.assetPackages.export({
        name: values.name,
        version: values.version,
        description: values.description,
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
      a.download = `${values.name}-v${values.version}.zip`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);

      message.success('导出成功');
      setExportModalVisible(false);
      loadPackages();
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || error.message || '导出失败';
      message.error(errorMsg);
    } finally {
      setExporting(false);
    }
  };

  // 删除资产包
  const handleDelete = async (id: string) => {
    try {
      await api.assetPackages.delete(id);
      message.success('删除成功');
      loadPackages();
    } catch (error) {
      message.error('删除失败');
    }
  };

  // 查看详情
  const handleViewDetail = async (pkg: AssetPackage) => {
    try {
      const detail = await api.assetPackages.get(pkg.id);
      setSelectedPackage(detail);
      setDetailModalVisible(true);
    } catch (error) {
      message.error('获取详情失败');
    }
  };

  // 格式化日期
  const formatDate = (dateStr: string) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleString('zh-CN');
  };

  // 表格列定义
  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => (
        <Space>
          <InboxOutlined style={{ color: '#1890ff' }} />
          <Text strong>{name}</Text>
        </Space>
      ),
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      render: (version: string) => <Tag color="blue">{version}</Tag>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (desc: string) => desc || '-',
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      render: formatDate,
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_: any, record: AssetPackage) => (
        <Space>
          <Button
            type="text"
            icon={<EyeOutlined />}
            onClick={() => handleViewDetail(record)}
          />
          <Popconfirm
            title="确定要删除这个资产包吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="text" icon={<DeleteOutlined />} danger />
          </Popconfirm>
        </Space>
      ),
    },
  ];

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

  // 渲染导入结果详情
  const renderImportDetails = () => {
    if (!importResult) return null;

    return (
      <div style={{ marginTop: 16 }}>
        <Alert
          type={importResult.failed > 0 ? 'warning' : 'success'}
          message={`导入完成：成功 ${importResult.success}，跳过 ${importResult.skipped}，失败 ${importResult.failed}`}
          showIcon
        />
        {importResult.details && importResult.details.length > 0 && (
          <Table
            style={{ marginTop: 12 }}
            dataSource={importResult.details}
            rowKey={(item, index) => `${item.assetType}-${item.name}-${index}`}
            pagination={false}
            size="small"
            columns={[
              {
                title: '类型',
                dataIndex: 'assetType',
                key: 'assetType',
                render: (type: string) => (
                  <Tag color={assetTypeColors[type] || 'default'}>{type}</Tag>
                ),
              },
              { title: '名称', dataIndex: 'name', key: 'name' },
              { title: '版本', dataIndex: 'version', key: 'version' },
              {
                title: '状态',
                dataIndex: 'status',
                key: 'status',
                render: (status: string) => (
                  <Tag color={statusColors[status] || 'default'}>{status}</Tag>
                ),
              },
              { title: '信息', dataIndex: 'message', key: 'message', ellipsis: true },
            ]}
          />
        )}
      </div>
    );
  };

  return (
    <div style={{ padding: 12 }}>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>资产包管理</Title>
          <Text type="secondary">导入导出资产包，便于团队协作和版本管理</Text>
        </div>
        <Space>
          <Button icon={<CloudUploadOutlined />} onClick={handleImportOpen}>
            导入
          </Button>
          <Button type="primary" icon={<CloudDownloadOutlined />} onClick={handleExportOpen}>
            导出
          </Button>
        </Space>
      </div>

      {/* 搜索区域 */}
      <Card style={{ marginBottom: 16 }}>
        <Input.Search
          placeholder="搜索资产包..."
          allowClear
          style={{ width: 300 }}
          onSearch={(value) => { setSearchText(value); setPage(1); }}
        />
      </Card>

      {/* 资产包列表 */}
      <Card>
        <Spin spinning={loading}>
          {packages.length === 0 ? (
            <Empty description="暂无资产包" style={{ padding: 48 }} />
          ) : (
            <Table
              dataSource={packages}
              columns={columns}
              rowKey="id"
              pagination={false}
            />
          )}
        </Spin>

        {/* 分页 */}
        {total > pageSize && (
          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'center' }}>
            <Pagination
              current={page}
              pageSize={pageSize}
              total={total}
              onChange={(p, ps) => {
                setPage(p);
                setPageSize(ps);
              }}
              showSizeChanger
              showTotal={(t) => `共 ${t} 条`}
              pageSizeOptions={['10', '20', '50']}
            />
          </div>
        )}
      </Card>

      {/* 导入模态框 */}
      <Modal
        title="导入资产包"
        open={importModalVisible}
        onCancel={() => setImportModalVisible(false)}
        footer={importResult ? [
          <Button key="close" onClick={() => setImportModalVisible(false)}>
            关闭
          </Button>,
        ] : null}
        width={600}
      >
        {!importResult ? (
          <Upload.Dragger {...uploadProps}>
            <p className="ant-upload-drag-icon">
              <FileZipOutlined style={{ fontSize: 48, color: '#1890ff' }} />
            </p>
            <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
            <p className="ant-upload-hint">支持 .zip 格式的资产包文件</p>
          </Upload.Dragger>
        ) : (
          renderImportDetails()
        )}
        {importing && !importResult && (
          <div style={{ textAlign: 'center', marginTop: 16 }}>
            <Spin tip="正在导入..." />
          </div>
        )}
      </Modal>

      {/* 导出模态框 */}
      <Modal
        title="导出资产包"
        open={exportModalVisible}
        onOk={() => exportForm.submit()}
        onCancel={() => setExportModalVisible(false)}
        okText="导出"
        confirmLoading={exporting}
        width={700}
      >
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
            <Input placeholder="如：my-team-assets" />
          </Form.Item>

          <Form.Item
            name="version"
            label="版本号"
            rules={[
              { required: true, message: '请输入版本号' },
              { pattern: /^\d+\.\d+\.\d+$/, message: '版本号格式如 1.0.0' },
            ]}
          >
            <Input placeholder="如：1.0.0" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="资产包描述" />
          </Form.Item>
        </Form>

        <Divider>选择资产</Divider>

        {/* 资产类型选择 */}
        <Space style={{ marginBottom: 12 }}>
          {assetTypeOptions.map(opt => (
            <Tag
              key={opt.key}
              color={selectedAssetType === opt.key ? assetTypeColors[opt.key] : 'default'}
              style={{ cursor: 'pointer' }}
              onClick={() => setSelectedAssetType(opt.key)}
            >
              {opt.title} ({getCurrentAssetList().length})
            </Tag>
          ))}
        </Space>

        {/* 已选资产统计 */}
        <div style={{ marginBottom: 12, color: '#666' }}>
          已选：技能 {selectedSkillIds.length} / 命令 {selectedCommandIds.length} /
          子代理 {selectedSubagentIds.length} / 规则 {selectedRuleIds.length} /
          配置 {selectedSettingsIds.length}
        </div>

        {/* Transfer 选择器 */}
        <Transfer
          dataSource={getCurrentAssetList()}
          titles={['可选', '已选']}
          targetKeys={getCurrentSelectedIds()}
          onChange={(targetKeys) => setCurrentSelectedIds(targetKeys as string[])}
          render={(item) => item.title}
          listStyle={{ width: 280, height: 300 }}
          showSearch
          filterOption={(input, option) =>
            (option.title as string).toLowerCase().includes(input.toLowerCase())
          }
        />
      </Modal>

      {/* 详情模态框 */}
      <Modal
        title="资产包详情"
        open={detailModalVisible}
        onCancel={() => setDetailModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setDetailModalVisible(false)}>
            关闭
          </Button>,
        ]}
        width={600}
      >
        {selectedPackage && (
          <Descriptions column={2} bordered size="small">
            <Descriptions.Item label="名称">{selectedPackage.name}</Descriptions.Item>
            <Descriptions.Item label="版本">{selectedPackage.version}</Descriptions.Item>
            <Descriptions.Item label="描述" span={2}>
              {selectedPackage.description || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="创建时间">
              {formatDate(selectedPackage.createdAt)}
            </Descriptions.Item>
            <Descriptions.Item label="更新时间">
              {formatDate(selectedPackage.updatedAt)}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  );
};

export default AssetPackageManagement;