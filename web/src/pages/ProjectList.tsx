import React, { useEffect, useState } from 'react';
import { Table, Button, Card, Space, Modal, Form, Input, Select, message, Tag, Typography, Popconfirm } from 'antd';
import { PlusOutlined, FolderOutlined, FolderOpenOutlined, DeleteOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '@/api/client';
import type { Project, WorkflowTemplate } from '@/types';
import PathSelector from '@/components/PathSelector';

const { Title, Text } = Typography;

const ProjectList: React.FC = () => {
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[]>([]);
  const [workflowTemplates, setWorkflowTemplates] = useState<WorkflowTemplate[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [pathSelectorVisible, setPathSelectorVisible] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    loadProjects();
    loadWorkflowTemplates();
  }, []);

  const loadProjects = async () => {
    setLoading(true);
    try {
      const data = await api.projects.list();
      // 处理可能返回 null 的情况
      setProjects((data as unknown as Project[]) || []);
    } catch (error) {
      message.error('加载项目列表失败');
    } finally {
      setLoading(false);
    }
  };

  const loadWorkflowTemplates = async () => {
    try {
      const data = await api.workflows.list();
      setWorkflowTemplates(data || []);
    } catch (error) {
      console.error('加载Agent团队失败', error);
    }
  };

  const handleCreate = async (values: Partial<Project>) => {
    try {
      const newProject = await api.projects.create(values);
      message.success('项目创建成功');
      setModalVisible(false);
      form.resetFields();
      loadProjects();
      // 创建成功后跳转到项目详情页
      navigate(`/projects/${(newProject as unknown as Project).id}`);
    } catch (error) {
      message.error('创建项目失败');
    }
  };

  // 处理路径选择
  const handlePathSelect = (path: string) => {
    form.setFieldsValue({ localPath: path });
    setPathSelectorVisible(false);
  };

  // 获取Agent团队名称
  const getWorkflowTemplateName = (templateId?: string) => {
    if (!templateId) return '-';
    const template = workflowTemplates.find(t => t.id === templateId);
    return template?.name || '-';
  };

  const columns = [
    {
      title: '项目名称',
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: (name: string, record: Project) => (
        <a onClick={() => navigate(`/projects/${record.id}`)}>{name}</a>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 80,
      render: (status: string) => (
        <Tag color={status === 'active' ? 'green' : 'default'}>
          {status === 'active' ? '活跃' : '归档'}
        </Tag>
      ),
    },
    {
      title: 'Agent团队',
      dataIndex: 'workflowTemplateId',
      key: 'workflowTemplateId',
      width: 150,
      render: (templateId?: string) => getWorkflowTemplateName(templateId),
    },
    {
      title: '本地路径',
      dataIndex: 'localPath',
      key: 'localPath',
      ellipsis: true,
      render: (path?: string) => path || '-',
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
      fixed: 'right' as const,
      render: (_: unknown, record: Project) => (
        <Space>
          <Button
            type="link"
            icon={<FolderOutlined />}
            onClick={() => navigate(`/projects/${record.id}`)}
          >
            进入
          </Button>
          <Popconfirm
            title="确定删除此项目？"
            description="删除后无法恢复，项目下的任务也会被删除"
            onConfirm={async () => {
              try {
                await api.projects.delete(record.id);
                message.success('项目已删除');
                loadProjects();
              } catch (error) {
                console.error('Failed to delete project:', error);
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

  return (
    <div style={{ padding: 12 }}>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>项目管理</Title>
          <Text type="secondary">管理开发项目，配置团队和 Agent</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalVisible(true)}>
          新建项目
        </Button>
      </div>

      <Card>
        <Table
          dataSource={projects}
          columns={columns}
          rowKey="id"
          loading={loading}
          scroll={{ x: 'max-content' }}
        />
      </Card>

      <Modal
        title="新建项目"
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate} initialValues={{ type: 'service', mode: 'new' }}>
          <Form.Item name="name" label="项目名称" rules={[{ required: true, message: '请输入项目名称' }]}>
            <Input placeholder="请输入项目名称" autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="localPath"
            label="本地路径"
            rules={[{ required: true, message: '请选择本地路径' }]}
          >
            <Input
              placeholder="点击选择或输入本地路径"
              addonAfter={
                <Button
                  icon={<FolderOpenOutlined />}
                  onClick={() => setPathSelectorVisible(true)}
                  style={{ border: 'none', background: 'transparent' }}
                >
                  浏览
                </Button>
              }
            />
          </Form.Item>
          <Form.Item name="type" label="项目类型" rules={[{ required: true }]}>
            <Select placeholder="选择项目类型">
              <Select.Option value="service">服务型项目</Select.Option>
              <Select.Option value="app">应用型项目</Select.Option>
              <Select.Option value="task">任务型项目</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="mode" label="创建模式" rules={[{ required: true }]}>
            <Select placeholder="选择创建模式">
              <Select.Option value="new">全新项目</Select.Option>
              <Select.Option value="enhance">增强现有项目</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      {/* 路径选择器 */}
      <PathSelector
        visible={pathSelectorVisible}
        onSelect={handlePathSelect}
        onCancel={() => setPathSelectorVisible(false)}
        title="选择项目保存路径"
      />
    </div>
  );
};

export default ProjectList;