import React, { useEffect, useState } from 'react';
import { Table, Button, Card, Modal, Form, Input, Select, InputNumber, message, Space, Tag, Typography, Tooltip, Collapse } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, ApiOutlined, RobotOutlined, StarOutlined, StarFilled, SettingOutlined } from '@ant-design/icons';
import api from '@/api/client';
import type { BaseAgent, BaseAgentType, BaseAgentTypeInfo } from '@/types';

const { Title, Text } = Typography;

const BaseAgentSettings: React.FC = () => {
  const [agents, setAgents] = useState<BaseAgent[]>([]);
  const [agentTypes, setAgentTypes] = useState<BaseAgentTypeInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);
  const [settingDefault, setSettingDefault] = useState<string | null>(null);
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

  const handleSetDefault = async (id: string) => {
    setSettingDefault(id);
    try {
      const data = await api.baseAgents.setDefault(id);
      setAgents(data);
      message.success('已设为默认');
    } catch (error) {
      message.error('设置默认失败');
    } finally {
      setSettingDefault(null);
    }
  };

  const handleClearDefault = async (id: string) => {
    setSettingDefault(id);
    try {
      const data = await api.baseAgents.clearDefault(id);
      setAgents(data);
      message.success('已取消默认');
    } catch (error) {
      message.error('取消默认失败');
    } finally {
      setSettingDefault(null);
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
      render: (name: string, record: BaseAgent) => (
        <Space>
          <RobotOutlined />
          <span>{name}</span>
          {record.isDefault && (
            <Tooltip title="默认基础Agent">
              <StarFilled style={{ color: '#faad14' }} />
            </Tooltip>
          )}
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
      title: '模型',
      dataIndex: 'defaultModel',
      key: 'defaultModel',
    },
    {
      title: 'API URL',
      dataIndex: 'apiUrl',
      key: 'apiUrl',
      ellipsis: true,
      render: (url: string) => url || '-',
    },
    {
      title: '操作',
      key: 'actions',
      width: 340,
      render: (_: unknown, record: BaseAgent) => (
        <Space size="small">
          <Tooltip title={record.isDefault ? '点击取消默认' : '设为默认'}>
            <Button
              type="link"
              size="small"
              icon={record.isDefault ? <StarFilled style={{ color: '#faad14' }} /> : <StarOutlined />}
              onClick={() => record.isDefault ? handleClearDefault(record.id) : handleSetDefault(record.id)}
              loading={settingDefault === record.id}
            >
              {record.isDefault ? '取消默认' : '设为默认'}
            </Button>
          </Tooltip>
          <Button
            type="link"
            size="small"
            icon={<ApiOutlined />}
            onClick={() => handleTest(record.id)}
            loading={testing === record.id}
          >
            测试
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
    <div className="base-agent-settings">
      <div style={{ marginBottom: 24 }}>
        <Title level={3}>
          <Space>
            <RobotOutlined />
            基础Agent设置
          </Space>
        </Title>
        <Text type="secondary">管理Claude Code和OpenCode等基础Agent实例的配置。角色中未指定基础Agent时将使用默认的基础Agent。</Text>
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

          <Form.Item
            name="defaultModel"
            label="模型"
            rules={[{ required: true, message: '请输入模型' }]}
            extra="指定Agent使用的模型名称"
          >
            <Input placeholder="如: claude-sonnet-4-20250514" />
          </Form.Item>

          {/* API 配置区域 */}
          <Form.Item shouldUpdate noStyle>
            {({ getFieldValue }) => {
              const agentType = getFieldValue('type');
              if (agentType === 'open_code') {
                return (
                  <>
                    <Form.Item
                      name="apiUrl"
                      label="API URL"
                      extra={
                        <div>
                          <Text type="secondary">需要配置 OpenAI 协议兼容的 API 地址</Text>
                        </div>
                      }
                    >
                      <Input placeholder="如: https://your-custom-api.com/v1" />
                    </Form.Item>
                    <Form.Item
                      name="apiToken"
                      label="API Token"
                      extra="OpenCode API 令牌，用于身份认证"
                    >
                      <Input.Password placeholder="输入API令牌" />
                    </Form.Item>
                  </>
                );
              }
              return (
                <>
                  <Form.Item
                    name="apiUrl"
                    label="API URL"
                    extra="需要配置 Anthropic 协议的 API 地址"
                  >
                    <Input placeholder="如: https://api.anthropic.com" />
                  </Form.Item>
                  <Form.Item
                    name="apiToken"
                    label="API Token"
                    extra="Anthropic API 令牌，用于身份认证"
                  >
                    <Input.Password placeholder="输入API令牌" />
                  </Form.Item>
                </>
              );
            }}
          </Form.Item>

          {/* GitBash 路径配置 */}
          <Form.Item shouldUpdate noStyle>
            {({ getFieldValue }) => {
              const agentType = getFieldValue('type');
              if (agentType === 'claude_code') {
                return (
                  <Form.Item
                    name="gitBashPath"
                    label="Git-Bash路径"
                    extra="Windows下 Claude CLI 需要 git-bash 执行。如果 Git 已添加到系统 PATH，此项可留空；若 Claude CLI 无法启动，请配置 Git 安装目录下的 bash.exe 路径"
                  >
                    <Input placeholder="如: D:\Program Files\Git\bin\bash.exe" />
                  </Form.Item>
                );
              }
              return null;
            }}
          </Form.Item>

          {/* 高级配置：默认折叠 */}
          <Collapse
            style={{ marginBottom: 16 }}
            items={[
              {
                key: 'advanced',
                label: <Space><SettingOutlined />高级配置</Space>,
                children: (
                  <>
                    <Form.Item name="cliPath" label="CLI路径" extra="CLI 命令路径，默认为 claude 或 opencode">
                      <Input placeholder="如: claude, opencode, /usr/local/bin/claude" />
                    </Form.Item>

                    <Form.Item name="maxTokens" label="最大Token数" extra="限制输出 Token 数量，0 表示不限制">
                      <InputNumber min={0} max={100000} style={{ width: '100%' }} />
                    </Form.Item>

                    <Form.Item name="timeoutMinutes" label="超时时间(分钟)" extra="Agent 执行超时时间，默认30分钟">
                      <InputNumber min={1} max={120} style={{ width: '100%' }} />
                    </Form.Item>
                  </>
                ),
              },
            ]}
          />
        </Form>
      </Modal>
    </div>
  );
};

export default BaseAgentSettings;