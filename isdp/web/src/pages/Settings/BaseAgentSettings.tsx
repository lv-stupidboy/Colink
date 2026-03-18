import React, { useEffect, useState } from 'react';
import { Table, Button, Card, Modal, Form, Input, Select, InputNumber, message, Space, Tag, Typography } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, ApiOutlined, RobotOutlined } from '@ant-design/icons';
import api from '@/api/client';
import type { BaseAgent, BaseAgentType, BaseAgentTypeInfo } from '@/types';

const { Title, Text } = Typography;

const BaseAgentSettings: React.FC = () => {
  const [agents, setAgents] = useState<BaseAgent[]>([]);
  const [agentTypes, setAgentTypes] = useState<BaseAgentTypeInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingAgent, setEditingAgent] = useState<BaseAgent | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    loadAgents();
    loadAgentTypes();
  }, []);

  const loadAgents = async () => {
    setLoading(true);
    try {
      const data = await api.baseAgents.list();
      setAgents(data);
    } catch (error) {
      message.error('加载基础Agent失败');
    } finally {
      setLoading(false);
    }
  };

  const loadAgentTypes = async () => {
    try {
      const data = await api.baseAgents.getTypes();
      setAgentTypes(data);
    } catch (error) {
      console.error('加载Agent类型失败', error);
    }
  };

  const handleCreate = () => {
    setEditingAgent(null);
    form.resetFields();
    form.setFieldsValue({
      type: 'claude_code',
      cliPath: 'claude',
      maxTokens: 4096,
      timeoutMinutes: 30,
    });
    setModalVisible(true);
  };

  const handleEdit = (record: BaseAgent) => {
    setEditingAgent(record);
    form.setFieldsValue(record);
    setModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除此基础Agent吗？删除后无法恢复。',
      okText: '确定',
      cancelText: '取消',
      onOk: async () => {
        try {
          await api.baseAgents.delete(id);
          message.success('删除成功');
          loadAgents();
        } catch (error) {
          message.error('删除失败');
        }
      },
    });
  };

  const handleTest = async (id: string) => {
    setTesting(id);
    try {
      const result = await api.baseAgents.test(id);
      if (result.success) {
        message.success('连接测试成功');
      } else {
        message.error(`连接测试失败: ${result.message}`);
      }
    } catch (error: any) {
      message.error(`连接测试失败: ${error.message || '未知错误'}`);
    } finally {
      setTesting(null);
    }
  };

  const handleSubmit = async (values: Partial<BaseAgent>) => {
    try {
      if (editingAgent) {
        await api.baseAgents.update(editingAgent.id, values);
        message.success('更新成功');
      } else {
        await api.baseAgents.create(values);
        message.success('创建成功');
      }
      setModalVisible(false);
      loadAgents();
    } catch (error) {
      message.error('操作失败');
    }
  };

  const getTypeLabel = (type: BaseAgentType) => {
    const typeInfo = agentTypes.find(t => t.type === type);
    return typeInfo?.name || type;
  };

  const getTypeColor = (type: BaseAgentType) => {
    switch (type) {
      case 'claude_code':
        return 'blue';
      case 'open_code':
        return 'green';
      default:
        return 'default';
    }
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, _record: BaseAgent) => (
        <Space>
          <RobotOutlined />
          <span>{name}</span>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (type: BaseAgentType) => (
        <Tag color={getTypeColor(type)}>{getTypeLabel(type)}</Tag>
      ),
    },
    {
      title: '默认模型',
      dataIndex: 'defaultModel',
      key: 'defaultModel',
    },
    {
      title: 'CLI路径',
      dataIndex: 'cliPath',
      key: 'cliPath',
      ellipsis: true,
    },
    {
      title: 'Git-Bash路径',
      dataIndex: 'gitBashPath',
      key: 'gitBashPath',
      ellipsis: true,
      render: (path: string) => path || '-',
    },
    {
      title: 'API URL',
      dataIndex: 'apiUrl',
      key: 'apiUrl',
      ellipsis: true,
      render: (url: string) => url || '-',
    },
    {
      title: '超时(分钟)',
      dataIndex: 'timeoutMinutes',
      key: 'timeoutMinutes',
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: unknown, record: BaseAgent) => (
        <Space>
          <Button
            type="link"
            icon={<ApiOutlined />}
            onClick={() => handleTest(record.id)}
            loading={testing === record.id}
          >
            测试
          </Button>
          <Button type="link" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            编辑
          </Button>
          <Button type="link" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.id)}>
            删除
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div className="base-agent-settings">
      <div style={{ marginBottom: 24 }}>
        <Title level={3}>
          <Space>
            <RobotOutlined />
            基础Agent设置
          </Space>
        </Title>
        <Text type="secondary">管理Claude Code和OpenCode等基础Agent实例的配置</Text>
      </div>

      <Card
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            新建基础Agent
          </Button>
        }
      >
        <Table
          dataSource={agents}
          columns={columns}
          rowKey="id"
          loading={loading}
          pagination={false}
        />
      </Card>

      <Modal
        title={editingAgent ? '编辑基础Agent' : '新建基础Agent'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={handleSubmit}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如: Claude Sonnet, OpenCode Local" />
          </Form.Item>

          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select>
              {agentTypes.map(t => (
                <Select.Option key={t.type} value={t.type}>
                  {t.name}
                </Select.Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item name="defaultModel" label="默认模型">
            <Input placeholder="如: claude-sonnet-4-6, gpt-4" />
          </Form.Item>

          <Form.Item name="cliPath" label="CLI路径">
            <Input placeholder="如: claude, opencode, /usr/local/bin/claude" />
          </Form.Item>

          <Form.Item
            name="gitBashPath"
            label="Git-Bash路径 (Windows)"
            extra="Windows下Claude CLI需要git-bash。如: D:\Program Files\Git\bin\bash.exe"
          >
            <Input placeholder="如: D:\Program Files\Git\bin\bash.exe" />
          </Form.Item>

          <Form.Item name="apiUrl" label="API URL (可选)">
            <Input placeholder="自定义API地址，留空使用默认" />
          </Form.Item>

          <Form.Item name="apiToken" label="API Token (可选)">
            <Input.Password placeholder="API令牌，留空使用环境变量" />
          </Form.Item>

          <Form.Item name="maxTokens" label="最大Token数">
            <InputNumber min={100} max={100000} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item name="timeoutMinutes" label="超时时间(分钟)">
            <InputNumber min={1} max={120} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default BaseAgentSettings;