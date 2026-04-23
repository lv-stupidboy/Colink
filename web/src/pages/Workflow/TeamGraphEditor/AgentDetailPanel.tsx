// web/src/pages/Workflow/TeamGraphEditor/AgentDetailPanel.tsx
import React, { useState, useEffect } from 'react';
import { Descriptions, Button, Tag, Divider, Form, Input, Select, Switch, message, Spin, Collapse, Space } from 'antd';
import { DeleteOutlined, EditOutlined, SaveOutlined, CloseOutlined, BookOutlined, ApiOutlined, CodeOutlined, SafetyCertificateOutlined, SettingOutlined } from '@ant-design/icons';
import AgentTypeIcon from '@/components/AgentTypeIcon';
import { useGraphStore } from './useGraphStore';
import api from '@/api/client';
import type { AgentConfig, BaseAgent, Skill, Subagent, Command, Rule, Settings } from '@/types';

interface AgentDetailPanelProps {
  nodeId: string;
  readOnly?: boolean;
  onEditComplete?: () => void;
}

const AgentDetailPanel: React.FC<AgentDetailPanelProps> = ({ nodeId, readOnly = false, onEditComplete }) => {
  const { nodes, removeNode, mode, refreshAgent } = useGraphStore();
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  // Asset lists for binding
  const [baseAgents, setBaseAgents] = useState<BaseAgent[]>([]);
  const [skills, setSkills] = useState<Skill[]>([]);
  const [subagents, setSubagents] = useState<Subagent[]>([]);
  const [commands, setCommands] = useState<Command[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [settings, setSettings] = useState<Settings[]>([]);
  const [loadingAssets, setLoadingAssets] = useState(false);

  // Selected assets
  const [selectedSkillIds, setSelectedSkillIds] = useState<string[]>([]);
  const [selectedSubagentIds, setSelectedSubagentIds] = useState<string[]>([]);
  const [selectedCommandIds, setSelectedCommandIds] = useState<string[]>([]);
  const [selectedRuleIds, setSelectedRuleIds] = useState<string[]>([]);
  const [selectedSettingsIds, setSelectedSettingsIds] = useState<string[]>([]);

  const node = nodes.find(n => n.id === nodeId);
  const agent: AgentConfig | undefined = node?.data?.agent as AgentConfig | undefined;

  // Load asset lists on mount
  useEffect(() => {
    loadAssetLists();
  }, []);

  // Load agent's bound assets when editing starts
  useEffect(() => {
    if (editing && agent) {
      loadAgentAssets(agent.id);
    }
  }, [editing, agent?.id]);

  const loadAssetLists = async () => {
    setLoadingAssets(true);
    try {
      const [baseAgentsData, skillsData, subagentsData, commandsData, rulesData, settingsData] = await Promise.all([
        api.baseAgents.list(),
        api.skills.list({ pageSize: 100 }),
        api.subagents.list({ pageSize: 100 }),
        api.commands.list({ pageSize: 100 }),
        api.rules.list({ pageSize: 100 }),
        api.settings.list({ pageSize: 100 }),
      ]);
      setBaseAgents(baseAgentsData);
      setSkills(skillsData.data || []);
      setSubagents(subagentsData.data || []);
      setCommands(commandsData.data || []);
      setRules(rulesData.data || []);
      setSettings(settingsData.data || []);
    } catch (error) {
      console.error('Failed to load asset lists:', error);
    } finally {
      setLoadingAssets(false);
    }
  };

  const loadAgentAssets = async (agentId: string) => {
    try {
      const [skillsRes, subagentsRes, commandsRes, rulesRes, settingsRes] = await Promise.all([
        api.agents.getSkills(agentId),
        api.agents.getSubagents(agentId),
        api.commands.getAgentCommands(agentId),
        api.rules.getAgentRules(agentId),
        api.settings.getAgentSettings(agentId),
      ]);
      setSelectedSkillIds(skillsRes.skills?.map(s => s.id) || []);
      setSelectedSubagentIds(subagentsRes.subagents?.map(s => s.id) || []);
      setSelectedCommandIds(commandsRes.commands?.map(c => c.id) || []);
      setSelectedRuleIds(rulesRes.rules?.map(r => r.id) || []);
      setSelectedSettingsIds(settingsRes.settings?.map(s => s.id) || []);
    } catch (error) {
      console.error('Failed to load agent assets:', error);
    }
  };

  if (!agent) {
    return (
      <div className="panel-content">
        <div className="panel-section">
          <p style={{ color: 'var(--text-secondary)' }}>Agent 信息不可用</p>
        </div>
      </div>
    );
  }

  const handleRemove = () => {
    removeNode(nodeId);
  };

  const handleEdit = () => {
    form.setFieldsValue({
      name: agent.name,
      description: agent.description,
      baseAgentId: agent.baseAgentId,
      requiresHuman: agent.requiresHuman,
      systemPrompt: agent.systemPrompt,
      mentionPatterns: agent.mentionPatterns || [],
    });
    setEditing(true);
  };

  const handleCancel = () => {
    setEditing(false);
    form.resetFields();
    setSelectedSkillIds([]);
    setSelectedSubagentIds([]);
    setSelectedCommandIds([]);
    setSelectedRuleIds([]);
    setSelectedSettingsIds([]);
  };

  const handleSave = async (values: Partial<AgentConfig>) => {
    setSaving(true);
    try {
      // Update agent basic info
      await api.agents.update(agent.id, values);

      // Update asset bindings
      await Promise.all([
        api.agents.bindSkills(agent.id, selectedSkillIds),
        api.agents.bindSubagents(agent.id, selectedSubagentIds),
        api.commands.bindCommandsToAgent(agent.id, selectedCommandIds),
        api.rules.bindRulesToAgent(agent.id, selectedRuleIds),
        api.settings.bindToAgent(agent.id, selectedSettingsIds),
      ]);

      message.success('更新成功');

      // Refresh agent data in store
      if (refreshAgent) {
        await refreshAgent(agent.id);
      }

      setEditing(false);
      onEditComplete?.();
    } catch (error) {
      message.error('更新失败');
      console.error('Failed to save:', error);
    } finally {
      setSaving(false);
    }
  };

  // Render edit form
  if (editing && !readOnly) {
    return (
      <div className="panel-content">
        <Spin spinning={loadingAssets}>
          <Form
            form={form}
            layout="vertical"
            onFinish={handleSave}
            size="small"
          >
            <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
              <Input placeholder="角色名称" />
            </Form.Item>

            <Form.Item name="description" label="描述">
              <Input.TextArea rows={2} placeholder="角色描述" />
            </Form.Item>

            <Form.Item name="baseAgentId" label="基础Agent">
              <Select placeholder="选择基础Agent" allowClear>
                {baseAgents.map(ag => (
                  <Select.Option key={ag.id} value={ag.id}>
                    {ag.name} ({ag.type === 'claude_code' ? 'Claude Code' : 'OpenCode'})
                  </Select.Option>
                ))}
              </Select>
            </Form.Item>

            <Form.Item
              name="requiresHuman"
              label="需要人工参与"
              valuePropName="checked"
            >
              <Switch checkedChildren="是" unCheckedChildren="否" />
            </Form.Item>

            <Form.Item name="systemPrompt" label="系统提示词" rules={[{ required: true, message: '请输入系统提示词' }]}>
              <Input.TextArea rows={6} placeholder="系统提示词，定义Agent的行为和能力" />
            </Form.Item>

            <Form.Item name="mentionPatterns" label="触发模式">
              <Select
                mode="tags"
                placeholder="输入触发模式，如 @developer"
                tokenSeparators={[',', ' ']}
              />
            </Form.Item>

            <Collapse
              size="small"
              items={[
                {
                  key: 'assets',
                  label: '绑定资产',
                  children: (
                    <Space direction="vertical" style={{ width: '100%' }} size="small">
                      <Form.Item label={<Space><BookOutlined /> Skills</Space>}>
                        <Select
                          mode="multiple"
                          placeholder="选择 Skills"
                          value={selectedSkillIds}
                          onChange={setSelectedSkillIds}
                          optionFilterProp="label"
                          options={skills.map(s => ({ label: s.name, value: s.id }))}
                        />
                      </Form.Item>

                      <Form.Item label={<Space><ApiOutlined /> Subagents</Space>}>
                        <Select
                          mode="multiple"
                          placeholder="选择 Subagents"
                          value={selectedSubagentIds}
                          onChange={setSelectedSubagentIds}
                          optionFilterProp="label"
                          options={subagents.map(s => ({ label: s.name, value: s.id }))}
                        />
                      </Form.Item>

                      <Form.Item label={<Space><CodeOutlined /> Commands</Space>}>
                        <Select
                          mode="multiple"
                          placeholder="选择 Commands"
                          value={selectedCommandIds}
                          onChange={setSelectedCommandIds}
                          optionFilterProp="label"
                          options={commands.map(c => ({ label: c.name, value: c.id }))}
                        />
                      </Form.Item>

                      <Form.Item label={<Space><SafetyCertificateOutlined /> Rules</Space>}>
                        <Select
                          mode="multiple"
                          placeholder="选择 Rules"
                          value={selectedRuleIds}
                          onChange={setSelectedRuleIds}
                          optionFilterProp="label"
                          options={rules.map(r => ({ label: r.name, value: r.id }))}
                        />
                      </Form.Item>

                      <Form.Item label={<Space><SettingOutlined /> Settings</Space>}>
                        <Select
                          mode="multiple"
                          placeholder="选择 Settings"
                          value={selectedSettingsIds}
                          onChange={setSelectedSettingsIds}
                          optionFilterProp="label"
                          options={settings.map(s => ({ label: s.name, value: s.id }))}
                        />
                      </Form.Item>
                    </Space>
                  ),
                },
              ]}
            />

            <Divider />

            <Space style={{ width: '100%' }} direction="vertical">
              <Button
                type="primary"
                htmlType="submit"
                icon={<SaveOutlined />}
                loading={saving}
                block
              >
                保存
              </Button>
              <Button
                icon={<CloseOutlined />}
                onClick={handleCancel}
                block
              >
                取消
              </Button>
            </Space>
          </Form>
        </Spin>
      </div>
    );
  }

  // Render view mode
  return (
    <div className="panel-content">
      <div className="panel-header" style={{ display: 'flex', alignItems: 'center', gap: 12, justifyContent: 'space-between' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <AgentTypeIcon
            requiresHuman={agent.requiresHuman}
            isSystem={agent.isSystem}
            size={24}
          />
          <span className="panel-title">{agent.name}</span>
        </div>
        {!readOnly && mode === 'edit' && (
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={handleEdit}
          >
            编辑
          </Button>
        )}
      </div>

      <Divider />

      <Descriptions column={1} size="small">
        <Descriptions.Item label="角色">{agent.role}</Descriptions.Item>
        <Descriptions.Item label="基础 Agent">{agent.baseAgent?.name || '-'}</Descriptions.Item>
        <Descriptions.Item label="需要人工">
          {agent.requiresHuman ? <Tag color="blue">是</Tag> : <Tag>否</Tag>}
        </Descriptions.Item>
        {agent.isSystem && (
          <Descriptions.Item label="系统角色">
            <Tag color="purple">系统预置</Tag>
          </Descriptions.Item>
        )}
        <Descriptions.Item label="描述">
          {agent.description || '-'}
        </Descriptions.Item>
        <Descriptions.Item label="系统提示词">
          {agent.systemPrompt ? (
            agent.systemPrompt.length > 50
              ? `${agent.systemPrompt.substring(0, 50)}...`
              : agent.systemPrompt
          ) : '-'}
        </Descriptions.Item>
      </Descriptions>

      {!readOnly && mode === 'edit' && !agent.isSystem && (
        <>
          <Divider />
          <div className="panel-section">
            <Button
              danger
              icon={<DeleteOutlined />}
              onClick={handleRemove}
              block
            >
              从团队移除
            </Button>
          </div>
        </>
      )}
    </div>
  );
};

export default AgentDetailPanel;