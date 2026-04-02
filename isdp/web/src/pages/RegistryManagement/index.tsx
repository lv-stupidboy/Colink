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
import type { SkillRegistry, CreateRegistryRequest, RegistryType, RegistryStatus } from '@/types';

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
  const [form] = Form.useForm();

  useEffect(() => {
    loadRegistries();
  }, [page, pageSize]);

  const loadRegistries = async () => {
    setLoading(true);
    try {
      const response = await api.registries.list({ page, size: pageSize });
      setRegistries(response.data || []);
      setTotal(response.total || 0);
    } catch (error) {
      message.error('加载注册表列表失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingRegistry(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (registry: SkillRegistry) => {
    setEditingRegistry(registry);
    form.setFieldsValue({
      name: registry.name,
      displayName: registry.displayName,
      type: registry.type,
      url: registry.url,
      syncInterval: registry.syncInterval,
      status: registry.status,
    });
    setModalVisible(true);
  };

  const handleSubmit = async (values: any) => {
    try {
      if (editingRegistry) {
        await api.registries.update(editingRegistry.id, values);
        message.success('注册表更新成功');
      } else {
        await api.registries.create(values as CreateRegistryRequest);
        message.success('注册表创建成功');
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
      message.success('注册表已删除');
      loadRegistries();
    } catch (error: any) {
      message.error(error.response?.data?.error || '删除失败');
    }
  };

  const handleSync = async (id: string) => {
    setSyncingId(id);
    try {
      const result = await api.registries.sync(id);
      if (result.error) {
        message.error(`同步失败: ${result.error}`);
      } else {
        message.success(`同步成功：新增 ${result.skillsAdded}，更新 ${result.skillsUpdated}`);
        loadRegistries();
      }
    } catch (error: any) {
      message.error(error.response?.data?.error || '同步失败');
    } finally {
      setSyncingId(null);
    }
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
            title="确定要删除此注册表吗？"
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
            新建注册表
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
        title={editingRegistry ? '编辑注册表' : '新建注册表'}
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
            <Select disabled={!!editingRegistry}>
              <Option value="github">GitHub</Option>
              <Option value="gitlab">GitLab</Option>
              <Option value="api">API</Option>
              <Option value="custom">自定义</Option>
            </Select>
          </Form.Item>
          <Form.Item name="url" label="URL" rules={[{ required: true, message: '请输入URL' }]}>
            <Input placeholder="https://github.com/owner/repo" />
          </Form.Item>
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
    </div>
  );
};

export default RegistryManagement;