import React, { useState, useEffect, useCallback } from 'react';
import {
  Button,
  Modal,
  Form,
  Input,
  message,
  Spin,
  Empty,
} from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { api } from '@/api/client';
import type { AgentConfig, Transition } from '@/types';
import AgentTriggerModal from './AgentTriggerModal';
import TeamCard, { TeamAgent, TeamView, AgentTrigger, teamViewToTransitions } from './TeamCard';
import './Workflow.css';

// Transition 转换为 TeamAgent 视图
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

  // 更新 Agent 排序
  const handleUpdateAgentOrder = useCallback(async (teamId: string, agentIds: string[]) => {
    const team = teams.find(t => t.id === teamId);
    if (!team) return;

    // 根据新的 agentIds 顺序重建 agents 数组
    const agentMap = new Map(team.agents.map(a => [a.config.id, a]));
    const updatedAgents = agentIds.map(id => {
      const agent = agentMap.get(id);
      if (!agent) {
        // 如果 agentId 不在现有 agents 中，可能是新添加的
        const config = agents.find(a => a.id === id);
        if (config) {
          return { config, triggers: [] };
        }
        return { config: { id, name: '未知 Agent' } as AgentConfig, triggers: [] };
      }
      return agent;
    }).filter((a): a is TeamAgent => a !== null);

    const transitions = teamViewToTransitions(updatedAgents);

    try {
      await api.workflows.update(teamId, {
        agentIds,
        transitions,
      });

      setTeams(teams.map(t =>
        t.id === teamId ? { ...t, agents: updatedAgents } : t
      ));
      message.success('排序更新成功');
    } catch (error: any) {
      message.error(error?.response?.data?.error || '排序更新失败');
    }
  }, [teams, agents]);

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
          teams.map(team => (
            <TeamCard
              key={team.id}
              team={team}
              allAgents={agents}
              onUpdateName={(name, description) => handleUpdateTeamName(team.id, name, description)}
              onDelete={() => handleDeleteTeam(team.id)}
              onAddAgent={(agentId) => handleAddAgent(team.id, agentId)}
              onRemoveAgent={(index) => handleRemoveAgent(team.id, index)}
              onOpenTriggerModal={(agent, index) => handleOpenTriggerModal(team.id, agent, index)}
              onSaveAgentOrder={(agentIds) => handleUpdateAgentOrder(team.id, agentIds)}
            />
          ))
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

export default WorkflowPage;