import React, { useEffect, useState } from 'react';
import { Table, Button, Card, Modal, Form, Input, Select, message, Space, Tag, Typography } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, RobotOutlined, BugOutlined, CopyOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '@/api/client';
import type { AgentConfig, BaseAgent } from '@/types';

const { Title, Text } = Typography;

const AgentRoleList: React.FC = () => {
  const navigate = useNavigate();
  const [configs, setConfigs] = useState<AgentConfig[]>([]);
  const [baseAgents, setBaseAgents] = useState<BaseAgent[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingConfig, setEditingConfig] = useState<AgentConfig | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    loadConfigs();
    loadBaseAgents();
  }, []);

  const loadConfigs = async () => {
    setLoading(true);
    try {
      const data = await api.agents.list();
      setConfigs(data);
    } catch (error) {
      message.error('加载Agent角色失败');
    } finally {
      setLoading(false);
    }
  };

  const loadBaseAgents = async () => {
    try {
      const data = await api.baseAgents.list();
      setBaseAgents(data);
    } catch (error) {
      console.error('加载基础Agent失败', error);
    }
  };

  const handleCreate = () => {
    setEditingConfig(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: AgentConfig) => {
    setEditingConfig(record);
    form.setFieldsValue(record);
    setModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.agents.delete(id);
      message.success('删除成功');
      loadConfigs();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const handleSubmit = async (values: Partial<AgentConfig>) => {
    try {
      if (editingConfig) {
        await api.agents.update(editingConfig.id, values);
        message.success('更新成功');
      } else {
        await api.agents.create(values);
        message.success('创建成功');
      }
      setModalVisible(false);
      loadConfigs();
    } catch (error) {
      message.error('操作失败');
    }
  };

  const handleDebug = (record: AgentConfig) => {
    navigate(`/agents/${record.id}/debug`);
  };

  const handleCopy = async (record: AgentConfig) => {
    try {
      await api.agents.copy(record.id);
      message.success('复制成功');
      loadConfigs();
    } catch (error) {
      message.error('复制失败');
    }
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => (
        <Space>
          <RobotOutlined />
          <span>{name}</span>
        </Space>
      ),
    },
    {
      title: '基础Agent',
      dataIndex: 'baseAgentId',
      key: 'baseAgentId',
      render: (baseAgentId: string) => {
        const agent = baseAgents.find(a => a.id === baseAgentId);
        return agent ? (
          <Tag color={agent.type === 'claude_code' ? 'blue' : 'green'}>
            {agent.name}
          </Tag>
        ) : <Tag>默认</Tag>;
      },
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '操作',
      key: 'actions',
      width: 280,
      fixed: 'right' as const,
      render: (_: unknown, record: AgentConfig) => (
        <Space size="small">
          <Button type="link" size="small" icon={<BugOutlined />} onClick={() => handleDebug(record)}>
            调试
          </Button>
          <Button type="link" size="small" icon={<CopyOutlined />} onClick={() => handleCopy(record)}>
            复制
          </Button>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            编辑
          </Button>
          <Button type="link" size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.id)}>
            删除
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div className="agent-role-list">
      <div style={{ marginBottom: 24 }}>
        <Title level={2}>
          <Space>
            <RobotOutlined />
            Agent角色
          </Space>
        </Title>
        <Text type="secondary">管理不同职责的Agent角色配置</Text>
      </div>

      <Card
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            新建角色
          </Button>
        }
      >
        <Table
          dataSource={configs}
          columns={columns}
          rowKey="id"
          loading={loading}
          scroll={{ x: 'max-content' }}
        />
      </Card>

      <Modal
        title={editingConfig ? '编辑Agent角色' : '新建Agent角色'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={800}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{}}
        >
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="Agent角色名称" />
          </Form.Item>

          <Form.Item name="baseAgentId" label="基础Agent">
            <Select placeholder="选择基础Agent" allowClear>
              {baseAgents.map(agent => (
                <Select.Option key={agent.id} value={agent.id}>
                  {agent.name} ({agent.type === 'claude_code' ? 'Claude Code' : 'OpenCode'})
                </Select.Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="Agent角色描述" />
          </Form.Item>

          <Form.Item name="systemPrompt" label="系统提示词" rules={[{ required: true, message: '请输入系统提示词' }]}>
            <Input.TextArea rows={8} placeholder="系统提示词，定义Agent的行为和能力" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default AgentRoleList;