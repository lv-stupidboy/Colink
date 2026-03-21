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
  Drawer,
  List,
  Empty,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  DatabaseOutlined,
  SearchOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import api from '@/api/client';
import type { KnowledgeBase, CreateKnowledgeBaseRequest, KnowledgeBaseType, KnowledgeBaseStatus, KnowledgeSnippet } from '@/types';

const { Text, Title, Paragraph } = Typography;
const { Option } = Select;
const { TextArea } = Input;

/**
 * 知识库管理页面
 */
const KnowledgeManagement: React.FC = () => {
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBase[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingKb, setEditingKb] = useState<KnowledgeBase | null>(null);
  const [queryDrawerVisible, setQueryDrawerVisible] = useState(false);
  const [queryingKb, setQueryingKb] = useState<KnowledgeBase | null>(null);
  const [queryInput, setQueryInput] = useState('');
  const [queryResults, setQueryResults] = useState<KnowledgeSnippet[]>([]);
  const [queryLoading, setQueryLoading] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    loadKnowledgeBases();
  }, [page, pageSize]);

  const loadKnowledgeBases = async () => {
    setLoading(true);
    try {
      const response = await api.knowledge.list({ page, size: pageSize });
      setKnowledgeBases(response.data || []);
      setTotal(response.total || 0);
    } catch (error) {
      message.error('加载知识库列表失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingKb(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (kb: KnowledgeBase) => {
    setEditingKb(kb);
    form.setFieldsValue({
      name: kb.name,
      displayName: kb.displayName,
      description: kb.description,
      type: kb.type,
      queryEndpoint: kb.queryEndpoint,
      status: kb.status,
    });
    setModalVisible(true);
  };

  const handleSubmit = async (values: any) => {
    try {
      if (editingKb) {
        await api.knowledge.update(editingKb.id, values);
        message.success('知识库更新成功');
      } else {
        await api.knowledge.create(values as CreateKnowledgeBaseRequest);
        message.success('知识库创建成功');
      }
      setModalVisible(false);
      loadKnowledgeBases();
    } catch (error: any) {
      message.error(error.response?.data?.error || '操作失败');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.knowledge.delete(id);
      message.success('知识库已删除');
      loadKnowledgeBases();
    } catch (error: any) {
      message.error(error.response?.data?.error || '删除失败');
    }
  };

  const handleQuery = (kb: KnowledgeBase) => {
    setQueryingKb(kb);
    setQueryInput('');
    setQueryResults([]);
    setQueryDrawerVisible(true);
  };

  const executeQuery = async () => {
    if (!queryInput.trim() || !queryingKb) return;

    setQueryLoading(true);
    try {
      const result = await api.knowledge.query(queryingKb.id, { query: queryInput, limit: 10 });
      setQueryResults(result.results || []);
      if (result.error) {
        message.warning(result.error);
      }
    } catch (error: any) {
      message.error(error.response?.data?.error || '查询失败');
    } finally {
      setQueryLoading(false);
    }
  };

  const getTypeTag = (type: KnowledgeBaseType) => {
    const typeConfig: Record<KnowledgeBaseType, { color: string; text: string }> = {
      git: { color: 'blue', text: 'Git' },
      mcp: { color: 'green', text: 'MCP' },
      api: { color: 'orange', text: 'API' },
    };
    const config = typeConfig[type] || { color: 'default', text: type };
    return <Tag color={config.color}>{config.text}</Tag>;
  };

  const getStatusBadge = (status: KnowledgeBaseStatus) => {
    return status === 'active' ? (
      <Badge status="success" text="活跃" />
    ) : (
      <Badge status="default" text="停用" />
    );
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: (name: string, record: KnowledgeBase) => (
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
      width: 80,
      render: (type: KnowledgeBaseType) => getTypeTag(type),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (desc: string) => desc || '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 80,
      render: (status: KnowledgeBaseStatus) => getStatusBadge(status),
    },
    {
      title: '查询统计',
      key: 'stats',
      width: 150,
      render: (_: unknown, record: KnowledgeBase) => (
        <Space>
          <Text>{record.queryCount} 次查询</Text>
          {record.lastQueryAt && (
            <Tooltip title={`最后查询: ${new Date(record.lastQueryAt).toLocaleString()}`}>
              <ClockCircleOutlined style={{ color: '#8c8c8c' }} />
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 180,
      render: (_: unknown, record: KnowledgeBase) => (
        <Space>
          <Tooltip title="查询">
            <Button
              type="link"
              icon={<SearchOutlined />}
              onClick={() => handleQuery(record)}
              disabled={record.status !== 'active'}
            />
          </Tooltip>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          />
          <Popconfirm
            title="确定要删除此知识库吗？"
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
    <div className="knowledge-management" style={{ padding: 24 }}>
      <Card>
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
          <Title level={4} style={{ margin: 0 }}>
            <DatabaseOutlined style={{ marginRight: 8 }} />
            知识库管理
          </Title>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            新建知识库
          </Button>
        </div>

        <Table
          dataSource={knowledgeBases}
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
        title={editingKb ? '编辑知识库' : '新建知识库'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{ status: 'active', type: 'mcp' }}
        >
          <Form.Item
            name="name"
            label="标识名称"
            rules={[
              { required: true, message: '请输入标识名称' },
              { pattern: /^[a-z0-9_-]+$/, message: '只能包含小写字母、数字、下划线和连字符' },
            ]}
          >
            <Input placeholder="例如: tech-docs" disabled={!!editingKb} />
          </Form.Item>
          <Form.Item name="displayName" label="显示名称">
            <Input placeholder="技术文档库" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <TextArea rows={2} placeholder="知识库描述" />
          </Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select disabled={!!editingKb}>
              <Option value="mcp">MCP (Model Context Protocol)</Option>
              <Option value="api">API</Option>
              <Option value="git">Git 仓库</Option>
            </Select>
          </Form.Item>
          <Form.Item
            name="queryEndpoint"
            label="查询端点"
            extra="MCP 服务地址或 API 端点 URL"
          >
            <Input placeholder="http://localhost:3000/mcp" />
          </Form.Item>
          {editingKb && (
            <Form.Item name="status" label="状态">
              <Select>
                <Option value="active">活跃</Option>
                <Option value="inactive">停用</Option>
              </Select>
            </Form.Item>
          )}
        </Form>
      </Modal>

      <Drawer
        title={`查询知识库: ${queryingKb?.displayName || queryingKb?.name}`}
        placement="right"
        width={600}
        onClose={() => setQueryDrawerVisible(false)}
        open={queryDrawerVisible}
      >
        <Space.Compact style={{ width: '100%', marginBottom: 16 }}>
          <Input
            placeholder="输入查询内容"
            value={queryInput}
            onChange={(e) => setQueryInput(e.target.value)}
            onPressEnter={executeQuery}
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={executeQuery} loading={queryLoading}>
            查询
          </Button>
        </Space.Compact>

        {queryResults.length > 0 ? (
          <List
            dataSource={queryResults}
            renderItem={(item, index) => (
              <List.Item key={index}>
                <Card size="small" style={{ width: '100%' }}>
                  {item.title && <Text strong>{item.title}</Text>}
                  <Paragraph ellipsis={{ rows: 3 }} style={{ marginTop: 8, marginBottom: 0 }}>
                    {item.content}
                  </Paragraph>
                  <Space style={{ marginTop: 8 }}>
                    <Tag>{item.source}</Tag>
                    {item.relevance !== undefined && (
                      <Text type="secondary">相关度: {(item.relevance * 100).toFixed(1)}%</Text>
                    )}
                  </Space>
                </Card>
              </List.Item>
            )}
          />
        ) : (
          !queryLoading && (
            <Empty description="输入查询内容并点击查询按钮" image={Empty.PRESENTED_IMAGE_SIMPLE} />
          )
        )}
      </Drawer>
    </div>
  );
};

export default KnowledgeManagement;