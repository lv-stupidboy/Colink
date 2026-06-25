import React, { useEffect, useState, useCallback, useRef } from 'react';
import {
  Card, Button, Modal, Form, Input, message, Space, Typography,
  Popconfirm, Empty, Spin, Divider, Tooltip, Table
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  PlusOutlined,
  DeleteOutlined,
  FolderOpenOutlined,
  TeamOutlined,
  EditOutlined,
} from '@ant-design/icons';
import JSZip from 'jszip';
import settingsApi from '@/api/settingsApi';
import api from '@/api/client';
import type { Settings, AssetAgentsResponse, SettingsUpdateResponse } from '@/types';

const { Title, Text } = Typography;

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
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editingSettings, setEditingSettings] = useState<Settings | null>(null);
  const [editForm] = Form.useForm();
  const [submitLoading, setSubmitLoading] = useState(false);
  const [submitLoadingText, setSubmitLoadingText] = useState('');
  const [affectedAgents, setAffectedAgents] = useState<{ id: string; name: string }[]>([]);
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
    form.setFieldsValue({ name: '', description: '' });
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

    try {
      if (isAfterUpload && pendingZipBlobRef.current) {
        message.loading({ content: '正在创建Settings...', key: 'uploading' });

        const formData = new FormData();
        formData.append('file', pendingZipBlobRef.current, 'settings.zip');
        formData.append('name', values.name.trim());
        formData.append('description', values.description || '');

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

  // 查看引用的角色
  const handleViewRefs = async (settings: Settings) => {
    try {
      const result: AssetAgentsResponse = await api.settings.getBoundAgents(settings.id);
      if (result.agents && result.agents.length > 0) {
        Modal.info({
          title: '角色引用',
          width: 500,
          content: (
            <div>
              <p>该 Settings 被以下 <strong>{result.count}</strong> 个角色引用：</p>
              <ul style={{ marginTop: 8, paddingLeft: 20 }}>
                {result.agents.map((agent: { id: string; name: string }) => (
                  <li key={agent.id}>{agent.name}</li>
                ))}
              </ul>
            </div>
          ),
        });
      } else {
        Modal.info({
          title: '角色引用',
          content: <p>该 Settings 暂未被任何角色引用</p>,
        });
      }
    } catch (error) {
      message.error('查询引用失败');
    }
  };

  // 编辑Settings
  const handleEdit = (record: Settings) => {
    setEditingSettings(record);
    editForm.setFieldsValue({
      description: record.description,
    });
    setEditModalVisible(true);
  };

  // 提交编辑
  const handleEditSubmit = async (values: any) => {
    if (!editingSettings) return;

    setSubmitLoading(true);
    setSubmitLoadingText('正在更新Settings配置...');
    setAffectedAgents([]);

    try {
      const startTime = Date.now();
      const result: SettingsUpdateResponse = await api.settings.update(editingSettings.id, {
        description: values.description,
      });

      // 设置受影响的角色列表
      if (result.affectedAgents && result.affectedAgents.length > 0) {
        setAffectedAgents(result.affectedAgents);
        setSubmitLoadingText(`正在为 ${result.affectedCount} 个角色刷新配置...`);
      }

      // 确保 loading 效果至少显示 1500ms
      const elapsed = Date.now() - startTime;
      const remainingTime = Math.max(0, 1500 - elapsed);
      await new Promise(resolve => setTimeout(resolve, remainingTime));

      setSubmitLoading(false);
      setSubmitLoadingText('');
      setAffectedAgents([]);

      if (result.affectedCount && result.affectedCount > 0) {
        message.success(`更新成功，已自动刷新 ${result.affectedCount} 个角色的配置`);
      } else {
        message.success('更新成功，暂未被角色引用，无需更新配置');
      }

      setEditModalVisible(false);
      loadSettings();
    } catch (error: any) {
      setSubmitLoading(false);
      setSubmitLoadingText('');
      setAffectedAgents([]);
      const errorData = error.response?.data;
      if (errorData?.error) {
        message.error(errorData.error);
      } else {
        message.error('更新失败');
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
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      render: (time: string) => new Date(time).toLocaleString('zh-CN'),
    },
    {
      title: '操作',
      key: 'action',
      width: 280,
      render: (_, record) => (
        <Space size="small">
          <Tooltip title="查看引用的角色">
            <Button
              type="link"
              size="small"
              icon={<TeamOutlined />}
              onClick={() => handleViewRefs(record)}
            />
          </Tooltip>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这个Settings吗？"
            description="删除后将无法恢复"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 12 }}>
      {/* 全屏 loading overlay */}
      {submitLoading && (
        <div style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          backgroundColor: 'rgba(0, 0, 0, 0.6)',
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          zIndex: 9999,
        }}>
          <div style={{
            backgroundColor: 'var(--bg-container, #fff)',
            padding: 32,
            borderRadius: 12,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: 16,
            boxShadow: '0 4px 20px rgba(0, 0, 0, 0.3)',
            minWidth: 300,
            maxWidth: 400,
          }}>
            <Spin size="large" />
            <span style={{ fontSize: 16, color: 'var(--text-primary, #333)' }}>{submitLoadingText}</span>
            {affectedAgents.length > 0 && (
              <div style={{
                marginTop: 8,
                maxHeight: 150,
                overflow: 'auto',
                width: '100%',
              }}>
                <div style={{
                  fontSize: 12,
                  color: 'var(--text-secondary, #666)',
                  marginBottom: 8,
                }}>正在更新的角色：</div>
                {affectedAgents.map(agent => (
                  <div key={agent.id} style={{
                    fontSize: 13,
                    color: 'var(--text-primary, #333)',
                    padding: '4px 8px',
                    background: 'var(--bg-secondary, #f5f5f5)',
                    borderRadius: 4,
                    marginBottom: 4,
                  }}>
                    {agent.name}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
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
        </Form>
      </Modal>

      {/* 编辑Settings弹窗 */}
      <Modal
        title={`编辑 Settings: ${editingSettings?.name}`}
        open={editModalVisible}
        onOk={() => editForm.submit()}
        onCancel={() => setEditModalVisible(false)}
        width={600}
        okText="保存"
      >
        <Form
          form={editForm}
          layout="vertical"
          onFinish={handleEditSubmit}
        >
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="配置描述（可选）" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default SettingsManagement;