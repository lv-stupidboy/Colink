import React, { useEffect, useState } from 'react';
import { Table, Button, Card, Modal, Form, Input, Select, message, Space, Tag, Typography, Tooltip } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, RobotOutlined, BugOutlined, CopyOutlined, ExclamationCircleOutlined, EyeOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '@/api/client';
import type { AgentConfig, BaseAgent } from '@/types';

const { Title, Text } = Typography;

// 截断文本并添加省略号
const truncateText = (text: string, maxLength: number = 50): string => {
  if (!text) return '-';
  if (text.length <= maxLength) return text;
  return text.substring(0, maxLength) + '...';
};

const AgentRoleList: React.FC = () => {
  const navigate = useNavigate();
  const [configs, setConfigs] = useState<AgentConfig[]>([]);
  const [baseAgents, setBaseAgents] = useState<BaseAgent[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingConfig, setEditingConfig] = useState<AgentConfig | null>(null);
  const [deleteLoading, setDeleteLoading] = useState<string | null>(null);
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

  const handleDelete = (record: AgentConfig) => {
    Modal.confirm({
      title: '确认删除',
      icon: <ExclamationCircleOutlined />,
      content: `确定要删除Agent角色「${record.name}」吗？此操作不可恢复。`,
      okText: '确认删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        setDeleteLoading(record.id);
        try {
          await api.agents.delete(record.id);
          message.success('删除成功');
          loadConfigs();
        } catch (error: any) {
          const errorData = error.response?.data;
          if (errorData?.referenced && errorData?.referenceNames) {
            Modal.error({
              title: '无法删除',
              content: (
                <div>
                  <p>该Agent角色被以下工作流引用，无法删除：</p>
                  <ul style={{ marginTop: 8, paddingLeft: 20 }}>
                    {errorData.referenceNames.map((name: string) => (
                      <li key={name}>{name}</li>
                    ))}
                  </ul>
                  <p style={{ marginTop: 8 }}>请先从工作流中移除该Agent，再进行删除。</p>
                </div>
              ),
            });
          } else {
            message.error('删除失败');
          }
        } finally {
          setDeleteLoading(null);
        }
      },
    });
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

  const handleViewRefs = async (record: AgentConfig) => {
    try {
      const result = await api.agents.checkReferences(record.id);
      if (result.referenced) {
        Modal.info({
          title: '工作流引用',
          content: (
            <div>
              <p>该Agent角色被以下 <strong>{result.referenceCount}</strong> 个工作流引用：</p>
              <ul style={{ marginTop: 8, paddingLeft: 20 }}>
                {result.referenceNames.map((name: string) => (
                  <li key={name}>{name}</li>
                ))}
              </ul>
            </div>
          ),
        });
      } else {
        Modal.info({
          title: '工作流引用',
          content: <p>该Agent角色暂未被任何工作流引用</p>,
        });
      }
    } catch (error) {
      message.error('查询引用失败');
    }
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 150,
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
      width: 120,
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
      width: 200,
      ellipsis: true,
      render: (desc?: string) => (
        <Tooltip title={desc} placement="topLeft">
          {desc || '-'}
        </Tooltip>
      ),
    },
    {
      title: '系统提示词',
      dataIndex: 'systemPrompt',
      key: 'systemPrompt',
      width: 350,
      render: (prompt?: string) => {
        if (!prompt) return '-';
        const truncated = truncateText(prompt, 80);
        if (truncated === prompt) return <Text style={{ maxWidth: 350 }}>{prompt}</Text>;
        return (
          <Tooltip title={<div style={{ maxWidth: 400, whiteSpace: 'pre-wrap' }}>{prompt}</div>} placement="topLeft">
            <Text style={{ maxWidth: 350, cursor: 'pointer' }}>{truncated}</Text>
          </Tooltip>
        );
      },
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
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => handleViewRefs(record)}>
            引用
          </Button>
          <Button type="link" size="small" icon={<CopyOutlined />} onClick={() => handleCopy(record)}>
            复制
          </Button>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            编辑
          </Button>
          <Button
            type="link"
            size="small"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handleDelete(record)}
            loading={deleteLoading === record.id}
          >
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