import React, { useEffect, useState } from 'react';
import { Table, Button, Card, Modal, Form, Input, Select, message, Space, Tag, Typography, Tooltip } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, RobotOutlined, BugOutlined, CopyOutlined, ExclamationCircleOutlined, EyeOutlined, SettingOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '@/api/client';
import type { AgentConfig, BaseAgent, Skill, Subagent, Command, Rule } from '@/types';

const { Title, Text } = Typography;

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
  const [publicRules, setPublicRules] = useState<Rule[]>([]);
  const [instanceRules, setInstanceRules] = useState<Rule[]>([]);
  const [selectedRuleIds, setSelectedRuleIds] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingConfig, setEditingConfig] = useState<AgentConfig | null>(null);
  const [deleteLoading, setDeleteLoading] = useState<string | null>(null);
  const [generateLoading, setGenerateLoading] = useState<string | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    loadConfigs();
    loadBaseAgents();
    loadSkills();
    loadSubagents();
    loadCommands();
    loadRules();
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
      const [publicResult, instanceResult] = await Promise.all([
        api.rules.getPublicRules(),
        api.rules.getInstanceRules(),
      ]);
      setPublicRules(publicResult || []);
      setInstanceRules(instanceResult || []);
    } catch (error) {
      console.error('加载规约列表失败', error);
      setPublicRules([]);
      setInstanceRules([]);
    }
  };

  const loadAgentRules = async (agentId: string) => {
    try {
      const result = await api.rules.getAgentRules(agentId);
      setSelectedRuleIds(result.agents?.map(a => a.id) || []);
    } catch (error) {
      console.error('加载Agent绑定的规约失败', error);
      setSelectedRuleIds([]);
    }
  };

  const handleCreate = () => {
    setEditingConfig(null);
    form.resetFields();
    setSelectedSkillIds([]);
    setSelectedSubagentIds([]);
    setSelectedCommandIds([]);
    // 公共规约默认选中
    setSelectedRuleIds(publicRules.map(r => r.id));
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
                  <p>该Agent角色被以下工作流引用，无法删除：</p>
                  <ul style={{ marginTop: 8, paddingLeft: 20 }}>
                    {errorData.referenceNames.map((name: string) => (
                      <li key={name}>{name}</li>
                    ))}
                  </ul>
                  <p style={{ marginTop: 8 }}>请先从工作流中移除该Agent，再进行删除。</p>
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
        await api.agents.update(editingConfig.id, values);
        // 更新技能绑定
        await api.agents.bindSkills(editingConfig.id, selectedSkillIds);
        // 更新子代理绑定
        await api.agents.bindSubagents(editingConfig.id, selectedSubagentIds);
        // 更新命令绑定
        if (selectedCommandIds.length > 0) {
          await api.commands.bindCommandsToAgent(editingConfig.id, selectedCommandIds);
        }
        // 更新规约绑定
        if (selectedRuleIds.length > 0) {
          await api.rules.bindRulesToAgent(editingConfig.id, selectedRuleIds);
        }
        message.success('更新成功');
      } else {
        const newAgent = await api.agents.create(values);
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
          title: '工作流引用',
          content: (
            <div>
              <p>该Agent角色被以下 <strong>{result.referenceCount}</strong> 个工作流引用：</p>
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
          title: '工作流引用',
          content: <p>该Agent角色暂未被任何工作流引用</p>,
        });
      }
    } catch (error) {
      message.error('查询引用失败');
    }
  };

  const handleGenerateConfig = (record: AgentConfig) => {
    Modal.confirm({
      title: '生成配置',
      content: (
        <div>
          <p>确定要为Agent「{record.name}」生成配置吗？</p>
          <p>这将创建独立的配置目录，包含绑定的技能和子代理。</p>
        </div>
      ),
      onOk: async () => {
        setGenerateLoading(record.id);
        try {
          const result = await api.agents.generateConfig(record.id, 'claude_code');
          message.success(`配置生成成功，包含 ${result.skills_count} 个技能和 ${result.subagents_count} 个子代理`);
          loadConfigs();
        } catch (error) {
          message.error('配置生成失败');
        } finally {
          setGenerateLoading(null);
        }
      },
    });
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 150,
      render: (name: string) => (
        <Space>
          <RobotOutlined />
          <span>{name}</span>
        </Space>
      ),
    },
    {
      title: '基础Agent',
      dataIndex: 'baseAgentId',
      key: 'baseAgentId',
      width: 120,
      render: (baseAgentId: string) => {
        const agent = baseAgents.find(a => a.id === baseAgentId);
        return agent ? (
          <Tag color={agent.type === 'claude_code' ? 'blue' : 'green'}>
            {agent.name}
          </Tag>
        ) : <Tag>默认</Tag>;
      },
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      width: 200,
      ellipsis: true,
      render: (desc?: string) => (
        <Tooltip title={desc} placement="topLeft">
          {desc || '-'}
        </Tooltip>
      ),
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
      width: 280,
      fixed: 'right' as const,
      render: (_: unknown, record: AgentConfig) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<SettingOutlined />}
            onClick={() => handleGenerateConfig(record)}
            loading={generateLoading === record.id}
          >
            生成配置
          </Button>
          <Button type="link" size="small" icon={<BugOutlined />} onClick={() => handleDebug(record)}>
            调试
          </Button>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => handleViewRefs(record)}>
            引用
          </Button>
          <Button type="link" size="small" icon={<CopyOutlined />} onClick={() => handleCopy(record)}>
            复制
          </Button>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            编辑
          </Button>
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
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 12 }}>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>Agent 角色</Title>
          <Text type="secondary">管理不同职责的 Agent 角色配置</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新建角色
        </Button>
      </div>

      <Card>
        <Table
          dataSource={configs}
          columns={columns}
          rowKey="id"
          loading={loading}
          scroll={{ x: 'max-content' }}
        />
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
            <Input placeholder="Agent角色名称" />
          </Form.Item>

          <Form.Item name="baseAgentId" label="基础Agent">
            <Select placeholder="选择基础Agent" allowClear>
              {baseAgents.map(agent => (
                <Select.Option key={agent.id} value={agent.id}>
                  {agent.name} ({agent.type === 'claude_code' ? 'Claude Code' : 'OpenCode'})
                </Select.Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="Agent角色描述" />
          </Form.Item>

          <Form.Item name="systemPrompt" label="系统提示词" rules={[{ required: true, message: '请输入系统提示词' }]}>
            <Input.TextArea rows={8} placeholder="系统提示词，定义Agent的行为和能力" />
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

          <Form.Item label="绑定子代理">
            <Select
              mode="multiple"
              placeholder="选择要绑定的子代理"
              value={selectedSubagentIds}
              onChange={setSelectedSubagentIds}
              style={{ width: '100%' }}
              optionLabelProp="label"
              options={subagents.map(s => ({
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

          <Form.Item label="绑定命令">
            <Select
              mode="multiple"
              placeholder="选择要绑定的命令"
              value={selectedCommandIds}
              onChange={setSelectedCommandIds}
              style={{ width: '100%' }}
              optionLabelProp="label"
              options={commands.map(c => ({
                label: c.name,
                value: c.id,
                desc: c.description || '暂无描述',
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

          <Form.Item label="绑定规约">
            <div style={{ marginBottom: 8 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                公共规约（默认选中，可取消）:
              </Text>
              <Select
                mode="multiple"
                placeholder="选择公共规约"
                value={selectedRuleIds.filter(id => publicRules.some(r => r.id === id))}
                onChange={(ids) => {
                  // 合并公共规约选择和实例规约选择
                  const instanceSelected = selectedRuleIds.filter(id => instanceRules.some(r => r.id === id));
                  setSelectedRuleIds([...ids, ...instanceSelected]);
                }}
                style={{ width: '100%', marginTop: 4 }}
                optionLabelProp="label"
                options={publicRules.map(r => ({
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
            <div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                实例规约（按需绑定）:
              </Text>
              <Select
                mode="multiple"
                placeholder="选择实例规约"
                value={selectedRuleIds.filter(id => instanceRules.some(r => r.id === id))}
                onChange={(ids) => {
                  // 合并公共规约选择和实例规约选择
                  const publicSelected = selectedRuleIds.filter(id => publicRules.some(r => r.id === id));
                  setSelectedRuleIds([...publicSelected, ...ids]);
                }}
                style={{ width: '100%', marginTop: 4 }}
                optionLabelProp="label"
                options={instanceRules.map(r => ({
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
        </Form>
      </Modal>
    </div>
  );
};

export default AgentRoleList;