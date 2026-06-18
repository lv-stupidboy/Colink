import React, { useCallback, useEffect, useState } from 'react';
import { Button, Form, Input, Modal, Popconfirm, Select, Space, Table, Tag, Typography, message } from 'antd';
import { ApiOutlined, DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';
import api from '@/api/client';
import type { BaseAgentTypeInfo, MCPServer, MCPServerListResponse, MCPTransport } from '@/types';
import { getTypeColorByIndex } from '@/config/agentTypeColors';

const { Title, Text } = Typography;
const { TextArea } = Input;

const parseLines = (value?: string): string[] =>
  (value || '')
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);

const parseJSONMap = (value?: string): Record<string, string> => {
  if (!value?.trim()) return {};
  const parsed = JSON.parse(value);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('必须是 JSON 对象');
  }
  return parsed;
};

const stringifyMap = (value?: Record<string, string>) =>
  value && Object.keys(value).length > 0 ? JSON.stringify(value, null, 2) : '';

const MCPServerList: React.FC = () => {
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [agentTypes, setAgentTypes] = useState<BaseAgentTypeInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [submitLoading, setSubmitLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editing, setEditing] = useState<MCPServer | null>(null);
  const [searchText, setSearchText] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [transport, setTransport] = useState<MCPTransport>('stdio');
  const [form] = Form.useForm();

  const loadServers = useCallback(async () => {
    setLoading(true);
    try {
      const result: MCPServerListResponse = await api.mcpServers.list({
        search: searchText,
        page,
        pageSize,
      });
      setServers(result.data || []);
      setTotal(result.total || 0);
    } catch {
      message.error('加载 MCP Server 失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText]);

  useEffect(() => {
    loadServers();
    api.baseAgents.getTypes().then(setAgentTypes).catch(() => {});
  }, [loadServers]);

  const handleCreate = () => {
    setEditing(null);
    setTransport('stdio');
    form.resetFields();
    form.setFieldsValue({
      transport: 'stdio',
      supportedAgents: [],
    });
    setModalVisible(true);
  };

  const handleEdit = (record: MCPServer) => {
    setEditing(record);
    setTransport(record.transport);
    form.setFieldsValue({
      ...record,
      argsText: (record.args || []).join('\n'),
      envText: stringifyMap(record.env),
      headersText: stringifyMap(record.headers),
    });
    setModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.mcpServers.delete(id);
      message.success('删除成功');
      loadServers();
    } catch {
      message.error('删除失败');
    }
  };

  const handleSubmit = async (values: any) => {
    setSubmitLoading(true);
    try {
      const payload = {
        name: values.name,
        description: values.description,
        transport: values.transport,
        command: values.command,
        args: parseLines(values.argsText),
        env: parseJSONMap(values.envText),
        url: values.url,
        headers: parseJSONMap(values.headersText),
        supportedAgents: values.supportedAgents || [],
      };
      if (editing) {
        const { name, ...updatePayload } = payload;
        void name;
        await api.mcpServers.update(editing.id, updatePayload);
        message.success('更新成功');
      } else {
        await api.mcpServers.create(payload);
        message.success('创建成功');
      }
      setModalVisible(false);
      loadServers();
    } catch (error: any) {
      message.error(error?.message || '保存失败');
    } finally {
      setSubmitLoading(false);
    }
  };

  return (
    <div style={{ padding: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>
          MCP Servers
        </Title>
        <Space>
          <Input.Search
            placeholder="搜索 MCP Server"
            allowClear
            onSearch={(value) => {
              setSearchText(value);
              setPage(1);
            }}
            style={{ width: 260 }}
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            新建
          </Button>
        </Space>
      </div>

      <Table
        rowKey="id"
        loading={loading}
        dataSource={servers}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: (nextPage, nextPageSize) => {
            setPage(nextPage);
            setPageSize(nextPageSize);
          },
        }}
        columns={[
          {
            title: '名称',
            dataIndex: 'name',
            render: (_: string, record) => (
              <Space direction="vertical" size={0}>
                <Space>
                  <ApiOutlined />
                  <Text strong>{record.name}</Text>
                </Space>
                <Text type="secondary">{record.description || '暂无描述'}</Text>
              </Space>
            ),
          },
          {
            title: 'Transport',
            dataIndex: 'transport',
            width: 120,
            render: (value: string) => <Tag color={value === 'stdio' ? 'blue' : 'purple'}>{value}</Tag>,
          },
          {
            title: '入口',
            width: 260,
            render: (_, record) => record.transport === 'stdio' ? record.command : record.url,
          },
          {
            title: '适配 Agent',
            dataIndex: 'supportedAgents',
            render: (types: string[] = []) => (
              <Space wrap>
                {types.length === 0 ? (
                  <Tag>claude_code</Tag>
                ) : types.map((type, index) => (
                  <Tag key={type} color={getTypeColorByIndex(index)}>{type}</Tag>
                ))}
              </Space>
            ),
          },
          {
            title: '操作',
            width: 140,
            render: (_, record) => (
              <Space>
                <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
                <Popconfirm title="确认删除？" onConfirm={() => handleDelete(record.id)}>
                  <Button size="small" danger icon={<DeleteOutlined />} />
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={editing ? '编辑 MCP Server' : '新建 MCP Server'}
        open={modalVisible}
        width={720}
        onCancel={() => setModalVisible(false)}
        onOk={() => form.submit()}
        confirmLoading={submitLoading}
        destroyOnClose
        forceRender
      >
        <Form form={form} layout="vertical" onFinish={handleSubmit}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]} extra="只能包含小写字母、数字和中划线">
            <Input disabled={!!editing} placeholder="github-tools" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input />
          </Form.Item>
          <Form.Item name="transport" label="Transport" rules={[{ required: true }]}>
            <Select
              onChange={(value: MCPTransport) => setTransport(value)}
              options={[
                { label: 'stdio', value: 'stdio' },
                { label: 'http', value: 'http' },
                { label: 'sse', value: 'sse' },
              ]}
            />
          </Form.Item>
          {transport === 'stdio' ? (
            <>
              <Form.Item name="command" label="Command" rules={[{ required: true }]}>
                <Input placeholder="npx" />
              </Form.Item>
              <Form.Item name="argsText" label="Args">
                <TextArea rows={4} placeholder="-y&#10;@modelcontextprotocol/server-github" />
              </Form.Item>
              <Form.Item name="envText" label="Env JSON">
                <TextArea rows={4} placeholder='{"GITHUB_TOKEN":"..."}' />
              </Form.Item>
            </>
          ) : (
            <>
              <Form.Item name="url" label="URL" rules={[{ required: true }]}>
                <Input placeholder="https://example.com/mcp" />
              </Form.Item>
              <Form.Item name="headersText" label="Headers JSON">
                <TextArea rows={4} placeholder='{"Authorization":"Bearer ..."}' />
              </Form.Item>
            </>
          )}
          <Form.Item name="supportedAgents" label="支持的基础 Agent">
            <Select
              mode="multiple"
              placeholder="留空默认仅支持 claude_code"
              options={agentTypes.map((type) => ({
                label: type.name || type.type,
                value: type.type,
              }))}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default MCPServerList;
