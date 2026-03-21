import React, { useEffect, useState } from 'react';
import { Table, Button, Card, Modal, Form, Input, Select, InputNumber, Switch, message, Space, Tag, Typography } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import api from '@/api/client';
import type { AgentConfig, AgentRole } from '@/types';
import { AgentRoleLabels } from '@/types';

const { Title, Text } = Typography;

const AgentConfigPage: React.FC = () => {
  const [configs, setConfigs] = useState<AgentConfig[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingConfig, setEditingConfig] = useState<AgentConfig | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    loadConfigs();
  }, []);

  const loadConfigs = async () => {
    setLoading(true);
    try {
      const data = await api.agents.list();
      setConfigs(data);
    } catch (error) {
      message.error('加载Agent配置失败');
    } finally {
      setLoading(false);
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

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '角色',
      dataIndex: 'role',
      key: 'role',
      render: (role: AgentRole) => AgentRoleLabels[role] || role,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '模型',
      dataIndex: 'modelName',
      key: 'modelName',
    },
    {
      title: '默认',
      dataIndex: 'isDefault',
      key: 'isDefault',
      render: (isDefault: boolean) => isDefault ? <Tag color="blue">默认</Tag> : null,
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: unknown, record: AgentConfig) => (
        <Space>
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
    <div style={{ padding: 12 }}>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>Agent 配置</Title>
          <Text type="secondary">管理 Agent 角色的系统提示词和模型参数</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新建配置
        </Button>
      </div>

      <Card>
        <Table
          dataSource={configs}
          columns={columns}
          rowKey="id"
          loading={loading}
        />
      </Card>

      <Modal
        title={editingConfig ? '编辑Agent配置' : '新建Agent配置'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={700}
      >
        <Form form={form} layout="vertical" onFinish={handleSubmit}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input placeholder="Agent名称" />
          </Form.Item>

          <Form.Item name="role" label="角色" rules={[{ required: true }]}>
            <Select placeholder="选择角色">
              {Object.entries(AgentRoleLabels).map(([key, label]) => (
                <Select.Option key={key} value={key}>{label}</Select.Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="Agent描述" />
          </Form.Item>

          <Form.Item name="systemPrompt" label="系统提示词" rules={[{ required: true }]}>
            <Input.TextArea rows={6} placeholder="系统提示词，定义Agent的行为和能力" />
          </Form.Item>

          <Form.Item name="modelName" label="模型名称" initialValue="claude-sonnet-4-6">
            <Select>
              <Select.Option value="claude-opus-4-6">Claude Opus 4.6</Select.Option>
              <Select.Option value="claude-sonnet-4-6">Claude Sonnet 4.6</Select.Option>
              <Select.Option value="claude-haiku-4-5">Claude Haiku 4.5</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item name="maxTokens" label="最大Token数" initialValue={4096}>
            <InputNumber min={100} max={100000} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item name="temperature" label="温度" initialValue={0.7}>
            <InputNumber min={0} max={2} step={0.1} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item name="isDefault" label="设为默认" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default AgentConfigPage;