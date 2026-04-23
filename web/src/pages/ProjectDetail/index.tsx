import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Card,
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
  Divider,
  Popconfirm,
  Tooltip,
} from 'antd';
import {
  ArrowLeftOutlined,
  PlusOutlined,
  FolderOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
} from '@ant-design/icons';
import api from '@/api/client';
import type { Project, Thread, WorkflowTemplate } from '@/types';

const { Text } = Typography;
const { Option } = Select;

/**
 * 项目详情页
 * PRD Section 2.1.1 - 项目空间管理
 *
 * 功能要点：
 * - 项目属性展示与编辑（紧凑布局）
 * - 团队绑定（可直接修改）
 * - 项目任务列表 (Thread列表)
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

  // 任务创建时的团队选择状态
  const [selectedTeamId, setSelectedTeamId] = useState<string | undefined>(undefined);

  useEffect(() => {
    if (projectId) {
      loadProjectData();
    }
  }, [projectId]);

  // 获取Agent团队列表
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

  // 打开创建任务弹窗时初始化团队选择
  const openCreateThreadModal = () => {
    // 默认继承项目团队
    setSelectedTeamId(project?.workflowTemplateId || undefined);
    setCreateThreadModalVisible(true);
  };

  // 创建任务时传递选择的团队
  const handleCreateThread = async (values: { name?: string }) => {
    try {
      const thread = await api.threads.create(
        projectId!,
        values.name || '新任务',
        selectedTeamId
      );
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

  // 更新团队绑定
  const handleTeamChange = async (value: string | undefined) => {
    try {
      await api.projects.update(projectId!, { workflowTemplateId: value });
      message.success('团队绑定已更新');
      loadProjectData();
    } catch (error) {
      message.error('更新失败');
    }
  };

  // 更新任务的团队绑定
  const handleThreadTeamChange = async (threadId: string, workflowTemplateId: string | undefined) => {
    try {
      await api.threads.update(threadId, { workflowTemplateId });
      message.success('团队已更新');
      loadProjectData();
    } catch (error) {
      message.error('更新失败');
    }
  };

  // Thread 列表列定义
  const threadColumns = [
    {
      title: '任务名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      render: (name: string, record: Thread) => (
        <a onClick={() => navigate(`/projects/${projectId}/threads/${record.id}`)}>
          {name || '未命名任务'}
        </a>
      ),
    },
    {
      title: '团队',
      dataIndex: 'workflowTemplateId',
      key: 'workflowTemplateId',
      width: 180,
      render: (templateId: string | undefined, record: Thread) => (
        <Select
          style={{ width: 160 }}
          size="small"
          value={templateId || undefined}
          placeholder="选择团队"
          loading={loadingTemplates}
          onChange={(value) => handleThreadTeamChange(record.id, value)}
        >
          {workflowTemplates.map((t) => (
            <Option key={t.id} value={t.id}>
              {t.name} {t.isDefault ? '(默认)' : ''}
            </Option>
          ))}
        </Select>
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
      width: 150,
      render: (_: unknown, record: Thread) => (
        <Space>
          <Button
            type="link"
            icon={<PlayCircleOutlined />}
            onClick={() => navigate(`/projects/${projectId}/threads/${record.id}`)}
          >
            进入
          </Button>
          <Popconfirm
            title="确定删除此任务？"
            description="删除后无法恢复"
            onConfirm={async () => {
              try {
                await api.threads.delete(record.id);
                message.success('任务已删除');
                loadProjectData();
              } catch (error) {
                console.error('Failed to delete thread:', error);
                message.error('删除失败');
              }
            }}
            okText="删除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
          >
            <Button
              type="link"
              icon={<DeleteOutlined />}
              danger
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

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
    <div className="project-detail" style={{ padding: '0 16px' }}>
      {/* 顶部导航 */}
      <div style={{ marginBottom: 16 }}>
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/projects')}>
            返回项目列表
          </Button>
        </Space>
      </div>

      {/* 项目信息卡片 - 紧凑布局 */}
      <Card
        style={{ marginBottom: 16 }}
        title={
          <Space>
            <FolderOutlined />
            <span style={{ fontSize: 16 }}>{project.name}</span>
          </Space>
        }
        extra={
          <Space>
            <Button size="small" icon={<EditOutlined />} onClick={() => {
              form.setFieldsValue(project);
              setEditModalVisible(true);
            }}>
              编辑
            </Button>
            <Button size="small" danger icon={<DeleteOutlined />} onClick={handleDeleteProject}>
              删除
            </Button>
          </Space>
        }
      >
        <Descriptions column={2} size="small" labelStyle={{ width: 80, paddingBottom: 8 }} contentStyle={{ paddingBottom: 8, flex: 1 }}>
          {/* 第一行 */}
          <Descriptions.Item label="描述">
            <Tooltip title={project.description || '-'} placement="topLeft">
              <Text ellipsis>{project.description || '-'}</Text>
            </Tooltip>
          </Descriptions.Item>
          <Descriptions.Item label="本地路径">
            <Tooltip title={project.localPath || '-'} placement="topLeft">
              <Text ellipsis>{project.localPath || '-'}</Text>
            </Tooltip>
          </Descriptions.Item>
          {/* 第二行 */}
          <Descriptions.Item label="绑定团队">
            <Select
              style={{ width: 180 }}
              placeholder="选择团队（必填）"
              value={project.workflowTemplateId || undefined}
              loading={loadingTemplates}
              onChange={handleTeamChange}
              size="small"
            >
              {workflowTemplates.map((t) => (
                <Option key={t.id} value={t.id}>
                  {t.name} {t.isDefault ? '(默认)' : ''}
                </Option>
              ))}
            </Select>
          </Descriptions.Item>
          <Descriptions.Item label="仓库地址">
            <Tooltip title={project.repositoryUrl || '-'} placement="topLeft">
              <Text ellipsis>{project.repositoryUrl || '-'}</Text>
            </Tooltip>
          </Descriptions.Item>
        </Descriptions>
      </Card>

      {/* 任务列表版块 */}
      <Card
        title={
          <Space>
            <PlayCircleOutlined />
            <span>任务列表</span>
            <Tag color="blue">{threads.length}</Tag>
          </Space>
        }
        extra={
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={openCreateThreadModal}>
            新建任务
          </Button>
        }
      >
        {threads.length > 0 ? (
          <Table
            dataSource={threads}
            columns={threadColumns}
            rowKey="id"
            pagination={{ pageSize: 10 }}
            size="small"
          />
        ) : (
          <Empty
            description="暂无任务"
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          >
            <Button type="primary" onClick={openCreateThreadModal}>
              创建第一个任务
            </Button>
          </Empty>
        )}
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
            <Input.TextArea rows={3} placeholder="请输入项目描述" />
          </Form.Item>
          <Form.Item name="workflowTemplateId" label="绑定团队" rules={[{ required: true, message: '请选择团队' }]}>
            <Select placeholder="选择Agent团队" loading={loadingTemplates}>
              {workflowTemplates.map((t) => (
                <Option key={t.id} value={t.id}>
                  {t.name} {t.isDefault ? '(默认)' : ''} {t.isSystem ? '[系统]' : ''}
                </Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="type" label="项目类型">
            <Select>
              <Option value="service">服务</Option>
              <Option value="app">应用</Option>
              <Option value="task">任务</Option>
            </Select>
          </Form.Item>
          <Form.Item name="mode" label="项目模式">
            <Select>
              <Option value="new">全新开发</Option>
              <Option value="enhance">功能增强</Option>
            </Select>
          </Form.Item>
          <Form.Item name="repositoryUrl" label="仓库地址">
            <Input placeholder="Git 仓库地址（可选）" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 创建任务弹窗 - 支持团队选择 */}
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
            <Input placeholder="为任务起个名字" autoComplete="off" />
          </Form.Item>

          <Divider style={{ margin: '12px 0' }} />

          <Form.Item label="使用团队">
            <Space direction="vertical" style={{ width: '100%' }}>
              <Select
                style={{ width: 280 }}
                value={selectedTeamId}
                onChange={(value) => {
                  setSelectedTeamId(value);
                }}
                allowClear
                loading={loadingTemplates}
                placeholder="选择团队"
              >
                {workflowTemplates.map((t) => (
                  <Option key={t.id} value={t.id}>
                    {t.name} {t.isDefault ? '(默认)' : ''} {t.isSystem ? '[系统]' : ''}
                  </Option>
                ))}
              </Select>

              {/* 提示信息 */}
              <div style={{ marginTop: 4 }}>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  默认继承项目团队，可按任务单独指定
                </Text>
              </div>
            </Space>
          </Form.Item>

          <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 12 }}>
            创建后将进入开发工作台，您可以描述您的需求并启动 AI 开发流程。
          </Text>
        </Form>
      </Modal>
    </div>
  );
};

export default ProjectDetail;