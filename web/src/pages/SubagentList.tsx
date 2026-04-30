import React, { useEffect, useState, useCallback, useRef } from 'react';
import {
  Card, Button, Modal, Form, Input, message, Space, Typography, Tag,
  Popconfirm, Empty, Spin, Pagination, Table, Tooltip, Radio, Select
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  EyeOutlined,
  CloudUploadOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { Subagent, SubagentListResponse, Skill } from '@/types';

const { Title, Text, Paragraph } = Typography;
const { TextArea } = Input;

// 解析 .md 文件提取描述
const parseSubagentMD = (content: string): { description: string } => {
  let description = '';

  // 1. 首先尝试解析 YAML front matter
  const frontMatterMatch = content.match(/^---\s*\n([\s\S]*?)\n---/);
  if (frontMatterMatch) {
    const frontMatter = frontMatterMatch[1];
    const descMatch = frontMatter.match(/description:\s*(.+)/i);
    if (descMatch) {
      description = descMatch[1].trim();
      // 移除引号
      description = description.replace(/^["']|["']$/g, '');
    }
  }

  // 2. 如果没有从 front matter 获取到，尝试从 ## Description 获取
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

  return { description };
};

// 清理名称格式
const cleanName = (name: string): string => {
  if (!name) return '';
  let cleaned = name.toLowerCase().replace(/[^a-z0-9-]/g, '-');
  cleaned = cleaned.replace(/^-+|-+$/g, '');
  // 确保以字母开头
  if (cleaned && !/^[a-z]/.test(cleaned)) {
    cleaned = 's-' + cleaned;
  }
  return cleaned;
};

// 根据子代理名称生成头像
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
    '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16', '#52c41a',
    '#1890ff', '#2f54eb', '#faad14', '#a0d911', '#f5222d'
  ];
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash);
  }
  const color = colors[Math.abs(hash) % colors.length];

  return { initials, color };
};

// 子代理头像组件
const SubagentAvatar: React.FC<{ name: string }> = ({ name }) => {
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

const SubagentList: React.FC = () => {
  const [subagents, setSubagents] = useState<Subagent[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [modalVisible, setModalVisible] = useState(false);
  const [viewModalVisible, setViewModalVisible] = useState(false);
  const [editingSubagent, setEditingSubagent] = useState<Subagent | null>(null);
  const [viewingSubagent, setViewingSubagent] = useState<Subagent | null>(null);
  const [searchText, setSearchText] = useState('');
  const [form] = Form.useForm();
  const [createMethod, setCreateMethod] = useState<'upload' | 'manual'>('upload');
  const [isAfterUpload, setIsAfterUpload] = useState(false);
  // 技能绑定相关
  const [skills, setSkills] = useState<Skill[]>([]);
  const [selectedSkillIds, setSelectedSkillIds] = useState<string[]>([]);
  const [subagentSkillCounts, setSubagentSkillCounts] = useState<Record<string, number>>({});
  const [subagentSkillsMap, setSubagentSkillsMap] = useState<Record<string, Skill[]>>({});

  // Agent 类型选项
  const agentTypeOptions = [
    { label: 'Claude Code', value: 'claude_code', color: 'blue' },
    { label: 'OpenCode', value: 'open_code', color: 'green' },
  ];

  // 存储解析后的内容，用户确认后才上传
  const pendingContentRef = useRef<string>('');

  const loadSubagents = useCallback(async () => {
    setLoading(true);
    try {
      const result: SubagentListResponse = await api.subagents.list({
        search: searchText,
        page,
        pageSize,
      });
      setSubagents(result.data || []);
      setTotal(result.total || 0);

      // 加载每个子代理的关联技能
      const counts: Record<string, number> = {};
      const skillsMap: Record<string, Skill[]> = {};
      for (const sub of result.data || []) {
        try {
          const skillsRes = await api.subagents.getSkills(sub.id);
          counts[sub.id] = skillsRes.count || 0;
          skillsMap[sub.id] = skillsRes.skills || [];
        } catch {
          counts[sub.id] = 0;
          skillsMap[sub.id] = [];
        }
      }
      setSubagentSkillCounts(counts);
      setSubagentSkillsMap(skillsMap);
    } catch (error) {
      message.error('加载子代理列表失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText]);

  useEffect(() => {
    loadSubagents();
  }, [loadSubagents]);

  // 加载技能列表
  const loadSkills = async () => {
    try {
      const result = await api.skills.list({ pageSize: 100 });
      setSkills(result.data || []);
    } catch (error) {
      console.error('加载技能列表失败', error);
      setSkills([]);
    }
  };

  // 加载子代理绑定的技能
  const loadSubagentSkills = async (subagentId: string) => {
    try {
      const result = await api.subagents.getSkills(subagentId);
      setSelectedSkillIds(result.skills?.map(s => s.id) || []);
    } catch (error) {
      console.error('加载子代理绑定的技能失败', error);
      setSelectedSkillIds([]);
    }
  };

  // 初始加载技能列表
  useEffect(() => {
    loadSkills();
  }, []);

  const handleCreate = () => {
    setEditingSubagent(null);
    setCreateMethod('upload');
    setIsAfterUpload(false);
    pendingContentRef.current = '';
    setSelectedSkillIds([]);
    form.resetFields();
    form.setFieldsValue({ supportedAgents: [] });
    setModalVisible(true);
  };

  const handleEdit = async (record: Subagent) => {
    setEditingSubagent(record);
    setCreateMethod('manual');
    setIsAfterUpload(false);
    pendingContentRef.current = '';
    form.setFieldsValue({
      name: record.name,
      description: record.description,
      content: record.content,
      supportedAgents: record.supportedAgents || [],
    });
    // 加载已绑定的技能
    await loadSubagentSkills(record.id);
    setModalVisible(true);
  };

  const handleView = (record: Subagent) => {
    setViewingSubagent(record);
    setViewModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.subagents.delete(id);
      message.success('删除成功');
      loadSubagents();
    } catch (error: any) {
      const errorData = error.response?.data;
      if (errorData?.error) {
        message.error(errorData.error);
      } else {
        message.error('删除失败');
      }
    }
  };

  const handleSubmit = async (values: any) => {
    try {
      if (editingSubagent) {
        await api.subagents.update(editingSubagent.id, {
          description: values.description,
          content: values.content,
          supportedAgents: values.supportedAgents,
        });
        // 更新技能绑定
        await api.subagents.bindSkills(editingSubagent.id, selectedSkillIds);
        message.success('更新成功');
      } else {
        // 新建时，使用 pendingContentRef 或表单中的 content
        const content = isAfterUpload ? pendingContentRef.current : values.content;
        const newSubagent = await api.subagents.create({
          name: values.name,
          description: values.description,
          content: content,
          supportedAgents: values.supportedAgents,
        });
        // 为新创建的子代理绑定技能
        if (selectedSkillIds.length > 0) {
          await api.subagents.bindSkills(newSubagent.id, selectedSkillIds);
        }
        message.success('创建成功');
      }
      setModalVisible(false);
      loadSubagents();
    } catch (error: any) {
      const errorData = error.response?.data;
      if (errorData?.error) {
        message.error(errorData.error);
      } else {
        message.error('操作失败');
      }
    }
  };

  const handleSearch = (value: string) => {
    setSearchText(value);
    setPage(1);
  };

  // 上传处理 - 前端解析，不直接入库
  const handleFileSelect = async (file: File) => {
    // 检查文件格式
    const isValid = file.name.endsWith('.md') || file.name.endsWith('.zip');
    if (!isValid) {
      message.error('只支持 .md 和 .zip 格式的文件');
      return false;
    }
    // 检查文件大小
    if (file.size / 1024 / 1024 > 2) {
      message.error('文件大小不能超过 2MB');
      return false;
    }

    try {
      // 目前只处理 .md 文件
      if (file.name.endsWith('.md')) {
        // 读取文件内容
        const content = await file.text();
        const metadata = parseSubagentMD(content);

        // 从文件名提取名称（去掉 .md 后缀）
        const fileName = file.name.replace(/\.md$/i, '');
        const name = cleanName(fileName);

        // 存储解析后的内容
        pendingContentRef.current = content;

        // 设置表单值，显示给用户确认
        setIsAfterUpload(true);
        form.setFieldsValue({
          name: name,
          description: metadata.description,
          content: content,
        });

        message.success('文件解析成功，请确认后保存');
      } else {
        // .zip 文件暂不支持前端解析，提示用户
        message.warning('.zip 格式暂不支持预览解析，请使用手动填写');
        return false;
      }
      return false;
    } catch (error) {
      message.error('读取文件失败');
      return false;
    }
  };

  // 表格列定义
  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: (name: string) => (
        <Space>
          <SubagentAvatar name={name} />
          <Text strong style={{ fontSize: 14 }}>{name}</Text>
        </Space>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      width: 200,
      ellipsis: true,
      render: (description: string) => (
        <Tooltip title={description} placement="topLeft">
          <Text type="secondary" style={{ maxWidth: 180, display: 'inline-block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {description || '暂无描述'}
          </Text>
        </Tooltip>
      ),
    },
    {
      title: '关联 Skills',
      dataIndex: 'id',
      key: 'skillCount',
      width: 200,
      render: (id: string) => {
        const count = subagentSkillCounts[id] || 0;
        const skillList = subagentSkillsMap[id] || [];
        if (count === 0) {
          return <Tag>0 个</Tag>;
        }
        return (
          <Tooltip title={skillList.map(s => s.name).join('、')}>
            <Tag color="blue" style={{ cursor: 'pointer' }}>
              {count} 个 Skills
            </Tag>
          </Tooltip>
        );
      },
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
      width: 160,
      render: (date: string) => (
        <Text type="secondary" style={{ fontSize: 12 }}>
          {date ? new Date(date).toLocaleString('zh-CN') : '-'}
        </Text>
      ),
    },
    {
      title: '操作',
      key: 'action',
      width: 220,
      fixed: 'right' as const,
      render: (_: any, record: Subagent) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => handleView(record)}
          >
            查看
          </Button>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这个 Subagent 吗？"
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
      {/* 页面标题 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>Subagents 管理</Title>
          <Text type="secondary">管理可复用的 Subagent 配置</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新建 Subagent
        </Button>
      </div>

      {/* 搜索区域 */}
      <Card style={{ marginBottom: 16 }} styles={{ body: { padding: '12px 16px' } }}>
        <Input.Search
          placeholder="搜索 Subagents..."
          allowClear
          style={{ width: 300 }}
          onSearch={handleSearch}
          enterButton
        />
      </Card>

      {/* 数据表格 */}
      <Card>
        <Spin spinning={loading}>
          <Table
            dataSource={subagents}
            columns={columns}
            rowKey="id"
            pagination={false}
            locale={{
              emptyText: (
                <Empty
                  description="暂无 Subagents"
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                />
              ),
            }}
          />
        </Spin>

        {/* 分页 */}
        <div style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-end' }}>
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
      </Card>

      {/* 新建/编辑弹窗 */}
      <Modal
        title={editingSubagent ? '编辑 Subagent' : '新建 Subagent'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={700}
        okText="保存"
        destroyOnClose
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
        >
          <Form.Item
            name="name"
            label="名称"
            rules={[
              { required: true, message: '请输入名称' },
              { pattern: /^[a-z][a-z0-9-]*$/, message: '名称只能包含小写字母、数字和中划线，且必须以字母开头' }
            ]}
            extra="只允许小写字母、数字和中划线，如：code-reviewer"
          >
            <Input
              placeholder="如：code-reviewer"
              disabled={!!editingSubagent || isAfterUpload}
            />
          </Form.Item>

          {/* 创建方式选择 - 仅新建时显示 */}
          {!editingSubagent && !isAfterUpload && (
            <div style={{ marginBottom: 16, padding: 16, background: 'var(--bg-container)', borderRadius: 8, border: '1px solid var(--border-color)' }}>
              <Text strong style={{ marginRight: 12 }}>创建方式：</Text>
              <Radio.Group
                value={createMethod}
                onChange={(e) => setCreateMethod(e.target.value)}
              >
                <Radio.Button value="manual">
                  <EditOutlined /> 手动填写
                </Radio.Button>
                <Radio.Button value="upload">
                  <CloudUploadOutlined /> 本地上传
                </Radio.Button>
              </Radio.Group>

              {createMethod === 'upload' && (
                <div style={{ marginTop: 12 }}>
                  <input
                    type="file"
                    accept=".md,.zip"
                    style={{ display: 'none' }}
                    id="subagent-file-input"
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file) {
                        handleFileSelect(file);
                      }
                      // 重置 input，允许选择相同文件
                      e.target.value = '';
                    }}
                  />
                  <div
                    onClick={() => document.getElementById('subagent-file-input')?.click()}
                    style={{
                      border: '1px dashed var(--ant-color-border)',
                      borderRadius: 8,
                      padding: 24,
                      textAlign: 'center',
                      cursor: 'pointer',
                      transition: 'border-color 0.3s',
                    }}
                    onMouseEnter={(e) => e.currentTarget.style.borderColor = 'var(--ant-color-primary)'}
                    onMouseLeave={(e) => e.currentTarget.style.borderColor = 'var(--ant-color-border)'}
                  >
                    <CloudUploadOutlined style={{ fontSize: 32, color: 'var(--ant-color-primary)' }} />
                    <p style={{ marginTop: 8 }}>点击或拖拽文件到此区域上传</p>
                    <p style={{ fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
                      支持 .md 格式，最大 2MB
                    </p>
                  </div>
                </div>
              )}
            </div>
          )}

          <Form.Item
            name="description"
            label="描述"
          >
            <Input.TextArea
              rows={2}
              placeholder="简要描述这个 Subagent 的用途"
            />
          </Form.Item>

          <Form.Item
            name="content"
            label="配置内容"
            rules={[{ required: true, message: '请输入配置内容' }]}
            extra="Subagent 的完整配置内容，支持 Markdown 格式"
          >
            <TextArea
              rows={12}
              placeholder="输入 Subagent 的配置内容..."
              style={{ fontFamily: 'monospace' }}
            />
          </Form.Item>

          <Form.Item
            label="兼容 Agent 类型"
            name="supportedAgents"
            extra="选择此 Subagent 支持的 Agent 类型"
            rules={[{ required: true, message: '请至少选择一种 Agent 类型' }]}
          >
            <Select
              mode="multiple"
              placeholder="选择支持的 Agent 类型"
              style={{ width: '100%' }}
              options={agentTypeOptions}
            />
          </Form.Item>

          <Form.Item label="绑定 Skills">
            <Select
              mode="multiple"
              placeholder="选择要绑定的 Skill"
              value={selectedSkillIds}
              onChange={setSelectedSkillIds}
              style={{ width: '100%' }}
              optionLabelProp="label"
              showSearch
              filterOption={(input, option) =>
                (option?.label as string)?.toLowerCase().includes(input.toLowerCase()) ||
                (option?.desc as string)?.toLowerCase().includes(input.toLowerCase())
              }
              options={skills.map(s => ({
                label: s.name,
                value: s.id,
                desc: s.description || '暂无描述',
              }))}
              optionRender={(option) => (
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                  <span style={{ fontWeight: 500 }}>{option.label}</span>
                  <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 400 }}>
                    {option.data?.desc}
                  </span>
                </div>
              )}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 查看详情弹窗 */}
      <Modal
        title={
          <Space>
            <SubagentAvatar name={viewingSubagent?.name || ''} />
            <span>{viewingSubagent?.name}</span>
          </Space>
        }
        open={viewModalVisible}
        onCancel={() => setViewModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setViewModalVisible(false)}>
            关闭
          </Button>,
          <Button
            key="edit"
            type="primary"
            icon={<EditOutlined />}
            onClick={() => {
              setViewModalVisible(false);
              if (viewingSubagent) {
                handleEdit(viewingSubagent);
              }
            }}
          >
            编辑
          </Button>,
        ]}
        width={700}
      >
        {viewingSubagent && (
          <div>
            <Paragraph>
              <Text strong>描述：</Text>
              <br />
              <Text type="secondary">{viewingSubagent.description || '暂无描述'}</Text>
            </Paragraph>

            <Paragraph>
              <Text strong>配置内容：</Text>
            </Paragraph>
            <div
              style={{
                background: 'var(--bg-container)',
                border: '1px solid var(--border-color)',
                borderRadius: 8,
                padding: 12,
                maxHeight: 400,
                overflow: 'auto',
              }}
            >
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontFamily: 'monospace', fontSize: 13, background: 'transparent' }}>
                {viewingSubagent.content}
              </pre>
            </div>

            <div style={{ marginTop: 16, color: 'var(--ant-color-text-secondary)', fontSize: 12 }}>
              <Text type="secondary">
                创建时间：{viewingSubagent.createdAt ? new Date(viewingSubagent.createdAt).toLocaleString('zh-CN') : '-'}
              </Text>
              <br />
              <Text type="secondary">
                更新时间：{viewingSubagent.updatedAt ? new Date(viewingSubagent.updatedAt).toLocaleString('zh-CN') : '-'}
              </Text>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default SubagentList;