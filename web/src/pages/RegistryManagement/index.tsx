import React, { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Typography,
  Modal,
  Form,
  Input,
  Select,
  message,
  Popconfirm,
  Tooltip,
  Badge,
  Radio,
} from 'antd';
import {
  PlusOutlined,
  SyncOutlined,
  EditOutlined,
  DeleteOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import api from '@/api/client';
import type {
  SkillRegistry,
  CreateRegistryRequest,
  RegistryType,
  RegistryStatus,
  SyncPreviewResult,
  SyncOperation,
  SkillSourceType,
} from '@/types';

const { Text, Title } = Typography;
const { Option } = Select;

/**
 * 联邦技能源管理页面
 */
const RegistryManagement: React.FC = () => {
  const [registries, setRegistries] = useState<SkillRegistry[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingRegistry, setEditingRegistry] = useState<SkillRegistry | null>(null);
  const [syncingId, setSyncingId] = useState<string | null>(null);
  const [selectedType, setSelectedType] = useState<RegistryType>('github');
  const [syncPreview, setSyncPreview] = useState<SyncPreviewResult | null>(null);
  const [conflictModalVisible, setConflictModalVisible] = useState(false);
  const [conflictChoices, setConflictChoices] = useState<Record<string, 'update' | 'skip'>>({});
  const [syncingRegistryId, setSyncingRegistryId] = useState<string | null>(null);
  const [syncingRegistryName, setSyncingRegistryName] = useState('');
  const [form] = Form.useForm();

  useEffect(() => {
    loadRegistries();
  }, [page, pageSize]);

  // 获取来源类型颜色
  const getSourceTypeColor = (sourceType: SkillSourceType): string => {
    const colors: Record<SkillSourceType, string> = {
      personal: 'blue',
      platform: 'green',
      federated: 'purple',
    };
    return colors[sourceType] || 'default';
  };

  // 获取来源类型标签
  const getSourceTypeLabel = (sourceType: SkillSourceType): string => {
    const labels: Record<SkillSourceType, string> = {
      personal: '个人',
      platform: '平台',
      federated: '联邦源',
    };
    return labels[sourceType] || sourceType;
  };

  const loadRegistries = async () => {
    setLoading(true);
    try {
      const response = await api.registries.list({ page, size: pageSize });
      setRegistries(response.data || []);
      setTotal(response.total || 0);
    } catch (error) {
      message.error('加载联邦源列表失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingRegistry(null);
    setSelectedType('github');
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (registry: SkillRegistry) => {
    setEditingRegistry(registry);
    setSelectedType(registry.type);
    form.setFieldsValue({
      name: registry.name,
      displayName: registry.displayName,
      type: registry.type,
      url: registry.url,
      syncInterval: registry.syncInterval,
      status: registry.status,
      authConfig: registry.authConfig || {},
    });
    setModalVisible(true);
  };

  const handleSubmit = async (values: any) => {
    try {
      if (editingRegistry) {
        await api.registries.update(editingRegistry.id, values);
        message.success('联邦源更新成功');
      } else {
        await api.registries.create(values as CreateRegistryRequest);
        message.success('联邦源创建成功');
      }
      setModalVisible(false);
      loadRegistries();
    } catch (error: any) {
      message.error(error.response?.data?.error || '操作失败');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.registries.delete(id);
      message.success('联邦源已删除');
      loadRegistries();
    } catch (error: any) {
      message.error(error.response?.data?.error || '删除失败');
    }
  };

  const handleSync = async (id: string) => {
    setSyncingId(id);
    try {
      // 先调用 sync-preview API
      const preview = await api.registries.syncPreview(id);

      // 分析冲突情况
      if (preview.conflictSkills.length === 0) {
        // 无冲突，直接执行同步（调用原有 sync API）
        const result = await api.registries.sync(id);
        if (result.error) {
          message.error(`同步失败: ${result.error}`);
        } else {
          message.success(`同步完成：自动更新 ${preview.autoUpdateSkills.length} 个，跳过 ${preview.newSkills.length} 个新 skill`);
          loadRegistries();
        }
      } else {
        // 有冲突，显示弹窗
        setSyncPreview(preview);
        setSyncingRegistryId(id);
        setSyncingRegistryName(preview.registryName);
        setConflictChoices({});
        setConflictModalVisible(true);
      }
    } catch (error: any) {
      message.error(error.response?.data?.error || '同步预览失败');
    } finally {
      setSyncingId(null);
    }
  };

  // 确认冲突选择
  const handleConfirmConflict = async () => {
    if (!syncPreview || !syncingRegistryId) return;

    // 检查是否所有冲突项都已选择
    const unselected = syncPreview.conflictSkills.filter(s => !conflictChoices[s.name]);
    if (unselected.length > 0) {
      message.error(`以下 Skill 未选择操作：${unselected.map(s => s.name).join(', ')}`);
      return;
    }

    setConflictModalVisible(false);
    setSyncingId(syncingRegistryId);

    try {
      // 构建同步确认请求
      const operations: SyncOperation[] = [];
      for (const skill of syncPreview.conflictSkills) {
        const choice = conflictChoices[skill.name];
        operations.push({
          action: choice,
          skillName: skill.name,
          targetSkillId: choice === 'update' ? skill.localSkill.id : undefined,
          description: skill.description,
        });
      }

      const result = await api.registries.syncConfirm(syncingRegistryId, {
        registryId: syncingRegistryId,
        operations,
      });

      // 显示结果汇总
      let successMsg = `同步完成：自动更新 ${result.autoUpdated} 个`;
      if (result.userUpdated > 0) {
        successMsg += `，更新 ${result.userUpdated} 个`;
      }
      if (result.userSkipped > 0) {
        successMsg += `，跳过 ${result.userSkipped} 个`;
      }
      message.success(successMsg);

      if (result.skipped.length > 0) {
        message.warning(`跳过 ${result.skipped.length} 个失败项：${result.skipped.map(s => s.name).join(', ')}`);
      }

      loadRegistries();
    } catch (error: any) {
      message.error(error.response?.data?.error || '同步确认失败');
    } finally {
      setSyncingId(null);
      setSyncPreview(null);
      setSyncingRegistryId(null);
    }
  };

  // 全部更新
  const handleAllUpdate = () => {
    if (!syncPreview) return;
    const choices: Record<string, 'update'> = {};
    syncPreview.conflictSkills.forEach(s => choices[s.name] = 'update');
    setConflictChoices(choices);
  };

  // 全部跳过
  const handleAllSkip = () => {
    if (!syncPreview) return;
    const choices: Record<string, 'skip'> = {};
    syncPreview.conflictSkills.forEach(s => choices[s.name] = 'skip');
    setConflictChoices(choices);
  };

  const handleSyncAll = async () => {
    setSyncingId('all');
    try {
      const result = await api.registries.syncAll();
      message.success(result.message);
      loadRegistries();
    } catch (error: any) {
      message.error(error.response?.data?.error || '同步失败');
    } finally {
      setSyncingId(null);
    }
  };

  const getTypeTag = (type: RegistryType) => {
    const typeConfig: Record<RegistryType, { color: string; text: string }> = {
      github: { color: 'blue', text: 'GitHub' },
      gitlab: { color: 'orange', text: 'GitLab' },
      api: { color: 'green', text: 'API' },
      custom: { color: 'purple', text: '自定义' },
      codehub: { color: 'cyan', text: 'CodeHub' },
    };
    const config = typeConfig[type] || { color: 'default', text: type };
    return <Tag color={config.color}>{config.text}</Tag>;
  };

  const getStatusBadge = (status: RegistryStatus) => {
    return status === 'active' ? (
      <Badge status="success" text="活跃" />
    ) : (
      <Badge status="default" text="停用" />
    );
  };

  const getSyncStatusIcon = (status: string) => {
    switch (status) {
      case 'success':
        return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
      case 'failed':
        return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />;
      default:
        return <ClockCircleOutlined style={{ color: '#faad14' }} />;
    }
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 150,
      render: (name: string, record: SkillRegistry) => (
        <Space direction="vertical" size={0}>
          <Text strong>{record.displayName || name}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>{name}</Text>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 100,
      render: (type: RegistryType) => getTypeTag(type),
    },
    {
      title: 'URL',
      dataIndex: 'url',
      key: 'url',
      ellipsis: true,
      render: (url: string) => (
        <a href={url} target="_blank" rel="noopener noreferrer">
          {url}
        </a>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 80,
      render: (status: RegistryStatus) => getStatusBadge(status),
    },
    {
      title: '同步状态',
      dataIndex: 'syncStatus',
      key: 'syncStatus',
      width: 100,
      render: (status: string, record: SkillRegistry) => (
        <Tooltip title={record.lastSyncAt ? `最后同步: ${new Date(record.lastSyncAt).toLocaleString()}` : '从未同步'}>
          <Space>
            {getSyncStatusIcon(status)}
            <Text>{record.skillCount} Skills</Text>
          </Space>
        </Tooltip>
      ),
    },
    {
      title: '同步间隔',
      dataIndex: 'syncInterval',
      key: 'syncInterval',
      width: 100,
      render: (interval: number) => {
        const hours = Math.floor(interval / 3600);
        return <Text>{hours > 0 ? `${hours}小时` : `${interval}秒`}</Text>;
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 200,
      render: (_: unknown, record: SkillRegistry) => (
        <Space>
          <Tooltip title="同步">
            <Button
              type="link"
              icon={<SyncOutlined spin={syncingId === record.id} />}
              onClick={() => handleSync(record.id)}
              loading={syncingId === record.id}
              disabled={syncingId !== null}
            />
          </Tooltip>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          />
          <Popconfirm
            title="确定要删除此联邦源吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className="registry-management" style={{ padding: 12 }}>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>联邦 Skills 源</Title>
          <Text type="secondary">管理外部 Skills 仓库，同步联邦 Skills</Text>
        </div>
        <Space>
          <Button
            icon={<SyncOutlined spin={syncingId === 'all'} />}
            onClick={handleSyncAll}
            loading={syncingId === 'all'}
            disabled={syncingId !== null}
          >
            同步全部
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            新建联邦源
          </Button>
        </Space>
      </div>

      <Card>
        <Table
          dataSource={registries}
          columns={columns}
          rowKey="id"
          loading={loading}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条`,
            onChange: (p, ps) => {
              setPage(p);
              setPageSize(ps);
            },
          }}
        />
      </Card>

      <Modal
        title={editingRegistry ? '编辑联邦源' : '新建联邦源'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{ syncInterval: 3600, status: 'active', type: 'github' }}
        >
          <Form.Item
            name="name"
            label="标识名称"
            rules={[
              { required: true, message: '请输入标识名称' },
              { pattern: /^[a-z0-9_-]+$/, message: '只能包含小写字母、数字、下划线和连字符' },
            ]}
          >
            <Input placeholder="例如: my-github-skills" disabled={!!editingRegistry} />
          </Form.Item>
          <Form.Item name="displayName" label="显示名称">
            <Input placeholder="我的 GitHub Skills 库" />
          </Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select disabled={!!editingRegistry} onChange={(value) => setSelectedType(value as RegistryType)}>
              <Option value="github">GitHub</Option>
              <Option value="gitlab">GitLab</Option>
              <Option value="api">API</Option>
              <Option value="custom">自定义</Option>
              <Option value="codehub">CodeHub 代码托管服务</Option>
            </Select>
          </Form.Item>
          <Form.Item name="url" label="URL" rules={[{ required: true, message: '请输入URL' }]}>
            <Input placeholder="https://github.com/owner/repo" />
          </Form.Item>
          {selectedType === 'codehub' && (
            <>
              <Form.Item
                name={['authConfig', 'username']}
                label="用户名"
                extra="HTTPS 认证账号（SSH 格式 URL 可不填）"
              >
                <Input placeholder="CodeHub 用户名" />
              </Form.Item>
              <Form.Item
                name={['authConfig', 'password']}
                label="密码"
                extra="HTTPS 认证密码（SSH 格式 URL 可不填）"
              >
                <Input.Password placeholder="CodeHub 密码" />
              </Form.Item>
              <div style={{ marginBottom: 16, padding: '8px 12px', background: '#f5f5f5', borderRadius: 4 }}>
                <Text type="secondary">
                  SSH 格式 URL 将使用系统全局 SSH Key 认证，无需配置账号密码
                </Text>
              </div>
            </>
          )}
          <Form.Item name="syncInterval" label="同步间隔（秒）">
            <Select>
              <Option value={1800}>30分钟</Option>
              <Option value={3600}>1小时</Option>
              <Option value={21600}>6小时</Option>
              <Option value={43200}>12小时</Option>
              <Option value={86400}>24小时</Option>
            </Select>
          </Form.Item>
          {editingRegistry && (
            <Form.Item name="status" label="状态">
              <Select>
                <Option value="active">活跃</Option>
                <Option value="inactive">停用</Option>
              </Select>
            </Form.Item>
          )}
        </Form>
      </Modal>

      {/* 同步冲突处理弹窗 */}
      <Modal
        title="同步冲突处理"
        open={conflictModalVisible}
        onCancel={() => setConflictModalVisible(false)}
        width={800}
        footer={[
          <Button key="cancel" onClick={() => setConflictModalVisible(false)}>取消</Button>,
          <Button key="all-skip" onClick={handleAllSkip}>
            全部跳过
          </Button>,
          <Button key="all-update" type="primary" onClick={handleAllUpdate}>
            全部更新
          </Button>,
          <Button key="confirm" type="primary" onClick={handleConfirmConflict}>
            确认同步
          </Button>,
        ]}
      >
        <Text type="secondary" style={{ marginBottom: 16, display: 'block' }}>
          以下 Skill 与本地已有同名 Skill 来源不同，请选择处理方式：
        </Text>
        {syncPreview && (
          <>
            {syncPreview.autoUpdateSkills.length > 0 && (
              <div style={{ marginBottom: 12 }}>
                <Tag color="green">{syncPreview.autoUpdateSkills.length} 个同源 Skill 将自动更新</Tag>
              </div>
            )}
            <Table
              dataSource={syncPreview.conflictSkills}
              columns={[
                {
                  title: '名称',
                  dataIndex: 'name',
                  key: 'name',
                  width: 120,
                },
                {
                  title: '本地来源',
                  key: 'localSource',
                  width: 120,
                  render: (_, record) => {
                    const sourceType = record.localSkill.sourceType as SkillSourceType;
                    return (
                      <Tag color={getSourceTypeColor(sourceType)}>
                        {record.localSkill.sourceRegistryName || getSourceTypeLabel(sourceType)}
                      </Tag>
                    );
                  },
                },
                {
                  title: '远程来源',
                  key: 'remoteSource',
                  width: 120,
                  render: () => (
                    <Tag color="cyan">{syncingRegistryName}</Tag>
                  ),
                },
                {
                  title: '本地描述',
                  key: 'localDesc',
                  width: 200,
                  render: (_, record) => (
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {record.localSkill.description?.slice(0, 50) || '暂无'}
                      {record.localSkill.description?.length > 50 ? '...' : ''}
                    </Text>
                  ),
                },
                {
                  title: '远程描述',
                  dataIndex: 'description',
                  key: 'remoteDesc',
                  width: 200,
                  render: (desc: string) => (
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {desc?.slice(0, 50) || '暂无'}
                      {desc?.length > 50 ? '...' : ''}
                    </Text>
                  ),
                },
                {
                  title: '操作',
                  key: 'action',
                  width: 150,
                  render: (_, record) => (
                    <Radio.Group
                      value={conflictChoices[record.name]}
                      onChange={(e) => {
                        setConflictChoices(prev => ({
                          ...prev,
                          [record.name]: e.target.value,
                        }));
                      }}
                    >
                      <Radio value="skip">跳过</Radio>
                      <Radio value="update">更新</Radio>
                    </Radio.Group>
                  ),
                },
              ]}
              rowKey="name"
              pagination={false}
              size="small"
            />
          </>
        )}
      </Modal>
    </div>
  );
};

export default RegistryManagement;