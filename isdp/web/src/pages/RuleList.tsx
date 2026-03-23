import React, { useEffect, useState, useCallback } from 'react';
import {
  Card, Button, Modal, Form, Input, message, Space, Typography, Tag,
  Popconfirm, Empty, Spin, Pagination, Table, Tooltip, Upload, Radio, Select
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  SafetyOutlined,
  EyeOutlined,
  CloudUploadOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { Rule, RuleListResponse, RuleScope } from '@/types';

const { Title, Text, Paragraph } = Typography;

const RuleList: React.FC = () => {
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [modalVisible, setModalVisible] = useState(false);
  const [viewModalVisible, setViewModalVisible] = useState(false);
  const [editingRule, setEditingRule] = useState<Rule | null>(null);
  const [viewingRule, setViewingRule] = useState<Rule | null>(null);
  const [searchText, setSearchText] = useState('');
  const [scopeFilter, setScopeFilter] = useState<RuleScope | ''>('');
  const [form] = Form.useForm();
  const [createMethod, setCreateMethod] = useState<'upload' | 'manual'>('manual');
  const [isAfterUpload, setIsAfterUpload] = useState(false);

  const loadRules = useCallback(async () => {
    setLoading(true);
    try {
      const result: RuleListResponse = await api.rules.list({
        search: searchText,
        scope: scopeFilter || undefined,
        page,
        pageSize,
      });
      setRules(result.data || []);
      setTotal(result.total || 0);
    } catch (error) {
      message.error('加载规约列表失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText, scopeFilter]);

  useEffect(() => {
    loadRules();
  }, [loadRules]);

  const handleCreate = () => {
    setEditingRule(null);
    setCreateMethod('manual');
    setIsAfterUpload(false);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: Rule) => {
    setEditingRule(record);
    setCreateMethod('manual');
    setIsAfterUpload(false);
    form.setFieldsValue({
      name: record.name,
      description: record.description,
      scope: record.scope,
    });
    setModalVisible(true);
  };

  const handleView = (record: Rule) => {
    setViewingRule(record);
    setViewModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.rules.delete(id);
      message.success('删除成功');
      loadRules();
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
      if (editingRule) {
        await api.rules.update(editingRule.id, {
          description: values.description,
          scope: values.scope,
        });
        message.success('更新成功');
      } else {
        await api.rules.create({
          name: values.name,
          description: values.description,
          scope: values.scope,
        });
        message.success('创建成功');
      }
      setModalVisible(false);
      loadRules();
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

  const handleScopeFilter = (value: RuleScope | '') => {
    setScopeFilter(value);
    setPage(1);
  };

  // 上传处理
  const handleUploadSuccess = (response: { message: string; file_path: string }) => {
    if (response && response.file_path) {
      // 从文件路径提取名称
      const fileName = response.file_path.split('/').pop()?.replace('.md', '') || '';
      form.setFieldsValue({
        name: fileName,
        description: '',
        scope: 'public',
      });
      setIsAfterUpload(true);
      message.success('规约文件上传成功，请补充完整信息后保存');
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

  // Scope 标签颜色
  const getScopeTag = (scope: RuleScope) => {
    if (scope === 'public') {
      return <Tag color="blue">公共规约</Tag>;
    }
    return <Tag color="green">实例规约</Tag>;
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
          <SafetyOutlined style={{ color: 'var(--ant-color-primary)', fontSize: 16 }} />
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
      title: '范围',
      dataIndex: 'scope',
      key: 'scope',
      width: 120,
      render: (scope: RuleScope) => getScopeTag(scope),
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
      render: (_: any, record: Rule) => (
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
            title="确定要删除这个规约吗？"
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
            <SafetyOutlined style={{ marginRight: 8, color: 'var(--ant-color-primary)' }} />
            规约管理
          </Title>
          <Text type="secondary">管理 Agent 行为规约</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新建规约
        </Button>
      </div>

      {/* 搜索和过滤区域 */}
      <Card style={{ marginBottom: 16 }} styles={{ body: { padding: '12px 16px' } }}>
        <Space>
          <Input.Search
            placeholder="搜索规约名称或描述..."
            allowClear
            style={{ width: 300 }}
            onSearch={handleSearch}
            enterButton
          />
          <Select
            style={{ width: 150 }}
            placeholder="选择范围"
            allowClear
            value={scopeFilter || undefined}
            onChange={handleScopeFilter}
            options={[
              { label: '全部范围', value: '' },
              { label: '公共规约', value: 'public' },
              { label: '实例规约', value: 'instance' },
            ]}
          />
        </Space>
      </Card>

      {/* 数据表格 */}
      <Card>
        <Spin spinning={loading}>
          <Table
            dataSource={rules}
            columns={columns}
            rowKey="id"
            pagination={false}
            locale={{
              emptyText: (
                <Empty
                  description="暂无规约"
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
        title={editingRule ? '编辑规约' : '新建规约'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={600}
        okText="保存"
        destroyOnClose
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{ scope: 'public' }}
        >
          {/* 创建方式选择 - 仅新建时显示 */}
          {!editingRule && !isAfterUpload && (
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
                    action="/api/v1/rules/upload"
                    accept=".md"
                    onChange={handleUpload}
                    multiple={false}
                    showUploadList={false}
                    beforeUpload={(file) => {
                      const isValid = file.name.endsWith('.md');
                      if (!isValid) {
                        message.error('只支持 .md 格式的文件');
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
                      支持 .md 格式，最大 2MB
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
            extra="只允许小写字母、数字和中划线，如：code-style"
          >
            <Input
              placeholder="如：code-style"
              disabled={!!editingRule || isAfterUpload}
            />
          </Form.Item>

          <Form.Item
            name="description"
            label="描述"
          >
            <Input.TextArea
              rows={3}
              placeholder="简要描述这个规约的内容"
            />
          </Form.Item>

          <Form.Item
            name="scope"
            label="范围"
            rules={[{ required: true, message: '请选择范围' }]}
            extra="公共规约：对所有 Agent 生效；实例规约：仅对绑定的 Agent 生效"
          >
            <Radio.Group>
              <Radio value="public">
                <Space>
                  <Tag color="blue">公共规约</Tag>
                  <Text type="secondary" style={{ fontSize: 12 }}>对所有 Agent 生效</Text>
                </Space>
              </Radio>
              <Radio value="instance">
                <Space>
                  <Tag color="green">实例规约</Tag>
                  <Text type="secondary" style={{ fontSize: 12 }}>仅对绑定的 Agent 生效</Text>
                </Space>
              </Radio>
            </Radio.Group>
          </Form.Item>
        </Form>
      </Modal>

      {/* 查看详情弹窗 */}
      <Modal
        title={
          <Space>
            <SafetyOutlined style={{ color: 'var(--ant-color-primary)' }} />
            <span>{viewingRule?.name}</span>
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
              if (viewingRule) {
                handleEdit(viewingRule);
              }
            }}
          >
            编辑
          </Button>,
        ]}
        width={600}
      >
        {viewingRule && (
          <div>
            <Paragraph>
              <Text strong>名称：</Text>
              <br />
              <Text>{viewingRule.name}</Text>
            </Paragraph>

            <Paragraph>
              <Text strong>描述：</Text>
              <br />
              <Text type="secondary">{viewingRule.description || '暂无描述'}</Text>
            </Paragraph>

            <Paragraph>
              <Text strong>范围：</Text>
              <br />
              {getScopeTag(viewingRule.scope)}
              <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>
                {viewingRule.scope === 'public' ? '对所有 Agent 生效' : '仅对绑定的 Agent 生效'}
              </Text>
            </Paragraph>

            <div style={{ marginTop: 16, color: 'var(--ant-color-text-secondary)', fontSize: 12 }}>
              <Text type="secondary">
                创建时间：{viewingRule.createdAt ? new Date(viewingRule.createdAt).toLocaleString('zh-CN') : '-'}
              </Text>
              <br />
              <Text type="secondary">
                更新时间：{viewingRule.updatedAt ? new Date(viewingRule.updatedAt).toLocaleString('zh-CN') : '-'}
              </Text>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default RuleList;