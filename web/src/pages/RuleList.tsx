import React, { useEffect, useState, useCallback, useRef } from 'react';
import {
  Card, Button, Modal, Form, Input, message, Space, Typography, Tag,
  Popconfirm, Empty, Spin, Pagination, Table, Tooltip, Radio, Select
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  SafetyOutlined,
  EyeOutlined,
  CloudUploadOutlined,
  TeamOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import { getTypeColorByIndex } from '@/config/agentTypeColors';
import type { Rule, RuleListResponse, BaseAgentTypeInfo, AssetAgentsResponse } from '@/types';

const { Title, Text, Paragraph } = Typography;

// 解析 .md 文件提取描述
const parseRuleMD = (content: string): { description: string } => {
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
    cleaned = 'r-' + cleaned;
  }
  return cleaned;
};

const RuleList: React.FC = () => {
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [modalVisible, setModalVisible] = useState(false);
  const [viewModalVisible, setViewModalVisible] = useState(false);
  const [editingRule, setEditingRule] = useState<Rule | null>(null);
  const [viewingRule, setViewingRule] = useState<Rule | null>(null);
  const [searchText, setSearchText] = useState('');
  const [form] = Form.useForm();
  const [createMethod, setCreateMethod] = useState<'upload' | 'manual'>('upload');
  const [isAfterUpload, setIsAfterUpload] = useState(false);
  // 存储解析后的内容，用户确认后才上传
  const pendingContentRef = useRef<string>('');
  const [agentTypes, setAgentTypes] = useState<BaseAgentTypeInfo[]>([]);
  const [submitLoading, setSubmitLoading] = useState(false);
  const [submitLoadingText, setSubmitLoadingText] = useState('');
  const [affectedAgents, setAffectedAgents] = useState<{ id: string; name: string }[]>([]);

  const loadRules = useCallback(async () => {
    setLoading(true);
    try {
      const result: RuleListResponse = await api.rules.list({
        search: searchText,
        page,
        pageSize,
      });
      setRules(result.data || []);
      setTotal(result.total || 0);
    } catch (error) {
      message.error('加载规约列表失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText]);

  useEffect(() => {
    loadRules();
    api.baseAgents.getTypes().then(setAgentTypes).catch(() => {});
  }, [loadRules]);

  const handleCreate = () => {
    setEditingRule(null);
    setCreateMethod('upload');
    setIsAfterUpload(false);
    pendingContentRef.current = '';
    form.resetFields();
    form.setFieldsValue({ supportedAgents: [] });
    setModalVisible(true);
  };

  const handleEdit = (record: Rule) => {
    setEditingRule(record);
    setCreateMethod('manual');
    setIsAfterUpload(false);
    pendingContentRef.current = '';
    form.setFieldsValue({
      name: record.name,
      description: record.description,
      supportedAgents: record.supportedAgents || [],
    });
    setModalVisible(true);
  };

  const handleView = (record: Rule) => {
    setViewingRule(record);
    setViewModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.rules.delete(id);
      message.success('删除成功');
      loadRules();
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
  const handleViewRefs = async (rule: Rule) => {
    try {
      const result: AssetAgentsResponse = await api.rules.getBoundAgents(rule.id);
      if (result.agents && result.agents.length > 0) {
        Modal.info({
          title: '角色引用',
          width: 500,
          content: (
            <div>
              <p>该 Rule 被以下 <strong>{result.count}</strong> 个角色引用：</p>
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
          content: <p>该 Rule 暂未被任何角色引用</p>,
        });
      }
    } catch (error) {
      message.error('查询引用失败');
    }
  };

  const handleSubmit = async (values: any) => {
    try {
      if (editingRule) {
        setSubmitLoading(true);
        setSubmitLoadingText('正在更新Rule配置...');
        setAffectedAgents([]);

        const startTime = Date.now();
        const result = await api.rules.update(editingRule.id, {
          description: values.description,
          supportedAgents: values.supportedAgents,
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
      } else {
        // 新建时，如果是上传模式，带上 content
        await api.rules.create({
          name: values.name,
          description: values.description,
          content: isAfterUpload ? pendingContentRef.current : undefined,
          supportedAgents: values.supportedAgents,
        });
        message.success('创建成功');
      }
      setModalVisible(false);
      loadRules();
    } catch (error: any) {
      setSubmitLoading(false);
      setSubmitLoadingText('');
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
      const metadata = parseRuleMD(content);

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
      width: 200,
      render: (name: string) => (
        <Space>
          <SafetyOutlined style={{ color: 'var(--ant-color-primary)', fontSize: 16 }} />
          <Text strong style={{ fontSize: 14 }}>{name}</Text>
        </Space>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      width: 300,
      ellipsis: true,
      render: (description: string) => (
        <Tooltip title={description} placement="topLeft">
          <Text type="secondary" style={{ maxWidth: 280, display: 'inline-block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {description || '暂无描述'}
          </Text>
        </Tooltip>
      ),
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
      width: 280,
      fixed: 'right' as const,
      render: (_: any, record: Rule) => (
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
            title="确定要删除这个 Rule 吗？"
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
      {/* 页面标题 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>Rules 管理</Title>
          <Text type="secondary">管理 Agent 行为 Rules</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新建 Rule
        </Button>
      </div>

      {/* 搜索区域 */}
      <Card style={{ marginBottom: 16 }} styles={{ body: { padding: '12px 16px' } }}>
        <Input.Search
          placeholder="搜索 Rules..."
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
            dataSource={rules}
            columns={columns}
            rowKey="id"
            pagination={false}
            locale={{
              emptyText: (
                <Empty
                  description="暂无 Rules"
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
        title={editingRule ? '编辑 Rule' : '新建 Rule'}
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
            extra="只允许小写字母、数字和中划线，如：code-style"
          >
            <Input
              placeholder="如：code-style"
              disabled={!!editingRule || isAfterUpload}
            />
          </Form.Item>

          {/* 创建方式选择 - 仅新建时显示 */}
          {!editingRule && !isAfterUpload && (
            <div style={{ marginBottom: 16, padding: 16, background: 'var(--ant-color-bg-container)', borderRadius: 8, border: '1px solid var(--ant-color-border)' }}>
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
                    id="rule-file-input"
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
                    onClick={() => document.getElementById('rule-file-input')?.click()}
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
              placeholder="简要描述这个 Rule 的内容"
            />
          </Form.Item>

          <Form.Item
            label="兼容 Agent 类型"
            name="supportedAgents"
            extra="选择此 Rule 支持的 Agent 类型"
            rules={[{ required: true, message: '请至少选择一种 Agent 类型' }]}
          >
            <Select
              mode="multiple"
              placeholder="选择支持的 Agent 类型"
              style={{ width: '100%' }}
              options={agentTypes.map(t => ({ label: t.name, value: t.type, color: t.color }))}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 查看详情弹窗 */}
      <Modal
        title={
          <Space>
            <SafetyOutlined style={{ color: 'var(--ant-color-primary)' }} />
            <span>{viewingRule?.name}</span>
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
              if (viewingRule) {
                handleEdit(viewingRule);
              }
            }}
          >
            编辑
          </Button>,
        ]}
        width={600}
      >
        {viewingRule && (
          <div>
            <Paragraph>
              <Text strong>名称：</Text>
              <br />
              <Text>{viewingRule.name}</Text>
            </Paragraph>

            <Paragraph>
              <Text strong>描述：</Text>
              <br />
              <Text type="secondary">{viewingRule.description || '暂无描述'}</Text>
            </Paragraph>

            <Paragraph>
              <Text strong>Rule 内容：</Text>
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
                {viewingRule.content || '暂无内容'}
              </pre>
            </div>

            <div style={{ marginTop: 16, color: 'var(--ant-color-text-secondary)', fontSize: 12 }}>
              <Text type="secondary">
                创建时间：{viewingRule.createdAt ? new Date(viewingRule.createdAt).toLocaleString('zh-CN') : '-'}
              </Text>
              <br />
              <Text type="secondary">
                更新时间：{viewingRule.updatedAt ? new Date(viewingRule.updatedAt).toLocaleString('zh-CN') : '-'}
              </Text>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default RuleList;