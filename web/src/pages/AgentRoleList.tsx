import React, { useEffect, useState, useMemo } from 'react';
import { Table, Button, Card, Modal, Form, Input, Select, message, Space, Tag, Typography, Tooltip, Alert, Spin, Collapse } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, RobotOutlined, BugOutlined, CopyOutlined, CrownOutlined, UserOutlined, ExclamationCircleOutlined, EyeOutlined, SettingOutlined, BookOutlined, ApiOutlined, CodeOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '@/api/client';
import type { AgentConfig, BaseAgent, Skill, Subagent, Command, Rule, Settings } from '@/types';

const { Title, Text } = Typography;

// 预览结果类型
interface PreviewItem {
  id: string;
  name: string;
  description: string;
}

interface ConfigPreview {
  agentId: string;
  agentName: string;
  skills: PreviewItem[];
  commands: PreviewItem[];
  subagents: PreviewItem[];
  rules: PreviewItem[];
  settings: PreviewItem[];
  skillsCount: number;
  commandsCount: number;
  subagentsCount: number;
  rulesCount: number;
  settingsCount: number;
}

// 截断文本并添加省略号
const truncateText = (text: string, maxLength: number = 50): string => {
  if (!text) return '-';
  if (text.length <= maxLength) return text;
  return text.substring(0, maxLength) + '...';
};

const AgentRoleList: React.FC = () => {
  const navigate = useNavigate();
  const [configs, setConfigs] = useState<AgentConfig[]>([]);
  const [baseAgents, setBaseAgents] = useState<BaseAgent[]>([]);
  const [skills, setSkills] = useState<Skill[]>([]);
  const [selectedSkillIds, setSelectedSkillIds] = useState<string[]>([]);
  const [subagents, setSubagents] = useState<Subagent[]>([]);
  const [selectedSubagentIds, setSelectedSubagentIds] = useState<string[]>([]);
  const [commands, setCommands] = useState<Command[]>([]);
  const [selectedCommandIds, setSelectedCommandIds] = useState<string[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [selectedRuleIds, setSelectedRuleIds] = useState<string[]>([]);
  const [settings, setSettings] = useState<Settings[]>([]);
  const [selectedSettingsIds, setSelectedSettingsIds] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingConfig, setEditingConfig] = useState<AgentConfig | null>(null);
  const [deleteLoading, setDeleteLoading] = useState<string | null>(null);
  const [generateLoading, setGenerateLoading] = useState<string | null>(null);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [previewData, setPreviewData] = useState<ConfigPreview | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [form] = Form.useForm();
  // 分页状态
  const [systemPageSize, setSystemPageSize] = useState(5);
  const [customPageSize, setCustomPageSize] = useState(5);

  useEffect(() => {
    loadConfigs();
    loadBaseAgents();
    loadSkills();
    loadSubagents();
    loadCommands();
    loadRules();
    loadSettings();
  }, []);

  const loadConfigs = async () => {
    setLoading(true);
    try {
      const data = await api.agents.list();
      console.log('[DEBUG] Agent configs loaded:', data);
      console.log('[DEBUG] First config sample:', data[0]);
      setConfigs(data);
    } catch (error) {
      console.error('[DEBUG] Error loading configs:', error);
      message.error('加载Agent角色失败');
    } finally {
      setLoading(false);
    }
  };

  const loadBaseAgents = async () => {
    try {
      const data = await api.baseAgents.list();
      setBaseAgents(data);
    } catch (error) {
      console.error('加载基础Agent失败', error);
    }
  };

  const loadSkills = async () => {
    try {
      const result = await api.skills.list({ pageSize: 100 });
      setSkills(result.data || []);
    } catch (error) {
      console.error('加载技能列表失败', error);
      setSkills([]);
    }
  };

  const loadAgentSkills = async (agentId: string) => {
    try {
      const result = await api.agents.getSkills(agentId);
      setSelectedSkillIds(result.skills?.map(s => s.id) || []);
    } catch (error) {
      console.error('加载Agent绑定的技能失败', error);
      setSelectedSkillIds([]);
    }
  };

  const loadSubagents = async () => {
    try {
      const result = await api.subagents.list({ pageSize: 100 });
      setSubagents(result.data || []);
    } catch (error) {
      console.error('加载子代理列表失败', error);
      setSubagents([]);
    }
  };

  const loadAgentSubagents = async (agentId: string) => {
    try {
      const result = await api.agents.getSubagents(agentId);
      setSelectedSubagentIds(result.subagents?.map(s => s.id) || []);
    } catch (error) {
      console.error('加载Agent绑定的子代理失败', error);
      setSelectedSubagentIds([]);
    }
  };

  const loadCommands = async () => {
    try {
      const result = await api.commands.list({ pageSize: 100 });
      setCommands(result.data || []);
    } catch (error) {
      console.error('加载命令列表失败', error);
      setCommands([]);
    }
  };

  const loadAgentCommands = async (agentId: string) => {
    try {
      const result = await api.commands.getAgentCommands(agentId);
      setSelectedCommandIds(result.commands?.map(c => c.id) || []);
    } catch (error) {
      console.error('加载Agent绑定的命令失败', error);
      setSelectedCommandIds([]);
    }
  };

  const loadRules = async () => {
    try {
      const result = await api.rules.list({ pageSize: 100 });
      setRules(result.data || []);
    } catch (error) {
      console.error('加载规约列表失败', error);
      setRules([]);
    }
  };

  const loadAgentRules = async (agentId: string) => {
    try {
      const result = await api.rules.getAgentRules(agentId);
      setSelectedRuleIds(result.rules?.map(r => r.id) || []);
    } catch (error) {
      console.error('加载Agent绑定的规约失败', error);
      setSelectedRuleIds([]);
    }
  };

  const loadSettings = async () => {
    try {
      const result = await api.settings.list({ pageSize: 100 });
      setSettings(result.data || []);
    } catch (error) {
      console.error('加载配置列表失败', error);
      setSettings([]);
    }
  };

  const loadAgentSettings = async (agentId: string) => {
    try {
      const result = await api.settings.getAgentSettings(agentId);
      setSelectedSettingsIds(result.settings?.map(s => s.id) || []);
    } catch (error) {
      console.error('加载Agent绑定的配置失败', error);
      setSelectedSettingsIds([]);
    }
  };

  const handleCreate = () => {
    setEditingConfig(null);
    form.resetFields();
    // 默认角色类型为 agent
    form.setFieldsValue({ role: 'agent' });
    setSelectedSkillIds([]);
    setSelectedSubagentIds([]);
    setSelectedCommandIds([]);
    setSelectedSettingsIds([]);
    // 默认选中所有规约
    setSelectedRuleIds(rules.map(r => r.id));
    setModalVisible(true);
  };

  const handleEdit = async (record: AgentConfig) => {
    setEditingConfig(record);
    form.setFieldsValue(record);
    await Promise.all([
      loadAgentSkills(record.id),
      loadAgentSubagents(record.id),
      loadAgentCommands(record.id),
      loadAgentRules(record.id),
      loadAgentSettings(record.id),
    ]);
    setModalVisible(true);
  };

  const handleDelete = (record: AgentConfig) => {
    Modal.confirm({
      title: '确认删除',
      icon: <ExclamationCircleOutlined />,
      content: `确定要删除Agent角色「${record.name}」吗？此操作不可恢复。`,
      okText: '确认删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        setDeleteLoading(record.id);
        try {
          await api.agents.delete(record.id);
          message.success('删除成功');
          loadConfigs();
        } catch (error: any) {
          const errorData = error.response?.data;
          if (errorData?.referenced && errorData?.referenceNames) {
            Modal.error({
              title: '无法删除',
              content: (
                <div>
                  <p>该Agent角色被以下团队引用，无法删除：</p>
                  <ul style={{ marginTop: 8, paddingLeft: 20 }}>
                    {errorData.referenceNames.map((name: string) => (
                      <li key={name}>{name}</li>
                    ))}
                  </ul>
                  <p style={{ marginTop: 8 }}>请先从团队中移除该Agent，再进行删除。</p>
                </div>
              ),
            });
          } else {
            message.error('删除失败');
          }
        } finally {
          setDeleteLoading(null);
        }
      },
    });
  };

  const handleSubmit = async (values: Partial<AgentConfig>) => {
    try {
      if (editingConfig) {
        // 更新时，只有当 mentionPatterns 有值时才传递
        const updateData = { ...values };
        await api.agents.update(editingConfig.id, updateData);
        // 更新技能绑定
        await api.agents.bindSkills(editingConfig.id, selectedSkillIds);
        // 更新子代理绑定
        await api.agents.bindSubagents(editingConfig.id, selectedSubagentIds);
        // 更新命令绑定 - 无论是否为空都调用，以支持清空绑定
        await api.commands.bindCommandsToAgent(editingConfig.id, selectedCommandIds);
        // 更新规约绑定 - 无论是否为空都调用，以支持清空绑定
        await api.rules.bindRulesToAgent(editingConfig.id, selectedRuleIds);
        // 更新配置绑定 - 无论是否为空都调用，以支持清空绑定
        await api.settings.bindToAgent(editingConfig.id, selectedSettingsIds);
        message.success('更新成功');
      } else {
        // 新建时，如果没有设置触发模式，则默认生成 @ + 名称
        const createData = { ...values };
        if (!createData.mentionPatterns || createData.mentionPatterns.length === 0) {
          if (createData.name) {
            // 生成默认触发模式：@ + 名称（去除空格）
            const defaultPattern = '@' + createData.name.replace(/\s+/g, '');
            createData.mentionPatterns = [defaultPattern];
          }
        }
        const newAgent = await api.agents.create(createData);
        // 为新创建的Agent绑定技能
        if (selectedSkillIds.length > 0) {
          await api.agents.bindSkills(newAgent.id, selectedSkillIds);
        }
        // 为新创建的Agent绑定子代理
        if (selectedSubagentIds.length > 0) {
          await api.agents.bindSubagents(newAgent.id, selectedSubagentIds);
        }
        // 为新创建的Agent绑定命令
        if (selectedCommandIds.length > 0) {
          await api.commands.bindCommandsToAgent(newAgent.id, selectedCommandIds);
        }
        // 为新创建的Agent绑定规约
        if (selectedRuleIds.length > 0) {
          await api.rules.bindRulesToAgent(newAgent.id, selectedRuleIds);
        }
        // 为新创建的Agent绑定配置
        if (selectedSettingsIds.length > 0) {
          await api.settings.bindToAgent(newAgent.id, selectedSettingsIds);
        }
        message.success('创建成功');
      }
      setModalVisible(false);
      loadConfigs();
    } catch (error) {
      message.error('操作失败');
    }
  };

  const handleDebug = (record: AgentConfig) => {
    navigate(`/agents/${record.id}/debug`);
  };

  const handleCopy = async (record: AgentConfig) => {
    try {
      await api.agents.copy(record.id);
      message.success('复制成功');
      loadConfigs();
    } catch (error) {
      message.error('复制失败');
    }
  };

  const handleViewRefs = async (record: AgentConfig) => {
    try {
      const result = await api.agents.checkReferences(record.id);
      if (result.referenced) {
        Modal.info({
          title: '团队引用',
          content: (
            <div>
              <p>该Agent角色被以下 <strong>{result.referenceCount}</strong> 个团队引用：</p>
              <ul style={{ marginTop: 8, paddingLeft: 20 }}>
                {result.referenceNames.map((name: string) => (
                  <li key={name}>{name}</li>
                ))}
              </ul>
            </div>
          ),
        });
      } else {
        Modal.info({
          title: '团队引用',
          content: <p>该Agent角色暂未被任何团队引用</p>,
        });
      }
    } catch (error) {
      message.error('查询引用失败');
    }
  };

// 预览配置
  const handlePreviewConfig = async (record: AgentConfig) => {
    setPreviewLoading(true);
    setPreviewVisible(true);
    try {
      const result = await api.agents.previewConfig(record.id);
      setPreviewData(result);
    } catch (error) {
      message.error('获取配置预览失败');
      setPreviewVisible(false);
    } finally {
      setPreviewLoading(false);
    }
  };

  // 确认生成配置
  const handleConfirmGenerate = async () => {
    if (!previewData) return;
    setGenerateLoading(previewData.agentId);
    try {
      const result = await api.agents.generateConfig(previewData.agentId, 'claude_code');
      message.success(`配置生成成功，包含 ${result.commandsCount} 个 Commands、${result.subagentsCount} 个 Subagents、${result.skillsCount} 个 Skills、${result.rulesCount} 个 Rules、${result.settingsCount} 个 Settings`);
      setPreviewVisible(false);
      setPreviewData(null);
      loadConfigs();
    } catch (error) {
      message.error('配置生成失败');
    } finally {
      setGenerateLoading(null);
    }
  };

  // 分组显示：系统预置和自定义
  const { systemAgents, customAgents } = useMemo(() => {
    const system = configs.filter(c => c.isSystem);
    const custom = configs.filter(c => !c.isSystem);
    return { systemAgents: system, customAgents: custom };
  }, [configs]);

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 150,
      render: (name: string, record: AgentConfig) => (
        <Space>
          {record.isSystem ? <CrownOutlined style={{ color: '#faad14' }} /> : (record.role === 'human' ? <UserOutlined /> : <RobotOutlined />)}
          <span>{name}</span>
        </Space>
      ),
    },
    {
      title: '角色类型',
      dataIndex: 'role',
      key: 'role',
      width: 100,
      render: (role: string) => (
        <Tag color={role === 'human' ? 'purple' : 'blue'}>
          {role === 'human' ? '人角色' : 'Agent'}
        </Tag>
      ),
    },
    {
      title: '基础Agent',
      dataIndex: 'baseAgentId',
      key: 'baseAgentId',
      width: 120,
      render: (baseAgentId: string, record: AgentConfig) => {
        if (record.role === 'human') return <Tag color="default">无</Tag>;
        const agent = baseAgents.find(a => a.id === baseAgentId);
        return agent ? (
          <Tag color={agent.type === 'claude_code' ? 'blue' : 'green'}>
            {agent.name}
          </Tag>
        ) : <Tag>默认</Tag>;
      },
    },
    {
      title: '配置状态',
      key: 'configStatus',
      width: 150,
      render: (_: unknown, record: AgentConfig) => (
        record.configGeneratedAt ? (
          <Tooltip title={`路径: ${record.configPath}`}>
            <Tag color="green">
              已生成 ({new Date(record.configGeneratedAt).toLocaleDateString()})
            </Tag>
          </Tooltip>
        ) : (
          <Tag color="default">未生成</Tag>
        )
      ),
    },
    {
      title: '系统提示词',
      dataIndex: 'systemPrompt',
      key: 'systemPrompt',
      width: 350,
      render: (prompt?: string) => {
        if (!prompt) return '-';
        const truncated = truncateText(prompt, 80);
        if (truncated === prompt) return <Text style={{ maxWidth: 350 }}>{prompt}</Text>;
        return (
          <Tooltip title={<div style={{ maxWidth: 400, whiteSpace: 'pre-wrap' }}>{prompt}</div>} placement="topLeft">
            <Text style={{ maxWidth: 350, cursor: 'pointer' }}>{truncated}</Text>
          </Tooltip>
        );
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 360,
      fixed: 'right' as const,
      render: (_: unknown, record: AgentConfig) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<SettingOutlined />}
            onClick={() => handlePreviewConfig(record)}
          >
            生成配置
          </Button>
          <Tooltip title={!record.configPath ? '请先生成配置' : ''}>
            <Button
              type="link"
              size="small"
              icon={<BugOutlined />}
              disabled={!record.configPath}
              onClick={() => handleDebug(record)}
            >
              调试
            </Button>
          </Tooltip>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => handleViewRefs(record)}>
            引用
          </Button>
          <Button type="link" size="small" icon={<CopyOutlined />} onClick={() => handleCopy(record)}>
            复制
          </Button>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            编辑
          </Button>
          {!record.isSystem && (
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
              onClick={() => handleDelete(record)}
              loading={deleteLoading === record.id}
            >
              删除
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 12 }}>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>Agent角色</Title>
          <Text type="secondary">管理不同职责的 Agent 角色配置</Text>
        </div>
      </div>

      {/* 系统预置角色 */}
      {systemAgents.length > 0 && (
        <Card
          title={
            <Space>
              <CrownOutlined style={{ color: '#faad14' }} />
              <span>系统预置角色</span>
              <Tag color="gold">{systemAgents.length}</Tag>
            </Space>
          }
          style={{ marginBottom: 16 }}
          extra={
            <Text type="secondary" style={{ fontSize: 12 }}>
              系统预置角色不可删除，可复制后修改
            </Text>
          }
        >
          <Table
            dataSource={systemAgents}
            columns={columns}
            rowKey="id"
            loading={loading}
            pagination={{
              pageSize: systemPageSize,
              showSizeChanger: true,
              pageSizeOptions: ['5', '10', '20'],
              showTotal: (total) => `共 ${total} 条`,
              hideOnSinglePage: false,
              onShowSizeChange: (_, size) => setSystemPageSize(size),
            }}
            size="small"
            scroll={{ x: 1130 }}
          />
        </Card>
      )}

      {/* 自定义角色 */}
      <Card
        title={
          <Space>
            <UserOutlined />
            <span>自定义角色</span>
            <Tag color="blue">{customAgents.length}</Tag>
          </Space>
        }
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            新建角色
          </Button>
        }
      >
        {customAgents.length === 0 ? (
          <Alert
            message="暂无自定义角色"
            description="点击「新建角色」创建您自己的 Agent 角色，或复制系统预置角色进行修改"
            type="info"
            showIcon
          />
        ) : (
          <Table
            dataSource={customAgents}
            columns={columns}
            rowKey="id"
            loading={loading}
            pagination={{
              pageSize: customPageSize,
              showSizeChanger: true,
              pageSizeOptions: ['5', '10', '20'],
              showTotal: (total) => `共 ${total} 条`,
              hideOnSinglePage: false,
              onShowSizeChange: (_, size) => setCustomPageSize(size),
            }}
            size="small"
            scroll={{ x: 1130 }}
          />
        )}
      </Card>

      <Modal
        title={editingConfig ? '编辑Agent角色' : '新建Agent角色'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={800}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{}}
        >
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="角色名称" />
          </Form.Item>

          <Form.Item name="role" label="角色类型" rules={[{ required: true, message: '请选择角色类型' }]}>
            <Select placeholder="选择角色类型">
              <Select.Option value="agent">Agent（CLI 执行）</Select.Option>
              <Select.Option value="human">人角色（任务卡片）</Select.Option>
            </Select>
          </Form.Item>

          {/* 仅 Agent 角色显示以下字段 */}
          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.role !== cur.role}>
            {({ getFieldValue }) => {
              const role = getFieldValue('role');
              if (role !== 'agent') return null;
              return (
                <>
                  <Form.Item name="baseAgentId" label="基础Agent">
                    <Select placeholder="选择基础Agent" allowClear>
                      {baseAgents.map(agent => (
                        <Select.Option key={agent.id} value={agent.id}>
                          {agent.name} ({agent.type === 'claude_code' ? 'Claude Code' : 'OpenCode'})
                        </Select.Option>
                      ))}
                    </Select>
                  </Form.Item>

                  <Form.Item label="绑定 Skills">
                    <Select
                      mode="multiple"
                      placeholder="选择要绑定的 Skills"
                      value={selectedSkillIds}
                      onChange={setSelectedSkillIds}
                      style={{ width: '100%' }}
                      optionLabelProp="label"
                      showSearch
                      filterOption={(input, option) =>
                        (option?.label as string)?.toLowerCase().includes(input.toLowerCase()) ||
                        (option?.desc as string)?.toLowerCase().includes(input.toLowerCase())
                      }
                      options={skills.map(s => ({
                        label: s.name,
                        value: s.id,
                        desc: s.description || '暂无描述',
                      }))}
                      optionRender={(option) => (
                        <div style={{ display: 'flex', flexDirection: 'column' }}>
                          <span style={{ fontWeight: 500 }}>{option.label}</span>
                          <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 400 }}>
                            {option.data?.desc}
                          </span>
                        </div>
                      )}
                    />
                  </Form.Item>

                  <Form.Item label="绑定 Subagents">
                    <Select
                      mode="multiple"
                      placeholder="选择要绑定的 Subagents"
                      value={selectedSubagentIds}
                      onChange={setSelectedSubagentIds}
                      style={{ width: '100%' }}
                      optionLabelProp="label"
                      showSearch
                      filterOption={(input, option) =>
                        (option?.label as string)?.toLowerCase().includes(input.toLowerCase()) ||
                        (option?.desc as string)?.toLowerCase().includes(input.toLowerCase())
                      }
                      options={subagents.map(s => ({
                        label: s.name,
                        value: s.id,
                        desc: s.description || '暂无描述',
                      }))}
                      optionRender={(option) => (
                        <div style={{ display: 'flex', flexDirection: 'column' }}>
                          <span style={{ fontWeight: 500 }}>{option.label}</span>
                          <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 400 }}>
                            {option.data?.desc}
                          </span>
                        </div>
                      )}
                    />
                  </Form.Item>

                  <Form.Item label="绑定 Commands">
                    <Select
                      mode="multiple"
                      placeholder="选择要绑定的 Commands"
                      value={selectedCommandIds}
                      onChange={setSelectedCommandIds}
                      style={{ width: '100%' }}
                      optionLabelProp="label"
                      showSearch
                      filterOption={(input, option) =>
                        (option?.label as string)?.toLowerCase().includes(input.toLowerCase()) ||
                        (option?.desc as string)?.toLowerCase().includes(input.toLowerCase())
                      }
                      options={commands.map(c => ({
                        label: c.name,
                        value: c.id,
                        desc: c.description || '暂无描述',
                      }))}
                      optionRender={(option) => (
                        <div style={{ display: 'flex', flexDirection: 'column' }}>
                          <span style={{ fontWeight: 500 }}>{option.label}</span>
                          <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 400 }}>
                            {option.data?.desc}
                          </span>
                        </div>
                      )}
                    />
                  </Form.Item>

                  <Form.Item label="绑定 Rules">
                    <div style={{ marginBottom: 8 }}>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        Rules（默认全选，可取消）：
                      </Text>
                      <Select
                        mode="multiple"
                        placeholder="选择 Rules"
                        value={selectedRuleIds}
                        onChange={setSelectedRuleIds}
                        style={{ width: '100%', marginTop: 4 }}
                        optionLabelProp="label"
                        showSearch
                        filterOption={(input, option) =>
                          (option?.label as string)?.toLowerCase().includes(input.toLowerCase()) ||
                          (option?.desc as string)?.toLowerCase().includes(input.toLowerCase())
                        }
                        options={rules.map(r => ({
                          label: r.name,
                          value: r.id,
                          desc: r.description || '暂无描述',
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
                    </div>
                  </Form.Item>

                  <Form.Item label="绑定 Settings">
                    <Select
                      mode="multiple"
                      placeholder="选择要绑定的 Settings（配置目录）"
                      value={selectedSettingsIds}
                      onChange={setSelectedSettingsIds}
                      style={{ width: '100%' }}
                      optionLabelProp="label"
                      showSearch
                      filterOption={(input, option) =>
                        (option?.label as string)?.toLowerCase().includes(input.toLowerCase()) ||
                        (option?.desc as string)?.toLowerCase().includes(input.toLowerCase())
                      }
                      options={settings.map(s => ({
                        label: s.name,
                        value: s.id,
                        desc: s.description || '暂无描述',
                      }))}
                      optionRender={(option) => (
                        <div style={{ display: 'flex', flexDirection: 'column' }}>
                          <span style={{ fontWeight: 500 }}>{option.label}</span>
                          <span style={{ fontSize: 12, color: '#999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 400 }}>
                            {option.data?.desc}
                          </span>
                        </div>
                      )}
                    />
                  </Form.Item>
                </>
              );
            }}
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="角色描述" />
          </Form.Item>

          <Form.Item name="mentionPatterns" label="触发模式" extra="设置 @mention 触发模式，用户可通过这些模式唤起该角色。新建时默认为 @+名称">
            <Select
              mode="tags"
              placeholder="输入触发模式，如 @developer、@开发者"
              style={{ width: '100%' }}
              tokenSeparators={[',', ' ']}
            />
          </Form.Item>

          {/* 系统提示词根据角色类型显示不同提示 */}
          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.role !== cur.role}>
            {({ getFieldValue }) => {
              const role = getFieldValue('role');
              const placeholder = role === 'human'
                ? '职责和交付物描述。\n推荐格式：\n职责：[角色职责]\n交付物：[期望交付物格式]'
                : '系统提示词，定义Agent的行为和能力';
              return (
                <Form.Item name="systemPrompt" label={role === 'human' ? '职责与交付物' : '系统提示词'} rules={[{ required: true, message: '请输入内容' }]}>
                  <Input.TextArea rows={8} placeholder={placeholder} />
                </Form.Item>
              );
            }}
          </Form.Item>
        </Form>
      </Modal>

      {/* 配置预览弹窗 */}
      <Modal
        className="config-preview-modal"
        title={
          <Space>
            <SettingOutlined style={{ color: 'var(--color-primary)' }} />
            <span>配置预览 - {previewData?.agentName}</span>
          </Space>
        }
        open={previewVisible}
        onCancel={() => {
          setPreviewVisible(false);
          setPreviewData(null);
        }}
        footer={[
          <Button key="cancel" onClick={() => {
            setPreviewVisible(false);
            setPreviewData(null);
          }}>
            取消
          </Button>,
          <Button
            key="generate"
            type="primary"
            loading={generateLoading !== null}
            onClick={handleConfirmGenerate}
          >
            确认生成配置
          </Button>,
        ]}
        width={700}
        styles={{ body: { padding: '16px 24px' } }}
      >
        {previewLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin size="large" />
          </div>
        ) : previewData ? (
          <div>
            <Alert
              type="info"
              message="即将生成以下资产配置"
              description="配置将包含角色直接绑定的资产，以及 Command、Subagent 关联的 Skill"
              style={{ marginBottom: 16 }}
              showIcon
            />

            <Collapse
              defaultActiveKey={['commands', 'subagents', 'skills', 'rules', 'settings']}
              items={[
                {
                  key: 'commands',
                  label: (
                    <Space>
                      <CodeOutlined style={{ color: '#1890ff' }} />
                      <span>Commands</span>
                      <Tag color="blue">{previewData.commandsCount}</Tag>
                    </Space>
                  ),
                  children: previewData.commands.length > 0 ? (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 16px' }}>
                      {previewData.commands.map((item) => (
                        <Tag key={item.id} color="default">{item.name}</Tag>
                      ))}
                    </div>
                  ) : <Text type="secondary">暂无绑定的 Commands</Text>,
                },
                {
                  key: 'subagents',
                  label: (
                    <Space>
                      <ApiOutlined style={{ color: '#52c41a' }} />
                      <span>Subagents</span>
                      <Tag color="green">{previewData.subagentsCount}</Tag>
                    </Space>
                  ),
                  children: previewData.subagents.length > 0 ? (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 16px' }}>
                      {previewData.subagents.map((item) => (
                        <Tag key={item.id} color="default">{item.name}</Tag>
                      ))}
                    </div>
                  ) : <Text type="secondary">暂无绑定的 Subagents</Text>,
                },
                {
                  key: 'skills',
                  label: (
                    <Space>
                      <BookOutlined style={{ color: '#722ed1' }} />
                      <span>Skills（含 Command/Subagent 关联）</span>
                      <Tag color="purple">{previewData.skillsCount}</Tag>
                    </Space>
                  ),
                  children: previewData.skills.length > 0 ? (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 16px' }}>
                      {previewData.skills.map((item) => (
                        <Tag key={item.id} color="default">{item.name}</Tag>
                      ))}
                    </div>
                  ) : <Text type="secondary">暂无关联的 Skills</Text>,
                },
                {
                  key: 'rules',
                  label: (
                    <Space>
                      <SafetyCertificateOutlined style={{ color: '#fa8c16' }} />
                      <span>Rules</span>
                      <Tag color="orange">{previewData.rulesCount}</Tag>
                    </Space>
                  ),
                  children: previewData.rules.length > 0 ? (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 16px' }}>
                      {previewData.rules.map((item) => (
                        <Tag key={item.id} color="default">{item.name}</Tag>
                      ))}
                    </div>
                  ) : <Text type="secondary">暂无绑定的 Rules</Text>,
                },
                {
                  key: 'settings',
                  label: (
                    <Space>
                      <SettingOutlined style={{ color: '#13c2c2' }} />
                      <span>Settings</span>
                      <Tag color="cyan">{previewData.settingsCount}</Tag>
                    </Space>
                  ),
                  children: previewData.settings.length > 0 ? (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 16px' }}>
                      {previewData.settings.map((item) => (
                        <Tag key={item.id} color="default">{item.name}</Tag>
                      ))}
                    </div>
                  ) : <Text type="secondary">暂无绑定的 Settings</Text>,
                },
              ]}
            />
          </div>
        ) : null}
      </Modal>
    </div>
  );
};

export default AgentRoleList;