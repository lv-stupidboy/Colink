import React, { useState, useEffect } from 'react';
import {
  Typography,
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
  Alert,
  Spin,
  Popconfirm,
  Tabs,
  Empty,
} from 'antd';
import {
  PlusOutlined,
  ApartmentOutlined,
  PlayCircleOutlined,
  CheckCircleOutlined,
  UserOutlined,
  DeleteOutlined,
  EditOutlined,
  EyeOutlined,
  CrownOutlined,
  ThunderboltOutlined,
  BranchesOutlined,
  MergeCellsOutlined,
  SafetyOutlined,
} from '@ant-design/icons';
import { api } from '@/api/client';
import type { AgentConfig, WorkflowTemplate, Transition } from '@/types';
import { AgentRoleLabels } from '@/types';
import WorkflowEditor from '@/components/WorkflowEditor';
import './Workflow.css';

const { Text, Paragraph } = Typography;
const { Option } = Select;
const { TextArea } = Input;

/**
 * Agent团队页面
 * PRD Section 2.2 - 多Agent协同系统
 *
 * 功能要点：
 * - 可视化团队编辑器
 * - Agent 节点配置
 * - Agent团队选择
 */
const WorkflowPage: React.FC = () => {
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<WorkflowTemplate | null>(null);
  const [editingTemplate, setEditingTemplate] = useState<WorkflowTemplate | null>(null);
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

  // 获取Agent团队列表
  const fetchWorkflowTemplates = () => {
    setLoadingTemplates(true);
    api.workflows.list()
      .then(setWorkflowTemplates)
      .catch((error) => {
        console.error('Failed to fetch workflow templates:', error);
        message.error('获取Agent团队失败');
      })
      .finally(() => setLoadingTemplates(false));
  };

  useEffect(() => {
    fetchWorkflowTemplates();
  }, []);

  // 创建团队
  const handleCreateWorkflow = async (values: any) => {
    setSubmitting(true);
    try {
      await api.workflows.create({
        name: values.name,
        description: values.description || '',
        agentIds: [],
        transitions: [],
        checkpoints: values.checkpoints || [],
        estimatedTime: '自定义',
      });
      message.success('团队创建成功');
      setCreateModalVisible(false);
      form.resetFields();
      fetchWorkflowTemplates();
    } catch (error: any) {
      console.error('Failed to create workflow:', error);
      message.error(error?.response?.data?.error || '团队创建失败');
    } finally {
      setSubmitting(false);
    }
  };

  // 编辑团队
  const handleEditWorkflow = (template: WorkflowTemplate) => {
    setEditingTemplate(template);
    setEditModalVisible(true);
  };

  // 保存团队配置
  const handleSaveWorkflow = async (transitions: Transition[], agentIds: string[]) => {
    if (!editingTemplate) return;

    try {
      await api.workflows.update(editingTemplate.id, {
        agentIds,
        transitions,
      });
      fetchWorkflowTemplates();
    } catch (error: any) {
      throw error;
    }
  };

  // 删除团队
  const handleDeleteWorkflow = async (id: string) => {
    try {
      await api.workflows.delete(id);
      message.success('团队删除成功');
      fetchWorkflowTemplates();
      if (selectedTemplate?.id === id) {
        setSelectedTemplate(null);
      }
    } catch (error: any) {
      console.error('Failed to delete workflow:', error);
      message.error(error?.response?.data?.error || '团队删除失败');
    }
  };

  // 设置默认团队
  const handleSetDefault = async (id: string) => {
    try {
      await api.workflows.setDefault(id);
      message.success('已设置为默认团队');
      fetchWorkflowTemplates();
    } catch (error: any) {
      console.error('Failed to set default workflow:', error);
      message.error(error?.response?.data?.error || '设置默认团队失败');
    }
  };

  /**
   * 渲染Agent团队卡片
   */
  const renderTemplateCard = (template: WorkflowTemplate) => {
    // 根据agentIds获取对应的Agent实例
    const templateAgents = (agents || []).filter(a => (template.agentIds || []).includes(a.id));

    // 统计转换规则
    const transitions = template.transitions || [];
    const transitionStats = {
      total: transitions.length,
      parallel: transitions.filter(t => t.type === 'parallel').length,
      merge: transitions.filter(t => t.type === 'merge').length,
      sequence: transitions.filter(t => t.type === 'sequence').length,
    };

    return (
      <div
      className={`workflow-template-card ${selectedTemplate?.id === template.id ? 'selected' : ''}`}
      onClick={() => setSelectedTemplate(template)}
    >
      <div className="workflow-template-inner">
        {/* 头部 */}
        <div className="workflow-template-header">
          <div className="workflow-template-name">
            <span className="workflow-template-title">{template.name}</span>
            {template.isDefault && <Tag color="gold" bordered={false}>默认</Tag>}
            {template.isSystem && <Tag color="purple" bordered={false}>系统预设</Tag>}
          </div>
          <div className="workflow-template-actions">
            <Tag color="blue" bordered={false}>{template.estimatedTime}</Tag>
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
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={(e) => {
                e.stopPropagation();
                handleEditWorkflow(template);
              }}
            >
              编辑
            </Button>
            {!template.isSystem && (
              <Popconfirm
                title="确定删除此团队？"
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
          </div>
        </div>

        {/* 描述 */}
        {template.description && (
          <div className="workflow-template-desc">{template.description}</div>
        )}

        {/* Agent配置 */}
        <div className="workflow-template-section">
          <div className="workflow-template-section-title">Agent 配置</div>
          {templateAgents.length > 0 ? (
            <div className="workflow-agent-tags">
              {templateAgents.map((agent) => (
                <span key={agent.id} className="workflow-agent-tag">
                  <UserOutlined />
                  {agent.name}
                </span>
              ))}
            </div>
          ) : (
            <Text type="secondary" style={{ fontSize: 12 }}>请在编辑模式下配置Agent</Text>
          )}
        </div>

        {/* 转换规则统计 */}
        {transitionStats.total > 0 && (
          <div className="workflow-template-section">
            <div className="workflow-template-section-title">编排规则</div>
            <div className="workflow-rule-tags">
              {transitionStats.sequence > 0 && (
                <span className="workflow-rule-tag sequence">
                  <ThunderboltOutlined /> 顺序执行 ×{transitionStats.sequence}
                </span>
              )}
              {transitionStats.parallel > 0 && (
                <span className="workflow-rule-tag parallel">
                  <BranchesOutlined /> 并行触发 ×{transitionStats.parallel}
                </span>
              )}
              {transitionStats.merge > 0 && (
                <span className="workflow-rule-tag merge">
                  <MergeCellsOutlined /> 汇聚等待 ×{transitionStats.merge}
                </span>
              )}
            </div>
          </div>
        )}

        {/* 检查点 */}
        {(template.checkpoints?.length || 0) > 0 && (
          <div className="workflow-template-section">
            <div className="workflow-template-section-title">人工检查点</div>
            <div className="workflow-checkpoint-tags">
              {template.checkpoints?.map((checkpoint) => (
                <span key={checkpoint} className="workflow-checkpoint-tag">
                  <SafetyOutlined style={{ marginRight: 4 }} />
                  {checkpoint}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
    );
  };

  /**
   * 渲染 Agent 实例列表（分层显示）
   */
  const renderAgentRoles = () => {
    if (loadingAgents) {
      return (
        <div className="workflow-loading-container">
          <Spin />
        </div>
      );
    }

    if ((agents || []).length === 0) {
      return (
        <Empty
          description="暂无Agent实例"
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          style={{ padding: 20 }}
        />
      );
    }

    // 分组：系统预置和自定义
    const systemAgents = (agents || []).filter(a => a.isSystem);
    const customAgents = (agents || []).filter(a => !a.isSystem);

    return (
      <>
        {/* 系统预置角色 */}
        {systemAgents.length > 0 && (
          <div>
            <div className="workflow-agent-group-title">
              <CrownOutlined style={{ color: '#f59e0b' }} />
              <span>系统预置</span>
              <Tag color="gold" bordered={false} style={{ marginLeft: 4, fontSize: 11 }}>{systemAgents.length}</Tag>
            </div>
            <div className="workflow-agents-grid">
              {systemAgents.map((agent) => (
                <div key={agent.id} className="workflow-agent-card">
                  <div className="workflow-agent-avatar system">
                    <CrownOutlined />
                  </div>
                  <div className="workflow-agent-name">{agent.name}</div>
                  <div className="workflow-agent-role">
                    {AgentRoleLabels[agent.role] || agent.role}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* 自定义角色 */}
        {customAgents.length > 0 && (
          <div>
            <div className="workflow-agent-group-title">
              <UserOutlined />
              <span>自定义</span>
              <Tag color="blue" bordered={false} style={{ marginLeft: 4, fontSize: 11 }}>{customAgents.length}</Tag>
            </div>
            <div className="workflow-agents-grid">
              {customAgents.map((agent) => (
                <div key={agent.id} className="workflow-agent-card">
                  <div className="workflow-agent-avatar custom">
                    <UserOutlined />
                  </div>
                  <div className="workflow-agent-name">{agent.name}</div>
                  <div className="workflow-agent-role">
                    {AgentRoleLabels[agent.role] || agent.role}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </>
    );
  };

  return (
    <div className="workflow-page-wrapper">
      {/* 页面头部 */}
      <div className="workflow-page-header">
        <div>
          <h2 className="workflow-page-title">Agent团队</h2>
          <p className="workflow-page-subtitle">可视化配置 Agent 协作流程，定义任务执行顺序和条件</p>
        </div>
        <div className="workflow-header-actions">
          <button
            className="workflow-action-btn primary"
            onClick={() => setCreateModalVisible(true)}
          >
            <PlusOutlined />
            新建团队
          </button>
        </div>
      </div>

      {/* 主内容区 */}
      <div className="workflow-main-content">
        {/* 左侧：Agent团队选择 */}
        <div className="workflow-templates-panel">
          <div className="workflow-floating-card">
            <div className="workflow-card-header">
              <div className="workflow-card-title">
                <ApartmentOutlined className="workflow-card-title-icon" />
                <span>Agent团队</span>
                <Tag color="blue" bordered={false}>{(workflowTemplates || []).length} 个模板</Tag>
              </div>
            </div>
            <div className="workflow-card-content">
              <div className="workflow-empty-hint">
                选择一个预设模板快速开始，或创建自定义团队
              </div>

              {loadingTemplates ? (
                <div className="workflow-loading-container">
                  <Spin />
                </div>
              ) : (
                (workflowTemplates || []).map(renderTemplateCard)
              )}
            </div>
          </div>
        </div>

        {/* 右侧：Agent 实例说明 + 编排规则 */}
        <div className="workflow-side-panel">
          {/* Agent 实例 */}
          <div className="workflow-agents-section">
            <div className="workflow-card-header">
              <div className="workflow-card-title">
                <UserOutlined className="workflow-card-title-icon" />
                <span>Agent 实例</span>
              </div>
            </div>
            <div className="workflow-card-content">
              {renderAgentRoles()}
            </div>
          </div>

          {/* Agent团队编排说明 */}
          <div className="workflow-rules-section">
            <div className="workflow-card-header">
              <div className="workflow-card-title">
                <BranchesOutlined className="workflow-card-title-icon" />
                <span>编排规则</span>
              </div>
            </div>
            <div className="workflow-card-content" style={{ padding: 0 }}>
              <Collapse
                className="workflow-rules-collapse"
                defaultActiveKey={['1']}
                ghost
                items={[
                  {
                    key: '1',
                    label: (
                      <span>
                        <ThunderboltOutlined style={{ marginRight: 8, color: '#0369a1' }} />
                        顺序执行模式
                      </span>
                    ),
                    children: (
                      <>
                        <Paragraph style={{ marginBottom: 8, fontSize: 13 }}>
                          Agent 按团队编排顺序依次执行，前一个完成后下一个自动开始。
                        </Paragraph>
                        <div className="workflow-rule-flow">
                          <Tag color="green" bordered={false}>需求分析</Tag>
                          <span className="arrow">→</span>
                          <Tag color="purple" bordered={false}>架构设计</Tag>
                          <span className="arrow">→</span>
                          <Tag color="blue" bordered={false}>代码实现</Tag>
                          <span className="arrow">→</span>
                          <Tag color="orange" bordered={false}>代码审查</Tag>
                        </div>
                      </>
                    ),
                  },
                  {
                    key: '2',
                    label: (
                      <span>
                        <BranchesOutlined style={{ marginRight: 8, color: '#15803d' }} />
                        并行触发模式
                      </span>
                    ),
                    children: (
                      <>
                        <Paragraph style={{ marginBottom: 8, fontSize: 13 }}>
                          一个Agent完成后，同时触发多个下游Agent并行工作。
                        </Paragraph>
                        <div className="workflow-rule-flow">
                          <Tag color="blue" bordered={false}>需求分析</Tag>
                          <span className="arrow">→</span>
                          <Tag color="green" bordered={false}>前端开发</Tag>
                          <span className="plus">+</span>
                          <Tag color="purple" bordered={false}>后端开发</Tag>
                        </div>
                      </>
                    ),
                  },
                  {
                    key: '3',
                    label: (
                      <span>
                        <MergeCellsOutlined style={{ marginRight: 8, color: '#7c3aed' }} />
                        汇聚等待模式
                      </span>
                    ),
                    children: (
                      <>
                        <Paragraph style={{ marginBottom: 8, fontSize: 13 }}>
                          等待多个上游Agent都完成后，才触发下游Agent。
                        </Paragraph>
                        <div className="workflow-rule-flow">
                          <Tag color="green" bordered={false}>前端开发</Tag>
                          <span className="plus">+</span>
                          <Tag color="purple" bordered={false}>后端开发</Tag>
                          <span className="arrow">→</span>
                          <Tag color="orange" bordered={false}>测试工程师</Tag>
                        </div>
                      </>
                    ),
                  },
                  {
                    key: '4',
                    label: (
                      <span>
                        <SafetyOutlined style={{ marginRight: 8, color: '#c2410c' }} />
                        人工检查点
                      </span>
                    ),
                    children: (
                      <>
                        <Paragraph style={{ marginBottom: 8, fontSize: 13 }}>
                          关键节点需要用户确认才能继续执行。
                        </Paragraph>
                        <List
                          size="small"
                          dataSource={['需求确认', '方案确认', '代码合入', '部署确认']}
                          renderItem={(item) => (
                            <List.Item style={{ padding: '6px 0', border: 'none' }}>
                              <Space>
                                <CheckCircleOutlined style={{ color: '#52c41a' }} />
                                <span style={{ fontSize: 13 }}>{item}</span>
                              </Space>
                            </List.Item>
                          )}
                        />
                      </>
                    ),
                  },
                ]}
              />
            </div>
          </div>
        </div>
      </div>

      {/* 底部操作栏 */}
      {selectedTemplate && (
        <div className="workflow-footer-bar">
          <div className="workflow-footer-info">
            <span className="workflow-footer-label">已选择模板：</span>
            <Tag color="blue" bordered={false}>{selectedTemplate.name}</Tag>
            <span className="workflow-footer-value">预计耗时 {selectedTemplate.estimatedTime}</span>
          </div>
          <div className="workflow-footer-actions">
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
          </div>
        </div>
      )}

      {/* 新建团队弹窗 */}
      <Modal
        title="新建团队"
        open={createModalVisible}
        onOk={() => form.submit()}
        onCancel={() => {
          setCreateModalVisible(false);
          form.resetFields();
        }}
        confirmLoading={submitting}
        width={600}
        className="workflow-modal"
      >
        <Form form={form} layout="vertical" onFinish={handleCreateWorkflow}>
          <Form.Item name="name" label="团队名称" rules={[{ required: true, message: '请输入团队名称' }]}>
            <Input placeholder="例如：我的自定义流程" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <TextArea rows={2} placeholder="描述这个团队的用途" />
          </Form.Item>

          <Form.Item name="agentIds" label="Agent配置" rules={[{ required: true, message: '请选择至少一个Agent实例' }]}>
            <Select
              mode="multiple"
              placeholder="选择团队中的Agent实例"
              loading={loadingAgents}
              showSearch
              optionFilterProp="label"
            >
              {/* 系统预置角色分组 */}
              {agents.filter(a => a.isSystem).length > 0 && (
                <Select.OptGroup label={
                  <Space>
                    <CrownOutlined style={{ color: '#faad14' }} />
                    <span>系统预置</span>
                  </Space>
                }>
                  {agents.filter(a => a.isSystem).map((agent) => (
                    <Option key={agent.id} value={agent.id} label={agent.name}>
                      <Space>
                        <CrownOutlined style={{ color: '#faad14' }} />
                        <span>{agent.name}</span>
                        <Text type="secondary" style={{ fontSize: 12 }}>({AgentRoleLabels[agent.role] || agent.role})</Text>
                      </Space>
                    </Option>
                  ))}
                </Select.OptGroup>
              )}
              {/* 自定义角色分组 */}
              {agents.filter(a => !a.isSystem).length > 0 && (
                <Select.OptGroup label={
                  <Space>
                    <UserOutlined />
                    <span>自定义</span>
                  </Space>
                }>
                  {agents.filter(a => !a.isSystem).map((agent) => (
                    <Option key={agent.id} value={agent.id} label={agent.name}>
                      <Space>
                        <UserOutlined />
                        <span>{agent.name}</span>
                        <Text type="secondary" style={{ fontSize: 12 }}>({AgentRoleLabels[agent.role] || agent.role})</Text>
                      </Space>
                    </Option>
                  ))}
                </Select.OptGroup>
              )}
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
              {(workflowTemplates || []).map((t) => (
                <Option key={t.id} value={t.id}>{t.name}</Option>
              ))}
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      {/* 编辑团队弹窗 */}
      <Modal
        title={`编辑团队：${editingTemplate?.name || ''}`}
        open={editModalVisible}
        onCancel={() => {
          setEditModalVisible(false);
          setEditingTemplate(null);
        }}
        footer={null}
        width={900}
        styles={{ body: { padding: 0 } }}
        className="workflow-modal"
      >
        {editingTemplate && (
          <Tabs
            defaultActiveKey="visual"
            items={[
              {
                key: 'visual',
                label: (
                  <span>
                    <ApartmentOutlined style={{ marginRight: 4 }} />
                    可视化编辑
                  </span>
                ),
                children: (
                  <div style={{ padding: 16 }}>
                    <Alert
                      type="info"
                      message="操作提示"
                      description={
                        <ul style={{ margin: 0, paddingLeft: 20 }}>
                          <li>从下拉框选择Agent添加到画布</li>
                          <li>拖拽节点底部的连接点，连接到另一个节点的顶部</li>
                          <li>点击连接线可编辑转换规则</li>
                          <li>配置完成后点击"保存团队"</li>
                        </ul>
                      }
                      style={{ marginBottom: 16 }}
                      showIcon
                    />
                    <WorkflowEditor
                      agents={agents}
                      initialTransitions={editingTemplate.transitions || []}
                      onSave={handleSaveWorkflow}
                    />
                  </div>
                ),
              },
              {
                key: 'basic',
                label: (
                  <span>
                    <EditOutlined style={{ marginRight: 4 }} />
                    基本设置
                  </span>
                ),
                children: (
                  <div style={{ padding: 16 }}>
                    <Form
                      layout="vertical"
                      initialValues={{
                        name: editingTemplate.name,
                        description: editingTemplate.description,
                        checkpoints: editingTemplate.checkpoints || [],
                      }}
                      onFinish={async (values) => {
                        try {
                          await api.workflows.update(editingTemplate.id, values);
                          message.success('基本信息更新成功');
                          fetchWorkflowTemplates();
                        } catch (error: any) {
                          message.error(error?.response?.data?.error || '更新失败');
                        }
                      }}
                    >
                      <Form.Item name="name" label="团队名称" rules={[{ required: true }]}>
                        <Input />
                      </Form.Item>
                      <Form.Item name="description" label="描述">
                        <TextArea rows={2} />
                      </Form.Item>
                      <Form.Item name="checkpoints" label="人工检查点">
                        <Select mode="multiple">
                          <Option value="需求确认">需求确认</Option>
                          <Option value="方案确认">方案确认</Option>
                          <Option value="代码合入">代码合入</Option>
                          <Option value="部署确认">部署确认</Option>
                          <Option value="修复确认">修复确认</Option>
                        </Select>
                      </Form.Item>
                      <Form.Item>
                        <Button type="primary" htmlType="submit">
                          保存基本信息
                        </Button>
                      </Form.Item>
                    </Form>
                  </div>
                ),
              },
              {
                key: 'preview',
                label: (
                  <span>
                    <EyeOutlined style={{ marginRight: 4 }} />
                    预览
                  </span>
                ),
                children: (
                  <div style={{ padding: 16 }}>
                    <WorkflowEditor
                      agents={agents}
                      initialTransitions={editingTemplate.transitions || []}
                      onSave={async () => {}}
                      readOnly
                    />
                  </div>
                ),
              },
            ]}
          />
        )}
      </Modal>
    </div>
  );
};

export default WorkflowPage;