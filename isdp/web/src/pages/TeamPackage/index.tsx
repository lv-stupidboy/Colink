import React, { useState, useEffect } from 'react';
import {
  Card, Button, Upload, Select, Table, Tag, Space, Typography, message,
  Divider, Alert, Spin, Empty, Popconfirm
} from 'antd';
import {
  CloudUploadOutlined,
  CloudDownloadOutlined,
  FileZipOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import type { UploadProps } from 'antd';
import api from '@/api/client';

const { Title, Text } = Typography;
const { Dragger } = Upload;

// TeamPackagePreview 类型定义
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

// ImportConfirm 类型定义
interface ImportConfirm {
  mode: 'overwrite' | 'skip' | 'selective';
  workflowAction: 'overwrite' | 'skip';
  roleActions: Array<{ name: string; action: 'overwrite' | 'skip' }>;
  assetActions: Array<{ assetType: string; name: string; action: 'overwrite' | 'skip' }>;
}

const TeamPackageManagement: React.FC = () => {
  // 工作流列表
  const [workflows, setWorkflows] = useState<any[]>([]);
  const [selectedWorkflowId, setSelectedWorkflowId] = useState<string>('');
  const [loadingWorkflows, setLoadingWorkflows] = useState(false);

  // 导入状态
  const [importFile, setImportFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<TeamPackagePreview | null>(null);
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [importing, setImporting] = useState(false);

  // 导出状态
  const [exporting, setExporting] = useState(false);

  // 加载工作流列表
  useEffect(() => {
    loadWorkflows();
  }, []);

  const loadWorkflows = async () => {
    setLoadingWorkflows(true);
    try {
      const result = await api.workflows.list();
      setWorkflows(result);
    } catch (error) {
      message.error('加载团队列表失败');
    } finally {
      setLoadingWorkflows(false);
    }
  };

  // 处理文件上传预览
  const handleUploadPreview = async (file: File) => {
    setImportFile(file);
    setLoadingPreview(true);
    setPreview(null);
    try {
      const result = await api.teamPackages.import(file);
      setPreview(result);
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || error.message || '解析团队包失败';
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
  const buildImportConfirm = (mode: 'overwrite' | 'skip'): ImportConfirm => {
    if (!preview) return { mode, workflowAction: mode, roleActions: [], assetActions: [] };

    const roleActions = preview.roles.map(r => ({ name: r.name, action: mode }));
    const assetActions: Array<{ assetType: string; name: string; action: 'overwrite' | 'skip' }> = [];

    // 收集所有资产
    Object.entries(preview.assets).forEach(([assetType, items]) => {
      items.forEach(item => {
        assetActions.push({ assetType, name: item.name, action: mode });
      });
    });

    return {
      mode,
      workflowAction: preview.workflow.exists ? mode : 'overwrite',
      roleActions,
      assetActions,
    };
  };

  // 执行确认导入
  const handleImportConfirm = async (mode: 'overwrite' | 'skip') => {
    if (!importFile || !preview) return;

    setImporting(true);
    try {
      const confirm = buildImportConfirm(mode);
      const result = await api.teamPackages.importConfirm(importFile, confirm);
      // 从 details 中统计各类型的成功数量
      const countByType = (type: string) =>
        result.details?.filter((d: any) => d.assetType === type && d.status === 'success').length || 0;
      const skillsCount = countByType('skill');
      const commandsCount = countByType('command');
      const subagentsCount = countByType('subagent');
      const rulesCount = countByType('rule');
      const settingsCount = countByType('settings');
      const rolesCount = countByType('role');
      const workflowName = result.details?.find((d: any) => d.assetType === 'workflow')?.name || '';
      message.success(`导入成功：团队 ${workflowName}，角色 ${rolesCount} 个，Skills ${skillsCount}、Commands ${commandsCount}、Subagents ${subagentsCount}、Rules ${rulesCount}、Settings ${settingsCount}`);
      // 清理状态
      setImportFile(null);
      setPreview(null);
      // 刷新工作流列表
      loadWorkflows();
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || error.message || '导入失败';
      message.error(errorMsg);
    } finally {
      setImporting(false);
    }
  };

  // 执行导出
  const handleExport = async () => {
    if (!selectedWorkflowId) {
      message.warning('请先选择要导出的工作流');
      return;
    }

    setExporting(true);
    try {
      const blob = await api.teamPackages.export(selectedWorkflowId);
      const workflow = workflows.find(w => w.id === selectedWorkflowId);
      const fileName = `${workflow?.name || 'team-package'}.zip`;

      // 创建下载链接
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = fileName;
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

  // 清除预览
  const handleClearPreview = () => {
    setImportFile(null);
    setPreview(null);
  };

  // 渲染预览表格
  const renderPreviewTable = () => {
    if (!preview) return null;

    // 构建表格数据
    const dataSource: any[] = [];

    // 工作流
    dataSource.push({
      key: 'workflow',
      type: 'Team',
      name: preview.workflow.name,
      exists: preview.workflow.exists,
      action: preview.workflow.exists ? '待处理' : '新增',
    });

    // 角色
    preview.roles.forEach((role, idx) => {
      dataSource.push({
        key: `role-${idx}`,
        type: 'Role',
        name: role.name,
        exists: role.exists,
        action: role.exists ? '待处理' : '新增',
      });
    });

    // 技能
    preview.assets.skills.forEach((skill, idx) => {
      dataSource.push({
        key: `skill-${idx}`,
        type: 'Skill',
        name: skill.name,
        exists: skill.exists,
        action: skill.exists ? '待处理' : '新增',
      });
    });

    // 命令
    preview.assets.commands.forEach((cmd, idx) => {
      dataSource.push({
        key: `command-${idx}`,
        type: 'Command',
        name: cmd.name,
        exists: cmd.exists,
        action: cmd.exists ? '待处理' : '新增',
      });
    });

    // 子代理
    preview.assets.subagents.forEach((sub, idx) => {
      dataSource.push({
        key: `subagent-${idx}`,
        type: 'Subagent',
        name: sub.name,
        exists: sub.exists,
        action: sub.exists ? '待处理' : '新增',
      });
    });

    // 规则
    preview.assets.rules.forEach((rule, idx) => {
      dataSource.push({
        key: `rule-${idx}`,
        type: 'Rule',
        name: rule.name,
        exists: rule.exists,
        action: rule.exists ? '待处理' : '新增',
      });
    });

    // 配置
    preview.assets.settings.forEach((setting, idx) => {
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
        render: (type: string) => <Tag>{type}</Tag>,
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
    if (preview.workflow.exists) count++;
    count += preview.roles.filter(r => r.exists).length;
    count += preview.assets.skills.filter(s => s.exists).length;
    count += preview.assets.commands.filter(c => c.exists).length;
    count += preview.assets.subagents.filter(s => s.exists).length;
    count += preview.assets.rules.filter(r => r.exists).length;
    count += preview.assets.settings.filter(s => s.exists).length;
    return count;
  };

  return (
    <div style={{ padding: 24 }}>
      <Title level={2}>团队包管理</Title>
      <Text type="secondary">导入导出团队配置包，包含团队、角色和资产</Text>

      <Divider />

      <div style={{ display: 'flex', gap: 24 }}>
        {/* 左侧：导入区域 */}
        <Card
          title="导入团队包"
          style={{ flex: 1, minHeight: 400 }}
          extra={preview && (
            <Button size="small" onClick={handleClearPreview}>
              清除
            </Button>
          )}
        >
          {!preview ? (
            <Spin spinning={loadingPreview}>
              <Dragger {...uploadProps}>
                <p className="ant-upload-drag-icon">
                  <FileZipOutlined style={{ fontSize: 48, color: '#1890ff' }} />
                </p>
                <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
                <p className="ant-upload-hint">支持 .zip 格式的团队包文件</p>
              </Dragger>
            </Spin>
          ) : (
            <div>
              {/* 冲突提示 */}
              {getConflictCount() > 0 && (
                <Alert
                  type="warning"
                  icon={<WarningOutlined />}
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
          title="导出团队包"
          style={{ flex: 1, minHeight: 400 }}
        >
          <Spin spinning={loadingWorkflows || exporting}>
            <div style={{ marginBottom: 24 }}>
              <Text>选择要导出的团队：</Text>
              <Select
                style={{ width: '100%', marginTop: 8 }}
                placeholder="请选择团队"
                value={selectedWorkflowId}
                onChange={setSelectedWorkflowId}
                loading={loadingWorkflows}
                options={workflows.map(w => ({
                  value: w.id,
                  label: w.name,
                }))}
              />
            </div>

            {workflows.length === 0 && !loadingWorkflows && (
              <Empty description="暂无可用团队" style={{ marginTop: 48 }} />
            )}

            {selectedWorkflowId && (
              <Alert
                type="info"
                message="导出内容包括"
                description={
                  <ul style={{ margin: 0, paddingLeft: 20 }}>
                    <li>团队配置（阶段、流转规则）</li>
                    <li>关联的角色 Agent 配置</li>
                    <li>角色绑定的 Skills、Commands、Subagents、Rules、Settings</li>
                  </ul>
                }
                style={{ marginBottom: 24 }}
              />
            )}

            <div style={{ textAlign: 'center' }}>
              <Button
                type="primary"
                icon={<CloudDownloadOutlined />}
                size="large"
                onClick={handleExport}
                loading={exporting}
                disabled={!selectedWorkflowId}
              >
                导出团队包
              </Button>
            </div>
          </Spin>
        </Card>
      </div>
    </div>
  );
};

export default TeamPackageManagement;