import React, { useEffect, useState, useMemo } from 'react';
import { Table, Button, Card, Modal, Form, Input, Select, message, Space, Tag, Typography, Tooltip, Alert, Spin, Collapse, Switch, Dropdown } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, RobotOutlined, CopyOutlined, CrownOutlined, ExclamationCircleOutlined, EyeOutlined, SettingOutlined, BookOutlined, ApiOutlined, CodeOutlined, SafetyCertificateOutlined, MoreOutlined } from '@ant-design/icons';
import api from '@/api/client';
import { getTypeColorByIndex } from '@/config/agentTypeColors';
import AgentTypeIcon from '@/components/AgentTypeIcon';
import type { AgentConfig, BaseAgent, Skill, Subagent, Command, Rule, Settings, MCPServer, BatchGenerateResult, BatchUpdateResult, GenerateResultItem, WorkflowTemplate, BaseAgentTypeInfo } from '@/types';

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
  baseAgentType?: string;
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
  const [configs, setConfigs] = useState<AgentConfig[]>([]);
  const [baseAgents, setBaseAgents] = useState<BaseAgent[]>([]);
  const [agentTypes, setAgentTypes] = useState<BaseAgentTypeInfo[]>([]);
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
  const [mcpServers, setMCPServers] = useState<MCPServer[]>([]);
  const [selectedMCPServerIds, setSelectedMCPServerIds] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingConfig, setEditingConfig] = useState<AgentConfig | null>(null);
  const [submitLoading, setSubmitLoading] = useState(false);
  const [submitLoadingText, setSubmitLoadingText] = useState('');
  const [deleteLoading, setDeleteLoading] = useState<string | null>(null);
  const [generateLoading, setGenerateLoading] = useState<string | null>(null);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [previewData, setPreviewData] = useState<ConfigPreview | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [form] = Form.useForm();
  // 分页状态
  const [systemPageSize, setSystemPageSize] = useState(10);
  const [customPageSize, setCustomPageSize] = useState(10);
  // 批量删除相关状态
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [batchDeleteLoading, setBatchDeleteLoading] = useState(false);
  // 批量生成配置相关状态
  const [batchGenerateLoading, setBatchGenerateLoading] = useState(false);
  const [batchResultVisible, setBatchResultVisible] = useState(false);
  const [batchResultData, setBatchResultData] = useState<BatchGenerateResult | null>(null);
  // 批量修改基础Agent相关状态
  const [batchUpdateVisible, setBatchUpdateVisible] = useState(false);
  const [batchUpdateLoading, setBatchUpdateLoading] = useState(false);
  const [batchUpdateResultVisible, setBatchUpdateResultVisible] = useState(false);
  const [batchUpdateResultData, setBatchUpdateResultData] = useState<BatchUpdateResult | null>(null);
  const [targetBaseAgentId, setTargetBaseAgentId] = useState<string>('');
  // 团队筛选相关状态
  const [workflows, setWorkflows] = useState<WorkflowTemplate[]>([]);
  const [selectedWorkflowId, setSelectedWorkflowId] = useState<string>('');

  useEffect(() => {
    loadConfigs();
    loadBaseAgents();
    loadSkills();
    loadSubagents();
    loadCommands();
    loadRules();
    loadSettings();
    loadMCPServers();
    loadWorkflows();
    api.baseAgents.getTypes().then(setAgentTypes).catch(() => {});
  }, []);

  const loadConfigs = async () => {
    setLoading(true);
    try {
      const data = await api.agents.list();
      setConfigs(data);
    } catch (error) {
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

  const loadWorkflows = async () => {
    try {
      const data = await api.workflows.list();
      setWorkflows(data);
    } catch (error) {
      console.error('加载团队列表失败', error);
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

  const loadMCPServers = async () => {
    try {
      const result = await api.mcpServers.list({ pageSize: 100, status: 'active' });
      setMCPServers(result.data || []);
    } catch (error) {
      console.error('加载 MCP Server 列表失败', error);
      setMCPServers([]);
    }
  };

  const loadAgentMCPServers = async (agentId: string) => {
    try {
      const result = await api.mcpServers.getAgentMCPServers(agentId);
      setSelectedMCPServerIds(result.data?.map(s => s.id) || []);
    } catch (error) {
      console.error('加载Agent绑定的 MCP Server 失败', error);
      setSelectedMCPServerIds([]);
    }
  };

  const handleCreate = () => {
    setEditingConfig(null);
    form.resetFields();
    setSelectedSkillIds([]);
    setSelectedSubagentIds([]);
    setSelectedCommandIds([]);
    setSelectedSettingsIds([]);
    setSelectedRuleIds([]);
    setSelectedMCPServerIds([]);
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
      loadAgentMCPServers(record.id),
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

  // 批量删除
  const handleBatchDelete = () => {
    if (selectedRowKeys.length === 0) return;

    // 获取选中项数据
    const selectedItems = customAgents.filter(a => selectedRowKeys.includes(a.id));

    Modal.confirm({
      title: '批量删除确认',
      icon: <ExclamationCircleOutlined />,
      width: 520,
      content: (
        <div>
          <p style={{ marginBottom: 12 }}>确定要删除以下 {selectedRowKeys.length} 个 Agent 角色吗？此操作不可恢复。</p>
          <Table
            dataSource={selectedItems}
            columns={[
              {
                title: '名称',
                dataIndex: 'name',
                key: 'name',
              },
              {
                title: '基础Agent',
                dataIndex: 'baseAgentId',
                key: 'baseAgentId',
                width: 140,
                ellipsis: true,
                render: (baseAgentId: string) => {
                  const agent = baseAgents.find(a => a.id === baseAgentId);
                  return agent ? `${agent.name} (${agent.defaultModel})` : '默认';
                },
              },
            ]}
            rowKey="id"
            size="small"
            pagination={{
              pageSize: 10,
              showSizeChanger: false,
              showTotal: (total) => `共 ${total} 条`,
            }}
            scroll={{ y: 280 }}
          />
        </div>
      ),
      okText: '确认删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        setBatchDeleteLoading(true);
        try {
          await api.agents.batchDelete(selectedRowKeys as string[]);
          message.success(`成功删除 ${selectedRowKeys.length} 个 Agent 角色`);
          setSelectedRowKeys([]);
          loadConfigs();
        } catch (error: any) {
          const errorData = error.response?.data;
          if (errorData?.referencedAgents) {
            Modal.error({
              title: '无法删除',
              width: 520,
              content: (
                <div>
                  <p>以下 Agent 角色被团队引用，无法删除：</p>
                  <Table
                    dataSource={errorData.referencedAgents}
                    columns={[
                      { title: '名称', dataIndex: 'name', key: 'name', ellipsis: true },
                      {
                        title: '引用团队',
                        dataIndex: 'workflowNames',
                        key: 'workflowNames',
                        ellipsis: true,
                        render: (names: string[]) => names.join('、'),
                      },
                    ]}
                    rowKey="id"
                    size="small"
                    pagination={{
                      pageSize: 10,
                      showSizeChanger: false,
                      showTotal: (total) => `共 ${total} 条`,
                    }}
                    scroll={{ y: 280 }}
                  />
                  <p style={{ marginTop: 8 }}>请先从团队中移除这些 Agent，再进行删除。</p>
                </div>
              ),
            });
          } else if (errorData?.hasSystemAgent) {
            Modal.error({
              title: '无法删除',
              content: <p>系统预置角色「{errorData.systemAgentName}」不可删除</p>,
            });
          } else {
            message.error('批量删除失败');
          }
        } finally {
          setBatchDeleteLoading(false);
        }
      },
    });
  };

  // 批量生成配置
  const handleBatchGenerateConfig = () => {
    if (selectedRowKeys.length === 0) return;

    // 获取选中项数据
    const selectedItems = customAgents.filter(a => selectedRowKeys.includes(a.id));

    Modal.confirm({
      title: '批量生成配置',
      icon: <ExclamationCircleOutlined />,
      width: 520,
      content: (
        <div>
          <p style={{ marginBottom: 12 }}>确定为选中的 {selectedRowKeys.length} 个 Agent 生成配置？</p>
          <Alert
            type="warning"
            message="已有配置将被覆盖"
            showIcon
            style={{ marginBottom: 12 }}
          />
          <Table
            dataSource={selectedItems}
            columns={[
              {
                title: '名称',
                dataIndex: 'name',
                key: 'name',
                ellipsis: true,
              },
              {
                title: '配置状态',
                key: 'configStatus',
                width: 80,
                render: (_: unknown, record: AgentConfig) => (
                  record.configGeneratedAt ? (
                    <Tag color="green">已生成</Tag>
                  ) : (
                    <Tag color="default">未生成</Tag>
                  )
                ),
              },
            ]}
            rowKey="id"
            size="small"
            pagination={{
              pageSize: 10,
              showSizeChanger: false,
              showTotal: (total) => `共 ${total} 条`,
            }}
            scroll={{ y: 280 }}
          />
        </div>
      ),
      okText: '确认生成',
      okType: 'primary',
      cancelText: '取消',
      onOk: async () => {
        setBatchGenerateLoading(true);
        try {
          const result = await api.agents.batchGenerateConfig(selectedRowKeys as string[]);
          setBatchResultData(result);
          setBatchResultVisible(true);
          setSelectedRowKeys([]);
          loadConfigs();
        } catch (error: any) {
          const errorData = error.response?.data;
          if (errorData?.error) {
            Modal.error({
              title: '批量生成失败',
              content: errorData.error,
            });
          } else {
            message.error('批量生成失败');
          }
        } finally {
          setBatchGenerateLoading(false);
        }
      },
    });
  };

  // 批量修改基础Agent
  const handleBatchUpdateBaseAgent = async () => {
    if (!targetBaseAgentId) {
      message.warning('请选择目标基础Agent');
      return;
    }

    setBatchUpdateLoading(true);
    try {
      const result = await api.agents.batchUpdateBaseAgent(
        selectedRowKeys as string[],
        targetBaseAgentId
      );
      setBatchUpdateResultData(result);
      setBatchUpdateVisible(false);
      setBatchUpdateResultVisible(true);
      setSelectedRowKeys([]);
      setTargetBaseAgentId('');
      loadConfigs();
    } catch (error: any) {
      const errorData = error.response?.data;
      if (errorData?.error) {
        Modal.error({
          title: '批量修改失败',
          content: errorData.error,
        });
      } else {
        message.error('批量修改失败');
      }
    } finally {
      setBatchUpdateLoading(false);
    }
  };

  const handleSubmit = async (values: Partial<AgentConfig>) => {
    setSubmitLoading(true);
    setSubmitLoadingText(editingConfig ? '正在更新角色配置...' : '正在创建角色配置...');
    const startTime = Date.now();
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
        // 更新 MCP Server 绑定 - 无论是否为空都调用，以支持清空绑定
        await api.mcpServers.bindToAgent(editingConfig.id, selectedMCPServerIds);
        // 刷新配置（自动检测类型，只调用一次）
        await api.agents.refreshConfig(editingConfig.id);
        // 确保 loading 效果至少显示 1500ms
        const elapsed = Date.now() - startTime;
        const remainingTime = Math.max(0, 1500 - elapsed);
        await new Promise(resolve => setTimeout(resolve, remainingTime));
        message.success('更新成功，配置已自动生成');
        setSubmitLoading(false);
        setSubmitLoadingText('');
        setModalVisible(false);
        loadConfigs();
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
        // 为新创建的Agent绑定 MCP Server
        if (selectedMCPServerIds.length > 0) {
          await api.mcpServers.bindToAgent(newAgent.id, selectedMCPServerIds);
        }
        // 刷新配置（自动检测类型，只调用一次）
        await api.agents.refreshConfig(newAgent.id);
        // 确保 loading 效果至少显示 1500ms
        const elapsed = Date.now() - startTime;
        const remainingTime = Math.max(0, 1500 - elapsed);
        await new Promise(resolve => setTimeout(resolve, remainingTime));
        message.success('创建成功，配置已自动生成');
        setSubmitLoading(false);
        setSubmitLoadingText('');
        setModalVisible(false);
        loadConfigs();
      }
    } catch (error) {
      setSubmitLoading(false);
      setSubmitLoadingText('');
      message.error('操作失败');
    }
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
      // 获取 agent 关联的 baseAgentType
      const agentConfig = configs.find(c => c.id === previewData.agentId);
      const baseAgentType = previewData.baseAgentType || agentConfig?.baseAgent?.type || 'claude_code';
      const result = await api.agents.generateConfig(previewData.agentId, baseAgentType);
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

  // 构建 Agent → 团队 映射
  const agentTeamsMap = useMemo(() => {
    const map = new Map<string, string[]>();
    workflows.forEach(w => {
      (w.agentIds || []).forEach(agentId => {
        if (!map.has(agentId)) map.set(agentId, []);
        map.get(agentId)!.push(w.name);
      });
    });
    return map;
  }, [workflows]);

  // 分组显示：系统预置和自定义，支持团队筛选
  const { systemAgents, customAgents } = useMemo(() => {
    // 先获取选中团队中的 agentIds
    let filteredConfigs = configs;
    if (selectedWorkflowId) {
      const selectedWorkflow = workflows.find(w => w.id === selectedWorkflowId);
      if (selectedWorkflow && selectedWorkflow.agentIds) {
        // 只保留该团队中的角色
        filteredConfigs = configs.filter(c => selectedWorkflow.agentIds.includes(c.id));
      }
    }

    const system = filteredConfigs.filter(c => c.isSystem);
    const custom = filteredConfigs.filter(c => !c.isSystem);
    return { systemAgents: system, customAgents: custom };
  }, [configs, selectedWorkflowId, workflows]);

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 150,
      render: (name: string, record: AgentConfig) => (
        <Space>
          <AgentTypeIcon
            requiresHuman={record.requiresHuman}
            isSystem={record.isSystem}
            size={16}
          />
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
        if (agent) {
          const typeInfo = agentTypes.find(t => t.type === agent.type);
          const color = getTypeColorByIndex(agentTypes, agent.type);
          return (
            <Tag color={color}>
              {agent.name}
            </Tag>
          );
        }
        return <Tag>默认</Tag>;
      },
    },
    {
      title: '团队',
      dataIndex: 'id',
      key: 'team',
      width: 180,
      render: (id: string) => {
        const teams = agentTeamsMap.get(id);
        if (!teams || teams.length === 0) return <Text type="secondary">-</Text>;
        return (
          <Space size={4} wrap>
            {teams.map(name => (
              <Tag key={name} color="blue">{name}</Tag>
            ))}
          </Space>
        );
      },
    },
    {
      title: '人工参与',
      dataIndex: 'requiresHuman',
      key: 'requiresHuman',
      width: 100,
      render: (requiresHuman: boolean) => (
        <span style={{ display: 'inline-block', width: 85 }}>
          <Tag
            color={requiresHuman ? 'green' : 'default'}
            style={{ width: '100%', textAlign: 'center' }}
          >
            {requiresHuman ? 'Human In' : 'Human Out'}
          </Tag>
        </span>
      ),
    },
    {
      title: '配置',
      key: 'configStatus',
      width: 80,
      render: (_: unknown, record: AgentConfig) => (
        record.configGeneratedAt ? (
          <Tooltip title={`路径: ${record.configPath}\n生成时间: ${new Date(record.configGeneratedAt).toLocaleString('zh-CN')}`}>
            <Tag color="green" style={{ margin: 0 }}>已生成</Tag>
          </Tooltip>
        ) : (
          <Tag color="default" style={{ margin: 0 }}>未生成</Tag>
        )
      ),
    },
    {
      title: '系统提示词',
      dataIndex: 'systemPrompt',
      key: 'systemPrompt',
      width: 280,
      render: (prompt?: string) => {
        if (!prompt) return '-';
        const truncated = truncateText(prompt, 60);
        if (truncated === prompt) return <Text style={{ maxWidth: 280 }}>{prompt}</Text>;
        return (
          <Tooltip title={<div style={{ maxWidth: 400, whiteSpace: 'pre-wrap' }}>{prompt}</div>} placement="topLeft">
            <Text style={{ maxWidth: 280, cursor: 'pointer' }}>{truncated}</Text>
          </Tooltip>
        );
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 200,
      fixed: 'right' as const,
      render: (_: unknown, record: AgentConfig) => {
        const moreItems = [
          {
            key: 'generate',
            label: '生成配置',
            icon: <SettingOutlined />,
            onClick: () => handlePreviewConfig(record),
          },
          {
            key: 'refs',
            label: '引用',
            icon: <EyeOutlined />,
            onClick: () => handleViewRefs(record),
          },
          {
            key: 'copy',
            label: '复制',
            icon: <CopyOutlined />,
            onClick: () => handleCopy(record),
          },
        ];

        return (
          <Space size="small">
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
            <Dropdown menu={{ items: moreItems }} trigger={['click']}>
              <Button type="link" size="small" icon={<MoreOutlined />}>
                更多
              </Button>
            </Dropdown>
          </Space>
        );
      },
    },
  ];

  return (
    <div style={{ padding: 12 }}>
      {/* 全屏 loading overlay */}
      {submitLoading && (
        <div style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          backgroundColor: 'rgba(0, 0, 0, 0.6)',
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          zIndex: 9999,
        }}>
          <div style={{
            backgroundColor: 'var(--bg-container, #fff)',
            padding: 32,
            borderRadius: 12,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: 16,
            boxShadow: '0 4px 20px rgba(0, 0, 0, 0.3)',
          }}>
            <Spin size="large" />
            <span style={{ fontSize: 16, color: 'var(--text-primary, #333)' }}>{submitLoadingText}</span>
          </div>
        </div>
      )}
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>Agent角色</Title>
          <Text type="secondary">管理不同职责的 Agent 角色配置</Text>
        </div>
        <Space>
          <Text type="secondary">筛选团队：</Text>
          <Select
            placeholder="选择团队"
            allowClear
            style={{ width: 200 }}
            value={selectedWorkflowId}
            onChange={setSelectedWorkflowId}
            options={workflows.map(w => ({
              label: w.name,
              value: w.id,
            }))}
          />
        </Space>
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
              pageSizeOptions: ['10', '20', '50'],
              showTotal: (total) => `共 ${total} 条`,
              hideOnSinglePage: false,
              onShowSizeChange: (_, size) => setSystemPageSize(size),
            }}
            size="small"
            scroll={{ x: 1110 }}
          />
        </Card>
      )}

      {/* 自定义角色 */}
      <Card
        title={
          <Space>
            <RobotOutlined />
            <span>自定义角色</span>
            <Tag color="blue">{customAgents.length}</Tag>
          </Space>
        }
        extra={
          <Space>
            {selectedRowKeys.length > 0 && (
              <>
                <Button
                  icon={<EditOutlined />}
                  aria-label={`批量修改基础Agent，已选择${selectedRowKeys.length}项`}
                  onClick={() => setBatchUpdateVisible(true)}
                >
                  批量修改基础Agent ({selectedRowKeys.length})
                </Button>
                <Button
                  icon={<SettingOutlined />}
                  aria-label={`批量生成配置，已选择${selectedRowKeys.length}项`}
                  onClick={handleBatchGenerateConfig}
                  loading={batchGenerateLoading}
                >
                  批量生成配置 ({selectedRowKeys.length})
                </Button>
                <Button
                  danger
                  icon={<DeleteOutlined />}
                  aria-label={`批量删除，已选择${selectedRowKeys.length}项`}
                  loading={batchDeleteLoading}
                  onClick={handleBatchDelete}
                >
                  批量删除 ({selectedRowKeys.length})
                </Button>
              </>
            )}
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
              新建角色
            </Button>
          </Space>
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
            rowSelection={{
              selectedRowKeys,
              onChange: setSelectedRowKeys,
              getCheckboxProps: (record: AgentConfig) => ({
                disabled: record.isSystem,
              }),
            }}
            pagination={{
              pageSize: customPageSize,
              showSizeChanger: true,
              pageSizeOptions: ['10', '20', '50'],
              showTotal: (total) => `共 ${total} 条`,
              hideOnSinglePage: false,
              onShowSizeChange: (_, size) => setCustomPageSize(size),
            }}
            size="small"
            scroll={{ x: 1110 }}
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

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="角色描述" />
          </Form.Item>

          <Form.Item name="baseAgentId" label="基础Agent">
            <Select placeholder="选择基础Agent" allowClear>
              {baseAgents.map(agent => {
                const typeInfo = agentTypes.find(t => t.type === agent.type);
                return (
                  <Select.Option key={agent.id} value={agent.id}>
                    {agent.name} ({typeInfo?.name || agent.type} - {agent.defaultModel})
                  </Select.Option>
                );
              })}
            </Select>
          </Form.Item>

          <Form.Item
            name="requiresHuman"
            label="需要人工参与"
            valuePropName="checked"
            extra="开启后，该角色在执行过程中会在关键节点等待人工确认或输入"
          >
            <Switch checkedChildren="是" unCheckedChildren="否" />
          </Form.Item>

          <Form.Item name="systemPrompt" label="系统提示词" rules={[{ required: true, message: '请输入系统提示词' }]}>
            <Input.TextArea rows={8} placeholder="系统提示词，定义Agent的行为和能力" />
          </Form.Item>

          <Form.Item name="mentionPatterns" label="触发模式" extra="设置 @mention 触发模式，用户可通过这些模式唤起该角色。新建时默认为 @+名称">
            <Select
              mode="tags"
              placeholder="输入触发模式，如 @developer、@开发者"
              style={{ width: '100%' }}
              tokenSeparators={[',', ' ']}
            />
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
                label: `${s.name} (${s.description || '暂无描述'})`,
                value: s.id,
                desc: s.description || '暂无描述',
              }))}
              optionRender={(option) => (
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontWeight: 500 }}>{option.data?.label?.split(' (')[0]}</span>
                  <span style={{ fontSize: 12, color: 'var(--text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 300 }}>
                    ({option.data?.desc})
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

          <Form.Item label="绑定 MCP Servers">
            <Select
              mode="multiple"
              placeholder="选择要绑定的 MCP Server"
              value={selectedMCPServerIds}
              onChange={setSelectedMCPServerIds}
              style={{ width: '100%' }}
              optionLabelProp="label"
              showSearch
              filterOption={(input, option) =>
                (option?.label as string)?.toLowerCase().includes(input.toLowerCase()) ||
                (option?.desc as string)?.toLowerCase().includes(input.toLowerCase())
              }
              options={mcpServers.map(server => ({
                label: server.displayName || server.name,
                value: server.id,
                desc: `${server.transport} · ${server.description || '暂无描述'}`,
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

      {/* 批量修改基础Agent选择弹窗 */}
      <Modal
        title="批量修改基础Agent"
        open={batchUpdateVisible}
        width={520}
        styles={{ body: { maxHeight: '70vh' } }}
        onCancel={() => {
          setBatchUpdateVisible(false);
          setTargetBaseAgentId('');
        }}
        onOk={handleBatchUpdateBaseAgent}
        okText="确认修改"
        cancelText="取消"
        confirmLoading={batchUpdateLoading}
      >
        <div style={{ marginBottom: 16 }}>
          <p style={{ marginBottom: 8 }}>将为选中的 {selectedRowKeys.length} 个 Agent 角色修改基础Agent：</p>
          <Table
            dataSource={customAgents.filter(a => selectedRowKeys.includes(a.id))}
            columns={[
              {
                title: '名称',
                dataIndex: 'name',
                key: 'name',
              },
              {
                title: '当前基础Agent',
                dataIndex: 'baseAgentId',
                key: 'baseAgentId',
                width: 120,
                render: (baseAgentId: string) => {
                  const agent = baseAgents.find(a => a.id === baseAgentId);
                  if (agent) {
                    const typeInfo = agentTypes.find(t => t.type === agent.type);
                    const color = getTypeColorByIndex(agentTypes, agent.type);
                    return (
                      <Tag color={color}>
                        {agent.name}
                      </Tag>
                    );
                  }
                  return <Tag>默认</Tag>;
                },
              },
            ]}
            rowKey="id"
            size="small"
            pagination={{
              pageSize: 10,
              showSizeChanger: false,
              showTotal: (total) => `共 ${total} 条`,
            }}
            scroll={{ y: 220 }}
          />
        </div>
        <div style={{ marginBottom: 8 }}>
          <label style={{ display: 'block', marginBottom: 4 }}>目标基础Agent</label>
          <Select
            placeholder="选择基础Agent"
            value={targetBaseAgentId}
            onChange={setTargetBaseAgentId}
            style={{ width: '100%' }}
          >
            {baseAgents.map(agent => {
              const typeInfo = agentTypes.find(t => t.type === agent.type);
              return (
                <Select.Option key={agent.id} value={agent.id}>
                  {agent.name} ({typeInfo?.name || agent.type})
                </Select.Option>
              );
            })}
          </Select>
        </div>
      </Modal>

      {/* 批量生成配置结果弹窗 */}
      <Modal
        title="批量生成配置结果"
        open={batchResultVisible}
        width={600}
        onCancel={() => {
          setBatchResultVisible(false);
          setBatchResultData(null);
        }}
        footer={[
          <Button key="close" onClick={() => {
            setBatchResultVisible(false);
            setBatchResultData(null);
          }}>
            关闭
          </Button>,
        ]}
      >
        {batchResultData && (
          <div>
            <Alert
              type={batchResultData.failed > 0 ? 'warning' : 'success'}
              message={`成功 ${batchResultData.success} 个，失败 ${batchResultData.failed} 个`}
              style={{ marginBottom: 16 }}
              showIcon
            />
            {(batchResultData.results?.length ?? 0) > 0 && (
              <Table
                dataSource={batchResultData.results}
                columns={[
                  {
                    title: 'Agent名称',
                    dataIndex: 'agentName',
                    key: 'agentName',
                    ellipsis: true,
                  },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    key: 'status',
                    width: 70,
                    render: (status: string) => (
                      <Tag color={status === 'success' ? 'green' : 'red'}>
                        {status === 'success' ? '成功' : '失败'}
                      </Tag>
                    ),
                  },
                  {
                    title: '生成数量',
                    key: 'counts',
                    ellipsis: true,
                    render: (_: unknown, record: GenerateResultItem) => (
                      <span style={{ fontSize: 12 }}>
                        S:{record.skillsCount} C:{record.commandsCount}
                      </span>
                    ),
                  },
                  {
                    title: '错误',
                    dataIndex: 'error',
                    key: 'error',
                    ellipsis: true,
                    render: (error?: string) => error ? <Text type="danger" style={{ fontSize: 12 }}>{error}</Text> : '-',
                  },
                ]}
                rowKey="agentId"
                size="small"
                pagination={{
                  pageSize: 10,
                  showSizeChanger: false,
                  showTotal: (total) => `共 ${total} 条`,
                }}
                scroll={{ y: 300 }}
              />
            )}
          </div>
        )}
      </Modal>

      {/* 批量修改基础Agent结果弹窗 */}
      <Modal
        title="批量修改基础Agent结果"
        open={batchUpdateResultVisible}
        width={520}
        onCancel={() => {
          setBatchUpdateResultVisible(false);
          setBatchUpdateResultData(null);
        }}
        footer={[
          <Button key="close" onClick={() => {
            setBatchUpdateResultVisible(false);
            setBatchUpdateResultData(null);
          }}>
            关闭
          </Button>,
        ]}
      >
        {batchUpdateResultData && (
          <div>
            <Alert
              type={batchUpdateResultData.failed > 0 ? 'warning' : 'success'}
              message={`成功 ${batchUpdateResultData.success} 个，失败 ${batchUpdateResultData.failed} 个`}
              style={{ marginBottom: 16 }}
              showIcon
            />
            {(batchUpdateResultData.results?.length ?? 0) > 0 && (
              <Table
                dataSource={batchUpdateResultData.results}
                columns={[
                  {
                    title: 'Agent名称',
                    dataIndex: 'agentName',
                    key: 'agentName',
                    ellipsis: true,
                  },
                  {
                    title: '基础Agent',
                    dataIndex: 'baseAgentName',
                    key: 'baseAgentName',
                    width: 100,
                    ellipsis: true,
                  },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    key: 'status',
                    width: 70,
                    render: (status: string) => (
                      <Tag color={status === 'success' ? 'green' : 'red'}>
                        {status === 'success' ? '成功' : '失败'}
                      </Tag>
                    ),
                  },
                  {
                    title: '错误',
                    dataIndex: 'error',
                    key: 'error',
                    ellipsis: true,
                    render: (error?: string) => error ? <Text type="danger">{error}</Text> : '-',
                  },
                ]}
                rowKey="agentId"
                size="small"
                pagination={{
                  pageSize: 10,
                  showSizeChanger: false,
                  showTotal: (total) => `共 ${total} 条`,
                }}
                scroll={{ y: 300 }}
              />
            )}
          </div>
        )}
      </Modal>
    </div>
  );
};

export default AgentRoleList;
