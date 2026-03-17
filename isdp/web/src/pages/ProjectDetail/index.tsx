import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Card,
  Tabs,
  Button,
  Space,
  Tag,
  Typography,
  Descriptions,
  Table,
  Modal,
  Form,
  Input,
  Select,
  message,
  Spin,
  Empty,
  Progress,
  Tooltip,
  Divider,
} from 'antd';
import {
  ArrowLeftOutlined,
  SettingOutlined,
  PlusOutlined,
  FolderOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
} from '@ant-design/icons';
import api from '@/api/client';
import type { Project, Thread, WorkflowTemplate } from '@/types';
import { PhaseLabels, PhaseColors, ThreadStatus } from '@/types';

const { Text } = Typography;
const { TabPane } = Tabs;
const { Option } = Select;
const { TextArea } = Input;

/**
 * 项目详情页
 * PRD Section 2.1.1 - 项目空间管理
 *
 * 功能要点：
 * - 项目属性展示与编辑
 * - 项目状态跟踪与可视化
 * - 项目任务列表 (Thread列表)
 * - 项目设置
 */
const ProjectDetail: React.FC = () => {
  const { projectId } = useParams<{ projectId: string }>();
  const navigate = useNavigate();

  const [project, setProject] = useState<Project | null>(null);
  const [threads, setThreads] = useState<Thread[]>([]);
  const [loading, setLoading] = useState(true);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [createThreadModalVisible, setCreateThreadModalVisible] = useState(false);
  const [form] = Form.useForm();
  const [threadForm] = Form.useForm();
  const [workflowTemplates, setWorkflowTemplates] = useState<WorkflowTemplate[]>([]);
  const [loadingTemplates, setLoadingTemplates] = useState(false);

  useEffect(() => {
    if (projectId) {
      loadProjectData();
    }
  }, [projectId]);

  // 获取工作流模板列表
  const fetchWorkflowTemplates = async () => {
    setLoadingTemplates(true);
    try {
      const templates = await api.workflows.list();
      setWorkflowTemplates(templates as unknown as WorkflowTemplate[]);
    } catch (error) {
      console.error('Failed to fetch workflow templates:', error);
    } finally {
      setLoadingTemplates(false);
    }
  };

  useEffect(() => {
    fetchWorkflowTemplates();
  }, []);

  const loadProjectData = async () => {
    setLoading(true);
    try {
      const projectData = await api.projects.get(projectId!);
      setProject(projectData as unknown as Project);

      // 加载该项目的 Thread 列表
      const threadsData = await api.threads.list(projectId!);
      // 处理可能返回 null 的情况
      setThreads((threadsData as unknown as Thread[]) || []);
    } catch (error) {
      message.error('加载项目数据失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  const handleUpdateProject = async (values: Partial<Project>) => {
    try {
      await api.projects.update(projectId!, values);
      message.success('项目更新成功');
      setEditModalVisible(false);
      loadProjectData();
    } catch (error) {
      message.error('更新失败');
    }
  };

  const handleCreateThread = async () => {
    try {
      const thread = await api.threads.create(projectId!);
      message.success('任务创建成功');
      setCreateThreadModalVisible(false);
      threadForm.resetFields();
      // 跳转到新创建的 Thread
      navigate(`/projects/${projectId}/threads/${thread.id}`);
    } catch (error) {
      message.error('创建任务失败');
    }
  };

  const handleDeleteProject = () => {
    Modal.confirm({
      title: '确认删除项目？',
      content: '删除后将无法恢复，关联的任务也会被删除',
      okText: '确认删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await api.projects.delete(projectId!);
          message.success('项目已删除');
          navigate('/projects');
        } catch (error) {
          message.error('删除失败');
        }
      },
    });
  };

  // Thread 列表列定义
  const threadColumns = [
    {
      title: '任务 ID',
      dataIndex: 'id',
      key: 'id',
      width: 120,
      render: (id: string) => (
        <Tooltip title={id}>
          <Text code>{id.slice(0, 8)}...</Text>
        </Tooltip>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: ThreadStatus) => {
        const statusConfig: Record<ThreadStatus, { color: string; text: string }> = {
          idle: { color: 'default', text: '空闲' },
          running: { color: 'processing', text: '运行中' },
          paused: { color: 'warning', text: '已暂停' },
          complete: { color: 'success', text: '已完成' },
          failed: { color: 'error', text: '失败' },
        };
        const config = statusConfig[status] || { color: 'default', text: status };
        return <Tag color={config.color}>{config.text}</Tag>;
      },
    },
    {
      title: '当前阶段',
      dataIndex: 'currentPhase',
      key: 'currentPhase',
      width: 120,
      render: (phase: string) => (
        <Tag color={PhaseColors[phase as keyof typeof PhaseColors] || 'default'}>
          {PhaseLabels[phase as keyof typeof PhaseLabels] || phase}
        </Tag>
      ),
    },
    {
      title: '进度',
      key: 'progress',
      width: 200,
      render: (_: unknown, record: Thread) => {
        const phases = ['requirement', 'design', 'development', 'review', 'test', 'merge', 'complete'];
        const currentIndex = phases.indexOf(record.currentPhase);
        const percent = Math.round(((currentIndex + 1) / phases.length) * 100);
        return <Progress percent={percent} size="small" status="active" />;
      },
    },
    {
      title: '深度',
      dataIndex: 'depth',
      key: 'depth',
      width: 80,
      render: (depth: number) => (
        <Tooltip title={`Agent 调用深度: ${depth}/15`}>
          <Text>{depth}/15</Text>
        </Tooltip>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      render: (date: string) => new Date(date).toLocaleString(),
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      render: (_: unknown, record: Thread) => (
        <Space>
          <Button
            type="link"
            icon={<PlayCircleOutlined />}
            onClick={() => navigate(`/projects/${projectId}/threads/${record.id}`)}
          >
            进入
          </Button>
        </Space>
      ),
    },
  ];

  // 项目类型配置
  const projectTypeConfig: Record<string, { label: string; color: string }> = {
    service: { label: '服务', color: 'blue' },
    app: { label: '应用', color: 'green' },
    task: { label: '任务', color: 'orange' },
  };

  // 项目模式配置
  const projectModeConfig: Record<string, { label: string; color: string }> = {
    new: { label: '全新开发', color: 'cyan' },
    enhance: { label: '功能增强', color: 'purple' },
  };

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 400 }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!project) {
    return (
      <Empty
        description="项目不存在"
        image={Empty.PRESENTED_IMAGE_SIMPLE}
      >
        <Button type="primary" onClick={() => navigate('/projects')}>
          返回项目列表
        </Button>
      </Empty>
    );
  }

  return (
    <div className="project-detail">
      {/* 顶部导航 */}
      <div style={{ marginBottom: 24 }}>
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/projects')}>
            返回项目列表
          </Button>
        </Space>
      </div>

      {/* 项目信息卡片 */}
      <Card
        title={
          <Space>
            <FolderOutlined />
            <span>{project.name}</span>
            <Tag color={project.status === 'active' ? 'green' : 'default'}>
              {project.status === 'active' ? '活跃' : '归档'}
            </Tag>
          </Space>
        }
        extra={
          <Space>
            <Button icon={<EditOutlined />} onClick={() => {
              form.setFieldsValue(project);
              setEditModalVisible(true);
            }}>
              编辑
            </Button>
            <Button danger icon={<DeleteOutlined />} onClick={handleDeleteProject}>
              删除
            </Button>
          </Space>
        }
      >
        <Descriptions column={3} bordered size="small">
          <Descriptions.Item label="项目类型">
            <Tag color={projectTypeConfig[project.type || 'service']?.color || 'default'}>
              {projectTypeConfig[project.type || 'service']?.label || project.type}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="开发模式">
            <Tag color={projectModeConfig[project.mode || 'new']?.color || 'default'}>
              {projectModeConfig[project.mode || 'new']?.label || project.mode}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="状态">
            <Tag color={project.status === 'active' ? 'green' : 'default'}>
              {project.status === 'active' ? '活跃' : '归档'}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="描述" span={3}>
            {project.description || '暂无描述'}
          </Descriptions.Item>
          <Descriptions.Item label="仓库地址" span={3}>
            {project.repositoryUrl ? (
              <a href={project.repositoryUrl} target="_blank" rel="noopener noreferrer">
                {project.repositoryUrl}
              </a>
            ) : (
              <Text type="secondary">未配置</Text>
            )}
          </Descriptions.Item>
          <Descriptions.Item label="创建时间">
            {new Date(project.createdAt).toLocaleString()}
          </Descriptions.Item>
          <Descriptions.Item label="更新时间">
            {new Date(project.updatedAt).toLocaleString()}
          </Descriptions.Item>
          <Descriptions.Item label="任务数量">
            <Tag color="blue">{threads.length}</Tag>
          </Descriptions.Item>
        </Descriptions>
      </Card>

      {/* Tab 页签：任务列表 / 项目设置 */}
      <Card style={{ marginTop: 16 }}>
        <Tabs defaultActiveKey="threads">
          <TabPane
            tab={
              <span>
                <PlayCircleOutlined />
                任务列表
              </span>
            }
            key="threads"
          >
            <div style={{ marginBottom: 16 }}>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => setCreateThreadModalVisible(true)}
              >
                新建任务
              </Button>
            </div>

            {threads.length > 0 ? (
              <Table
                dataSource={threads}
                columns={threadColumns}
                rowKey="id"
                pagination={{ pageSize: 10 }}
              />
            ) : (
              <Empty
                description="暂无任务"
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              >
                <Button type="primary" onClick={() => setCreateThreadModalVisible(true)}>
                  创建第一个任务
                </Button>
              </Empty>
            )}
          </TabPane>

          <TabPane
            tab={
              <span>
                <SettingOutlined />
                项目设置
              </span>
            }
            key="settings"
          >
            <Card>
              <Descriptions title="项目配置" column={2} bordered>
                <Descriptions.Item label="项目 ID">
                  <Text code>{project.id}</Text>
                </Descriptions.Item>
                <Descriptions.Item label="项目名称">
                  {project.name}
                </Descriptions.Item>
                <Descriptions.Item label="项目类型">
                  {projectTypeConfig[project.type || 'service']?.label || project.type}
                </Descriptions.Item>
                <Descriptions.Item label="开发模式">
                  {projectModeConfig[project.mode || 'new']?.label || project.mode}
                </Descriptions.Item>
                <Descriptions.Item label="绑定工作流" span={2}>
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <Select
                      style={{ width: 300 }}
                      placeholder="选择工作流模板"
                      value={project.workflowTemplateId || undefined}
                      loading={loadingTemplates}
                      allowClear
                      onChange={async (value) => {
                        try {
                          await api.projects.update(projectId!, { workflowTemplateId: value });
                          message.success('工作流绑定已更新');
                          loadProjectData();
                        } catch (error) {
                          message.error('更新失败');
                        }
                      }}
                    >
                      {workflowTemplates.map((t) => (
                        <Option key={t.id} value={t.id}>
                          {t.name} {t.isDefault ? '(默认)' : ''} {t.isSystem ? '[系统]' : ''}
                        </Option>
                      ))}
                    </Select>
                    {project.workflowTemplateId ? (
                      <Space direction="vertical" size="small">
                        <Text type="secondary">当前绑定：</Text>
                        {(() => {
                          const boundTemplate = workflowTemplates.find(t => t.id === project.workflowTemplateId);
                          if (!boundTemplate) return <Text type="warning">工作流不存在</Text>;
                          return (
                            <>
                              <Text strong>{boundTemplate.name}</Text>
                              <div>
                                <Text type="secondary">检查点：</Text>
                                {boundTemplate.checkpoints?.map((cp) => (
                                  <Tag key={cp} color="orange" style={{ marginBottom: 4 }}>{cp}</Tag>
                                ))}
                              </div>
                            </>
                          );
                        })()}
                      </Space>
                    ) : (
                      <Text type="secondary">
                        未绑定工作流，将使用系统默认工作流
                        {workflowTemplates.find(t => t.isDefault) && (
                          <>：{workflowTemplates.find(t => t.isDefault)?.name}</>
                        )}
                      </Text>
                    )}
                  </Space>
                </Descriptions.Item>
              </Descriptions>

              <Divider />

              <div style={{ marginTop: 16 }}>
                <Button type="primary" icon={<EditOutlined />} onClick={() => {
                  form.setFieldsValue(project);
                  setEditModalVisible(true);
                }}>
                  编辑项目信息
                </Button>
              </div>
            </Card>
          </TabPane>
        </Tabs>
      </Card>

      {/* 编辑项目弹窗 */}
      <Modal
        title="编辑项目"
        open={editModalVisible}
        onOk={() => form.submit()}
        onCancel={() => setEditModalVisible(false)}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={handleUpdateProject}>
          <Form.Item name="name" label="项目名称" rules={[{ required: true }]}>
            <Input placeholder="请输入项目名称" />
          </Form.Item>
          <Form.Item name="description" label="项目描述">
            <TextArea rows={3} placeholder="请输入项目描述" />
          </Form.Item>
          <Form.Item name="type" label="项目类型">
            <Select placeholder="请选择项目类型">
              <Option value="service">服务</Option>
              <Option value="app">应用</Option>
              <Option value="task">任务</Option>
            </Select>
          </Form.Item>
          <Form.Item name="mode" label="开发模式">
            <Select placeholder="请选择开发模式">
              <Option value="new">全新开发</Option>
              <Option value="enhance">功能增强</Option>
            </Select>
          </Form.Item>
          <Form.Item name="repositoryUrl" label="仓库地址">
            <Input placeholder="https://github.com/user/repo" />
          </Form.Item>
          <Form.Item name="workflowTemplateId" label="绑定工作流">
            <Select placeholder="选择工作流模板" allowClear loading={loadingTemplates}>
              {workflowTemplates.map((t) => (
                <Option key={t.id} value={t.id}>
                  {t.name} {t.isDefault ? '(默认)' : ''} {t.isSystem ? '[系统]' : ''}
                </Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="status" label="状态">
            <Select>
              <Option value="active">活跃</Option>
              <Option value="archived">归档</Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      {/* 创建任务弹窗 */}
      <Modal
        title="新建开发任务"
        open={createThreadModalVisible}
        onOk={() => threadForm.submit()}
        onCancel={() => {
          setCreateThreadModalVisible(false);
          threadForm.resetFields();
        }}
        width={500}
      >
        <Form form={threadForm} layout="vertical" onFinish={handleCreateThread}>
          <Form.Item name="name" label="任务名称（可选）">
            <Input placeholder="为任务起个名字" />
          </Form.Item>
          <Divider style={{ margin: '12px 0' }} />
          <div>
            <Text strong>使用工作流：</Text>
            <div style={{ marginTop: 8 }}>
              {project.workflowTemplateId ? (
                <>
                  <Tag color="blue">
                    {workflowTemplates.find(t => t.id === project.workflowTemplateId)?.name || '未知工作流'}
                  </Tag>
                  <Text type="secondary" style={{ marginLeft: 8 }}>（来自项目绑定）</Text>
                </>
              ) : (
                <>
                  <Tag color="gold">
                    {workflowTemplates.find(t => t.isDefault)?.name || '系统默认'}
                  </Tag>
                  <Text type="secondary" style={{ marginLeft: 8 }}>（系统默认工作流）</Text>
                </>
              )}
            </div>
          </div>
          <Text type="secondary" style={{ display: 'block', marginTop: 16 }}>
            创建后将进入开发工作台，您可以描述您的需求并启动 AI 开发流程。
          </Text>
        </Form>
      </Modal>
    </div>
  );
};

export default ProjectDetail;