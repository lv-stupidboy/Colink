import React, { useState } from 'react';
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
} from 'antd';
import {
  PlusOutlined,
  ApartmentOutlined,
  PlayCircleOutlined,
  CheckCircleOutlined,
  UserOutlined,
} from '@ant-design/icons';

const { Title, Text, Paragraph } = Typography;
const { Panel } = Collapse;
const { Option } = Select;
const { TextArea } = Input;

/**
 * 工作流模板
 */
interface WorkflowTemplate {
  id: string;
  name: string;
  description: string;
  phases: string[];
  checkpoints: string[];
  estimatedTime: string;
}

/**
 * 工作流编排页面
 * PRD Section 2.2 - 多Agent协同系统
 *
 * 功能要点：
 * - 可视化工作流编辑器
 * - Agent 节点配置
 * - 工作流模板选择
 *
 * TODO: 实现完整的工作流编排功能（二期功能）
 */
const WorkflowPage: React.FC = () => {
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<WorkflowTemplate | null>(null);
  const [form] = Form.useForm();

  // 预设工作流模板
  const workflowTemplates: WorkflowTemplate[] = [
    {
      id: 'standard',
      name: '标准开发流程',
      description: '完整的软件开发流程，从需求到部署',
      phases: ['需求分析', '架构设计', '代码实现', '代码审查', '测试验证', '部署发布'],
      checkpoints: ['需求确认', '方案确认', '代码合入', '部署确认'],
      estimatedTime: '2-4小时',
    },
    {
      id: 'rapid',
      name: '快速原型流程',
      description: '快速构建原型，验证想法',
      phases: ['需求分析', '代码实现', '快速测试'],
      checkpoints: ['需求确认'],
      estimatedTime: '30分钟-1小时',
    },
    {
      id: 'refactor',
      name: '代码重构流程',
      description: '优化现有代码结构和质量',
      phases: ['代码审查', '架构优化', '重构实现', '测试验证'],
      checkpoints: ['方案确认', '代码合入'],
      estimatedTime: '1-3小时',
    },
    {
      id: 'bugfix',
      name: '问题修复流程',
      description: '快速定位和修复问题',
      phases: ['问题分析', '代码修复', '测试验证'],
      checkpoints: ['修复确认'],
      estimatedTime: '30分钟-2小时',
    },
  ];

  // Agent 角色配置
  const agentRoles = [
    { id: 'requirement', name: '需求分析师', icon: '📋', color: '#1890ff' },
    { id: 'architect', name: '架构师', icon: '🏗️', color: '#722ed1' },
    { id: 'developer', name: '开发者', icon: '💻', color: '#52c41a' },
    { id: 'reviewer', name: '审查员', icon: '🔍', color: '#faad14' },
    { id: 'testengineer', name: '测试工程师', icon: '🧪', color: '#eb2f96' },
    { id: 'devops', name: '运维工程师', icon: '🚀', color: '#13c2c2' },
  ];

  const handleCreateWorkflow = (values: any) => {
    console.log('Create workflow:', values);
    message.success('工作流创建成功（功能开发中）');
    setCreateModalVisible(false);
    form.resetFields();
  };

  /**
   * 渲染工作流模板卡片
   */
  const renderTemplateCard = (template: WorkflowTemplate) => (
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
          <Title level={5} style={{ margin: 0 }}>{template.name}</Title>
          <Tag color="blue">{template.estimatedTime}</Tag>
        </div>
        <Text type="secondary">{template.description}</Text>

        <Divider style={{ margin: '12px 0' }} />

        <div>
          <Text strong>阶段流程：</Text>
          <div style={{ marginTop: 8 }}>
            {template.phases.map((phase, index) => (
              <span key={phase}>
                <Tag>{phase}</Tag>
                {index < template.phases.length - 1 && <span style={{ margin: '0 4px' }}>→</span>}
              </span>
            ))}
          </div>
        </div>

        <div>
          <Text strong>人工检查点：</Text>
          <div style={{ marginTop: 8 }}>
            {template.checkpoints.map((checkpoint) => (
              <Tag key={checkpoint} color="orange">{checkpoint}</Tag>
            ))}
          </div>
        </div>
      </Space>
    </Card>
  );

  /**
   * 渲染 Agent 角色列表
   */
  const renderAgentRoles = () => (
    <List
      grid={{ gutter: 16, column: 3 }}
      dataSource={agentRoles}
      renderItem={(agent) => (
        <List.Item>
          <Card
            hoverable
            size="small"
            style={{ textAlign: 'center' }}
          >
            <Avatar
              size={48}
              style={{ backgroundColor: agent.color, marginBottom: 8 }}
              icon={<UserOutlined />}
            />
            <div>
              <Text strong>{agent.name}</Text>
            </div>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {agent.id}
            </Text>
          </Card>
        </List.Item>
      )}
    />
  );

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

            {workflowTemplates.map(renderTemplateCard)}
          </Card>
        </Col>

        {/* 右侧：Agent 角色说明 */}
        <Col xs={24} lg={10}>
          <Card title="Agent 角色">
            <Paragraph type="secondary" style={{ marginBottom: 16 }}>
              每个 Agent 负责特定领域的任务，通过 @mention 触发协作
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
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={handleCreateWorkflow}>
          <Form.Item name="name" label="工作流名称" rules={[{ required: true }]}>
            <Input placeholder="例如：我的自定义流程" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <TextArea rows={2} placeholder="描述这个工作流的用途" />
          </Form.Item>

          <Form.Item name="phases" label="阶段配置" rules={[{ required: true }]}>
            <Select mode="multiple" placeholder="选择工作流阶段">
              <Option value="requirement">需求分析</Option>
              <Option value="design">架构设计</Option>
              <Option value="development">代码实现</Option>
              <Option value="review">代码审查</Option>
              <Option value="test">测试验证</Option>
              <Option value="deploy">部署发布</Option>
            </Select>
          </Form.Item>

          <Form.Item name="checkpoints" label="人工检查点">
            <Select mode="multiple" placeholder="选择需要用户确认的节点">
              <Option value="requirement">需求确认</Option>
              <Option value="design">方案确认</Option>
              <Option value="review">代码合入</Option>
              <Option value="deploy">部署确认</Option>
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

        <Alert
          type="info"
          message="自定义工作流功能正在开发中"
          description="完整的工作流编辑器将在二期版本中推出"
          showIcon
          style={{ marginTop: 16 }}
        />
      </Modal>
    </div>
  );
};

export default WorkflowPage;