import React, { useEffect, useState, useCallback } from 'react';
import {
  Table, Button, Card, Modal, Form, Input, Select, message, Space, Tag, Typography,
  Tooltip, Popconfirm, Tabs, Badge
} from 'antd';
import {
  PlusOutlined,
  StarOutlined, HeartOutlined, BookOutlined, CodeOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { Skill, SkillType, SkillSourceType } from '@/types';

const { Title, Text } = Typography;

const SkillLibrary: React.FC = () => {
  const [skills, setSkills] = useState<Skill[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingSkill, setEditingSkill] = useState<Skill | null>(null);
  const [searchText, setSearchText] = useState('');
  const [typeFilter, setTypeFilter] = useState<string>('');
  const [sourceFilter, setSourceFilter] = useState<string>('');
  const [form] = Form.useForm();

  const loadSkills = useCallback(async () => {
    setLoading(true);
    try {
      const result = await api.skills.list({
        page,
        pageSize,
        search: searchText,
        type: typeFilter,
        sourceType: sourceFilter,
      });
      setSkills(result.data);
      setTotal(result.total);
    } catch (error) {
      message.error('加载技能列表失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText, typeFilter, sourceFilter]);

  useEffect(() => {
    loadSkills();
  }, [loadSkills]);

  const handleCreate = () => {
    setEditingSkill(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: Skill) => {
    setEditingSkill(record);
    form.setFieldsValue(record);
    setModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.skills.delete(id);
      message.success('删除成功');
      loadSkills();
    } catch (error: any) {
      const errorData = error.response?.data;
      if (errorData?.error) {
        message.error(errorData.error);
      } else {
        message.error('删除失败');
      }
    }
  };

  const handleSubmit = async (values: any) => {
    try {
      if (editingSkill) {
        await api.skills.update(editingSkill.id, values);
        message.success('更新成功');
      } else {
        await api.skills.create(values);
        message.success('创建成功');
      }
      setModalVisible(false);
      loadSkills();
    } catch (error) {
      message.error('操作失败');
    }
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      render: (name: string, record: Skill) => (
        <Space>
          {record.type === 'rule' ? <BookOutlined /> : <CodeOutlined />}
          <span>{record.displayName || name}</span>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 80,
      render: (type: SkillType) => (
        <Tag color={type === 'rule' ? 'purple' : 'blue'}>
          {type === 'rule' ? '规则' : '技能'}
        </Tag>
      ),
    },
    {
      title: '分类',
      dataIndex: 'category',
      key: 'category',
      width: 120,
      render: (category: string) => category || '-',
    },
    {
      title: '来源',
      dataIndex: 'sourceType',
      key: 'sourceType',
      width: 100,
      render: (sourceType: SkillSourceType) => {
        const colorMap: Record<string, string> = {
          built_in: 'green',
          uploaded: 'orange',
          federated: 'cyan',
        };
        const labelMap: Record<string, string> = {
          built_in: '内置',
          uploaded: '上传',
          federated: '联邦',
        };
        return <Tag color={colorMap[sourceType]}>{labelMap[sourceType]}</Tag>;
      },
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (desc: string) => (
        <Tooltip title={desc}>
          {desc || '-'}
        </Tooltip>
      ),
    },
    {
      title: '统计',
      key: 'stats',
      width: 180,
      render: (_: unknown, record: Skill) => (
        <Space size="small">
          <Tooltip title="使用次数">
            <Badge count={record.useCount} showZero style={{ backgroundColor: '#52c41a' }} />
          </Tooltip>
          <Tooltip title="点赞">
            <StarOutlined /> {record.starCount}
          </Tooltip>
          <Tooltip title="收藏">
            <HeartOutlined /> {record.favoriteCount}
          </Tooltip>
        </Space>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 150,
      fixed: 'right' as const,
      render: (_: unknown, record: Skill) => (
        <Space size="small">
          <Button type="link" size="small" onClick={() => handleEdit(record)}>
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这个技能吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className="skill-library">
      <div style={{ marginBottom: 24 }}>
        <Title level={2}>技能库</Title>
        <Text type="secondary">管理可复用的技能和规则</Text>
      </div>

      <Card>
        <Tabs defaultActiveKey="all" onChange={(key) => setSourceFilter(key === 'all' ? '' : key)}>
          <Tabs.TabPane tab="全部" key="all" />
          <Tabs.TabPane tab="内置" key="built_in" />
          <Tabs.TabPane tab="上传" key="uploaded" />
          <Tabs.TabPane tab="联邦" key="federated" />
        </Tabs>

        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
          <Space>
            <Select
              placeholder="类型筛选"
              style={{ width: 120 }}
              allowClear
              onChange={(value) => setTypeFilter(value || '')}
            >
              <Select.Option value="skill">技能</Select.Option>
              <Select.Option value="rule">规则</Select.Option>
            </Select>
            <Input.Search
              placeholder="搜索技能..."
              allowClear
              style={{ width: 200 }}
              onSearch={(value) => setSearchText(value)}
            />
          </Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            新建技能
          </Button>
        </div>

        <Table
          dataSource={skills}
          columns={columns}
          rowKey="id"
          loading={loading}
          scroll={{ x: 'max-content' }}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条`,
            onChange: (p, ps) => {
              setPage(p);
              setPageSize(ps);
            },
          }}
        />
      </Card>

      <Modal
        title={editingSkill ? '编辑技能' : '新建技能'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{ type: 'skill', sourceType: 'uploaded', version: '1.0.0' }}
        >
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="技能唯一标识（如 java-coding-standards）" disabled={!!editingSkill} />
          </Form.Item>

          <Form.Item name="displayName" label="显示名称">
            <Input placeholder="显示名称" />
          </Form.Item>

          <Form.Item name="type" label="类型">
            <Select>
              <Select.Option value="skill">技能</Select.Option>
              <Select.Option value="rule">规则</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item name="sourceType" label="来源">
            <Select disabled={!!editingSkill}>
              <Select.Option value="built_in">内置</Select.Option>
              <Select.Option value="uploaded">上传</Select.Option>
              <Select.Option value="federated">联邦</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item name="category" label="分类">
            <Input placeholder="如：开发规范、中间件、前端" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="技能描述" />
          </Form.Item>

          <Form.Item name="version" label="版本">
            <Input placeholder="如：1.0.0" />
          </Form.Item>

          <Form.Item name="isPublic" label="公开">
            <Select>
              <Select.Option value={false}>私有</Select.Option>
              <Select.Option value={true}>公开</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default SkillLibrary;