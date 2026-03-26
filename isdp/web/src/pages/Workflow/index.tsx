import React, { useState, useEffect, useCallback } from 'react';
import {
  Typography,
  Button,
  Modal,
  Form,
  Input,
  message,
  Tag,
  Popconfirm,
  Spin,
  Empty,
  Popover,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  TeamOutlined,
  UserOutlined,
  CrownOutlined,
  EditOutlined,
} from '@ant-design/icons';
import { api } from '@/api/client';
import type { AgentConfig, Transition } from '@/types';
import { AgentRoleLabels } from '@/types';
import AgentTriggerModal from './AgentTriggerModal';
import './Workflow.css';

const { Text } = Typography;

// Agent 触发配置
interface AgentTrigger {
  toAgentId: string;
  triggerHint: string;
}

// 团队视图中的 Agent
interface TeamAgent {
  config: AgentConfig;
  triggers: AgentTrigger[];
}

// 团队视图
interface TeamView {
  id: string;
  name: string;
  description?: string;
  agents: TeamAgent[];
  isDefault: boolean;
  isSystem: boolean;
}

// Transition 转换为 AgentTrigger 视图
function transitionsToTeamView(
  agentIds: string[],
  transitions: Transition[],
  agents: AgentConfig[]
): TeamAgent[] {
  const agentMap = new Map(agents.map(a => [a.id, a]));

  return agentIds.map(agentId => {
    const config = agentMap.get(agentId);
    if (!config) {
      return {
        config: { id: agentId, name: '未知 Agent' } as AgentConfig,
        triggers: [],
      };
    }

    // 找出以此 Agent 为源的所有触发
    const triggers: AgentTrigger[] = transitions
      .filter(t => t.fromAgentId === agentId)
      .map(t => ({
        toAgentId: t.toAgentId,
        triggerHint: t.triggerHint || '',
      }));

    return { config, triggers };
  });
}

// AgentTrigger 视图转换为 Transition 数组
function teamViewToTransitions(teamAgents: TeamAgent[]): Transition[] {
  const transitions: Transition[] = [];

  teamAgents.forEach(agent => {
    agent.triggers.forEach(trigger => {
      transitions.push({
        fromAgentId: agent.config.id,
        toAgentId: trigger.toAgentId,
        type: 'sequence', // 默认顺序执行
        triggerHint: trigger.triggerHint,
      });
    });
  });

  return transitions;
}

const WorkflowPage: React.FC = () => {
  const [teams, setTeams] = useState<TeamView[]>([]);
  const [agents, setAgents] = useState<AgentConfig[]>([]);
  const [loading, setLoading] = useState(false);
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [createForm] = Form.useForm();
  const [submitting, setSubmitting] = useState(false);

  // Agent 触发配置弹窗
  const [triggerModalVisible, setTriggerModalVisible] = useState(false);
  const [currentTeamId, setCurrentTeamId] = useState<string>('');
  const [currentAgent, setCurrentAgent] = useState<TeamAgent | null>(null);
  const [currentAgentIndex, setCurrentAgentIndex] = useState<number>(-1);

  // 加载数据
  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    setLoading(true);
    try {
      const [templatesData, agentsData] = await Promise.all([
        api.workflows.list(),
        api.agents.list(),
      ]);

      setAgents(agentsData || []);

      // 转换为 TeamView
      const teamViews = (templatesData || []).map(t => ({
        id: t.id,
        name: t.name,
        description: t.description || '',
        agents: transitionsToTeamView(t.agentIds || [], t.transitions || [], agentsData || []),
        isDefault: t.isDefault,
        isSystem: t.isSystem,
      }));

      setTeams(teamViews);
    } catch (error) {
      console.error('Failed to load data:', error);
      message.error('加载数据失败');
    } finally {
      setLoading(false);
    }
  };

  // 创建团队
  const handleCreateTeam = async (values: { name: string; description?: string }) => {
    setSubmitting(true);
    try {
      const newTemplate = await api.workflows.create({
        name: values.name,
        description: values.description || '',
        agentIds: [],
        transitions: [],
        checkpoints: [],
        estimatedTime: '自定义',
      });

      const newTeam: TeamView = {
        id: newTemplate.id,
        name: newTemplate.name,
        description: newTemplate.description || '',
        agents: [],
        isDefault: false,
        isSystem: false,
      };

      setTeams([...teams, newTeam]);
      setCreateModalVisible(false);
      createForm.resetFields();
      message.success('团队创建成功');
    } catch (error: any) {
      message.error(error?.response?.data?.error || '创建失败');
    } finally {
      setSubmitting(false);
    }
  };

  // 删除团队
  const handleDeleteTeam = async (teamId: string) => {
    try {
      await api.workflows.delete(teamId);
      setTeams(teams.filter(t => t.id !== teamId));
      message.success('删除成功');
    } catch (error: any) {
      message.error(error?.response?.data?.error || '删除失败');
    }
  };

  // 更新团队名称/描述
  const handleUpdateTeamName = async (teamId: string, name: string, description?: string) => {
    try {
      const team = teams.find(t => t.id === teamId);
      if (!team) return;

      await api.workflows.update(teamId, { name, description });

      setTeams(teams.map(t =>
        t.id === teamId ? { ...t, name, description } : t
      ));
      message.success('更新成功');
    } catch (error: any) {
      message.error(error?.response?.data?.error || '更新失败');
    }
  };

  // 添加 Agent 到团队
  const handleAddAgent = async (teamId: string, agentId: string) => {
    const team = teams.find(t => t.id === teamId);
    const agent = agents.find(a => a.id === agentId);
    if (!team || !agent) return;

    // 检查是否已存在
    if (team.agents.some(a => a.config.id === agentId)) {
      message.info('该 Agent 已在团队中');
      return;
    }

    const newAgent: TeamAgent = {
      config: agent,
      triggers: [],
    };

    const updatedAgents = [...team.agents, newAgent];
    const agentIds = updatedAgents.map(a => a.config.id);
    const transitions = teamViewToTransitions(updatedAgents);

    try {
      await api.workflows.update(teamId, {
        agentIds,
        transitions,
      });

      setTeams(teams.map(t =>
        t.id === teamId ? { ...t, agents: updatedAgents } : t
      ));
      message.success('添加成功');
    } catch (error: any) {
      message.error(error?.response?.data?.error || '添加失败');
    }
  };

  // 从团队移除 Agent
  const handleRemoveAgent = async (teamId: string, agentIndex: number) => {
    const team = teams.find(t => t.id === teamId);
    if (!team) return;

    const updatedAgents = team.agents.filter((_, i) => i !== agentIndex);
    const agentIds = updatedAgents.map(a => a.config.id);

    // 过滤掉涉及被删除 Agent 的 transitions
    const removedAgentId = team.agents[agentIndex].config.id;
    const transitions = teamViewToTransitions(updatedAgents).filter(
      t => t.fromAgentId !== removedAgentId && t.toAgentId !== removedAgentId
    );

    try {
      await api.workflows.update(teamId, {
        agentIds,
        transitions,
      });

      setTeams(teams.map(t =>
        t.id === teamId ? { ...t, agents: updatedAgents } : t
      ));
      message.success('移除成功');
    } catch (error: any) {
      message.error(error?.response?.data?.error || '移除失败');
    }
  };

  // 打开 Agent 触发配置弹窗
  const handleOpenTriggerModal = (teamId: string, agent: TeamAgent, index: number) => {
    setCurrentTeamId(teamId);
    setCurrentAgent(agent);
    setCurrentAgentIndex(index);
    setTriggerModalVisible(true);
  };

  // 保存 Agent 触发配置
  const handleSaveTriggers = async (triggers: AgentTrigger[]) => {
    if (!currentAgent) return;

    const team = teams.find(t => t.id === currentTeamId);
    if (!team) return;

    const updatedAgents = team.agents.map((a, i) =>
      i === currentAgentIndex ? { ...a, triggers } : a
    );

    const agentIds = updatedAgents.map(a => a.config.id);
    const transitions = teamViewToTransitions(updatedAgents);

    try {
      await api.workflows.update(currentTeamId, {
        agentIds,
        transitions,
      });

      setTeams(teams.map(t =>
        t.id === currentTeamId ? { ...t, agents: updatedAgents } : t
      ));
      message.success('配置保存成功');
      setTriggerModalVisible(false);
    } catch (error: any) {
      message.error(error?.response?.data?.error || '保存失败');
      throw error;
    }
  };

  // 获取可添加的 Agent 列表
  const getAvailableAgents = useCallback((teamId: string) => {
    const team = teams.find(t => t.id === teamId);
    if (!team) return agents;

    const existingIds = new Set(team.agents.map(a => a.config.id));
    return agents.filter(a => !existingIds.has(a.id));
  }, [teams, agents]);

  // 渲染团队卡片
  const renderTeamCard = (team: TeamView) => {
    const availableAgents = getAvailableAgents(team.id);

    return (
      <div key={team.id} className="workflow-team-card">
        {/* 团队头部 */}
        <div className="workflow-team-header">
          <div className="workflow-team-title-wrapper">
            <TeamNameEditor
              name={team.name}
              description={team.description}
              onSave={(name, desc) => handleUpdateTeamName(team.id, name, desc)}
              disabled={team.isSystem}
            />
            {team.isDefault && <Tag color="gold" style={{ marginLeft: 8 }}>默认</Tag>}
            {team.isSystem && <Tag color="purple" style={{ marginLeft: 4 }}>系统</Tag>}
          </div>
          {!team.isSystem && (
            <Popconfirm
              title="确定删除此团队？"
              onConfirm={() => handleDeleteTeam(team.id)}
              okText="确定"
              cancelText="取消"
            >
              <Button type="text" danger icon={<DeleteOutlined />} size="small" />
            </Popconfirm>
          )}
        </div>

        {/* Agent 列表 */}
        <div className="workflow-team-agents">
          {team.agents.map((agent, index) => (
            <div
              key={agent.config.id}
              className="workflow-agent-avatar-wrapper"
            >
              <div
                className="workflow-agent-avatar"
                onClick={() => handleOpenTriggerModal(team.id, agent, index)}
              >
                {agent.config.isSystem ? (
                  <CrownOutlined className="workflow-agent-icon system" />
                ) : (
                  <UserOutlined className="workflow-agent-icon" />
                )}
              </div>
              <div className="workflow-agent-name">{agent.config.name}</div>
              <Popconfirm
                title="确定移除此 Agent？"
                onConfirm={() => handleRemoveAgent(team.id, index)}
                okText="确定"
                cancelText="取消"
              >
                <Button
                  type="text"
                  danger
                  size="small"
                  icon={<DeleteOutlined />}
                  className="workflow-agent-remove"
                />
              </Popconfirm>
            </div>
          ))}

          {/* 添加 Agent 按钮 */}
          {availableAgents.length > 0 && (
            <Popover
              trigger="click"
              placement="bottom"
              content={
                <div className="workflow-add-agent-popover">
                  <div className="workflow-add-agent-title">选择 Agent</div>
                  <div className="workflow-add-agent-list">
                    {availableAgents.map(agent => (
                      <div
                        key={agent.id}
                        className="workflow-add-agent-item"
                        onClick={() => handleAddAgent(team.id, agent.id)}
                      >
                        {agent.isSystem ? (
                          <CrownOutlined style={{ color: '#faad14', marginRight: 8 }} />
                        ) : (
                          <UserOutlined style={{ marginRight: 8 }} />
                        )}
                        <span>{agent.name}</span>
                        <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>
                          {AgentRoleLabels[agent.role] || agent.role}
                        </Text>
                      </div>
                    ))}
                  </div>
                </div>
              }
            >
              <div className="workflow-agent-add">
                <PlusOutlined />
              </div>
            </Popover>
          )}
        </div>
      </div>
    );
  };

  return (
    <div className="workflow-page-wrapper">
      {/* 页面头部 */}
      <div className="workflow-page-header">
        <div>
          <h2 className="workflow-page-title">Agent 团队</h2>
          <p className="workflow-page-subtitle">配置多 Agent 协作团队，定义触发规则</p>
        </div>
        <div className="workflow-header-actions">
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateModalVisible(true)}
          >
            新建团队
          </Button>
        </div>
      </div>

      {/* 团队列表 */}
      <div className="workflow-teams-container">
        {loading ? (
          <div className="workflow-loading-container">
            <Spin />
          </div>
        ) : teams.length === 0 ? (
          <Empty
            description="暂无团队"
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            style={{ padding: 48 }}
          />
        ) : (
          teams.map(renderTeamCard)
        )}
      </div>

      {/* 新建团队弹窗 */}
      <Modal
        title="新建团队"
        open={createModalVisible}
        onOk={() => createForm.submit()}
        onCancel={() => {
          setCreateModalVisible(false);
          createForm.resetFields();
        }}
        confirmLoading={submitting}
        width={500}
      >
        <Form form={createForm} layout="vertical" onFinish={handleCreateTeam}>
          <Form.Item
            name="name"
            label="团队名称"
            rules={[{ required: true, message: '请输入团队名称' }]}
          >
            <Input placeholder="例如：全栈开发团队" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="描述团队的用途" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Agent 触发配置弹窗 */}
      <AgentTriggerModal
        visible={triggerModalVisible}
        agent={currentAgent}
        allAgents={agents}
        onSave={handleSaveTriggers}
        onClose={() => {
          setTriggerModalVisible(false);
          setCurrentAgent(null);
        }}
      />
    </div>
  );
};

// 团队名称行内编辑组件
const TeamNameEditor: React.FC<{
  name: string;
  description?: string;
  onSave: (name: string, description?: string) => void;
  disabled?: boolean;
}> = ({ name, description, onSave, disabled }) => {
  const [editing, setEditing] = useState(false);
  const [editName, setEditName] = useState(name);
  const [editDesc, setEditDesc] = useState(description || '');

  const handleSave = () => {
    if (editName.trim()) {
      onSave(editName.trim(), editDesc.trim() || undefined);
      setEditing(false);
    }
  };

  if (editing) {
    return (
      <div className="workflow-team-name-editor">
        <Input
          value={editName}
          onChange={e => setEditName(e.target.value)}
          placeholder="团队名称"
          style={{ width: 160, marginRight: 8 }}
          autoFocus
        />
        <Input
          value={editDesc}
          onChange={e => setEditDesc(e.target.value)}
          placeholder="描述（可选）"
          style={{ width: 200, marginRight: 8 }}
        />
        <Button type="primary" size="small" onClick={handleSave}>保存</Button>
        <Button size="small" onClick={() => setEditing(false)} style={{ marginLeft: 4 }}>取消</Button>
      </div>
    );
  }

  return (
    <div
      className={`workflow-team-name ${disabled ? '' : 'editable'}`}
      onClick={() => !disabled && setEditing(true)}
    >
      <TeamOutlined style={{ marginRight: 8 }} />
      <span>{name}</span>
      {!disabled && <EditOutlined style={{ marginLeft: 8, fontSize: 12, opacity: 0.5 }} />}
    </div>
  );
};

export default WorkflowPage;