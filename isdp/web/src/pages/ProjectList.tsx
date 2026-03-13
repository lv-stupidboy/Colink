import React, { useEffect, useState } from 'react';
import { Table, Button, Card, Space, Modal, Form, Input, message, Tag, Select } from 'antd';
import { PlusOutlined, FolderOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '@/api/client';
import type { Project } from '@/types';

const { TextArea } = Input;
const { Option } = Select;

const ProjectList: React.FC = () => {
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    loadProjects();
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

  const columns = [
    {
      title: '项目名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: Project) => (
        <a onClick={() => navigate(`/projects/${record.id}`)}>{name}</a>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (type?: string) => {
        const typeMap: Record<string, string> = {
          service: '服务',
          app: '应用',
          task: '任务',
        };
        return typeMap[type || 'service'] || type || '服务';
      },
    },
    {
      title: '模式',
      dataIndex: 'mode',
      key: 'mode',
      render: (mode?: string) => {
        const modeMap: Record<string, string> = {
          new: '全新开发',
          enhance: '功能增强',
        };
        return modeMap[mode || 'new'] || mode || '全新开发';
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={status === 'active' ? 'green' : 'default'}>
          {status === 'active' ? '活跃' : '归档'}
        </Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (date: string) => new Date(date).toLocaleString(),
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: unknown, record: Project) => (
        <Space>
          <Button
            type="link"
            icon={<FolderOutlined />}
            onClick={() => navigate(`/projects/${record.id}`)}
          >
            进入项目
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Card
        title="项目列表"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalVisible(true)}>
            新建项目
          </Button>
        }
      >
        <Table
          dataSource={projects}
          columns={columns}
          rowKey="id"
          loading={loading}
        />
      </Card>

      <Modal
        title="新建项目"
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="name" label="项目名称" rules={[{ required: true, message: '请输入项目名称' }]}>
            <Input placeholder="请输入项目名称" />
          </Form.Item>
          <Form.Item name="description" label="项目描述">
            <TextArea placeholder="请输入项目描述" rows={3} />
          </Form.Item>
          <Form.Item name="type" label="项目类型" rules={[{ required: true, message: '请选择项目类型' }]}>
            <Select placeholder="请选择项目类型">
              <Option value="service">服务</Option>
              <Option value="app">应用</Option>
              <Option value="task">任务</Option>
            </Select>
          </Form.Item>
          <Form.Item name="mode" label="开发模式" rules={[{ required: true, message: '请选择开发模式' }]}>
            <Select placeholder="请选择开发模式">
              <Option value="new">全新开发</Option>
              <Option value="enhance">功能增强</Option>
            </Select>
          </Form.Item>
          <Form.Item name="existing_repo_url" label="现有仓库 URL">
            <Input placeholder="https://github.com/user/repo" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ProjectList;