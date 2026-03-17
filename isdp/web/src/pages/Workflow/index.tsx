import React, { useState, useEffect } from 'react';
import {
  Card,
  Typography,
  Row,
  Col,
  Button,
  Space,
  Tag,
  Modal,
  Form,
  Select,
  Input,
  message,
  Collapse,
  List,
  Avatar,
  Divider,
  Alert,
  Spin,
  Popconfirm,
} from 'antd';
import {
  PlusOutlined,
  ApartmentOutlined,
  PlayCircleOutlined,
  CheckCircleOutlined,
  UserOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { api } from '@/api/client';
import type { AgentConfig, WorkflowTemplate } from '@/types';
import { AgentRoleLabels } from '@/types';

const { Title, Text, Paragraph } = Typography;
const { Panel } = Collapse;
const { Option } = Select;
const { TextArea } = Input;

/**
 * 工作流编排页面
 * PRD Section 2.2 - 多Agent协同系统
 *
 * 功能要点：
 * - 可视化工作流编辑器
 * - Agent 节点配置
 * - 工作流模板选择
 */
const WorkflowPage: React.FC = () => {
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<WorkflowTemplate | null>(null);
  const [form] = Form.useForm();
  const [agents, setAgents] = useState<AgentConfig[]>([]);
  const [loadingAgents, setLoadingAgents] = useState(false);
  const [workflowTemplates, setWorkflowTemplates] = useState<WorkflowTemplate[]>([]);
  const [loadingTemplates, setLoadingTemplates] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  // 获取Agent实例列表
  useEffect(() => {
    setLoadingAgents(true);
    api.agents.list()
      .then(setAgents)
      .catch((error) => {
        console.error('Failed to fetch agents:', error);
        message.error('获取Agent列表失败');
      })
      .finally(() => setLoadingAgents(false));
  }, []);

  // 获取工作流模板列表
  const fetchWorkflowTemplates = () => {
    setLoadingTemplates(true);
    api.workflows.list()
      .then(setWorkflowTemplates)
      .catch((error) => {
        console.error('Failed to fetch workflow templates:', error);
        message.error('获取工作流模板失败');
      })
      .finally(() => setLoadingTemplates(false));
  };

  useEffect(() => {
    fetchWorkflowTemplates();
  }, []);

  // 创建工作流
  const handleCreateWorkflow = async (values: any) => {
    setSubmitting(true);
    try {
      await api.workflows.create({
        name: values.name,
        description: values.description || '',
        agentIds: values.agentIds || [],
        checkpoints: values.checkpoints || [],
        estimatedTime: '自定义',
      });
      message.success('工作流创建成功');
      setCreateModalVisible(false);
      form.resetFields();
      fetchWorkflowTemplates(); // 刷新列表
    } catch (error: any) {
      console.error('Failed to create workflow:', error);
      message.error(error?.response?.data?.error || '工作流创建失败');
    } finally {
      setSubmitting(false);
    }
  };

  // 删除工作流
  const handleDeleteWorkflow = async (id: string) => {
    try {
      await api.workflows.delete(id);
      message.success('工作流删除成功');
      fetchWorkflowTemplates(); // 刷新列表
      if (selectedTemplate?.id === id) {
        setSelectedTemplate(null);
      }
    } catch (error: any) {
      console.error('Failed to delete workflow:', error);
      message.error(error?.response?.data?.error || '工作流删除失败');
    }
  };

  // 设置默认工作流
  const handleSetDefault = async (id: string) => {
    try {
      await api.workflows.setDefault(id);
      message.success('已设置为默认工作流');
      fetchWorkflowTemplates(); // 刷新列表
    } catch (error: any) {
      console.error('Failed to set default workflow:', error);
      message.error(error?.response?.data?.error || '设置默认工作流失败');
    }
  };

  /**
   * 渲染工作流模板卡片
   */
  const renderTemplateCard = (template: WorkflowTemplate) => {
    // 根据agentIds获取对应的Agent实例
    const templateAgents = agents.filter(a => template.agentIds?.includes(a.id));

    return (
      <Card
        hoverable
        className={`workflow-template-card ${selectedTemplate?.id === template.id ? 'selected' : ''}`}
        onClick={() => setSelectedTemplate(template)}
        style={{
          marginBottom: 16,
          border: selectedTemplate?.id === template.id ? '2px solid #1890ff' : undefined,
        }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Space>
              <Title level={5} style={{ margin: 0 }}>{template.name}</Title>
              {template.isDefault && <Tag color="gold">默认</Tag>}
              {template.isSystem && <Tag color="purple">系统预设</Tag>}
            </Space>
            <Space>
              <Tag color="blue">{template.estimatedTime}</Tag>
              {!template.isDefault && (
                <Button
                  type="link"
                  size="small"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleSetDefault(template.id);
                  }}
                >
                  设为默认
                </Button>
              )}
              {!template.isSystem && (
                <Popconfirm
                  title="确定删除此工作流？"
                  onConfirm={(e) => {
                    e?.stopPropagation();
                    handleDeleteWorkflow(template.id);
                  }}
                  onCancel={(e) => e?.stopPropagation()}
                >
                  <Button
                    type="text"
                    danger
                    size="small"
                    icon={<DeleteOutlined />}
                    onClick={(e) => e.stopPropagation()}
                  />
                </Popconfirm>
              )}
            </Space>
          </div>
          {template.description && (
            <Text type="secondary">{template.description}</Text>
          )}

          <Divider style={{ margin: '12px 0' }} />

          <div>
            <Text strong>Agent配置：</Text>
            <div style={{ marginTop: 8 }}>
              {templateAgents.length > 0 ? (
                templateAgents.map((agent, index) => (
                  <span key={agent.id}>
                    <Tag color="blue">{agent.name} ({AgentRoleLabels[agent.role]})</Tag>
                    {index < templateAgents.length - 1 && <span style={{ margin: '0 4px' }}>→</span>}
                  </span>
                ))
              ) : (
                <Text type="secondary">请在自定义工作流中配置Agent</Text>
              )}
            </div>
          </div>

          <div>
            <Text strong>人工检查点：</Text>
            <div style={{ marginTop: 8 }}>
              {template.checkpoints?.map((checkpoint) => (
                <Tag key={checkpoint} color="orange">{checkpoint}</Tag>
              ))}
            </div>
          </div>
        </Space>
      </Card>
    );
  };

  /**
   * 渲染 Agent 实例列表
   */
  const renderAgentRoles = () => {
    if (loadingAgents) {
      return (
        <div style={{ textAlign: 'center', padding: 24 }}>
          <Spin />
        </div>
      );
    }

    if (agents.length === 0) {
      return (
        <Alert
          type="info"
          message="暂无Agent实例"
          description="请先在Agent管理页面创建Agent实例"
          showIcon
        />
      );
    }

    return (
      <List
        grid={{ gutter: 16, column: 2 }}
        dataSource={agents}
        renderItem={(agent) => (
          <List.Item>
            <Card
              hoverable
              size="small"
              style={{ textAlign: 'center' }}
            >
              <Avatar
                size={48}
                style={{ backgroundColor: '#1890ff', marginBottom: 8 }}
                icon={<UserOutlined />}
              />
              <div>
                <Text strong>{agent.name}</Text>
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {AgentRoleLabels[agent.role]}
              </Text>
            </Card>
          </List.Item>
        )}
      />
    );
  };

  return (
    <div className="workflow-page">
      <div style={{ marginBottom: 24 }}>
        <Title level={2}>
          <ApartmentOutlined style={{ marginRight: 8 }} />
          工作流编排
        </Title>
        <Text type="secondary">
          可视化配置 Agent 协作流程，定义任务执行顺序和条件
        </Text>
      </div>

      <Row gutter={24}>
        {/* 左侧：工作流模板选择 */}
        <Col xs={24} lg={14}>
          <Card
            title={
              <Space>
                <span>工作流模板</span>
                <Tag color="blue">{workflowTemplates.length} 个模板</Tag>
              </Space>
            }
            extra={
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => setCreateModalVisible(true)}
              >
                自定义工作流
              </Button>
            }
          >
            <Paragraph type="secondary">
              选择一个预设模板快速开始，或创建自定义工作流
            </Paragraph>

            {loadingTemplates ? (
              <div style={{ textAlign: 'center', padding: 24 }}>
                <Spin />
              </div>
            ) : (
              workflowTemplates.map(renderTemplateCard)
            )}
          </Card>
        </Col>

        {/* 右侧：Agent 实例说明 */}
        <Col xs={24} lg={10}>
          <Card title="Agent 实例">
            <Paragraph type="secondary" style={{ marginBottom: 16 }}>
              已配置的Agent实例，可在工作流中选择使用
            </Paragraph>
            {renderAgentRoles()}
          </Card>

          {/* 工作流编排说明 */}
          <Card style={{ marginTop: 16 }} title="编排规则">
            <Collapse defaultActiveKey={['1']}>
              <Panel header="顺序执行模式" key="1">
                <Paragraph>
                  Agent 按工作流顺序依次执行，前一个完成后下一个自动开始。
                </Paragraph>
                <Space wrap>
                  <Tag color="green">需求分析</Tag>
                  <span>→</span>
                  <Tag color="purple">架构设计</Tag>
                  <span>→</span>
                  <Tag color="blue">代码实现</Tag>
                  <span>→</span>
                  <Tag color="orange">代码审查</Tag>
                </Space>
              </Panel>
              <Panel header="人工检查点" key="2">
                <Paragraph>
                  关键节点需要用户确认才能继续执行。
                </Paragraph>
                <List
                  size="small"
                  dataSource={['需求确认', '方案确认', '代码合入', '部署确认']}
                  renderItem={(item) => (
                    <List.Item>
                      <Space>
                        <CheckCircleOutlined style={{ color: '#52c41a' }} />
                        <span>{item}</span>
                      </Space>
                    </List.Item>
                  )}
                />
              </Panel>
              <Panel header="A2A 路由机制" key="3">
                <Paragraph>
                  Agent 可以通过 @mention 触发其他 Agent 协作。
                </Paragraph>
                <Text code>@审查员 请帮我 review 这段代码</Text>
              </Panel>
            </Collapse>
          </Card>
        </Col>
      </Row>

      {/* 使用选中模板创建任务 */}
      {selectedTemplate && (
        <Card style={{ marginTop: 24 }}>
          <Space style={{ width: '100%', justifyContent: 'space-between' }}>
            <Space>
              <Text strong>已选择模板：</Text>
              <Tag color="blue">{selectedTemplate.name}</Tag>
              <Text type="secondary">预计耗时 {selectedTemplate.estimatedTime}</Text>
            </Space>
            <Space>
              <Button onClick={() => setSelectedTemplate(null)}>取消选择</Button>
              <Button
                type="primary"
                icon={<PlayCircleOutlined />}
                onClick={() => {
                  message.info('请在项目空间中创建任务开始开发');
                }}
              >
                使用此模板创建任务
              </Button>
            </Space>
          </Space>
        </Card>
      )}

      {/* 自定义工作流弹窗 */}
      <Modal
        title="自定义工作流"
        open={createModalVisible}
        onOk={() => form.submit()}
        onCancel={() => {
          setCreateModalVisible(false);
          form.resetFields();
        }}
        confirmLoading={submitting}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={handleCreateWorkflow}>
          <Form.Item name="name" label="工作流名称" rules={[{ required: true, message: '请输入工作流名称' }]}>
            <Input placeholder="例如：我的自定义流程" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <TextArea rows={2} placeholder="描述这个工作流的用途" />
          </Form.Item>

          <Form.Item name="agentIds" label="Agent配置" rules={[{ required: true, message: '请选择至少一个Agent实例' }]}>
            <Select
              mode="multiple"
              placeholder="选择工作流中的Agent实例"
              loading={loadingAgents}
              showSearch
              filterOption={(input, option) =>
                (option?.children as unknown as string)?.toLowerCase().includes(input.toLowerCase())
              }
            >
              {agents.map((agent) => (
                <Option key={agent.id} value={agent.id}>
                  {agent.name} ({AgentRoleLabels[agent.role]})
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item name="checkpoints" label="人工检查点">
            <Select mode="multiple" placeholder="选择需要用户确认的节点">
              <Option value="需求确认">需求确认</Option>
              <Option value="方案确认">方案确认</Option>
              <Option value="代码合入">代码合入</Option>
              <Option value="部署确认">部署确认</Option>
              <Option value="修复确认">修复确认</Option>
            </Select>
          </Form.Item>

          <Form.Item name="basedOn" label="基于模板">
            <Select placeholder="选择一个基础模板" allowClear>
              {workflowTemplates.map((t) => (
                <Option key={t.id} value={t.id}>{t.name}</Option>
              ))}
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default WorkflowPage;