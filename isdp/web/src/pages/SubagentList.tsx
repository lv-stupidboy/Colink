import React, { useEffect, useState, useCallback } from 'react';
import {
  Card, Button, Modal, Form, Input, message, Space, Typography,
  Popconfirm, Empty, Spin, Pagination, Table, Tooltip, Upload, Radio, Select
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  RobotOutlined,
  EyeOutlined,
  CloudUploadOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { Subagent, SubagentListResponse, Skill } from '@/types';

const { Title, Text, Paragraph } = Typography;
const { TextArea } = Input;

// 根据子代理名称生成头像
const generateAvatar = (name: string): { initials: string; color: string } => {
  const words = name.split(/[-_\s]+/);
  let initials = '';
  if (words.length >= 2) {
    initials = (words[0][0] + words[1][0]).toUpperCase();
  } else if (name.length >= 2) {
    initials = name.substring(0, 2).toUpperCase();
  } else {
    initials = name.toUpperCase();
  }

  const colors = [
    '#722ed1', '#13c2c2', '#eb2f96', '#fa8c16', '#52c41a',
    '#1890ff', '#2f54eb', '#faad14', '#a0d911', '#f5222d'
  ];
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash);
  }
  const color = colors[Math.abs(hash) % colors.length];

  return { initials, color };
};

// 子代理头像组件
const SubagentAvatar: React.FC<{ name: string }> = ({ name }) => {
  const { initials, color } = generateAvatar(name);
  return (
    <div
      style={{
        width: 36,
        height: 36,
        borderRadius: 8,
        background: `linear-gradient(135deg, ${color}dd, ${color})`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: '#fff',
        fontWeight: 600,
        fontSize: 13,
        boxShadow: `0 2px 6px ${color}40`,
      }}
    >
      {initials}
    </div>
  );
};

const SubagentList: React.FC = () => {
  const [subagents, setSubagents] = useState<Subagent[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [modalVisible, setModalVisible] = useState(false);
  const [viewModalVisible, setViewModalVisible] = useState(false);
  const [editingSubagent, setEditingSubagent] = useState<Subagent | null>(null);
  const [viewingSubagent, setViewingSubagent] = useState<Subagent | null>(null);
  const [searchText, setSearchText] = useState('');
  const [form] = Form.useForm();
  const [createMethod, setCreateMethod] = useState<'upload' | 'manual'>('manual');
  const [isAfterUpload, setIsAfterUpload] = useState(false);
  // 技能绑定相关
  const [skills, setSkills] = useState<Skill[]>([]);
  const [selectedSkillIds, setSelectedSkillIds] = useState<string[]>([]);

  const loadSubagents = useCallback(async () => {
    setLoading(true);
    try {
      const result: SubagentListResponse = await api.subagents.list({
        search: searchText,
        page,
        pageSize,
      });
      setSubagents(result.data || []);
      setTotal(result.total || 0);
    } catch (error) {
      message.error('加载子代理列表失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText]);

  useEffect(() => {
    loadSubagents();
  }, [loadSubagents]);

  // 加载技能列表
  const loadSkills = async () => {
    try {
      const result = await api.skills.list({ pageSize: 100 });
      setSkills(result.data || []);
    } catch (error) {
      console.error('加载技能列表失败', error);
      setSkills([]);
    }
  };

  // 加载子代理绑定的技能
  const loadSubagentSkills = async (subagentId: string) => {
    try {
      const result = await api.subagents.getSkills(subagentId);
      setSelectedSkillIds(result.skills?.map(s => s.id) || []);
    } catch (error) {
      console.error('加载子代理绑定的技能失败', error);
      setSelectedSkillIds([]);
    }
  };

  // 初始加载技能列表
  useEffect(() => {
    loadSkills();
  }, []);

  const handleCreate = () => {
    setEditingSubagent(null);
    setCreateMethod('manual');
    setIsAfterUpload(false);
    setSelectedSkillIds([]);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = async (record: Subagent) => {
    setEditingSubagent(record);
    setCreateMethod('manual');
    setIsAfterUpload(false);
    form.setFieldsValue({
      name: record.name,
      description: record.description,
      content: record.content,
    });
    // 加载已绑定的技能
    await loadSubagentSkills(record.id);
    setModalVisible(true);
  };

  const handleView = (record: Subagent) => {
    setViewingSubagent(record);
    setViewModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.subagents.delete(id);
      message.success('删除成功');
      loadSubagents();
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
      if (editingSubagent) {
        await api.subagents.update(editingSubagent.id, {
          description: values.description,
          content: values.content,
        });
        // 更新技能绑定
        await api.subagents.bindSkills(editingSubagent.id, selectedSkillIds);
        message.success('更新成功');
      } else {
        const newSubagent = await api.subagents.create({
          name: values.name,
          description: values.description,
          content: values.content,
        });
        // 为新创建的子代理绑定技能
        if (selectedSkillIds.length > 0) {
          await api.subagents.bindSkills(newSubagent.id, selectedSkillIds);
        }
        message.success('创建成功');
      }
      setModalVisible(false);
      loadSubagents();
    } catch (error: any) {
      const errorData = error.response?.data;
      if (errorData?.error) {
        message.error(errorData.error);
      } else {
        message.error('操作失败');
      }
    }
  };

  const handleSearch = (value: string) => {
    setSearchText(value);
    setPage(1);
  };

  // 上传处理
  const handleUploadSuccess = (response: Subagent) => {
    if (response && response.id) {
      setEditingSubagent(response);
      setIsAfterUpload(true);
      form.setFieldsValue({
        name: response.name,
        description: response.description || '',
        content: response.content,
      });
      message.success('子代理文件上传成功，请补充完整信息后保存');
    }
  };

  const handleUpload = (info: any) => {
    if (info.file.status === 'done') {
      handleUploadSuccess(info.file.response);
    } else if (info.file.status === 'error') {
      const errorData = info.file.response;
      message.error(errorData?.error || '上传失败');
    }
  };

  // 表格列定义
  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      render: (name: string) => (
        <Space>
          <SubagentAvatar name={name} />
          <Text strong style={{ fontSize: 14 }}>{name}</Text>
        </Space>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (description: string) => (
        <Tooltip title={description}>
          <Text type="secondary">{description || '暂无描述'}</Text>
        </Tooltip>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      render: (date: string) => (
        <Text type="secondary" style={{ fontSize: 12 }}>
          {date ? new Date(date).toLocaleString('zh-CN') : '-'}
        </Text>
      ),
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_: any, record: Subagent) => (
        <Space size="small">
          <Tooltip title="查看详情">
            <Button
              type="text"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => handleView(record)}
            />
          </Tooltip>
          <Tooltip title="编辑">
            <Button
              type="text"
              size="small"
              icon={<EditOutlined />}
              onClick={() => handleEdit(record)}
            />
          </Tooltip>
          <Popconfirm
            title="确定要删除这个子代理吗？"
            description="删除后将无法恢复"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Tooltip title="删除">
              <Button
                type="text"
                size="small"
                danger
                icon={<DeleteOutlined />}
              />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 12 }}>
      {/* 页面标题 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>
            <RobotOutlined style={{ marginRight: 8, color: 'var(--ant-color-primary)' }} />
            子代理管理
          </Title>
          <Text type="secondary">管理可复用的子代理配置</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新建子代理
        </Button>
      </div>

      {/* 搜索区域 */}
      <Card style={{ marginBottom: 16 }} styles={{ body: { padding: '12px 16px' } }}>
        <Input.Search
          placeholder="搜索子代理名称或描述..."
          allowClear
          style={{ width: 300 }}
          onSearch={handleSearch}
          enterButton
        />
      </Card>

      {/* 数据表格 */}
      <Card>
        <Spin spinning={loading}>
          <Table
            dataSource={subagents}
            columns={columns}
            rowKey="id"
            pagination={false}
            locale={{
              emptyText: (
                <Empty
                  description="暂无子代理"
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                />
              ),
            }}
          />
        </Spin>

        {/* 分页 */}
        {total > 0 && (
          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-end' }}>
            <Pagination
              current={page}
              pageSize={pageSize}
              total={total}
              onChange={(p, ps) => {
                setPage(p);
                setPageSize(ps);
              }}
              showSizeChanger
              showTotal={(t) => `共 ${t} 条`}
              pageSizeOptions={['10', '20', '50']}
            />
          </div>
        )}
      </Card>

      {/* 新建/编辑弹窗 */}
      <Modal
        title={editingSubagent ? '编辑子代理' : '新建子代理'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={700}
        okText="保存"
        destroyOnClose
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
        >
          {/* 创建方式选择 - 仅新建时显示 */}
          {!editingSubagent && !isAfterUpload && (
            <div style={{ marginBottom: 16, padding: 16, background: 'var(--ant-color-bg-container)', borderRadius: 8, border: '1px solid var(--ant-color-border)' }}>
              <Text strong style={{ marginRight: 12 }}>创建方式：</Text>
              <Radio.Group
                value={createMethod}
                onChange={(e) => setCreateMethod(e.target.value)}
              >
                <Radio.Button value="manual">
                  <EditOutlined /> 手动填写
                </Radio.Button>
                <Radio.Button value="upload">
                  <CloudUploadOutlined /> 本地上传
                </Radio.Button>
              </Radio.Group>

              {createMethod === 'upload' && (
                <div style={{ marginTop: 12 }}>
                  <Upload.Dragger
                    name="file"
                    action="/api/v1/subagents/upload"
                    accept=".md,.zip"
                    onChange={handleUpload}
                    multiple={false}
                    showUploadList={false}
                    beforeUpload={(file) => {
                      const isValid = file.name.endsWith('.md') || file.name.endsWith('.zip');
                      if (!isValid) {
                        message.error('只支持 .md 和 .zip 格式的文件');
                        return Upload.LIST_IGNORE;
                      }
                      const isLt2M = file.size / 1024 / 1024 < 2;
                      if (!isLt2M) {
                        message.error('文件大小不能超过 2MB');
                        return Upload.LIST_IGNORE;
                      }
                      return true;
                    }}
                  >
                    <p className="ant-upload-drag-icon">
                      <CloudUploadOutlined style={{ fontSize: 32, color: 'var(--ant-color-primary)' }} />
                    </p>
                    <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
                    <p className="ant-upload-hint" style={{ fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
                      支持 .md 和 .zip 格式，最大 2MB
                    </p>
                  </Upload.Dragger>
                </div>
              )}
            </div>
          )}

          <Form.Item
            name="name"
            label="名称"
            rules={[
              { required: true, message: '请输入名称' },
              { pattern: /^[a-z][a-z0-9-]*$/, message: '名称只能包含小写字母、数字和中划线，且必须以字母开头' }
            ]}
            extra="只允许小写字母、数字和中划线，如：code-reviewer"
          >
            <Input
              placeholder="如：code-reviewer"
              disabled={!!editingSubagent || isAfterUpload}
            />
          </Form.Item>

          <Form.Item
            name="description"
            label="描述"
          >
            <Input.TextArea
              rows={2}
              placeholder="简要描述这个子代理的用途"
            />
          </Form.Item>

          <Form.Item
            name="content"
            label="配置内容"
            rules={[{ required: true, message: '请输入配置内容' }]}
            extra="子代理的完整配置内容，支持 Markdown 格式"
          >
            <TextArea
              rows={12}
              placeholder="输入子代理的配置内容..."
              style={{ fontFamily: 'monospace' }}
            />
          </Form.Item>

          <Form.Item label="绑定技能">
            <Select
              mode="multiple"
              placeholder="选择要绑定的技能"
              value={selectedSkillIds}
              onChange={setSelectedSkillIds}
              style={{ width: '100%' }}
              optionLabelProp="label"
              options={skills.map(s => ({
                label: s.name,
                value: s.id,
                desc: s.description || '暂无描述',
              }))}
              optionRender={(option) => (
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                  <span style={{ fontWeight: 500 }}>{option.label}</span>
                  <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 400 }}>
                    {option.data.desc}
                  </span>
                </div>
              )}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 查看详情弹窗 */}
      <Modal
        title={
          <Space>
            <SubagentAvatar name={viewingSubagent?.name || ''} />
            <span>{viewingSubagent?.name}</span>
          </Space>
        }
        open={viewModalVisible}
        onCancel={() => setViewModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setViewModalVisible(false)}>
            关闭
          </Button>,
          <Button
            key="edit"
            type="primary"
            icon={<EditOutlined />}
            onClick={() => {
              setViewModalVisible(false);
              if (viewingSubagent) {
                handleEdit(viewingSubagent);
              }
            }}
          >
            编辑
          </Button>,
        ]}
        width={700}
      >
        {viewingSubagent && (
          <div>
            <Paragraph>
              <Text strong>描述：</Text>
              <br />
              <Text type="secondary">{viewingSubagent.description || '暂无描述'}</Text>
            </Paragraph>

            <Paragraph>
              <Text strong>配置内容：</Text>
            </Paragraph>
            <div
              style={{
                background: 'var(--ant-color-bg-container)',
                border: '1px solid var(--ant-color-border)',
                borderRadius: 8,
                padding: 12,
                maxHeight: 400,
                overflow: 'auto',
              }}
            >
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontFamily: 'monospace', fontSize: 13 }}>
                {viewingSubagent.content}
              </pre>
            </div>

            <div style={{ marginTop: 16, color: 'var(--ant-color-text-secondary)', fontSize: 12 }}>
              <Text type="secondary">
                创建时间：{viewingSubagent.createdAt ? new Date(viewingSubagent.createdAt).toLocaleString('zh-CN') : '-'}
              </Text>
              <br />
              <Text type="secondary">
                更新时间：{viewingSubagent.updatedAt ? new Date(viewingSubagent.updatedAt).toLocaleString('zh-CN') : '-'}
              </Text>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default SubagentList;