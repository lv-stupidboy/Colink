import React, { useEffect, useState, useCallback, useRef } from 'react';
import {
  Card, Button, Modal, Form, Input, message, Space, Typography, Tag,
  Popconfirm, Empty, Spin, Pagination, Table, Tooltip, Radio, Select
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  CodeOutlined,
  EyeOutlined,
  CloudUploadOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import { getTypeColorByIndex } from '@/config/agentTypeColors';
import type { Command, CommandListResponse, Skill, BaseAgentTypeInfo } from '@/types';

const { Title, Text, Paragraph } = Typography;

// 解析 .md 文件提取描述
const parseCommandMD = (content: string): { description: string } => {
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
    cleaned = 'c-' + cleaned;
  }
  return cleaned;
};

const CommandList: React.FC = () => {
  const [commands, setCommands] = useState<Command[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [modalVisible, setModalVisible] = useState(false);
  const [viewModalVisible, setViewModalVisible] = useState(false);
  const [editingCommand, setEditingCommand] = useState<Command | null>(null);
  const [viewingCommand, setViewingCommand] = useState<Command | null>(null);
  const [searchText, setSearchText] = useState('');
  const [form] = Form.useForm();
  const [createMethod, setCreateMethod] = useState<'upload' | 'manual'>('upload');
  const [isAfterUpload, setIsAfterUpload] = useState(false);
  const [allSkills, setAllSkills] = useState<Skill[]>([]);
  const [selectedSkillIds, setSelectedSkillIds] = useState<string[]>([]);
  const [commandSkillCounts, setCommandSkillCounts] = useState<Record<string, number>>({});
  const [commandSkillsMap, setCommandSkillsMap] = useState<Record<string, Skill[]>>({});
  const [agentTypes, setAgentTypes] = useState<BaseAgentTypeInfo[]>([]);

  // 存储解析后的内容，用户确认后才上传
  const pendingContentRef = useRef<string>('');
  const pendingFileNameRef = useRef<string>('');

  const loadCommands = useCallback(async () => {
    setLoading(true);
    try {
      const result: CommandListResponse = await api.commands.list({
        search: searchText,
        page,
        pageSize,
      });
      setCommands(result.data || []);
      setTotal(result.total || 0);

      // 加载每个 command 的关联技能数
      const counts: Record<string, number> = {};
      const skillsMap: Record<string, Skill[]> = {};
      for (const cmd of result.data || []) {
        try {
          const skillsRes = await api.commands.getSkills(cmd.id);
          counts[cmd.id] = skillsRes.count || 0;
          skillsMap[cmd.id] = skillsRes.skills || [];
        } catch {
          counts[cmd.id] = 0;
          skillsMap[cmd.id] = [];
        }
      }
      setCommandSkillCounts(counts);
      setCommandSkillsMap(skillsMap);
    } catch (error) {
      message.error('加载命令列表失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText]);

  const loadAllSkills = useCallback(async () => {
    try {
      const result = await api.skills.list({ pageSize: 100 });
      setAllSkills(result.data || []);
    } catch (error) {
      console.error('加载技能列表失败', error);
    }
  }, []);

  useEffect(() => {
    loadCommands();
    api.baseAgents.getTypes().then(setAgentTypes).catch(() => {});
  }, [loadCommands]);

  useEffect(() => {
    loadAllSkills();
  }, [loadAllSkills]);

  const handleCreate = () => {
    setEditingCommand(null);
    setCreateMethod('upload');
    setIsAfterUpload(false);
    pendingContentRef.current = '';
    pendingFileNameRef.current = '';
    setSelectedSkillIds([]);
    form.resetFields();
    form.setFieldsValue({ supportedAgents: [] });
    setModalVisible(true);
  };

  const handleEdit = async (record: Command) => {
    setEditingCommand(record);
    setCreateMethod('manual');
    setIsAfterUpload(false);
    pendingContentRef.current = '';
    pendingFileNameRef.current = '';
    form.setFieldsValue({
      name: record.name,
      description: record.description,
      supportedAgents: record.supportedAgents || [],
    });
    // 加载已绑定的技能
    try {
      const result = await api.commands.getSkills(record.id);
      setSelectedSkillIds((result.skills || []).map((s: Skill) => s.id));
    } catch {
      setSelectedSkillIds([]);
    }
    setModalVisible(true);
  };

  const handleView = (record: Command) => {
    setViewingCommand(record);
    setViewModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.commands.delete(id);
      message.success('删除成功');
      loadCommands();
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
      if (editingCommand) {
        await api.commands.update(editingCommand.id, {
          description: values.description,
          supportedAgents: values.supportedAgents,
        });
        // 全量更新技能绑定（传空数组表示清空绑定）
        await api.commands.bindSkills(editingCommand.id, selectedSkillIds);
        message.success('更新成功');
      } else {
        // 新建时，如果是上传模式，带上 content
        const newCommand = await api.commands.create({
          name: values.name,
          description: values.description,
          content: isAfterUpload ? pendingContentRef.current : undefined,
          supportedAgents: values.supportedAgents,
        });
        // 绑定技能（如果有选择）
        if (selectedSkillIds.length > 0) {
          await api.commands.bindSkills(newCommand.id, selectedSkillIds);
        }
        message.success('创建成功');
      }
      setModalVisible(false);
      loadCommands();
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
    if (!file.name.endsWith('.md')) {
      message.error('只支持 .md 格式的文件');
      return false;
    }
    // 检查文件大小
    if (file.size / 1024 / 1024 > 2) {
      message.error('文件大小不能超过 2MB');
      return false;
    }

    try {
      // 读取文件内容
      const content = await file.text();
      const metadata = parseCommandMD(content);

      // 从文件名提取名称（去掉 .md 后缀）
      const fileName = file.name.replace(/\.md$/i, '');
      const name = cleanName(fileName);

      // 存储解析后的内容
      pendingContentRef.current = content;
      pendingFileNameRef.current = name;

      // 设置表单值，显示给用户确认
      setIsAfterUpload(true);
      form.setFieldsValue({
        name: name,
        description: metadata.description,
        content: content,
      });

      message.success('文件解析成功，请确认后保存');
      return false; // 阻止 Upload 组件的自动上传
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
          <CodeOutlined style={{ color: 'var(--ant-color-primary)', fontSize: 16 }} />
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
        const count = commandSkillCounts[id] || 0;
        const skills = commandSkillsMap[id] || [];
        if (count === 0) {
          return <Tag>0 个</Tag>;
        }
        return (
          <Tooltip title={skills.map(s => s.name).join('、')}>
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
          return <Tag color="blue">默认</Tag>;
        }
        return (
          <Space size="small">
            {supportedAgents.map(agent => {
              const typeInfo = agentTypes.find(t => t.type === agent);
              const color = getTypeColorByIndex(agentTypes, agent);
              return (
                <Tag key={agent} color={color}>
                  {typeInfo?.name || agent}
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
      render: (_: any, record: Command) => (
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
            title="确定要删除这个 Command 吗？"
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
          <Title level={2} style={{ margin: 0 }}>Commands管理</Title>
          <Text type="secondary">管理可复用的 Commands</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新建 Command
        </Button>
      </div>

      {/* 搜索区域 */}
      <Card style={{ marginBottom: 16 }} styles={{ body: { padding: '12px 16px' } }}>
        <Input.Search
          placeholder="搜索 Commands..."
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
            dataSource={commands}
            columns={columns}
            rowKey="id"
            pagination={false}
            locale={{
              emptyText: (
                <Empty
                  description="暂无 Commands"
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
        title={editingCommand ? '编辑 Command' : '新建 Command'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={600}
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
            extra="只允许小写字母、数字和中划线，如：code-review"
          >
            <Input
              placeholder="如：code-review"
              disabled={!!editingCommand || isAfterUpload}
            />
          </Form.Item>

          {/* 创建方式选择 - 仅新建时显示 */}
          {!editingCommand && !isAfterUpload && (
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
                    accept=".md"
                    style={{ display: 'none' }}
                    id="command-file-input"
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
                    onClick={() => document.getElementById('command-file-input')?.click()}
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
              rows={3}
              placeholder="简要描述这个 Command 的用途"
            />
          </Form.Item>

          <Form.Item
            name="content"
            label="Command 内容"
            extra="Command 的具体内容，保存在文件中"
          >
            <Input.TextArea
              rows={10}
              placeholder="输入 Command 的具体内容..."
              style={{ fontFamily: 'monospace' }}
              disabled={isAfterUpload}
            />
          </Form.Item>

          <Form.Item
            label="兼容 Agent 类型"
            name="supportedAgents"
            extra="选择此 Command 支持的 Agent 类型"
            rules={[{ required: true, message: '请至少选择一种 Agent 类型' }]}
          >
            <Select
              mode="multiple"
              placeholder="选择支持的 Agent 类型"
              style={{ width: '100%' }}
              options={agentTypes.map(t => ({ label: t.name, value: t.type }))}
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
              options={allSkills.map(s => ({
                label: `${s.name} (${s.description || '暂无描述'})`,
                value: s.id,
                desc: s.description || '暂无描述',
              }))}
              optionRender={(option) => (
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontWeight: 500 }}>{option.data?.label?.split(' (')[0]}</span>
                  <span style={{ fontSize: 12, color: 'var(--text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 300 }}>
                    ({option.data?.desc})
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
            <CodeOutlined style={{ color: 'var(--ant-color-primary)' }} />
            <span>{viewingCommand?.name}</span>
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
              if (viewingCommand) {
                handleEdit(viewingCommand);
              }
            }}
          >
            编辑
          </Button>,
        ]}
        width={600}
      >
        {viewingCommand && (
          <div>
            <Paragraph>
              <Text strong>名称：</Text>
              <br />
              <Text>{viewingCommand.name}</Text>
            </Paragraph>

            <Paragraph>
              <Text strong>描述：</Text>
              <br />
              <Text type="secondary">{viewingCommand.description || '暂无描述'}</Text>
            </Paragraph>

            <Paragraph>
              <Text strong>Command 内容：</Text>
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
                {viewingCommand.content || '暂无内容'}
              </pre>
            </div>

            <div style={{ marginTop: 16, color: 'var(--ant-color-text-secondary)', fontSize: 12 }}>
              <Text type="secondary">
                创建时间：{viewingCommand.createdAt ? new Date(viewingCommand.createdAt).toLocaleString('zh-CN') : '-'}
              </Text>
              <br />
              <Text type="secondary">
                更新时间：{viewingCommand.updatedAt ? new Date(viewingCommand.updatedAt).toLocaleString('zh-CN') : '-'}
              </Text>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default CommandList;