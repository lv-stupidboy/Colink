import React, { useEffect, useState, useCallback, useRef } from 'react';
import {
  Card, Button, Modal, Form, Input, Select, message, Space, Tag, Typography,
  Popconfirm, Empty, Spin, Divider, Tooltip, Table
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  PlusOutlined,
  EyeOutlined,
  DeleteOutlined,
  FolderOpenOutlined,
  LinkOutlined,
} from '@ant-design/icons';
import JSZip from 'jszip';
import settingsApi from '@/api/settingsApi';
import type { Settings, AgentConfig } from '@/types';
import api from '@/api/client';

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

// 验证Settings名称格式
const validateSettingsName = (name: string): { valid: boolean; message?: string } => {
  if (!name) {
    return { valid: false, message: '请输入名称' };
  }
  const pattern = /^[a-z][a-z0-9-]*$/;
  if (!pattern.test(name)) {
    return { valid: false, message: '名称只能包含小写字母、数字和中划线，且必须以字母开头' };
  }
  if (name.includes('--')) {
    return { valid: false, message: '名称不能包含连续的中划线' };
  }
  if (name.endsWith('-')) {
    return { valid: false, message: '名称不能以中划线结尾' };
  }
  return { valid: true };
};

// 清理名称格式
const cleanName = (name: string): string => {
  if (!name) return '';
  // 只保留小写字母、数字、中划线
  let cleaned = name.toLowerCase().replace(/[^a-z0-9-]/g, '-');
  cleaned = cleaned.replace(/^-+|-+$/g, '');
  // 确保以字母开头
  if (cleaned && !/^[a-z]/.test(cleaned)) {
    cleaned = 's-' + cleaned;
  }
  return cleaned;
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

  // Agent绑定Modal
  const [bindModalVisible, setBindModalVisible] = useState(false);
  const [agents, setAgents] = useState<AgentConfig[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState<string>('');
  const [availableSettings, setAvailableSettings] = useState<Settings[]>([]);
  const [selectedSettingsIds, setSelectedSettingsIds] = useState<string[]>([]);
  const [boundSettings, setBoundSettings] = useState<Settings[]>([]);
  const [bindLoading, setBindLoading] = useState(false);

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

  // 加载Agent列表
  const loadAgents = useCallback(async () => {
    try {
      const result = await api.agents.list();
      setAgents(result);
    } catch (error) {
      // 忽略错误
    }
  }, []);

  useEffect(() => {
    loadSettings();
    loadAgents();
  }, [loadSettings, loadAgents]);

  // 新建Settings
  const handleCreate = () => {
    setIsAfterUpload(false);
    pendingZipBlobRef.current = null;
    form.resetFields();
    form.setFieldsValue({ name: '', description: '', version: '' });
    setModalVisible(true);
  };

  // 目录选择
  const handleDirectorySelect = () => {
    directoryInputRef.current?.click();
  };

  // 解析 SETTINGS.md 内容提取描述
  const parseSettingsMD = (content: string): { name: string; description: string; version: string } => {
    let name = '';
    let description = '';
    let version = '';

    // 1. 首先尝试解析 YAML front matter
    const frontMatterMatch = content.match(/^---\s*\n([\s\S]*?)\n---/);
    if (frontMatterMatch) {
      const frontMatter = frontMatterMatch[1];
      // 提取 description
      const descMatch = frontMatter.match(/description:\s*(.+)/i);
      if (descMatch) {
        description = descMatch[1].trim();
        description = description.replace(/^["']|["']$/g, '');
      }
      // 提取 name
      const nameMatch = frontMatter.match(/name:\s*(.+)/i);
      if (nameMatch) {
        name = nameMatch[1].trim();
        name = name.replace(/^["']|["']$/g, '');
      }
      // 提取 version
      const versionMatch = frontMatter.match(/version:\s*(.+)/i);
      if (versionMatch) {
        version = versionMatch[1].trim();
        version = version.replace(/^["']|["']$/g, '');
      }
    }

    // 2. 如果没有从 front matter 获取到 name，尝试从标题获取
    if (!name) {
      const titleMatch = content.match(/^#\s+(.+)$/m);
      if (titleMatch) {
        name = titleMatch[1].trim();
      }
    }

    // 3. 如果没有从 front matter 获取到 description，尝试从 ## Description 获取
    if (!description) {
      const patterns = [
        /##\s*(?:Description|描述)\s*\n+([\s\S]*?)(?=\n##|$)/i,
        /##\s*(?:Description|描述)\s*[:：]?\s*([\s\S]*?)(?=\n##|$)/i,
      ];

      for (const pattern of patterns) {
        const descMatch = content.match(pattern);
        if (descMatch && descMatch[1]) {
          description = descMatch[1].trim();
          break;
        }
      }
    }

    return { name, description, version };
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
      message.error('目录名格式无效');
      return;
    }

    // 检查是否包含 SETTINGS.md
    const settingsMdFile = Array.from(files).find(f => {
      const parts = f.webkitRelativePath.split('/');
      return parts[parts.length - 1].toLowerCase() === 'settings.md';
    });

    if (!settingsMdFile) {
      message.error('目录中未找到 SETTINGS.md 文件');
      return;
    }

    // 检查总大小
    const totalSize = Array.from(files).reduce((sum, f) => sum + f.size, 0);
    if (totalSize > 10 * 1024 * 1024) {
      message.error('目录总大小不能超过 10MB');
      return;
    }

    try {
      // 解析 SETTINGS.md 获取描述
      const settingsMdContent = await settingsMdFile.text();
      const metadata = parseSettingsMD(settingsMdContent);

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
        description: metadata.description || '',
        version: metadata.version || '',
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
    const validation = validateSettingsName(values.name);
    if (!validation.valid) {
      message.error(validation.message);
      return;
    }

    try {
      if (isAfterUpload && pendingZipBlobRef.current) {
        message.loading({ content: '正在创建Settings...', key: 'uploading' });

        const formData = new FormData();
        formData.append('file', pendingZipBlobRef.current, 'settings.zip');
        formData.append('name', values.name);
        formData.append('description', values.description || '');
        formData.append('version', values.version || '');

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

  // 打开Agent绑定Modal
  const handleOpenBindModal = async () => {
    setBindLoading(true);
    try {
      // 加载所有Settings供选择
      const result = await settingsApi.list({ pageSize: 100 });
      setAvailableSettings(result.data);
      setBindModalVisible(true);
    } catch (error) {
      message.error('加载Settings失败');
    } finally {
      setBindLoading(false);
    }
  };

  // 选择Agent后加载已绑定的Settings
  const handleAgentSelect = async (agentId: string) => {
    setSelectedAgentId(agentId);
    setBindLoading(true);
    try {
      const result = await settingsApi.getAgentSettings(agentId);
      setBoundSettings(result.settings || []);
      setSelectedSettingsIds((result.settings || []).map(s => s.id));
    } catch (error) {
      message.error('加载绑定Settings失败');
    } finally {
      setBindLoading(false);
    }
  };

  // 绑定Settings到Agent
  const handleBindSettings = async () => {
    if (!selectedAgentId) {
      message.error('请选择Agent');
      return;
    }
    setBindLoading(true);
    try {
      await settingsApi.bindToAgent(selectedAgentId, selectedSettingsIds);
      message.success('绑定成功');
      setBindModalVisible(false);
    } catch (error: any) {
      message.error(error.response?.data?.error || '绑定失败');
    } finally {
      setBindLoading(false);
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
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 100,
      render: (version: string) => version ? <Tag color="blue">{version}</Tag> : '-',
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
          <Button
            icon={<LinkOutlined />}
            onClick={handleOpenBindModal}
            loading={bindLoading}
          >
            绑定到 Agent
          </Button>
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
              目录需包含 SETTINGS.md 文件，最大 10MB
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
            extra="只允许小写字母、数字和中划线，如：claude-project-settings"
          >
            <Input placeholder="claude-project-settings" disabled={isAfterUpload} />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="Settings描述" />
          </Form.Item>

          <Form.Item name="version" label="版本">
            <Input placeholder="如：1.0.0" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Agent绑定弹窗 */}
      <Modal
        title="绑定 Settings 到 Agent"
        open={bindModalVisible}
        onOk={handleBindSettings}
        onCancel={() => setBindModalVisible(false)}
        width={700}
        okText="绑定"
        confirmLoading={bindLoading}
      >
        <Form layout="vertical">
          <Form.Item label="选择 Agent">
            <Select
              placeholder="选择要绑定Settings的Agent"
              style={{ width: '100%' }}
              value={selectedAgentId || undefined}
              onChange={handleAgentSelect}
              options={agents.map(a => ({ label: a.name, value: a.id }))}
            />
          </Form.Item>

          {selectedAgentId && (
            <Form.Item label="选择 Settings">
              <Spin spinning={bindLoading}>
                <Select
                  mode="multiple"
                  placeholder="选择要绑定的Settings"
                  style={{ width: '100%' }}
                  value={selectedSettingsIds}
                  onChange={setSelectedSettingsIds}
                  options={availableSettings.map(s => ({ label: s.name, value: s.id }))}
                />
                {boundSettings.length > 0 && (
                  <div style={{ marginTop: 8 }}>
                    <Text type="secondary">当前已绑定：{boundSettings.length} 个Settings</Text>
                  </div>
                )}
              </Spin>
            </Form.Item>
          )}
        </Form>
      </Modal>
    </div>
  );
};

export default SettingsManagement;