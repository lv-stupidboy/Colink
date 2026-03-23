import React, { useEffect, useState, useCallback } from 'react';
import {
  Card, Button, Modal, Form, Input, message, Space, Typography, Tag,
  Popconfirm, Empty, Spin, Pagination, Table, Tooltip, Upload, Radio, Select
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  CodeOutlined,
  EyeOutlined,
  CloudUploadOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { Command, CommandListResponse, Skill } from '@/types';

const { Title, Text, Paragraph } = Typography;

const CommandList: React.FC = () => {
  const [commands, setCommands] = useState<Command[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [modalVisible, setModalVisible] = useState(false);
  const [viewModalVisible, setViewModalVisible] = useState(false);
  const [skillModalVisible, setSkillModalVisible] = useState(false);
  const [editingCommand, setEditingCommand] = useState<Command | null>(null);
  const [viewingCommand, setViewingCommand] = useState<Command | null>(null);
  const [searchText, setSearchText] = useState('');
  const [form] = Form.useForm();
  const [createMethod, setCreateMethod] = useState<'upload' | 'manual'>('manual');
  const [isAfterUpload, setIsAfterUpload] = useState(false);
  const [commandSkills, setCommandSkills] = useState<Skill[]>([]);
  const [allSkills, setAllSkills] = useState<Skill[]>([]);
  const [selectedSkillIds, setSelectedSkillIds] = useState<string[]>([]);
  const [editingSkillCommandId, setEditingSkillCommandId] = useState<string | null>(null);
  const [commandSkillCounts, setCommandSkillCounts] = useState<Record<string, number>>({});

  const loadCommands = useCallback(async () => {
    setLoading(true);
    try {
      const result: CommandListResponse = await api.commands.list({
        search: searchText,
        page,
        pageSize,
      });
      setCommands(result.data || []);
      setTotal(result.total || 0);

      // 加载每个 command 的关联技能数
      const counts: Record<string, number> = {};
      for (const cmd of result.data || []) {
        try {
          const skillsRes = await api.commands.getSkills(cmd.id);
          counts[cmd.id] = skillsRes.count || 0;
        } catch {
          counts[cmd.id] = 0;
        }
      }
      setCommandSkillCounts(counts);
    } catch (error) {
      message.error('加载命令列表失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText]);

  const loadAllSkills = useCallback(async () => {
    try {
      const result = await api.skills.list({ pageSize: 100 });
      setAllSkills(result.data || []);
    } catch (error) {
      console.error('加载技能列表失败', error);
    }
  }, []);

  useEffect(() => {
    loadCommands();
  }, [loadCommands]);

  useEffect(() => {
    loadAllSkills();
  }, [loadAllSkills]);

  const handleCreate = () => {
    setEditingCommand(null);
    setCreateMethod('manual');
    setIsAfterUpload(false);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: Command) => {
    setEditingCommand(record);
    setCreateMethod('manual');
    setIsAfterUpload(false);
    form.setFieldsValue({
      name: record.name,
      description: record.description,
    });
    setModalVisible(true);
  };

  const handleView = (record: Command) => {
    setViewingCommand(record);
    setViewModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.commands.delete(id);
      message.success('删除成功');
      loadCommands();
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
      if (editingCommand) {
        await api.commands.update(editingCommand.id, {
          description: values.description,
        });
        message.success('更新成功');
      } else {
        await api.commands.create({
          name: values.name,
          description: values.description,
        });
        message.success('创建成功');
      }
      setModalVisible(false);
      loadCommands();
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
  const handleUploadSuccess = (response: { message: string; file_path: string }) => {
    if (response && response.file_path) {
      // 从文件路径提取名称
      const fileName = response.file_path.split('/').pop()?.replace('.md', '') || '';
      form.setFieldsValue({
        name: fileName,
        description: '',
      });
      setIsAfterUpload(true);
      message.success('命令文件上传成功，请补充完整信息后保存');
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

  // 技能绑定相关
  const handleManageSkills = async (commandId: string) => {
    setEditingSkillCommandId(commandId);
    try {
      const result = await api.commands.getSkills(commandId);
      setCommandSkills(result.skills || []);
      setSelectedSkillIds((result.skills || []).map((s: Skill) => s.id));
      setSkillModalVisible(true);
    } catch (error) {
      message.error('加载关联技能失败');
    }
  };

  const handleSkillSelect = (skillIds: string[]) => {
    setSelectedSkillIds(skillIds);
  };

  const handleSaveSkills = async () => {
    if (!editingSkillCommandId) return;
    try {
      await api.commands.bindSkills(editingSkillCommandId, selectedSkillIds);
      message.success('技能绑定成功');
      setSkillModalVisible(false);
      loadCommands();
    } catch (error: any) {
      message.error(error.response?.data?.error || '技能绑定失败');
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
          <CodeOutlined style={{ color: 'var(--ant-color-primary)', fontSize: 16 }} />
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
      title: '关联技能',
      dataIndex: 'id',
      key: 'skillCount',
      width: 120,
      render: (id: string) => {
        const count = commandSkillCounts[id] || 0;
        return (
          <Tag color={count > 0 ? 'blue' : 'default'}>
            {count} 个技能
          </Tag>
        );
      },
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
      width: 220,
      render: (_: any, record: Command) => (
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
          <Tooltip title="绑定技能">
            <Button
              type="text"
              size="small"
              onClick={() => handleManageSkills(record.id)}
            >
              技能
            </Button>
          </Tooltip>
          <Popconfirm
            title="确定要删除这个命令吗？"
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
            <CodeOutlined style={{ marginRight: 8, color: 'var(--ant-color-primary)' }} />
            命令集管理
          </Title>
          <Text type="secondary">管理可复用的命令配置</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新建命令
        </Button>
      </div>

      {/* 搜索区域 */}
      <Card style={{ marginBottom: 16 }} styles={{ body: { padding: '12px 16px' } }}>
        <Input.Search
          placeholder="搜索命令名称或描述..."
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
            dataSource={commands}
            columns={columns}
            rowKey="id"
            pagination={false}
            locale={{
              emptyText: (
                <Empty
                  description="暂无命令"
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
        title={editingCommand ? '编辑命令' : '新建命令'}
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
        >
          {/* 创建方式选择 - 仅新建时显示 */}
          {!editingCommand && !isAfterUpload && (
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
                    action="/api/v1/commands/upload"
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
            extra="只允许小写字母、数字和中划线，如：code-review"
          >
            <Input
              placeholder="如：code-review"
              disabled={!!editingCommand || isAfterUpload}
            />
          </Form.Item>

          <Form.Item
            name="description"
            label="描述"
          >
            <Input.TextArea
              rows={3}
              placeholder="简要描述这个命令的用途"
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 查看详情弹窗 */}
      <Modal
        title={
          <Space>
            <CodeOutlined style={{ color: 'var(--ant-color-primary)' }} />
            <span>{viewingCommand?.name}</span>
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
              if (viewingCommand) {
                handleEdit(viewingCommand);
              }
            }}
          >
            编辑
          </Button>,
        ]}
        width={600}
      >
        {viewingCommand && (
          <div>
            <Paragraph>
              <Text strong>名称：</Text>
              <br />
              <Text>{viewingCommand.name}</Text>
            </Paragraph>

            <Paragraph>
              <Text strong>描述：</Text>
              <br />
              <Text type="secondary">{viewingCommand.description || '暂无描述'}</Text>
            </Paragraph>

            <div style={{ marginTop: 16, color: 'var(--ant-color-text-secondary)', fontSize: 12 }}>
              <Text type="secondary">
                创建时间：{viewingCommand.createdAt ? new Date(viewingCommand.createdAt).toLocaleString('zh-CN') : '-'}
              </Text>
              <br />
              <Text type="secondary">
                更新时间：{viewingCommand.updatedAt ? new Date(viewingCommand.updatedAt).toLocaleString('zh-CN') : '-'}
              </Text>
            </div>
          </div>
        )}
      </Modal>

      {/* 技能绑定弹窗 */}
      <Modal
        title="绑定技能"
        open={skillModalVisible}
        onOk={handleSaveSkills}
        onCancel={() => setSkillModalVisible(false)}
        width={600}
        okText="保存"
      >
        <div style={{ marginBottom: 16 }}>
          <Text type="secondary">选择要绑定到此命令的技能：</Text>
        </div>
        <Select
          mode="multiple"
          style={{ width: '100%' }}
          placeholder="选择技能"
          value={selectedSkillIds}
          onChange={handleSkillSelect}
          options={allSkills.map((skill) => ({
            label: skill.name,
            value: skill.id,
          }))}
        />
        {commandSkills.length > 0 && (
          <div style={{ marginTop: 16 }}>
            <Text strong>已绑定技能：</Text>
            <div style={{ marginTop: 8 }}>
              {commandSkills.map((skill) => (
                <Tag key={skill.id} color="blue" style={{ marginBottom: 4 }}>
                  {skill.name}
                </Tag>
              ))}
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default CommandList;