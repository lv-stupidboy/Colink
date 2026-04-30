import React, { useEffect, useState, useCallback, useRef } from 'react';
import {
  Card, Button, Modal, Form, Input, Select, message, Space, Typography, Tag,
  Popconfirm, Empty, Spin, Divider, Tooltip, Table
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  PlusOutlined,
  EyeOutlined,
  DeleteOutlined,
  FolderOpenOutlined,
} from '@ant-design/icons';
import JSZip from 'jszip';
import settingsApi from '@/api/settingsApi';
import type { Settings } from '@/types';

const { Title, Text } = Typography;

// Agent 类型选项
const agentTypeOptions = [
  { label: 'Claude Code', value: 'claude_code', color: 'blue' },
  { label: 'OpenCode', value: 'open_code', color: 'green' },
];

// 根据Settings名称生成头像
const generateAvatar = (name: string): { initials: string; color: string } => {
  const words = name.split(/[-_\s]+/);
  let initials = '';
  if (words.length >= 2) {
    initials = (words[0][0] + words[1][0]).toUpperCase();
  } else if (name.length >= 2) {
    initials = name.substring(0, 2).toUpperCase();
  } else {
    initials = name.toUpperCase();
  }

  const colors = [
    '#1890ff', '#52c41a', '#fa8c16', '#eb2f96', '#722ed1',
    '#13c2c2', '#2f54eb', '#faad14', '#a0d911', '#f5222d'
  ];
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash);
  }
  const color = colors[Math.abs(hash) % colors.length];

  return { initials, color };
};

// Settings头像组件
const SettingsAvatar: React.FC<{ name: string }> = ({ name }) => {
  const { initials, color } = generateAvatar(name);
  return (
    <div
      style={{
        width: 36,
        height: 36,
        borderRadius: 8,
        background: `linear-gradient(135deg, ${color}dd, ${color})`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: '#fff',
        fontWeight: 600,
        fontSize: 13,
        boxShadow: `0 2px 6px ${color}40`,
      }}
    >
      {initials}
    </div>
  );
};

// 清理名称格式（只去除首尾空格）
const cleanName = (name: string): string => {
  return name ? name.trim() : '';
};

const SettingsManagement: React.FC = () => {
  const [settingsList, setSettingsList] = useState<Settings[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [searchText, setSearchText] = useState('');
  const [modalVisible, setModalVisible] = useState(false);
  const [form] = Form.useForm();
  const directoryInputRef = useRef<HTMLInputElement>(null);
  const pendingZipBlobRef = useRef<Blob | null>(null);
  const [isAfterUpload, setIsAfterUpload] = useState(false);

  // 加载Settings列表
  const loadSettings = useCallback(async () => {
    setLoading(true);
    try {
      const result = await settingsApi.list({
        page,
        pageSize,
        search: searchText,
      });
      setSettingsList(result.data);
      setTotal(result.total);
    } catch (error) {
      message.error('加载Settings列表失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText]);

  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  // 新建Settings
  const handleCreate = () => {
    setIsAfterUpload(false);
    pendingZipBlobRef.current = null;
    form.resetFields();
    form.setFieldsValue({ name: '', description: '', supportedAgents: [] });
    setModalVisible(true);
  };

  // 目录选择
  const handleDirectorySelect = () => {
    directoryInputRef.current?.click();
  };

  // 处理目录选择
  const handleDirectoryChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;

    // 获取目录名
    const firstFile = files[0];
    const pathParts = firstFile.webkitRelativePath.split('/');
    const directoryName = cleanName(pathParts[0]);

    if (!directoryName) {
      message.error('目录名不能为空');
      return;
    }

    // 检查总大小
    const totalSize = Array.from(files).reduce((sum, f) => sum + f.size, 0);
    if (totalSize > 10 * 1024 * 1024) {
      message.error('目录总大小不能超过 10MB');
      return;
    }

    try {
      message.loading({ content: '正在解析目录...', key: 'packing' });

      // 打包 zip
      const zip = new JSZip();
      for (const file of Array.from(files)) {
        const parts = file.webkitRelativePath.split('/');
        const relativePath = parts.slice(1).join('/');
        if (relativePath) {
          const content = await file.arrayBuffer();
          zip.file(relativePath, content);
        }
      }

      const zipBlob = await zip.generateAsync({ type: 'blob' });
      pendingZipBlobRef.current = zipBlob;

      message.destroy('packing');

      // 展示表单让用户确认
      setIsAfterUpload(true);
      form.setFieldsValue({
        name: directoryName,
        description: '',
      });
      setModalVisible(true);
    } catch (error) {
      message.destroy('packing');
      message.error('解析目录失败');
    }

    e.target.value = '';
  };

  // 提交表单
  const handleSubmit = async (values: any) => {
    if (!values.name || !values.name.trim()) {
      message.error('请输入名称');
      return;
    }

    const supportedAgents = form.getFieldValue('supportedAgents') || [];

    try {
      if (isAfterUpload && pendingZipBlobRef.current) {
        message.loading({ content: '正在创建Settings...', key: 'uploading' });

        const formData = new FormData();
        formData.append('file', pendingZipBlobRef.current, 'settings.zip');
        formData.append('name', values.name.trim());
        formData.append('description', values.description || '');
        formData.append('supportedAgents', JSON.stringify(supportedAgents));

        await settingsApi.create(formData);

        message.destroy('uploading');
        pendingZipBlobRef.current = null;
        setIsAfterUpload(false);
        message.success('创建成功');
      } else {
        message.error('请先选择要上传的目录');
        return;
      }

      setModalVisible(false);
      loadSettings();
    } catch (error: any) {
      message.destroy('uploading');
      const errorMsg = error.message || error.response?.data?.error || '操作失败';
      message.error(errorMsg);
    }
  };

  // 删除Settings
  const handleDelete = async (id: string) => {
    try {
      await settingsApi.delete(id);
      message.success('删除成功');
      loadSettings();
    } catch (error: any) {
      const errorData = error.response?.data;
      if (errorData?.error) {
        message.error(errorData.error);
      } else {
        message.error('删除失败');
      }
    }
  };

  // 表格列定义
  const columns: ColumnsType<Settings> = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => (
        <Space>
          <SettingsAvatar name={name} />
          <Text strong>{name}</Text>
        </Space>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (desc: string) => desc || '-',
    },
    {
      title: '兼容 Agent',
      dataIndex: 'supportedAgents',
      key: 'supportedAgents',
      width: 150,
      render: (supportedAgents: string[] | undefined) => {
        if (!supportedAgents || supportedAgents.length === 0) {
          return <Tag color="blue">Claude Code</Tag>;
        }
        return (
          <Space size="small">
            {supportedAgents.map(agent => {
              const option = agentTypeOptions.find(o => o.value === agent);
              return (
                <Tag key={agent} color={option?.color || 'default'}>
                  {option?.label || agent}
                </Tag>
              );
            })}
          </Space>
        );
      },
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      render: (time: string) => new Date(time).toLocaleString('zh-CN'),
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_, record) => (
        <Space>
          <Tooltip title="查看详情">
            <EyeOutlined
              style={{ fontSize: 16, cursor: 'pointer' }}
              onClick={() => message.info('详情功能开发中')}
            />
          </Tooltip>
          <Popconfirm
            title="确定要删除这个Settings吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <DeleteOutlined style={{ fontSize: 16, color: '#ff4d4f', cursor: 'pointer' }} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 12 }}>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>Settings 管理</Title>
          <Text type="secondary">管理 Agent 配置目录资产</Text>
        </div>
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            新建 Settings
          </Button>
        </Space>
      </div>

      {/* 搜索区域 */}
      <Card style={{ marginBottom: 16 }}>
        <Space>
          <Input.Search
            placeholder="搜索Settings..."
            allowClear
            style={{ width: 300 }}
            onSearch={(value) => { setSearchText(value); setPage(1); }}
          />
        </Space>
      </Card>

      {/* Settings表格 */}
      <Card>
        <Spin spinning={loading}>
          <Table
            columns={columns}
            dataSource={settingsList}
            rowKey="id"
            pagination={{
              current: page,
              pageSize: pageSize,
              total: total,
              onChange: (p, ps) => {
                setPage(p);
                setPageSize(ps);
              },
              showSizeChanger: true,
              showTotal: (t) => `共 ${t} 条`,
              pageSizeOptions: ['10', '20', '50'],
            }}
            locale={{ emptyText: <Empty description="暂无Settings" /> }}
          />
        </Spin>
      </Card>

      {/* 新建Settings弹窗 */}
      <Modal
        title="新建 Settings"
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={600}
        okText="创建"
      >
        {/* 隐藏的目录选择 input */}
        <input
          ref={directoryInputRef}
          type="file"
          style={{ display: 'none' }}
          onChange={handleDirectoryChange}
          multiple
          // @ts-ignore webkitdirectory 属性
          webkitdirectory=""
          directory=""
        />

        {!isAfterUpload && (
          <div
            onClick={handleDirectorySelect}
            style={{
              border: '1px dashed var(--ant-color-border)',
              borderRadius: 8,
              padding: '32px 0',
              textAlign: 'center',
              cursor: 'pointer',
              transition: 'border-color 0.3s',
              marginBottom: 16,
            }}
          >
            <p>
              <FolderOpenOutlined style={{ fontSize: 36, color: 'var(--ant-color-primary)' }} />
            </p>
            <p style={{ color: 'var(--ant-color-text)', fontSize: 14 }}>点击选择配置目录</p>
            <p style={{ fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
              支持任意配置目录，最大 10MB
            </p>
          </div>
        )}

        {isAfterUpload && (
          <div style={{ marginBottom: 16, padding: 12, background: '#f6ffed', borderRadius: 8, border: '1px solid #b7eb8f' }}>
            <Text type="success">已选择目录，请确认以下信息后创建</Text>
          </div>
        )}

        <Divider />

        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
        >
          <Form.Item
            name="name"
            label="名称"
            rules={[{ required: true, message: '请输入名称' }]}
          >
            <Input placeholder="输入配置名称" disabled={isAfterUpload} />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="配置描述（可选）" />
          </Form.Item>

          <Form.Item
            label="兼容 Agent 类型"
            name="supportedAgents"
            extra="选择此 Settings 支持的 Agent 类型"
            rules={[{ required: true, message: '请至少选择一种 Agent 类型' }]}
          >
            <Select
              mode="multiple"
              placeholder="选择支持的 Agent 类型"
              style={{ width: '100%' }}
              options={agentTypeOptions}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default SettingsManagement;