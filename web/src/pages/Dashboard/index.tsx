import React, { useEffect, useState, useCallback } from 'react';
import { Card, Row, Col, Table, Tag, Space, Typography, Button, Modal, Select, Input, Empty } from 'antd';
import {
  ProjectOutlined,
  PlusOutlined,
  ArrowRightOutlined,
  TeamOutlined,
  RobotOutlined,
  RocketOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '@/api/client';
import type { Project, Thread, WorkflowTemplate } from '@/types';
import WorkflowCard from './WorkflowCard';

const { Title, Text } = Typography;

// 核心统计卡片组件 - 支持自定义颜色
const CoreStatCard: React.FC<{
  title: string;
  value: number;
  icon: React.ReactNode;
  color: string;
  bgColor: string;
  onClick: () => void;
}> = ({ title, value, icon, color, bgColor, onClick }) => (
  <Card
    hoverable
    onClick={onClick}
    style={{
      cursor: 'pointer',
      borderRadius: 10,
      border: '1px solid var(--border-color)',
      background: 'var(--bg-elevated)',
    }}
    styles={{
      body: { padding: '12px 16px' },
    }}
  >
    <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
      <div
        style={{
          width: 40,
          height: 40,
          borderRadius: 8,
          background: bgColor,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        {icon}
      </div>
      <div>
        <Text style={{ fontSize: 12, color: 'var(--text-secondary)' }}>{title}</Text>
        <Text strong style={{ fontSize: 20, color: color, display: 'block' }}>{value}</Text>
      </div>
    </div>
  </Card>
);

// 活跃任务卡片
const ActiveThreadCard: React.FC<{
  thread: ActiveThreadInfo;
  onClick: () => void;
}> = ({ thread, onClick }) => (
  <Card
    hoverable
    onClick={onClick}
    size="small"
    style={{ borderRadius: 12, cursor: 'pointer', border: '1px solid var(--border-color)', background: 'var(--bg-elevated)' }}
    styles={{ body: { padding: '12px 16px' } }}
  >
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
      <Text strong style={{ fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '60%', color: 'var(--text-primary)' }}>
        {thread.name || `任务 #${thread.id.slice(0, 8)}`}
      </Text>
      <Tag color="processing" style={{ fontSize: 11 }}>
        <span className="pulse-dot" style={{
          display: 'inline-block',
          width: 6,
          height: 6,
          borderRadius: '50%',
          background: '#52c41a',
          marginRight: 4,
        }} />
        运行中
      </Tag>
    </div>
    <Space size="small" split={<span style={{ color: 'var(--border-color)' }}>|</span>} style={{ marginBottom: 8 }}>
      {thread.projectName && (
        <Text type="secondary" style={{ fontSize: 11 }}>
          <ProjectOutlined style={{ marginRight: 4 }} />
          {thread.projectName}
        </Text>
      )}
      {thread.workflowName && (
        <Text type="secondary" style={{ fontSize: 11 }}>
          <TeamOutlined style={{ marginRight: 4 }} />
          {thread.workflowName}
        </Text>
      )}
    </Space>
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
      {thread.currentAgentNames?.map((agentName) => (
        <Tag key={agentName} color="blue" style={{ fontSize: 11 }}>
          <RobotOutlined style={{ marginRight: 4 }} />
          {agentName}
        </Tag>
      ))}
    </div>
  </Card>
);

// 快速开始Modal组件
const QuickStartModal: React.FC<{
  visible: boolean;
  onClose: () => void;
  projects: Project[];
  workflows: WorkflowTemplate[];
  onStart: (projectId: string, workflowId: string, taskName: string) => void;
  loading: boolean;
}> = ({ visible, onClose, projects, workflows, onStart, loading }) => {
  const [selectedProject, setSelectedProject] = useState<string>();
  const [selectedWorkflow, setSelectedWorkflow] = useState<string>();
  const [taskName, setTaskName] = useState('');

  const handleStart = () => {
    if (selectedProject && selectedWorkflow) {
      onStart(selectedProject, selectedWorkflow, taskName || '新任务');
      setTaskName('');
      setSelectedProject(undefined);
      setSelectedWorkflow(undefined);
      onClose();
    }
  };

  return (
    <Modal
      title={<Space><RocketOutlined style={{ color: 'var(--color-primary)' }} /><span>快速开始新任务</span></Space>}
      open={visible}
      onCancel={onClose}
      onOk={handleStart}
      okText="开始"
      cancelText="取消"
      confirmLoading={loading}
      okButtonProps={{ disabled: !selectedProject || !selectedWorkflow }}
      width={480}
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16, marginTop: 16 }}>
        <div>
          <Text strong style={{ marginBottom: 8, display: 'block' }}>选择项目</Text>
          <Select
            placeholder="选择一个项目"
            value={selectedProject}
            onChange={setSelectedProject}
            style={{ width: '100%' }}
            options={projects.map(p => ({ label: p.name, value: p.id }))}
            showSearch
            filterOption={(input, option) => (option?.label ?? '').toLowerCase().includes(input.toLowerCase())}
          />
        </div>
        <div>
          <Text strong style={{ marginBottom: 8, display: 'block' }}>选择Agent团队</Text>
          <Select
            placeholder="选择一个Agent团队"
            value={selectedWorkflow}
            onChange={setSelectedWorkflow}
            style={{ width: '100%' }}
            options={workflows.map(w => ({ label: `${w.name} (${w.agentIds?.length || 0} Agents)`, value: w.id }))}
            showSearch
            filterOption={(input, option) => (option?.label ?? '').toLowerCase().includes(input.toLowerCase())}
          />
        </div>
        <div>
          <Text strong style={{ marginBottom: 8, display: 'block' }}>任务名称（可选）</Text>
          <Input placeholder="例如：实现用户登录功能" value={taskName} onChange={(e) => setTaskName(e.target.value)} />
        </div>
      </div>
    </Modal>
  );
};

// 活跃线程信息
interface ActiveThreadInfo extends Thread {
  projectName?: string;
  workflowName?: string;
  currentAgentNames?: string[];
}

// 团队资产信息类型
interface WorkflowWithAssets {
  id: string;
  name: string;
  description: string;
  isSystem: boolean;
  agentCount: number;
  agents: Array<{
    id: string;
    name: string;
    role: string;
    skillsCount: number;
    commandsCount: number;
    subagentsCount: number;
    rulesCount: number;
  }>;
  skills: number;
  commands: number;
  subagents: number;
  rules: number;
  totalAssets: number;
}

const Dashboard: React.FC = () => {
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeThreads, setActiveThreads] = useState<ActiveThreadInfo[]>([]);
  const [workflowsWithAssets, setWorkflowsWithAssets] = useState<WorkflowWithAssets[]>([]);
  const [stats, setStats] = useState({
    totalProjects: 0,
    activeThreads: 0,
    workflowTeams: 0,
    agentRoles: 0,
    totalSkills: 0,
    totalCommands: 0,
    totalSubagents: 0,
    totalRules: 0,
  });
  const [loading, setLoading] = useState(false);
  const [workflows, setWorkflows] = useState<WorkflowTemplate[]>([]);
  const [quickStartVisible, setQuickStartVisible] = useState(false);
  const [quickStartLoading, setQuickStartLoading] = useState(false);

  // 加载Dashboard统计数据
  const loadDashboardStats = useCallback(async () => {
    try {
      const statsData = await api.dashboard.getStats();
      setStats({
        totalProjects: statsData.totalProjects,
        activeThreads: statsData.activeThreads,
        workflowTeams: statsData.workflowTeams,
        agentRoles: statsData.agentRoles,
        totalSkills: statsData.totalSkills,
        totalCommands: statsData.totalCommands,
        totalSubagents: statsData.totalSubagents,
        totalRules: statsData.totalRules,
      });
    } catch (error) {
      console.error('Failed to load dashboard stats:', error);
    }
  }, []);

  // 加载活跃线程
  const loadActiveThreads = useCallback(async () => {
    try {
      const threads = await api.dashboard.getActiveThreads();
      const threadsArray = Array.isArray(threads) ? threads : [];
      setActiveThreads(threadsArray.map(t => ({
        id: t.id,
        projectId: t.projectId,
        name: t.name,
        status: t.status as Thread['status'],
        currentPhase: t.currentPhase as Thread['currentPhase'],
        currentAgentNames: t.currentAgentNames || [],
        workflowTemplateId: t.workflowTemplateId,
        createdAt: t.createdAt,
        updatedAt: t.updatedAt,
        depth: 0,
        projectName: t.projectName,
        workflowName: t.workflowName,
      })));
      setStats(prev => ({ ...prev, activeThreads: threadsArray.length }));
    } catch (error) {
      console.error('Failed to load active threads:', error);
    }
  }, []);

  // 加载团队资产数据
  const loadWorkflowsWithAssets = useCallback(async () => {
    try {
      const workflowsData = await api.dashboard.getWorkflowsWithAssets();
      const workflowsArray = Array.isArray(workflowsData) ? workflowsData : [];
      setWorkflowsWithAssets(workflowsArray);
    } catch (error) {
      console.error('Failed to load workflows with assets:', error);
    }
  }, []);

  // 加载基础数据
  const loadBaseData = useCallback(async () => {
    setLoading(true);
    try {
      const [projectData, workflowsData] = await Promise.all([
        api.projects.list(),
        api.workflows.list(),
      ]);
      setProjects((projectData as unknown as Project[]) || []);
      setWorkflows((workflowsData as unknown as WorkflowTemplate[]) || []);
      await Promise.all([loadDashboardStats(), loadActiveThreads(), loadWorkflowsWithAssets()]);
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
    } finally {
      setLoading(false);
    }
  }, [loadDashboardStats, loadActiveThreads, loadWorkflowsWithAssets]);

  // 初始加载
  useEffect(() => {
    loadBaseData();
  }, [loadBaseData]);

  // 10秒轮询活跃线程
  useEffect(() => {
    const interval = setInterval(loadActiveThreads, 10000);
    return () => clearInterval(interval);
  }, [loadActiveThreads]);

  // 处理快速开始
  const handleQuickStart = async (projectId: string, _workflowId: string, taskName: string) => {
    setQuickStartLoading(true);
    try {
      const thread = await api.threads.create(projectId, taskName);
      navigate(`/projects/${projectId}/threads/${thread.id}`);
    } catch (error) {
      console.error('Failed to create thread:', error);
    } finally {
      setQuickStartLoading(false);
    }
  };

  const projectColumns = [
    { title: '项目名称', dataIndex: 'name', key: 'name', render: (name: string, record: Project) => <a onClick={() => navigate(`/projects/${record.id}`)} style={{ color: 'var(--color-primary)' }}>{name}</a> },
    { title: '路径', dataIndex: 'localPath', key: 'localPath', render: (path: string) => <Text style={{ fontSize: 12, color: 'var(--text-secondary)' }}>{path}</Text> },
    { title: '更新时间', dataIndex: 'updatedAt', key: 'updatedAt', render: (date: string) => new Date(date).toLocaleString() },
  ];

  return (
    <div style={{ padding: 24, maxWidth: 1400, margin: '0 auto' }}>
      {/* Hero Section - 居中布局 */}
      <Card
        style={{
          marginBottom: 20,
          borderRadius: 12,
          background: 'var(--bg-container)',
          border: '1px solid var(--border-color)',
          boxShadow: 'var(--shadow-sm)',
        }}
        styles={{ body: { padding: '24px 32px', textAlign: 'center' } }}
      >
        <Title level={3} style={{ margin: 0, marginBottom: 8, fontWeight: 600, color: 'var(--text-primary)' }}>
          让AI Agent团队，帮你高效完成开发任务
        </Title>
        <Text type="secondary" style={{ fontSize: 14, marginBottom: 16, display: 'block' }}>
          无需手动协调，AI团队自动分工、执行、交付，让开发效率翻倍
        </Text>
        <Button
          type="primary"
          size="large"
          icon={<RocketOutlined />}
          onClick={() => setQuickStartVisible(true)}
          style={{ borderRadius: 8, fontWeight: 600, height: 40 }}
        >
          开始新任务
        </Button>
      </Card>

      {/* 并排布局：项目与任务 + 团队与资产 */}
      <Row gutter={20} style={{ marginBottom: 20 }}>
        {/* 区块一：项目与任务 */}
        <Col xs={24} lg={12}>
          <Card
            style={{ borderRadius: 10, background: 'var(--bg-container)', border: '1px solid var(--border-color)', height: '100%' }}
            styles={{ body: { padding: 16 } }}
          >
            {/* 区块标题 - Level 1 最突出 */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                marginBottom: 16,
                padding: '10px 16px',
                background: 'linear-gradient(135deg, var(--color-section-blue-bg) 0%, var(--bg-elevated) 100%)',
                borderRadius: 8,
                borderLeft: '5px solid var(--color-section-blue)',
              }}
            >
              <Text strong style={{ fontSize: 15, color: 'var(--text-primary)', letterSpacing: 0.5 }}>
                <ProjectOutlined style={{ marginRight: 10, color: 'var(--color-section-blue)', fontSize: 16 }} />
                项目与任务
              </Text>
              <Button type="link" size="small" onClick={() => navigate('/projects')} style={{ color: 'var(--color-section-blue)', fontSize: 12 }}>
                管理 <ArrowRightOutlined />
              </Button>
            </div>

            {/* 统计卡片 - 只保留项目相关 */}
            <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
              <Col span={12}>
                <CoreStatCard
                  title="项目"
                  value={stats.totalProjects}
                  icon={<ProjectOutlined style={{ color: '#1890ff', fontSize: 18 }} />}
                  color="#1890ff"
                  bgColor="rgba(24, 144, 255, 0.1)"
                  onClick={() => navigate('/projects')}
                />
              </Col>
              <Col span={12}>
                <CoreStatCard
                  title="进行中"
                  value={stats.activeThreads}
                  icon={<ClockCircleOutlined style={{ color: '#52c41a', fontSize: 18 }} />}
                  color="#52c41a"
                  bgColor="rgba(82, 196, 26, 0.1)"
                  onClick={() => navigate('/projects')}
                />
              </Col>
            </Row>

            {/* 活跃任务 */}
            <div style={{ marginBottom: 16 }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
                <Space size={6}>
                  <ClockCircleOutlined style={{ color: 'var(--color-primary)', fontSize: 14 }} />
                  <Text style={{ fontSize: 13, color: 'var(--text-primary)', fontWeight: 500 }}>活跃任务</Text>
                  {stats.activeThreads > 0 && <Tag color="processing" style={{ fontSize: 11, margin: 0, padding: '0 6px' }}>{stats.activeThreads}</Tag>}
                </Space>
              </div>
              {activeThreads.length > 0 ? (
                <Row gutter={[12, 12]}>
                  {activeThreads.slice(0, 2).map((thread) => (
                    <Col span={12} key={thread.id}>
                      <ActiveThreadCard
                        thread={thread}
                        onClick={() => navigate(`/projects/${thread.projectId}/threads/${thread.id}`)}
                      />
                    </Col>
                  ))}
                </Row>
              ) : (
                <div style={{ padding: '16px', textAlign: 'center', background: 'var(--bg-elevated)', borderRadius: 8 }}>
                  <RocketOutlined style={{ fontSize: 24, color: 'var(--color-primary)', marginBottom: 8 }} />
                  <Text style={{ fontSize: 13, color: 'var(--text-secondary)', marginBottom: 12, display: 'block' }}>暂无活跃任务</Text>
                  <Button type="default" icon={<RocketOutlined />} onClick={() => setQuickStartVisible(true)} style={{ borderRadius: 6, border: '1px solid var(--color-primary)', color: 'var(--color-primary)' }}>
                    开始新任务
                  </Button>
                </div>
              )}
            </div>

            {/* 最近项目 */}
            <div>
              <Text style={{ fontSize: 13, color: 'var(--text-primary)', fontWeight: 500, marginBottom: 10, display: 'block' }}>
                最近项目
              </Text>
              {projects.length > 0 ? (
                <Table
                  dataSource={projects.slice(0, 2)}
                  columns={projectColumns}
                  rowKey="id"
                  loading={loading}
                  pagination={false}
                  size="small"
                  style={{ fontSize: 12 }}
                />
              ) : (
                <Empty description="暂无项目" image={Empty.PRESENTED_IMAGE_SIMPLE} style={{ padding: '12px 0' }}>
                  <Button type="primary" size="small" onClick={() => navigate('/projects')}>创建</Button>
                </Empty>
              )}
            </div>
          </Card>
        </Col>

        {/* 区块二：团队与资产 */}
        <Col xs={24} lg={12}>
          <Card
            style={{ borderRadius: 10, background: 'var(--bg-container)', border: '1px solid var(--border-color)', height: '100%' }}
            styles={{ body: { padding: 16 } }}
          >
            {/* 区块标题 - Level 1 最突出，紫色系 */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                marginBottom: 16,
                padding: '10px 16px',
                background: 'linear-gradient(135deg, var(--color-section-purple-bg) 0%, var(--bg-elevated) 100%)',
                borderRadius: 8,
                borderLeft: '5px solid var(--color-section-purple)',
              }}
            >
              <Text strong style={{ fontSize: 15, color: 'var(--text-primary)', letterSpacing: 0.5 }}>
                <TeamOutlined style={{ marginRight: 10, color: 'var(--color-section-purple)', fontSize: 16 }} />
                团队与资产
              </Text>
              <Space size={8}>
                <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => navigate('/workflow')} style={{ borderRadius: 6, fontSize: 12 }}>
                  新建团队
                </Button>
                <Button type="link" size="small" onClick={() => navigate('/workflow')} style={{ color: 'var(--color-section-purple)', fontSize: 12 }}>
                  配置 <ArrowRightOutlined />
                </Button>
              </Space>
            </div>

            {/* 第一行：Agent团队 + 角色配置 */}
            <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
              <Col span={12}>
                <CoreStatCard
                  title="Agent团队"
                  value={stats.workflowTeams}
                  icon={<TeamOutlined style={{ color: '#722ed1', fontSize: 18 }} />}
                  color="#722ed1"
                  bgColor="rgba(114, 46, 209, 0.1)"
                  onClick={() => navigate('/workflow')}
                />
              </Col>
              <Col span={12}>
                <CoreStatCard
                  title="角色配置"
                  value={stats.agentRoles}
                  icon={<RobotOutlined style={{ color: '#faad14', fontSize: 18 }} />}
                  color="#faad14"
                  bgColor="rgba(250, 173, 20, 0.1)"
                  onClick={() => navigate('/agents/roles')}
                />
              </Col>
            </Row>

            {/* 团队卡片列表标题 - 子标题样式 */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
              <Space size={6}>
                <TeamOutlined style={{ color: 'var(--color-section-purple)', fontSize: 13 }} />
                <Text style={{ fontSize: 13, color: 'var(--text-primary)', fontWeight: 500 }}>Agent团队</Text>
                <Tag style={{ margin: 0, background: 'var(--color-section-purple-bg)', border: '1px solid var(--color-section-purple)', color: 'var(--color-section-purple)', fontSize: 11 }}>{workflowsWithAssets.length}</Tag>
              </Space>
            </div>
            {workflowsWithAssets.length > 0 ? (
              <Row gutter={[12, 12]}>
                {workflowsWithAssets.slice(0, 4).map((workflow) => (
                  <Col span={12} key={workflow.id}>
                    <WorkflowCard
                      id={workflow.id}
                      name={workflow.name}
                      description={workflow.description}
                      isSystem={workflow.isSystem}
                      agents={workflow.agents}
                      skills={workflow.skills}
                      commands={workflow.commands}
                      subagents={workflow.subagents}
                      rules={workflow.rules}
                      totalAssets={workflow.totalAssets}
                    />
                  </Col>
                ))}
              </Row>
            ) : (
              <Empty description="暂无团队" image={Empty.PRESENTED_IMAGE_SIMPLE} style={{ padding: '12px 0' }}>
                <Button type="primary" size="small" onClick={() => navigate('/workflow')}>创建</Button>
              </Empty>
            )}

            {/* 查看全部 */}
            {workflowsWithAssets.length > 4 && (
              <div style={{ textAlign: 'center', marginTop: 12 }}>
                <Button type="link" size="small" onClick={() => navigate('/workflow')} style={{ color: 'var(--color-primary)', fontSize: 12 }}>
                  查看全部 ({workflowsWithAssets.length}) <ArrowRightOutlined />
                </Button>
              </div>
            )}

                      </Card>
        </Col>
      </Row>

      {/* 快速开始Modal */}
      <QuickStartModal
        visible={quickStartVisible}
        onClose={() => setQuickStartVisible(false)}
        projects={projects}
        workflows={workflows}
        onStart={handleQuickStart}
        loading={quickStartLoading}
      />
    </div>
  );
};

export default Dashboard;